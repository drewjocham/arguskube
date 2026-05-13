<script setup>
import { ref, computed, watch, onBeforeUnmount, nextTick } from 'vue'

// Select — single-source styled dropdown to replace every native <select>
// in the app. We rolled our own (instead of styling <select>) because the
// native control on macOS/Wails ignores most CSS and renders the OS chrome,
// which clashes with the rest of the dark UI.
//
// Shape: pass `options` as either an array of strings or `{value, label,
// disabled?}` objects. v-model binds the selected value. `placeholder` is
// shown when the value is empty/null/undefined.
//
// Keyboard model: Enter/Space opens, Esc closes, Arrow keys move highlight,
// Enter selects. Outside-click closes. Focus stays on the trigger so
// keyboard users don't get stranded inside the panel.

const props = defineProps({
  modelValue: { type: [String, Number, null, Boolean], default: null },
  options: { type: Array, required: true },
  placeholder: { type: String, default: 'Select…' },
  disabled: { type: Boolean, default: false },
  size: { type: String, default: 'normal' }, // 'sm' | 'normal'
  ariaLabel: { type: String, default: '' },
  testid: { type: String, default: '' },
  width: { type: String, default: '' }, // e.g. '180px'
})
const emit = defineEmits(['update:modelValue', 'change'])

const triggerEl = ref(null)
const panelEl = ref(null)
const open = ref(false)
const highlightIndex = ref(-1)

const normalized = computed(() =>
  props.options.map((o) => (
    typeof o === 'object' && o !== null
      ? { value: o.value, label: o.label ?? String(o.value), disabled: !!o.disabled }
      : { value: o, label: String(o), disabled: false }
  ))
)

const selected = computed(() => normalized.value.find((o) => o.value === props.modelValue) || null)

function toggle() {
  if (props.disabled) return
  open.value ? close() : openPanel()
}
function openPanel() {
  if (props.disabled) return
  open.value = true
  highlightIndex.value = Math.max(0, normalized.value.findIndex((o) => o.value === props.modelValue))
  nextTick(() => {
    const item = panelEl.value?.querySelector('.option.highlight')
    if (typeof item?.scrollIntoView === 'function') item.scrollIntoView({ block: 'nearest' })
  })
}
function close() {
  open.value = false
  highlightIndex.value = -1
}
function pick(opt) {
  if (opt.disabled) return
  emit('update:modelValue', opt.value)
  emit('change', opt.value)
  close()
  triggerEl.value?.focus()
}

function onKeydown(e) {
  if (props.disabled) return
  if (!open.value) {
    if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown') {
      e.preventDefault()
      openPanel()
    }
    return
  }
  switch (e.key) {
    case 'Escape':
      e.preventDefault()
      close()
      break
    case 'ArrowDown':
      e.preventDefault()
      moveHighlight(1)
      break
    case 'ArrowUp':
      e.preventDefault()
      moveHighlight(-1)
      break
    case 'Home':
      e.preventDefault()
      highlightIndex.value = firstEnabled(0, 1)
      break
    case 'End':
      e.preventDefault()
      highlightIndex.value = firstEnabled(normalized.value.length - 1, -1)
      break
    case 'Enter':
    case ' ': {
      e.preventDefault()
      const opt = normalized.value[highlightIndex.value]
      if (opt) pick(opt)
      break
    }
  }
}

function moveHighlight(delta) {
  const n = normalized.value.length
  if (!n) return
  let i = highlightIndex.value
  for (let step = 0; step < n; step++) {
    i = (i + delta + n) % n
    if (!normalized.value[i].disabled) {
      highlightIndex.value = i
      nextTick(() => {
        const item = panelEl.value?.querySelector('.option.highlight')
        if (typeof item?.scrollIntoView === 'function') item.scrollIntoView({ block: 'nearest' })
      })
      return
    }
  }
}
function firstEnabled(start, dir) {
  const n = normalized.value.length
  for (let i = 0; i < n; i++) {
    const idx = (start + i * dir + n) % n
    if (!normalized.value[idx].disabled) return idx
  }
  return -1
}

function onDocClick(e) {
  if (!open.value) return
  const t = triggerEl.value
  const p = panelEl.value
  if (t?.contains(e.target) || p?.contains(e.target)) return
  close()
}

watch(open, (v) => {
  if (v) document.addEventListener('mousedown', onDocClick, true)
  else document.removeEventListener('mousedown', onDocClick, true)
})
onBeforeUnmount(() => document.removeEventListener('mousedown', onDocClick, true))
</script>

<template>
  <div
    class="argus-select"
    :class="{ disabled, open, sm: size === 'sm' }"
    :style="width ? { width } : null"
  >
    <button
      ref="triggerEl"
      type="button"
      class="trigger"
      :disabled="disabled"
      :aria-haspopup="'listbox'"
      :aria-expanded="open"
      :aria-label="ariaLabel || placeholder"
      :data-testid="testid || undefined"
      @click="toggle"
      @keydown="onKeydown"
    >
      <span class="value" :class="{ placeholder: !selected }">
        {{ selected ? selected.label : placeholder }}
      </span>
      <span class="chev" aria-hidden="true">▾</span>
    </button>
    <div v-if="open" ref="panelEl" class="panel" role="listbox" :aria-label="ariaLabel || placeholder">
      <div
        v-for="(opt, i) in normalized"
        :key="String(opt.value) + i"
        class="option"
        :class="{
          selected: opt.value === props.modelValue,
          highlight: i === highlightIndex,
          disabled: opt.disabled,
        }"
        role="option"
        :aria-selected="opt.value === props.modelValue"
        :aria-disabled="opt.disabled"
        @mouseenter="highlightIndex = i"
        @mousedown.prevent="pick(opt)"
      >
        {{ opt.label }}
      </div>
      <div v-if="!normalized.length" class="empty">No options</div>
    </div>
  </div>
</template>

<style scoped>
.argus-select { position: relative; display: inline-block; min-width: 120px; }
.argus-select.disabled { opacity: 0.55; pointer-events: none; }

.trigger {
  width: 100%;
  display: inline-flex; align-items: center; justify-content: space-between; gap: 8px;
  background: var(--bg3); color: var(--text);
  border: 1px solid var(--border);
  border-radius: var(--r, 6px);
  padding: 6px 10px;
  font-size: 13px; line-height: 1.2;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
  font-family: inherit;
}
.argus-select.sm .trigger { padding: 4px 8px; font-size: 12px; }
.trigger:hover:not(:disabled) { background: var(--bg4); border-color: var(--border2); }
.trigger:focus-visible { outline: 2px solid var(--accent, #4f8cff); outline-offset: 1px; }
.argus-select.open .trigger { border-color: var(--accent, #4f8cff); }

.value { flex: 1; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.value.placeholder { color: var(--text2); }
.chev { font-size: 10px; color: var(--text2); transition: transform 0.15s; }
.argus-select.open .chev { transform: rotate(180deg); }

.panel {
  position: absolute; top: calc(100% + 4px); left: 0; right: 0;
  background: var(--bg2, var(--bg3)); color: var(--text);
  border: 1px solid var(--border2, var(--border));
  border-radius: var(--r, 6px);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.35);
  z-index: 1000;
  max-height: 280px; overflow-y: auto;
  padding: 4px;
}

.option {
  padding: 6px 10px;
  font-size: 13px;
  border-radius: 4px;
  cursor: pointer;
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.argus-select.sm .option { padding: 4px 8px; font-size: 12px; }
.option.highlight { background: var(--bg4); }
.option.selected { color: var(--accent, #4f8cff); font-weight: 500; }
.option.selected.highlight { background: var(--bg4); }
.option.disabled { opacity: 0.45; cursor: not-allowed; }
.empty { padding: 8px 10px; color: var(--text2); font-size: 12px; }
</style>
