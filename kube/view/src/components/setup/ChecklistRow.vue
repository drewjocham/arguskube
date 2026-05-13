<script setup>
import { computed } from 'vue'

// One row in the Setup Checklist. Stateless: every input comes from the
// `item` prop and the button click is reported via `@action`. The parent
// decides whether to call `item.action.dispatch()` directly or to route
// the click through an action dispatcher keyed on `actionId` — both
// shapes are supported.

const props = defineProps({
  item: {
    type: Object,
    required: true,
  },
  // `busy` is driven by the parent while an action is running so we can
  // disable the button without the row's owner having to mutate the item.
  busy: { type: Boolean, default: false },
})

const emit = defineEmits(['action'])

const statusLabel = computed(() => {
  switch (props.item.status) {
    case 'ok': return 'Ready'
    case 'warn': return 'Attention'
    case 'todo': return 'To do'
    case 'error': return 'Blocker'
    case 'running': return 'Checking…'
    default: return ''
  }
})

const hasAction = computed(() => !!props.item.action && props.item.status !== 'ok')

function onClick() {
  if (!hasAction.value || props.busy) return
  emit('action', props.item)
}
</script>

<template>
  <li
    class="checklist-row"
    :data-status="item.status"
    :data-testid="`checklist-row-${item.id}`"
  >
    <span class="status-dot" :aria-label="statusLabel"></span>
    <div class="row-text">
      <div class="row-title">{{ item.title }}</div>
      <div v-if="item.detail" class="row-detail">{{ item.detail }}</div>
    </div>
    <button
      v-if="hasAction"
      class="row-action"
      :disabled="busy"
      :data-testid="`checklist-action-${item.id}`"
      @click="onClick"
    >
      {{ busy ? 'Working…' : item.action.label }}
    </button>
    <span v-else-if="item.status === 'ok'" class="row-pill ok" aria-hidden="true">✓</span>
    <span v-else class="row-pill" aria-hidden="true">·</span>
  </li>
</template>

<style scoped>
.checklist-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border, #2a2a2a);
  border-radius: 6px;
  background: var(--bg2, #1a1a1a);
  list-style: none;
}

.checklist-row[data-status="error"] { border-color: #d05a5a; }
.checklist-row[data-status="todo"]  { border-color: #d4a256; }
.checklist-row[data-status="warn"]  { border-color: #c8b25a; }
.checklist-row[data-status="ok"]    { opacity: 0.7; }

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  background: var(--text3, #5a5a5a);
}
.checklist-row[data-status="error"] .status-dot { background: #d05a5a; }
.checklist-row[data-status="todo"]  .status-dot { background: #d4a256; }
.checklist-row[data-status="warn"]  .status-dot { background: #c8b25a; }
.checklist-row[data-status="ok"]    .status-dot { background: #5ad07a; }
.checklist-row[data-status="running"] .status-dot {
  background: var(--accent, #4a9eff);
  animation: row-pulse 1.4s ease-in-out infinite;
}

@keyframes row-pulse {
  0%, 100% { opacity: 0.4; }
  50%      { opacity: 1; }
}

.row-text {
  flex: 1;
  min-width: 0;
}
.row-title {
  font-size: 13px;
  color: var(--text, #e5e5e5);
}
.row-detail {
  margin-top: 2px;
  font-size: 11px;
  color: var(--text2, #b0b0b0);
}

.row-action {
  flex-shrink: 0;
  padding: 4px 10px;
  font-size: 12px;
  background: var(--accent, #4a9eff);
  color: #fff;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.row-action:hover:not(:disabled) { filter: brightness(1.1); }
.row-action:disabled { opacity: 0.5; cursor: wait; }

.row-pill {
  flex-shrink: 0;
  font-size: 13px;
  color: var(--text3, #5a5a5a);
  width: 18px;
  text-align: center;
}
.row-pill.ok { color: #5ad07a; }
</style>
