<script setup>
import { ref, onMounted } from 'vue'
import { callGo } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const namespace = ref('default')
const namespaces = ref([])
const profile = ref(null)
const loading = ref(false)
const error = ref(null)

onMounted(async () => {
  try {
    const nss = await callGo('ListNamespaces')
    if (nss) namespaces.value = nss
  } catch {}
})

async function analyze() {
  loading.value = true
  error.value = null
  try {
    profile.value = await callGo('ProfileWaste', namespace.value)
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

function scoreColor(score) {
  return score === 'critical' ? '#f05454' : score === 'high' ? '#f5a623' : score === 'medium' ? '#3794ff' : '#3ecf8e'
}
</script>

<template>
  <div class="heatmap-view">
    <div class="header">
      <div class="title">Over-Provisioning Heatmap</div>
      <div class="subtitle">Compares requested vs actual resource usage to find waste</div>
    </div>

    <div class="controls">
      <Select v-model="namespace" :options="namespaces" size="sm" aria-label="Namespace" />
      <button class="btn-analyze" @click="analyze" :disabled="loading">
        {{ loading ? 'Scanning…' : 'Analyze' }}
      </button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div v-if="profile" class="results">
      <div class="score-card" :style="{ borderColor: scoreColor(profile.score) }">
        <div class="score-header">
          <span class="score-label">Waste Score</span>
          <span class="score-value" :style="{ color: scoreColor(profile.score) }">{{ profile.score.toUpperCase() }}</span>
        </div>
        <div class="score-details">
          <span class="score-item">CPU Waste: <strong>{{ profile.totalWasteCPU }}</strong></span>
          <span class="score-item">Memory Waste: <strong>{{ profile.totalWasteMem }}</strong></span>
        </div>
      </div>

      <div class="table">
        <div class="table-header">
          <div class="h-name">Workload</div>
          <div class="h-req">Requested</div>
          <div class="h-est">Estimated Use</div>
          <div class="h-waste">Waste</div>
          <div class="h-ratio">Ratio</div>
        </div>
        <div
          v-for="d in profile.deployments"
          :key="d.name"
          class="table-row"
          :class="{
            critical: d.wasteCPU.includes('CPU') && parseFloat(d.wasteCPU) > 0.5,
            warning: d.wasteCPU.includes('m') && parseInt(d.wasteCPU) > 500,
          }"
        >
          <div class="cell-name font-mono">{{ d.name }}</div>
          <div class="cell-req">{{ d.cpuRequest }} / {{ d.memoryRequest }}</div>
          <div class="cell-est">{{ d.cpuUsage || '—' }} / {{ d.memoryUsage || '—' }}</div>
          <div class="cell-waste" :style="{ color: d.wasteCPU.includes('CPU') ? '#f05454' : '#f5a623' }">
            {{ d.wasteCPU }} / {{ d.wasteMem }}
          </div>
          <div class="cell-ratio">{{ d.ratio }}</div>
        </div>
        <div v-if="profile.deployments.length === 0" class="table-empty">No waste detected in this namespace</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.heatmap-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }
.controls { display: flex; gap: 8px; align-items: center; }
.btn-analyze { padding: 6px 16px; font-size: 12px; background: rgba(167,139,250,0.15); border: 1px solid rgba(167,139,250,0.3); color: #a78bfa; border-radius: 5px; cursor: pointer; }
.btn-analyze:disabled { opacity: 0.4; cursor: not-allowed; }
.error-banner { padding: 8px 12px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.results { display: flex; flex-direction: column; gap: 16px; }
.score-card { background: #1e2023; border: 2px solid; border-radius: 8px; padding: 16px; }
.score-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.score-label { font-size: 13px; color: #8b8f96; }
.score-value { font-size: 18px; font-weight: 700; letter-spacing: 0.05em; }
.score-details { display: flex; gap: 20px; font-size: 12px; color: #b0b4ba; }
.score-details strong { color: #e8eaec; }

.table { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; overflow: hidden; }
.table-header, .table-row { display: grid; grid-template-columns: 2fr 1.5fr 1.5fr 1.5fr 0.8fr; gap: 8px; padding: 10px 14px; font-size: 12px; align-items: center; }
.table-header { background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.08); color: #8b8f96; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; font-size: 10.5px; }
.table-row { border-bottom: 1px solid rgba(255,255,255,0.04); color: #e8eaec; }
.table-row:last-child { border-bottom: none; }
.table-row.critical { background: rgba(240,84,84,0.04); border-left: 3px solid #f05454; }
.table-row.warning { background: rgba(245,166,35,0.04); border-left: 3px solid #f5a623; }
.table-empty { padding: 24px; text-align: center; color: #8b8f96; }
.font-mono { font-family: var(--mono); }
</style>
