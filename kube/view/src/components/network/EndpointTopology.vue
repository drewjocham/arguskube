<script setup>
import { ref, onMounted } from 'vue'
import { callGo } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const slices = ref([])
const zoneDist = ref(null)
const warnings = ref([])
const loading = ref(false)
const error = ref(null)
const namespace = ref('default')
const namespaces = ref([])
const selectedSvc = ref('')

onMounted(async () => {
  try {
    const nss = await callGo('ListNamespaces')
    if (nss) namespaces.value = nss
  } catch {}
  await fetchAll()
})

async function fetchAll() {
  loading.value = true
  error.value = null
  try {
    const [s, z, w] = await Promise.all([
      callGo('ListEndpointSlices', namespace.value),
      callGo('GetZoneDistribution', namespace.value, selectedSvc.value || ''),
      callGo('CheckTopologyAwareRouting', namespace.value),
    ])
    slices.value = s || []
    zoneDist.value = z
    warnings.value = w || []
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

function maxZoneColor(pct) {
  if (pct > 80) return '#f05454'
  if (pct > 60) return '#f5a623'
  return '#3ecf8e'
}
</script>

<template>
  <div class="topology-view">
    <div class="header">
      <div class="title">Endpoint Topology</div>
      <div class="subtitle">Zone distribution and traffic routing visibility for EndpointSlices</div>
    </div>

    <div class="controls">
      <Select v-model="namespace" :options="namespaces" @change="fetchAll" size="sm" aria-label="Namespace" />
      <button class="btn-refresh" @click="fetchAll">Refresh</button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div v-if="warnings.length > 0" class="warnings-section">
      <div class="warnings-title">Traffic Imbalance Warnings</div>
      <div v-for="w in warnings" :key="w.serviceName" class="warning-card">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#f05454" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
        <div class="warning-body">
          <div class="warning-svc">{{ w.serviceName }}</div>
          <div class="warning-msg">{{ w.message }}</div>
          <div class="warning-meta">{{ w.totalInZone }} / {{ w.totalEndpoints }} endpoints in one zone</div>
        </div>
      </div>
    </div>

    <div v-if="zoneDist" class="zone-section">
      <div class="section-title">Zone Distribution</div>
      <div class="zone-bar-container">
        <div class="zone-bar">
          <div
            v-for="(count, zone) in zoneDist.zoneCounts"
            :key="zone"
            class="zone-segment"
            :style="{
              width: (count / zoneDist.totalEndpoints * 100) + '%',
              background: zone === 'unknown' ? '#6b7078' : '#3794ff'
            }"
            :title="zone + ': ' + count + ' endpoints'"
          ></div>
        </div>
        <div class="zone-legend">
          <div v-for="(count, zone) in zoneDist.zoneCounts" :key="zone" class="zone-legend-item">
            <span class="zone-dot" :style="{ background: zone === 'unknown' ? '#6b7078' : '#3794ff' }"></span>
            <span class="zone-name">{{ zone }}</span>
            <span class="zone-count font-mono">{{ count }}</span>
            <span class="zone-pct font-mono">({{ (count / zoneDist.totalEndpoints * 100).toFixed(0) }}%)</span>
          </div>
        </div>
        <div class="zone-summary">
          <span class="summary-total">{{ zoneDist.totalEndpoints }} total endpoints</span>
          <span class="summary-imbalance" v-if="zoneDist.imbalanced" style="color:#f05454">
            ⚠ {{ zoneDist.maxZonePct.toFixed(0) }}% in single zone — risk of overload
          </span>
        </div>
      </div>
    </div>

    <div class="section-title">EndpointSlices</div>
    <div v-if="loading" class="state-box">Loading…</div>
    <div v-else-if="slices.length === 0" class="state-box">No EndpointSlices found</div>
    <div v-else class="slice-table">
      <div class="slice-header">
        <div class="h-name">Name</div>
        <div class="h-svc">Service</div>
        <div class="h-ready">Ready</div>
        <div class="h-total">Total</div>
        <div class="h-zones">Zone Breakdown</div>
      </div>
      <div v-for="s in slices" :key="s.name" class="slice-row">
        <div class="slice-name font-mono">{{ s.name }}</div>
        <div class="slice-svc">{{ s.byService || '—' }}</div>
        <div class="slice-ready">
          <span class="ready-count" :class="{ all: s.readyCount === s.endpoints }">{{ s.readyCount }}/{{ s.endpoints }}</span>
        </div>
        <div class="slice-total font-mono">{{ s.endpoints }}</div>
        <div class="slice-zones">
          <span v-for="(count, zone) in s.zoneCount" :key="zone" class="zone-chip">
            {{ zone }}: {{ count }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.topology-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }
.controls { display: flex; gap: 8px; align-items: center; }
.btn-refresh { padding: 6px 14px; font-size: 12px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 5px; cursor: pointer; }
.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.warnings-section { display: flex; flex-direction: column; gap: 8px; }
.warnings-title { font-size: 14px; font-weight: 600; color: #f05454; }
.warning-card { display: flex; gap: 10px; padding: 12px; background: rgba(240,84,84,0.06); border: 1px solid rgba(240,84,84,0.2); border-radius: 6px; align-items: flex-start; }
.warning-body { flex: 1; }
.warning-svc { font-size: 13px; font-weight: 500; color: #e8eaec; }
.warning-msg { font-size: 12px; color: #8b8f96; margin: 2px 0; }
.warning-meta { font-size: 11px; color: #6b7078; }

.zone-section { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 20px; }
.section-title { font-size: 14px; font-weight: 600; color: #e8eaec; margin-bottom: 12px; }
.zone-bar-container { display: flex; flex-direction: column; gap: 12px; }
.zone-bar { display: flex; height: 24px; background: rgba(255,255,255,0.04); border-radius: 6px; overflow: hidden; }
.zone-segment { transition: width 0.3s ease; }
.zone-legend { display: flex; gap: 16px; flex-wrap: wrap; }
.zone-legend-item { display: flex; align-items: center; gap: 4px; font-size: 12px; color: #b0b4ba; }
.zone-dot { width: 8px; height: 8px; border-radius: 50%; }
.zone-count { color: #e8eaec; }
.zone-pct { color: #6b7078; }
.zone-summary { display: flex; gap: 16px; font-size: 12px; padding-top: 8px; border-top: 1px solid rgba(255,255,255,0.06); }
.summary-total { color: #8b8f96; }

.slice-table { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; overflow: hidden; }
.slice-header, .slice-row { display: grid; grid-template-columns: 2fr 1.5fr 80px 60px 1fr; gap: 8px; padding: 10px 14px; font-size: 12px; align-items: center; }
.slice-header { background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.08); color: #8b8f96; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; font-size: 10.5px; }
.slice-row { border-bottom: 1px solid rgba(255,255,255,0.04); color: #e8eaec; }
.slice-row:last-child { border-bottom: none; }
.slice-name { color: #a78bfa; }
.slice-svc { color: #b0b4ba; }
.ready-count { color: #3ecf8e; }
.ready-count.all { color: #3ecf8e; }
.slice-zones { display: flex; gap: 4px; flex-wrap: wrap; }
.zone-chip { padding: 2px 6px; background: rgba(55,148,255,0.1); border: 1px solid rgba(55,148,255,0.15); border-radius: 3px; font-size: 10.5px; color: #3794ff; }

.state-box { padding: 24px; text-align: center; color: #8b8f96; font-size: 13px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.font-mono { font-family: var(--mono); }
</style>
