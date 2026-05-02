<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useCostEstimate, invalidateCachePrefix } from '../../composables/useWails'

const { report, loading, error, provider, fetchCosts } = useCostEstimate()

const providers = [
  { id: 'aws', label: 'AWS', icon: '☁' },
  { id: 'gcp', label: 'GCP', icon: '◎' },
  { id: 'azure', label: 'Azure', icon: '◆' },
  { id: 'digitalocean', label: 'DigitalOcean', icon: '◉' }
]

const chartMode = ref('daily') // 'daily' | 'monthly'

function selectProvider(id) {
  provider.value = id
  fetchCosts(id)
}

onMounted(() => fetchCosts())

// Compute monthly aggregation from daily history
const monthlyHistory = computed(() => {
  if (!report.value?.dailyHistory) return []
  const byMonth = {}
  for (const d of report.value.dailyHistory) {
    const month = d.date.slice(0, 7) // "2025-01"
    if (!byMonth[month]) byMonth[month] = { date: month, costDay: 0 }
    byMonth[month].costDay += d.costDay
  }
  return Object.values(byMonth).sort((a, b) => a.date.localeCompare(b.date))
})

const chartData = computed(() => {
  if (chartMode.value === 'monthly') return monthlyHistory.value
  return report.value?.dailyHistory || []
})

const chartMax = computed(() => {
  if (!chartData.value.length) return 1
  return Math.max(...chartData.value.map(d => d.costDay)) * 1.15
})

function fmt(val) {
  if (val == null) return '—'
  return '$' + val.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function fmtK(val) {
  if (val == null) return '—'
  if (val >= 1000) return '$' + (val / 1000).toFixed(1) + 'k'
  return '$' + val.toFixed(2)
}

function fmtCores(val) {
  if (val == null) return '—'
  return val.toFixed(2)
}

function barLabel(item) {
  if (chartMode.value === 'monthly') {
    const [y, m] = item.date.split('-')
    const months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']
    return months[parseInt(m) - 1]
  }
  return item.date.slice(8) // day number
}
</script>

<template>
  <div class="finops-view">
    <div class="header">
      <div class="header-text">
        <div class="title">Cost Explorer</div>
        <div class="subtitle">
          Estimated compute costs from pod resource requests
          <span v-if="report" class="provider-tag">{{ report.providerLabel }} rates</span>
        </div>
      </div>
      <button class="refresh-btn" @click="invalidateCachePrefix('EstimateCosts'); fetchCosts()" :disabled="loading">
        {{ loading ? 'Calculating…' : '↻ Refresh' }}
      </button>
    </div>

    <!-- Provider Selector -->
    <div class="provider-bar">
      <button
        v-for="p in providers"
        :key="p.id"
        class="provider-btn"
        :class="{ active: provider === p.id }"
        @click="selectProvider(p.id)"
      >
        <span class="provider-icon">{{ p.icon }}</span>
        {{ p.label }}
      </button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>
    <div v-if="loading && !report" class="loading-state">Calculating costs across all pods…</div>

    <template v-if="report">
      <!-- Summary cards -->
      <div class="cost-summary">
        <div class="cost-card highlight">
          <div class="cost-label">Monthly Estimate</div>
          <div class="cost-value big">{{ fmt(report.totalCostMo) }}</div>
          <div class="cost-sub">{{ fmt(report.totalCostDay) }}/day · {{ fmt(report.totalCostHr) }}/hr</div>
        </div>
        <div class="cost-card">
          <div class="cost-label">Expected Yearly Cost</div>
          <div class="cost-value year">{{ fmtK(report.totalCostYear) }}</div>
          <div class="cost-sub">Projected from current usage</div>
        </div>
        <div class="cost-card">
          <div class="cost-label">CPU Requested</div>
          <div class="cost-value">{{ fmtCores(report.totalCpu) }} <span class="unit">cores</span></div>
        </div>
        <div class="cost-card">
          <div class="cost-label">Memory Requested</div>
          <div class="cost-value">{{ fmtCores(report.totalMemGb) }} <span class="unit">GB</span></div>
        </div>
      </div>

      <!-- Cost history bar chart -->
      <div class="section" v-if="chartData.length">
        <div class="section-header">
          <div class="section-title">Cost History</div>
          <div class="chart-toggle">
            <button :class="{ active: chartMode === 'daily' }" @click="chartMode = 'daily'">Daily</button>
            <button :class="{ active: chartMode === 'monthly' }" @click="chartMode = 'monthly'">Monthly</button>
          </div>
        </div>
        <div class="chart-container">
          <div class="chart-y-axis">
            <span>{{ fmtK(chartMax) }}</span>
            <span>{{ fmtK(chartMax / 2) }}</span>
            <span>$0</span>
          </div>
          <div class="chart-bars">
            <div
              v-for="(item, idx) in chartData"
              :key="idx"
              class="bar-col"
              :style="{ width: (100 / chartData.length) + '%' }"
            >
              <div class="bar-tooltip">{{ fmt(item.costDay) }}</div>
              <div
                class="bar"
                :style="{ height: (item.costDay / chartMax * 100) + '%' }"
              ></div>
              <div class="bar-label">{{ barLabel(item) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Cost Breakdown by Category -->
      <div class="section" v-if="report.costCategories && report.costCategories.length">
        <div class="section-title">Major Cost Drivers</div>
        <div class="category-grid">
          <div v-for="cat in report.costCategories" :key="cat.name" class="category-card">
            <div class="cat-header">
              <span class="cat-name">{{ cat.name }}</span>
              <span class="cat-pct">{{ cat.percentage }}%</span>
            </div>
            <div class="cat-bar-bg">
              <div class="cat-bar-fill" :style="{ width: cat.percentage + '%' }"></div>
            </div>
            <div class="cat-cost">{{ fmt(cat.costMo) }}/mo</div>
          </div>
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
.title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.subtitle { font-size: 13px; color: #8b8f96; display: flex; align-items: center; gap: 8px; }
.provider-tag { background: rgba(167, 139, 250, 0.15); color: #a78bfa; font-size: 11px; padding: 2px 8px; border-radius: 4px; font-weight: 600; }
.refresh-btn {
  background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba;
  padding: 6px 12px; border-radius: 6px; font-size: 12px; cursor: pointer; transition: all 0.15s;
}
.refresh-btn:hover { background: rgba(255,255,255,0.1); color: #fff; }
.refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }

/* Provider Selector */
.provider-bar { display: flex; gap: 8px; }
.provider-btn {
  display: flex; align-items: center; gap: 6px;
  background: #1e2023; border: 1px solid rgba(255,255,255,0.08); color: #8b8f96;
  padding: 8px 16px; border-radius: 6px; font-size: 13px; cursor: pointer; transition: all 0.15s;
  font-weight: 500;
}
.provider-btn:hover { border-color: rgba(255,255,255,0.2); color: #b0b4ba; }
.provider-btn.active {
  background: rgba(167, 139, 250, 0.1); border-color: rgba(167, 139, 250, 0.4); color: #a78bfa;
}
.provider-icon { font-size: 14px; }

.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.1); border: 1px solid rgba(240,84,84,0.2); border-radius: 6px; font-size: 13px; color: #f05454; }
.loading-state, .empty-state { padding: 40px; text-align: center; color: #8b8f96; font-size: 13px; }

/* Summary Cards */
.cost-summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; }
.cost-card {
  background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 16px;
  display: flex; flex-direction: column; gap: 4px;
}
.cost-card.highlight { border-color: rgba(62, 207, 142, 0.3); }
.cost-label { font-size: 11px; color: #8b8f96; text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600; }
.cost-value { font-size: 20px; font-weight: 600; color: #e8eaec; font-family: 'SF Mono', Consolas, monospace; }
.cost-value.big { font-size: 28px; color: #3ecf8e; }
.cost-value.year { font-size: 24px; color: #a78bfa; }
.cost-value .unit { font-size: 13px; font-weight: 400; color: #8b8f96; }
.cost-sub { font-size: 11px; color: #6b7078; }

/* Sections */
.section { display: flex; flex-direction: column; gap: 8px; }
.section-header { display: flex; justify-content: space-between; align-items: center; }
.section-title { font-size: 13px; font-weight: 600; color: #fff; }

/* Chart Toggle */
.chart-toggle { display: flex; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 4px; overflow: hidden; }
.chart-toggle button {
  background: none; border: none; color: #6b7078; padding: 4px 12px; font-size: 11px; cursor: pointer; font-weight: 600;
  transition: all 0.15s;
}
.chart-toggle button.active { background: rgba(167, 139, 250, 0.15); color: #a78bfa; }
.chart-toggle button:hover:not(.active) { color: #b0b4ba; }

/* Bar Chart */
.chart-container {
  display: flex; gap: 8px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08);
  border-radius: 8px; padding: 16px; height: 200px;
}
.chart-y-axis {
  display: flex; flex-direction: column; justify-content: space-between;
  font-size: 10px; color: #6b7078; font-family: 'SF Mono', Consolas, monospace;
  min-width: 50px; text-align: right; padding-bottom: 20px;
}
.chart-bars {
  flex: 1; display: flex; align-items: flex-end; gap: 2px; padding-bottom: 20px;
  position: relative;
  border-bottom: 1px solid rgba(255,255,255,0.06);
}
.bar-col {
  display: flex; flex-direction: column; align-items: center; position: relative; height: 100%;
  justify-content: flex-end;
}
.bar {
  width: 70%; min-width: 4px; max-width: 24px; background: rgba(62, 207, 142, 0.6);
  border-radius: 2px 2px 0 0; transition: height 0.3s ease, background 0.15s;
  cursor: pointer;
}
.bar:hover { background: #3ecf8e; }
.bar-col:hover .bar-tooltip { opacity: 1; transform: translateY(-4px); }
.bar-tooltip {
  position: absolute; top: -4px; font-size: 10px; color: #e8eaec; background: #2a2d31;
  padding: 2px 6px; border-radius: 3px; white-space: nowrap; pointer-events: none;
  opacity: 0; transition: all 0.15s; z-index: 2;
}
.bar-label {
  position: absolute; bottom: -18px; font-size: 9px; color: #6b7078;
  font-family: 'SF Mono', Consolas, monospace;
}

/* Cost Categories */
.category-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
.category-card {
  background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 14px;
  display: flex; flex-direction: column; gap: 8px;
}
.cat-header { display: flex; justify-content: space-between; align-items: center; }
.cat-name { font-size: 13px; font-weight: 500; color: #e8eaec; }
.cat-pct { font-size: 13px; font-weight: 600; color: #a78bfa; font-family: 'SF Mono', Consolas, monospace; }
.cat-bar-bg { height: 6px; background: rgba(255,255,255,0.06); border-radius: 3px; overflow: hidden; }
.cat-bar-fill { height: 100%; background: linear-gradient(90deg, #a78bfa, #3ecf8e); border-radius: 3px; transition: width 0.5s ease; }
.cat-cost { font-size: 12px; color: #8b8f96; font-family: 'SF Mono', Consolas, monospace; }

/* Tables */
.cost-table { display: flex; flex-direction: column; border: 1px solid rgba(255,255,255,0.08); border-radius: 6px; overflow: hidden; }
.table-header {
  display: flex; padding: 8px 12px; background: rgba(255,255,255,0.03);
  font-size: 11px; font-weight: 600; color: #8b8f96; text-transform: uppercase; letter-spacing: 0.04em;
  border-bottom: 1px solid rgba(255,255,255,0.08);
}
.table-row {
  display: flex; padding: 8px 12px; font-size: 12.5px; color: #b0b4ba;
  border-bottom: 1px solid rgba(255,255,255,0.03); transition: background 0.1s;
}
.table-row:hover { background: rgba(255,255,255,0.02); }
.table-row:last-child { border-bottom: none; }

.col-name { flex: 2; font-weight: 500; color: #e8eaec; font-family: 'SF Mono', Consolas, monospace; font-size: 12px; }
.col-ns { flex: 1.5; font-size: 12px; color: #8b8f96; }
.col-num { flex: 1; text-align: right; font-family: 'SF Mono', Consolas, monospace; font-size: 12px; }
.cost-val { color: #3ecf8e; font-weight: 600; }
</style>
