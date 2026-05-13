<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useTimeSeriesMetrics, callGo } from '../../composables/useWails'
import Select from '../common/Select.vue'

const { queryMetrics } = useTimeSeriesMetrics()

function isPromQLQuery(q) {
  if (!q || typeof q !== 'string') return false
  return q.includes('{') || q.includes('[') || /^(rate|sum|avg|min|max|count|histogram|increase|delta|irate|idelta)\(/.test(q.trim())
}

function timeRangeToDuration(tr) {
  if (!tr) return '1h'
  const n = tr.toLowerCase()
  if (n.includes('15')) return '15m'
  if (n.includes('1 hour') || n.includes('1h')) return '1h'
  if (n.includes('6')) return '6h'
  if (n.includes('24') || n.includes('day')) return '24h'
  if (n.includes('7 day') || n.includes('week')) return '168h'
  return '1h'
}

const timeRange = ref('Last 1 hour')
const isLive = ref(false)
let liveInterval = null

// ── Cluster Overview persistence ─────────────────────────────────
// Panel layout + custom panels survive reload. Without this every
// "Add Panel" the user creates was thrown away on the next launch,
// and editing a panel's PromQL was a transient session-only change.
const PERSIST_KEY = 'argus.metricsExplorer.v1'

// Namespace + timeRange selections are part of the persisted state so
// switching between dashboards (or reloading) preserves the user's
// scope. namespace='' = "All namespaces".
const selectedNamespace = ref('')
const availableNamespaces = ref([])
const namespacesLoading = ref(false)
const namespacesError = ref('')

function loadPersistedState() {
  try {
    const raw = localStorage.getItem(PERSIST_KEY)
    if (!raw) return null
    const p = JSON.parse(raw)
    if (!p || typeof p !== 'object') return null
    return p
  } catch { return null }
}
function persistState() {
  try {
    localStorage.setItem(PERSIST_KEY, JSON.stringify({
      timeRange: timeRange.value,
      namespace: selectedNamespace.value,
      // Strip volatile per-panel state (data arrays, loading, error)
      // — keep only the user-authored definition fields. Reloading
      // refetches data anyway.
      panels: panels.value.map(p => ({
        id: p.id,
        type: p.type,
        title: p.title,
        query: p.query,
        color: p.color,
        bg: p.bg,
        span: p.span,
      })),
    }))
  } catch { /* best-effort */ }
}

// Load the cluster's real namespaces so the selector reflects what
// the user actually has — not the previous hard-coded list of
// kube-system/default/all.
async function loadNamespaces() {
  namespacesLoading.value = true
  namespacesError.value = ''
  try {
    const res = await callGo('ListAllNamespaces')
    if (Array.isArray(res)) {
      availableNamespaces.value = res
        .map(ns => typeof ns === 'string' ? ns : (ns?.name || ''))
        .filter(Boolean)
        .sort()
    } else {
      availableNamespaces.value = []
    }
  } catch (e) {
    namespacesError.value = e?.message || String(e)
    availableNamespaces.value = []
  } finally {
    namespacesLoading.value = false
  }
}

function toggleLive() {
  isLive.value = !isLive.value
  if (isLive.value) {
    refreshAll(true)
    liveInterval = setInterval(() => {
      refreshAll(true)
    }, 1500)
  } else {
    if (liveInterval) clearInterval(liveInterval)
  }
}

function pointsToPath(data, width, height) {
  if (!data.length) return ''
  const step = width / (data.length - 1)
  const d = data.map((val, i) => {
    const y = height - (val / 100 * height)
    const x = i * step
    return `${i === 0 ? 'M' : 'L'} ${x} ${y}`
  })
  return d.join(' ')
}

function pointsToArea(data, width, height) {
  const line = pointsToPath(data, width, height)
  return `${line} L ${width} ${height} L 0 ${height} Z`
}

// Format helpers for computed values.
function fmtPct(data) {
  if (!data || !data.length) return '—'
  const avg = data.reduce((a, b) => a + b, 0) / data.length
  return `${avg.toFixed(1)}%`
}

function fmtBytes(data, baseGi = 16) {
  if (!data || !data.length) return '—'
  const avgPct = data.reduce((a, b) => a + b, 0) / data.length
  const gb = (avgPct / 100) * baseGi
  return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(gb * 1024).toFixed(0)} MB`
}

function fmtRate(data) {
  if (!data || !data.length) return '—'
  const avg = data.reduce((a, b) => a + b, 0) / data.length
  return `${avg.toFixed(0)} Mbps`
}

const DEFAULT_PANELS = [
  {
    id: 1, type: 'area', title: 'CPU Utilization',
    query: 'cpu',
    color: 'var(--accent)', bg: 'rgba(79, 142, 247, 0.15)',
    span: 1,
  },
  {
    id: 2, type: 'area', title: 'Memory Usage',
    query: 'memory',
    color: 'var(--purple)', bg: 'rgba(167, 139, 250, 0.15)',
    span: 1,
  },
  {
    id: 3, type: 'network', title: 'Network I/O',
    query: 'network_receive',
    span: 2,
  },
  {
    id: 4, type: 'stat', title: 'Cluster Health',
    query: 'cluster_health',
    span: 1,
  },
  {
    id: 5, type: 'gauge', title: 'Active Pods',
    query: 'count(kube_pod_info)',
    span: 1,
  },
]

function buildPanel(seed) {
  return {
    ...seed,
    data: [],
    rxData: [],
    txData: [],
    val: '—',
    limit: '—',
    gaugePct: 0,
    reads: '—',
    writes: '—',
    editing: false,
    loading: true,
    error: null,
  }
}

function initialPanels() {
  const persisted = loadPersistedState()
  const seeds = (persisted?.panels && Array.isArray(persisted.panels) && persisted.panels.length)
    ? persisted.panels
    : DEFAULT_PANELS
  return seeds.map(buildPanel)
}

const panels = ref(initialPanels())

// Apply persisted timeRange + namespace on boot.
{
  const persisted = loadPersistedState()
  if (persisted?.timeRange) timeRange.value = persisted.timeRange
  if (persisted?.namespace) selectedNamespace.value = persisted.namespace
}

async function refreshPanelData(p, isBackground = false) {
  if (!isBackground) p.loading = true
  p.error = null
  try {
    if (p.type === 'area' || p.type === 'line' || p.type === 'bar') {
      let data
      if (isPromQLQuery(p.query)) {
        data = await callGo('QueryPromQL', p.query, timeRangeToDuration(timeRange.value))
      } else {
        data = await queryMetrics(p.query, timeRange.value, selectedNamespace.value)
      }
      if (data && data.length) {
        p.data = data
        // Compute display value from real data.
        if (p.query.includes('memory') || p.query.includes('mem')) {
          p.val = fmtBytes(data)
        } else {
          p.val = fmtPct(data)
        }
      } else {
        p.data = []
        p.val = '—'
        p.error = 'No data available'
      }
    } else if (p.type === 'network') {
      const rx = await queryMetrics(p.query, timeRange.value, selectedNamespace.value)
      const tx = await queryMetrics('network_transmit', timeRange.value, selectedNamespace.value)
      p.rxData = rx && rx.length ? rx : []
      p.txData = tx && tx.length ? tx : []
      if (!p.rxData.length && !p.txData.length) {
        p.error = 'No network data available'
      }
    } else if (p.type === 'stat' || p.type === 'gauge') {
      // Fetch real cluster metrics from GetMetrics.
      const m = await callGo('GetMetrics')
      if (m) {
        if (p.type === 'stat') {
          p.reads = m.restartCount != null ? m.restartCount.toLocaleString() : '0'
          p.writes = m.warningEvents != null ? m.warningEvents.toLocaleString() : '0'
          // Override labels for real data.
          p.statLabel1 = 'Restarts'
          p.statLabel2 = 'Warning Events'
        } else if (p.type === 'gauge') {
          p.val = m.podsRunning != null ? String(m.podsRunning) : '0'
          p.limit = m.podsTotal != null ? String(m.podsTotal) : '0'
          p.gaugePct = m.podsTotal > 0 ? (m.podsRunning / m.podsTotal) : 0
        }
      } else {
        p.error = 'Failed to fetch cluster metrics'
      }
    }
  } catch (err) {
    console.error(`Failed to fetch metrics for ${p.title}:`, err)
    p.error = err?.message || 'Fetch failed'
    if (p.type === 'area' || p.type === 'line' || p.type === 'bar') { p.data = []; p.val = '—' }
    if (p.type === 'network') { p.rxData = []; p.txData = [] }
  } finally {
    if (!isBackground) p.loading = false
  }
}

async function refreshAll(isBackground = false) {
  await Promise.all(panels.value.map(p => refreshPanelData(p, isBackground)))
}

onMounted(() => {
  loadNamespaces()
  refreshAll()
})

onUnmounted(() => {
  if (liveInterval) clearInterval(liveInterval)
})

// timeRange + namespace persist + drive a refetch on change so the
// graphs match the visible parameters.
watch(timeRange, () => { persistState(); refreshAll() })
watch(selectedNamespace, () => { persistState(); refreshAll() })

// Persist whenever a panel's *user-authored* fields change. Use
// flush:'post' so we batch frequent edits (slider drags, typing).
// We deep-watch so changes inside individual panels (rename, query
// edit, span change, reorder) survive reload.
watch(
  panels,
  () => { persistState() },
  { deep: true, flush: 'post' },
)

function addPanel() {
  panels.value.push({
    id: Date.now(), type: 'area', title: 'New Custom Metric',
    query: 'cpu',
    color: 'var(--teal)', bg: 'rgba(45, 212, 191, 0.15)',
    data: [], editing: true, span: 1, isNew: true, loading: false, error: null
  })
}

function savePanel(p) {
  p.editing = false
  delete p.isNew
  refreshPanelData(p)
}

function deletePanel(p) {
  panels.value = panels.value.filter(panel => panel.id !== p.id)
}

function cancelEdit(p) {
  if (p.isNew) {
    deletePanel(p)
  } else {
    p.editing = false
  }
}

function generateQuery(p) {
  if (!p.aiPrompt) return
  const prompt = p.aiPrompt.toLowerCase()
  if (prompt.includes('memory') || prompt.includes('ram')) {
    p.query = 'memory'
  } else if (prompt.includes('cpu')) {
    p.query = 'cpu'
  } else if (prompt.includes('network') || prompt.includes('traffic')) {
    p.query = 'network_receive'
  } else if (prompt.includes('node')) {
    p.query = 'node_cpu'
  } else {
    p.query = 'cpu'
  }
  p.showAI = false
  p.aiPrompt = ''
}

// Gauge arc — compute SVG arc endpoint from percentage (0–1).
function gaugeArc(pct) {
  if (pct <= 0) return 'M 10 50 A 40 40 0 0 1 10 50'
  const clamp = Math.min(pct, 1)
  // Arc goes from (-170°) to (−10°), so 160° total.
  const angle = -170 + clamp * 160
  const rad = angle * Math.PI / 180
  const x = 50 + 40 * Math.cos(rad)
  const y = 50 + 40 * Math.sin(rad)
  const largeArc = clamp > 0.5 ? 1 : 0
  return `M 10 50 A 40 40 0 ${largeArc} 1 ${x.toFixed(1)} ${y.toFixed(1)}`
}

function gaugeColor(pct) {
  if (pct >= 0.9) return 'var(--green)'
  if (pct >= 0.7) return 'var(--amber, #f5a623)'
  return 'var(--red, #f05454)'
}

const expandedPanel = ref(null)

function toggleExpand(p) {
  if (expandedPanel.value === p.id) {
    expandedPanel.value = null
  } else {
    expandedPanel.value = p.id
  }
}

let draggedIndex = null

function onDragStart(event, index) {
  if (panels.value[index].editing || expandedPanel.value !== null) {
    event.preventDefault()
    return
  }
  draggedIndex = index
  event.dataTransfer.effectAllowed = 'move'
  event.dataTransfer.setData('text/plain', index)
  setTimeout(() => {
    event.target.classList.add('dragging')
  }, 0)
}

function onDragEnd(event) {
  event.target.classList.remove('dragging')
  draggedIndex = null
}

const dragOverIndex = ref(null)

function onDragOver(event, index) {
  if (draggedIndex !== null && draggedIndex !== index) {
    dragOverIndex.value = index
  }
}

function onDragLeave(event, index) {
  if (dragOverIndex.value === index) {
    dragOverIndex.value = null
  }
}

function onDrop(event, targetIndex) {
  dragOverIndex.value = null
  if (draggedIndex === null || draggedIndex === targetIndex) return
  const item = panels.value.splice(draggedIndex, 1)[0]
  panels.value.splice(targetIndex, 0, item)
}

function onGridDrop(event) {
  if (draggedIndex === null) return
  const item = panels.value.splice(draggedIndex, 1)[0]
  panels.value.push(item)
}

let resizingPanel = null
let startX = 0
let startSpan = 1

function startResize(event, p) {
  resizingPanel = p
  startX = event.clientX
  startSpan = p.span || 1
  document.addEventListener('mousemove', onResizeMove)
  document.addEventListener('mouseup', onResizeEnd)
  event.preventDefault()
}

function onResizeMove(event) {
  if (!resizingPanel) return
  const diffX = event.clientX - startX
  if (startSpan === 1 && diffX > 50) {
    resizingPanel.span = 2
  } else if (startSpan === 2 && diffX < -50) {
    resizingPanel.span = 1
  }
}

function onResizeEnd() {
  resizingPanel = null
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
}
</script>

<template>
  <div class="metrics-dashboard">
    <!-- Toolbar -->
    <div class="dashboard-toolbar">
      <div class="db-title">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right:8px; vertical-align:text-bottom;">
          <rect x="3" y="3" width="7" height="7"></rect>
          <rect x="14" y="3" width="7" height="7"></rect>
          <rect x="14" y="14" width="7" height="7"></rect>
          <rect x="3" y="14" width="7" height="7"></rect>
        </svg>
        Cluster Overview
      </div>
      <div class="db-controls">
        <button class="db-btn outline live-btn" :class="{ 'is-live': isLive }" @click="toggleLive">
          <span v-if="isLive" class="live-dot pulse"></span>
          <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline></svg>
          {{ isLive ? 'Live Stream' : 'Live' }}
        </button>
        <div class="db-divider"></div>
        <button class="db-btn outline" @click="addPanel">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line>
          </svg>
          Add Panel
        </button>
        <!-- Namespace selector — populated from the live cluster
             (ListAllNamespaces). Loading + error states are visible so
             users don't think the dropdown is just empty. -->
        <Select
          v-model="selectedNamespace"
          :options="[{value:'',label:namespacesLoading ? 'Loading namespaces…' : 'All Namespaces'}, ...availableNamespaces.map(ns => ({value:ns,label:ns}))]"
          :disabled="namespacesLoading"
          size="sm"
          :aria-label="namespacesError ? 'Failed to load namespaces: ' + namespacesError : 'Filter metrics by namespace'"
        />
        <Select
          v-model="timeRange"
          :options="['Last 15 minutes','Last 1 hour','Last 6 hours','Last 24 hours','Last 7 days']"
          size="sm"
          aria-label="Time range"
        />
        <button class="db-btn primary">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
          </svg>
        </button>
      </div>
    </div>

    <!-- Grid -->
    <div class="panels-grid" :class="{ 'has-expanded': expandedPanel !== null }"
         @dragover.prevent
         @dragenter.prevent
         @drop="onGridDrop">
      <div v-for="(p, index) in panels" :key="p.id" 
           class="panel" 
           :class="[(p.span === 2 ? 'span-2' : ''), (expandedPanel === p.id ? 'expanded' : ''), (p.editing ? 'editing' : ''), (dragOverIndex === index ? 'drag-over' : '')]"
           v-show="expandedPanel === null || expandedPanel === p.id"
           :draggable="!p.editing && expandedPanel === null"
           @dragstart="onDragStart($event, index)"
           @dragend="onDragEnd"
           @dragover.prevent="onDragOver($event, index)"
           @dragleave="onDragLeave($event, index)"
           @drop.stop="onDrop($event, index)">
        
        <div class="panel-header" :class="{ 'grab': !p.editing && expandedPanel === null }">
          <div style="display:flex; align-items:center; gap:8px;">
            <span class="panel-title">{{ p.title }}</span>
            <button class="icon-btn edit-icon" @click="p.editing = !p.editing" title="Edit PromQL Query">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path></svg>
            </button>
            <button class="icon-btn expand-icon" @click="toggleExpand(p)" :title="expandedPanel === p.id ? 'Collapse' : 'Expand'">
              <svg v-if="expandedPanel !== p.id" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 3 21 3 21 9"></polyline><polyline points="9 21 3 21 3 15"></polyline><line x1="21" y1="3" x2="14" y2="10"></line><line x1="3" y1="21" x2="10" y2="14"></line></svg>
              <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 14 10 14 10 20"></polyline><polyline points="20 10 14 10 14 4"></polyline><line x1="14" y1="10" x2="21" y2="3"></line><line x1="3" y1="21" x2="10" y2="14"></line></svg>
            </button>
          </div>
          <span class="panel-val" v-if="p.val && p.type !== 'network' && p.type !== 'stat' && p.type !== 'gauge'">{{ p.val }}</span>
          <div class="panel-legend" v-else-if="p.type === 'network'">
            <span class="leg-item"><span class="leg-dot rx"></span> RX: {{ fmtRate(p.rxData) }}</span>
            <span class="leg-item"><span class="leg-dot tx"></span> TX: {{ fmtRate(p.txData) }}</span>
          </div>
        </div>

        <!-- Edit Overlay -->
        <div class="panel-edit" v-if="p.editing">
          <div class="edit-group">
            <label>Panel Title</label>
            <input type="text" v-model="p.title" class="edit-input" />
          </div>
          <div class="edit-group">
            <label>Graph Type</label>
            <Select v-model="p.type" :options="[{value:'area',label:'Area Chart'},{value:'line',label:'Line Chart'},{value:'bar',label:'Bar Chart'},{value:'network',label:'Network Dual-Axis'},{value:'stat',label:'Stat Metric'},{value:'gauge',label:'Gauge'}]" size="sm" />
          </div>
          <div class="edit-group">
            <label>Available Metrics</label>
            <Select
              :model-value="''"
              @change="(val) => { if (val) p.query = val }"
              :options="[{value:'',label:'Select a metric...'},{value:'sum(rate(container_cpu_usage_seconds_total[5m])) by (pod)',label:'CPU Usage Rate'},{value:'sum(container_memory_working_set_bytes) by (pod)',label:'Memory Working Set'},{value:'rate(container_network_receive_bytes_total[5m])',label:'Network RX Rate'},{value:'rate(container_network_transmit_bytes_total[5m])',label:'Network TX Rate'},{value:'count(kube_pod_info)',label:'Pod Count'},{value:'rate(http_requests_total[5m])',label:'HTTP Requests Rate'},{value:'rate(container_fs_reads_total[5m])',label:'Disk Read IOPS'}]"
              size="sm"
              placeholder="Select a metric..."
            />
          </div>
          <div class="edit-group">
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <label>PromQL Query A</label>
              <button class="db-btn outline" style="padding: 2px 8px; font-size: 14px; color: var(--accent);" @click="p.showAI = !p.showAI" title="Ask AI">✨</button>
            </div>
            <div v-if="p.showAI" style="display: flex; gap: 8px; margin-bottom: 4px;">
              <input type="text" v-model="p.aiPrompt" placeholder="e.g. show me the top 5 pods by memory..." class="edit-input" style="flex: 1;" @keyup.enter="generateQuery(p)"/>
              <button class="db-btn primary" @click="generateQuery(p)">Generate</button>
            </div>
            <textarea v-model="p.query" class="edit-input font-mono" rows="3"></textarea>
          </div>
          <div class="edit-actions" style="gap: 8px;">
            <button class="db-btn outline" style="border-color: rgba(239, 68, 68, 0.5); color: #ef4444;" @click="deletePanel(p)">Delete Graph</button>
            <div style="flex: 1;"></div>
            <button class="db-btn outline" @click="cancelEdit(p)">Cancel</button>
            <button class="save-btn" @click="savePanel(p)">Apply Changes</button>
          </div>
        </div>

        <!-- Render visual based on type -->
        <div class="panel-body" v-else>
          
          <template v-if="p.type === 'area'">
            <div v-if="p.loading" class="loading-overlay">
              <div class="loader"></div>
            </div>
            <div v-else-if="p.error || !p.data.length" class="no-data-overlay">
              <span>{{ p.error || 'No data' }}</span>
            </div>
            <svg v-else viewBox="0 0 400 100" preserveAspectRatio="none" class="panel-svg">
              <path :d="pointsToArea(p.data, 400, 100)" :fill="p.bg || 'rgba(79, 142, 247, 0.15)'" />
              <path :d="pointsToPath(p.data, 400, 100)" fill="none" :stroke="p.color || 'var(--accent)'" stroke-width="1.5" />
            </svg>
          </template>

          <template v-else-if="p.type === 'line'">
            <div v-if="p.loading" class="loading-overlay">
              <div class="loader"></div>
            </div>
            <div v-else-if="p.error || !p.data.length" class="no-data-overlay">
              <span>{{ p.error || 'No data' }}</span>
            </div>
            <svg v-else viewBox="0 0 400 100" preserveAspectRatio="none" class="panel-svg">
              <path :d="pointsToPath(p.data, 400, 100)" fill="none" :stroke="p.color || 'var(--teal)'" stroke-width="2" />
            </svg>
          </template>

          <template v-else-if="p.type === 'bar'">
            <div v-if="p.loading" class="loading-overlay">
              <div class="loader"></div>
            </div>
            <div v-else-if="p.error || !p.data.length" class="no-data-overlay">
              <span>{{ p.error || 'No data' }}</span>
            </div>
            <svg v-else viewBox="0 0 400 100" preserveAspectRatio="none" class="panel-svg">
              <rect v-for="(val, i) in p.data" :key="i"
                    :x="i * (400 / p.data.length)"
                    :y="100 - (val / 100 * 100)"
                    :width="(400 / p.data.length) - 1"
                    :height="(val / 100 * 100)"
                    :fill="p.color || 'var(--purple)'" />
            </svg>
          </template>

          <template v-else-if="p.type === 'network'">
            <div v-if="p.loading" class="loading-overlay">
              <div class="loader"></div>
            </div>
            <div v-else-if="p.error || (!p.rxData.length && !p.txData.length)" class="no-data-overlay">
              <span>{{ p.error || 'No network data' }}</span>
            </div>
            <svg v-else viewBox="0 0 800 150" preserveAspectRatio="none" class="panel-svg">
              <line x1="0" y1="30" x2="800" y2="30" stroke="var(--border)" stroke-width="1" stroke-dasharray="4" />
              <line x1="0" y1="75" x2="800" y2="75" stroke="var(--border)" stroke-width="1" stroke-dasharray="4" />
              <line x1="0" y1="120" x2="800" y2="120" stroke="var(--border)" stroke-width="1" stroke-dasharray="4" />
              <path :d="pointsToArea(p.rxData, 800, 150)" fill="rgba(45, 212, 191, 0.15)" />
              <path :d="pointsToPath(p.rxData, 800, 150)" fill="none" stroke="var(--teal)" stroke-width="1.5" />
              <path :d="pointsToArea(p.txData, 800, 150)" fill="rgba(245, 166, 35, 0.15)" />
              <path :d="pointsToPath(p.txData, 800, 150)" fill="none" stroke="var(--amber)" stroke-width="1.5" />
            </svg>
          </template>

          <template v-else-if="p.type === 'stat'">
            <div v-if="p.loading" class="loading-overlay">
              <div class="loader"></div>
            </div>
            <div v-else-if="p.error" class="no-data-overlay">
              <span>{{ p.error }}</span>
            </div>
            <template v-else>
              <div class="stat-big">
                <div class="stat-num">{{ p.reads }}</div>
                <div class="stat-sub">{{ p.statLabel1 || 'Restarts' }}</div>
              </div>
              <div class="stat-big mt">
                <div class="stat-num">{{ p.writes }}</div>
                <div class="stat-sub">{{ p.statLabel2 || 'Warning Events' }}</div>
              </div>
            </template>
          </template>

          <template v-else-if="p.type === 'gauge'">
            <div v-if="p.loading" class="loading-overlay">
              <div class="loader"></div>
            </div>
            <div v-else-if="p.error" class="no-data-overlay">
              <span>{{ p.error }}</span>
            </div>
            <div v-else class="flex-center" style="flex:1; display:flex;">
              <div class="gauge">
                <svg viewBox="0 0 100 50">
                  <path d="M 10 50 A 40 40 0 0 1 90 50" fill="none" stroke="var(--bg4)" stroke-width="12" stroke-linecap="round" />
                  <path :d="gaugeArc(p.gaugePct || 0)" fill="none" :stroke="gaugeColor(p.gaugePct || 0)" stroke-width="12" stroke-linecap="round" />
                </svg>
                <div class="gauge-val" :style="{ color: gaugeColor(p.gaugePct || 0) }">{{ p.val }}</div>
                <div class="gauge-lbl">of {{ p.limit }} total</div>
              </div>
            </div>
          </template>

        </div>

        <!-- Resize Handle -->
        <div class="resize-handle" @mousedown.stop.prevent="startResize($event, p)" v-show="!p.editing && expandedPanel === null" title="Drag to resize">
          <svg width="10" height="10" viewBox="0 0 10 10"><path d="M10 0 L10 10 L0 10 Z" fill="currentColor" opacity="0.3"/></svg>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.metrics-dashboard {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #111217; /* Grafana-like dark bg */
  color: #c8c9ca;
}

.dashboard-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--bg2);
  border-bottom: 1px solid var(--border);
}

.db-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--text);
}

.db-controls {
  display: flex;
  gap: 8px;
}


.db-btn {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text);
  min-width: 30px;
  padding: 0 10px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
}
.db-btn:hover { background: var(--bg4); }
.db-btn.primary { background: rgba(79,142,247,0.15); border-color: rgba(79,142,247,0.3); color: var(--accent); padding: 0 8px; }
.db-btn.outline { background: transparent; border-color: var(--border2); color: var(--text2); }
.db-btn.outline:hover { color: var(--text); border-color: var(--text3); }

.ml-1 { margin-left: 8px; }

/* Grid layout */
.panels-grid {
  flex: 1;
  padding: 12px;
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-auto-rows: 240px;
  gap: 12px;
  overflow-y: auto;
}
.panels-grid.has-expanded {
  display: flex;
  overflow: hidden;
}

.panel {
  background: #181b1f; /* Grafana panel bg */
  border: 1px solid #2c3235;
  border-radius: 3px;
  display: flex;
  flex-direction: column;
  position: relative;
  transition: transform 0.2s, opacity 0.2s;
}
.panel.dragging {
  opacity: 0.4;
  transform: scale(0.98);
}
.panel.span-2 {
  grid-column: span 2;
}
.panel.editing {
  grid-row: span 2;
}

.panel.drag-over {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px rgba(79, 142, 247, 0.3);
}

.panel.expanded {
  flex: 1;
  border-color: var(--accent);
}

.panel::before {
  content: '';
  position: absolute;
  top: -1px; left: -1px; right: -1px; height: 2px;
  background: transparent;
  transition: background 0.2s;
  border-radius: 3px 3px 0 0;
}
.panel:hover::before {
  background: var(--accent);
}

.panel-header {
  padding: 8px 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.panel-header.grab {
  cursor: grab;
}
.panel-header.grab:active {
  cursor: grabbing;
}
.panel-title {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text);
}
.panel-val {
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
  font-family: var(--mono);
}

.icon-btn {
  background: transparent;
  border: none;
  color: var(--text3);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex; align-items: center; justify-content: center;
  transition: all 0.15s;
  opacity: 0;
}
.panel:hover .edit-icon, .panel:hover .expand-icon {
  opacity: 1;
}
.icon-btn:hover {
  background: rgba(255,255,255,0.1);
  color: var(--text);
}

/* Edit Overlay */
.panel-edit {
  position: absolute;
  top: 36px; left: 0; right: 0; bottom: 0;
  background: rgba(24, 27, 31, 0.95);
  backdrop-filter: blur(4px);
  z-index: 10;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
}
.edit-group { display: flex; flex-direction: column; gap: 6px; }
.edit-group label { font-size: 11px; color: var(--accent); font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; }
.edit-input { background: #0b0c10; border: 1px solid #2c3235; padding: 8px; border-radius: 4px; color: var(--text); font-size: 12px; resize: none; outline: none; }
.edit-input:focus { border-color: var(--accent); }
.font-mono { font-family: var(--mono); color: #a5d6ff; }
.edit-actions { margin-top: auto; display: flex; justify-content: flex-end; }
.save-btn { background: var(--accent); border: 1px solid var(--accent); color: white; padding: 6px 12px; border-radius: 4px; font-size: 12px; font-weight: 500; cursor: pointer; }
.save-btn:hover { background: var(--accent2); }


.panel-legend {
  display: flex;
  gap: 12px;
  font-size: 11px;
}
.leg-item { display: flex; align-items: center; gap: 4px; }
.leg-dot { width: 8px; height: 2px; }
.leg-dot.rx { background: var(--teal); }
.leg-dot.tx { background: var(--amber); }

.panel-body {
  flex: 1;
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.panel-svg {
  width: 100%;
  height: 100%;
  position: absolute;
  bottom: 0;
  left: 0;
  pointer-events: none;
}

/* Stat Blocks */
.stat-big {
  padding: 16px;
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  pointer-events: none;
}
.stat-big.mt {
  border-top: 1px solid #2c3235;
}
.stat-num {
  font-size: 32px;
  font-weight: 600;
  color: var(--text);
  font-family: var(--mono);
}
.stat-sub {
  font-size: 12px;
  color: var(--text3);
  margin-top: 4px;
}

/* Gauge */
.gauge {
  position: relative;
  width: 180px;
  text-align: center;
  pointer-events: none;
}
.gauge-val {
  position: absolute;
  bottom: 15px;
  width: 100%;
  font-size: 28px;
  font-weight: 600;
  color: var(--green);
  font-family: var(--mono);
}
.gauge-lbl {
  position: absolute;
  bottom: -5px;
  width: 100%;
  font-size: 11px;
  color: var(--text3);
}

/* Resize Handle */
.resize-handle {
  position: absolute;
  bottom: 0;
  right: 0;
  width: 15px;
  height: 15px;
  cursor: ew-resize;
  display: flex;
  align-items: flex-end;
  justify-content: flex-end;
  padding: 2px;
  z-index: 5;
  color: var(--text3);
}
.resize-handle:hover {
  color: var(--accent);
}
.resize-handle:hover svg path {
  opacity: 1;
}

/* No data state */
.no-data-overlay {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text3, #6b7078);
  font-size: 12px;
  font-style: italic;
}

/* Loading state */
.loading-overlay {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(24, 27, 31, 0.7);
  z-index: 5;
}
.loader {
  width: 24px;
  height: 24px;
  border: 2px solid rgba(79, 142, 247, 0.2);
  border-bottom-color: var(--accent);
  border-radius: 50%;
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}

.db-divider {
  width: 1px;
  background: var(--border);
  margin: 0 4px;
}

.live-btn {
  color: var(--text2);
}
.live-btn.is-live {
  background: rgba(45, 212, 191, 0.15);
  border-color: rgba(45, 212, 191, 0.3);
  color: var(--teal);
}
.live-dot {
  width: 8px;
  height: 8px;
  background: var(--teal);
  border-radius: 50%;
  display: inline-block;
  margin-right: 4px;
}
.pulse {
  animation: pulse-dot 1.5s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
@keyframes pulse-dot {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}
</style>
