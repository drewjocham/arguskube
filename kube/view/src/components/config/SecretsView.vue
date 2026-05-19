<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useResources } from '../../composables/useWails'
import SectionTabs from '../shared/SectionTabs.vue'
import CloudSecretsTab from './CloudSecretsTab.vue'
import { parseCertificateValidity, expiryStatus } from '../../lib/x509'

// Secrets is meaningful enough to deserve its own view — separated
// from ConfigMapList because it carries five SRE-specific surfaces
// (reveal/decode, TLS expiry, orphan detection, GitOps wrapping,
// RBAC audit) that don't apply to ConfigMaps.
//
// This file ships the Browse tab (full) and TLS Health tab (full)
// today. The remaining three tabs (Orphaned / GitOps / Access) land
// with the data they each need wired on the backend — the IA is in
// place so adding them is purely additive.

const { result, detail, loading, error, detailLoading, listResources, getResourceDetail } = useResources()

const secrets = ref([])
const expandedSecret = ref(null) // namespace + name compound key
const search = ref('')
const groupBy = ref('namespace') // 'namespace' | 'type' | 'none'
const collapsedGroups = ref(new Set())

// Reveal state — keyed by `${namespace}/${name}/${key}` so reveals
// are scoped to one value, not the whole secret. The Set holds keys
// the user has explicitly clicked reveal on. Switching collapse or
// navigating away clears it (handled by the watcher below).
const revealed = ref(new Set())

// Per-key edit state (Base64 painkiller — see SECRETS_TABS comment).
// Stored as { 'ns/name/key': 'edited plaintext' } so the user's edits
// survive a re-render without leaking across keys.
const drafts = ref({})

// --- Tab framework -------------------------------------------------

const SECRETS_TABS = [
  { id: 'browse', label: 'Browse' },
  { id: 'cloud', label: 'Cloud (AWS/GCP)' },
  { id: 'tls', label: 'TLS Health' },
  { id: 'orphaned', label: 'Orphaned' },
  { id: 'gitops', label: 'GitOps' },
  { id: 'access', label: 'Access' },
]
const activeTab = ref('browse')

// --- Browse-tab data shaping --------------------------------------

function mapItems() {
  if (result.value && result.value.items && result.value.items.length > 0) {
    secrets.value = result.value.items.map((item) => ({
      name: item.name,
      namespace: item.namespace,
      // The list endpoint returns "Opaque", "kubernetes.io/tls", …
      type: item.fields?.type || 'Opaque',
      // Number of keys stored in `data:` — visible without revealing.
      data: item.fields?.data || '0',
      age: item.age || '—',
    }))
  } else {
    secrets.value = []
  }
}

async function refresh(force = false) {
  await listResources('secrets', '', force)
  mapItems()
}

onMounted(() => refresh())

// Reset reveal/drafts whenever the user switches tab. Editing a value
// in Browse and then jumping to TLS Health and back shouldn't leak the
// draft into a different secret render.
watch(activeTab, () => {
  revealed.value = new Set()
  drafts.value = {}
  expandedSecret.value = null
})

const filteredSecrets = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return secrets.value
  return secrets.value.filter((s) =>
    s.name.toLowerCase().includes(q) ||
    s.namespace.toLowerCase().includes(q) ||
    s.type.toLowerCase().includes(q)
  )
})

const grouped = computed(() => {
  if (groupBy.value === 'none') {
    return [{ key: 'all', label: `All secrets (${filteredSecrets.value.length})`, items: filteredSecrets.value }]
  }
  const map = new Map()
  for (const s of filteredSecrets.value) {
    const key = groupBy.value === 'type' ? s.type : s.namespace
    if (!map.has(key)) map.set(key, [])
    map.get(key).push(s)
  }
  const out = []
  for (const [key, items] of map.entries()) {
    out.push({ key, label: `${key} (${items.length})`, items })
  }
  out.sort((a, b) => a.key.localeCompare(b.key))
  return out
})

function toggleGroup(key) {
  const next = new Set(collapsedGroups.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  collapsedGroups.value = next
}
function isGroupCollapsed(key) { return collapsedGroups.value.has(key) }

function secretKey(s) { return `${s.namespace}/${s.name}` }

async function toggleExpand(s) {
  const k = secretKey(s)
  if (expandedSecret.value === k) {
    expandedSecret.value = null
    return
  }
  expandedSecret.value = k
  await getResourceDetail('secrets', s.namespace, s.name)
}

// --- Reveal + Base64 painkiller -----------------------------------

function valueKey(s, dataKey) { return `${s.namespace}/${s.name}/${dataKey}` }

function isRevealed(s, dataKey) { return revealed.value.has(valueKey(s, dataKey)) }

function toggleReveal(s, dataKey) {
  const k = valueKey(s, dataKey)
  const next = new Set(revealed.value)
  if (next.has(k)) {
    next.delete(k)
    // Drop any in-progress draft for this key when re-hiding.
    if (drafts.value[k] !== undefined) {
      const nextDrafts = { ...drafts.value }
      delete nextDrafts[k]
      drafts.value = nextDrafts
    }
  } else {
    next.add(k)
  }
  revealed.value = next
}

function decodeBase64(raw) {
  if (raw == null) return ''
  // Backend returns secret values base64-encoded as Kubernetes stores
  // them. If the value isn't valid base64 (e.g. backend pre-decoded
  // it), return the raw string so the user still sees something.
  try {
    const bin = atob(String(raw).replace(/\s+/g, ''))
    // Best-effort UTF-8 decode — most secrets are tokens/strings/JSON.
    const bytes = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
    return new TextDecoder('utf-8', { fatal: false }).decode(bytes)
  } catch {
    return String(raw)
  }
}

function encodeBase64(plain) {
  try {
    const bytes = new TextEncoder().encode(plain)
    let bin = ''
    for (const b of bytes) bin += String.fromCharCode(b)
    return btoa(bin)
  } catch {
    return ''
  }
}

function decodedValue(s, dataKey, rawValue) {
  const k = valueKey(s, dataKey)
  if (k in drafts.value) return drafts.value[k]
  return decodeBase64(rawValue)
}

function onDraftInput(s, dataKey, value) {
  const k = valueKey(s, dataKey)
  drafts.value = { ...drafts.value, [k]: value }
}

// --- TLS Health tab -----------------------------------------------

// Live list of TLS-type secrets with their parsed certificate
// validity. Parsing is async (we fetch detail for each TLS secret)
// so this builds up as the data arrives.
const tlsRows = ref([])
const tlsLoading = ref(false)
const tlsError = ref('')

async function loadTLSHealth() {
  tlsLoading.value = true
  tlsError.value = ''
  tlsRows.value = []
  try {
    // Make sure we have the secret list.
    if (!secrets.value.length) await refresh()
    const candidates = secrets.value.filter((s) => s.type === 'kubernetes.io/tls')
    // Fetch detail in parallel — but bounded so a 200-cert cluster
    // doesn't fire 200 RPCs at once.
    const BATCH = 8
    const rows = []
    for (let i = 0; i < candidates.length; i += BATCH) {
      const slice = candidates.slice(i, i + BATCH)
      const fetched = await Promise.all(slice.map(async (s) => {
        try {
          await getResourceDetail('secrets', s.namespace, s.name)
          const data = detail.value?.data
          if (!data || !data['tls.crt']) return { ...s, error: 'no tls.crt key' }
          const pem = decodeBase64(data['tls.crt'])
          const v = parseCertificateValidity(pem)
          if (!v) return { ...s, error: 'could not parse certificate' }
          const status = expiryStatus(v.notAfter)
          return {
            ...s,
            notBefore: v.notBefore,
            notAfter: v.notAfter,
            daysLeft: status.daysLeft,
            health: status.status,
          }
        } catch (e) {
          return { ...s, error: e?.message || String(e) }
        }
      }))
      rows.push(...fetched)
    }
    // Sort by daysLeft ascending; nulls/errors last.
    rows.sort((a, b) => {
      if (a.daysLeft == null && b.daysLeft == null) return 0
      if (a.daysLeft == null) return 1
      if (b.daysLeft == null) return -1
      return a.daysLeft - b.daysLeft
    })
    tlsRows.value = rows
  } catch (e) {
    tlsError.value = e?.message || String(e)
  } finally {
    tlsLoading.value = false
  }
}

// Re-run TLS scan whenever the user enters that tab.
watch(activeTab, (t) => {
  if (t === 'tls') loadTLSHealth()
})

function fmtDate(d) {
  if (!(d instanceof Date) || Number.isNaN(d.getTime())) return '—'
  return d.toISOString().slice(0, 10)
}

// --- Misc ----------------------------------------------------------

async function copyToClipboard(text) {
  try { await navigator.clipboard.writeText(text) } catch { /* best effort */ }
}
</script>

<template>
  <div class="secrets-view">
    <div class="header">
      <div class="header-row">
        <div>
          <div class="title">Secrets</div>
          <div class="subtitle">Sensitive data stored as Kubernetes Secret resources</div>
        </div>
        <button class="refresh-btn" @click="refresh(true)" :disabled="loading">
          {{ loading ? 'Loading…' : '↻ Refresh' }}
        </button>
      </div>
    </div>

    <SectionTabs
      :tabs="SECRETS_TABS"
      :active-tab="activeTab"
      @update:active-tab="activeTab = $event"
    />

    <!-- ====================== BROWSE TAB ====================== -->
    <div v-if="activeTab === 'browse'" class="tab-pane">
      <div class="toolbar">
        <input
          v-model="search"
          class="search-input"
          type="text"
          placeholder="Search secrets by name, namespace, or type…"
          data-testid="secrets-search"
        />
        <label class="quick-pick">
          <span class="quick-pick-label">Group by</span>
          <select v-model="groupBy" class="quick-select" data-testid="secrets-group-by">
            <option value="namespace">Namespace</option>
            <option value="type">Type</option>
            <option value="none">None</option>
          </select>
        </label>
      </div>

      <div v-if="loading && !secrets.length" class="state-box">Loading secrets…</div>
      <div v-else-if="error" class="state-box state-error">{{ error }}</div>
      <div v-else-if="!filteredSecrets.length" class="state-box">
        {{ search ? `No secrets match "${search}".` : 'No secrets found in this cluster.' }}
      </div>

      <div v-else class="groups">
        <div
          v-for="group in grouped"
          :key="group.key"
          class="group"
          :data-testid="`secrets-group-${group.key}`"
        >
          <div
            class="group-header"
            @click="toggleGroup(group.key)"
            :data-collapsed="isGroupCollapsed(group.key)"
          >
            <span class="group-caret" :class="{ open: !isGroupCollapsed(group.key) }">▶</span>
            <span class="group-label">{{ group.label }}</span>
          </div>
          <div v-if="!isGroupCollapsed(group.key)" class="group-body">
            <template v-for="s in group.items" :key="secretKey(s)">
              <div
                class="secret-row"
                :data-testid="`secret-row-${s.namespace}-${s.name}`"
                @click="toggleExpand(s)"
              >
                <div class="col-name">
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
                  {{ s.name }}
                </div>
                <div class="col-type"><span class="type-pill">{{ s.type }}</span></div>
                <div class="col-data font-mono">{{ s.data }} key{{ s.data === '1' ? '' : 's' }}</div>
                <div class="col-age font-mono">{{ s.age }}</div>
              </div>
              <div
                v-if="expandedSecret === secretKey(s)"
                class="secret-expanded"
                :data-testid="`secret-expanded-${s.namespace}-${s.name}`"
              >
                <div v-if="detailLoading" class="state-box">Loading detail…</div>
                <div v-else-if="!detail || !detail.data || !Object.keys(detail.data).length" class="state-box">
                  No data keys in this secret.
                </div>
                <div v-else class="data-grid">
                  <div
                    v-for="(rawValue, dataKey) in detail.data"
                    :key="dataKey"
                    class="data-row"
                  >
                    <div class="data-row-head">
                      <span class="data-key font-mono">{{ dataKey }}</span>
                      <div class="data-actions">
                        <button
                          class="data-btn"
                          :class="{ active: isRevealed(s, dataKey) }"
                          :data-testid="`reveal-${s.namespace}-${s.name}-${dataKey}`"
                          @click="toggleReveal(s, dataKey)"
                        >
                          {{ isRevealed(s, dataKey) ? 'Hide' : 'Reveal' }}
                        </button>
                        <button
                          v-if="isRevealed(s, dataKey)"
                          class="data-btn"
                          @click="copyToClipboard(decodedValue(s, dataKey, rawValue))"
                          title="Copy decoded value"
                        >Copy</button>
                      </div>
                    </div>
                    <pre
                      v-if="!isRevealed(s, dataKey)"
                      class="data-val obfuscated font-mono"
                    >•••••••• ({{ String(rawValue).length }} chars b64)</pre>
                    <textarea
                      v-else
                      class="data-val font-mono"
                      :value="decodedValue(s, dataKey, rawValue)"
                      @input="onDraftInput(s, dataKey, $event.target.value)"
                      spellcheck="false"
                      rows="4"
                      :data-testid="`value-${s.namespace}-${s.name}-${dataKey}`"
                    ></textarea>
                    <div v-if="isRevealed(s, dataKey)" class="data-hint">
                      Auto-decoded from Base64. Edits stay local until
                      you save (write-back wiring lands in a follow-up).
                    </div>
                  </div>
                </div>
              </div>
            </template>
          </div>
        </div>
      </div>
    </div>

    <!-- ====================== CLOUD TAB ====================== -->
    <div v-else-if="activeTab === 'cloud'" class="tab-pane">
      <CloudSecretsTab />
    </div>

    <!-- ====================== TLS HEALTH TAB ====================== -->
    <div v-else-if="activeTab === 'tls'" class="tab-pane">
      <div class="toolbar">
        <div class="tls-summary">
          <span v-if="tlsLoading">Scanning certificates…</span>
          <template v-else>
            <span class="tls-stat">{{ tlsRows.length }} TLS secret{{ tlsRows.length === 1 ? '' : 's' }}</span>
            <span
              v-for="b in ['critical','warning','ok']"
              :key="b"
              class="tls-bucket"
              :data-status="b"
            >{{ tlsRows.filter(r => r.health === b).length }} {{ b }}</span>
          </template>
        </div>
        <button class="refresh-btn" @click="loadTLSHealth" :disabled="tlsLoading">
          {{ tlsLoading ? 'Scanning…' : '↻ Re-scan' }}
        </button>
      </div>
      <div v-if="tlsError" class="state-box state-error">{{ tlsError }}</div>
      <div v-else-if="!tlsLoading && !tlsRows.length" class="state-box">
        No <code>kubernetes.io/tls</code> secrets found in this cluster.
      </div>
      <div v-else class="tls-list">
        <div
          v-for="r in tlsRows"
          :key="`${r.namespace}/${r.name}`"
          class="tls-row"
          :data-health="r.health || 'unknown'"
          :data-testid="`tls-row-${r.namespace}-${r.name}`"
        >
          <div class="tls-name">
            <div class="font-mono">{{ r.namespace }}/{{ r.name }}</div>
            <div class="tls-sub">
              <span v-if="r.error" class="tls-error">{{ r.error }}</span>
              <template v-else>
                valid {{ fmtDate(r.notBefore) }} → {{ fmtDate(r.notAfter) }}
              </template>
            </div>
          </div>
          <div class="tls-days" v-if="r.daysLeft != null">
            <div class="tls-days-num" :data-status="r.health">
              {{ r.daysLeft < 0 ? `expired ${-r.daysLeft}d ago` : `${r.daysLeft}d left` }}
            </div>
          </div>
          <div v-else class="tls-days">
            <div class="tls-days-num" data-status="unknown">—</div>
          </div>
        </div>
      </div>
    </div>

    <!-- ====================== PLACEHOLDER TABS ====================== -->
    <!-- These three tabs ship with the IA in place so the discovery
         path is set. The data each one needs lands in a follow-up. -->
    <div v-else-if="activeTab === 'orphaned'" class="tab-pane">
      <div class="coming-soon">
        <div class="cs-title">Orphaned Secret Detector</div>
        <p>
          Scans for Kubernetes Secrets that aren't referenced by any
          live Pod, Deployment, StatefulSet, DaemonSet, Job, or
          ServiceAccount — so they can be safely cleaned up.
        </p>
        <p class="cs-note">
          Wiring: needs a backend call that joins Secret references
          across workload specs + ServiceAccount image-pull-secrets.
          The frontend table is ready to receive it.
        </p>
      </div>
    </div>

    <div v-else-if="activeTab === 'gitops'" class="tab-pane">
      <div class="coming-soon">
        <div class="cs-title">GitOps Encryption Hub</div>
        <p>
          One-click sealing for Bitnami SealedSecrets, plus a SOPS
          encrypt/decrypt panel for cloud KMS-backed files.
        </p>
        <ul class="cs-list">
          <li>Paste a plaintext <code>Secret</code> YAML → output a
              <code>SealedSecret</code> using the cluster's controller key.</li>
          <li>Drag a <code>.sops.yaml</code> file → encrypt/decrypt
              against AWS / GCP / Azure KMS without leaving the app.</li>
        </ul>
        <p class="cs-note">
          Wiring: needs <code>kubeseal</code> + <code>sops</code>
          binaries detected by the existing Setup &amp; Tools panel,
          then a thin Wails binding around each.
        </p>
      </div>
    </div>

    <div v-else-if="activeTab === 'access'" class="tab-pane">
      <div class="coming-soon">
        <div class="cs-title">Access (RBAC) Lens</div>
        <p>
          For any secret, list every ServiceAccount, User, and Group
          that has <code>get</code> or <code>list</code> permission
          on it — answering "who can read this?" without leaving the
          view.
        </p>
        <p class="cs-note">
          Wiring: backend RoleBinding + ClusterRoleBinding scan, or a
          per-subject <code>SelfSubjectAccessReview</code> sweep.
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.secrets-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 0;
  flex: 1;
  box-sizing: border-box;
}
.header-row { display: flex; justify-content: space-between; align-items: flex-start; }
.title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.subtitle { font-size: 13px; color: #8b8f96; }
.refresh-btn {
  background: rgba(255,255,255,0.06);
  border: 1px solid rgba(255,255,255,0.1);
  color: #b0b4ba;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  cursor: pointer;
}
.refresh-btn:hover { background: rgba(255,255,255,0.1); color: #fff; }
.refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.tab-pane {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
}

.toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
}
.search-input {
  flex: 1;
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 7px 12px;
  border-radius: 6px;
  font-size: 13px;
  font-family: inherit;
}
.search-input:focus { outline: none; border-color: var(--accent2); }
.quick-pick { display: inline-flex; align-items: center; gap: 6px; }
.quick-pick-label {
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text3);
}
.quick-select {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text);
  font: inherit;
  font-size: 12.5px;
  padding: 5px 8px;
  border-radius: 4px;
  cursor: pointer;
}

.state-box {
  background: rgba(255,255,255,0.03);
  border: 1px solid rgba(255,255,255,0.08);
  padding: 16px;
  border-radius: 6px;
  font-size: 13px;
  color: #8b8f96;
}
.state-error { color: #f15c5c; border-color: rgba(241,92,92,0.3); }

.groups {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.group {
  background: #1e2023;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 8px;
  overflow: hidden;
}
.group-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  background: rgba(255,255,255,0.03);
  cursor: pointer;
  border-bottom: 1px solid rgba(255,255,255,0.06);
  font-size: 12.5px;
  color: var(--text);
}
.group-header[data-collapsed="true"] { border-bottom: none; }
.group-header:hover { background: rgba(255,255,255,0.05); }
.group-caret {
  display: inline-block;
  font-size: 9px;
  color: var(--text3);
  transition: transform 0.15s;
}
.group-caret.open { transform: rotate(90deg); }
.group-body { display: flex; flex-direction: column; }

.secret-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 80px;
  gap: 12px;
  align-items: center;
  padding: 10px 14px;
  border-top: 1px solid rgba(255,255,255,0.04);
  cursor: pointer;
}
.secret-row:hover { background: rgba(255,255,255,0.03); }
.col-name { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--text); }
.type-pill {
  font-size: 10.5px;
  padding: 2px 8px;
  border-radius: 10px;
  background: rgba(79,142,247,0.12);
  color: var(--accent2);
  font-family: var(--mono);
}
.col-data, .col-age { font-size: 11.5px; color: #8b8f96; }

.secret-expanded {
  background: #15171a;
  padding: 14px;
  border-top: 1px solid rgba(255,255,255,0.04);
  font-size: 12px;
}
.data-grid { display: flex; flex-direction: column; gap: 10px; }
.data-row {
  background: #0d0d0d;
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 4px;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.data-row-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.data-key { color: #3794ff; font-weight: 600; font-size: 12px; word-break: break-all; }
.data-actions { display: flex; gap: 6px; flex-shrink: 0; }
.data-btn {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text2);
  padding: 3px 10px;
  border-radius: 4px;
  font: inherit;
  font-size: 11px;
  cursor: pointer;
}
.data-btn:hover { background: var(--bg4); color: var(--text); }
.data-btn.active {
  background: rgba(79,142,247,0.16);
  color: var(--accent2);
  border-color: rgba(79,142,247,0.3);
}
.data-val {
  margin: 0;
  width: 100%;
  background: #050507;
  border: 1px solid rgba(255,255,255,0.04);
  color: #e8eaec;
  font-size: 11.5px;
  padding: 8px;
  border-radius: 4px;
  box-sizing: border-box;
  resize: vertical;
  font-family: var(--mono);
  white-space: pre-wrap;
  word-break: break-all;
}
.data-val.obfuscated { color: #6b7078; }
.data-hint {
  font-size: 10.5px;
  color: var(--text3);
}

/* --- TLS Health tab --- */
.tls-summary {
  flex: 1;
  display: flex;
  gap: 12px;
  align-items: center;
  font-size: 12px;
  color: var(--text2);
  flex-wrap: wrap;
}
.tls-stat { color: var(--text); font-weight: 500; }
.tls-bucket {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 10px;
}
.tls-bucket[data-status="critical"] { background: rgba(241,92,92,0.16); color: #f15c5c; }
.tls-bucket[data-status="warning"] { background: rgba(245,166,35,0.16); color: #f5a623; }
.tls-bucket[data-status="ok"] { background: rgba(62,207,142,0.14); color: #3ecf8e; }
.tls-list { display: flex; flex-direction: column; gap: 6px; }
.tls-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 14px;
  align-items: center;
  background: #1e2023;
  border: 1px solid rgba(255,255,255,0.08);
  border-left-width: 3px;
  border-left-color: rgba(255,255,255,0.08);
  border-radius: 6px;
  padding: 10px 14px;
}
.tls-row[data-health="critical"] { border-left-color: #f15c5c; }
.tls-row[data-health="warning"] { border-left-color: #f5a623; }
.tls-row[data-health="ok"] { border-left-color: #3ecf8e; }
.tls-row[data-health="expired"] { border-left-color: #f15c5c; background: rgba(241,92,92,0.06); }
.tls-name { display: flex; flex-direction: column; gap: 2px; }
.tls-sub { font-size: 11px; color: #8b8f96; }
.tls-error { color: #f5a623; }
.tls-days-num { font-family: var(--mono); font-size: 12px; color: var(--text2); white-space: nowrap; }
.tls-days-num[data-status="critical"] { color: #f15c5c; font-weight: 600; }
.tls-days-num[data-status="warning"] { color: #f5a623; }
.tls-days-num[data-status="ok"] { color: #3ecf8e; }
.tls-days-num[data-status="expired"] { color: #f15c5c; font-weight: 700; }

/* --- Coming-soon tab content --- */
.coming-soon {
  background: #1e2023;
  border: 1px dashed rgba(167,139,250,0.3);
  border-radius: 8px;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-width: 680px;
}
.cs-title {
  font-size: 14px;
  font-weight: 600;
  color: #c4b3fd;
}
.coming-soon p { margin: 0; font-size: 13px; color: var(--text2); line-height: 1.5; }
.cs-list { margin: 0; padding-left: 20px; font-size: 12.5px; color: var(--text2); }
.cs-list li { margin: 4px 0; }
.cs-note { font-size: 11.5px; color: var(--text3); font-style: italic; }
.coming-soon code {
  background: rgba(255,255,255,0.06);
  padding: 1px 5px;
  border-radius: 3px;
  font-family: var(--mono);
  font-size: 11px;
}
</style>
