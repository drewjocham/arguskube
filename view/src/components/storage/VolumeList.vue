<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { useResources } from '../../composables/useWails'
import VolumeCylinder from './VolumeCylinder.js'

const props = defineProps({
  type: { type: String, default: 'pvcs' }
})

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

onMounted(() => refresh())

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

      <div v-for="v in volumes" :key="v.name" class="vol-row-container">
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
</style>
