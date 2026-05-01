<script setup>
import { onMounted } from 'vue'
import { useCostEstimate } from '../../composables/useWails'

const { report, loading, error, fetchCosts } = useCostEstimate()

onMounted(fetchCosts)

function fmt(val) {
  if (val == null) return '—'
  return '$' + val.toFixed(2)
}

function fmtCores(val) {
  if (val == null) return '—'
  return val.toFixed(2)
}
</script>

<template>
  <div class="finops-view">
    <div class="header">
      <div class="header-text">
        <div class="title">Cost Explorer</div>
        <div class="subtitle">Estimated compute cost from pod resource requests (AWS on-demand rates)</div>
      </div>
      <button class="refresh-btn" @click="fetchCosts" :disabled="loading">
        {{ loading ? 'Calculating…' : 'Refresh' }}
      </button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div v-if="loading && !report" class="loading-state">Calculating costs across all pods…</div>

    <template v-if="report">
      <!-- Summary cards -->
      <div class="cost-summary">
        <div class="cost-card">
          <div class="cost-label">Monthly Estimate</div>
          <div class="cost-value big">{{ fmt(report.totalCostMo) }}</div>
          <div class="cost-sub">{{ fmt(report.totalCostDay) }}/day · {{ fmt(report.totalCostHr) }}/hr</div>
        </div>
        <div class="cost-card">
          <div class="cost-label">Total CPU Requested</div>
          <div class="cost-value">{{ fmtCores(report.totalCpu) }} cores</div>
        </div>
        <div class="cost-card">
          <div class="cost-label">Total Memory Requested</div>
          <div class="cost-value">{{ fmtCores(report.totalMemGb) }} GB</div>
        </div>
        <div class="cost-card">
          <div class="cost-label">Running Pods</div>
          <div class="cost-value">{{ report.podCount }}</div>
        </div>
      </div>

      <!-- Namespace breakdown -->
      <div class="section">
        <div class="section-title">Cost by Namespace</div>
        <div class="cost-table">
          <div class="table-header">
            <span class="col-name">Namespace</span>
            <span class="col-num">Pods</span>
            <span class="col-num">CPU</span>
            <span class="col-num">Memory</span>
            <span class="col-num">$/month</span>
          </div>
          <div v-for="ns in report.namespaces" :key="ns.name" class="table-row">
            <span class="col-name">{{ ns.name }}</span>
            <span class="col-num">{{ ns.podCount }}</span>
            <span class="col-num">{{ fmtCores(ns.cpuCores) }}</span>
            <span class="col-num">{{ fmtCores(ns.memoryGB) }} GB</span>
            <span class="col-num cost-val">{{ fmt(ns.totalCostMo) }}</span>
          </div>
        </div>
      </div>

      <!-- Top deployments -->
      <div class="section" v-if="report.topDeployments && report.topDeployments.length > 0">
        <div class="section-title">Top Deployments by Cost</div>
        <div class="cost-table">
          <div class="table-header">
            <span class="col-name">Deployment</span>
            <span class="col-ns">Namespace</span>
            <span class="col-num">Pods</span>
            <span class="col-num">CPU</span>
            <span class="col-num">$/month</span>
          </div>
          <div v-for="d in report.topDeployments" :key="d.namespace + '/' + d.name" class="table-row">
            <span class="col-name">{{ d.name }}</span>
            <span class="col-ns">{{ d.namespace }}</span>
            <span class="col-num">{{ d.podCount }}</span>
            <span class="col-num">{{ fmtCores(d.cpuCores) }}</span>
            <span class="col-num cost-val">{{ fmt(d.totalCostMo) }}</span>
          </div>
        </div>
      </div>
    </template>

    <div v-if="!loading && !report && !error" class="empty-state">
      No cost data available. Connect to a cluster and click Refresh.
    </div>
  </div>
</template>

<style scoped>
.finops-view { padding: 24px; display: flex; flex-direction: column; gap: 20px; overflow-y: auto; height: 100%; }
.header { display: flex; justify-content: space-between; align-items: flex-start; }
.title { font-size: 20px; font-weight: 500; color: var(--text); margin-bottom: 4px; }
.subtitle { font-size: 13px; color: var(--text3); }
.refresh-btn {
  background: transparent; border: 1px solid var(--border2); color: var(--text2);
  padding: 5px 12px; border-radius: 4px; font-size: 12px; cursor: pointer;
}
.refresh-btn:hover { color: var(--text); border-color: rgba(255,255,255,0.2); }
.refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.1); border: 1px solid rgba(240,84,84,0.2); border-radius: 6px; font-size: 13px; color: #f05454; }
.loading-state { padding: 40px; text-align: center; color: var(--text3); font-size: 13px; }
.empty-state { padding: 40px; text-align: center; color: var(--text3); font-size: 13px; }

.cost-summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.cost-card {
  background: var(--bg3); border: 1px solid var(--border); border-radius: 8px; padding: 16px;
  display: flex; flex-direction: column; gap: 4px;
}
.cost-label { font-size: 11px; color: var(--text3); text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600; }
.cost-value { font-size: 20px; font-weight: 600; color: var(--text); font-family: 'SF Mono', Consolas, monospace; }
.cost-value.big { font-size: 28px; color: var(--accent2); }
.cost-sub { font-size: 11px; color: var(--text3); }

.section { display: flex; flex-direction: column; gap: 8px; }
.section-title { font-size: 13px; font-weight: 600; color: var(--text); }

.cost-table { display: flex; flex-direction: column; border: 1px solid var(--border); border-radius: 6px; overflow: hidden; }
.table-header {
  display: flex; padding: 8px 12px; background: var(--bg3);
  font-size: 11px; font-weight: 600; color: var(--text3); text-transform: uppercase; letter-spacing: 0.04em;
  border-bottom: 1px solid var(--border);
}
.table-row {
  display: flex; padding: 8px 12px; font-size: 12.5px; color: var(--text2);
  border-bottom: 1px solid rgba(255,255,255,0.03); transition: background 0.1s;
}
.table-row:hover { background: var(--bg3); }
.table-row:last-child { border-bottom: none; }

.col-name { flex: 2; font-weight: 500; color: var(--text); font-family: 'SF Mono', Consolas, monospace; font-size: 12px; }
.col-ns { flex: 1.5; font-size: 12px; color: var(--text3); }
.col-num { flex: 1; text-align: right; font-family: 'SF Mono', Consolas, monospace; font-size: 12px; }
.cost-val { color: var(--accent2); font-weight: 600; }
</style>
