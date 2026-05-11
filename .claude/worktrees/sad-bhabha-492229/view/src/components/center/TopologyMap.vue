<script setup>
import { computed, onMounted } from 'vue'
import { useTopology } from '../../composables/useWails'

const props = defineProps({
  alerts: { type: Array, default: () => [] }
})

const { topology, loading: topoLoading, fetchTopology } = useTopology()

onMounted(() => {
  fetchTopology('')
})

// Use real topology if available, else derive from alerts.
const nodes = computed(() => {
  // Real backend topology.
  if (topology.value && topology.value.nodes && topology.value.nodes.length > 0) {
    return topology.value.nodes.map(n => ({
      name: n.name,
      kind: n.kind,
      status: n.status,
      namespace: n.namespace,
    }))
  }

  // Fallback: derive from alerts.
  const statuses = {}
  for (const alert of props.alerts) {
    const name = alert.podName || alert.nodeName
    if (!name) continue
    const parts = name.split('-')
    const deploy = parts.length >= 2 ? parts.slice(0, 2).join('-') : name
    if (!statuses[deploy] || sevPriority(alert.severity) < sevPriority(statuses[deploy])) {
      statuses[deploy] = alert.severity === 'critical' ? 'crit' : alert.severity === 'warning' ? 'warn' : 'ok'
    }
  }
  return Object.entries(statuses).map(([name, status]) => ({ name, status, kind: 'deployment' }))
})

const edges = computed(() => {
  if (topology.value && topology.value.edges) {
    return topology.value.edges
  }
  return []
})

// Group nodes by kind for organized display.
const nodesByKind = computed(() => {
  const groups = { node: [], service: [], deployment: [], pod: [] }
  for (const n of nodes.value) {
    const kind = n.kind || 'deployment'
    if (!groups[kind]) groups[kind] = []
    groups[kind].push(n)
  }
  return groups
})

function sevPriority(s) {
  return s === 'critical' ? 0 : s === 'warning' ? 1 : 2
}
</script>

<template>
  <div>
    <div class="section-header">
      <div class="section-title">Service Topology</div>
      <button v-if="!topoLoading" class="topo-refresh" @click="fetchTopology('')">↻</button>
      <span v-else class="topo-loading">loading…</span>
    </div>
    <div class="topo-container">
      <template v-if="nodes.length > 0">
        <template v-for="(kindNodes, kind) in nodesByKind" :key="kind">
          <div v-if="kindNodes.length > 0" class="topo-kind-row">
            <div class="kind-label">{{ kind }}s</div>
            <div class="topo-row">
              <div
                v-for="node in kindNodes"
                :key="node.name"
                class="topo-node"
                :class="node.status"
                :title="node.namespace ? node.namespace + '/' + node.name : node.name"
              >
                {{ node.name }}
              </div>
            </div>
          </div>
        </template>
      </template>
      <div v-else class="topo-empty">No topology data — connect a cluster</div>
    </div>
  </div>
</template>

<style scoped>
.section-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.section-title { font-size: 11px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--text3); }

.topo-refresh { background: none; border: none; color: var(--text3); cursor: pointer; font-size: 14px; transition: color 0.1s; }
.topo-refresh:hover { color: var(--accent2); }
.topo-loading { font-size: 10px; color: var(--text3); }

.topo-container {
  background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r);
  padding: 12px 14px; display: flex; flex-direction: column; gap: 10px;
}

.topo-kind-row { display: flex; flex-direction: column; gap: 4px; }
.kind-label { font-size: 9px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em; color: var(--text3); }

.topo-row { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; }

.topo-node {
  padding: 4px 8px; border-radius: 5px; font-size: 10px; font-family: var(--mono);
  border: 1px solid var(--border); background: var(--bg3); color: var(--text2); white-space: nowrap;
  max-width: 180px; overflow: hidden; text-overflow: ellipsis;
}
.topo-node.crit { border-color: rgba(240,84,84,0.4); color: var(--red2); background: rgba(240,84,84,0.07); }
.topo-node.warn { border-color: rgba(245,166,35,0.35); color: var(--amber2); background: rgba(245,166,35,0.07); }
.topo-node.ok { border-color: rgba(62,207,142,0.3); color: var(--green2); background: rgba(62,207,142,0.06); }

.topo-empty { color: var(--text3); font-size: 12px; text-align: center; padding: 16px; }
</style>
