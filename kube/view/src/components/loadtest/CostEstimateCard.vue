<script setup>
import { ref, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useDistLoadStore } from '../../stores/distload'

const props = defineProps({
  spec: { type: Object, required: true },
})

const store = useDistLoadStore()
const estimatedCost = ref(null)
const estimating = ref(false)
const error = ref(null)

// useDebounceFn replaces the hand-rolled setTimeout-+-clearTimeout
// dance the audit flagged. VueUse internally tracks the timer via
// tryOnScopeDispose so the callback can't fire after the component's
// effect scope is torn down — no manual onUnmounted cleanup needed.
const debouncedEstimate = useDebounceFn(async (spec) => {
  if (!spec.regions?.length) {
    estimatedCost.value = null
    return
  }
  estimating.value = true
  error.value = null
  try {
    const cost = await store.estimateCost(spec)
    estimatedCost.value = cost
  } catch (e) {
    error.value = e.message ?? String(e)
    estimatedCost.value = null
  } finally {
    estimating.value = false
  }
}, 500)

watch(() => props.spec, (spec) => {
  debouncedEstimate(spec)
}, { deep: true, immediate: false })
</script>

<template>
  <div class="cost-estimate">
    <span class="cost-label">Estimated cost:</span>
    <span v-if="estimating" class="cost-value estimating">Calculating…</span>
    <span v-else-if="estimatedCost != null" class="cost-value">
      <strong>{{ estimatedCost.toFixed(1) }}</strong> credits
    </span>
    <span v-else-if="error" class="cost-value error">{{ error }}</span>
    <span v-else class="cost-value hint">—</span>
  </div>
</template>

<style scoped>
.cost-estimate { display: flex; align-items: center; gap: 8px; font-size: 13px; }
.cost-label { color: var(--text2); }
.cost-value { color: var(--text); }
.cost-value.estimating { color: var(--text3); font-style: italic; }
.cost-value.error { color: #d05858; }
.cost-value.hint { color: var(--text3); }
</style>
