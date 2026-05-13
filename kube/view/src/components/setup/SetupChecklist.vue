<script setup>
import { ref, computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useSetupChecklistStore } from '../../stores/setupChecklist'
import ChecklistRow from './ChecklistRow.vue'

// Container for the "Get Argus ready" section in Settings. Reads the
// store's already-sorted items list and renders one ChecklistRow each.
// The component is intentionally dumb about what individual probes mean
// — that's the producer's job. When everything is `ok`, the panel
// collapses to a one-line "Argus is ready" pill so it stops being noise.

const store = useSetupChecklistStore()
const { items, allGreen, blockerCount, warnCount } = storeToRefs(store)

const expanded = ref(true)

const summary = computed(() => {
  if (allGreen.value) return 'Argus is ready'
  if (blockerCount.value > 0) {
    const n = blockerCount.value
    return `${n} setup ${n === 1 ? 'item' : 'items'} need your attention`
  }
  if (warnCount.value > 0) {
    return `${warnCount.value} warning${warnCount.value === 1 ? '' : 's'}`
  }
  return 'Setup checks running…'
})

// Tracks the row whose action is currently executing, so we can disable
// its button while the producer's dispatch resolves.
const busyId = ref('')

async function onRowAction(item) {
  if (!item?.action) return
  const dispatch = item.action.dispatch
  if (typeof dispatch !== 'function') return
  busyId.value = item.id
  try {
    await dispatch()
  } finally {
    busyId.value = ''
  }
}
</script>

<template>
  <section class="setup-checklist" data-testid="setup-checklist">
    <header class="header">
      <button
        class="toggle"
        :aria-expanded="expanded"
        @click="expanded = !expanded"
      >
        <span class="caret" :class="{ open: expanded }" aria-hidden="true">▶</span>
        <h2 class="title">Get Argus ready</h2>
        <span
          class="summary"
          :data-status="allGreen ? 'ok' : (blockerCount > 0 ? 'todo' : 'warn')"
          data-testid="checklist-summary"
        >{{ summary }}</span>
      </button>
    </header>

    <ul v-if="expanded && items.length > 0" class="rows" data-testid="checklist-rows">
      <ChecklistRow
        v-for="it in items"
        :key="it.id"
        :item="it"
        :busy="busyId === it.id"
        @action="onRowAction"
      />
    </ul>

    <p v-else-if="expanded && items.length === 0" class="empty">
      Argus is checking your setup…
    </p>
  </section>
</template>

<style scoped>
.setup-checklist {
  margin-bottom: 18px;
  padding: 14px;
  background: var(--bg, #141414);
  border: 1px solid var(--border, #2a2a2a);
  border-radius: 8px;
}

.header { margin-bottom: 12px; }
.toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
  text-align: left;
  padding: 0;
}

.caret {
  display: inline-block;
  width: 12px;
  font-size: 10px;
  color: var(--text3, #5a5a5a);
  transition: transform 0.15s ease;
}
.caret.open { transform: rotate(90deg); }

.title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text, #e5e5e5);
}

.summary {
  margin-left: auto;
  font-size: 12px;
  color: var(--text2, #b0b0b0);
}
.summary[data-status="ok"]   { color: #5ad07a; }
.summary[data-status="todo"] { color: #d4a256; }
.summary[data-status="warn"] { color: #c8b25a; }

.rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
  list-style: none;
  margin: 0;
  padding: 0;
}

.empty {
  margin: 0;
  font-size: 12px;
  color: var(--text3, #5a5a5a);
  font-style: italic;
}
</style>
