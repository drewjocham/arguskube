<script setup>
import { ref, onMounted } from 'vue'
import { callGo } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const services = ref([])
const selectedSvc = ref(null)
const diffResult = ref(null)
const loading = ref(false)
const error = ref(null)
const namespace = ref('default')
const namespaces = ref([])

onMounted(async () => {
  try {
    const nss = await callGo('ListNamespaces')
    if (nss) namespaces.value = nss
  } catch {}
  await fetchServices()
})

async function fetchServices() {
  loading.value = true
  error.value = null
  try {
    const result = await callGo('ListServiceSelectors', namespace.value)
    services.value = result || []
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

async function analyzeMatch(svcName) {
  selectedSvc.value = svcName
  loading.value = true
  error.value = null
  try {
    const result = await callGo('AnalyzeLabelMatch', namespace.value, svcName)
    diffResult.value = result
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}
</script>

<template>
  <div class="matcher-view">
    <div class="header">
      <div class="title">Label Matcher & Drift Detector</div>
      <div class="subtitle">Compare Service selectors against Pod labels to find mismatches</div>
    </div>

    <div class="controls">
      <Select v-model="namespace" :options="namespaces" @change="fetchServices" size="sm" aria-label="Namespace" />
      <button class="btn-refresh" @click="fetchServices">Refresh</button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div class="matcher-layout">
      <div class="svc-list">
        <div class="list-title">Services with Selectors</div>
        <div v-if="loading && !diffResult" class="state-box">Loading…</div>
        <div v-else-if="services.length === 0" class="state-box">No services with selectors found</div>
        <div
          v-for="s in services"
          :key="s.serviceName"
          class="svc-item"
          :class="{ active: selectedSvc === s.serviceName, 'has-issues': s.matchingPods === 0 }"
          @click="analyzeMatch(s.serviceName)"
        >
          <div class="svc-name">{{ s.serviceName }}</div>
          <div class="svc-meta">
            <span class="pod-count" :class="{ zero: s.matchingPods === 0 }">{{ s.matchingPods }} pods</span>
          </div>
        </div>
      </div>

      <div class="diff-panel" v-if="diffResult">
        <div class="diff-header">
          <span class="diff-title">{{ diffResult.serviceName }}</span>
          <span class="diff-status" :class="diffResult.hasIssues ? 'fail' : 'ok'">
            {{ diffResult.hasIssues ? 'Mismatch' : 'All matching' }}
          </span>
        </div>

        <div class="diff-section">
          <div class="diff-section-title">Service Selector</div>
          <div class="selector-grid">
            <div v-for="(v, k) in diffResult.selector" :key="k" class="selector-chip">
              <span class="sel-key">{{ k }}</span>=<span class="sel-val">{{ v }}</span>
            </div>
            <div v-if="Object.keys(diffResult.selector).length === 0" class="no-data">No selector</div>
          </div>
        </div>

        <div class="diff-section">
          <div class="diff-section-title">Matching Pods</div>
          <div v-if="diffResult.matches.length === 0" class="no-data">No pods match this selector</div>
          <div v-for="m in diffResult.matches" :key="m.podName" class="match-row">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" :style="{ color: m.complete ? '#3ecf8e' : '#f5a623' }">
              <template v-if="m.complete">
                <polyline points="20 6 9 17 4 12"></polyline>
              </template>
              <template v-else>
                <line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/>
              </template>
            </svg>
            <span class="match-pod">{{ m.podName }}</span>
            <span class="match-badge" :class="m.complete ? 'ok' : 'partial'">{{ m.complete ? 'Full Match' : 'Partial' }}</span>
          </div>
        </div>

        <div class="diff-section" v-if="diffResult.mismatches.length > 0">
          <div class="diff-section-title" style="color:#f05454">Mismatches</div>
          <div v-for="mm in diffResult.mismatches" :key="mm.podName + mm.selectorKey" class="mismatch-row">
            <div class="mm-pod">{{ mm.podName }}</div>
            <div class="mm-detail">
              <span class="mm-label">key:</span> <span class="font-mono">{{ mm.selectorKey }}</span>
              <span class="mm-label">expected:</span> <span class="mm-expected font-mono">{{ mm.selectorValue }}</span>
              <span v-if="mm.missing" class="mm-missing">(missing)</span>
              <span v-else>
                <span class="mm-label">got:</span>
                <span class="mm-actual font-mono">{{ mm.actualValue }}</span>
              </span>
            </div>
          </div>
        </div>

        <div class="diff-section" v-if="diffResult.orphanedPods && diffResult.orphanedPods.length > 0">
          <div class="diff-section-title" style="color:#f5a623">Orphaned Endpoints</div>
          <div v-for="o in diffResult.orphanedPods" :key="o.podName" class="orphan-row">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#f5a623" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>
            <span class="font-mono">{{ o.podName }}</span>
            <span class="orphan-detail">Endpoint IP no longer matches any active pod</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.matcher-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }
.controls { display: flex; gap: 8px; align-items: center; }
.btn-refresh { padding: 6px 14px; font-size: 12px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 5px; cursor: pointer; }
.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.matcher-layout { flex: 1; display: flex; gap: 16px; min-height: 0; overflow: hidden; }
.svc-list { width: 280px; flex-shrink: 0; display: flex; flex-direction: column; gap: 4px; }
.list-title { font-size: 12px; font-weight: 600; color: #8b8f96; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 4px; }
.svc-item { display: flex; align-items: center; justify-content: space-between; padding: 8px 10px; background: #1e2023; border: 1px solid rgba(255,255,255,0.06); border-radius: 5px; cursor: pointer; transition: all 0.1s; }
.svc-item:hover { border-color: rgba(167,139,250,0.3); }
.svc-item.active { border-color: #a78bfa; background: rgba(167,139,250,0.08); }
.svc-item.has-issues { border-color: rgba(240,84,84,0.3); }
.svc-name { font-size: 12.5px; color: #e8eaec; }
.svc-meta { font-size: 11px; }
.pod-count { color: #3ecf8e; }
.pod-count.zero { color: #f05454; }

.diff-panel { flex: 1; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 20px; overflow-y: auto; display: flex; flex-direction: column; gap: 16px; }
.diff-header { display: flex; align-items: center; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid rgba(255,255,255,0.06); }
.diff-title { font-size: 16px; font-weight: 500; color: #fff; }
.diff-status { font-size: 11px; font-weight: 600; padding: 3px 8px; border-radius: 4px; }
.diff-status.ok { background: rgba(62,207,142,0.15); color: #3ecf8e; }
.diff-status.fail { background: rgba(240,84,84,0.15); color: #f05454; }

.diff-section { display: flex; flex-direction: column; gap: 6px; }
.diff-section-title { font-size: 13px; font-weight: 600; color: #8b8f96; }

.selector-grid { display: flex; gap: 4px; flex-wrap: wrap; }
.selector-chip { padding: 3px 8px; background: rgba(167,139,250,0.1); border: 1px solid rgba(167,139,250,0.2); border-radius: 4px; font-size: 12px; color: #a78bfa; }
.sel-key { color: #e8eaec; }
.sel-val { color: #a78bfa; }

.match-row { display: flex; align-items: center; gap: 8px; padding: 6px 8px; background: rgba(255,255,255,0.02); border-radius: 4px; }
.match-pod { font-size: 12px; color: #e8eaec; flex: 1; }
.match-badge { font-size: 10px; padding: 2px 6px; border-radius: 3px; font-weight: 600; }
.match-badge.ok { background: rgba(62,207,142,0.15); color: #3ecf8e; }
.match-badge.partial { background: rgba(245,166,35,0.15); color: #f5a623; }

.mismatch-row { display: flex; flex-direction: column; gap: 2px; padding: 6px 8px; background: rgba(240,84,84,0.05); border: 1px solid rgba(240,84,84,0.15); border-radius: 4px; }
.mm-pod { font-size: 12px; color: #f05454; font-weight: 500; }
.mm-detail { font-size: 11px; color: #8b8f96; display: flex; gap: 4px; flex-wrap: wrap; align-items: center; }
.mm-label { color: #6b7078; }
.mm-expected { color: #3ecf8e; }
.mm-actual { color: #f05454; }
.mm-missing { color: #f05454; font-style: italic; }

.orphan-row { display: flex; align-items: center; gap: 8px; padding: 6px 8px; background: rgba(245,166,35,0.05); border: 1px solid rgba(245,166,35,0.15); border-radius: 4px; font-size: 12px; }
.orphan-detail { color: #8b8f96; font-size: 11px; }

.state-box { padding: 24px; text-align: center; color: #8b8f96; font-size: 13px; }
.no-data { color: #6b7078; font-size: 12px; font-style: italic; }
.font-mono { font-family: var(--mono); }
</style>
