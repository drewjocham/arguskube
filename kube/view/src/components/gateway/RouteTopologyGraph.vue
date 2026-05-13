<script setup>
import { ref, onMounted } from 'vue'
import { callGo } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const graph = ref(null)
const loading = ref(false)
const error = ref(null)
const namespace = ref('')
const namespaces = ref([])

onMounted(async () => {
  try {
    const nss = await callGo('ListNamespaces')
    if (nss) namespaces.value = nss
  } catch {}
  await fetchGraph()
})

async function fetchGraph() {
  loading.value = true
  error.value = null
  try {
    const result = await callGo('GetRouteTopologyGraph', namespace.value)
    graph.value = result
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

function routeParents(route) {
  return route.parentRefs?.filter(p => p.kind === 'Gateway').map(p => p.name) || []
}

function routeForGateway(gwName) {
  if (!graph.value?.httpRoutes) return []
  return graph.value.httpRoutes.filter(r => routeParents(r).includes(gwName))
}
</script>

<template>
  <div class="topology-view">
    <div class="header">
      <div class="title">Route Topology</div>
      <div class="subtitle">Gateways and their attached HTTPRoutes with conflict detection</div>
    </div>

    <div class="controls">
      <Select v-model="namespace" :options="[{value:'',label:'All Namespaces'}, ...namespaces.map(ns => ({value: ns, label: ns}))]" @change="fetchGraph" size="sm" />
      <button class="btn-refresh" @click="fetchGraph">Refresh</button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div v-if="graph?.conflicts?.length > 0" class="conflicts-section">
      <div class="conflicts-title">Hostname Conflicts</div>
      <div v-for="c in graph.conflicts" :key="c.hostname + c.routeNameA + c.routeNameB" class="conflict-card">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#f5a623" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
        <div class="conflict-body">
          <div class="conflict-host">Hostname <strong>{{ c.hostname }}</strong> is claimed by multiple routes</div>
          <div class="conflict-routes">
            <span class="conflict-route">{{ c.namespaceA }}/{{ c.routeNameA }}</span>
            <span class="conflict-vs">vs</span>
            <span class="conflict-route">{{ c.namespaceB }}/{{ c.routeNameB }}</span>
          </div>
        </div>
      </div>
    </div>

    <div v-if="loading" class="state-box">Loading topology…</div>
    <div v-else-if="!graph" class="state-box">No topology data available</div>
    <template v-else>
      <div class="section-title">Gateways</div>
      <div v-if="graph.gateways?.length === 0" class="state-box">No Gateways found</div>
      <div v-else class="gateway-list">
        <div v-for="gw in graph.gateways" :key="gw.name + gw.namespace" class="gateway-card">
          <div class="gw-header">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#a78bfa" stroke-width="2"><rect x="2" y="8" width="20" height="8" rx="1"/><path d="M8 6l-4 4 4 4M16 6l4 4-4 4"/></svg>
            <span class="gw-name">{{ gw.name }}</span>
            <span class="gw-ns font-mono">{{ gw.namespace }}</span>
            <span class="gw-class font-mono">{{ gw.className }}</span>
          </div>
          <div class="gw-meta">
            <div class="meta-chip">
              <span class="chip-label">Listeners</span>
              <span class="chip-value">{{ gw.listeners }}</span>
            </div>
            <div class="meta-chip" v-if="gw.addresses?.length">
              <span class="chip-label">Addresses</span>
              <span class="chip-value font-mono">{{ gw.addresses.join(', ') }}</span>
            </div>
            <div class="meta-chip">
              <span class="chip-label">Attached Routes</span>
              <span class="chip-value">{{ gw.attachedRoutes || routeForGateway(gw.name).length }}</span>
            </div>
          </div>
          <div v-if="gw.conditions?.length" class="gw-conditions">
            <div v-for="cond in gw.conditions" :key="cond.type" class="cond-row" :class="cond.status === 'True' ? 'ok' : 'fail'">
              <span class="cond-type">{{ cond.type }}</span>
              <span class="cond-status">{{ cond.status }}</span>
              <span class="cond-reason font-mono">{{ cond.reason }}</span>
              <span class="cond-msg" v-if="cond.message">{{ cond.message }}</span>
            </div>
          </div>
          <div v-if="routeForGateway(gw.name).length > 0" class="gw-routes">
            <div class="routes-subtitle">Attached HTTPRoutes</div>
            <div v-for="route in routeForGateway(gw.name)" :key="route.name + route.namespace" class="route-chip">
              <span class="route-name">{{ route.name }}</span>
              <span class="route-ns font-mono">{{ route.namespace }}</span>
              <span class="route-hosts" v-if="route.hostnames?.length">{{ route.hostnames.join(', ') }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="section-title">HTTPRoutes</div>
      <div v-if="graph.httpRoutes?.length === 0" class="state-box">No HTTPRoutes found</div>
      <div v-else class="route-list">
        <div v-for="route in graph.httpRoutes" :key="route.name + route.namespace" class="route-card">
          <div class="route-header">
            <span class="route-name">{{ route.name }}</span>
            <span class="route-ns font-mono">{{ route.namespace }}</span>
          </div>
          <div class="route-meta">
            <div class="meta-chip" v-if="route.hostnames?.length">
              <span class="chip-label">Hostnames</span>
              <span class="chip-value">{{ route.hostnames.join(', ') }}</span>
            </div>
            <div class="meta-chip">
              <span class="chip-label">Rules</span>
              <span class="chip-value">{{ route.matches }}</span>
            </div>
            <div class="meta-chip" v-if="route.parentRefs?.length">
              <span class="chip-label">Parent</span>
              <span class="chip-value font-mono">{{ route.parentRefs.map(p => p.name).join(', ') }}</span>
            </div>
          </div>
          <div v-if="route.backendRefs?.length" class="route-backends">
            <div class="backends-subtitle">Backends</div>
            <div v-for="b in route.backendRefs" :key="b.name + b.port" class="backend-chip">
              <span class="bk-name">{{ b.name }}</span>
              <span class="bk-port font-mono">:{{ b.port }}</span>
              <span class="bk-weight font-mono" v-if="b.weight > 1">weight {{ b.weight }}</span>
            </div>
          </div>
          <div v-if="route.conditions?.length" class="gw-conditions">
            <div v-for="cond in route.conditions" :key="cond.type" class="cond-row" :class="cond.status === 'True' ? 'ok' : 'fail'">
              <span class="cond-type">{{ cond.type }}</span>
              <span class="cond-status">{{ cond.status }}</span>
              <span class="cond-reason font-mono">{{ cond.reason }}</span>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.topology-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }
.controls { display: flex; gap: 8px; align-items: center; }
.btn-refresh { padding: 6px 14px; font-size: 12px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 5px; cursor: pointer; }
.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.conflicts-section { display: flex; flex-direction: column; gap: 8px; }
.conflicts-title { font-size: 14px; font-weight: 600; color: #f5a623; }
.conflict-card { display: flex; gap: 10px; padding: 12px; background: rgba(245,166,35,0.06); border: 1px solid rgba(245,166,35,0.2); border-radius: 6px; align-items: flex-start; }
.conflict-body { flex: 1; }
.conflict-host { font-size: 13px; color: #e8eaec; margin-bottom: 4px; }
.conflict-host strong { color: #f5a623; }
.conflict-routes { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.conflict-route { padding: 2px 6px; background: rgba(245,166,35,0.1); border-radius: 3px; color: #f5a623; font-family: var(--mono); }
.conflict-vs { color: #6b7078; font-size: 10px; text-transform: uppercase; }

.section-title { font-size: 14px; font-weight: 600; color: #e8eaec; margin-bottom: 4px; }

.gateway-list { display: flex; flex-direction: column; gap: 8px; }
.gateway-card { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 16px; display: flex; flex-direction: column; gap: 10px; }
.gw-header { display: flex; align-items: center; gap: 8px; }
.gw-name { font-size: 14px; font-weight: 500; color: #e8eaec; }
.gw-ns { font-size: 11px; color: #6b7078; }
.gw-class { font-size: 11px; padding: 1px 6px; background: rgba(167,139,250,0.1); border-radius: 3px; color: #a78bfa; }
.gw-meta { display: flex; gap: 12px; flex-wrap: wrap; }

.meta-chip { display: flex; align-items: center; gap: 4px; font-size: 11px; color: #8b8f96; }
.chip-label { color: #6b7078; }
.chip-value { color: #e8eaec; }

.gw-conditions { display: flex; flex-direction: column; gap: 4px; }
.cond-row { display: flex; align-items: center; gap: 8px; padding: 4px 8px; border-radius: 4px; font-size: 11px; }
.cond-row.ok { background: rgba(62,207,142,0.08); }
.cond-row.fail { background: rgba(240,84,84,0.08); }
.cond-type { font-weight: 500; min-width: 90px; }
.cond-row.ok .cond-type { color: #3ecf8e; }
.cond-row.fail .cond-type { color: #f05454; }
.cond-status { padding: 1px 5px; border-radius: 3px; font-size: 10px; font-weight: 600; }
.cond-row.ok .cond-status { background: rgba(62,207,142,0.15); color: #3ecf8e; }
.cond-row.fail .cond-status { background: rgba(240,84,84,0.15); color: #f05454; }
.cond-reason { color: #8b8f96; }
.cond-msg { color: #6b7078; flex: 1; }

.gw-routes { border-top: 1px solid rgba(255,255,255,0.06); padding-top: 8px; display: flex; flex-direction: column; gap: 4px; }
.routes-subtitle { font-size: 10.5px; font-weight: 600; color: #8b8f96; text-transform: uppercase; letter-spacing: 0.04em; }
.route-chip { display: flex; align-items: center; gap: 6px; padding: 4px 8px; background: rgba(167,139,250,0.06); border-radius: 4px; font-size: 12px; }
.route-name { color: #a78bfa; }
.route-ns { color: #6b7078; font-size: 10.5px; }
.route-hosts { color: #8b8f96; font-size: 11px; margin-left: auto; }

.route-list { display: flex; flex-direction: column; gap: 8px; }
.route-card { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 14px 16px; display: flex; flex-direction: column; gap: 8px; }
.route-header { display: flex; align-items: center; gap: 8px; }
.route-name { font-size: 13px; font-weight: 500; color: #e8eaec; }
.route-ns { font-size: 11px; color: #6b7078; }
.route-meta { display: flex; gap: 12px; flex-wrap: wrap; }

.route-backends { border-top: 1px solid rgba(255,255,255,0.04); padding-top: 6px; display: flex; flex-direction: column; gap: 4px; }
.backends-subtitle { font-size: 10px; font-weight: 600; color: #6b7078; text-transform: uppercase; letter-spacing: 0.04em; }
.backend-chip { display: flex; align-items: center; gap: 4px; padding: 3px 8px; background: rgba(62,207,142,0.08); border-radius: 4px; font-size: 12px; }
.bk-name { color: #e8eaec; }
.bk-port { color: #3ecf8e; }
.bk-weight { color: #8b8f96; }

.state-box { padding: 24px; text-align: center; color: #8b8f96; font-size: 13px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.font-mono { font-family: var(--mono); }
</style>
