<script setup>
import { ref, computed, onMounted } from 'vue'
import { callGo } from '../../composables/useBridge'

// Cloud Secrets — Phase 1: identity status + list + reveal for AWS
// and GCP. Login flow and service-account impersonation land in
// follow-up PRs; this tab surfaces *ambient* credentials so an SRE
// already logged in via aws/gcloud CLIs sees their secrets here.

const PROVIDERS = [
  { id: 'aws', label: 'AWS' },
  { id: 'gcp', label: 'GCP' },
  { id: 'vault', label: 'HashiCorp Vault' },
  { id: 'azure', label: 'Azure Key Vault' },
]

const activeProvider = ref('aws')

const identities = ref([])     // [{ provider, authenticated, subject, account, source, expiresAt, expired, error }]
const identitiesLoading = ref(false)
const identitiesError = ref('')

const secrets = ref([])
const secretsLoading = ref(false)
const secretsError = ref('')
const region = ref('')          // AWS region override
const project = ref('')         // GCP project override
const vaultMount = ref('secret') // HashiCorp Vault KV mount (default "secret")
const vaultPath = ref('')       // optional sub-path within the mount
const azureVaultURL = ref('')   // full URL: https://<name>.vault.azure.net

// Reveal/draft state — keyed by `${provider}/${name}` so reveals are
// scoped per value the same way kube-secrets are.
const revealed = ref(new Set())
const revealedValues = ref({})  // `${provider}/${name}` -> plaintext
const revealError = ref({})     // per-key error

// ── identities ───────────────────────────────────────────────────

async function loadIdentities() {
  identitiesLoading.value = true
  identitiesError.value = ''
  try {
    const out = await callGo('CloudIdentities')
    identities.value = Array.isArray(out) ? out : []
  } catch (e) {
    identitiesError.value = e?.message || String(e)
  } finally {
    identitiesLoading.value = false
  }
}

const activeIdentity = computed(() =>
  identities.value.find((i) => i.provider === activeProvider.value) || null,
)

function statusBadge(id) {
  if (!id) return { label: 'unknown', tone: 'neutral' }
  if (id.error) return { label: 'not logged in', tone: 'error' }
  if (id.expired) return { label: 'expired', tone: 'warn' }
  if (id.authenticated) return { label: 'logged in', tone: 'ok' }
  return { label: 'unknown', tone: 'neutral' }
}

function fmtExpiry(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  const now = Date.now()
  const diff = d.getTime() - now
  const abs = Math.abs(diff)
  const hours = Math.floor(abs / 3_600_000)
  const mins = Math.floor((abs % 3_600_000) / 60_000)
  const human = hours > 0 ? `${hours}h ${mins}m` : `${mins}m`
  return diff < 0 ? `expired ${human} ago` : `in ${human}`
}

// ── secret listing ───────────────────────────────────────────────

async function loadSecrets() {
  secretsLoading.value = true
  secretsError.value = ''
  secrets.value = []
  try {
    let opts
    switch (activeProvider.value) {
      case 'aws':
        opts = { region: region.value || '' }
        break
      case 'gcp':
        opts = { project: project.value || '' }
        break
      case 'vault':
        opts = { vaultMount: vaultMount.value || 'secret', vaultPath: vaultPath.value || '' }
        break
      case 'azure':
        opts = { azureVaultUrl: azureVaultURL.value || '' }
        break
      default:
        opts = {}
    }
    const out = await callGo('CloudListSecrets', activeProvider.value, opts)
    secrets.value = Array.isArray(out) ? out : []
  } catch (e) {
    secretsError.value = e?.message || String(e)
  } finally {
    secretsLoading.value = false
  }
}

function rowKey(s) { return `${activeProvider.value}/${s.name}` }

function isRevealed(s) { return revealed.value.has(rowKey(s)) }

async function toggleReveal(s) {
  const k = rowKey(s)
  if (revealed.value.has(k)) {
    const next = new Set(revealed.value)
    next.delete(k)
    revealed.value = next
    const nv = { ...revealedValues.value }
    delete nv[k]
    revealedValues.value = nv
    return
  }
  revealError.value = { ...revealError.value, [k]: '' }
  try {
    const out = await callGo('CloudRevealSecret', activeProvider.value, s.name)
    const value = out?.isBinary
      ? `(binary, ${out?.value?.length || 0} chars base64)`
      : (out?.value ?? '')
    revealedValues.value = { ...revealedValues.value, [k]: value }
    const next = new Set(revealed.value)
    next.add(k)
    revealed.value = next
  } catch (e) {
    revealError.value = { ...revealError.value, [k]: e?.message || String(e) }
  }
}

async function copyToClipboard(text) {
  try { await navigator.clipboard.writeText(text) } catch { /* best effort */ }
}

function fmtDate(iso) {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toISOString().slice(0, 10)
}

// ── lifecycle ────────────────────────────────────────────────────

onMounted(() => {
  loadIdentities()
})

function switchProvider(id) {
  activeProvider.value = id
  // Reset per-tab state on provider switch — leaves identities alone.
  secrets.value = []
  secretsError.value = ''
  revealed.value = new Set()
  revealedValues.value = {}
  revealError.value = {}
  // Provider-specific inputs reset on switch so the previous tab's
  // value doesn't accidentally apply to the next call.
  if (id !== 'aws') region.value = ''
  if (id !== 'gcp') project.value = ''
  if (id !== 'vault') vaultPath.value = ''
  if (id !== 'azure') azureVaultURL.value = ''
}
</script>

<template>
  <div class="cloud-secrets">
    <!-- ── Provider sub-tabs ──────────────────────────────── -->
    <div class="provider-tabs" role="tablist">
      <button
        v-for="p in PROVIDERS"
        :key="p.id"
        type="button"
        role="tab"
        class="provider-tab"
        :class="{ active: activeProvider === p.id }"
        :aria-selected="activeProvider === p.id"
        :data-testid="`cloud-tab-${p.id}`"
        @click="switchProvider(p.id)"
      >{{ p.label }}</button>
      <button
        type="button"
        class="provider-refresh"
        :disabled="identitiesLoading"
        @click="loadIdentities"
      >{{ identitiesLoading ? '…' : '↻ Re-check identities' }}</button>
    </div>

    <output v-if="identitiesError" class="state-box state-error">
      {{ identitiesError }}
    </output>

    <!-- ── Identity card ─────────────────────────────────────── -->
    <div class="identity-card" :data-tone="statusBadge(activeIdentity).tone">
      <div class="ic-header">
        <span class="ic-provider">{{ activeProvider.toUpperCase() }}</span>
        <span class="ic-badge" :data-tone="statusBadge(activeIdentity).tone">
          {{ statusBadge(activeIdentity).label }}
        </span>
      </div>
      <div v-if="activeIdentity" class="ic-body">
        <div class="ic-row"><span class="ic-key">Subject</span><span class="ic-val font-mono">{{ activeIdentity.subject || '—' }}</span></div>
        <div class="ic-row"><span class="ic-key">Account</span><span class="ic-val font-mono">{{ activeIdentity.account || '—' }}</span></div>
        <div class="ic-row"><span class="ic-key">Source</span><span class="ic-val">{{ activeIdentity.source || '—' }}</span></div>
        <div class="ic-row"><span class="ic-key">Token</span><span class="ic-val">{{ fmtExpiry(activeIdentity.expiresAt) }}</span></div>
        <div v-if="activeIdentity.error" class="ic-hint">{{ activeIdentity.error }}</div>
        <div v-else class="ic-hint">
          Phase 1 ships with ambient credentials only. A login button +
          service-account impersonation land in a follow-up.
        </div>
      </div>
      <div v-else-if="identitiesLoading" class="ic-body">Checking…</div>
    </div>

    <!-- ── Secret list controls ─────────────────────────────── -->
    <div class="list-toolbar">
      <input
        v-if="activeProvider === 'aws'"
        v-model="region"
        class="search-input"
        placeholder="AWS region (blank = default profile)"
        data-testid="cloud-aws-region"
      />
      <input
        v-else-if="activeProvider === 'gcp'"
        v-model="project"
        class="search-input"
        placeholder="GCP project ID (blank = ADC default)"
        data-testid="cloud-gcp-project"
      />
      <template v-else-if="activeProvider === 'vault'">
        <input
          v-model="vaultMount"
          class="search-input narrow"
          placeholder="KV mount (e.g. secret)"
          data-testid="cloud-vault-mount"
        />
        <input
          v-model="vaultPath"
          class="search-input"
          placeholder="Path within mount (optional)"
          data-testid="cloud-vault-path"
        />
      </template>
      <input
        v-else-if="activeProvider === 'azure'"
        v-model="azureVaultURL"
        class="search-input"
        placeholder="https://<your-vault>.vault.azure.net"
        data-testid="cloud-azure-vault-url"
      />
      <button
        class="data-btn primary"
        :disabled="secretsLoading || (activeIdentity && activeIdentity.error)"
        @click="loadSecrets"
        data-testid="cloud-list-secrets"
      >
        {{ secretsLoading ? 'Loading…' : 'List secrets' }}
      </button>
    </div>

    <output v-if="secretsError" class="state-box state-error">
      {{ secretsError }}
    </output>

    <!-- ── Secret rows ───────────────────────────────────────── -->
    <div v-if="secrets.length" class="data-grid">
      <div
        v-for="s in secrets"
        :key="rowKey(s)"
        class="data-row"
        :data-testid="`cloud-secret-${s.name}`"
      >
        <div class="data-row-head">
          <div class="secret-meta">
            <span class="data-key font-mono">{{ s.displayName }}</span>
            <span v-if="s.region" class="meta-pill">{{ s.region }}</span>
            <span v-if="s.updated" class="meta-pill subtle">updated {{ fmtDate(s.updated) }}</span>
          </div>
          <div class="data-actions">
            <button
              class="data-btn"
              :class="{ active: isRevealed(s) }"
              :data-testid="`cloud-reveal-${s.name}`"
              @click="toggleReveal(s)"
            >
              {{ isRevealed(s) ? 'Hide' : 'Reveal' }}
            </button>
            <button
              v-if="isRevealed(s)"
              class="data-btn"
              @click="copyToClipboard(revealedValues[rowKey(s)])"
            >Copy</button>
          </div>
        </div>
        <div v-if="s.description" class="data-hint">{{ s.description }}</div>
        <pre v-if="!isRevealed(s) && !revealError[rowKey(s)]" class="data-val obfuscated font-mono">•••••••• (click Reveal)</pre>
        <pre v-else-if="revealError[rowKey(s)]" class="data-val state-error font-mono">{{ revealError[rowKey(s)] }}</pre>
        <pre v-else class="data-val font-mono">{{ revealedValues[rowKey(s)] }}</pre>
      </div>
    </div>
    <div v-else-if="!secretsLoading && !secretsError" class="state-box">
      <template v-if="activeProvider === 'aws'">Click <b>List secrets</b> to fetch from <b>AWS Secrets Manager</b>.</template>
      <template v-else-if="activeProvider === 'gcp'">Click <b>List secrets</b> to fetch from <b>Google Secret Manager</b>.</template>
      <template v-else-if="activeProvider === 'vault'">Pick a KV mount (and optional path), then click <b>List secrets</b>.</template>
      <template v-else-if="activeProvider === 'azure'">Enter your Key Vault URL, then click <b>List secrets</b>.</template>
    </div>
  </div>
</template>

<style scoped>
.cloud-secrets {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.provider-tabs {
  display: flex;
  gap: 6px;
  align-items: center;
  border-bottom: 1px solid rgba(255,255,255,0.08);
  padding-bottom: 8px;
}
.provider-tab {
  background: transparent;
  border: 1px solid rgba(255,255,255,0.08);
  color: var(--text2);
  padding: 5px 14px;
  font-size: 12px;
  border-radius: 6px;
  cursor: pointer;
}
.provider-tab:hover { color: var(--text); }
.provider-tab.active {
  background: rgba(167,139,250,0.16);
  border-color: rgba(167,139,250,0.4);
  color: var(--text);
}
.provider-refresh {
  margin-left: auto;
  background: transparent;
  border: 1px solid rgba(255,255,255,0.08);
  color: var(--text3);
  padding: 4px 10px;
  font-size: 11px;
  border-radius: 5px;
  cursor: pointer;
}
.provider-refresh:hover:not(:disabled) { color: var(--text); }
.provider-refresh:disabled { opacity: 0.5; cursor: not-allowed; }

.identity-card {
  background: #1e2023;
  border: 1px solid rgba(255,255,255,0.08);
  border-left: 3px solid var(--border);
  border-radius: 6px;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.identity-card[data-tone="ok"]    { border-left-color: #3ecf8e; }
.identity-card[data-tone="warn"]  { border-left-color: #f5a623; }
.identity-card[data-tone="error"] { border-left-color: #f15c5c; }
.ic-header { display: flex; align-items: center; justify-content: space-between; }
.ic-provider { font-size: 12.5px; font-weight: 600; color: var(--text); letter-spacing: 0.05em; }
.ic-badge {
  font-size: 10.5px;
  padding: 2px 8px;
  border-radius: 10px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
/* Badge foregrounds bumped to lighter tints — same hue family, but
   high enough contrast against the translucent tinted backgrounds to
   clear WCAG AA. The deeper saturations (#3ecf8e / #f5a623 / #f15c5c)
   sit right at the threshold against same-hue overlays. */
.ic-badge[data-tone="ok"]    { background: rgba(62,207,142,0.14); color: #6ee7b7; }
.ic-badge[data-tone="warn"]  { background: rgba(245,166,35,0.16); color: #fcd34d; }
.ic-badge[data-tone="error"] { background: rgba(241,92,92,0.16); color: #fca5a5; }
.ic-badge[data-tone="neutral"] { background: rgba(255,255,255,0.06); color: var(--text3); }
.ic-body { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: var(--text2); }
.ic-row { display: grid; grid-template-columns: 90px 1fr; gap: 8px; align-items: baseline; }
.ic-key { color: var(--text3); font-size: 10.5px; text-transform: uppercase; letter-spacing: 0.05em; }
.ic-val { color: var(--text); word-break: break-all; }
.ic-hint { color: var(--text3); font-size: 11px; margin-top: 4px; }

.list-toolbar {
  display: flex;
  gap: 10px;
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
.search-input.narrow { flex: 0 0 160px; }

.data-btn {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text2);
  padding: 5px 12px;
  border-radius: 5px;
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.data-btn.primary {
  background: rgba(167,139,250,0.18);
  border-color: rgba(167,139,250,0.45);
  color: #c4b3fd; /* lighter purple for WCAG AA on the tinted bg */
}
.data-btn:hover:not(:disabled) { background: var(--bg4); color: var(--text); }
.data-btn.primary:hover:not(:disabled) { background: rgba(167,139,250,0.25); }
.data-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.data-btn.active {
  background: rgba(79,142,247,0.16);
  color: var(--accent2);
  border-color: rgba(79,142,247,0.3);
}

.state-box {
  background: rgba(255,255,255,0.03);
  border: 1px solid rgba(255,255,255,0.08);
  padding: 14px;
  border-radius: 6px;
  font-size: 13px;
  color: var(--text2, #b0b4ba); /* brighter than #8b8f96 to clear AA */
  display: block;
}
.state-error { color: #f15c5c; border-color: rgba(241,92,92,0.3); }

.data-grid { display: flex; flex-direction: column; gap: 10px; }
.data-row {
  background: #0d0d0d;
  border: 1px solid rgba(255,255,255,0.06);
  border-radius: 5px;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.data-row-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; }
.secret-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; flex: 1; min-width: 0; }
.data-key { color: #3794ff; font-weight: 600; font-size: 12px; word-break: break-all; }
.meta-pill {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 8px;
  background: rgba(79,142,247,0.1);
  color: var(--accent2);
  font-family: var(--mono);
}
.meta-pill.subtle { background: rgba(255,255,255,0.05); color: var(--text3); }
.data-actions { display: flex; gap: 6px; flex-shrink: 0; }

.data-val {
  margin: 0;
  background: #050507;
  border: 1px solid rgba(255,255,255,0.04);
  color: #e8eaec;
  font-size: 11.5px;
  padding: 8px;
  border-radius: 4px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: var(--mono);
}
.data-val.obfuscated { color: #6b7078; }
.data-hint { font-size: 11px; color: var(--text3); }
</style>
