<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useDashboardMetrics, METRIC_CATEGORIES } from '../../composables/useDashboardMetrics'

const props = defineProps({
  categoryId: { type: String, required: true },
  show: { type: Boolean, default: false },
})

const emit = defineEmits(['close', 'add-widget'])

const {
  getMetricValue, findMetric, metricColor, sparklines,
  fetchSparkline, clusterMetrics,
} = useDashboardMetrics()

const category = computed(() => METRIC_CATEGORIES.find(c => c.id === props.categoryId))

// Expanded single metric view
const expandedMetricId = ref(null)

const expandedDef = computed(() => {
  if (!expandedMetricId.value) return null
  return findMetric(expandedMetricId.value)
})

const expandedValue = computed(() => {
  const dm = expandedDef.value?.metric
  return dm ? getMetricValue(dm) : null
})

const expandedColor = computed(() => {
  const dm = expandedDef.value?.metric
  return dm ? metricColor(dm, expandedValue.value) : 'norm'
})

const expandedFormatted = computed(() => {
  const dm = expandedDef.value?.metric
  if (!dm || expandedValue.value == null) return '—'
  return dm.format(expandedValue.value)
})

const expandedSparkline = computed(() => {
  if (!expandedMetricId.value) return []
  return sparklines[expandedMetricId.value] || []
})

const expandedSparklinePath = computed(() => {
  const data = expandedSparkline.value
  if (!data || data.length < 2) return ''
  const vals = data.map(d => d.value)
  const min = Math.min(...vals)
  const max = Math.max(...vals) || 1
  const range = max - min || 1
  const w = 300
  const h = 80
  const points = vals.map((v, i) => {
    const px = (i / (vals.length - 1)) * w
    const py = h - ((v - min) / range) * h
    return `${px},${py}`
  })
  return `M${points.join(' L')}`
})

function openExpanded(metricId) {
  expandedMetricId.value = metricId
  const found = findMetric(metricId)
  if (found) fetchSparkline(metricId, found.metric)
}

function closeExpanded() {
  expandedMetricId.value = null
}

function getMetricCardValue(metricDef) {
  const v = getMetricValue(metricDef)
  if (v == null) return '—'
  return metricDef.format(v)
}

function getMetricCardColor(metricDef) {
  const v = getMetricValue(metricDef)
  return metricColor(metricDef, v)
}

function onAddWidget(metricId) {
  emit('add-widget', metricId)
}

// Close on Escape
function onKeyDown(e) {
  if (e.key === 'Escape') {
    if (expandedMetricId.value) closeExpanded()
    else emit('close')
  }
}

watch(() => props.show, (v) => {
  if (v) {
    document.addEventListener('keydown', onKeyDown)
    // Fetch sparklines for all metrics in the category
    if (category.value) {
      for (const m of category.value.metrics) {
        fetchSparkline(m.id, m)
      }
    }
  } else {
    document.removeEventListener('keydown', onKeyDown)
    expandedMetricId.value = null
  }
})
</script>

<template>
  <Teleport to="body">
    <div v-if="show" class="popup-backdrop" @click.self="emit('close')">
      <div class="popup-container">
        <!-- Header -->
        <div class="popup-header">
          <div class="popup-title">
            <span v-if="expandedMetricId" class="popup-back" @click="closeExpanded">←</span>
            {{ expandedMetricId ? expandedDef?.metric?.label : (category?.label || 'Metrics') }}
          </div>
          <button class="popup-close" @click="emit('close')">×</button>
        </div>

        <!-- Expanded single metric view -->
        <div v-if="expandedMetricId" class="popup-expanded">
          <div class="expanded-value-row">
            <span class="expanded-label">{{ expandedDef?.metric?.label }}</span>
            <span class="expanded-value" :class="'metric-' + expandedColor">
              {{ expandedFormatted }}
            </span>
          </div>
          <div class="expanded-meta">
            Category: {{ expandedDef?.category?.label || '—' }}
            &nbsp;·&nbsp;
            Query: <code>{{ expandedDef?.metric?.query }}</code>
          </div>
          <svg
            v-if="expandedSparklinePath"
            class="expanded-chart"
            viewBox="0 0 300 80"
            preserveAspectRatio="xMidYMid meet"
          >
            <path
              :d="expandedSparklinePath"
              fill="none"
              :class="'sparkline-' + expandedColor"
              stroke-width="2"
            />
          </svg>
          <div v-else class="expanded-chart expanded-chart-empty">
            No time-series data available yet
          </div>
          <button
            class="popup-add-btn"
            @click="onAddWidget(expandedMetricId)"
          >
            + Add to Dashboard
          </button>
        </div>

        <!-- Metric grid (when NOT expanded) -->
        <div v-else class="popup-grid">
          <div
            v-for="m in category?.metrics || []"
            :key="m.id"
            class="popup-card"
            :class="'card-' + getMetricCardColor(m)"
            @click="openExpanded(m.id)"
          >
            <div class="popup-card-label">{{ m.label }}</div>
            <div class="popup-card-value" :class="'metric-' + getMetricCardColor(m)">
              {{ getMetricCardValue(m) }}
            </div>
            <div class="popup-card-query">{{ m.query }}</div>
            <button
              class="popup-card-add"
              @click.stop="onAddWidget(m.id)"
              title="Add to Dashboard"
            >+</button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.popup-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,0.55);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
}
.popup-container {
  background: var(--bg2);
  border: 1px solid var(--border2);
  border-radius: var(--r);
  width: min(680px, 90vw);
  max-height: 75vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.popup-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}
.popup-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  display: flex;
  align-items: center;
  gap: 8px;
}
.popup-back {
  cursor: pointer;
  color: var(--accent);
  font-size: 16px;
  padding: 0 4px;
}
.popup-back:hover { color: var(--accent2); }
.popup-close {
  background: none;
  border: none;
  color: var(--text3);
  font-size: 18px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 3px;
}
.popup-close:hover { color: var(--text); background: var(--bg4); }

/* Grid view */
.popup-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 10px;
  padding: 16px;
  overflow-y: auto;
  flex: 1;
}
.popup-card {
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: var(--r);
  padding: 12px 14px;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
  position: relative;
}
.popup-card:hover {
  background: var(--bg4);
  border-color: var(--border2);
}
.popup-card.card-warn { border-left: 2px solid var(--amber); }
.popup-card.card-crit { border-left: 2px solid var(--red); }
.popup-card.card-up   { border-left: 2px solid var(--green); }

.popup-card-label {
  font-size: 10.5px;
  color: var(--text3);
  margin-bottom: 4px;
  letter-spacing: 0.02em;
}
.popup-card-value {
  font-size: 24px;
  font-weight: 500;
  font-family: var(--mono);
  letter-spacing: -0.02em;
  margin-bottom: 6px;
}
.popup-card-query {
  font-size: 9px;
  color: var(--text3);
  font-family: var(--mono);
  opacity: 0.7;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.popup-card-add {
  position: absolute;
  top: 6px;
  right: 8px;
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: 3px;
  width: 20px;
  height: 20px;
  font-size: 14px;
  line-height: 1;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.15s;
}
.popup-card:hover .popup-card-add { opacity: 1; }
.popup-card-add:hover { background: var(--accent2); }

/* Expanded single metric view */
.popup-expanded {
  padding: 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  overflow-y: auto;
  flex: 1;
}
.expanded-value-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}
.expanded-label {
  font-size: 13px;
  color: var(--text2);
  font-weight: 500;
}
.expanded-value {
  font-size: 36px;
  font-weight: 500;
  font-family: var(--mono);
  letter-spacing: -0.03em;
}
.expanded-meta {
  font-size: 10px;
  color: var(--text3);
}
.expanded-meta code {
  font-family: var(--mono);
  font-size: 10px;
  color: var(--accent);
  background: var(--bg4);
  padding: 1px 5px;
  border-radius: 2px;
}
.expanded-chart {
  width: 100%;
  height: 120px;
  background: var(--bg);
  border-radius: var(--r);
  border: 1px solid var(--border);
}
.expanded-chart-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  color: var(--text3);
}
.popup-add-btn {
  align-self: flex-start;
  padding: 6px 14px;
  background: var(--accent);
  color: #fff;
  border: none;
  border-radius: var(--r);
  font-size: 12px;
  cursor: pointer;
  font-weight: 500;
}
.popup-add-btn:hover { background: var(--accent2); }

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
