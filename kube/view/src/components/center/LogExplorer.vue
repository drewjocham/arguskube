<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useLogs } from '../../composables/useWails'
import { Events } from '../../composables/useEvents'
import { bus } from '../../lib/bus'
import { tokenize } from '../../utils/logHighlight'
import { buildSuggestions, fixQuerySyntax } from '../../utils/logQuery'
import { useSavedFiltersStore } from '../../stores/savedFilters'

const savedFilters = useSavedFiltersStore()
const { sortedEntries: savedFilterEntries } = storeToRefs(savedFilters)
const showSaveDialog = ref(false)
const saveDialogName = ref('')

function openSaveDialog() {
  // Pre-fill with the most recently loaded set name if any, otherwise blank.
  saveDialogName.value = ''
  showSaveDialog.value = true
}

function confirmSave() {
  const name = saveDialogName.value.trim()
  if (!name) return
  savedFilters.save(name, {
    query: query.value,
    filters: filters.value,
    limit: limit.value,
  })
  showSaveDialog.value = false
}

function loadSavedFilter(id) {
  const entry = savedFilters.entries.find(e => e.id === id)
  if (!entry) return
  query.value = entry.query
  filters.value = entry.filters.map(f => ({ ...f }))
  if (entry.limit) limit.value = entry.limit
  // Re-sync the sidebar checkboxes with the loaded filters.
  for (const [field, section] of Object.entries(fieldSections.value)) {
    for (const item of section.items) {
      const v = parseFieldLabel(item.label).value
      item.checked = filters.value.some(f => f.field === field && f.value === v)
    }
  }
  executeQuery()
}

function deleteSavedFilter(id, e) {
  if (e) e.stopPropagation()
  savedFilters.remove(id)
}

const { entries: backendLogs, histogram: backendHistogram, fields: backendFields, total, loading: logLoading, queryTime: backendQueryTime, error: logError, queryLogs } = useLogs()



const allLogs = computed(() => backendLogs.value || [])
const histogramData = computed(() => backendHistogram.value || [])

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
  // Sync the corresponding sidebar checkbox so the UI stays consistent.
  const removed = filters.value[index]
  filters.value.splice(index, 1)
  if (removed) {
    syncSidebarCheckbox(removed.field, removed.value, false)
  }
}

function clearFilters() {
  // Uncheck every sidebar checkbox so removing all filters also clears
  // the visible state in the side rail.
  for (const section of Object.values(fieldSections.value)) {
    for (const item of section.items) {
      item.checked = false
    }
  }
  filters.value = []
}

function copyFilters() {
  const text = filters.value.map(f => `{${f.field}="${f.value}"}`).join(' ')
  navigator.clipboard.writeText(text)
}

// Map sidebar section keys to the actual filter `field` names. The sidebar
// uses long form (kubernetes.pod_namespace) but the filter tag should
// render the same — both are fine, no translation needed currently.
function syncSidebarCheckbox(field, value, checked) {
  const sec = fieldSections.value[field]
  if (!sec) return
  const item = sec.items.find(i => parseFieldLabel(i.label).value === value)
  if (item) item.checked = checked
}

// fieldSections.items[].label is rendered as `<value> (<count>)` — split it
// back to get the value when wiring filters from a checkbox.
function parseFieldLabel(label) {
  const m = String(label || '').match(/^(.*)\s+\((\d+)\)$/)
  if (m) return { value: m[1], count: Number(m[2]) }
  return { value: String(label || ''), count: 0 }
}

function toggleFilterFromCheckbox(field, item) {
  const { value } = parseFieldLabel(item.label)
  const idx = filters.value.findIndex(f => f.field === field && f.value === value)
  if (item.checked && idx === -1) {
    filters.value.push({ field, value })
  } else if (!item.checked && idx !== -1) {
    filters.value.splice(idx, 1)
  }
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

// Pinned logs.
//
// We keep the FULL pinned log object alongside its key so pinned entries
// survive when the live-stream truncates `backendLogs` (which would drop
// the pin and leave only a stale counter). Persist to localStorage so pins
// survive navigation away and back.
const PINNED_STORAGE_KEY = 'argus.logExplorer.pinned.v1'
const pinnedLogs = ref(loadPinnedLogs())
const showPinnedOnly = ref(false)

function loadPinnedLogs() {
  try {
    const raw = window.localStorage?.getItem(PINNED_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch (_) {
    return []
  }
}

function persistPinnedLogs() {
  try {
    window.localStorage?.setItem(PINNED_STORAGE_KEY, JSON.stringify(pinnedLogs.value))
  } catch (_) { /* best effort */ }
}

function pinKey(log) {
  return `${log.time}::${log.message}`
}

const pinnedKeys = computed(() => new Set(pinnedLogs.value.map(pinKey)))

function togglePin(log) {
  const key = pinKey(log)
  const idx = pinnedLogs.value.findIndex(l => pinKey(l) === key)
  if (idx >= 0) {
    pinnedLogs.value.splice(idx, 1)
  } else {
    // Snapshot the entry so subsequent stream truncation can't mutate it.
    pinnedLogs.value.push({
      time: log.time,
      message: log.message,
      pod: log.pod,
      namespace: log.namespace,
      container: log.container,
      node: log.node,
      level: log.level,
    })
  }
  persistPinnedLogs()
}

function isPinned(log) {
  return pinnedKeys.value.has(pinKey(log))
}

function togglePinnedView() {
  showPinnedOnly.value = !showPinnedOnly.value
}

function clearPins() {
  pinnedLogs.value = []
  showPinnedOnly.value = false
  persistPinnedLogs()
}

// Filtered logs view.
//
// When the user has filtered to "pinned only", the source is the persistent
// pinnedLogs list — NOT the truncated live buffer. That way pinned entries
// don't disappear after ~10s when the stream rolls them out.
const displayLogs = computed(() => {
  if (showPinnedOnly.value) {
    return pinnedLogs.value.slice(0, limit.value)
  }
  return allLogs.value.slice(0, limit.value)
})

// Resizable time column.
const timeColWidth = ref(170)
let colDragging = false
let colStartX = 0
let colStartWidth = 0

function onColDragStart(e) {
  colDragging = true
  colStartX = e.clientX
  colStartWidth = timeColWidth.value
  document.addEventListener('mousemove', onColDragMove)
  document.addEventListener('mouseup', onColDragEnd)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
  e.preventDefault()
}

function onColDragMove(e) {
  if (!colDragging) return
  const delta = e.clientX - colStartX
  timeColWidth.value = Math.max(80, Math.min(400, colStartWidth + delta))
}

function onColDragEnd() {
  colDragging = false
  document.removeEventListener('mousemove', onColDragMove)
  document.removeEventListener('mouseup', onColDragEnd)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
}

onUnmounted(() => {
  document.removeEventListener('mousemove', onColDragMove)
  document.removeEventListener('mouseup', onColDragEnd)
})

// Initial load.
onMounted(() => {
  queryLogs('*', '', limit.value)
})

// Streaming toggle — when paused, new live log lines are dropped. The
// existing buffer (and any pinned entries) stay visible. The pause flag is
// also surfaced in the histogram/toolbar so the user can tell at a glance.
const streamingEnabled = ref(true)
const droppedWhilePaused = ref(0)

function toggleStreaming() {
  streamingEnabled.value = !streamingEnabled.value
  if (streamingEnabled.value) {
    droppedWhilePaused.value = 0
  }
}

// Listen for live log streams
bus.useWailsEvent(Events.LOG_LINE, (data) => {
  if (!data || !backendLogs.value || logLoading.value) return
  if (!streamingEnabled.value) {
    droppedWhilePaused.value++
    return
  }
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
})

// ── Suggestions dropdown ─────────────────────────────────────────────────
//
// `fieldValueIndex` extracts known values per field from the live log buffer
// so the dropdown can offer real completions (e.g. typing namespace= surfaces
// the namespaces actually present right now).
const queryFocused = ref(false)

const fieldValueIndex = computed(() => {
  const idx = {}
  const push = (key, v) => {
    if (!v) return
    idx[key] = idx[key] || new Set()
    idx[key].add(v)
  }
  for (const l of allLogs.value) {
    push('kubernetes.pod_namespace', l.namespace)
    push('kubernetes.pod_name', l.pod)
    push('kubernetes.container_name', l.container)
    push('level', l.level)
  }
  // Convert sets to arrays so the suggestion builder can iterate stably.
  const out = {}
  for (const [k, set] of Object.entries(idx)) out[k] = Array.from(set)
  return out
})

const querySuggestions = computed(() =>
  buildSuggestions(query.value, fieldValueIndex.value),
)

function applySuggestion(s) {
  query.value = s.query
  queryFocused.value = false
  executeQuery()
}

// Delay closing the dropdown so a click on a suggestion fires before the
// blur handler unmounts it. mousedown.prevent on the suggestion row keeps
// focus on the input, but we still need a small grace period.
function onQueryBlur() {
  setTimeout(() => { queryFocused.value = false }, 150)
}

// ── Syntax-fix wand ──────────────────────────────────────────────────────
//
// Click the ✨ button to run the cheap client-side fixer (balances braces /
// quotes, normalizes smart quotes, quotes bare values). The user sees a
// chip listing what changed so the fix isn't a black-box rewrite.
const lastFixNotes = ref([])

function fixQuery() {
  const { fixed, changed, notes } = fixQuerySyntax(query.value)
  if (changed) {
    query.value = fixed
    lastFixNotes.value = notes
    setTimeout(() => { lastFixNotes.value = [] }, 5000)
  } else {
    lastFixNotes.value = ['No syntax issues detected.']
    setTimeout(() => { lastFixNotes.value = [] }, 3000)
  }
}
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
            <input type="checkbox" v-model="item.checked" @change="toggleFilterFromCheckbox(key, item)" /> {{ item.label }}
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
            <input
              type="text"
              class="query-input"
              v-model="query"
              @keydown.enter="executeQuery"
              @focus="queryFocused = true"
              @blur="onQueryBlur"
              autocomplete="off"
              spellcheck="false"
            />
            <button
              class="query-wand"
              @click="fixQuery"
              title="Fix common syntax issues in the query (balance braces/quotes, normalize curly quotes, quote bare values)"
            >✨ Fix</button>
            <div v-if="queryFocused && querySuggestions.length" class="suggestions-dropdown">
              <div
                v-for="(s, i) in querySuggestions"
                :key="i"
                class="suggestion-row"
                @mousedown.prevent="applySuggestion(s)"
              >
                <div class="sugg-main">
                  <span class="sugg-label">{{ s.label }}</span>
                  <span class="sugg-tag" :class="s.kind">{{ s.kind === 'curated' ? 'preset' : 'value' }}</span>
                </div>
                <code class="sugg-query">{{ s.query }}</code>
                <div class="sugg-desc" v-if="s.description">{{ s.description }}</div>
              </div>
            </div>
          </div>
          <div class="limit-input-wrap">
            <span class="input-label">Limit</span>
            <input type="number" class="limit-input" v-model.number="limit" />
          </div>
        </div>

        <div v-if="lastFixNotes.length" class="fix-notes">
          <span v-for="(n, i) in lastFixNotes" :key="i" class="fix-note">{{ n }}</span>
        </div>

        <div class="action-row">
          <div class="action-left">
            <button class="action-btn" @click="filters.length ? clearFilters() : null">
              <svg width="12" height="12" viewBox="0 0 12 12"><path d="M2 3h8M4 6h4M5 9h2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg> Filters ({{ filters.length }})
            </button>
            <button class="action-btn" @click="clearFilters">Clear filters</button>
            <button class="action-btn" @click="copyFilters">Copy filters</button>
            <button class="action-btn" @click="openSaveDialog" :disabled="!query.trim() && filters.length === 0">
              💾 Save
            </button>
            <div class="saved-filters-wrap" v-if="savedFilterEntries.length">
              <select
                class="saved-filters-select"
                @change="(e) => { if (e.target.value) { loadSavedFilter(e.target.value); e.target.value = '' } }"
                :title="'Load a saved filter set'"
              >
                <option value="">📂 Load saved…</option>
                <option v-for="e in savedFilterEntries" :key="e.id" :value="e.id">
                  {{ e.name }}
                </option>
              </select>
            </div>
            <button
              class="action-btn"
              :class="{ 'stream-on': streamingEnabled, 'stream-off': !streamingEnabled }"
              @click="toggleStreaming"
              :title="streamingEnabled ? 'Pause live log streaming' : 'Resume live log streaming'"
            >
              <span class="stream-dot"></span>
              {{ streamingEnabled ? 'Live' : 'Paused' }}
              <span v-if="!streamingEnabled && droppedWhilePaused > 0" class="stream-count">+{{ droppedWhilePaused }}</span>
            </button>
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

        <!-- Saved-filters management list — clicking the name loads, × removes. -->
        <div v-if="savedFilterEntries.length" class="saved-filters-list">
          <span class="saved-filters-list-label">Saved:</span>
          <span
            v-for="e in savedFilterEntries"
            :key="e.id"
            class="saved-filter-pill"
            @click="loadSavedFilter(e.id)"
            :title="'Click to load — ' + (e.query || '(no query)') + ' · ' + e.filters.length + ' filter' + (e.filters.length === 1 ? '' : 's')"
          >
            {{ e.name }}
            <span class="x" @click="deleteSavedFilter(e.id, $event)" title="Delete saved filter">×</span>
          </span>
        </div>
      </div>

      <!-- Save dialog — modal-ish overlay anchored to the query area. -->
      <div v-if="showSaveDialog" class="save-dialog-backdrop" @click.self="showSaveDialog = false">
        <div class="save-dialog">
          <div class="save-dialog-title">Save filter set</div>
          <input
            class="save-dialog-input"
            v-model="saveDialogName"
            placeholder="Name (e.g. 'kube-system errors')"
            @keydown.enter="confirmSave"
            @keydown.escape="showSaveDialog = false"
            ref="saveInput"
          />
          <div class="save-dialog-hint">
            Saving the query, {{ filters.length }} filter{{ filters.length === 1 ? '' : 's' }}, and limit {{ limit }}.
          </div>
          <div class="save-dialog-actions">
            <button class="action-btn" @click="showSaveDialog = false">Cancel</button>
            <button class="action-btn primary" @click="confirmSave" :disabled="!saveDialogName.trim()">Save</button>
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
          <div class="toolbar-right">
            <button class="pin-master" :class="{ active: showPinnedOnly }" @click="togglePinnedView" :disabled="pinnedLogs.length === 0" :title="showPinnedOnly ? 'Show all logs' : 'Show pinned only (' + pinnedLogs.length + ')'">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none">
                <path d="M12 2L12 22" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" v-if="showPinnedOnly"/>
                <path d="M15.5 4.5L8.5 4.5C8.5 4.5 7 8 7 10.5C7 11.5 7.5 12 8.5 12.5L10 13V17L12 22L14 17V13L15.5 12.5C16.5 12 17 11.5 17 10.5C17 8 15.5 4.5 15.5 4.5Z" :fill="showPinnedOnly ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>
              </svg>
              <span class="pin-count" v-if="pinnedLogs.length > 0">{{ pinnedLogs.length }}</span>
            </button>
            <button v-if="pinnedLogs.length > 0" class="pin-clear" @click="clearPins" title="Clear all pins">×</button>
            <div class="toolbar-stats">
              Total logs returned: <b>{{ displayLogs.length }}</b>
            </div>
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
            <div v-for="(log, i) in displayLogs" :key="i">
              <div class="log-row" :class="{ pinned: isPinned(log) }" @click="toggleLogRow(i)">
                <div class="log-pin" :class="{ active: isPinned(log) }" @click.stop="togglePin(log)" title="Pin this log">
                  <svg width="11" height="11" viewBox="0 0 24 24" fill="none">
                    <path d="M15.5 4.5L8.5 4.5C8.5 4.5 7 8 7 10.5C7 11.5 7.5 12 8.5 12.5L10 13V17L12 22L14 17V13L15.5 12.5C16.5 12 17 11.5 17 10.5C17 8 15.5 4.5 15.5 4.5Z" :fill="isPinned(log) ? 'currentColor' : 'none'" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
                  </svg>
                </div>
                <div class="log-expander" :class="{ expanded: expandedRow === i }">›</div>
                <div class="log-time" :style="{ width: timeColWidth + 'px' }">{{ log.time }}</div>
                <div class="col-resize-handle" @mousedown.stop="onColDragStart"></div>
                <div class="log-message"><template v-for="(seg, si) in tokenize(log.message)" :key="si"><span v-if="seg.cls" :class="seg.cls">{{ seg.text }}</span><template v-else>{{ seg.text }}</template></template></div>
              </div>
              <div v-if="expandedRow === i" class="log-expanded">
                <div class="expanded-field"><span class="ef-key">_time</span><span class="ef-val">{{ log.time }}</span></div>
                <div class="expanded-field"><span class="ef-key">_msg</span><span class="ef-val"><template v-for="(seg, si) in tokenize(log.message)" :key="si"><span v-if="seg.cls" :class="seg.cls">{{ seg.text }}</span><template v-else>{{ seg.text }}</template></template></span></div>
                <div class="expanded-field"><span class="ef-key">kubernetes.container_name</span><span class="ef-val">traefik</span></div>
                <div class="expanded-field"><span class="ef-key">kubernetes.pod_namespace</span><span class="ef-val">traefik</span></div>
              </div>
            </div>
          </template>
          <!-- JSON view -->
          <template v-else>
            <pre class="json-view">{{ JSON.stringify(displayLogs, null, 2) }}</pre>
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

/* Wand button — sits at the right end of the query input. */
.query-wand {
  position: absolute;
  right: 6px;
  top: 24px;
  background: rgba(167, 139, 250, 0.12);
  border: 1px solid rgba(167, 139, 250, 0.3);
  color: #c4b3fd;
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s;
}
.query-wand:hover { background: rgba(167, 139, 250, 0.22); color: #fff; }

/* Suggestions dropdown — appears below the query input on focus. */
.suggestions-dropdown {
  position: absolute;
  top: calc(100% + 4px);
  left: 0;
  right: 0;
  z-index: 50;
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: 6px;
  max-height: 320px;
  overflow-y: auto;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
}
.suggestion-row {
  padding: 8px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  cursor: pointer;
  transition: background 0.1s;
}
.suggestion-row:last-child { border-bottom: none; }
.suggestion-row:hover { background: rgba(79, 142, 247, 0.08); }
.sugg-main { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.sugg-label { font-size: 12.5px; font-weight: 500; color: var(--text); }
.sugg-tag {
  font-size: 9.5px; text-transform: uppercase; letter-spacing: 0.06em;
  padding: 1px 6px; border-radius: 3px;
  background: rgba(255, 255, 255, 0.06); color: var(--text3);
}
.sugg-tag.curated { background: rgba(167, 139, 250, 0.15); color: #a78bfa; }
.sugg-tag.field-value { background: rgba(79, 142, 247, 0.12); color: var(--accent2); }
.sugg-query {
  display: block;
  font-family: var(--mono);
  font-size: 11px;
  color: var(--text2);
  margin-top: 3px;
}
.sugg-desc { font-size: 10.5px; color: var(--text3); margin-top: 2px; }

/* Fix-notes chip below the query input — explains what the wand changed. */
.fix-notes {
  display: flex; flex-wrap: wrap; gap: 6px;
  margin-top: 6px;
}
.fix-note {
  font-size: 10.5px;
  padding: 2px 8px;
  border-radius: 10px;
  background: rgba(167, 139, 250, 0.1);
  border: 1px solid rgba(167, 139, 250, 0.25);
  color: #c4b3fd;
}

/* Streaming toggle pill. */
.action-btn.stream-on,
.action-btn.stream-off {
  display: inline-flex; align-items: center; gap: 6px;
}
.action-btn.stream-on .stream-dot {
  width: 7px; height: 7px; border-radius: 50%;
  background: #3ecf8e;
  animation: stream-pulse 1.4s ease-in-out infinite;
}
.action-btn.stream-off .stream-dot {
  width: 7px; height: 7px; border-radius: 50%;
  background: #6b7078;
}
.action-btn.stream-off { color: var(--text3); border-color: rgba(255, 255, 255, 0.08); }
.action-btn .stream-count {
  font-family: var(--mono); font-size: 10.5px;
  background: rgba(245, 166, 35, 0.18);
  color: #f5a623;
  padding: 1px 6px; border-radius: 8px;
  margin-left: 4px;
}
@keyframes stream-pulse {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0.4; }
}

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

/* Saved filter sets — pill list under the active filters, plus a select
   in the toolbar. Both live in localStorage via the savedFilters store. */
.saved-filters-wrap { display: inline-flex; align-items: center; }
.saved-filters-select {
  background: var(--bg3);
  border: 1px solid var(--border2);
  color: var(--text2);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 11.5px;
  cursor: pointer;
  outline: none;
}
.saved-filters-select:hover { background: var(--bg4); color: var(--text); }
.saved-filters-list {
  display: flex; align-items: center; flex-wrap: wrap; gap: 6px;
  margin-top: 6px;
}
.saved-filters-list-label {
  font-size: 10.5px; color: var(--text3);
  text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600;
}
.saved-filter-pill {
  display: inline-flex; align-items: center; gap: 5px;
  padding: 2px 8px;
  background: rgba(167, 139, 250, 0.1);
  border: 1px solid rgba(167, 139, 250, 0.3);
  color: #c4b3fd;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.12s;
}
.saved-filter-pill:hover {
  background: rgba(167, 139, 250, 0.2);
  color: #fff;
  border-color: rgba(167, 139, 250, 0.5);
}
.saved-filter-pill .x {
  cursor: pointer;
  font-size: 13px; line-height: 1;
  color: rgba(255, 255, 255, 0.5);
  padding: 0 2px;
}
.saved-filter-pill .x:hover { color: #f05454; }

/* Save dialog overlay anchored to the query area. */
.save-dialog-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  z-index: 60;
  display: flex; align-items: flex-start; justify-content: center;
  padding-top: 80px;
  backdrop-filter: blur(2px);
}
.save-dialog {
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: 8px;
  padding: 16px;
  width: 360px;
  display: flex; flex-direction: column; gap: 10px;
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.45);
}
.save-dialog-title { font-size: 13px; font-weight: 600; color: var(--text); }
.save-dialog-input {
  background: var(--bg2); border: 1px solid var(--border2); border-radius: 5px;
  padding: 7px 10px; font-size: 12.5px; color: var(--text);
  outline: none;
}
.save-dialog-input:focus { border-color: rgba(167, 139, 250, 0.5); }
.save-dialog-hint { font-size: 11px; color: var(--text3); }
.save-dialog-actions { display: flex; justify-content: flex-end; gap: 8px; }
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
  color: var(--text3);
  flex-shrink: 0;
}
.col-resize-handle {
  width: 5px;
  flex-shrink: 0;
  cursor: col-resize;
  background: transparent;
  transition: background 0.15s;
  align-self: stretch;
}
.col-resize-handle:hover {
  background: var(--accent);
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

/* Pin — per-row icon */
.log-pin {
  width: 18px;
  flex-shrink: 0;
  color: transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color 0.15s var(--ease);
}
.log-row:hover .log-pin { color: var(--text3); }
.log-pin.active { color: var(--accent); }
.log-pin:hover { color: var(--accent2) !important; }
.log-row.pinned { background: rgba(79,142,247,0.04); }

/* Pin — master toolbar button */
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 6px;
}
.pin-master {
  display: flex;
  align-items: center;
  gap: 4px;
  background: none;
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 3px 7px;
  color: var(--text3);
  cursor: pointer;
  font-size: 11px;
  transition: all 0.15s var(--ease);
}
.pin-master:hover:not(:disabled) { color: var(--accent2); border-color: var(--accent); background: rgba(79,142,247,0.08); }
.pin-master.active { color: var(--accent); border-color: var(--accent); background: rgba(79,142,247,0.12); }
.pin-master:disabled { opacity: 0.35; cursor: default; }
.pin-count {
  font-family: var(--mono);
  font-size: 10px;
  background: rgba(79,142,247,0.15);
  color: var(--accent2);
  padding: 0 5px;
  border-radius: 8px;
  font-weight: 600;
}
.pin-clear {
  background: none;
  border: none;
  color: var(--text3);
  cursor: pointer;
  font-size: 14px;
  padding: 0 4px;
  line-height: 1;
  transition: color 0.15s;
}
.pin-clear:hover { color: var(--red2); }

/* Execute button states */
.action-btn.executing { opacity: 0.7; pointer-events: none; }

/* Log syntax highlighting */
.hl-fatal    { color: #ff4040; font-weight: 700; background: rgba(255,64,64,0.12); padding: 0 3px; border-radius: 2px; }
.hl-error    { color: var(--red2); font-weight: 600; }
.hl-warn     { color: var(--amber2); font-weight: 600; }
.hl-info     { color: var(--accent2); }
.hl-debug    { color: var(--text3); }
.hl-method   { color: #c792ea; font-weight: 500; }
.hl-string   { color: #c3e88d; }
.hl-key      { color: #89ddff; }
.hl-ip       { color: #f78c6c; }
.hl-label    { color: #a78bfa; }
.hl-duration { color: var(--green); }
</style>
