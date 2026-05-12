<script setup>
import { computed } from 'vue'

const props = defineProps({
  metrics: { type: Object, default: null }
})

function formatBytes(bytes) {
  if (!bytes) return '—'
  if (bytes >= 1073741824) return `${(bytes / 1073741824).toFixed(1)} Gi`
  if (bytes >= 1048576) return `${(bytes / 1048576).toFixed(0)} Mi`
  return `${(bytes / 1024).toFixed(0)} Ki`
}

function formatCPU(millis) {
  if (!millis) return '—'
  if (millis >= 1000) return `${(millis / 1000).toFixed(1)} cores`
  return `${millis}m`
}

const cards = computed(() => {
  const m = props.metrics
  if (!m) return []
  return [
    {
      label: 'Pod Health',
      value: `${m.podHealthPct?.toFixed(1) || '—'}%`,
      sub: `${m.podsRunning || 0} / ${m.podsTotal || 0} running` +
           (m.podsPending ? ` · ${m.podsPending} pending` : '') +
           (m.podsFailed ? ` · ${m.podsFailed} failed` : ''),
      color: m.podHealthPct >= 95 ? 'up' : m.podHealthPct >= 80 ? 'warn' : 'crit',
    },
    {
      label: 'Error Rate',
      value: `${m.errorRate?.toFixed(2) || '0.00'}%`,
      sub: m.warningEvents > 0 ? `${m.warningEvents} warning events (30m)` : 'No warnings',
      color: m.errorRate > 5 ? 'crit' : m.errorRate > 1 ? 'warn' : 'up',
    },
    {
      label: 'Restart Count',
      value: String(m.restartCount || 0),
      sub: m.restartTop || 'No restarts',
      color: m.restartCount > 20 ? 'crit' : m.restartCount > 5 ? 'warn' : 'norm',
    },
    {
      label: 'Cluster Resources',
      value: formatCPU(m.totalCpuMillis),
      sub: `${formatBytes(m.totalMemoryBytes)} mem requested`,
      color: 'norm',
    },
  ]
})
</script>

<template>
  <div class="metrics-row">
    <div v-for="(card, i) in cards" :key="i" class="metric-card">
      <div class="metric-label">{{ card.label }}</div>
      <div class="metric-value" :class="'metric-' + card.color">{{ card.value }}</div>
      <div class="metric-sub">{{ card.sub }}</div>
    </div>
    <div v-if="!metrics" class="metric-card metric-loading">
      <div class="metric-label">Connecting...</div>
      <div class="metric-value metric-norm">—</div>
      <div class="metric-sub">Waiting for cluster data</div>
    </div>
  </div>
</template>

<style scoped>
.metrics-row { display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; }

.metric-card {
  background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r);
  padding: 12px 14px; cursor: pointer; transition: border-color 0.15s, background 0.15s;
}
.metric-card:hover { background: var(--bg4); border-color: var(--border2); }
.metric-loading { grid-column: 1 / -1; text-align: center; }

.metric-label { font-size: 11px; color: var(--text2); font-weight: 400; margin-bottom: 5px; letter-spacing: 0.02em; }
.metric-value { font-size: 22px; font-weight: 500; font-family: var(--mono); letter-spacing: -0.02em; line-height: 1; }
.metric-sub { font-size: 10.5px; color: var(--text3); margin-top: 4px; }

.metric-up { color: var(--green); }
.metric-warn { color: var(--amber); }
.metric-crit { color: var(--red); }
.metric-norm { color: var(--text); }
</style>
