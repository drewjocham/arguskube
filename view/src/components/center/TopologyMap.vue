<script setup>
import { computed } from 'vue'

const props = defineProps({
  alerts: { type: Array, default: () => [] }
})

// Derive topology nodes from active alerts.
// In production, this would come from service mesh / eBPF data.
const nodes = computed(() => {
  const statuses = {}

  for (const alert of props.alerts) {
    const name = alert.podName || alert.nodeName
    if (!name) continue

    // Derive deployment name.
    const parts = name.split('-')
    const deploy = parts.length >= 2 ? parts.slice(0, 2).join('-') : name

    if (!statuses[deploy] || sevPriority(alert.severity) < sevPriority(statuses[deploy])) {
      statuses[deploy] = alert.severity === 'critical' ? 'crit' : alert.severity === 'warning' ? 'warn' : 'ok'
    }
  }

  return Object.entries(statuses).map(([name, status]) => ({ name, status }))
})

function sevPriority(s) {
  return s === 'critical' ? 0 : s === 'warning' ? 1 : 2
}
</script>

<template>
  <div>
    <div class="section-header">
      <div class="section-title">Service Topology</div>
    </div>
    <div class="topo-container">
      <div v-if="nodes.length > 0" class="topo-row">
        <div
          v-for="node in nodes"
          :key="node.name"
          class="topo-node"
          :class="node.status"
        >
          {{ node.name }}
        </div>
      </div>
      <div v-else class="topo-empty">No topology data — connect a cluster</div>
    </div>
  </div>
</template>

<style scoped>
.section-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.section-title { font-size: 11px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--text3); }

.topo-container {
  background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r);
  padding: 12px 14px;
}

.topo-row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }

.topo-node {
  padding: 5px 10px; border-radius: 6px; font-size: 11px; font-family: var(--mono);
  border: 1px solid var(--border); background: var(--bg3); color: var(--text2); white-space: nowrap;
}
.topo-node.crit { border-color: rgba(240,84,84,0.4); color: var(--red2); background: rgba(240,84,84,0.07); }
.topo-node.warn { border-color: rgba(245,166,35,0.35); color: var(--amber2); background: rgba(245,166,35,0.07); }
.topo-node.ok { border-color: rgba(62,207,142,0.3); color: var(--green2); background: rgba(62,207,142,0.06); }

.topo-empty { color: var(--text3); font-size: 12px; text-align: center; padding: 16px; }
</style>
