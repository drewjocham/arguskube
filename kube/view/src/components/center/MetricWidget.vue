<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useDashboardMetrics, METRIC_CATEGORIES } from '../../composables/useDashboardMetrics'

const props = defineProps({
  metricId: { type: String, required: true },
  x: { type: Number, default: 0 },
  y: { type: Number, default: 0 },
  editMode: { type: Boolean, default: false },
})

const emit = defineEmits(['remove', 'move'])

const { getMetricValue, findMetric, metricColor, sparklines, fetchSparkline } = useDashboardMetrics()

const found = computed(() => findMetric(props.metricId))
const metricDef = computed(() => found.value?.metric || null)
const category = computed(() => found.value?.category || null)
const value = computed(() => metricDef.value ? getMetricValue(metricDef.value) : null)
const color = computed(() => metricDef.value ? metricColor(metricDef.value, value.value) : 'norm')
const formattedValue = computed(() => {
  if (value.value == null) return '—'
  return metricDef.value ? metricDef.value.format(value.value) : String(value.value)
})

const sparklineData = computed(() => sparklines[props.metricId] || [])

// Sparkline SVG path
const sparklinePath = computed(() => {
  const data = sparklineData.value
  if (!data || data.length < 2) return ''
  const vals = data.map(d => d.value)
  const min = Math.min(...vals)
  const max = Math.max(...vals) || 1
  const range = max - min || 1
  const w = 100
  const h = 30
  const points = vals.map((v, i) => {
    const px = (i / (vals.length - 1)) * w
    const py = h - ((v - min) / range) * h
    return `${px},${py}`
  })
  return `M${points.join(' L')}`
})

// Dragging state
const widgetEl = ref(null)
const dragging = ref(false)
const dragOffset = ref({ x: 0, y: 0 })

function onMouseDown(e) {
  if (!props.editMode) return
  if (e.target.closest('.widget-close')) return
  dragging.value = true
  const rect = widgetEl.value.getBoundingClientRect()
  dragOffset.value = {
    x: e.clientX - rect.left,
    y: e.clientY - rect.top,
  }
  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

function onMouseMove(e) {
  if (!dragging.value) return
  // Grid-snap to 25% columns
  const gridW = widgetEl.value.parentElement.clientWidth / 4
  const newX = Math.round(e.clientX / gridW) * gridW
  emit('move', props.metricId, newX, e.clientY)
}

function onMouseUp() {
  dragging.value = false
  document.removeEventListener('mousemove', onMouseMove)
  document.removeEventListener('mouseup', onMouseUp)
}

onMounted(() => {
  if (metricDef.value) fetchSparkline(props.metricId, metricDef.value)
})

onUnmounted(() => {
  document.removeEventListener('mousemove', onMouseMove)
  document.removeEventListener('mouseup', onMouseUp)
})
</script>

<template>
  <div
    ref="widgetEl"
    class="metric-widget"
    :class="{
      'edit-mode': editMode,
      'is-dragging': dragging,
    }"
    :style="{ left: x + 'px', top: y + 'px' }"
    @mousedown="onMouseDown"
  >
    <button
      v-if="editMode"
      class="widget-close"
      @click.stop="emit('remove', metricId)"
      title="Remove widget"
    >×</button>
    <div class="widget-header">
      <span class="widget-label">{{ metricDef?.label || metricId }}</span>
      <span class="widget-cat" v-if="category">{{ category.label }}</span>
    </div>
    <div class="widget-value" :class="'metric-' + color">
      {{ formattedValue }}
    </div>
    <svg
      v-if="sparklinePath"
      class="widget-sparkline"
      viewBox="0 0 100 30"
      preserveAspectRatio="none"
    >
      <path :d="sparklinePath" fill="none" :class="'sparkline-' + color" stroke-width="1.2" />
    </svg>
    <div v-else-if="!sparklineData.length" class="widget-sparkline widget-sparkline-empty">
      ⏳
    </div>
    <div v-else class="widget-sparkline widget-sparkline-empty">
      no data
    </div>
  </div>
</template>

<style scoped>
.metric-widget {
  position: absolute;
  width: calc(25% - 10px);
  min-height: 130px;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: var(--r);
  padding: 12px 14px 8px;
  cursor: default;
  transition: border-color 0.15s, box-shadow 0.15s;
  display: flex;
  flex-direction: column;
  gap: 4px;
  user-select: none;
}
.metric-widget:hover {
  border-color: var(--border2);
}
.metric-widget.edit-mode {
  cursor: grab;
  border-style: dashed;
  border-color: var(--accent);
}
.metric-widget.is-dragging {
  cursor: grabbing;
  opacity: 0.85;
  box-shadow: 0 4px 16px rgba(0,0,0,0.3);
  z-index: 10;
}
.widget-close {
  position: absolute;
  top: 4px;
  right: 6px;
  background: none;
  border: none;
  color: var(--text3);
  font-size: 16px;
  line-height: 1;
  cursor: pointer;
  padding: 0 4px;
  border-radius: 2px;
}
.widget-close:hover { color: var(--red); background: var(--bg4); }
.widget-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}
.widget-label {
  font-size: 11px;
  color: var(--text2);
  font-weight: 500;
  letter-spacing: 0.02em;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.widget-cat {
  font-size: 9px;
  color: var(--text3);
  background: var(--bg4);
  padding: 1px 6px;
  border-radius: 3px;
  white-space: nowrap;
}
.widget-value {
  font-size: 28px;
  font-weight: 500;
  font-family: var(--mono);
  letter-spacing: -0.03em;
  line-height: 1.1;
}
.widget-sparkline {
  width: 100%;
  height: 40px;
  margin-top: auto;
}
.widget-sparkline-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  color: var(--text3);
  height: 30px;
}

/* Colors */
.metric-up { color: var(--green); }
.metric-warn { color: var(--amber); }
.metric-crit { color: var(--red); }
.metric-norm { color: var(--text); }

.sparkline-up { stroke: var(--green); }
.sparkline-warn { stroke: var(--amber); }
.sparkline-crit { stroke: var(--red); }
.sparkline-norm { stroke: var(--accent); }
</style>
