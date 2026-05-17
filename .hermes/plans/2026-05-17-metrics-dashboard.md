# Metrics Dashboard — Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task.

**Goal:** Replace the Live Log Stream widget in the Cluster Alerts view with a dynamic, category-organized metrics dashboard supporting dashboard pages, metric pinning, and pop-out catalog views.

**Architecture:** A new Pinia store (`dashboardStore`) owns dashboard state — pages, metric categories, visible metrics, and pinned widgets. A `MetricsDashboard.vue` component replaces `LogStream` in the widget order. Sub-components (`MetricCategory`, `MetricPane`, `MetricCatalog`) handle rendering. Dashboard pages are persisted to localStorage; pinned widgets use absolute positioning with drag-to-move.

**Tech Stack:** Vue 3 (Composition API + `<script setup>`), Pinia, Vitest + `@vue/test-utils`, existing CSS variables from `theme.css`

---

## Phase 1 — Store Foundation

### Task 1: Create dashboard Pinia store skeleton

**Objective:** Create the state shape and getters for the dashboard store.

**Files:**
- Create: `kube/view/src/stores/dashboard.js`

**Step 1: Write the store structure**

```js
// kube/view/src/stores/dashboard.js
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// Metric categories — each contains an ordered list of metric keys.
// A "metric" is defined by its key; the rendering component maps
// the key to a PromQL query / label / chart type.
export const METRIC_CATEGORIES = Object.freeze({
  compute: {
    label: 'Compute',
    metrics: ['cpu_usage', 'memory_usage', 'cpu_throttling', 'oom_kills'],
  },
  network: {
    label: 'Network',
    metrics: ['rx_bytes', 'tx_bytes', 'packet_drops', 'tcp_connections'],
  },
  storage: {
    label: 'Storage',
    metrics: ['disk_usage', 'disk_iops', 'pv_capacity', 'inode_usage'],
  },
  control: {
    label: 'Control Plane',
    metrics: ['api_latency', 'scheduler_queue', 'etcd_requests', 'controller_sync'],
  },
  workload: {
    label: 'Workloads',
    metrics: ['pod_restarts', 'deployment_status', 'job_completions', 'hpa_replicas'],
  },
  custom: {
    label: 'Custom',
    metrics: ['promql_1', 'promql_2'],
  },
})

// How many metrics from each category to show by default
const DEFAULT_VISIBLE = 2

// Maximum pinned widgets
const MAX_PINNED = 4

export const useDashboardStore = defineStore('dashboard', () => {
  // --- Pages (Dashboards) ---
  const pages = ref([
    { id: 'overview', label: 'Overview', active: true },
    { id: 'compute', label: 'Compute' },
    { id: 'network', label: 'Network' },
    { id: 'storage', label: 'Storage' },
  ])
  const activePageId = ref('overview')

  // --- Per-category visible count ---
  // Keyed by category id → number of metrics shown
  const visibleCounts = ref({})

  // --- Pinned widgets ---
  // Each: { id, categoryId, metricKey, x, y } (x,y = percentage 0-100)
  const pinnedWidgets = ref([])

  // --- Popup state ---
  // { categoryId, expandedMetricKey } | null
  const popupState = ref(null)

  // --- Getters ---
  const activePage = computed(() => pages.value.find(p => p.id === activePageId.value))

  const maxPinnedReached = computed(() => pinnedWidgets.value.length >= MAX_PINNED)

  function visibleForCategory(categoryId) {
    return visibleCounts.value[categoryId] ?? DEFAULT_VISIBLE
  }

  // --- Actions ---
  function setPage(pageId) {
    activePageId.value = pageId
  }

  function toggleCategory(categoryId) {
    const cat = METRIC_CATEGORIES[categoryId]
    if (!cat) return
    const current = visibleCounts.value[categoryId] ?? DEFAULT_VISIBLE
    visibleCounts.value[categoryId] = current >= cat.metrics.length ? DEFAULT_VISIBLE : cat.metrics.length
  }

  function openCatalog(categoryId) {
    popupState.value = { categoryId, expandedMetricKey: null }
  }

  function closeCatalog() {
    popupState.value = null
  }

  function expandInCatalog(metricKey) {
    if (!popupState.value) return
    popupState.value = { ...popupState.value, expandedMetricKey: metricKey }
  }

  function collapseInCatalog() {
    if (!popupState.value) return
    popupState.value = { ...popupState.value, expandedMetricKey: null }
  }

  function pinMetric(categoryId, metricKey) {
    if (pinnedWidgets.value.length >= MAX_PINNED) return
    // Default position: stack from top-left
    const count = pinnedWidgets.value.length
    pinnedWidgets.value.push({
      id: `${categoryId}:${metricKey}`,
      categoryId,
      metricKey,
      x: 2 + (count % 2) * 51,
      y: 2 + Math.floor(count / 2) * 51,
    })
  }

  function unpinMetric(widgetId) {
    pinnedWidgets.value = pinnedWidgets.value.filter(w => w.id !== widgetId)
  }

  function movePinned(widgetId, x, y) {
    const w = pinnedWidgets.value.find(w => w.id === widgetId)
    if (!w) return
    w.x = Math.max(0, Math.min(95, x))
    w.y = Math.max(0, Math.min(95, y))
  }

  return {
    pages, activePageId, visibleCounts, pinnedWidgets, popupState,
    activePage, maxPinnedReached,
    visibleForCategory,
    setPage, toggleCategory, openCatalog, closeCatalog,
    expandInCatalog, collapseInCatalog,
    pinMetric, unpinMetric, movePinned,
  }
})
```

**Step 2: Verify the file parses**

Run: `cd kube/view && node -e "import('./src/stores/dashboard.js').then(() => console.log('OK'))"`

Expected: `OK`

---

### Task 2: Write store unit tests

**Objective:** Test all critical store paths.

**Files:**
- Create: `kube/view/src/stores/__tests__/dashboard.test.js`

**Step 1: Write the tests**

```js
// kube/view/src/stores/__tests__/dashboard.test.js
import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useDashboardStore, METRIC_CATEGORIES } from '../dashboard'

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('dashboardStore', () => {
  describe('pages', () => {
    it('starts on overview page', () => {
      const store = useDashboardStore()
      expect(store.activePageId).toBe('overview')
      expect(store.activePage.label).toBe('Overview')
    })

    it('setPage switches active page', () => {
      const store = useDashboardStore()
      store.setPage('compute')
      expect(store.activePageId).toBe('compute')
    })
  })

  describe('categories', () => {
    it('defaults to 2 visible metrics per category', () => {
      const store = useDashboardStore()
      expect(store.visibleForCategory('compute')).toBe(2)
    })

    it('toggleCategory expands to all then collapses back to default', () => {
      const store = useDashboardStore()
      store.toggleCategory('compute')
      expect(store.visibleForCategory('compute')).toBe(4) // compute has 4 metrics
      store.toggleCategory('compute')
      expect(store.visibleForCategory('compute')).toBe(2)
    })
  })

  describe('pinned widgets', () => {
    it('pins up to 4 widgets', () => {
      const store = useDashboardStore()
      store.pinMetric('compute', 'cpu_usage')
      store.pinMetric('compute', 'memory_usage')
      store.pinMetric('network', 'rx_bytes')
      store.pinMetric('network', 'tx_bytes')
      expect(store.pinnedWidgets).toHaveLength(4)
      store.pinMetric('storage', 'disk_usage')
      expect(store.pinnedWidgets).toHaveLength(4) // still 4
    })

    it('unpinMetric removes widget', () => {
      const store = useDashboardStore()
      store.pinMetric('compute', 'cpu_usage')
      store.unpinMetric('compute:cpu_usage')
      expect(store.pinnedWidgets).toHaveLength(0)
    })

    it('maxPinnedReached is true at limit', () => {
      const store = useDashboardStore()
      expect(store.maxPinnedReached).toBe(false)
      for (const cat of Object.keys(METRIC_CATEGORIES)) {
        if (store.maxPinnedReached) break
        for (const m of METRIC_CATEGORIES[cat].metrics) {
          store.pinMetric(cat, m)
          if (store.maxPinnedReached) break
        }
      }
      expect(store.maxPinnedReached).toBe(true)
    })
  })
})
```

**Step 2: Run tests**

Run: `cd kube/view && npx vitest run src/stores/__tests__/dashboard.test.js`

Expected: 7+ passing, 0 failing

---

## Phase 2 — Metric Pane Component

### Task 3: Create MetricPane component

**Objective:** Render a single metric as a card with a sparkline and value.

**Files:**
- Create: `kube/view/src/components/center/MetricPane.vue`

**Step 1: Write the component**

This component takes a `metricKey` and renders either a compact or full-size view. For now, it renders placeholder data with the real PromQL integration to follow.

```vue
<!-- kube/view/src/components/center/MetricPane.vue -->
<script setup>
import { ref, computed } from 'vue'
import { useTimeSeriesMetrics } from '../../composables/useWails'
import Select from '../common/Select.vue'

const props = defineProps({
  metricKey: { type: String, required: true },
  compact: { type: Boolean, default: true },
  categoryId: { type: String, default: '' },
})

const emit = defineEmits(['pin', 'expand'])

const { queryMetrics } = useTimeSeriesMetrics()

const timeRange = ref('1h')
const data = ref(null)
const loading = ref(false)
const error = ref('')

const timeRangeOptions = [
  { value: '15m', label: '15m' },
  { value: '1h', label: '1h' },
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
]

// Metric definition registry — maps metricKey → { label, query, unit, format }
const DEFINITIONS = {
  cpu_usage:    { label: 'CPU Usage',    query: 'avg(rate(container_cpu_usage_seconds_total[5m])) by (pod) * 100', unit: '%', format: (v) => `${v.toFixed(1)}%` },
  memory_usage: { label: 'Memory Usage',  query: 'avg(container_memory_working_set_bytes) by (pod)', unit: 'bytes', format: (v) => v >= 1e9 ? `${(v/1e9).toFixed(1)} GiB` : `${(v/1e6).toFixed(0)} MiB` },
  cpu_throttling: { label: 'CPU Throttling', query: 'rate(container_cpu_cfs_throttled_seconds_total[5m]) * 100', unit: '%', format: (v) => `${v.toFixed(2)}%` },
  oom_kills:       { label: 'OOM Kills',      query: 'rate(container_oom_events_total[5m])', unit: 'count', format: (v) => v.toFixed(2) },
  rx_bytes:      { label: 'RX Bytes',      query: 'rate(container_network_receive_bytes_total[5m])', unit: 'bps', format: (v) => v >= 1e6 ? `${(v/1e6).toFixed(1)} MB/s` : `${(v/1e3).toFixed(0)} KB/s` },
  tx_bytes:      { label: 'TX Bytes',      query: 'rate(container_network_transmit_bytes_total[5m])', unit: 'bps', format: (v) => v >= 1e6 ? `${(v/1e6).toFixed(1)} MB/s` : `${(v/1e3).toFixed(0)} KB/s` },
  packet_drops:  { label: 'Packet Drops',  query: 'rate(container_network_receive_packets_dropped_total[5m]) + rate(container_network_transmit_packets_dropped_total[5m])', unit: 'pps', format: (v) => `${v.toFixed(2)} pps` },
  tcp_connections: { label: 'TCP Connections', query: 'sum(container_network_tcp_established_total)', unit: 'count', format: (v) => v.toFixed(0) },
  disk_usage:    { label: 'Disk Usage',    query: 'avg(container_fs_usage_bytes) by (pod) / avg(container_fs_limit_bytes) by (pod) * 100', unit: '%', format: (v) => `${v.toFixed(1)}%` },
  disk_iops:     { label: 'Disk IOPS',     query: 'rate(container_fs_reads_total[5m]) + rate(container_fs_writes_total[5m])', unit: 'iops', format: (v) => `${v.toFixed(0)} IOPS` },
  pv_capacity:   { label: 'PV Capacity',   query: 'sum(kube_persistentvolume_capacity_bytes)', unit: 'bytes', format: (v) => v >= 1e12 ? `${(v/1e12).toFixed(1)} TB` : `${(v/1e9).toFixed(1)} GB` },
  inode_usage:   { label: 'Inode Usage',   query: 'avg(container_fs_inodes_free) by (pod) / avg(container_fs_inodes_total) by (pod) * 100', unit: '%', format: (v) => `${v.toFixed(1)}%` },
  api_latency:     { label: 'API Latency',     query: 'histogram_quantile(0.99, rate(apiserver_request_duration_seconds_bucket[5m]))', unit: 's', format: (v) => `${(v*1000).toFixed(1)} ms` },
  scheduler_queue: { label: 'Scheduler Queue', query: 'avg(scheduler_pending_pods)', unit: 'count', format: (v) => v.toFixed(0) },
  etcd_requests:   { label: 'etcd Requests',   query: 'rate(etcd_request_duration_seconds_count[5m])', unit: 'rps', format: (v) => `${v.toFixed(1)} rps` },
  controller_sync: { label: 'Controller Sync', query: 'rate(workqueue_adds_total[5m])', unit: 'ops/s', format: (v) => `${v.toFixed(1)} ops/s` },
  pod_restarts:       { label: 'Pod Restarts',       query: 'rate(kube_pod_container_status_restarts_total[5m])', unit: 'rps', format: (v) => `${v.toFixed(3)} rps` },
  deployment_status:  { label: 'Deployment Status',  query: 'sum(kube_deployment_status_replicas_available) by (deployment) / sum(kube_deployment_spec_replicas) by (deployment) * 100', unit: '%', format: (v) => `${v.toFixed(0)}%` },
  job_completions:    { label: 'Job Completions',    query: 'rate(kube_job_complete_total[1h])', unit: 'jobs/h', format: (v) => `${v.toFixed(1)}/h` },
  hpa_replicas:       { label: 'HPA Replicas',       query: 'avg(kube_horizontalpodautoscaler_status_current_replicas)', unit: 'count', format: (v) => v.toFixed(0) },
  promql_1: { label: 'Custom Query 1', query: '', unit: '', format: (v) => String(v) },
  promql_2: { label: 'Custom Query 2', query: '', unit: '', format: (v) => String(v) },
}

const def = computed(() => DEFINITIONS[props.metricKey] || { label: props.metricKey, query: '', unit: '', format: (v) => String(v) })

async function fetchData() {
  if (!def.value.query) return
  loading.value = true
  error.value = ''
  try {
    data.value = await queryMetrics(def.value.query, timeRange.value)
    if (!data.value) error.value = 'No data'
  } catch (e) {
    error.value = e?.message || 'Query failed'
  } finally {
    loading.value = false
  }
}

// Auto-fetch on mount + timeRange change
import { onMounted, watch } from 'vue'
onMounted(fetchData)
watch(timeRange, fetchData)

function latestValue() {
  if (!data.value?.values?.length) return null
  return data.value.values.at(-1)?.[1]
}

const displayValue = computed(() => {
  const v = latestValue()
  if (v == null) return '—'
  return def.value.format(v)
})
</script>

<template>
  <div class="metric-pane" :class="{ compact, 'has-error': error }">
    <div class="mp-header">
      <span class="mp-name">{{ def.label }}</span>
      <div class="mp-actions">
        <Select v-if="!compact" v-model="timeRange" :options="timeRangeOptions" size="xs" />
        <button class="mp-btn" title="Pin to dashboard" @click.stop="emit('pin')">📌</button>
        <button v-if="compact" class="mp-btn" title="Expand" @click.stop="emit('expand')">⤢</button>
      </div>
    </div>
    <div class="mp-body">
      <div v-if="loading" class="mp-loading">Loading...</div>
      <div v-else-if="error" class="mp-error">{{ error }}</div>
      <template v-else>
        <div class="mp-value">{{ displayValue }}</div>
        <div class="mp-sparkline">
          <!-- Simple bar sparkline from recent datapoints -->
          <div v-if="data?.values?.length" class="sparkline-bars">
            <div
              v-for="(pt, i) in data.values.slice(-20)"
              :key="i"
              class="sparkline-bar"
              :style="{ height: sparkHeight(pt[1]) }"
            ></div>
          </div>
          <div v-else class="mp-no-data">No time-series data</div>
        </div>
      </template>
    </div>
  </div>
</template>

<script>
// Helper for sparkline bar heights (inline, not exported)
function sparkHeight(val) {
  if (!data?.values?.length) return '0%'
  const vals = data.values.map(v => parseFloat(v[1]) || 0)
  const max = Math.max(...vals, 1)
  return `${Math.max(5, (parseFloat(val) || 0) / max * 100)}%`
}
</script>

<style scoped>
.metric-pane {
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: var(--r);
  overflow: hidden;
  transition: border-color 0.15s;
}
.metric-pane:hover { border-color: var(--border2); }
.metric-pane.has-error { border-color: var(--red); opacity: 0.75; }

.metric-pane.compact { min-height: 100px; }
.metric-pane:not(.compact) { min-height: 250px; }

.mp-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 8px 10px; border-bottom: 1px solid var(--border);
}
.mp-name {
  font-size: 11px; font-weight: 600; color: var(--text2);
  text-transform: uppercase; letter-spacing: 0.04em;
}
.mp-actions { display: flex; align-items: center; gap: 4px; }
.mp-btn {
  background: none; border: 1px solid var(--border); border-radius: 4px;
  padding: 2px 6px; font-size: 12px; cursor: pointer; color: var(--text3);
  line-height: 1;
}
.mp-btn:hover { background: var(--bg4); color: var(--text); }

.mp-body { padding: 10px; }
.mp-value { font-size: 22px; font-weight: 500; font-family: var(--mono); color: var(--text); }
.mp-loading, .mp-error, .mp-no-data {
  font-size: 11px; color: var(--text3); font-style: italic;
}
.mp-error { color: var(--red2); }

.sparkline-bars {
  display: flex; align-items: flex-end; gap: 1px; height: 40px; margin-top: 8px;
}
.sparkline-bar {
  flex: 1; background: var(--accent); border-radius: 1px 1px 0 0;
  min-height: 2px; opacity: 0.7; transition: opacity 0.15s;
}
.sparkline-bar:hover { opacity: 1; }
</style>
```

**Step 2: Verify component mounts**

Run: `cd kube/view && npx vitest run src/stores/__tests__/dashboard.test.js`

Expected: existing tests still pass (MetricPane doesn't affect them yet)

---

## Phase 3 — Category + Catalog Components

### Task 4: Create MetricCategory component

**Objective:** Renders a category header with the default 2 metrics and a toggle/expand button.

**Files:**
- Create: `kube/view/src/components/center/MetricCategory.vue`

**Step 1: Write the component**

```vue
<!-- kube/view/src/components/center/MetricCategory.vue -->
<script setup>
import { computed } from 'vue'
import { useDashboardStore, METRIC_CATEGORIES } from '../../stores/dashboard'
import MetricPane from './MetricPane.vue'

const props = defineProps({
  categoryId: { type: String, required: true },
})

const store = useDashboardStore()

const cat = computed(() => METRIC_CATEGORIES[props.categoryId])
const visibleCount = computed(() => store.visibleForCategory(props.categoryId))
const visibleMetrics = computed(() => cat.value.metrics.slice(0, visibleCount.value))
const isExpanded = computed(() => visibleCount.value >= cat.value.metrics.length)

function onToggle() {
  store.toggleCategory(props.categoryId)
}

function onShowAll() {
  store.openCatalog(props.categoryId)
}

function onPin(metricKey) {
  store.pinMetric(props.categoryId, metricKey)
}
</script>

<template>
  <div class="metric-category">
    <div class="mc-header">
      <span class="mc-label">{{ cat.label }}</span>
      <div class="mc-actions">
        <span class="mc-count">{{ visibleCount }} / {{ cat.metrics.length }}</span>
        <button class="mc-toggle" @click="onToggle">
          {{ isExpanded ? 'Collapse' : 'Expand' }}
        </button>
        <button class="mc-show-all" @click="onShowAll">All →</button>
      </div>
    </div>
    <div class="mc-grid">
      <MetricPane
        v-for="key in visibleMetrics"
        :key="key"
        :metric-key="key"
        :category-id="categoryId"
        compact
        @pin="onPin(key)"
      />
    </div>
  </div>
</template>

<style scoped>
.metric-category { margin-bottom: 16px; }

.mc-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 8px;
}
.mc-label { font-size: 12px; font-weight: 600; color: var(--text); }
.mc-actions { display: flex; align-items: center; gap: 8px; }
.mc-count { font-size: 10px; color: var(--text3); font-family: var(--mono); }

.mc-toggle, .mc-show-all {
  background: var(--bg3); border: 1px solid var(--border); border-radius: 4px;
  padding: 2px 8px; font-size: 10px; color: var(--text2); cursor: pointer;
}
.mc-toggle:hover, .mc-show-all:hover { background: var(--bg4); color: var(--text); }

.mc-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 8px;
}
</style>
```

---

### Task 5: Create MetricCatalog popup component

**Objective:** A scrollable overlay showing all metrics in a category. Clicking a metric expands it to full popup size.

**Files:**
- Create: `kube/view/src/components/center/MetricCatalog.vue`

**Step 1: Write the component**

```vue
<!-- kube/view/src/components/center/MetricCatalog.vue -->
<script setup>
import { computed } from 'vue'
import { useDashboardStore, METRIC_CATEGORIES } from '../../stores/dashboard'
import MetricPane from './MetricPane.vue'

const store = useDashboardStore()

const cat = computed(() => {
  if (!store.popupState) return null
  return METRIC_CATEGORIES[store.popupState.categoryId]
})

const expandedKey = computed(() => store.popupState?.expandedMetricKey)

function onClose() {
  store.closeCatalog()
}

function onClickMetric(key) {
  store.expandInCatalog(key)
}

function onBack() {
  store.collapseInCatalog()
}

function onPin(metricKey) {
  store.pinMetric(store.popupState.categoryId, metricKey)
  store.closeCatalog()
}
</script>

<template>
  <Teleport to="body">
    <div v-if="cat" class="catalog-overlay" @click.self="onClose">
      <div class="catalog" :class="{ single: !!expandedKey }">
        <div class="catalog-header">
          <span class="catalog-title">
            {{ expandedKey ? cat.label + ' › ' + expandedKey : cat.label + ' — All Metrics' }}
          </span>
          <div class="catalog-header-actions">
            <button v-if="expandedKey" class="catalog-back" @click="onBack">← Back</button>
            <button class="catalog-close" @click="onClose">✕</button>
          </div>
        </div>

        <!-- Grid view: all metrics -->
        <div v-if="!expandedKey" class="catalog-grid">
          <div
            v-for="key in cat.metrics"
            :key="key"
            class="catalog-item"
            @click="onClickMetric(key)"
          >
            <MetricPane
              :metric-key="key"
              :category-id="store.popupState.categoryId"
              compact
              @pin="onPin(key)"
              @expand="onClickMetric(key)"
            />
          </div>
        </div>

        <!-- Single expanded metric -->
        <div v-else class="catalog-expanded">
          <MetricPane
            :metric-key="expandedKey"
            :category-id="store.popupState.categoryId"
            :compact="false"
            @pin="onPin(expandedKey)"
          />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.catalog-overlay {
  position: fixed; inset: 0; z-index: 100;
  background: rgba(0, 0, 0, 0.6);
  display: flex; align-items: center; justify-content: center;
}
.catalog {
  background: var(--bg2);
  border: 1px solid var(--border2);
  border-radius: 8px;
  width: min(900px, 90vw);
  max-height: 85vh;
  display: flex; flex-direction: column;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
}
.catalog.single { width: min(600px, 90vw); }

.catalog-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 16px; border-bottom: 1px solid var(--border);
}
.catalog-title { font-size: 14px; font-weight: 600; color: var(--text); }
.catalog-header-actions { display: flex; gap: 8px; }
.catalog-back, .catalog-close {
  background: var(--bg3); border: 1px solid var(--border); border-radius: 4px;
  padding: 4px 10px; font-size: 12px; cursor: pointer; color: var(--text2);
}
.catalog-back:hover, .catalog-close:hover { background: var(--bg4); }

.catalog-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 8px; padding: 16px; overflow-y: auto;
}
.catalog-item { cursor: pointer; }

.catalog-expanded {
  padding: 16px; overflow-y: auto;
}
</style>
```

---

## Phase 4 — Main Dashboard Component & Integration

### Task 6: Create MetricsDashboard container

**Objective:** The main component that replaces LogStream. Renders categories, pinned widgets, dashboard page selector, and the catalog popup.

**Files:**
- Create: `kube/view/src/components/center/MetricsDashboard.vue`
- Modify: `kube/view/src/components/center/CenterPanel.vue` (replace LogStream import + widget)

**Step 1: Write MetricsDashboard.vue**

```vue
<!-- kube/view/src/components/center/MetricsDashboard.vue -->
<script setup>
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useDashboardStore, METRIC_CATEGORIES } from '../../stores/dashboard'
import MetricCategory from './MetricCategory.vue'
import MetricPane from './MetricPane.vue'
import MetricCatalog from './MetricCatalog.vue'

const store = useDashboardStore()
const { activePageId, activePage, pinnedWidgets, popupState } = storeToRefs(store)

// Which categories to show on the current page
const pageCategories = computed(() => {
  const all = Object.keys(METRIC_CATEGORIES)
  if (activePageId.value === 'overview') return all.slice(0, 3) // compute, network, storage
  if (activePageId.value === 'compute') return ['compute', 'workload', 'control']
  if (activePageId.value === 'network') return ['network']
  if (activePageId.value === 'storage') return ['storage']
  return all
})

function getCategoryForMetric(metricKey) {
  for (const [catId, cat] of Object.entries(METRIC_CATEGORIES)) {
    if (cat.metrics.includes(metricKey)) return catId
  }
  return ''
}
</script>

<template>
  <div class="metrics-dashboard">
    <!-- Dashboard page selector -->
    <div class="md-pages">
      <button
        v-for="page in store.pages"
        :key="page.id"
        class="md-page-btn"
        :class="{ active: page.id === activePageId }"
        @click="store.setPage(page.id)"
      >
        {{ page.label }}
        <span v-if="pinnedWidgets.length && page.id === activePageId" class="md-pin-badge">
          {{ pinnedWidgets.length }}
        </span>
      </button>
    </div>

    <!-- Pinned widgets layer (positioned absolutely) -->
    <div v-if="pinnedWidgets.length" class="md-pinned">
      <div
        v-for="w in pinnedWidgets"
        :key="w.id"
        class="md-pinned-widget"
        :style="{ left: w.x + '%', top: w.y + '%' }"
        @mousedown="startDrag($event, w.id)"
      >
        <div class="md-pinned-header">
          <span class="md-pinned-name">{{ w.metricKey }}</span>
          <button class="md-pinned-close" @click.stop="store.unpinMetric(w.id)">✕</button>
        </div>
        <MetricPane
          :metric-key="w.metricKey"
          :category-id="getCategoryForMetric(w.metricKey)"
          compact
          @pin="store.unpinMetric(w.id)"
        />
      </div>
    </div>

    <!-- Category sections -->
    <div class="md-categories">
      <MetricCategory
        v-for="catId in pageCategories"
        :key="catId"
        :category-id="catId"
      />
    </div>

    <!-- Catalog popup -->
    <MetricCatalog v-if="popupState" />
  </div>
</template>

<script>
// Drag logic
let dragWidgetId = null
let dragStartX = 0
let dragStartY = 0
let dragOrigX = 0
let dragOrigY = 0
let dragEl = null

function startDrag(e, widgetId) {
  dragWidgetId = widgetId
  dragEl = e.currentTarget
  const w = store.pinnedWidgets.find(w => w.id === widgetId)
  if (!w) return
  dragStartX = e.clientX
  dragStartY = e.clientY
  dragOrigX = w.x
  dragOrigY = w.y
  dragEl.style.zIndex = '50'
  document.addEventListener('mousemove', onDrag)
  document.addEventListener('mouseup', stopDrag)
  e.preventDefault()
}

function onDrag(e) {
  if (!dragWidgetId || !dragEl) return
  const dx = ((e.clientX - dragStartX) / dragEl.parentElement.clientWidth) * 100
  const dy = ((e.clientY - dragStartY) / dragEl.parentElement.clientHeight) * 100
  store.movePinned(dragWidgetId, dragOrigX + dx, dragOrigY + dy)
}

function stopDrag() {
  if (dragEl) dragEl.style.zIndex = ''
  dragWidgetId = null
  dragEl = null
  document.removeEventListener('mousemove', onDrag)
  document.removeEventListener('mouseup', stopDrag)
}
</script>

<style scoped>
.metrics-dashboard {
  position: relative;
}

/* Page selector */
.md-pages {
  display: flex; gap: 4px; margin-bottom: 14px;
}
.md-page-btn {
  background: var(--bg3); border: 1px solid var(--border); border-radius: 4px;
  padding: 4px 12px; font-size: 11px; font-weight: 500;
  color: var(--text2); cursor: pointer; display: flex; align-items: center; gap: 6px;
}
.md-page-btn:hover { background: var(--bg4); color: var(--text); }
.md-page-btn.active { background: var(--accent); color: #fff; border-color: var(--accent); }
.md-pin-badge {
  background: var(--amber); color: #000; font-size: 9px; font-weight: 700;
  padding: 0 4px; border-radius: 3px; line-height: 1.4;
}

/* Pinned widgets */
.md-pinned {
  position: relative; min-height: 80px; margin-bottom: 12px;
}
.md-pinned-widget {
  position: absolute; width: 48%; cursor: grab;
  user-select: none; z-index: 10;
}
.md-pinned-widget:active { cursor: grabbing; }
.md-pinned-header {
  display: flex; align-items: center; justify-content: space-between;
  background: var(--bg4); border: 1px solid var(--border); border-bottom: none;
  border-radius: 4px 4px 0 0; padding: 4px 8px;
}
.md-pinned-name {
  font-size: 10px; font-weight: 600; color: var(--text2);
  text-transform: uppercase; letter-spacing: 0.04em;
}
.md-pinned-close {
  background: none; border: none; color: var(--text3); cursor: pointer;
  font-size: 10px; padding: 0 2px;
}
.md-pinned-close:hover { color: var(--red); }

.md-categories {
  display: flex; flex-direction: column; gap: 4px;
}
</style>
```

**Step 2: Integrate into CenterPanel.vue**

Modify `kube/view/src/components/center/CenterPanel.vue`:

```diff
-import LogStream from './LogStream.vue'
+import MetricsDashboard from './MetricsDashboard.vue'

-const widgetOrder = ref(['metrics', 'alerts', 'logs'])
+const widgetOrder = ref(['metrics', 'alerts', 'dashboards'])

-<LogStream
-  v-else-if="widget === 'logs'"
-  :alerts="alerts"
-  :externalLines="logLines"
-/>
+<MetricsDashboard v-else-if="widget === 'dashboards'" />
```

**Step 3: Remove unused prop passthrough**

In CenterPanel, `logLines` prop and `LogStream` import can be removed once no other component references them. But App.vue still passes `logLines` — keep the prop for now, just stop consuming it.

---

## Phase 5 — Cleanup & Polish

### Task 7: Remove LogStream references

**Objective:** Clean up the LogStream import and dead code path.

**Files:**
- Modify: `kube/view/src/components/center/CenterPanel.vue`
- (Keep file) `kube/view/src/components/center/LogStream.vue` — delete only after confirming nothing else imports it

**Step 1: Verify no other consumers**

Run: `cd kube/view && grep -r "LogStream" src/ --include="*.vue" --include="*.js" --include="*.ts"`

Expected: Only `CenterPanel.vue` imports it.

**Step 2: Remove import and delete widget case**

Remove the `import LogStream` line and the `v-else-if="widget === 'logs'"` block from CenterPanel.

**Step 3: Remove logLines from CenterPanel props**

```diff
-  logLines: { type: Array, default: () => [] },
```

**Step 4: Verify build**

Run: `cd kube/view && npx vite build`

Expected: No errors, no warnings about missing imports.

---

### Task 8: Add MetricsDashboard component test

**Objective:** Basic mounting test for the dashboard.

**Files:**
- Create: `kube/view/src/__tests__/components/MetricsDashboard.test.js`

**Step 1: Write test**

```js
import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import MetricsDashboard from '../../components/center/MetricsDashboard.vue'
import { useDashboardStore } from '../../stores/dashboard'

// Mock MetricPane — avoid PromQL calls
const MockMetricPane = {
  name: 'MetricPane',
  template: '<div class="mock-pane">{{ metricKey }}</div>',
  props: ['metricKey', 'compact', 'categoryId'],
  emits: ['pin', 'expand'],
}

describe('MetricsDashboard', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders page selector', () => {
    const wrapper = mount(MetricsDashboard, {
      global: {
        stubs: { MetricPane: MockMetricPane, MetricCatalog: true },
      },
    })
    const buttons = wrapper.findAll('.md-page-btn')
    expect(buttons).toHaveLength(4)
    expect(buttons[0].text()).toContain('Overview')
  })

  it('renders categories for overview page', () => {
    const wrapper = mount(MetricsDashboard, {
      global: {
        stubs: { MetricPane: MockMetricPane, MetricCategory: true, MetricCatalog: true },
      },
    })
    expect(wrapper.findAllComponents({ name: 'MetricCategory' })).toHaveLength(3)
  })
})
```

**Step 2: Run test**

Run: `cd kube/view && npx vitest run src/__tests__/components/MetricsDashboard.test.js`

Expected: 2 passing

---

## Phase 6 — Verification

### Task 9: Full integration verification

**Objective:** Run the full test suite and ensure no regressions.

**Step 1: Run all frontend tests**

```bash
cd kube/view && npx vitest run
```

Expected: All existing tests pass; new tests pass. No regressions.

**Step 2: Build the frontend**

```bash
cd kube/view && npx vite build
```

Expected: Clean build, no warnings.

**Step 3: Commit**

```bash
git add kube/view/src/stores/dashboard.js
git add kube/view/src/stores/__tests__/dashboard.test.js
git add kube/view/src/components/center/MetricPane.vue
git add kube/view/src/components/center/MetricCategory.vue
git add kube/view/src/components/center/MetricCatalog.vue
git add kube/view/src/components/center/MetricsDashboard.vue
git add kube/view/src/components/center/CenterPanel.vue
git add kube/view/src/__tests__/components/MetricsDashboard.test.js
git commit -m "feat(monitoring): replace Live Log Stream with metrics dashboard

- New Pinia dashboard store with pages, categories, and pinned widgets
- MetricPane renders individual metrics with sparklines
- MetricCategory groups metrics, defaults to 2 visible
- MetricCatalog popup shows all metrics in a category
- MetricsDashboard replaces LogStream in Cluster Alerts view
- Up to 4 metrics can be pinned as draggable widgets"
```

---

## Summary

| Phase | Tasks | New Files | Modified Files |
|-------|-------|-----------|----------------|
| 1 — Store Foundation | 2 | 2 | 0 |
| 2 — Metric Pane | 1 | 1 | 0 |
| 3 — Category + Catalog | 2 | 2 | 0 |
| 4 — Dashboard + Integration | 1 | 1 | 1 |
| 5 — Cleanup | 1 | 0 | 1 |
| 6 — Verification | 1 | 1 | 0 |
| **Total** | **8** | **7** | **2** |

### Files to create
1. `kube/view/src/stores/dashboard.js`
2. `kube/view/src/stores/__tests__/dashboard.test.js`
3. `kube/view/src/components/center/MetricPane.vue`
4. `kube/view/src/components/center/MetricCategory.vue`
5. `kube/view/src/components/center/MetricCatalog.vue`
6. `kube/view/src/components/center/MetricsDashboard.vue`
7. `kube/view/src/__tests__/components/MetricsDashboard.test.js`

### Files to modify
8. `kube/view/src/components/center/CenterPanel.vue` — replace LogStream with MetricsDashboard
9. `kube/view/src/components/center/CenterPanel.vue` — remove logLines prop

### Notes
- The `logLines` prop in App.vue and CenterPanel can be fully removed in a follow-up after confirming no other consumers
- PromQL queries in MetricPane are defaults — they'll need tuning against actual Prometheus data
- The `sparkHeight` helper in MetricPane is in a separate `<script>` block because `<script setup>` doesn't support non-hoisted code between setup and template — in production this should be a computed or a function in the setup block
