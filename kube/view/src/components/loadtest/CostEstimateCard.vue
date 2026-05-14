<script setup>
import { ref, watch, onUnmounted } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useDistLoadStore } from '../../stores/distload'

const props = defineProps({
  spec: { type: Object, required: true },
})

const store = useDistLoadStore()
const estimatedCost = ref(null)
const estimating = ref(false)
const error = ref(null)

// alive guards against the debounced callback firing after the
// component unmounts. useDebounceFn (VueUse) does NOT cancel pending
// timers on scope dispose — the returned function is a simple
// debounce wrapper without a cancel method exposed. Rather than
// re-implement timer tracking, we let the timer fire but no-op the
// callback when alive=false. Net effect: no network call after
// unmount, which is what the audit's CostEstimateCard finding
// required.
const alive = ref(true)
onUnmounted(() => { alive.value = false })

const debouncedEstimate = useDebounceFn(async (spec) => {
  if (!alive.value) return
  if (!spec.regions?.length) {
    estimatedCost.value = null
    return
  }
  estimating.value = true
  error.value = null
  try {
    const cost = await store.estimateCost(spec)
    if (!alive.value) return
    estimatedCost.value = cost
  } catch (e) {
    if (!alive.value) return
    error.value = e.message ?? String(e)
    estimatedCost.value = null
  } finally {
    if (alive.value) estimating.value = false
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
