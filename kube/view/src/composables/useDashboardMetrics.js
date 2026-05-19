import { ref, reactive, computed, watch } from 'vue'
import { callGo } from './useBridge'

// ── Metric Categories & Definitions ──────────────────────────────
// Each category has an id, label, and array of metric definitions.
// Each metric def has: id, label, query (PromQL or special key), 
// format function, and value range for coloring.

export function formatBytes(bytes) {
  if (bytes == null || isNaN(bytes)) return '—'
  if (bytes >= 1073741824) return `${(bytes / 1073741824).toFixed(1)} Gi`
  if (bytes >= 1048576) return `${(bytes / 1048576).toFixed(0)} Mi`
  return `${(bytes / 1024).toFixed(0)} Ki`
}

export function formatCPU(millis) {
  if (millis == null || isNaN(millis)) return '—'
  if (millis >= 1000) return `${(millis / 1000).toFixed(1)} cores`
  return `${millis}m`
}

export function formatPct(v) {
  if (v == null || isNaN(v)) return '—'
  return `${Number(v).toFixed(1)}%`
}

export function formatCount(v) {
  if (v == null || isNaN(v)) return '—'
  return String(Math.round(v))
}

export const METRIC_CATEGORIES = Object.freeze([
  {
    id: 'pod-health',
    label: 'Pod Health',
    metrics: [
      {
        id: 'pod-health-pct', label: 'Healthy Pods', query: 'podHealthPct',
        format: formatPct, colorRange: { green: 90, amber: 70 },
        sparklineQuery: 'pod_health', 
      },
      {
        id: 'pods-running', label: 'Running Pods', query: 'podsRunning',
        format: formatCount, colorRange: {},
        sparklineQuery: 'pod_health',
      },
      {
        id: 'pods-pending', label: 'Pending Pods', query: 'podsPending',
        format: formatCount, colorRange: {},
        sparklineQuery: 'pod_health',
      },
      {
        id: 'pods-failed', label: 'Failed Pods', query: 'podsFailed',
        format: formatCount, colorRange: { red: 1, amber: 0 },
        sparklineQuery: 'pod_health',
      },
      {
        id: 'restart-count', label: 'Restarts (24h)', query: 'restartCount',
        format: formatCount, colorRange: { red: 20, amber: 5 },
        sparklineQuery: 'pod_health',
      },
    ],
    defaultToggled: ['pod-health-pct', 'restart-count'],
  },
  {
    id: 'cpu',
    label: 'CPU',
    metrics: [
      {
        id: 'cpu-total', label: 'Total CPU', query: 'totalCpuMillis',
        format: formatCPU, colorRange: {},
        sparklineQuery: 'cpu',
      },
      {
        id: 'cpu-util-pct', label: 'CPU Utilization', query: 'cpu_util',
        format: formatPct, colorRange: { amber: 70, red: 90 },
        sparklineQuery: 'cpu',
      },
      {
        id: 'cpu-throttle', label: 'CPU Throttling', query: 'cpu_throttle',
        format: formatPct, colorRange: { amber: 10, red: 25 },
        sparklineQuery: 'cpu',
      },
      {
        id: 'cpu-node-max', label: 'Hottest Node CPU', query: 'cpu_node_max',
        format: formatPct, colorRange: { amber: 70, red: 90 },
        sparklineQuery: 'cpu',
      },
    ],
    defaultToggled: ['cpu-total', 'cpu-util-pct'],
  },
  {
    id: 'memory',
    label: 'Memory',
    metrics: [
      {
        id: 'mem-total', label: 'Total Memory', query: 'totalMemoryBytes',
        format: formatBytes, colorRange: {},
        sparklineQuery: 'memory',
      },
      {
        id: 'mem-util-pct', label: 'Memory Utilization', query: 'memory_util',
        format: formatPct, colorRange: { amber: 70, red: 90 },
        sparklineQuery: 'memory',
      },
      {
        id: 'mem-pressure', label: 'Memory Pressure', query: 'memory_pressure',
        format: formatPct, colorRange: { amber: 50, red: 80 },
        sparklineQuery: 'memory',
      },
      {
        id: 'mem-node-max', label: 'Hottest Node Mem', query: 'memory_node_max',
        format: formatPct, colorRange: { amber: 70, red: 90 },
        sparklineQuery: 'memory',
      },
    ],
    defaultToggled: ['mem-total', 'mem-util-pct'],
  },
  {
    id: 'network',
    label: 'Network',
    metrics: [
      {
        id: 'error-rate', label: 'Error Rate', query: 'errorRate',
        format: formatPct, colorRange: { amber: 1, red: 5 },
        sparklineQuery: 'network',
      },
      {
        id: 'warning-events', label: 'Warnings (30m)', query: 'warningEvents',
        format: formatCount, colorRange: { amber: 5, red: 20 },
        sparklineQuery: 'network',
      },
      {
        id: 'net-rx', label: 'Network RX', query: 'network_receive',
        format: formatBytes, colorRange: {},
        sparklineQuery: 'network',
      },
      {
        id: 'net-tx', label: 'Network TX', query: 'network_transmit',
        format: formatBytes, colorRange: {},
        sparklineQuery: 'network',
      },
    ],
    defaultToggled: ['error-rate', 'warning-events'],
  },
  {
    id: 'latency',
    label: 'Latency & SLO',
    metrics: [
      {
        id: 'p99-latency', label: 'P99 Latency', query: 'p99Latency',
        format: (v) => v || '—', colorRange: {},
        sparklineQuery: 'latency',
      },
      {
        id: 'slo-status', label: 'SLO Status', query: 'sloStatus',
        format: (v) => v || '—', colorRange: {},
        sparklineQuery: 'latency',
      },
      {
        id: 'api-errors', label: 'API 5xx Rate', query: 'api_5xx',
        format: formatPct, colorRange: { amber: 0.5, red: 2 },
        sparklineQuery: 'latency',
      },
      {
        id: 'api-p99', label: 'API P99', query: 'api_p99',
        format: (v) => v ? `${v}ms` : '—', colorRange: {},
        sparklineQuery: 'latency',
      },
    ],
    defaultToggled: ['p99-latency', 'slo-status'],
  },
])

// ── Default dashboard ─────────────────────────────────────────────
const DEFAULT_DASHBOARD = {
  id: 'default',
  name: 'Default',
  categories: Object.fromEntries(
    METRIC_CATEGORIES.map(c => [c.id, [...c.defaultToggled]])
  ),
  widgets: [
    { metricId: 'cpu-util-pct', x: 0, y: 0, w: 1, h: 1 },
    { metricId: 'mem-util-pct', x: 1, y: 0, w: 1, h: 1 },
  ],
}

// ── Persistence key ──────────────────────────────────────────────
const PERSIST_KEY = 'argus.dashboards.v1'

function loadDashboards() {
  try {
    const raw = localStorage.getItem(PERSIST_KEY)
    if (!raw) return [structuredClone(DEFAULT_DASHBOARD)]
    const arr = JSON.parse(raw)
    if (!Array.isArray(arr) || arr.length === 0) return [structuredClone(DEFAULT_DASHBOARD)]
    return arr
  } catch {
    return [structuredClone(DEFAULT_DASHBOARD)]
  }
}

function saveDashboards(dashboards) {
  try {
    localStorage.setItem(PERSIST_KEY, JSON.stringify(dashboards))
  } catch { /* quota exceeded — silently ignore */ }
}

// ── Metric value color ───────────────────────────────────────────
function metricColor(metricDef, value) {
  const range = metricDef.colorRange || {}
  const v = Number(value)
  if (isNaN(v)) return 'norm'
  if (range.red != null && v >= range.red) return 'crit'
  if (range.amber != null && v >= range.amber) return 'warn'
  if (range.green != null && v < range.green) return 'crit'
  return 'up'
}

// ── Composable ───────────────────────────────────────────────────
export function useDashboardMetrics() {
  // Dashboard list + active index
  const dashboards = ref(loadDashboards())
  const activeIndex = ref(0)
  const editMode = ref(false)

  const activeDashboard = computed(() => dashboards.value[activeIndex.value] || dashboards.value[0])

  // Cached cluster metrics (from GetMetrics polling)
  const clusterMetrics = ref(null)

  // Sparkline data: metricId → array of {time, value}
  const sparklines = reactive({})

  // Loading state per metric
  const loadingSparklines = reactive({})

  // ── Cluster metrics fetching ──────────────────────────────────
  async function fetchClusterMetrics() {
    try {
      const result = await callGo('GetMetrics')
      if (result) clusterMetrics.value = result
    } catch (e) {
      console.warn('[dashboards] GetMetrics failed:', e)
    }
  }

  // ── Metric value resolution ───────────────────────────────────
  // Resolves a metric definition to its current value.
  // query can be: a key in ClusterMetrics, a derived PromQL, or a special key.
  function getMetricValue(metricDef) {
    const cm = clusterMetrics.value
    if (!cm) return null
    // Direct ClusterMetrics fields
    if (metricDef.query in cm) return cm[metricDef.query]
    // Derived PromQL queries handled externally via sparklines
    return null
  }

  // ── Sparkline fetching ────────────────────────────────────────
  async function fetchSparkline(metricId, metricDef) {
    if (loadingSparklines[metricId]) return
    loadingSparklines[metricId] = true
    try {
      const range = '1h'
      const query = metricDef.sparklineQuery || metricDef.query
      const data = await callGo('QueryTimeSeriesMetrics', query, range)
      if (Array.isArray(data)) {
        sparklines[metricId] = data.map((v, i) => ({ time: i, value: v }))
      }
    } catch (e) {
      console.warn(`[dashboards] sparkline ${metricId} failed:`, e)
    } finally {
      loadingSparklines[metricId] = false
    }
  }

  // ── Category helpers ──────────────────────────────────────────
  function getCategoryToggled(categoryId) {
    return activeDashboard.value.categories[categoryId] || []
  }

  function toggleCategoryMetric(categoryId, metricId) {
    const dash = dashboards.value[activeIndex.value]
    if (!dash) return
    const cat = METRIC_CATEGORIES.find(c => c.id === categoryId)
    if (!cat) return
    const toggled = dash.categories[categoryId] || []
    const idx = toggled.indexOf(metricId)
    if (idx >= 0) {
      // Remove it — cycle to the next available metric, *excluding* the
      // one we just removed. Otherwise find() picks the just-removed id
      // back up on the next iteration and the toggle has no visible effect.
      toggled.splice(idx, 1)
      const next = cat.metrics.find(m => m.id !== metricId && !toggled.includes(m.id))
      if (next) toggled.push(next.id)
    }
    // Always keep exactly 2 toggled.
    while (toggled.length < 2) {
      const next = cat.metrics.find(m => !toggled.includes(m.id))
      if (!next) break
      toggled.push(next.id)
    }
    dash.categories[categoryId] = toggled.slice(0, 2)
    persistDashboards()
  }

  // ── Widget management ─────────────────────────────────────────
  function addWidget(metricId) {
    const dash = dashboards.value[activeIndex.value]
    if (!dash || dash.widgets.length >= 4) return false
    // Find a free position
    const usedPositions = new Set(dash.widgets.map(w => `${w.x},${w.y}`))
    let x = 0, y = 0
    while (usedPositions.has(`${x},${y}`)) {
      x++
      if (x > 3) { x = 0; y++ }
    }
    dash.widgets.push({ metricId, x, y, w: 1, h: 1 })
    persistDashboards()
    return true
  }

  function removeWidget(metricId) {
    const dash = dashboards.value[activeIndex.value]
    if (!dash) return
    dash.widgets = dash.widgets.filter(w => w.metricId !== metricId)
    persistDashboards()
  }

  function moveWidget(metricId, x, y) {
    const dash = dashboards.value[activeIndex.value]
    if (!dash) return
    const widget = dash.widgets.find(w => w.metricId === metricId)
    if (widget) { widget.x = x; widget.y = y }
    persistDashboards()
  }

  // ── Dashboard CRUD ────────────────────────────────────────────
  function createDashboard(name) {
    const newDash = {
      id: `dash-${Date.now()}`,
      name: name || `Dashboard ${dashboards.value.length + 1}`,
      categories: Object.fromEntries(
        METRIC_CATEGORIES.map(c => [c.id, [...c.defaultToggled]])
      ),
      widgets: [],
    }
    dashboards.value.push(newDash)
    activeIndex.value = dashboards.value.length - 1
    persistDashboards()
  }

  function deleteDashboard(index) {
    if (dashboards.value.length <= 1) return
    dashboards.value.splice(index, 1)
    if (activeIndex.value >= dashboards.value.length) {
      activeIndex.value = dashboards.value.length - 1
    }
    persistDashboards()
  }

  function renameDashboard(index, name) {
    if (!dashboards.value[index]) return
    dashboards.value[index].name = name
    persistDashboards()
  }

  function persistDashboards() {
    saveDashboards(dashboards.value)
  }

  // ── Find a metric definition by id ────────────────────────────
  function findMetric(metricId) {
    for (const cat of METRIC_CATEGORIES) {
      const m = cat.metrics.find(mm => mm.id === metricId)
      if (m) return { category: cat, metric: m }
    }
    return null
  }

  // ── Refresh all sparklines for visible widgets ────────────────
  async function refreshSparklines() {
    const dash = activeDashboard.value
    if (!dash) return
    const metricIds = new Set()
    // Widget metrics
    for (const w of dash.widgets) metricIds.add(w.metricId)
    // Toggled category metrics
    for (const [catId, toggled] of Object.entries(dash.categories)) {
      for (const mid of toggled) metricIds.add(mid)
    }
    const promises = []
    for (const mid of metricIds) {
      const found = findMetric(mid)
      if (found) promises.push(fetchSparkline(mid, found.metric))
    }
    await Promise.allSettled(promises)
  }

  return {
    // State
    dashboards,
    activeIndex,
    activeDashboard,
    editMode,
    clusterMetrics,
    sparklines,
    loadingSparklines,
    // Categories
    METRIC_CATEGORIES,
    // Dashboard CRUD
    createDashboard,
    deleteDashboard,
    renameDashboard,
    // Category toggling
    getCategoryToggled,
    toggleCategoryMetric,
    // Widgets
    addWidget,
    removeWidget,
    moveWidget,
    // Metrics
    getMetricValue,
    findMetric,
    metricColor,
    fetchSparkline,
    refreshSparklines,
    fetchClusterMetrics,
  }
}
