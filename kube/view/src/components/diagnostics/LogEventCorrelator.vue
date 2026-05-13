<script setup>
import { ref } from 'vue'
import { callGo } from '../../composables/useBridge'

const props = defineProps({
  podName: { type: String, default: '' },
  namespace: { type: String, default: '' },
})

const result = ref(null)
const loading = ref(false)
const error = ref(null)

async function correlate() {
  if (!props.podName || !props.namespace) return
  loading.value = true
  error.value = null
  try {
    result.value = await callGo('CorrelatePodEvents', props.namespace, props.podName, 100)
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

function levelColor(level) {
  return level === 'error' ? '#f05454' : level === 'warning' ? '#f5a623' : '#3ecf8e'
}

function typeIcon(type) {
  return type === 'log' ? '📋' : '🔔'
}
</script>

<template>
  <div class="correlator">
    <div class="correlator-header">
      <span class="correlator-title">Log & Event Timeline</span>
      <div class="correlator-meta" v-if="result">
        {{ result.totalLogs }} log lines · {{ result.totalEvents }} events
      </div>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div class="timeline-scroll" v-if="result && result.timeline.length">
      <div v-for="(ev, i) in result.timeline" :key="i" class="tl-entry" :class="ev.type">
        <div class="tl-dot" :style="{ background: levelColor(ev.level) }"></div>
        <div class="tl-line" v-if="i < result.timeline.length - 1"></div>
        <div class="tl-body">
          <div class="tl-header-row">
            <span class="tl-type-badge" :class="ev.type">{{ typeIcon(ev.type) }} {{ ev.type }}</span>
            <span class="tl-source font-mono">{{ ev.source }}</span>
            <span class="tl-level" :style="{ color: levelColor(ev.level) }">{{ ev.level }}</span>
          </div>
          <div class="tl-msg font-mono">{{ ev.message }}</div>
        </div>
      </div>
    </div>
    <div v-else-if="!loading && !error" class="empty-state">
      <p>Click "Correlate" to see the log and event timeline for this pod.</p>
      <button class="btn-primary" @click="correlate">Correlate</button>
    </div>
    <div v-else-if="loading" class="empty-state">Loading timeline…</div>
  </div>
</template>

<style scoped>
.correlator { display: flex; flex-direction: column; gap: 12px; }
.correlator-header { display: flex; align-items: center; justify-content: space-between; }
.correlator-title { font-size: 14px; font-weight: 600; color: #e8eaec; }
.correlator-meta { font-size: 11px; color: #8b8f96; }
.error-banner { padding: 8px 12px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }
.timeline-scroll { max-height: 400px; overflow-y: auto; display: flex; flex-direction: column; gap: 0; }
.tl-entry { display: flex; gap: 10px; padding: 8px 0 8px 0; position: relative; }
.tl-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; margin-top: 4px; z-index: 1; }
.tl-line { position: absolute; left: 4px; top: 18px; width: 2px; bottom: 0; background: rgba(255,255,255,0.06); }
.tl-body { flex: 1; min-width: 0; }
.tl-header-row { display: flex; align-items: center; gap: 8px; font-size: 11px; margin-bottom: 2px; }
.tl-type-badge { padding: 1px 6px; border-radius: 3px; font-size: 10px; font-weight: 600; }
.tl-type-badge.log { background: rgba(55,148,255,0.12); color: #3794ff; }
.tl-type-badge.event { background: rgba(167,139,250,0.12); color: #a78bfa; }
.tl-source { color: #8b8f96; }
.tl-level { font-weight: 600; font-size: 10px; }
.tl-msg { font-size: 11.5px; line-height: 1.4; color: #b0b4ba; white-space: pre-wrap; word-break: break-all; font-family: var(--mono); max-height: 40px; overflow: hidden; }
.tl-msg:hover { max-height: none; }
.empty-state { text-align: center; padding: 24px; color: #8b8f96; font-size: 13px; }
.btn-primary { padding: 6px 16px; font-size: 12px; background: rgba(167,139,250,0.15); border: 1px solid rgba(167,139,250,0.3); color: #a78bfa; border-radius: 5px; cursor: pointer; margin-top: 8px; }
.font-mono { font-family: var(--mono); }
</style>
