<script setup>
import { onMounted, onUnmounted } from 'vue'

// ContextMenu — a minimal viewport-positioned popover used for the
// sidebar section right-click menu (§C3) and any future quick-actions
// surface. Stateless: parent owns open/close + items.
//
// Props:
//   x, y     — viewport-absolute pixel coordinates (from event.clientX/Y)
//   items    — [{ id, label, disabled?, danger? }]
//   testId   — optional data-testid prefix
//
// Emits:
//   select(id) — when a non-disabled item is clicked
//   close       — outside click, escape key, or after a select

const props = defineProps({
  x: { type: Number, required: true },
  y: { type: Number, required: true },
  items: { type: Array, required: true },
  testId: { type: String, default: 'context-menu' },
})

const emit = defineEmits(['select', 'close'])

function onItemClick(item) {
  if (item.disabled) return
  emit('select', item.id)
  emit('close')
}

function onDocClick(e) {
  // Don't dismiss when the click is inside the menu — Vue's @click on
  // each item runs first, fires select+close, and then this handler
  // sees a click on a detached node (item is already removed). The
  // explicit containment check makes the order irrelevant.
  if (e.target?.closest?.(`[data-testid="${props.testId}"]`)) return
  emit('close')
}

function onKeydown(e) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => {
  document.addEventListener('mousedown', onDocClick)
  document.addEventListener('keydown', onKeydown)
})
onUnmounted(() => {
  document.removeEventListener('mousedown', onDocClick)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <Teleport to="body">
    <div
      class="context-menu"
      :style="{ top: y + 'px', left: x + 'px' }"
      :data-testid="testId"
      role="menu"
    >
      <button
        v-for="it in items"
        :key="it.id"
        type="button"
        class="context-item"
        :class="{ disabled: it.disabled, danger: it.danger }"
        :disabled="it.disabled"
        :data-testid="`${testId}-${it.id}`"
        role="menuitem"
        @click.stop="onItemClick(it)"
      >{{ it.label }}</button>
    </div>
  </Teleport>
</template>

<style scoped>
.context-menu {
  position: fixed;
  z-index: 200;
  background: var(--bg2, #1a1a1a);
  border: 1px solid var(--border, #2a2a2a);
  border-radius: 6px;
  padding: 4px;
  min-width: 180px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
}
.context-item {
  display: block;
  width: 100%;
  text-align: left;
  background: none;
  border: none;
  color: var(--text, #e5e5e5);
  padding: 6px 10px;
  font-size: 12px;
  font-family: inherit;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
}
.context-item:hover:not(.disabled) {
  background: var(--bg3, #222);
}
.context-item.disabled {
  color: var(--text3, #5a5a5a);
  cursor: default;
}
.context-item.danger { color: var(--red, #d05a5a); }
.context-item.danger:hover:not(.disabled) {
  background: rgba(208, 90, 90, 0.12);
}
</style>
