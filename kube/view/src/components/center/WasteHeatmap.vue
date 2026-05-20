<script setup>
import { ref, computed, onMounted, nextTick } from 'vue'
import { callGo } from '../../composables/useBridge'

// Over-Provisioning Heatmap — cluster-wide view. On mount we pull
// ProfileClusterWaste once (paginates internally across every
// namespace) and render a grid of namespace cards sorted critical
// first. Each card shows the namespace's waste score + top 4
// deployments. Click → modal with the full deployment list.
//
// This replaces the earlier picker+Analyze flow which only showed
// one namespace at a time.

const profiles = ref([])
const loading = ref(false)
const error = ref('')
const search = ref('')

// Modal state — when set, this namespace's full deployment list
// renders in a scrollable popup driven by the native <dialog>
// element. Null = closed.
const expanded = ref(null)
const dialogRef = ref(null)

async function loadAll() {
  loading.value = true
  error.value = ''
  try {
    const out = await callGo('ProfileClusterWaste')
    profiles.value = Array.isArray(out) ? out : []
  } catch (e) {
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

onMounted(loadAll)

const filteredProfiles = computed(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return profiles.value
  return profiles.value.filter((p) => p.namespace.toLowerCase().includes(q))
})

const totals = computed(() => {
  const counts = { critical: 0, high: 0, medium: 0, low: 0, unknown: 0 }
  for (const p of profiles.value) {
    counts[p.score] = (counts[p.score] || 0) + 1
  }
  return counts
})

function scoreColor(score) {
  return {
    critical: '#f05454',
    high: '#f5a623',
    medium: '#3794ff',
    low: '#3ecf8e',
    unknown: '#8b8f96',
  }[score] || '#8b8f96'
}

// Each card surfaces the top 4 deployments by waste-CPU value.
// Deployments are pre-sorted by the backend, so this is just a slice.
function topDeployments(p) {
  return (p.deployments || []).slice(0, 4)
}

async function openExpanded(p) {
  expanded.value = p
  // Wait for Vue to render the dialog's content before calling
  // showModal() — without this, the dialog opens with stale or
  // missing children on the first open.
  await nextTick()
  dialogRef.value?.showModal?.()
}

function closeExpanded() {
  // close() is a no-op on an already-closed dialog, so this works
  // both as the @close handler (Escape / form-method=dialog submit)
  // AND as the explicit ×-button handler. Always clear `expanded`
  // last so the v-if doesn't unmount the children before close()
  // finishes its animation/event chain.
  if (dialogRef.value?.open) dialogRef.value.close()
  expanded.value = null
}

// Native <dialog>'s click event fires on the dialog element itself
// when the user clicks the backdrop (the dialog has a "::backdrop"
// pseudo-element, but the click event targets the dialog). When the
// target IS the dialog (not an inner descendant), close.
function onDialogClick(e) {
  if (e.target === dialogRef.value) closeExpanded()
}
</script>

<template>
  <div class="heatmap-view">
    <div class="header">
      <div>
        <div class="title">Over-Provisioning Heatmap</div>
        <div class="subtitle">
          Cluster-wide view — namespaces sorted by estimated waste,
          worst first. Click a card to see all workloads in that
          namespace.
        </div>
      </div>
      <button class="btn-rescan" :disabled="loading" @click="loadAll">
        {{ loading ? 'Scanning…' : '↻ Re-scan' }}
      </button>
    </div>

    <div class="summary-bar">
      <input
        v-model="search"
        class="search-input"
        type="search"
        placeholder="Filter by namespace…"
        data-testid="waste-search"
      />
      <div class="totals">
        <span class="total-pill" :data-tone="'critical'" v-if="totals.critical">{{ totals.critical }} critical</span>
        <span class="total-pill" :data-tone="'high'" v-if="totals.high">{{ totals.high }} high</span>
        <span class="total-pill" :data-tone="'medium'" v-if="totals.medium">{{ totals.medium }} medium</span>
        <span class="total-pill" :data-tone="'low'" v-if="totals.low">{{ totals.low }} low</span>
        <span class="total-pill" :data-tone="'unknown'" v-if="totals.unknown">{{ totals.unknown }} unknown</span>
      </div>
    </div>

    <div v-if="error" class="error-banner" data-testid="waste-error-banner">
      <span class="error-dot"></span>
      <span class="error-msg">{{ error }}</span>
      <button class="btn-retry" @click="loadAll" :disabled="loading">Retry</button>
    </div>

    <div v-if="loading && !profiles.length" class="state-box">Scanning every namespace in the cluster…</div>
    <div v-else-if="!loading && !filteredProfiles.length && !error" class="state-box">
      <template v-if="search">No namespace matches "{{ search }}".</template>
      <template v-else>No namespaces found in this cluster.</template>
    </div>

    <div v-else class="ns-grid" data-testid="waste-ns-grid">
      <button
        v-for="p in filteredProfiles"
        :key="p.namespace"
        type="button"
        class="ns-card"
        :data-score="p.score"
        :data-testid="`waste-ns-card-${p.namespace}`"
        :aria-label="`${p.namespace}: ${p.score} waste`"
        @click="openExpanded(p)"
      >
        <div class="ns-header">
          <span class="ns-name font-mono">{{ p.namespace }}</span>
          <span class="score-pill" :style="{ color: scoreColor(p.score) }">{{ p.score }}</span>
        </div>
        <div v-if="p.error" class="ns-error">{{ p.error }}</div>
        <template v-else>
          <div class="ns-totals">
            <span>CPU <strong>{{ p.totalWasteCPU }}</strong></span>
            <span>Mem <strong>{{ p.totalWasteMem }}</strong></span>
          </div>
          <div v-if="topDeployments(p).length" class="dep-list">
            <div
              v-for="d in topDeployments(p)"
              :key="d.name"
              class="dep-row"
            >
              <span class="dep-name font-mono">{{ d.name }}</span>
              <span class="dep-waste" :style="{ color: scoreColor(p.score) }">{{ d.wasteCPU }}</span>
            </div>
          </div>
          <div v-else class="dep-empty">No deployments with measurable waste.</div>
          <div v-if="(p.deployments || []).length > 4" class="more-hint">
            +{{ (p.deployments || []).length - 4 }} more — click to expand
          </div>
        </template>
      </button>
    </div>

    <!-- Per-namespace popup. Renders every deployment in a scrollable
         list so the user can dig past the top-4 preview. Backdrop
         click and Escape both close it. The native HTML dialog
         element gives us a real focus trap and platform Escape
         handling for free, which a div with a role attribute does
         not. -->
    <dialog
      ref="dialogRef"
      class="modal"
      data-testid="waste-modal"
      :aria-label="expanded ? `${expanded.namespace} waste detail` : ''"
      @close="closeExpanded"
      @click="onDialogClick"
    >
      <div v-if="expanded" class="modal-inner">
        <div class="modal-header">
          <div>
            <div class="modal-title font-mono">{{ expanded.namespace }}</div>
            <div class="modal-sub">
              <span class="score-pill" :style="{ color: scoreColor(expanded.score) }">{{ expanded.score }}</span>
              CPU <strong>{{ expanded.totalWasteCPU }}</strong>
              <span class="dot">·</span>
              Mem <strong>{{ expanded.totalWasteMem }}</strong>
              <span class="dot">·</span>
              {{ (expanded.deployments || []).length }} deployment{{ (expanded.deployments || []).length === 1 ? '' : 's' }}
            </div>
          </div>
          <button
            class="modal-close"
            aria-label="Close"
            data-testid="waste-modal-close"
            @click="closeExpanded"
          >×</button>
        </div>
        <div class="modal-body">
          <div v-if="expanded.error" class="state-box state-error">{{ expanded.error }}</div>
          <div v-else-if="!(expanded.deployments || []).length" class="state-box">
            No deployments with measurable waste in this namespace.
          </div>
          <div v-else class="modal-table">
            <div class="modal-row modal-row-head">
              <div>Workload</div>
              <div>Requested</div>
              <div>Estimated</div>
              <div>Waste</div>
              <div>Ratio</div>
            </div>
            <div
              v-for="d in expanded.deployments"
              :key="d.name"
              class="modal-row"
            >
              <div class="cell-name font-mono">{{ d.name }}</div>
              <div>{{ d.cpuRequest }} / {{ d.memoryRequest }}</div>
              <div>{{ d.cpuUsage || '—' }} / {{ d.memoryUsage || '—' }}</div>
              <div :style="{ color: scoreColor(expanded.score) }">{{ d.wasteCPU }} / {{ d.wasteMem }}</div>
              <div>{{ d.ratio }}</div>
            </div>
          </div>
        </div>
      </div>
    </dialog>
  </div>
</template>

<style scoped>
.heatmap-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}
.header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
/* Subtitle bumped from #8b8f96 (3.7:1) to var(--text2) (~7:1)
   to clear WCAG AA on the dark bg. Sonar css:S7924. */
.header .subtitle { font-size: 13px; color: var(--text2, #b0b4ba); max-width: 640px; }
.btn-rescan {
  /* Solid neutral button bg + white text — translucent variants
     fail AA against the surrounding wash per Sonar css:S7924. */
  background: #2a2e35;
  border: 1px solid #3d424c;
  color: #fff;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  cursor: pointer;
  flex-shrink: 0;
}
.btn-rescan:hover:not(:disabled) { background: #3a4049; }
.btn-rescan:disabled { opacity: 0.5; cursor: not-allowed; }

.summary-bar { display: flex; align-items: center; gap: 16px; flex-wrap: wrap; }
.search-input {
  flex: 1;
  min-width: 200px;
  background: var(--bg3);
  border: 1px solid var(--border);
  color: var(--text);
  padding: 7px 12px;
  border-radius: 6px;
  font-size: 13px;
  font-family: inherit;
}
.search-input:focus { outline: none; border-color: var(--accent2); }
.totals { display: flex; gap: 6px; flex-wrap: wrap; }
.total-pill {
  font-size: 11px;
  padding: 3px 10px;
  border-radius: 10px;
  font-family: var(--mono);
  color: #fff;
}
.total-pill[data-tone="critical"] { background: #b8392f; }
.total-pill[data-tone="high"]     { background: #8c5a0c; }
.total-pill[data-tone="medium"]   { background: #2c5f9f; }
.total-pill[data-tone="low"]      { background: #1d6f49; }
.total-pill[data-tone="unknown"]  { background: #4a4f57; }

.error-banner {
  display: flex; align-items: center; gap: 12px;
  padding: 8px 12px;
  background: rgba(240,84,84,0.12);
  border: 1px solid rgba(240,84,84,0.45);
  border-radius: 6px;
  color: var(--text, #e8eaec);
  font-size: 12px;
}
.error-dot { width: 6px; height: 6px; border-radius: 50%; background: #f05454; flex-shrink: 0; }
.error-msg { flex: 1; }
.btn-retry {
  padding: 3px 10px;
  font-size: 11px;
  background: #b8392f;
  border: 1px solid #b8392f;
  color: #fff;
  border-radius: 4px;
  cursor: pointer;
  flex-shrink: 0;
}
.btn-retry:hover:not(:disabled) { background: #d23f33; border-color: #d23f33; }
.btn-retry:disabled { opacity: 0.4; cursor: not-allowed; }

.state-box {
  background: rgba(255,255,255,0.03);
  border: 1px solid rgba(255,255,255,0.08);
  padding: 16px;
  border-radius: 6px;
  font-size: 13px;
  color: var(--text2, #b0b4ba);
}
.state-error { color: #f15c5c; border-color: rgba(241,92,92,0.3); }

/* ── Card grid ────────────────────────────────────────────────── */
.ns-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}
.ns-card {
  background: #1e2023;
  border: 1px solid rgba(255,255,255,0.08);
  border-left: 3px solid rgba(255,255,255,0.08);
  border-radius: 8px;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s, transform 0.08s;
}
.ns-card:hover { background: #23262a; }
.ns-card:focus { outline: 1px solid var(--accent2); outline-offset: 1px; }
.ns-card:active { transform: scale(0.995); }
.ns-card[data-score="critical"] { border-left-color: #f05454; }
.ns-card[data-score="high"]     { border-left-color: #f5a623; }
.ns-card[data-score="medium"]   { border-left-color: #3794ff; }
.ns-card[data-score="low"]      { border-left-color: #3ecf8e; }
.ns-card[data-score="unknown"]  { border-left-color: #8b8f96; opacity: 0.85; }

.ns-header { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.ns-name { color: var(--text); font-size: 13px; font-weight: 600; word-break: break-all; }
.score-pill {
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 700;
  flex-shrink: 0;
}
.ns-totals {
  display: flex;
  gap: 12px;
  font-size: 11px;
  color: var(--text3);
}
.ns-totals strong { color: var(--text); font-weight: 600; }
.ns-error {
  /* Solid amber fill + white text (PR #129 warn-badge palette)
     instead of #f5a623 on a faint same-hue wash — the latter
     sits at the AA threshold per Sonar css:S7924. */
  font-size: 11px;
  color: #fff;
  padding: 4px 6px;
  background: #8c5a0c;
  border-radius: 4px;
  word-break: break-all;
}
.dep-list { display: flex; flex-direction: column; gap: 4px; }
.dep-row {
  display: flex; justify-content: space-between; align-items: baseline;
  gap: 8px;
  font-size: 11px;
  padding: 3px 6px;
  background: rgba(255,255,255,0.02);
  border-radius: 3px;
}
.dep-name { color: var(--text); word-break: break-all; }
.dep-waste { font-family: var(--mono); flex-shrink: 0; font-weight: 600; }
/* dep-empty + more-hint bumped to text2 so they read on the card
   background (text3 fails AA at 10.5-11px on the dark wash). */
.dep-empty { font-size: 11px; color: var(--text2, #b0b4ba); font-style: italic; }
.more-hint {
  font-size: 10.5px;
  color: var(--text2, #b0b4ba);
  font-style: italic;
  text-align: right;
}

/* ── Modal popup (native <dialog>) ───────────────────────────── */
.modal {
  background: #1a1c1f;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 8px;
  max-width: 900px;
  width: calc(100vw - 48px);
  max-height: 80vh;
  padding: 0;
  color: var(--text, #e8eaec);
}
.modal::backdrop {
  background: rgba(0,0,0,0.6);
}
.modal-inner {
  display: flex;
  flex-direction: column;
  max-height: calc(80vh - 2px);
  overflow: hidden;
}
.modal-header {
  display: flex; justify-content: space-between; align-items: flex-start;
  gap: 16px;
  padding: 14px 18px;
  border-bottom: 1px solid rgba(255,255,255,0.08);
}
.modal-title { font-size: 16px; color: var(--text); font-weight: 600; word-break: break-all; }
.modal-sub {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text2);
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
}
.modal-sub strong { color: var(--text); font-weight: 600; }
.modal-sub .dot { color: var(--text3); }
.modal-close {
  background: none;
  border: none;
  color: var(--text3);
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
  padding: 4px 10px;
  flex-shrink: 0;
}
.modal-close:hover { color: var(--text); }
.modal-body { flex: 1; overflow-y: auto; padding: 14px 18px; }

.modal-table { display: flex; flex-direction: column; }
.modal-row {
  display: grid;
  grid-template-columns: 2fr 1.5fr 1.5fr 1.5fr 0.7fr;
  gap: 8px;
  padding: 8px 4px;
  font-size: 12px;
  color: var(--text);
  border-bottom: 1px solid rgba(255,255,255,0.04);
}
.modal-row:last-child { border-bottom: none; }
.modal-row-head {
  color: var(--text3);
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid rgba(255,255,255,0.08);
}
.cell-name { word-break: break-all; }
.font-mono { font-family: var(--mono); }
</style>
