<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useResources, callGo } from '../../composables/useWails'
import VolumeCylinder from './VolumeCylinder.js'
import VolumeAlertModal from './VolumeAlertModal.vue'
import { useVolumeAlertsStore } from '../../stores/volumeAlerts'
import { useNotificationChannelsStore } from '../../stores/notificationChannels'
import { useAppNavStore } from '../../stores/appNav'

const props = defineProps({
  type: { type: String, default: 'pvcs' }
})

// Per-volume alert configs (localStorage). Volume identity is keyed by
// (scope, namespace, name); see stores/volumeAlerts.js.
const alertsStore = useVolumeAlertsStore()
const channels = useNotificationChannelsStore()
const appNav = useAppNavStore()

// Modal state.
const alertModalOpen = ref(false)
const alertModalTarget = ref(null) // { scope, namespace, name, capacity }

function openAlertModal(v) {
  alertModalTarget.value = {
    scope: isPV.value ? 'pv' : 'pvc',
    namespace: v.namespace || '',
    name: v.name,
    capacity: v.capacity,
  }
  alertModalOpen.value = true
}
function closeAlertModal() {
  alertModalOpen.value = false
  alertModalTarget.value = null
}

function alertFor(v) {
  return alertsStore.get(isPV.value ? 'pv' : 'pvc', v.namespace || '', v.name)
}

// Yellow when an alert is set but there's no notification channel that
// could deliver it. The visual cue points the user at the missing setup
// without breaking the flow.
function alertNeedsChannel(v) {
  return Boolean(alertFor(v)) && !channels.hasAny
}

function alertSummary(v) {
  const a = alertFor(v)
  if (!a) return ''
  if (!a.enabled) return 'paused'
  return a.mode === 'pct' ? `≥ ${a.value}%` : `≥ ${formatBytes(a.value)}`
}

function formatBytes(n) {
  if (!Number.isFinite(n) || n <= 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(v < 10 ? 2 : 1)} ${units[i]}`
}

function jumpToNotificationChannels(v) {
  appNav.requestNav({
    navId: 'settings',
    anchor: 'notification-channels',
    returnTo: {
      navId: isPV.value ? 'pvs' : 'pvcs',
      anchor: 'vol-' + (v.namespace ? v.namespace + '--' : '') + v.name,
      label: v.name,
    },
  })
}

function volAnchorId(v) {
  return 'vol-' + (v.namespace ? v.namespace + '--' : '') + v.name
}

// Encrypted-sources footer: shows native Secrets, SealedSecrets, and
// ExternalSecrets in the user-selected namespace so a user browsing PVCs
// can see possible mount sources without leaving the page.
const encNamespace = ref('')
const encSources = ref([])
const encLoading = ref(false)
const encError = ref('')

const { result, detail, loading, error, detailLoading, listResources, getResourceDetail } = useResources()

const resourceKind = props.type || 'pvcs'

// PVs are cluster-scoped (no namespace, no usage estimate via storage class
// binding). PVCs are namespaced and bind to a PV. The view differentiates
// title, columns, and empty-state copy so it's obvious which one is rendered.
const isPV = computed(() => resourceKind === 'pvs')

const headerTitle = computed(() => isPV.value ? 'Persistent Volumes' : 'Volume Claims')
const headerSubtitle = computed(() =>
  isPV.value
    ? 'Cluster-scoped storage backing PersistentVolumeClaims.'
    : 'Namespaced storage requests bound to a PersistentVolume.'
)
const emptyText = computed(() =>
  isPV.value
    ? 'No PersistentVolumes found in this cluster.'
    : 'No PersistentVolumeClaims found in this cluster.'
)
const iconColor = computed(() => isPV.value ? '#a78bfa' : '#3ecf8e')

const volumes = ref([])
const volDetail = ref(null)
const expandedVol = ref(null)
const viewRef = ref(null)

function mapItems() {
  if (result.value && result.value.items && result.value.items.length > 0) {
    volumes.value = result.value.items.map(item => ({
      name: item.name,
      namespace: item.namespace,
      status: item.status || 'Pending',
      statusColor: item.statusColor,
      capacity: item.fields?.capacity || '—',
      capacityBytes: parseBytes(item.fields?.capacity),
      accessModes: item.fields?.access_modes || '—',
      storageClass: item.fields?.storage_class || '—',
      age: item.age || '—'
    }))
  } else {
    volumes.value = []
  }
}

function parseBytes(str) {
  if (!str || str === '—') return 0
  const num = parseFloat(str)
  if (str.includes('Ti')) return num * 1024 * 1024 * 1024 * 1024
  if (str.includes('Gi')) return num * 1024 * 1024 * 1024
  if (str.includes('Mi')) return num * 1024 * 1024
  if (str.includes('Ki')) return num * 1024
  return num
}

/** Estimate usage — Bound volumes get a deterministic pseudo-usage (35–75%). */
function usagePct(v) {
  if (v.status === 'Bound') return 0.35 + (Math.abs(hashCode(v.name) % 40) / 100)
  return 0.05
}

function hashCode(s) {
  let h = 0
  for (let i = 0; i < s.length; i++) h = ((h << 5) - h) + s.charCodeAt(i) | 0
  return Math.abs(h)
}

function cylinderColor(v) {
  const hue = 140 + (hashCode(v.name) % 30)
  return `hsl(${hue}, 65%, 50%)`
}

async function refresh(force = false) {
  await listResources(resourceKind, '', force)
  mapItems()
}

// Unique namespaces seen across the loaded PVCs; drives the namespace
// picker on the encrypted-sources footer.
const namespaceOptions = computed(() => {
  const set = new Set()
  for (const v of volumes.value) {
    if (v.namespace) set.add(v.namespace)
  }
  return Array.from(set).sort()
})

async function loadEncryptedSources() {
  if (isPV.value) return
  if (!encNamespace.value) return
  encLoading.value = true
  encError.value = ''
  try {
    const res = await callGo('ListEncryptedSecretSources', encNamespace.value)
    encSources.value = Array.isArray(res) ? res : []
  } catch (e) {
    encError.value = e?.message || String(e)
    encSources.value = []
  } finally {
    encLoading.value = false
  }
}

// When PVCs load, default the encrypted-sources picker to the first
// namespace seen and trigger an initial probe. Subsequent refreshes only
// re-probe if the namespace itself changed (avoids busy-looping).
watch(namespaceOptions, (list) => {
  if (isPV.value) return
  if (!encNamespace.value && list.length > 0) {
    encNamespace.value = list[0]
    loadEncryptedSources()
  }
}, { immediate: false })

onMounted(async () => {
  await refresh()
  // If navigation arrived via a deep-link back-pointer, scroll to the
  // originating volume + briefly highlight it so the user re-locates the
  // exact volume they were configuring.
  const pending = appNav.consumeNav()
  if (pending && pending.navId === resourceKind && pending.anchor) {
    await nextTick()
    const el = viewRef.value?.querySelector('#' + CSS.escape(pending.anchor))
    if (el && typeof el.scrollIntoView === 'function') {
      el.scrollIntoView({ block: 'center', behavior: 'smooth' })
      el.classList.add('vol-row-highlight')
      setTimeout(() => el.classList.remove('vol-row-highlight'), 2500)
    }
  }
})

async function toggleExpand(volName) {
  if (expandedVol.value === volName) {
    expandedVol.value = null
    volDetail.value = null
    return
  }

  expandedVol.value = volName
  const vol = volumes.value.find(v => v.name === volName)
  if (vol) {
    await getResourceDetail(resourceKind, vol.namespace, volName)
    if (detail.value) {
      volDetail.value = detail.value
    }
  }
  await nextTick()
  scrollExpandedIntoView()
}

function scrollExpandedIntoView() {
  const el = viewRef.value?.querySelector('.vol-expanded')
  if (el && typeof el.scrollIntoView === 'function') {
    el.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }
}
</script>

<template>
  <div class="vol-view" ref="viewRef">
    <div class="header">
      <div class="header-row">
        <div>
          <div class="title">
            {{ headerTitle }}
            <span class="scope-chip" :class="{ 'scope-cluster': isPV, 'scope-ns': !isPV }">
              {{ isPV ? 'Cluster-scoped' : 'Namespaced' }}
            </span>
          </div>
          <div class="subtitle">{{ headerSubtitle }}</div>
        </div>
        <button class="refresh-btn" @click="refresh(true)" :disabled="loading">{{ loading ? 'Loading…' : '↻ Refresh' }}</button>
      </div>
    </div>

    <div v-if="loading && !volumes.length" class="state-box">Loading volumes…</div>
    <div v-else-if="error" class="state-box state-error">{{ error }}</div>
    <div v-else-if="!volumes.length" class="state-box">{{ emptyText }}</div>

    <div v-else class="vol-list" :class="{ 'is-pv': isPV }">
      <div class="vol-header-row">
        <div class="col-name">Name</div>
        <div v-if="!isPV" class="col-ns">Namespace</div>
        <div class="col-status">Status</div>
        <div class="col-cap">Capacity</div>
        <div class="col-modes">Access Modes</div>
        <div class="col-sc">Storage Class</div>
      </div>

      <div v-for="v in volumes" :key="v.name" :id="volAnchorId(v)" class="vol-row-container">
        <div class="vol-row" @click="toggleExpand(v.name)">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :style="{ color: iconColor, marginRight: '8px' }"><ellipse cx="12" cy="5" rx="9" ry="3"></ellipse><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"></path><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path></svg>
            {{ v.name }}
          </div>
          <div v-if="!isPV" class="col-ns font-mono">{{ v.namespace || '—' }}</div>
          <div class="col-status">
            <span class="status-badge" :class="v.status.toLowerCase()">{{ v.status }}</span>
          </div>

          <div class="col-cap">
            <VolumeCylinder :pct="usagePct(v)" :color="cylinderColor(v)" :size="32" />
            <span class="cap-text font-mono">{{ v.capacity }}</span>
          </div>

          <div class="col-modes font-mono">{{ v.accessModes }}</div>
          <div class="col-sc font-mono" style="display:flex; justify-content:space-between; align-items:center;">
            {{ v.storageClass }}
            <svg class="chevron" :class="{ open: expandedVol === v.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded Volume Details -->
        <div class="vol-expanded" v-if="expandedVol === v.name">
          <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
          <div v-else-if="volDetail" class="expanded-grid">
            <div class="expanded-card vol-usage-card">
              <h4 class="card-title">Usage</h4>
              <div class="vol-usage-row">
                <VolumeCylinder :pct="usagePct(v)" :color="cylinderColor(v)" :size="72" :showPct="true" />
                <div class="vol-usage-info">
                  <div class="vol-usage-stat">
                    <span class="stat-label">Capacity</span>
                    <span class="stat-value">{{ v.capacity }}</span>
                  </div>
                  <div class="vol-usage-stat">
                    <span class="stat-label">Estimated Used</span>
                    <span class="stat-value">{{ Math.round(usagePct(v) * 100) }}%</span>
                  </div>
                  <div class="vol-usage-bar-track">
                    <div class="vol-usage-bar-fill" :style="{ width: (usagePct(v) * 100) + '%', background: cylinderColor(v) }"></div>
                  </div>
                </div>
              </div>

              <!-- Per-volume alert: "Set Alert" or "Edit Alert" plus a
                   yellow status pill when the alert exists but the user has
                   no notification channel that could deliver it. The pill's
                   tooltip ("Add a notification channel in settings") deep-
                   links to the Notification Channels area in Settings; on
                   return, the row scrolls itself back into view. -->
              <div class="vol-alert-row">
                <button
                  class="vol-alert-btn"
                  :class="{ active: alertFor(v) }"
                  @click.stop="openAlertModal(v)"
                >
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
                    <path d="M18 8a6 6 0 0 0-9.33-5"></path>
                    <path d="M6 8a6 6 0 0 1 6 -6"></path>
                    <path d="M6 8c0 7-3 9-3 9h18s-3-2-3-9"></path>
                    <path d="M13.73 21a2 2 0 0 1-3.46 0"></path>
                  </svg>{{ alertFor(v) ? 'Edit alert' : 'Set alert' }}
                  <span v-if="alertFor(v)" class="vol-alert-summary mono">{{ alertSummary(v) }}</span>
                </button>

                <span
                  v-if="alertNeedsChannel(v)"
                  class="vol-alert-warn"
                  tabindex="0"
                >
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" style="vertical-align: middle;">
                    <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                    <line x1="12" y1="9" x2="12" y2="13"></line>
                    <line x1="12" y1="17" x2="12.01" y2="17"></line>
                  </svg>
                  No notification channel
                  <span class="vol-alert-tip" role="tooltip">
                    Add a notification channel in
                    <button type="button" class="vol-alert-link" @click.stop="jumpToNotificationChannels(v)">settings</button>.
                  </span>
                </span>
              </div>
            </div>
            <div class="expanded-card">
              <h4 class="card-title">Properties</h4>
              <div class="props-grid">
                <div class="prop-row" v-for="prop in volDetail.properties" :key="prop.key">
                  <span class="prop-label">{{ prop.key }}</span>
                  <span class="prop-value font-mono">{{ prop.value }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="volDetail.labels && Object.keys(volDetail.labels).length">
              <h4 class="card-title">Labels</h4>
              <div class="labels-grid">
                <span class="label-chip" v-for="(val, k) in volDetail.labels" :key="k">{{ k }}={{ val }}</span>
              </div>
            </div>

            <div class="expanded-card" v-if="volDetail.events && volDetail.events.length">
              <h4 class="card-title">Recent Events</h4>
              <div class="events-mini">
                <div class="event-mini-row" v-for="(ev, i) in volDetail.events" :key="i" :class="ev.type?.toLowerCase()">
                  <span class="ev-type">{{ ev.type }}</span>
                  <span class="ev-reason font-mono">{{ ev.reason }}</span>
                  <span class="ev-msg">{{ ev.message }}</span>
                  <span class="ev-age font-mono">{{ ev.age }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <VolumeAlertModal
      v-if="alertModalTarget"
      :open="alertModalOpen"
      :scope="alertModalTarget.scope"
      :namespace="alertModalTarget.namespace"
      :name="alertModalTarget.name"
      :capacity="alertModalTarget.capacity"
      @close="closeAlertModal"
    />

    <!-- Possible volume-mount sources in the inspected namespace.
         Surfaces native Secrets, SealedSecrets, and ExternalSecrets so
         users browsing a namespace's PVCs can see at a glance which
         encrypted-secret pipelines are in play. -->
    <div v-if="!isPV && volumes.length" class="enc-sources-footer">
      <div class="enc-header">
        <div class="enc-title">Encrypted secret sources</div>
        <div class="enc-controls">
          <select class="enc-ns" v-model="encNamespace" @change="loadEncryptedSources">
            <option v-for="ns in namespaceOptions" :key="ns" :value="ns">{{ ns }}</option>
          </select>
          <button class="enc-refresh" @click="loadEncryptedSources" :disabled="encLoading">
            {{ encLoading ? 'Loading…' : '↻' }}
          </button>
        </div>
      </div>
      <div v-if="encError" class="enc-error">{{ encError }}</div>
      <div v-else-if="encLoading && !encSources.length" class="enc-loading">Probing {{ encNamespace }}…</div>
      <div v-else-if="!encSources.length" class="enc-empty">
        No Secret-backed sources in <span class="font-mono">{{ encNamespace }}</span>.
      </div>
      <div v-else class="enc-grid">
        <div v-for="s in encSources" :key="s.kind + '/' + s.name" class="enc-row" :data-kind="s.kind.toLowerCase()" :class="{ encrypted: s.encrypted }">
          <div class="enc-row-kind">{{ s.kind }}</div>
          <div class="enc-row-name font-mono">{{ s.name }}</div>
          <div class="enc-row-type">{{ s.type || '—' }}</div>
          <div class="enc-row-hint">{{ s.hint }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.vol-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; display: flex; align-items: center; gap: 10px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

/* Scope chip — gives the user an unmistakable signal which list they're
   looking at. PVs are cluster-scoped, PVCs are per-namespace. */
.scope-chip {
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding: 3px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.scope-chip.scope-cluster {
  background: rgba(167, 139, 250, 0.15);
  color: #a78bfa;
  border: 1px solid rgba(167, 139, 250, 0.3);
}
.scope-chip.scope-ns {
  background: rgba(62, 207, 142, 0.12);
  color: #3ecf8e;
  border: 1px solid rgba(62, 207, 142, 0.3);
}
.header-row { display: flex; justify-content: space-between; align-items: flex-start; }
.refresh-btn { background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; padding: 6px 12px; border-radius: 6px; font-size: 12px; cursor: pointer; transition: all 0.15s; }
.refresh-btn:hover { background: rgba(255,255,255,0.1); color: #fff; }
.refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.state-box { padding: 40px; text-align: center; color: #8b8f96; font-size: 13px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.state-error { color: #f05454; }

.vol-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

/* PV variant collapses the namespace column since PVs are cluster-scoped. */
.vol-list.is-pv .vol-header-row,
.vol-list.is-pv .vol-row { grid-template-columns: 2fr 100px 180px 120px 1.5fr; }

.vol-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 180px 120px 1.5fr;
  gap: 16px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
  font-weight: 600;
  color: #8b8f96;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.vol-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 180px 120px 1.5fr;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.vol-row:last-child { border-bottom: none; }
.vol-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.col-cap { display: flex; align-items: center; gap: 8px; }
.cap-text { white-space: nowrap; }
.font-mono { font-family: var(--mono); color: #b0b4ba; font-size: 12px; }

.status-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; background: rgba(255,255,255,0.05); color: #ccc; }
.status-badge.bound { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-badge.pending { background: rgba(245, 166, 35, 0.15); color: #f5a623; }


.vol-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}
.vol-row-container:last-child { border-bottom: none; }

.chevron { transition: transform 0.2s ease; color: #6b7078; }
.chevron.open { transform: rotate(180deg); }

/* Expanded Area */
.vol-expanded {
  padding: 16px;
  background: #141517;
  border-top: 1px dashed rgba(255,255,255,0.08);
}
.expanded-grid {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}
.expanded-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
  padding: 16px;
  flex: 1;
  min-width: 200px;
}
.vol-usage-card { flex: 0 0 auto; }
.card-title {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  margin: 0 0 16px 0;
}

/* Usage detail */
.vol-usage-row {
  display: flex;
  align-items: center;
  gap: 16px;
}
.vol-usage-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.vol-usage-stat {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  font-size: 12px;
}
.stat-label { color: #8b8f96; }
.stat-value { color: #e8eaec; font-weight: 500; }
.vol-usage-bar-track {
  width: 160px;
  height: 4px;
  background: rgba(255,255,255,0.08);
  border-radius: 2px;
  overflow: hidden;
}
.vol-usage-bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.3s ease;
}

/* Properties */
.props-grid { display: flex; flex-direction: column; gap: 4px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }

/* Labels */
.labels-grid { display: flex; flex-wrap: wrap; gap: 6px; }
.label-chip { background: rgba(55, 148, 255, 0.1); color: #3794ff; font-size: 11px; padding: 3px 8px; border-radius: 4px; font-family: var(--mono); }

/* Mini Events */
.events-mini { display: flex; flex-direction: column; gap: 4px; max-height: 200px; overflow-y: auto; }
.event-mini-row { display: grid; grid-template-columns: 60px 120px 1fr 50px; gap: 8px; font-size: 11px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); align-items: center; }
.event-mini-row.warning { color: #f5a623; }
.event-mini-row.normal { color: #b0b4ba; }
.ev-type { font-weight: 600; }
.ev-reason { color: #a78bfa; }
.ev-msg { color: #8b8f96; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ev-age { color: #6b7078; text-align: right; }

/* Per-volume alert button + tooltip */
.vol-alert-row {
  display: flex; align-items: center; gap: 10px; flex-wrap: wrap;
  margin-top: 12px; padding-top: 10px;
  border-top: 1px dashed rgba(255,255,255,0.06);
}
.vol-alert-btn {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 5px 10px; border-radius: 5px;
  background: rgba(79,142,247,0.08); border: 1px solid rgba(79,142,247,0.25);
  color: #4f8ef7; font-size: 11.5px; font-weight: 500; cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
}
.vol-alert-btn:hover { background: rgba(79,142,247,0.15); border-color: rgba(79,142,247,0.45); }
.vol-alert-btn.active {
  background: rgba(79,142,247,0.16); border-color: rgba(79,142,247,0.5);
  color: #6ea7fb;
}
.vol-alert-summary {
  margin-left: 6px; padding: 1px 6px; border-radius: 3px;
  background: rgba(79,142,247,0.18); color: #cbdaf6; font-size: 10.5px;
}

.vol-alert-warn {
  position: relative; display: inline-flex; align-items: center; gap: 5px;
  padding: 4px 9px; border-radius: 4px;
  background: rgba(245,166,35,0.14); border: 1px solid rgba(245,166,35,0.35);
  color: #f5a623; font-size: 11px; font-weight: 500;
  cursor: help; outline: none;
}
.vol-alert-warn:focus { box-shadow: 0 0 0 2px rgba(245,166,35,0.35); }
.vol-alert-tip {
  visibility: hidden; opacity: 0; pointer-events: none;
  position: absolute; bottom: calc(100% + 8px); left: 0;
  width: max-content; max-width: 280px; padding: 8px 10px;
  background: #1a1d23; border: 1px solid rgba(245,166,35,0.4); border-radius: 5px;
  color: #d8dde3; font-size: 11.5px; font-weight: 400; line-height: 1.45;
  white-space: normal; box-shadow: 0 4px 16px rgba(0,0,0,0.45);
  transition: opacity 0.15s;
  z-index: 5;
}
.vol-alert-warn:hover .vol-alert-tip,
.vol-alert-warn:focus .vol-alert-tip,
.vol-alert-warn:focus-within .vol-alert-tip {
  visibility: visible; opacity: 1; pointer-events: auto;
}
.vol-alert-link {
  background: transparent; border: 0; padding: 0;
  color: #4f8ef7; font: inherit; cursor: pointer; text-decoration: underline;
}
.vol-alert-link:hover { color: #6ea7fb; }

/* Briefly highlight the volume row that the user just returned to. */
.vol-row-highlight {
  animation: vol-row-flash 2.4s ease-out;
  border-radius: 6px;
}
@keyframes vol-row-flash {
  0%   { box-shadow: 0 0 0 2px rgba(79,142,247,0.0), 0 0 0 6px rgba(79,142,247,0.0); }
  18%  { box-shadow: 0 0 0 2px rgba(79,142,247,0.65), 0 0 0 6px rgba(79,142,247,0.18); }
  100% { box-shadow: 0 0 0 2px rgba(79,142,247,0.0), 0 0 0 6px rgba(79,142,247,0.0); }
}

/* Encrypted-sources footer */
.enc-sources-footer {
  margin-top: 16px;
  padding: 14px 16px;
  background: rgba(255,255,255,0.025);
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 8px;
}
.enc-header {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 10px; gap: 10px;
}
.enc-title {
  font-size: 13px; color: #fff; font-weight: 500;
  display: flex; align-items: center; gap: 8px;
}
.enc-controls { display: flex; gap: 6px; align-items: center; }
.enc-ns {
  background: var(--bg, #1e2023); color: var(--text, #fff);
  border: 1px solid rgba(255,255,255,0.12); border-radius: 4px;
  padding: 4px 8px; font-size: 11.5px; outline: none;
  font-family: var(--mono, monospace);
}
.enc-refresh {
  background: transparent; border: 1px solid rgba(255,255,255,0.12);
  color: #8b8f96; padding: 3px 8px; border-radius: 4px;
  cursor: pointer; font-size: 12px;
  transition: border-color 0.12s, color 0.12s;
}
.enc-refresh:hover:not(:disabled) { border-color: var(--accent, #4f8ef7); color: #fff; }
.enc-refresh:disabled { opacity: 0.5; cursor: not-allowed; }

.enc-error {
  font-size: 11.5px; color: #f05454;
  padding: 6px 10px; background: rgba(240,84,84,0.08);
  border: 1px solid rgba(240,84,84,0.3); border-radius: 4px;
}
.enc-loading, .enc-empty {
  font-size: 12px; color: #8b8f96; padding: 8px 0;
}

.enc-grid { display: flex; flex-direction: column; gap: 4px; }
.enc-row {
  display: grid; grid-template-columns: 110px minmax(140px, 1fr) minmax(120px, 1fr) 1fr;
  gap: 10px; align-items: center;
  padding: 6px 10px; border-radius: 5px;
  background: rgba(255,255,255,0.02); border: 1px solid rgba(255,255,255,0.06);
  font-size: 11.5px;
}
.enc-row.encrypted { border-color: rgba(167,139,250,0.4); background: rgba(167,139,250,0.05); }
.enc-row[data-kind="sealedsecret"]   { border-color: rgba(245,166,35,0.45); background: rgba(245,166,35,0.05); }
.enc-row[data-kind="externalsecret"] { border-color: rgba(62,207,142,0.4); background: rgba(62,207,142,0.04); }

.enc-row-kind {
  font-size: 9.5px; font-weight: 700; letter-spacing: 0.06em;
  text-transform: uppercase; color: #fff;
  padding: 2px 7px; border-radius: 9px;
  background: rgba(79,142,247,0.18);
  text-align: center;
}
.enc-row[data-kind="sealedsecret"]   .enc-row-kind { background: rgba(245,166,35,0.22); color: #f5a623; }
.enc-row[data-kind="externalsecret"] .enc-row-kind { background: rgba(62,207,142,0.22); color: #3ecf8e; }

.enc-row-name { color: #fff; word-break: break-all; }
.enc-row-type { color: #8b8f96; font-size: 10.5px; }
.enc-row-hint { color: #b8bcc4; font-size: 10.5px; word-break: break-word; }
</style>
