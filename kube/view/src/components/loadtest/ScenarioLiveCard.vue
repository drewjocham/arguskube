<script setup>
import { computed, ref, watch } from 'vue'

// Per-endpoint live progress table. Animation goals (per spec):
//   - status icon pulses for ~600ms when executions tick up between polls
//   - success-rate fill bar at the row bottom reflects successes/executions
//   - sparkline (last 30 executions samples) gives time-shape at a glance
//
// Polling cadence is 1.5s in the store. We diff against the previous
// snapshot to detect "ticked up since last poll" rather than absolute
// counts, since long-running rows are always > 0.

const props = defineProps({
  scenario: { type: Object, default: null }, // ScenarioStatus
})

const HISTORY = 30
// Per-row state keyed by `${method} ${url}` (unique identifier from backend).
const rowState = ref(new Map())

function rowKey(ep) { return `${ep.method} ${ep.url}` }

watch(() => props.scenario?.endpoints, (eps) => {
  if (!eps) return
  for (const ep of eps) {
    const key = rowKey(ep)
    const prev = rowState.value.get(key) || { lastExecutions: 0, history: [], pulsing: false }
    const ticked = ep.executions > prev.lastExecutions
    const history = [...prev.history, ep.executions].slice(-HISTORY)
    rowState.value.set(key, {
      lastExecutions: ep.executions,
      history,
      pulsing: ticked,
    })
    if (ticked) {
      // settle after ~600ms — CSS animation handles the visual; we flip
      // the class so consecutive ticks re-trigger.
      const k = key
      setTimeout(() => {
        const s = rowState.value.get(k)
        if (s) { rowState.value.set(k, { ...s, pulsing: false }) }
      }, 600)
    }
  }
}, { deep: true, immediate: true })

const endpoints = computed(() => props.scenario?.endpoints || [])

function successRate(ep) {
  if (!ep.executions) return 0
  return Math.max(0, Math.min(1, ep.successes / ep.executions))
}

function sparkPath(history) {
  if (!history.length) return ''
  const w = 60, h = 16
  const max = Math.max(...history, 1)
  const min = Math.min(...history, 0)
  const range = Math.max(1, max - min)
  const pts = history.map((v, i) => {
    const x = (i / Math.max(1, history.length - 1)) * w
    const y = h - ((v - min) / range) * h
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })
  return `M ${pts.join(' L ')}`
}

function statusIcon(ep) {
  if (ep.httpFails + ep.assertFails > 0 && ep.successes === 0) return '✕'
  if (ep.executions > 0) return '●'
  return '○'
}

function rowClass(ep) {
  const s = rowState.value.get(rowKey(ep))
  return { pulsing: s?.pulsing, failing: ep.httpFails + ep.assertFails > 0 && ep.successes === 0 }
}
</script>

<template>
  <div v-if="endpoints.length" class="scenario-live" data-testid="scenario-live-card">
    <h4>Scenario steps (live)</h4>
    <div class="table" role="table">
      <div class="row head" role="row">
        <div class="col-status">·</div>
        <div class="col-endpoint">Endpoint</div>
        <div class="col-num">Executions</div>
        <div class="col-num">Success</div>
        <div class="col-num">HTTP fails</div>
        <div class="col-num">Assert fails</div>
        <div class="col-pcts">P50 / P95 / P99</div>
        <div class="col-spark">Trend</div>
        <div class="col-last">Last failure</div>
      </div>
      <div v-for="ep in endpoints" :key="`${ep.method} ${ep.url}`" class="row" :class="rowClass(ep)" role="row">
        <div class="col-status">
          <span class="status-icon" :class="rowClass(ep)">{{ statusIcon(ep) }}</span>
        </div>
        <div class="col-endpoint">
          <span class="method">{{ ep.method }}</span>
          <span class="url">{{ ep.name || ep.url }}</span>
        </div>
        <div class="col-num">{{ ep.executions?.toLocaleString() }}</div>
        <div class="col-num good">{{ ep.successes?.toLocaleString() }}</div>
        <div class="col-num bad">{{ ep.httpFails?.toLocaleString() }}</div>
        <div class="col-num bad">{{ ep.assertFails?.toLocaleString() }}</div>
        <div class="col-pcts">
          {{ ep.p50Ms?.toFixed(0) }} / {{ ep.p95Ms?.toFixed(0) }} / {{ ep.p99Ms?.toFixed(0) }} ms
        </div>
        <div class="col-spark">
          <svg width="60" height="16" viewBox="0 0 60 16" aria-hidden="true">
            <path :d="sparkPath(rowState.get(`${ep.method} ${ep.url}`)?.history || [])"
              fill="none" stroke="var(--accent2, #4f8cff)" stroke-width="1.2" />
          </svg>
        </div>
        <div class="col-last" :title="ep.lastFail">{{ ep.lastFail || '—' }}</div>
        <!-- Success-rate fill bar at row bottom -->
        <div class="rate-bar" :style="{ width: (successRate(ep) * 100) + '%' }"></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.scenario-live { background: var(--bg2); border: 1px solid var(--border); border-radius: 8px; padding: 12px; }
.scenario-live h4 { margin: 0 0 8px; font-size: 13px; font-weight: 600; color: var(--text); }
.table { display: flex; flex-direction: column; }
.row { display: grid; grid-template-columns: 24px 1.4fr 70px 70px 70px 70px 100px 70px 1fr; gap: 8px; padding: 6px 4px; font-size: 12px; color: var(--text2); position: relative; align-items: center; }
.row.head { color: var(--text3); font-size: 10px; text-transform: uppercase; letter-spacing: 0.04em; border-bottom: 1px solid var(--border); }
.row + .row { border-top: 1px solid var(--border); }
.col-num { text-align: right; font-variant-numeric: tabular-nums; }
.col-pcts { text-align: right; font-variant-numeric: tabular-nums; }
.col-last { color: #d05858; font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.method { display: inline-block; background: var(--bg3); border-radius: 3px; padding: 1px 4px; font-size: 10px; margin-right: 4px; color: var(--text2); }
.url { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; color: var(--text); }
.good { color: #4fdc78; }
.bad { color: #d05858; }
.status-icon { color: var(--accent); display: inline-block; }
.status-icon.failing { color: #d05858; }
/* pulse keyframe — the "icon that updates and shows movement" the user asked for. */
@keyframes scenario-pulse {
  0%   { transform: scale(1);   opacity: 1; }
  40%  { transform: scale(1.4); opacity: 0.6; }
  100% { transform: scale(1);   opacity: 1; }
}
.row.pulsing .status-icon { animation: scenario-pulse 0.6s ease-out; }
/* Success-rate bar at row bottom — animates as ratio changes. */
.rate-bar { position: absolute; left: 0; bottom: 0; height: 2px; background: linear-gradient(90deg, #4fdc78, var(--accent)); transition: width 0.4s ease; }
</style>
