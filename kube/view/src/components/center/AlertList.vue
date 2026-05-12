<script setup>
import { computed } from 'vue'

const props = defineProps({
  alerts: { type: Array, default: () => [] },
  selectedAlert: { type: Object, default: null }
})

const emit = defineEmits(['select'])

const criticalCount = computed(() => props.alerts.filter(a => a.severity === 'critical').length)
const warningCount = computed(() => props.alerts.filter(a => a.severity === 'warning').length)

function formatTime(ts) {
  if (!ts) return '—'
  const d = new Date(ts)
  return d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
}

function sevClass(severity) {
  switch (severity) {
    case 'critical': return 'sev-critical'
    case 'warning': return 'sev-warning'
    default: return 'sev-info'
  }
}
</script>

<template>
  <div>
    <div class="section-header">
      <div class="section-title">Active Alerts</div>
      <span v-if="criticalCount" class="count-pill badge-red">{{ criticalCount }} critical</span>
      <span v-if="warningCount" class="count-pill badge-amber">{{ warningCount }} warning</span>
    </div>

    <div
      v-for="alert in alerts"
      :key="alert.id"
      class="alert-item"
      :class="{ selected: selectedAlert?.id === alert.id }"
      @click="emit('select', alert)"
    >
      <div class="alert-severity" :class="sevClass(alert.severity)"></div>
      <div class="alert-body">
        <div class="alert-title-row">
          <div class="alert-name">{{ alert.name }}</div>
          <div class="alert-namespace">{{ alert.namespace }}</div>
        </div>
        <div class="alert-desc">{{ alert.description }}</div>
        <div class="alert-tags">
          <span v-for="(tag, i) in alert.tags" :key="i" class="tag" :class="'tag-' + tag.color">
            {{ tag.label }}
          </span>
        </div>
      </div>
      <div class="alert-right">
        <div class="alert-time">{{ formatTime(alert.timestamp) }}</div>
        <div class="diagnose-btn" @click.stop="emit('select', alert)">Diagnose ↗</div>
      </div>
    </div>

    <div v-if="alerts.length === 0" class="empty-state">
      No active alerts — cluster healthy
    </div>
  </div>
</template>

<style scoped>
.section-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.section-title { font-size: 11px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--text3); }
.count-pill { padding: 1px 7px; border-radius: 10px; font-size: 10.5px; font-weight: 600; font-family: var(--mono); }

.alert-item {
  background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r);
  padding: 12px 14px; cursor: pointer; transition: all 0.15s;
  display: flex; gap: 12px; align-items: flex-start; margin-bottom: 8px;
}
.alert-item:hover { background: var(--bg4); border-color: var(--border2); }
.alert-item.selected { background: rgba(79,142,247,0.08); border-color: rgba(79,142,247,0.3); }

.alert-severity { width: 3px; border-radius: 2px; flex-shrink: 0; align-self: stretch; }
.sev-critical { background: var(--red); }
.sev-warning { background: var(--amber); }
.sev-info { background: var(--teal); }

.alert-body { flex: 1; min-width: 0; }
.alert-title-row { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.alert-name { font-size: 13px; font-weight: 500; color: var(--text); }
.alert-namespace { font-size: 10.5px; font-family: var(--mono); color: var(--text3); padding: 1px 6px; background: var(--bg5); border-radius: 4px; }
.alert-desc { font-size: 12px; color: var(--text2); line-height: 1.5; margin-bottom: 6px; }
.alert-tags { display: flex; gap: 5px; flex-wrap: wrap; }

.alert-right { display: flex; flex-direction: column; align-items: flex-end; gap: 6px; flex-shrink: 0; }
.alert-time { font-size: 10.5px; color: var(--text3); font-family: var(--mono); }

.diagnose-btn {
  padding: 4px 10px; border-radius: 6px; font-size: 11.5px; font-weight: 500;
  cursor: pointer; border: 1px solid rgba(79,142,247,0.35);
  background: rgba(79,142,247,0.1); color: var(--accent2); white-space: nowrap;
  transition: all 0.1s;
}
.diagnose-btn:hover { background: rgba(79,142,247,0.22); border-color: rgba(79,142,247,0.5); }

.empty-state { text-align: center; padding: 40px; color: var(--text3); font-size: 13px; }
</style>
