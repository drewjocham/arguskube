<script setup>
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps({
  alerts: { type: Array, default: () => [] },
  externalLines: { type: Array, default: () => [] },
})

const logEl = ref(null)

// Map external Wails log events into display format.
const logLines = computed(() => {
  return (props.externalLines || []).map(l => ({
    time: new Date(l.timestamp).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    level: l.level,
    source: l.source,
    message: l.message,
  }))
})

// Auto-scroll to bottom on new lines.
watch(logLines, async () => {
  await nextTick()
  if (logEl.value) {
    logEl.value.scrollTop = logEl.value.scrollHeight
  }
}, { deep: true })

function levelClass(level) {
  switch (level) {
    case 'error': return 'log-err'
    case 'warn': return 'log-warn'
    case 'ok': return 'log-ok'
    default: return 'log-info'
  }
}
</script>

<template>
  <div>
    <div class="section-header">
      <div class="section-title">Live Log Stream</div>
      <div class="live-dot"></div>
    </div>
    <div class="log-stream" ref="logEl">
      <div v-for="(line, i) in logLines" :key="i">
        <span class="log-time">{{ line.time }}</span>
        <span :class="levelClass(line.level)">{{ line.source }} {{ line.message }}</span>
      </div>
      <div v-if="logLines.length === 0" class="log-empty">Waiting for log data...</div>
    </div>
  </div>
</template>

<style scoped>
.section-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.section-title { font-size: 11px; font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--text3); }
.live-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--red); animation: pulse 1s ease-in-out infinite; }

.log-stream {
  background: var(--bg); border: 1px solid var(--border); border-radius: var(--r);
  padding: 10px 12px; font-family: var(--mono); font-size: 11.5px;
  line-height: 1.7; max-height: 160px; overflow-y: auto; color: var(--text2);
}

.log-time { color: var(--text3); }
.log-err { color: var(--red2); }
.log-warn { color: var(--amber2); }
.log-ok { color: var(--green2); }
.log-info { color: var(--accent2); }
.log-empty { color: var(--text3); font-style: italic; }
</style>
