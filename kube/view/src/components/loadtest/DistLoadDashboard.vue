<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useDistLoadStore } from '../../stores/distload'

const store = useDistLoadStore()
const { status, activeRunId, error, isRunning, isTerminal } = storeToRefs(store)

// Resume polling on mount if there's a persisted active run id from a
// previous session (closed tab mid-run). Without this the dashboard
// rendered "No active test" while a real cloud run was still burning
// credits in the background.
onMounted(() => {
  if (!activeRunId.value) {
    store.resumeActiveRun()
  }
})

// Without this, the store's setTimeout poll chain keeps firing
// callGo('GetDistributedLoadTestStatus', runId) every 1.5s
// indefinitely after the component unmounts. The store's stopPolling
// drops the timer cleanly. We do NOT clear activeRunId — the run
// is still in flight on the SaaS side and we want the next mount
// (or a reload) to pick it back up via resumeActiveRun().
onUnmounted(() => {
  store.stopPolling()
})

const STAGES = [
  { key: 'provisioning', label: 'Provisioning VMs' },
  { key: 'running', label: 'Running Test' },
  { key: 'collapsing', label: 'Collapsing VMs' },
  { key: 'done', label: 'Complete' },
]

const currentStageIndex = computed(() => {
  const s = status.value?.state
  if (!s) return -1
  if (s === 'done' || s === 'error') return 3
  if (s === 'collapsing') return 2
  if (s === 'running') return 1
  if (s === 'provisioning' || s === 'pending') return 0
  return -1
})

const stageLabel = computed(() => {
  const s = status.value?.state
  if (!s) return ''
  const stage = STAGES.find(st => st.key === s)
  if (stage) return stage.label
  if (s === 'error') return 'Error'
  if (s === 'canceled') return 'Canceled'
  return s
})

const regionStatuses = computed(() => {
  const progress = status.value?.provisionProgress
  if (!progress?.length) {
    const workers = status.value?.workers
    if (workers?.length) {
      return workers.map(w => ({
        region: w.region,
        state: w.state,
        healthy: w.state === 'running' || w.state === 'done',
        failing: w.state === 'error',
        sent: w.sent,
        acked: w.acked,
        errors: w.errors,
        throughput: w.throughput,
        p50Ms: w.p50Ms,
        p95Ms: w.p95Ms,
        p99Ms: w.p99Ms,
      }))
    }
    return []
  }
  return progress.map(p => ({
    region: p.region,
    state: p.state,
    healthy: p.state === 'ready' || p.state === 'running',
    failing: p.state === 'failed',
    vmsSpec: p.vmsSpec,
    vmsReady: p.vmsReady,
    errorMessage: p.errorMessage,
  }))
})

function handleCancel() {
  // Distributed load tests provision real cloud VMs. Mis-clicked
  // Cancel before, mid-run, used to silently tear them down and
  // burn the credits already spent on provisioning. The audit
  // flagged this as a UX gap. Native confirm() is the smallest
  // possible surface — the project doesn't ship a modal-dialog
  // component, and a one-line confirm is sufficient signal.
  const ok = window.confirm(
    'Cancel this distributed load test? VMs already provisioned will be torn down and the credits spent so far cannot be refunded.',
  )
  if (!ok) return
  store.cancel()
}

const hasPartialResults = computed(() => {
  return isTerminal.value && status.value?.error && status.value?.summary
})
</script>

<template>
  <div class="dashboard">
    <div v-if="!activeRunId" class="empty-state">
      <div class="empty-icon">⚡</div>
      <h3>No Active Test</h3>
      <p>Start a distributed load test from the form tab.</p>
    </div>

    <!-- Run id exists but first poll hasn't returned yet — render
         "Initializing" instead of leaving the dashboard blank or
         flashing "No Active Test" briefly. The audit flagged the
         empty-state flash as confusing for users who just clicked
         Start. -->
    <div v-else-if="!status" class="empty-state">
      <div class="empty-icon spinner-icon">⏳</div>
      <h3>Initializing…</h3>
      <p>Provisioning request sent. Waiting for the SaaS platform to acknowledge.</p>
    </div>

    <div v-else class="dashboard-content">
      <!-- Stage indicator -->
      <div class="stage-bar">
        <div
          v-for="(stage, i) in STAGES"
          :key="stage.key"
          class="stage-step"
          :class="{
            active: currentStageIndex === i,
            done: currentStageIndex > i,
            error: status?.state === 'error' && currentStageIndex === i,
            canceled: status?.state === 'canceled' && currentStageIndex >= i,
          }"
        >
          <div class="stage-dot"></div>
          <span class="stage-label">{{ stage.label }}</span>
        </div>
        <div class="stage-bar-fill" :style="{ width: currentStageIndex >= 0 ? `${(currentStageIndex / (STAGES.length - 1)) * 100}%` : '0%' }"></div>
      </div>

      <div class="stage-title">
        <strong>{{ stageLabel }}</strong>
        <span class="run-id">ID: {{ activeRunId?.slice(0, 8) }}…</span>
      </div>

      <!-- Error banner -->
      <div v-if="error" class="error-banner">{{ error }}</div>
      <div v-if="hasPartialResults" class="warn-banner">
        Test completed with errors. Some regions may have failed. Results are partial.
      </div>

      <!-- Region grid -->
      <div v-if="regionStatuses.length" class="region-grid">
        <div
          v-for="r in regionStatuses"
          :key="r.region"
          class="region-card"
          :class="{ healthy: r.healthy, failing: r.failing, pending: !r.healthy && !r.failing }"
        >
          <div class="region-header">
            <span class="region-name">{{ r.region }}</span>
            <span class="region-state-badge" :class="r.state">{{ r.state }}</span>
          </div>
          <div v-if="r.vmsSpec != null" class="region-detail">
            VMs: {{ r.vmsReady }}/{{ r.vmsSpec }} ready
          </div>
          <div v-if="r.sent != null" class="region-metrics">
            <div class="metric">Sent: {{ r.sent?.toLocaleString() }}</div>
            <div class="metric">Acked: {{ r.acked?.toLocaleString() }}</div>
            <div class="metric">Errors: {{ r.errors }}</div>
            <div class="metric">Throughput: {{ r.throughput?.toFixed(1) }} msg/s</div>
            <div class="metric">P50: {{ r.p50Ms?.toFixed(1) }}ms</div>
            <div class="metric">P95: {{ r.p95Ms?.toFixed(1) }}ms</div>
          </div>
          <div v-if="r.errorMessage" class="region-error">{{ r.errorMessage }}</div>
        </div>
      </div>

      <!-- Summary -->
      <div v-if="status?.summary" class="summary-card">
        <h4>Aggregate Results</h4>
        <div class="summary-grid">
          <div class="summary-stat">
            <span class="stat-value">{{ status.summary.totalSent?.toLocaleString() }}</span>
            <span class="stat-label">Sent</span>
          </div>
          <div class="summary-stat">
            <span class="stat-value">{{ status.summary.totalAcked?.toLocaleString() }}</span>
            <span class="stat-label">Acked</span>
          </div>
          <div class="summary-stat">
            <span class="stat-value">{{ status.summary.totalErrors }}</span>
            <span class="stat-label">Errors</span>
          </div>
          <div class="summary-stat">
            <span class="stat-value">{{ status.summary.throughput?.toFixed(1) }}</span>
            <span class="stat-label">Msg/s</span>
          </div>
          <div class="summary-stat">
            <span class="stat-value">{{ status.summary.p50LatencyMs?.toFixed(1) }}ms</span>
            <span class="stat-label">P50</span>
          </div>
          <div class="summary-stat">
            <span class="stat-value">{{ status.summary.p95LatencyMs?.toFixed(1) }}ms</span>
            <span class="stat-label">P95</span>
          </div>
        </div>
        <div v-if="status.creditsUsed" class="credits-used">Credits consumed: {{ status.creditsUsed.toFixed(1) }}</div>
        <div v-if="status.creditsEstimated && !status.creditsUsed" class="credits-used">Estimated credits: {{ status.creditsEstimated.toFixed(1) }}</div>
      </div>

      <!-- Actions -->
      <div class="actions-bar">
        <button v-if="isRunning" class="btn-cancel" @click="handleCancel">Cancel Test</button>
        <button v-if="isTerminal" class="btn-secondary" @click="store.reset()">Start New Test</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dashboard { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 16px; }
.empty-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; color: var(--text3); }
.empty-icon { font-size: 32px; }
.empty-state h3 { margin: 0; font-size: 16px; color: var(--text2); }
.empty-state p { margin: 0; font-size: 13px; }
.stage-bar { display: flex; align-items: center; gap: 0; position: relative; padding: 12px 0; }
.stage-step { display: flex; align-items: center; gap: 6px; flex: 1; position: relative; z-index: 1; }
.stage-dot { width: 10px; height: 10px; border-radius: 50%; background: var(--bg4); border: 2px solid var(--border); flex-shrink: 0; }
.stage-step.active .stage-dot { background: var(--accent); border-color: var(--accent); box-shadow: 0 0 0 3px rgba(79,142,247,0.2); }
.stage-step.done .stage-dot { background: var(--green); border-color: var(--green); }
.stage-step.error .stage-dot { background: #d05858; border-color: #d05858; }
.stage-step.canceled .stage-dot { background: #d09c58; border-color: #d09c58; }
.stage-label { font-size: 11px; color: var(--text3); white-space: nowrap; }
.stage-step.active .stage-label { color: var(--accent2); font-weight: 500; }
.stage-step.done .stage-label { color: var(--green); }
.step-bar-fill { position: absolute; top: 50%; left: 0; height: 2px; background: var(--accent); transition: width 0.3s; }
.stage-title { font-size: 14px; color: var(--text); display: flex; align-items: center; gap: 12px; }
.run-id { font-size: 11px; color: var(--text3); font-family: monospace; }
.error-banner { background: rgba(208,88,88,0.1); border: 1px solid rgba(208,88,88,0.3); border-radius: 6px; padding: 8px 12px; font-size: 12px; color: #d05858; }
.warn-banner { background: rgba(208,156,88,0.1); border: 1px solid rgba(208,156,88,0.3); border-radius: 6px; padding: 8px 12px; font-size: 12px; color: #d09c58; }
.region-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 12px; }
.region-card { background: var(--bg2); border: 1px solid var(--border); border-radius: 8px; padding: 12px; }
.region-card.healthy { border-left: 3px solid var(--green); }
.region-card.failing { border-left: 3px solid #d05858; }
.region-card.pending { border-left: 3px solid var(--text3); }
.region-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.region-name { font-size: 13px; font-weight: 500; color: var(--text); }
.region-state-badge { font-size: 10px; font-weight: 500; padding: 2px 6px; border-radius: 4px; background: var(--bg3); color: var(--text2); text-transform: uppercase; }
.region-state-badge.running { background: rgba(79,220,120,0.12); color: #4fdc78; }
.region-state-badge.done { background: rgba(79,220,120,0.12); color: #4fdc78; }
.region-state-badge.failed { background: rgba(208,88,88,0.12); color: #d05858; }
.region-state-badge.provisioning { background: rgba(79,142,247,0.12); color: var(--accent2); }
.region-detail { font-size: 11px; color: var(--text3); margin-bottom: 6px; }
.region-metrics { display: grid; grid-template-columns: 1fr 1fr; gap: 2px 12px; font-size: 11px; color: var(--text2); }
.metric { white-space: nowrap; }
.region-error { margin-top: 6px; font-size: 11px; color: #d05858; }
.summary-card { background: var(--bg2); border: 1px solid var(--border); border-radius: 8px; padding: 16px; }
.summary-card h4 { margin: 0 0 12px; font-size: 13px; font-weight: 600; color: var(--text); }
.summary-grid { display: grid; grid-template-columns: repeat(6, 1fr); gap: 12px; }
.summary-stat { display: flex; flex-direction: column; align-items: center; gap: 2px; }
.stat-value { font-size: 18px; font-weight: 700; color: var(--text); }
.stat-label { font-size: 10px; color: var(--text3); text-transform: uppercase; letter-spacing: 0.05em; }
.credits-used { margin-top: 8px; font-size: 11px; color: var(--text3); }
.actions-bar { display: flex; gap: 8px; }
.btn-cancel { padding: 6px 16px; border-radius: 6px; font-size: 12px; font-weight: 500; border: 1px solid #d05858; color: #d05858; background: transparent; cursor: pointer; }
.btn-cancel:hover { background: rgba(208,88,88,0.1); }
.btn-secondary { padding: 6px 16px; border-radius: 6px; font-size: 12px; font-weight: 500; border: 1px solid var(--border); color: var(--text2); background: var(--bg3); cursor: pointer; }
.btn-secondary:hover { background: var(--bg4); color: var(--text); }
</style>
