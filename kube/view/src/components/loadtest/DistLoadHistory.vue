<script setup>
import { ref, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useDistLoadStore } from '../../stores/distload'
import { useSectionTabsStore } from '../../stores/sectionTabs'

const store = useDistLoadStore()
const { runHistory, historyLoading, error } = storeToRefs(store)
const sectionTabs = useSectionTabsStore()

const selectedRun = ref(null)

function viewDetails(run) {
  selectedRun.value = run
}

function closeDetails() {
  selectedRun.value = null
}

function refresh() {
  store.loadHistory()
}

function stateClass(state) {
  switch (state) {
    case 'done': return 'state-done'
    case 'error': return 'state-error'
    case 'canceled': return 'state-canceled'
    default: return 'state-running'
  }
}

onMounted(() => {
  store.loadHistory()
})
</script>

<template>
  <div class="history">
    <div class="history-header">
      <h2>Distributed Load Test History</h2>
      <button class="btn-sm" @click="refresh" :disabled="historyLoading">
        {{ historyLoading ? 'Loading…' : 'Refresh' }}
      </button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <!-- Empty state -->
    <div v-if="!historyLoading && runHistory.length === 0" class="empty-state">
      <div class="empty-icon">📋</div>
      <h3>No Load Tests Yet</h3>
      <p>Run your first distributed load test to see results here.</p>
    </div>

    <!-- Loading -->
    <div v-if="historyLoading && runHistory.length === 0" class="loading-state">
      Loading history…
    </div>

    <!-- Detail view -->
    <div v-if="selectedRun" class="detail-panel">
      <div class="detail-header">
        <h3>Run Details</h3>
        <button class="btn-sm" @click="closeDetails">Back</button>
      </div>
      <div class="detail-body">
        <div class="detail-row"><span class="detail-label">ID</span><span class="detail-value">{{ selectedRun.runId }}</span></div>
        <div class="detail-row"><span class="detail-label">State</span><span class="detail-value" :class="stateClass(selectedRun.state)">{{ selectedRun.state }}</span></div>
        <div class="detail-row"><span class="detail-label">Started</span><span class="detail-value">{{ selectedRun.startedAt ? new Date(selectedRun.startedAt).toLocaleString() : '—' }}</span></div>
        <div class="detail-row"><span class="detail-label">Finished</span><span class="detail-value">{{ selectedRun.finishedAt ? new Date(selectedRun.finishedAt).toLocaleString() : '—' }}</span></div>
        <div v-if="selectedRun.creditsUsed" class="detail-row"><span class="detail-label">Credits</span><span class="detail-value">{{ selectedRun.creditsUsed.toFixed(1) }}</span></div>
        <div v-if="selectedRun.error" class="detail-row"><span class="detail-label">Error</span><span class="detail-value error-text">{{ selectedRun.error }}</span></div>
        <div v-if="selectedRun.summary" class="detail-summary">
          <h4>Summary</h4>
          <div class="summary-stats">
            <div>Sent: {{ selectedRun.summary.totalSent?.toLocaleString() }}</div>
            <div>Acked: {{ selectedRun.summary.totalAcked?.toLocaleString() }}</div>
            <div>Errors: {{ selectedRun.summary.totalErrors }}</div>
            <div>Throughput: {{ selectedRun.summary.throughput?.toFixed(1) }} msg/s</div>
            <div>P50: {{ selectedRun.summary.p50LatencyMs?.toFixed(1) }}ms</div>
            <div>P95: {{ selectedRun.summary.p95LatencyMs?.toFixed(1) }}ms</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Table view -->
    <div v-else class="history-table-wrap">
      <table v-if="runHistory.length > 0" class="history-table">
        <thead>
          <tr>
            <th>Name</th>
            <th>State</th>
            <th>Started</th>
            <th>Messages</th>
            <th>Credits</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="run in runHistory" :key="run.runId" @click="viewDetails(run)">
            <td class="cell-name">{{ run.name || '—' }}</td>
            <td><span class="state-badge" :class="stateClass(run.state)">{{ run.state }}</span></td>
            <td class="cell-date">{{ run.startedAt ? new Date(run.startedAt).toLocaleString() : '—' }}</td>
            <td>{{ run.summary?.totalSent?.toLocaleString() || '—' }}</td>
            <td>{{ run.creditsUsed?.toFixed(1) || '—' }}</td>
            <td><button class="btn-sm" @click.stop="viewDetails(run)">Details</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.history { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 12px; }
.history-header { display: flex; justify-content: space-between; align-items: center; }
.history-header h2 { margin: 0; font-size: 16px; font-weight: 600; color: var(--text); }
.btn-sm { padding: 4px 10px; border-radius: 6px; font-size: 11px; font-weight: 500; border: 1px solid var(--border); background: var(--bg3); color: var(--text2); cursor: pointer; }
.btn-sm:hover { background: var(--bg4); color: var(--text); }
.error-banner { background: rgba(208,88,88,0.1); border: 1px solid rgba(208,88,88,0.3); border-radius: 6px; padding: 8px 12px; font-size: 12px; color: #d05858; }
.empty-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px; color: var(--text3); }
.empty-icon { font-size: 32px; }
.empty-state h3 { margin: 0; font-size: 16px; color: var(--text2); }
.empty-state p { margin: 0; font-size: 13px; }
.loading-state { text-align: center; padding: 40px; color: var(--text3); font-size: 13px; }
.detail-panel { background: var(--bg2); border: 1px solid var(--border); border-radius: 8px; }
.detail-header { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border); }
.detail-header h3 { margin: 0; font-size: 14px; }
.detail-body { padding: 16px; display: flex; flex-direction: column; gap: 8px; }
.detail-row { display: flex; gap: 12px; font-size: 13px; }
.detail-label { color: var(--text3); min-width: 80px; }
.detail-value { color: var(--text); }
.error-text { color: #d05858; }
.detail-summary { margin-top: 8px; padding-top: 8px; border-top: 1px solid var(--border); }
.detail-summary h4 { margin: 0 0 8px; font-size: 13px; }
.summary-stats { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 6px; font-size: 12px; color: var(--text2); }
.history-table-wrap { flex: 1; overflow-y: auto; }
.history-table { width: 100%; border-collapse: collapse; font-size: 12px; }
.history-table th { text-align: left; padding: 8px 12px; color: var(--text3); font-weight: 500; border-bottom: 1px solid var(--border); }
.history-table td { padding: 8px 12px; border-bottom: 1px solid var(--border); color: var(--text2); }
.history-table tbody tr { cursor: pointer; }
.history-table tbody tr:hover { background: var(--bg2); }
.cell-name { font-weight: 500; color: var(--text); }
.cell-date { white-space: nowrap; }
.state-badge { font-size: 10px; font-weight: 500; padding: 2px 6px; border-radius: 4px; }
.state-done { color: #4fdc78; background: rgba(79,220,120,0.12); }
.state-error { color: #d05858; background: rgba(208,88,88,0.12); }
.state-canceled { color: #d09c58; background: rgba(208,156,88,0.12); }
.state-running { color: var(--accent2); background: rgba(79,142,247,0.12); }
</style>
