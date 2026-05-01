<script setup>
import { ref, inject, watch, nextTick, computed } from 'vue'
import { useChat } from '../../composables/useWails'

const props = defineProps({
  selectedAlert: { type: Object, default: null },
  bundle: { type: Object, default: null },
  loading: { type: Boolean, default: false },
  error: { type: String, default: null },
})

const emit = defineEmits(['diagnose'])
const isAllowed = inject('isAllowed')
const tier = inject('tier')

const question = ref('')
const chatScrollEl = ref(null)
const activeTab = ref('diagnostics')

const { history, sending, autoSummary, eventLog, sendMessage, refreshHistory, fetchAutoSummary, fetchEventLog } = useChat()

// When selected alert changes, fetch auto-summary and chat history.
watch(() => props.selectedAlert?.id, async (alertId) => {
  if (alertId) {
    await fetchAutoSummary(alertId)
    await refreshHistory(alertId)
    activeTab.value = 'diagnostics'
  }
}, { immediate: true })

// Periodically poll for auto-summary (agent investigates in background).
let summaryPoll = null
watch(() => props.selectedAlert?.id, (alertId) => {
  if (summaryPoll) clearInterval(summaryPoll)
  if (alertId) {
    summaryPoll = setInterval(() => fetchAutoSummary(alertId), 3000)
  }
}, { immediate: true })

// Auto-scroll chat to bottom.
watch(history, async () => {
  await nextTick()
  if (chatScrollEl.value) {
    chatScrollEl.value.scrollTop = chatScrollEl.value.scrollHeight
  }
}, { deep: true })

watch(activeTab, (tab) => {
  if (tab === 'activity') fetchEventLog()
})

const visibleHistory = computed(() =>
  (history.value || []).filter(m => m.role !== 'system')
)

async function onSend() {
  const val = question.value.trim()
  if (!val || !props.selectedAlert) return
  question.value = ''
  activeTab.value = 'chat'
  try {
    await sendMessage(props.selectedAlert.id, val)
  } catch (e) {
    console.error('[chat send]', e)
  }
}

function onKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    onSend()
  }
}

function formatContext(alert) {
  if (!alert) return ''
  const lines = []
  lines.push(`${alert.severity.toUpperCase()} · ${alert.name}`)
  if (alert.podName) lines.push(`pod: ${alert.podName}`)
  lines.push(`ns: ${alert.namespace} · restarts: ${alert.restartCount}`)
  if (alert.memoryLimit) lines.push(`mem: ${alert.memoryLimit} limit / ${alert.memoryRequest || '—'} req`)
  if (alert.cpuLimit) lines.push(`cpu: ${alert.cpuLimit} limit / ${alert.cpuRequest || '—'} req`)
  if (alert.nodeName) lines.push(`node: ${alert.nodeName}`)
  if (alert.imageTag) lines.push(`image: ${alert.imageTag}`)
  return lines.join('\n')
}

function formatTime(ts) {
  if (!ts) return ''
  return new Date(ts).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
}

function eventTypeIcon(type_) {
  switch (type_) {
    case 'alert': return '●'
    case 'resolution': return '✓'
    case 'investigation': return '◉'
    case 'pattern': return '◈'
    default: return '·'
  }
}

function eventTypeColor(type_) {
  switch (type_) {
    case 'alert': return 'var(--red2)'
    case 'resolution': return 'var(--green2)'
    case 'investigation': return 'var(--accent2)'
    case 'pattern': return 'var(--purple)'
    default: return 'var(--text3)'
  }
}
</script>

<template>
  <div class="ai-panel" :class="{ 'pro-gate locked': !isAllowed('ai_diagnostics') }">

    <!-- Pro upgrade CTA overlay -->
    <div v-if="!isAllowed('ai_diagnostics')" class="pro-overlay">
      <div class="pro-overlay-icon">
        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
      </div>
      <div class="pro-overlay-title">Argus Diagnostics requires Pro</div>
      <div class="pro-overlay-desc">AI-powered root cause analysis, contextual chat, and auto-investigation for every alert.</div>
      <a class="pro-overlay-btn" href="https://kubewatcher.dev/pro" target="_blank">Upgrade to Pro</a>
    </div>

    <!-- Header -->
    <div class="ai-panel-header">
      <div class="ai-orb"></div>
      <div class="ai-panel-title">Argus</div>
      <div class="ai-panel-sub">{{ selectedAlert?.name || 'Select an alert' }}</div>
    </div>

    <!-- Tabs -->
    <div class="panel-tabs">
      <div class="panel-tab" :class="{ active: activeTab === 'diagnostics' }" @click="activeTab = 'diagnostics'">
        Diagnostics
      </div>
      <div class="panel-tab" :class="{ active: activeTab === 'chat' }" @click="activeTab = 'chat'">
        Chat
        <span v-if="visibleHistory.length > 0" class="tab-count">{{ visibleHistory.length }}</span>
      </div>
      <div class="panel-tab" :class="{ active: activeTab === 'activity' }" @click="activeTab = 'activity'">
        Activity
      </div>
    </div>

    <!-- Diagnostics tab -->
    <div v-show="activeTab === 'diagnostics'" class="ai-scroll">
      <div v-if="!selectedAlert" class="empty-state">
        Click an alert to see Argus diagnostics
      </div>

      <div v-else-if="loading" class="loading-state">
        <div class="ai-orb" style="width: 28px; height: 28px; margin-bottom: 8px;"></div>
        Assembling context...
      </div>

      <div v-else-if="error" class="error-state">{{ error }}</div>

      <template v-else>
        <!-- Auto-investigation summary -->
        <div v-if="autoSummary" class="auto-summary">
          <div class="auto-summary-header">
            <div class="auto-summary-dot"></div>
            Argus Investigation
            <span class="auto-summary-time">{{ formatTime(autoSummary.timestamp) }}</span>
          </div>
          <div class="auto-summary-body">{{ autoSummary.summary }}</div>
        </div>

        <div v-else-if="selectedAlert && !bundle?.diagnosis" class="investigating-state">
          <div class="ai-orb" style="width: 20px; height: 20px;"></div>
          <span>Argus is investigating...</span>
        </div>

        <div class="ai-context-block">
          <div class="ai-block-label">
            <div class="ai-block-dot" style="background: var(--red);"></div>
            Alert Context
          </div>
          <pre class="ai-code">{{ formatContext(selectedAlert) }}</pre>
        </div>

        <div v-if="bundle?.diagnosis" class="ai-insight">
          <div class="ai-insight-title">Root Cause Hypothesis</div>
          <div class="ai-insight-body">{{ bundle.diagnosis.hypothesis }}</div>
        </div>

        <div v-if="bundle?.cascadeAlerts?.length" class="ai-insight cascade-insight">
          <div class="ai-insight-title cascade-title">Cascade Alert</div>
          <div class="ai-insight-body">
            Related alerts detected: {{ bundle.cascadeAlerts.map(a => a.name).join(', ') }}
          </div>
        </div>

        <div v-if="bundle?.anomalyResults?.length" class="ai-context-block">
          <div class="ai-block-label">
            <div class="ai-block-dot" style="background: var(--teal);"></div>
            Anomstack Anomaly Detection
          </div>
          <div v-for="(ar, i) in bundle.anomalyResults" :key="i" class="ai-code">
            {{ ar.metricName }}: {{ ar.isAnomaly ? 'ANOMALY' : 'normal' }} (score: {{ ar.score?.toFixed(2) }}, model: {{ ar.modelUsed }})
          </div>
        </div>

        <div v-if="bundle?.diagnosis?.steps?.length">
          <div class="ai-block-label">
            <div class="ai-block-dot" style="background: var(--accent);"></div>
            Recommended Actions
          </div>
          <div class="ai-steps">
            <div v-for="step in bundle.diagnosis.steps" :key="step.number" class="ai-step">
              <div class="step-num">{{ step.number }}</div>
              <div class="step-text">
                {{ step.text }}
                <span v-if="step.command" class="step-cmd">{{ step.command }}</span>
              </div>
            </div>
          </div>
        </div>

        <div v-if="bundle?.decisionLog?.length" class="ai-context-block">
          <div class="ai-block-label">
            <div class="ai-block-dot" style="background: var(--purple);"></div>
            From DECISION_LOG.md
          </div>
          <pre v-for="(entry, i) in bundle.decisionLog" :key="i" class="ai-code">
# {{ entry.date }}
{{ entry.content }}</pre>
        </div>
      </template>
    </div>

    <!-- Chat tab -->
    <div v-show="activeTab === 'chat'" class="ai-scroll" ref="chatScrollEl">
      <div v-if="visibleHistory.length === 0" class="empty-state">
        Ask the agent about this alert
      </div>

      <div v-for="(msg, i) in visibleHistory" :key="i" class="chat-msg" :class="'chat-' + msg.role">
        <div class="chat-role">{{ msg.role === 'assistant' ? 'Argus' : 'You' }}</div>
        <div class="chat-content">{{ msg.content }}</div>
        <div class="chat-time">{{ formatTime(msg.timestamp) }}</div>
      </div>

      <div v-if="sending" class="chat-typing">
        <div class="typing-dots"><span></span><span></span><span></span></div>
        Argus is thinking...
      </div>
    </div>

    <!-- Activity tab -->
    <div v-show="activeTab === 'activity'" class="ai-scroll">
      <div v-if="eventLog.length === 0" class="empty-state">
        Argus activity will appear here
      </div>

      <div v-for="(event, i) in [...eventLog].reverse().slice(0, 50)" :key="i" class="activity-item">
        <span class="activity-icon" :style="{ color: eventTypeColor(event.type) }">{{ eventTypeIcon(event.type) }}</span>
        <div class="activity-body">
          <div class="activity-summary">{{ event.summary }}</div>
          <div class="activity-meta">
            {{ formatTime(event.timestamp) }}
            <span v-if="event.namespace"> · {{ event.namespace }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Input -->
    <div class="ai-input-area">
      <textarea
        class="ai-input"
        v-model="question"
        :placeholder="selectedAlert ? 'Ask about this alert...' : 'Select an alert first'"
        rows="1"
        :disabled="!selectedAlert"
        @keydown="onKeydown"
      ></textarea>
      <button class="ai-send" @click="onSend" :disabled="!selectedAlert || sending">
        <svg width="13" height="13" viewBox="0 0 13 13" fill="none">
          <path d="M2 6.5h9M7.5 3L11 6.5L7.5 10" stroke="white" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.ai-panel {
  width: 310px; border-left: 1px solid var(--border); background: var(--bg2);
  display: flex; flex-direction: column; flex-shrink: 0;
}

.ai-panel-header {
  padding: 10px 14px; border-bottom: 1px solid var(--border);
  display: flex; align-items: center; gap: 8px;
}
.ai-orb {
  width: 22px; height: 22px; border-radius: 50%;
  background: conic-gradient(from 0deg, var(--accent), var(--purple), var(--teal), var(--accent));
  animation: spin 4s linear infinite; flex-shrink: 0;
}
.ai-panel-title { font-size: 12.5px; font-weight: 500; color: var(--text); }
.ai-panel-sub { font-size: 11px; color: var(--text3); margin-left: auto; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 140px; }

.panel-tabs { display: flex; border-bottom: 1px solid var(--border); padding: 0 8px; }
.panel-tab {
  padding: 7px 10px; font-size: 11.5px; font-weight: 400; color: var(--text3);
  cursor: pointer; border-bottom: 2px solid transparent; transition: all 0.12s;
  display: flex; align-items: center; gap: 4px;
}
.panel-tab:hover { color: var(--text2); }
.panel-tab.active { color: var(--accent2); border-bottom-color: var(--accent); font-weight: 500; }
.tab-count {
  font-size: 9px; font-family: var(--mono); padding: 1px 5px; border-radius: 8px;
  background: rgba(79,142,247,0.15); color: var(--accent2);
}

.ai-scroll { flex: 1; overflow-y: auto; padding: 12px; display: flex; flex-direction: column; gap: 10px; }

.auto-summary {
  background: rgba(62,207,142,0.06); border: 1px solid rgba(62,207,142,0.2);
  border-radius: var(--r2); padding: 10px 12px;
}
.auto-summary-header {
  font-size: 10px; font-weight: 600; letter-spacing: 0.07em; color: var(--green2);
  text-transform: uppercase; margin-bottom: 6px; display: flex; align-items: center; gap: 5px;
}
.auto-summary-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--green); animation: pulse 1.5s ease-in-out infinite; }
.auto-summary-time { margin-left: auto; font-weight: 400; color: var(--text3); text-transform: none; letter-spacing: 0; }
.auto-summary-body { font-size: 12px; color: var(--text2); line-height: 1.65; white-space: pre-wrap; }

.investigating-state {
  display: flex; align-items: center; gap: 8px; padding: 10px 12px;
  background: rgba(79,142,247,0.06); border: 1px solid rgba(79,142,247,0.15);
  border-radius: var(--r2); font-size: 12px; color: var(--text2);
}

.ai-context-block { background: var(--bg3); border: 1px solid var(--border); border-radius: var(--r2); padding: 10px 12px; }
.ai-block-label { font-size: 10px; font-weight: 600; letter-spacing: 0.07em; color: var(--text3); text-transform: uppercase; margin-bottom: 7px; display: flex; align-items: center; gap: 5px; }
.ai-block-dot { width: 5px; height: 5px; border-radius: 50%; }
.ai-code { font-family: var(--mono); font-size: 11px; color: var(--text2); line-height: 1.6; white-space: pre-wrap; word-break: break-all; }

.ai-insight { background: rgba(79,142,247,0.08); border: 1px solid rgba(79,142,247,0.2); border-radius: var(--r2); padding: 10px 12px; }
.ai-insight-title { font-size: 11.5px; font-weight: 500; color: var(--accent2); margin-bottom: 5px; }
.ai-insight-body { font-size: 12px; color: var(--text2); line-height: 1.6; }
.cascade-insight { border-color: rgba(245,166,35,0.3); background: rgba(245,166,35,0.06); }
.cascade-title { color: var(--amber2); }

.ai-steps { display: flex; flex-direction: column; gap: 6px; }
.ai-step { display: flex; gap: 9px; align-items: flex-start; padding: 8px 10px; background: var(--bg3); border-radius: var(--r2); border: 1px solid var(--border); cursor: pointer; transition: border-color 0.1s; }
.ai-step:hover { border-color: var(--border2); }
.step-num { width: 18px; height: 18px; border-radius: 50%; background: var(--bg5); display: flex; align-items: center; justify-content: center; font-size: 10px; font-weight: 600; color: var(--text2); flex-shrink: 0; margin-top: 1px; }
.step-text { font-size: 12px; color: var(--text2); line-height: 1.5; }
.step-cmd { display: block; font-family: var(--mono); font-size: 10.5px; color: var(--accent2); margin-top: 3px; padding: 2px 6px; background: rgba(79,142,247,0.08); border-radius: 4px; width: fit-content; }

.chat-msg { padding: 8px 10px; border-radius: var(--r2); font-size: 12px; line-height: 1.6; }
.chat-user { background: rgba(79,142,247,0.08); border: 1px solid rgba(79,142,247,0.15); align-self: flex-end; max-width: 90%; }
.chat-assistant { background: var(--bg3); border: 1px solid var(--border); align-self: flex-start; max-width: 95%; }
.chat-role { font-size: 10px; font-weight: 600; color: var(--text3); margin-bottom: 3px; text-transform: uppercase; letter-spacing: 0.05em; }
.chat-content { color: var(--text2); white-space: pre-wrap; word-break: break-word; }
.chat-time { font-size: 10px; color: var(--text3); margin-top: 3px; text-align: right; }

.chat-typing { display: flex; align-items: center; gap: 8px; padding: 8px 10px; font-size: 12px; color: var(--text3); }
.typing-dots { display: flex; gap: 3px; }
.typing-dots span { width: 4px; height: 4px; border-radius: 50%; background: var(--accent); animation: typing 1.2s ease-in-out infinite; }
.typing-dots span:nth-child(2) { animation-delay: 0.2s; }
.typing-dots span:nth-child(3) { animation-delay: 0.4s; }

.activity-item { display: flex; gap: 8px; padding: 6px 0; border-bottom: 1px solid var(--border); align-items: flex-start; }
.activity-item:last-child { border-bottom: none; }
.activity-icon { font-size: 10px; margin-top: 2px; flex-shrink: 0; }
.activity-body { flex: 1; min-width: 0; }
.activity-summary { font-size: 11.5px; color: var(--text2); line-height: 1.4; }
.activity-meta { font-size: 10px; color: var(--text3); margin-top: 2px; }

.ai-input-area { padding: 10px 12px; border-top: 1px solid var(--border); display: flex; gap: 7px; align-items: flex-end; }
.ai-input { flex: 1; background: var(--bg3); border: 1px solid var(--border2); border-radius: 8px; padding: 7px 10px; color: var(--text); font-family: var(--font); font-size: 12.5px; resize: none; outline: none; transition: border-color 0.15s; min-height: 34px; max-height: 80px; }
.ai-input:focus { border-color: rgba(79,142,247,0.5); }
.ai-input::placeholder { color: var(--text3); }
.ai-input:disabled { opacity: 0.5; }

.ai-send { width: 32px; height: 32px; border-radius: 8px; background: var(--accent); border: none; cursor: pointer; display: flex; align-items: center; justify-content: center; transition: background 0.1s; flex-shrink: 0; }
.ai-send:hover:not(:disabled) { background: var(--accent2); }
.ai-send:disabled { opacity: 0.5; cursor: default; }

.empty-state, .loading-state, .error-state { display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 40px 16px; color: var(--text3); font-size: 13px; text-align: center; }
.error-state { color: var(--red2); }

@keyframes typing { 0%, 100% { opacity: 0.3; transform: scale(0.8); } 50% { opacity: 1; transform: scale(1); } }

/* Pro gate overlay */
.ai-panel.locked { position: relative; }
.ai-panel.locked > *:not(.pro-overlay):not(.ai-panel-header) { filter: blur(3px); opacity: 0.3; pointer-events: none; }
.pro-overlay {
  position: absolute; top: 50px; left: 0; right: 0; z-index: 10;
  display: flex; flex-direction: column; align-items: center; gap: 10px;
  padding: 32px 24px; text-align: center;
}
.pro-overlay-icon { color: var(--purple); opacity: 0.7; }
.pro-overlay-title { font-size: 14px; font-weight: 600; color: var(--text); }
.pro-overlay-desc { font-size: 12px; color: var(--text3); line-height: 1.5; max-width: 240px; }
.pro-overlay-btn {
  display: inline-block; margin-top: 4px; padding: 7px 20px;
  background: linear-gradient(135deg, var(--accent) 0%, var(--purple) 100%);
  color: #fff; font-size: 12px; font-weight: 600; border-radius: 6px;
  text-decoration: none; transition: opacity 0.2s;
}
.pro-overlay-btn:hover { opacity: 0.9; }
</style>
