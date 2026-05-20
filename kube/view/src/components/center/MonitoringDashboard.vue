<script setup>
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import MetricCategoryStrip from './MetricCategoryStrip.vue'
import MetricCategoryPopup from './MetricCategoryPopup.vue'
import MetricWidget from './MetricWidget.vue'
import {
  useDashboardMetrics, METRIC_CATEGORIES,
} from '../../composables/useDashboardMetrics'

const {
  dashboards, activeIndex, activeDashboard, editMode,
  clusterMetricsState, clusterMetricsError,
  createDashboard, deleteDashboard, renameDashboard,
  addWidget, removeWidget, moveWidget,
  fetchClusterMetrics, refreshSparklines,
} = useDashboardMetrics()

// ── Popup state ──────────────────────────────────────────────────
const popupCategoryId = ref(null)
const popupShow = ref(false)

function openPopup(categoryId) {
  popupCategoryId.value = categoryId
  popupShow.value = true
}
function closePopup() {
  popupShow.value = false
}

// ── Dashboard name editing ───────────────────────────────────────
const editingName = ref(false)
const nameInput = ref('')
const nameInputEl = ref(null)

function startRename() {
  nameInput.value = activeDashboard.value?.name || ''
  editingName.value = true
  // Focus after Vue renders the input
  setTimeout(() => nameInputEl.value?.focus(), 50)
}
function commitRename() {
  const name = nameInput.value.trim()
  if (name) renameDashboard(activeIndex.value, name)
  editingName.value = false
}
function cancelRename() {
  editingName.value = false
}

// ── New dashboard prompt ─────────────────────────────────────────
const newDashName = ref('')

function promptNewDashboard() {
  const name = `Dashboard ${dashboards.value.length + 1}`
  createDashboard(name)
}

// ── Widget limit ─────────────────────────────────────────────────
const widgetCount = computed(() => activeDashboard.value?.widgets?.length || 0)
const canAddWidget = computed(() => widgetCount.value < 4)

// ── Banner state classifiers ─────────────────────────────────────
// The error message produced by callGo / fetch carries the HTTP
// status verbatim ("HTTP error! status: 403"). We pattern-match on
// that so the banner can tell the user the right next step instead
// of a generic "isn't reachable".
const clusterMetricsErrorIs401 = computed(() => {
  const m = String(clusterMetricsError.value || '')
  return /status:\s*401\b/i.test(m) || /unauthor/i.test(m)
})
const clusterMetricsErrorIs403 = computed(() => {
  const m = String(clusterMetricsError.value || '')
  return /status:\s*403\b/i.test(m) || /forbidden|not exposed/i.test(m)
})

// ── Handle add-widget from popup ─────────────────────────────────
function onAddWidget(metricId) {
  if (!canAddWidget.value) return
  addWidget(metricId)
  // Keep popup open so user can add more
}

// ── Polling ──────────────────────────────────────────────────────
let pollTimer = null

onMounted(() => {
  fetchClusterMetrics()
  refreshSparklines()
  pollTimer = setInterval(() => {
    fetchClusterMetrics()
    refreshSparklines()
  }, 10000) // every 10s
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="monitoring-dashboard">
    <!-- ── Top bar: dashboard management ───────────────────────── -->
    <div class="dashboard-bar">
      <div class="dashboard-selector">
        <template v-if="editingName">
          <input
            ref="nameInputEl"
            v-model="nameInput"
            class="name-input"
            @keydown.enter="commitRename"
            @keydown.escape="cancelRename"
            @blur="commitRename"
          />
        </template>
        <template v-else>
          <select
            class="dashboard-select"
            :value="activeIndex"
            @change="activeIndex = Number($event.target.value)"
          >
            <option
              v-for="(dash, i) in dashboards"
              :key="dash.id"
              :value="i"
            >{{ dash.name }}</option>
          </select>
          <button
            class="dash-btn dash-btn-icon"
            @click="startRename"
            title="Rename dashboard"
          >✎</button>
        </template>
        <button
          class="dash-btn"
          @click="promptNewDashboard"
          title="New dashboard"
        >+ New</button>
        <button
          v-if="dashboards.length > 1"
          class="dash-btn dash-btn-danger"
          @click="deleteDashboard(activeIndex)"
          title="Delete current dashboard"
        >🗑</button>
      </div>
      <div class="dashboard-actions">
        <span class="widget-count" v-if="widgetCount > 0">
          {{ widgetCount }}/4 widgets
        </span>
        <button
          class="dash-btn"
          :class="{ active: editMode }"
          @click="editMode = !editMode"
        >
          {{ editMode ? 'Done Editing' : 'Edit Layout' }}
        </button>
      </div>
    </div>

    <!-- ── Connection banner — surfaces the GetClusterMetrics failure
         that would otherwise leave every category showing "—" with no
         explanation. Shown only when the first fetch failed AND no
         cached data is available; once any successful fetch arrives
         we keep showing whatever we last had.

         The banner copy is shaped by the error's HTTP status so the
         user gets an actionable next step instead of a generic
         "isn't reachable":
           401 → auth required (sign in / unset session)
           403 → method not exposed (this binary doesn't allowlist it
                 — bug to file, not a user action)
           other / network → backend really isn't responding
         ──────────────────────────────────────────────────────────── -->
    <output
      v-if="clusterMetricsState === 'error'"
      class="dashboard-banner banner-error"
      data-testid="dashboard-disconnected-banner"
    >
      <template v-if="clusterMetricsErrorIs401">
        <strong>Sign-in required.</strong>
        The backend is up but rejected an unauthenticated request.
        Sign in via the title-bar Profile menu, or relaunch with
        <code>make no-auth-run</code> for local dev.
      </template>
      <template v-else-if="clusterMetricsErrorIs403">
        <strong>Backend rejected the call.</strong>
        The dashboard's RPC method isn't in this build's HTTP allowlist
        (<em>{{ clusterMetricsError }}</em>). Likely a version mismatch
        between the frontend and the backend — file a bug.
      </template>
      <template v-else>
        <strong>No live metrics.</strong>
        Couldn't reach the Argus backend at <code>:8080</code>
        <span v-if="clusterMetricsError">— <em>{{ clusterMetricsError }}</em></span>.
        Start the desktop app (<code>make dev</code> or <code>make no-auth-run</code>)
        to populate this dashboard.
      </template>
    </output>
    <output
      v-else-if="clusterMetricsState === 'loading'"
      class="dashboard-banner banner-loading"
      data-testid="dashboard-loading-banner"
    >
      Loading cluster metrics…
    </output>

    <!-- ── Info message for empty dashboards ────────────────────── -->
    <div v-if="widgetCount === 0" class="empty-hint">
      Expand a category, then click <b>+</b> on any metric to pin it here.
      You can pin up to 4 metrics from any category.
    </div>

    <!-- ── Category strips ─────────────────────────────────────── -->
    <div class="category-strips">
      <MetricCategoryStrip
        v-for="cat in METRIC_CATEGORIES"
        :key="cat.id"
        :category-id="cat.id"
        @expand="openPopup"
      />
    </div>

    <!-- ── Free-form canvas ────────────────────────────────────── -->
    <div class="canvas-area" :class="{ 'canvas-editing': editMode }">
      <div class="canvas-label">Pinned Metrics</div>
      <div class="canvas-grid">
        <MetricWidget
          v-for="w in activeDashboard.widgets"
          :key="w.metricId"
          :metric-id="w.metricId"
          :x="w.x"
          :y="w.y"
          :edit-mode="editMode"
          @remove="removeWidget"
          @move="moveWidget"
        />
        <div v-if="widgetCount === 0" class="canvas-placeholder">
          No metrics pinned yet — expand a category above to add some
        </div>
      </div>
    </div>

    <!-- ── Category popup (teleported to body) ─────────────────── -->
    <MetricCategoryPopup
      :category-id="popupCategoryId"
      :show="popupShow"
      @close="closePopup"
      @add-widget="onAddWidget"
    />
  </div>
</template>

<style scoped>
.monitoring-dashboard {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 10px;
  padding: 0 0 10px;
}

.dashboard-banner {
  display: block;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.4;
  flex-shrink: 0;
}
.dashboard-banner code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  padding: 1px 4px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 3px;
}
.dashboard-banner em {
  font-style: normal;
  opacity: 0.8;
}
.banner-error {
  background: rgba(239, 68, 68, 0.12);
  border: 1px solid rgba(239, 68, 68, 0.35);
  color: var(--text, #eee);
}
.banner-loading {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border, #2a2a2a);
  color: var(--text3, #888);
}

/* ── Top bar ──────────────────────────────────────────────────── */
.dashboard-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  gap: 10px;
  flex-shrink: 0;
}
.dashboard-selector {
  display: flex;
  align-items: center;
  gap: 6px;
}
.dashboard-select {
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: var(--r);
  color: var(--text);
  font-size: 12px;
  font-weight: 500;
  padding: 4px 8px;
  cursor: pointer;
  min-width: 140px;
  font-family: inherit;
}
.dashboard-select:focus { outline: none; border-color: var(--accent); }
.name-input {
  background: var(--bg3);
  border: 1px solid var(--accent);
  border-radius: var(--r);
  color: var(--text);
  font-size: 12px;
  font-weight: 500;
  padding: 4px 8px;
  width: 140px;
  font-family: inherit;
}
.name-input:focus { outline: none; }

.dashboard-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.widget-count {
  font-size: 10px;
  color: var(--text3);
  font-family: var(--mono);
}

.dash-btn {
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: var(--r);
  color: var(--text2);
  font-size: 11px;
  padding: 4px 10px;
  cursor: pointer;
  font-family: inherit;
  transition: border-color 0.15s, background 0.15s, color 0.15s;
  white-space: nowrap;
}
.dash-btn:hover { background: var(--bg4); border-color: var(--border2); }
.dash-btn.active { background: var(--accent); border-color: var(--accent); color: #fff; }
.dash-btn-icon { padding: 4px 6px; font-size: 12px; }
.dash-btn-danger:hover { color: var(--red); border-color: var(--red); }

/* ── Empty hint ───────────────────────────────────────────────── */
.empty-hint {
  font-size: 10.5px;
  color: var(--text3);
  background: var(--bg3);
  border: 1px dashed var(--border);
  border-radius: var(--r);
  padding: 8px 14px;
  flex-shrink: 0;
}

/* ── Category strips ──────────────────────────────────────────── */
.category-strips {
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex-shrink: 0;
}

/* ── Canvas ───────────────────────────────────────────────────── */
.canvas-area {
  flex: 1;
  min-height: 180px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: var(--r);
  padding: 10px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow: hidden;
}
.canvas-editing {
  border-style: dashed;
  border-color: var(--accent);
  background: var(--bg3);
}
.canvas-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text3);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  flex-shrink: 0;
}
.canvas-grid {
  flex: 1;
  position: relative;
  min-height: 140px;
}
.canvas-placeholder {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  color: var(--text3);
  font-style: italic;
}
</style>
