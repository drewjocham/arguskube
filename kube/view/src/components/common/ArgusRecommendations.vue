<script setup>
import { ref, computed } from 'vue'

// ArgusRecommendations is the shared in-view recommendations panel.
// Drop it under any list/detail surface and pass a list of records
// shaped:
//   {
//     id: 'stable-key',
//     severity: 'info' | 'warning' | 'critical',
//     title: 'short summary',
//     reasoning: 'longer prose, 1–3 sentences',
//     evidence: { key: 'value', ... }  // optional, rendered as a list
//     suggestedYAML: '...'              // optional, shows Apply button
//   }
//
// Each card collapses to the title + severity dot. Click → expands
// to show the reasoning and the evidence table. When suggestedYAML
// is present an "Apply suggested fix" button surfaces under the
// evidence — the parent owns the actual apply (we just emit), since
// only the parent knows what to do post-apply (refresh, navigate,
// etc.).

const props = defineProps({
  recommendations: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
  // Title shown above the panel — defaults to "Argus recommendations".
  title: { type: String, default: 'Argus recommendations' },
  // When true, the panel collapses to a header bar even when there
  // ARE recommendations. Lets the parent show a quiet pill in tight
  // layouts; user expands by clicking.
  collapsed: { type: Boolean, default: false },
})

const emit = defineEmits(['apply-fix', 'refresh'])

// Per-card expanded state — kept by id so re-renders don't collapse
// what the user just opened. dismissed: same shape — frontend-only
// hide for THIS session; refreshing the list brings them back.
const expanded = ref(new Set())
const dismissed = ref(new Set())
const panelOpen = ref(!props.collapsed)

const visible = computed(() =>
  props.recommendations.filter(r => !dismissed.value.has(r.id))
)

function toggleCard(id) {
  const next = new Set(expanded.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expanded.value = next
}

function dismiss(id, ev) {
  ev?.stopPropagation?.()
  const next = new Set(dismissed.value)
  next.add(id)
  dismissed.value = next
}

function severityClass(sev) {
  switch (sev) {
    case 'critical': return 'sev-crit'
    case 'warning':  return 'sev-warn'
    case 'info':     return 'sev-info'
    default:         return 'sev-info'
  }
}

function severityLabel(sev) {
  return (sev || 'info').toUpperCase()
}
</script>

<template>
  <section class="argus-recs" data-testid="argus-recommendations">
    <header class="recs-header" @click="panelOpen = !panelOpen">
      <span class="recs-title">{{ title }}</span>
      <span v-if="!loading && visible.length" class="recs-count">{{ visible.length }}</span>
      <span v-if="loading" class="recs-count loading">…</span>
      <button
        type="button"
        class="recs-refresh"
        :disabled="loading"
        :title="loading ? 'Refreshing…' : 'Re-scan'"
        @click.stop="emit('refresh')"
      >↻</button>
      <span class="recs-caret" :class="{ open: panelOpen }">▸</span>
    </header>

    <div v-show="panelOpen" class="recs-body">
      <div v-if="loading && !visible.length" class="recs-empty">Scanning…</div>
      <div v-else-if="!visible.length" class="recs-empty">
        Nothing to recommend right now. This view is in good shape.
      </div>
      <ul v-else class="recs-list">
        <li
          v-for="r in visible"
          :key="r.id"
          class="rec-card"
          :class="severityClass(r.severity)"
          :data-testid="`recommendation-${r.id}`"
        >
          <button
            type="button"
            class="rec-summary"
            :aria-expanded="expanded.has(r.id)"
            @click="toggleCard(r.id)"
          >
            <span class="rec-dot" :class="severityClass(r.severity)" :aria-label="severityLabel(r.severity)"></span>
            <span class="rec-title">{{ r.title }}</span>
            <span class="rec-caret" :class="{ open: expanded.has(r.id) }">▸</span>
          </button>
          <button
            type="button"
            class="rec-dismiss"
            :aria-label="`Dismiss ${r.title}`"
            title="Hide this recommendation for the session"
            @click="dismiss(r.id, $event)"
          >×</button>
          <div v-if="expanded.has(r.id)" class="rec-detail">
            <p class="rec-reasoning">{{ r.reasoning }}</p>
            <dl v-if="r.evidence && Object.keys(r.evidence).length" class="rec-evidence">
              <template v-for="(val, key) in r.evidence" :key="key">
                <dt>{{ key }}</dt>
                <dd class="font-mono">{{ val }}</dd>
              </template>
            </dl>
            <div v-if="r.suggestedYAML" class="rec-actions">
              <button
                type="button"
                class="rec-apply"
                :data-testid="`recommendation-apply-${r.id}`"
                @click="emit('apply-fix', r)"
              >Apply suggested fix</button>
            </div>
          </div>
        </li>
      </ul>
    </div>
  </section>
</template>

<style scoped>
.argus-recs {
  background: #1a1c1f;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 8px;
  overflow: hidden;
}
.recs-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: rgba(167,139,250,0.06);
  border-bottom: 1px solid rgba(255,255,255,0.04);
  cursor: pointer;
  user-select: none;
}
.recs-header:hover { background: rgba(167,139,250,0.1); }
.recs-title { font-size: 12px; font-weight: 600; color: var(--text, #e8eaec); letter-spacing: 0.02em; }
.recs-count {
  font-size: 10.5px;
  padding: 2px 8px;
  border-radius: 10px;
  background: rgba(167,139,250,0.2);
  color: #c4b3fd;
  font-family: var(--mono, ui-monospace, monospace);
}
.recs-count.loading { background: rgba(255,255,255,0.06); color: var(--text2, #b0b4ba); }
.recs-refresh {
  margin-left: auto;
  background: transparent;
  border: 1px solid rgba(255,255,255,0.08);
  color: var(--text2, #b0b4ba);
  padding: 2px 8px;
  font-size: 11px;
  border-radius: 4px;
  cursor: pointer;
}
.recs-refresh:hover:not(:disabled) { color: var(--text, #fff); }
.recs-refresh:disabled { opacity: 0.4; cursor: not-allowed; }
.recs-caret { font-size: 10px; color: var(--text3, #8b8f96); transition: transform 0.15s; }
.recs-caret.open { transform: rotate(90deg); }

.recs-body { padding: 8px 12px; }
.recs-empty { font-size: 12px; color: var(--text2, #b0b4ba); padding: 8px 4px; }

.recs-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
.rec-card {
  background: #23262a;
  border: 1px solid rgba(255,255,255,0.06);
  border-left: 3px solid transparent;
  border-radius: 5px;
  position: relative;
}
.rec-card.sev-crit { border-left-color: #f05454; }
.rec-card.sev-warn { border-left-color: #f5a623; }
.rec-card.sev-info { border-left-color: #4f8ef7; }

.rec-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  background: none;
  border: none;
  padding: 8px 36px 8px 10px;
  font: inherit;
  font-size: 12px;
  text-align: left;
  color: var(--text, #e8eaec);
  cursor: pointer;
}
.rec-summary:hover { background: rgba(255,255,255,0.04); }
.rec-dot { width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0; }
.rec-dot.sev-crit { background: #f05454; }
.rec-dot.sev-warn { background: #f5a623; }
.rec-dot.sev-info { background: #4f8ef7; }
.rec-title { flex: 1; }
.rec-caret { font-size: 9px; color: var(--text3, #8b8f96); transition: transform 0.15s; }
.rec-caret.open { transform: rotate(90deg); }

.rec-dismiss {
  position: absolute;
  top: 4px;
  right: 6px;
  background: none;
  border: none;
  color: var(--text3, #8b8f96);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 3px;
}
.rec-dismiss:hover { color: var(--text, #fff); background: rgba(255,255,255,0.06); }

.rec-detail {
  padding: 4px 12px 12px;
  border-top: 1px solid rgba(255,255,255,0.04);
  background: rgba(0,0,0,0.15);
}
.rec-reasoning {
  margin: 8px 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text2, #b0b4ba);
}
.rec-evidence {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px 12px;
  font-size: 11px;
  margin: 8px 0 0;
}
.rec-evidence dt { color: var(--text3, #8b8f96); text-transform: uppercase; letter-spacing: 0.05em; font-size: 10px; }
.rec-evidence dd { margin: 0; color: var(--text, #e8eaec); word-break: break-all; }

.rec-actions {
  margin-top: 10px;
  display: flex;
  gap: 6px;
}
.rec-apply {
  background: #6d4ade;
  border: 1px solid #6d4ade;
  color: #fff;
  padding: 5px 12px;
  font-size: 11px;
  border-radius: 4px;
  cursor: pointer;
  font: inherit;
  font-size: 11px;
}
.rec-apply:hover { background: #5a3bc7; border-color: #5a3bc7; }

.font-mono { font-family: var(--mono, ui-monospace, SFMono-Regular, Menlo, monospace); }
</style>
