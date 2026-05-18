<script setup>
import { computed } from 'vue'
import { useDashboardMetrics, METRIC_CATEGORIES } from '../../composables/useDashboardMetrics'

const props = defineProps({
  categoryId: { type: String, required: true },
})

const emit = defineEmits(['expand'])

const {
  getCategoryToggled, toggleCategoryMetric,
  getMetricValue, findMetric, metricColor, sparklines,
} = useDashboardMetrics()

const category = computed(() => METRIC_CATEGORIES.find(c => c.id === props.categoryId))

const toggledMetricIds = computed(() => getCategoryToggled(props.categoryId))

const toggledMetrics = computed(() =>
  toggledMetricIds.value
    .map(id => {
      const found = findMetric(id)
      return found ? { id, def: found.metric, category: found.category } : null
    })
    .filter(Boolean)
)

function getCardValue(metricId) {
  const found = findMetric(metricId)
  if (!found) return '—'
  const v = getMetricValue(found.metric)
  if (v == null) return '—'
  return found.metric.format(v)
}

function getCardColor(metricId) {
  const found = findMetric(metricId)
  if (!found) return 'norm'
  const v = getMetricValue(found.metric)
  return metricColor(found.metric, v)
}

function getMiniSparklinePath(metricId) {
  const data = sparklines[metricId]
  if (!data || data.length < 2) return ''
  const vals = data.map(d => d.value)
  const min = Math.min(...vals)
  const max = Math.max(...vals) || 1
  const range = max - min || 1
  const w = 50
  const h = 14
  const points = vals.map((v, i) => {
    const px = (i / (vals.length - 1)) * w
    const py = h - ((v - min) / range) * h
    return `${px},${py}`
  })
  return `M${points.join(' L')}`
}
</script>

<template>
  <div class="category-strip">
    <div class="strip-header">
      <span class="strip-label">{{ category?.label || categoryId }}</span>
      <span class="strip-count">{{ category?.metrics?.length || 0 }} metrics</span>
    </div>
    <div class="strip-cards">
      <div
        v-for="tm in toggledMetrics"
        :key="tm.id"
        class="strip-card"
        :class="'card-' + getCardColor(tm.id)"
        @click="toggleCategoryMetric(categoryId, tm.id)"
        :title="'Click to rotate metric'"
      >
        <span class="strip-card-label">{{ tm.def.label }}</span>
        <span class="strip-card-value" :class="'metric-' + getCardColor(tm.id)">
          {{ getCardValue(tm.id) }}
        </span>
        <svg
          v-if="getMiniSparklinePath(tm.id)"
          class="strip-card-sparkline"
          viewBox="0 0 50 14"
          preserveAspectRatio="none"
        >
          <path
            :d="getMiniSparklinePath(tm.id)"
            fill="none"
            :class="'spark-' + getCardColor(tm.id)"
            stroke-width="1"
          />
        </svg>
      </div>
    </div>
    <button class="strip-expand" @click="emit('expand', categoryId)" title="Show all metrics">
      <span class="expand-icon">▤</span>
    </button>
  </div>
</template>

<style scoped>
.category-strip {
  display: flex;
  align-items: stretch;
  gap: 8px;
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: var(--r);
  padding: 8px 10px;
  transition: border-color 0.15s;
}
.category-strip:hover {
  border-color: var(--border2);
}

.strip-header {
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-width: 90px;
  gap: 2px;
}
.strip-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--text2);
  letter-spacing: 0.03em;
  text-transform: uppercase;
}
.strip-count {
  font-size: 9px;
  color: var(--text3);
}

.strip-cards {
  display: flex;
  gap: 8px;
  flex: 1;
}
.strip-card {
  flex: 1;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: calc(var(--r) - 1px);
  padding: 6px 10px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 100px;
  position: relative;
}
.strip-card:hover {
  background: var(--bg4);
  border-color: var(--border2);
}
.strip-card.card-warn { border-bottom: 2px solid var(--amber); }
.strip-card.card-crit { border-bottom: 2px solid var(--red); }
.strip-card.card-up   { border-bottom: 2px solid var(--green); }

.strip-card-label {
  font-size: 9px;
  color: var(--text3);
  letter-spacing: 0.02em;
}
.strip-card-value {
  font-size: 16px;
  font-weight: 500;
  font-family: var(--mono);
  letter-spacing: -0.02em;
  line-height: 1;
}
.strip-card-sparkline {
  position: absolute;
  bottom: 4px;
  right: 6px;
  width: 50px;
  height: 14px;
  opacity: 0.6;
}

.strip-expand {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--r);
  cursor: pointer;
  color: var(--text3);
  font-size: 12px;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}
.strip-expand:hover {
  color: var(--accent);
  border-color: var(--accent);
  background: var(--bg4);
}
.expand-icon {
  font-size: 14px;
  line-height: 1;
}

/* Colors */
.metric-up { color: var(--green); }
.metric-warn { color: var(--amber); }
.metric-crit { color: var(--red); }
.metric-norm { color: var(--text); }

.spark-up { stroke: var(--green); }
.spark-warn { stroke: var(--amber); }
.spark-crit { stroke: var(--red); }
.spark-norm { stroke: var(--accent); }
</style>
