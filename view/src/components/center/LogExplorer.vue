<script setup>
import { ref, computed, onMounted } from 'vue'
import { useLogs } from '../../composables/useWails'
import { useWailsEvent, Events } from '../../composables/useEvents'

const { entries: backendLogs, histogram: backendHistogram, fields: backendFields, total, loading: logLoading, queryTime: backendQueryTime, error: logError, queryLogs } = useLogs()

// Fallback mock logs for dev mode.
const mockLogs = [
  { time: '2026-04-29 09:12:42.786', message: '172.70.111.27 - - [29/Apr/2026:07:12:42 +0000] "POST /select/logsql/field_names HTTP/2.0" 200 774 "-" "-" 4048 "logging-logs-ingress" 4ms', pod: 'traefik-abc12', namespace: 'traefik', container: 'traefik', node: 'node-1' },
  { time: '2026-04-29 09:12:41.956', message: '172.70.111.27 - - [29/Apr/2026:07:12:41 +0000] "POST /select/logsql/stream_field_values HTTP/2.0" 200 42 "-" "-" 4047 "logging-logs-ingress" 6ms', pod: 'traefik-abc12', namespace: 'traefik', container: 'traefik', node: 'node-1' },
  { time: '2026-04-29 09:12:41.955', message: '172.70.111.27 - - [29/Apr/2026:07:12:41 +0000] "POST /select/logsql/stream_field_values HTTP/2.0" 200 72 "-" "-" 4046 "logging-logs-ingress" 6ms', pod: 'traefik-abc12', namespace: 'traefik', container: 'traefik', node: 'node-1' },
  { time: '2026-04-29 09:12:41.942', message: '172.70.111.27 - - [29/Apr/2026:07:12:41 +0000] "POST /select/logsql/hits HTTP/2.0" 200 281 "-" "-" 4045 "logging-logs-ingress" 2ms', pod: 'traefik-def34', namespace: 'traefik', container: 'traefik', node: 'node-2' },
  { time: '2026-04-29 09:12:41.724', message: '172.70.111.27 - - [29/Apr/2026:07:12:41 +0000] "POST /select/logsql/stream_field_names HTTP/2.0" 200 149 "-" "-" 4043 "logging-logs-ingress" 8ms', pod: 'traefik-def34', namespace: 'traefik', container: 'traefik', node: 'node-2' },
  { time: '2026-04-29 09:12:41.721', message: '172.70.111.27 - - [29/Apr/2026:07:12:41 +0000] "POST /select/logsql/query HTTP/2.0" 200 3038 "-" "-" 4044 "logging-logs-ingress" 2ms', pod: 'traefik-def34', namespace: 'traefik', container: 'traefik', node: 'node-2' },
  { time: '2026-04-29 09:12:41.403', message: '172.70.111.27 - - [29/Apr/2026:07:12:41 +0000] "POST /select/logsql/field_names HTTP/2.0" 200 626 "-" "-" 4042 "logging-logs-ingress" 2ms', pod: 'traefik-abc12', namespace: 'traefik', container: 'traefik', node: 'node-1' },
]

const mockHistogram = [
  0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
  0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 2, 0, 0, 0, 0,
  1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8, 14, 12
]

// Computed: use backend data if available, else mock.
const allLogs = computed(() => backendLogs.value.length > 0 ? backendLogs.value : mockLogs)
const histogramData = computed(() => backendHistogram.value.length > 0 ? backendHistogram.value : mockHistogram)

// Query and filter state.
const query = ref('*')
const limit = ref(50)
const activeTab = ref('group')
const expandedRow = ref(null)
const namespaceFilter = ref('')

const filters = ref([])

// Stream field sidebar — dynamically built from backend fields + log data.
const fieldSections = computed(() => {
  const logs = allLogs.value
  const nsMap = {}
  const podMap = {}
  const containerMap = {}

  for (const l of logs) {
    if (l.namespace) nsMap[l.namespace] = (nsMap[l.namespace] || 0) + 1
    if (l.pod) podMap[l.pod] = (podMap[l.pod] || 0) + 1
    if (l.container) containerMap[l.container] = (containerMap[l.container] || 0) + 1
  }

  return {
    'kubernetes.pod_namespace': {
      open: openSections.value['kubernetes.pod_namespace'] ?? true,
      items: Object.entries(nsMap).map(([k, v]) => ({ label: `${k} (${v})`, checked: false })),
    },
    'kubernetes.pod_name': {
      open: openSections.value['kubernetes.pod_name'] ?? false,
      items: Object.entries(podMap).map(([k, v]) => ({ label: `${k} (${v})`, checked: false })),
    },
    'kubernetes.container_name': {
      open: openSections.value['kubernetes.container_name'] ?? false,
      items: Object.entries(containerMap).map(([k, v]) => ({ label: `${k} (${v})`, checked: false })),
    },
  }
})

// Track which field sections are open (separate from the computed so we can toggle).
const openSections = ref({ 'kubernetes.pod_namespace': true, 'kubernetes.pod_name': false, 'kubernetes.container_name': false })

function toggleFieldSection(key) {
  openSections.value[key] = !openSections.value[key]
}

// Filter actions.
function removeFilter(index) {
  filters.value.splice(index, 1)
}

function clearFilters() {
  filters.value = []
}

function copyFilters() {
  const text = filters.value.map(f => `{${f.field}="${f.value}"}`).join(' ')
  navigator.clipboard.writeText(text)
}

// Execute query.
const executing = computed(() => logLoading.value)
const queryTime = computed(() => backendQueryTime.value)

async function executeQuery() {
  await queryLogs(query.value, namespaceFilter.value, limit.value)
}

// Log row expand.
function toggleLogRow(index) {
  expandedRow.value = expandedRow.value === index ? null : index
}

// Filtered logs view.
const fakeLogs = computed(() => allLogs.value.slice(0, limit.value))

// Initial load.
onMounted(() => {
  queryLogs('*', '', limit.value)
})

// Listen for live log streams
useWailsEvent(Events.LOG_LINE, (data) => {
  if (data && backendLogs.value && !logLoading.value) {
    // Only prepend if we're not actively querying
    const newEntry = {
      time: new Date(data.timestamp).toISOString().replace('T', ' ').slice(0, -1),
      message: data.message,
      pod: data.source ? data.source.replace(/\[|\]/g, '') : 'unknown',
      namespace: '', 
      container: '',
      node: ''
    }
    backendLogs.value.unshift(newEntry)
    if (backendLogs.value.length > limit.value) {
      backendLogs.value.pop()
    }
  }
})
</script>

<template>
  <div class="log-explorer">
    <!-- Stream Fields Sidebar -->
    <div class="explorer-sidebar">
      <div class="sidebar-header">
        Stream fields
        <div class="sidebar-actions">
          <svg width="12" height="12" viewBox="0 0 12 12"><path d="M2 3h8M2 6h8M2 9h8" stroke="currentColor" stroke-width="1.5" fill="none"/></svg>
        </div>
      </div>

      <div v-for="(section, key) in fieldSections" :key="key" class="field-group">
        <div class="field-header" @click="toggleFieldSection(key)">
          <div class="field-icon">{{ section.open ? '-' : '+' }}</div>
          {{ key }} <span class="count">(49)</span>
          <span v-if="section.items.some(i => i.checked)" class="field-badge">{{ section.items.filter(i => i.checked).length }}</span>
        </div>
        <div v-if="section.open && section.items.length" class="field-items">
          <label v-for="(item, i) in section.items" :key="i" class="field-checkbox">
            <input type="checkbox" v-model="item.checked" /> {{ item.label }}
          </label>
        </div>
      </div>
    </div>

    <!-- Main Explorer Area -->
    <div class="explorer-main">
      
      <!-- Query Builder -->
      <div class="query-area">
        <div class="query-row">
          <div class="query-input-wrap">
            <span class="input-label">Query ({{ queryTime }}ms)</span>
            <input type="text" class="query-input" v-model="query" @keydown.enter="executeQuery" />
          </div>
          <div class="limit-input-wrap">
            <span class="input-label">Limit</span>
            <input type="number" class="limit-input" v-model.number="limit" />
          </div>
        </div>

        <div class="action-row">
          <div class="action-left">
            <button class="action-btn" @click="filters.length ? clearFilters() : null">
              <svg width="12" height="12" viewBox="0 0 12 12"><path d="M2 3h8M4 6h4M5 9h2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg> Filters ({{ filters.length }})
            </button>
            <button class="action-btn" @click="clearFilters">Clear filters</button>
            <button class="action-btn" @click="copyFilters">Copy filters</button>
          </div>
          <div class="action-right">
            <button class="action-btn primary" :class="{ executing }" @click="executeQuery">{{ executing ? '⏳ Running…' : '▶ Execute' }}</button>
          </div>
        </div>

        <div class="active-filters" v-if="filters.length">
          <div v-for="(f, i) in filters" :key="i" class="filter-tag">
            {{ '{' + f.field + '="' + f.value + '"}' }} <span class="x" @click="removeFilter(i)">×</span>
          </div>
        </div>
      </div>

      <!-- Histogram -->
      <div class="histogram-area">
        <div class="histogram-header">
          <div class="hist-stats">Total: <b>49</b> &nbsp; Query time: <b>1ms</b> &nbsp; Interval: <b>5s ▾</b></div>
          <div class="hist-toggles">
            <label><input type="radio" checked> Bars</label>
          </div>
        </div>
        <div class="histogram-chart">
          <!-- Background Grid Lines -->
          <div class="grid-line" style="bottom: 0%"><span>0</span></div>
          <div class="grid-line" style="bottom: 33%"><span>5</span></div>
          <div class="grid-line" style="bottom: 66%"><span>10</span></div>
          <div class="grid-line" style="bottom: 100%"><span>15</span></div>
          
          <div class="chart-bars">
            <div v-for="(val, i) in histogramData" :key="i" class="bar-wrapper">
              <div class="bar" :style="{ height: (val / 15 * 100) + '%' }"></div>
            </div>
          </div>
        </div>
        <div class="chart-x-axis">
          <span>09:08:00</span>
          <span>09:09:00</span>
          <span>09:10:00</span>
          <span>09:11:00</span>
          <span>09:12:00</span>
        </div>
      </div>

      <!-- Log Table -->
      <div class="log-table-area">
        <div class="table-toolbar">
          <div class="toolbar-tabs">
            <div class="toolbar-tab" :class="{ active: activeTab === 'group' }" @click="activeTab = 'group'">Group</div>
            <div class="toolbar-tab" :class="{ active: activeTab === 'table' }" @click="activeTab = 'table'">Table</div>
            <div class="toolbar-tab" :class="{ active: activeTab === 'json' }" @click="activeTab = 'json'">JSON</div>
          </div>
          <div class="toolbar-stats">
            Total logs returned: <b>{{ fakeLogs.length }}</b>
          </div>
        </div>

        <div v-if="activeTab === 'group'" class="group-header">
          <b>1. Group by "_stream"</b>
          <span v-for="f in filters" :key="f.field" class="group-tag">{{ f.field }}="{{ f.value }}"</span>
          <span v-if="!filters.length" class="group-tag">all streams</span>
        </div>

        <div class="log-list">
          <!-- Group / Table view -->
          <template v-if="activeTab !== 'json'">
            <div v-for="(log, i) in fakeLogs" :key="i">
              <div class="log-row" @click="toggleLogRow(i)">
                <div class="log-expander" :class="{ expanded: expandedRow === i }">›</div>
                <div class="log-time">{{ log.time }}</div>
                <div class="log-message">{{ log.message }}</div>
              </div>
              <div v-if="expandedRow === i" class="log-expanded">
                <div class="expanded-field"><span class="ef-key">_time</span><span class="ef-val">{{ log.time }}</span></div>
                <div class="expanded-field"><span class="ef-key">_msg</span><span class="ef-val">{{ log.message }}</span></div>
                <div class="expanded-field"><span class="ef-key">kubernetes.container_name</span><span class="ef-val">traefik</span></div>
                <div class="expanded-field"><span class="ef-key">kubernetes.pod_namespace</span><span class="ef-val">traefik</span></div>
              </div>
            </div>
          </template>
          <!-- JSON view -->
          <template v-else>
            <pre class="json-view">{{ JSON.stringify(fakeLogs, null, 2) }}</pre>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.log-explorer {
  display: flex;
  height: 100%;
  background: var(--bg);
  border-radius: var(--r);
  border: 1px solid var(--border);
  overflow: hidden;
  font-family: var(--font);
}

/* Sidebar */
.explorer-sidebar {
  width: 260px;
  background: var(--bg2);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
}
.sidebar-header {
  padding: 12px 14px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text);
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
}
.sidebar-actions { color: var(--text3); cursor: pointer; }

.field-group {
  border-bottom: 1px solid var(--border);
}
.field-header {
  padding: 8px 12px;
  font-size: 12px;
  color: var(--text);
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  background: var(--bg3);
}
.field-icon { width: 14px; height: 14px; background: var(--accent); color: white; display: flex; align-items: center; justify-content: center; border-radius: 50%; font-size: 10px; font-weight: bold; }
.count { color: var(--text3); font-size: 11px; }
.field-badge { margin-left: auto; background: var(--bg4); padding: 1px 6px; border-radius: 10px; font-size: 10px; color: var(--text2); }

.field-items {
  padding: 6px 12px;
  background: var(--bg2);
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.field-checkbox {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11.5px;
  color: var(--text2);
  cursor: pointer;
}
.field-checkbox input { accent-color: var(--accent); }

/* Main Area */
.explorer-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Query Builder */
.query-area {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
  background: var(--bg2);
}
.query-row {
  display: flex;
  gap: 12px;
  margin-bottom: 12px;
}
.query-input-wrap { flex: 1; position: relative; }
.limit-input-wrap { width: 80px; position: relative; }
.input-label {
  position: absolute;
  top: -6px; left: 8px;
  background: var(--bg2);
  padding: 0 4px;
  font-size: 10px;
  color: var(--text3);
}
.query-input, .limit-input {
  width: 100%;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 8px 12px;
  color: var(--text);
  font-family: var(--mono);
  font-size: 13px;
  outline: none;
}
.query-input:focus, .limit-input:focus { border-color: var(--accent); }

.action-row {
  display: flex;
  justify-content: space-between;
  margin-bottom: 12px;
}
.action-left, .action-right { display: flex; gap: 8px; }
.action-btn {
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text2);
  padding: 5px 12px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 5px;
}
.action-btn:hover { background: var(--bg4); color: var(--text); }
.action-btn.primary { background: var(--accent); color: white; border-color: var(--accent); font-weight: 500; }
.action-btn.primary:hover { background: var(--accent2); }

.active-filters { display: flex; gap: 8px; }
.filter-tag {
  background: rgba(79,142,247,0.1);
  border: 1px solid rgba(79,142,247,0.3);
  color: var(--accent2);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 11.5px;
  font-family: var(--mono);
  display: flex;
  align-items: center;
  gap: 6px;
}
.filter-tag .x { cursor: pointer; color: var(--text3); }
.filter-tag .x:hover { color: var(--red2); }

/* Histogram */
.histogram-area {
  padding: 16px;
  border-bottom: 1px solid var(--border);
  background: var(--bg);
  height: 200px;
  display: flex;
  flex-direction: column;
}
.histogram-header {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: var(--text3);
  margin-bottom: 10px;
}
.hist-stats b { color: var(--text); font-weight: 500; }
.hist-toggles { display: flex; gap: 10px; }

.histogram-chart {
  flex: 1;
  position: relative;
  margin-left: 20px;
}
.grid-line {
  position: absolute;
  left: 0; right: 0;
  border-bottom: 1px dashed var(--border);
}
.grid-line span {
  position: absolute;
  left: -20px;
  bottom: -6px;
  font-size: 10px;
  color: var(--text3);
}

.chart-bars {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
}
.bar-wrapper { flex: 1; display: flex; justify-content: center; height: 100%; align-items: flex-end; padding: 0 1px; }
.bar { width: 100%; background: var(--bg4); border-radius: 2px 2px 0 0; transition: height 0.3s; }
.bar:hover { background: var(--accent); }

.chart-x-axis {
  display: flex;
  justify-content: space-between;
  margin-left: 20px;
  margin-top: 8px;
  font-size: 10px;
  color: var(--text3);
}

/* Log Table */
.log-table-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: var(--bg);
  overflow: hidden;
}
.table-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid var(--border);
  background: var(--bg2);
}
.toolbar-tabs { display: flex; gap: 16px; }
.toolbar-tab {
  padding: 10px 0;
  font-size: 12px;
  color: var(--text2);
  cursor: pointer;
  border-bottom: 2px solid transparent;
}
.toolbar-tab.active { color: var(--accent2); border-bottom-color: var(--accent); }
.toolbar-stats { font-size: 11px; color: var(--text3); }
.toolbar-stats b { color: var(--text); font-weight: 500; }

.group-header {
  padding: 8px 16px;
  font-size: 12px;
  color: var(--text);
  background: var(--bg3);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  gap: 10px;
}
.group-tag {
  background: rgba(79,142,247,0.1);
  color: var(--accent2);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 10px;
  font-family: var(--mono);
}

.log-list {
  flex: 1;
  overflow-y: auto;
}
.log-row {
  display: flex;
  align-items: flex-start;
  padding: 6px 16px;
  border-bottom: 1px solid var(--border);
  font-family: var(--mono);
  font-size: 11.5px;
  background: rgba(255,255,255,0.02);
}
.log-row:nth-child(even) { background: transparent; }
.log-row:hover { background: var(--bg3); }
.log-expander {
  width: 20px;
  color: var(--text3);
  cursor: pointer;
  user-select: none;
  transition: transform 0.15s;
}
.log-expander.expanded { transform: rotate(90deg); color: var(--accent); }
.log-time {
  width: 170px;
  color: var(--text3);
  flex-shrink: 0;
}
.log-message {
  flex: 1;
  color: var(--text2);
  word-break: break-all;
}
.log-row { cursor: pointer; }

/* Expanded log row */
.log-expanded {
  background: var(--bg2);
  border-bottom: 1px solid var(--border);
  padding: 8px 16px 8px 36px;
}
.expanded-field {
  display: flex;
  gap: 12px;
  padding: 3px 0;
  font-size: 11px;
  font-family: var(--mono);
}
.ef-key { color: var(--accent2); min-width: 200px; flex-shrink: 0; }
.ef-val { color: var(--text2); word-break: break-all; }

/* JSON view */
.json-view {
  padding: 16px;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--text2);
  white-space: pre-wrap;
  margin: 0;
}

/* Execute button states */
.action-btn.executing { opacity: 0.7; pointer-events: none; }
</style>
