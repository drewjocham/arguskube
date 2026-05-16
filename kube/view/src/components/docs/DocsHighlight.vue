<script setup>
import { ref, watch, onBeforeUnmount } from 'vue'

const props = defineProps({
  targetSelector: { type: String, default: '' },
})

const emit = defineEmits(['target-found', 'target-lost'])

const rect = ref({ top: 0, left: 0, width: 0, height: 0 })
const visible = ref(false)
let rafId = null
let targetEl = null

function findAndTrack() {
  if (!props.targetSelector) {
    visible.value = false
    targetEl = null
    emit('target-lost')
    return
  }
  const el = document.querySelector(props.targetSelector)
  if (!el) {
    visible.value = false
    targetEl = null
    emit('target-lost')
    return
  }
  targetEl = el
  visible.value = true
  emit('target-found')
  el.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  updateRect()
}

function updateRect() {
  if (!targetEl) return
  const r = targetEl.getBoundingClientRect()
  rect.value = { top: r.top, left: r.left, width: r.width, height: r.height }
  rafId = requestAnimationFrame(updateRect)
}

watch(
  () => props.targetSelector,
  () => {
    if (rafId) cancelAnimationFrame(rafId)
    findAndTrack()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (rafId) cancelAnimationFrame(rafId)
})
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="docs-highlight-scrim">
      <div
        class="docs-highlight-ring"
        :style="{
          top: rect.top + 'px',
          left: rect.left + 'px',
          width: rect.width + 'px',
          height: rect.height + 'px',
        }"
      >
        <div class="docs-highlight-pulse"></div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.docs-highlight-scrim {
  position: fixed;
  inset: 0;
  z-index: 500;
  pointer-events: none;
  background: rgba(0, 0, 0, 0.35);
}

.docs-highlight-ring {
  position: fixed;
  z-index: 501;
  border-radius: 6px;
  box-shadow: 0 0 0 2px var(--accent2, #4a9eff), 0 0 24px rgba(74, 158, 255, 0.5);
  transition: top 0.35s cubic-bezier(0.4, 0, 0.2, 1),
              left 0.35s cubic-bezier(0.4, 0, 0.2, 1),
              width 0.35s cubic-bezier(0.4, 0, 0.2, 1),
              height 0.35s cubic-bezier(0.4, 0, 0.2, 1);
  pointer-events: none;
  background: transparent;
}

.docs-highlight-pulse {
  width: 100%;
  height: 100%;
  border-radius: 5px;
  animation: docs-glow-pulse 2s ease-in-out infinite;
}

@keyframes docs-glow-pulse {
  0%, 100% { box-shadow: inset 0 0 8px rgba(74, 158, 255, 0.15); }
  50% { box-shadow: inset 0 0 16px rgba(74, 158, 255, 0.3); }
}
</style>
