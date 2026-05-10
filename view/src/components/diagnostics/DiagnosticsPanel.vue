<script setup>
import { ref, inject, watch, nextTick, computed, onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useChat } from '../../composables/useWails'
import { parseCodeBlocks } from '../../utils/parseCodeBlocks'
import { renderMarkdown } from '../../utils/renderMarkdown'
import { useUIPrefsStore } from '../../stores/uiPrefs'
import { useArgusContextStore } from '../../stores/argusContext'
import CodeBlock from '../ai/CodeBlock.vue'

const GLOBAL_ALERT_ID = 'global'

// activeAlertId is what useChat() should fetch/send against. Falls back to
// 'global' when there's no selected alert so the panel works as a free-form
// Argus AI assistant in the right rail.
function alertIdFor(alert) {
  return alert?.id || GLOBAL_ALERT_ID
}

const uiPrefs = useUIPrefsStore()
const { rightPanelWidth, rightPanelMin, rightPanelMax } = storeToRefs(uiPrefs)

const argusContext = useArgusContextStore()
const { pending: pendingContext } = storeToRefs(argusContext)

// Resize state — drag the left edge of the panel to widen/narrow it.
let dragStartX = 0
let dragStartWidth = 0
let dragging = false

function onResizeMouseDown(e) {
  dragStartX = e.clientX
  dragStartWidth = rightPanelWidth.value
  dragging = true
  document.addEventListener('mousemove', onResizeMouseMove)
  document.addEventListener('mouseup', onResizeMouseUp)
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'
  e.preventDefault()
}

function onResizeMouseMove(e) {
  if (!dragging) return
  // Pulling the handle leftwards INCREASES the panel width.
  const next = dragStartWidth + (dragStartX - e.clientX)
  uiPrefs.setRightPanelWidth(next)
}

function onResizeMouseUp() {
  dragging = false
  document.removeEventListener('mousemove', onResizeMouseMove)
  document.removeEventListener('mouseup', onResizeMouseUp)
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
}

function onResizeKey(e) {
  // Keyboard accessibility: arrow keys nudge the width.
  const step = e.shiftKey ? 24 : 8
  if (e.key === 'ArrowLeft') {
    uiPrefs.setRightPanelWidth(rightPanelWidth.value + step)
    e.preventDefault()
  } else if (e.key === 'ArrowRight') {
    uiPrefs.setRightPanelWidth(rightPanelWidth.value - step)
    e.preventDefault()
  }
}

onUnmounted(() => {
  document.removeEventListener('mousemove', onResizeMouseMove)
  document.removeEventListener('mouseup', onResizeMouseUp)
})

function popOut() {
  uiPrefs.openChatPopOut()
}

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
// Default to chat: when there is no selected alert (the common case), the
// panel is the global Argus AI assistant. When the user clicks an alert we
// switch to diagnostics in the watcher below.
const activeTab = ref('chat')
const sendError = ref(null)

// Collapsible diagnostics sections.
const collapsedBlocks = ref(new Set())
function toggleBlock(key) {
  const s = new Set(collapsedBlocks.value)
  s.has(key) ? s.delete(key) : s.add(key)
  collapsedBlocks.value = s
}
function isBlockOpen(key) { return !collapsedBlocks.value.has(key) }

const { history, sending, autoSummary, eventLog, sendMessage, refreshHistory, fetchAutoSummary, fetchEventLog } = useChat()

// When selected alert changes, fetch auto-summary and chat history.
// When no alert is selected we still load the global chat history so the
// panel reflects the user's previous conversation across navigation.
watch(() => props.selectedAlert?.id, async (alertId) => {
  if (alertId) {
    await fetchAutoSummary(alertId)
    await refreshHistory(alertId)
    activeTab.value = 'diagnostics'
  } else {
    await refreshHistory(GLOBAL_ALERT_ID)
  }
}, { immediate: true })

onMounted(() => {
  if (!props.selectedAlert) {
    refreshHistory(GLOBAL_ALERT_ID)
  }
})

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
  if (!val) return
  question.value = ''
  sendError.value = null
  activeTab.value = 'chat'
  // Prepend any pending Argus context (selected finding, network policy,
  // etc.) so the AI doesn't lose track of what "this" is.
  const ctx = argusContext.consumeForSend()
  const payload = ctx ? `${ctx}\n\nUser question: ${val}` : val
  try {
    await sendMessage(alertIdFor(props.selectedAlert), payload)
  } catch (e) {
    sendError.value = e?.message || String(e)
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
  <div
    class="ai-panel"
    :class="{ 'pro-gate locked': !isAllowed('ai_diagnostics') }"
    :style="{ width: rightPanelWidth + 'px' }"
  >
    <div
      class="resize-handle"
      role="separator"
      aria-orientation="vertical"
      :aria-valuemin="rightPanelMin"
      :aria-valuemax="rightPanelMax"
      :aria-valuenow="rightPanelWidth"
      tabindex="0"
      title="Drag to resize the Argus panel"
      @mousedown="onResizeMouseDown"
      @keydown="onResizeKey"
    ></div>

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
      <div class="ai-panel-sub">{{ selectedAlert?.name || 'Cluster assistant' }}</div>
      <button
        class="popout-btn"
        title="Pop chat out into a window"
        @click="popOut"
      >
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M14 3h7v7"/>
          <path d="M21 3l-9 9"/>
          <path d="M21 14v5a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5"/>
        </svg>
      </button>
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
          <div class="ai-block-label collapsible" @click="toggleBlock('context')">
            <div class="ai-block-dot" style="background: var(--red);"></div>
            Alert Context
            <svg class="section-chevron" :class="{ collapsed: !isBlockOpen('context') }" width="10" height="10" viewBox="0 0 10 10"><path d="M2.5 3.5L5 6.5L7.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" fill="none"/></svg>
          </div>
          <pre v-show="isBlockOpen('context')" class="ai-code">{{ formatContext(selectedAlert) }}</pre>
        </div>

        <div v-if="bundle?.diagnosis" class="ai-insight">
          <div class="ai-insight-title collapsible" @click="toggleBlock('hypothesis')">
            Root Cause Hypothesis
            <svg class="section-chevron" :class="{ collapsed: !isBlockOpen('hypothesis') }" width="10" height="10" viewBox="0 0 10 10"><path d="M2.5 3.5L5 6.5L7.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" fill="none"/></svg>
          </div>
          <div v-show="isBlockOpen('hypothesis')" class="ai-insight-body">{{ bundle.diagnosis.hypothesis }}</div>
        </div>

        <div v-if="bundle?.cascadeAlerts?.length" class="ai-insight cascade-insight">
          <div class="ai-insight-title cascade-title collapsible" @click="toggleBlock('cascade')">
            Cascade Alert
            <svg class="section-chevron" :class="{ collapsed: !isBlockOpen('cascade') }" width="10" height="10" viewBox="0 0 10 10"><path d="M2.5 3.5L5 6.5L7.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" fill="none"/></svg>
          </div>
          <div v-show="isBlockOpen('cascade')" class="ai-insight-body">
            Related alerts detected: {{ bundle.cascadeAlerts.map(a => a.name).join(', ') }}
          </div>
        </div>

        <div v-if="bundle?.anomalyResults?.length" class="ai-context-block">
          <div class="ai-block-label collapsible" @click="toggleBlock('anomaly')">
            <div class="ai-block-dot" style="background: var(--teal);"></div>
            Anomstack Anomaly Detection
            <svg class="section-chevron" :class="{ collapsed: !isBlockOpen('anomaly') }" width="10" height="10" viewBox="0 0 10 10"><path d="M2.5 3.5L5 6.5L7.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" fill="none"/></svg>
          </div>
          <template v-if="isBlockOpen('anomaly')">
            <div v-for="(ar, i) in bundle.anomalyResults" :key="i" class="ai-code">
              {{ ar.metricName }}: {{ ar.isAnomaly ? 'ANOMALY' : 'normal' }} (score: {{ ar.score?.toFixed(2) }}, model: {{ ar.modelUsed }})
            </div>
          </template>
        </div>

        <div v-if="bundle?.diagnosis?.steps?.length">
          <div class="ai-block-label collapsible" @click="toggleBlock('actions')">
            <div class="ai-block-dot" style="background: var(--accent);"></div>
            Recommended Actions
            <svg class="section-chevron" :class="{ collapsed: !isBlockOpen('actions') }" width="10" height="10" viewBox="0 0 10 10"><path d="M2.5 3.5L5 6.5L7.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" fill="none"/></svg>
          </div>
          <div v-show="isBlockOpen('actions')" class="ai-steps">
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
          <div class="ai-block-label collapsible" @click="toggleBlock('decisions')">
            <div class="ai-block-dot" style="background: var(--purple);"></div>
            From DECISION_LOG.md
            <svg class="section-chevron" :class="{ collapsed: !isBlockOpen('decisions') }" width="10" height="10" viewBox="0 0 10 10"><path d="M2.5 3.5L5 6.5L7.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" fill="none"/></svg>
          </div>
          <template v-if="isBlockOpen('decisions')">
            <pre v-for="(entry, i) in bundle.decisionLog" :key="i" class="ai-code">
# {{ entry.date }}
{{ entry.content }}</pre>
          </template>
        </div>
      </template>
    </div>

    <!-- Chat tab -->
    <div v-show="activeTab === 'chat'" class="ai-scroll" ref="chatScrollEl">
      <div v-if="visibleHistory.length === 0" class="empty-state">
        {{ selectedAlert ? 'Ask Argus about this alert' : 'Ask Argus anything about your cluster' }}
      </div>

      <div v-for="(msg, i) in visibleHistory" :key="i" class="chat-msg" :class="'chat-' + msg.role">
        <div class="chat-role">{{ msg.role === 'assistant' ? 'Argus' : 'You' }}</div>
        <div class="chat-content">
          <template v-for="(seg, segIdx) in parseCodeBlocks(msg.content)" :key="segIdx">
            <div
              v-if="seg.type === 'text'"
              class="chat-text markdown-body"
              v-html="renderMarkdown(seg.text)"
            ></div>
            <CodeBlock
              v-else
              :code="seg.text"
              :language="seg.language"
              :allow-run="msg.role === 'assistant'"
            />
          </template>
        </div>
        <div class="chat-time">{{ formatTime(msg.timestamp) }}</div>
      </div>

      <div v-if="sending" class="chat-typing">
        <div class="typing-dots"><span></span><span></span><span></span></div>
        Argus is thinking...
      </div>

      <div v-if="sendError" class="chat-error">
        <span class="chat-error-icon">!</span>
        <span class="chat-error-text">{{ sendError }}</span>
        <button class="chat-error-close" @click="sendError = null">×</button>
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
    <div v-if="pendingContext" class="ai-context-chip">
      <span class="ai-context-kind">{{ pendingContext.kind }}</span>
      <span class="ai-context-label" :title="pendingContext.label">{{ pendingContext.label }}</span>
      <button class="ai-context-close" @click="argusContext.clearContext()" title="Detach this context">×</button>
    </div>

    <div class="ai-input-area">
      <textarea
        class="ai-input"
        v-model="question"
        :placeholder="selectedAlert ? 'Ask about this alert...' : 'Ask Argus about your cluster...'"
        rows="1"
        @keydown="onKeydown"
      ></textarea>
      <button class="ai-send" @click="onSend" :disabled="sending || !question.trim()">
        <svg width="13" height="13" viewBox="0 0 13 13" fill="none">
          <path d="M2 6.5h9M7.5 3L11 6.5L7.5 10" stroke="white" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.ai-panel {
  /* width is bound inline via :style and persisted in the uiPrefs store */
  border-left: 1px solid var(--border); background: var(--bg2);
  display: flex; flex-direction: column; flex-shrink: 0;
  position: relative;
  min-width: 280px;
  max-width: 720px;
}

/* Resize handle — straddles the left edge so a 6px hit target is comfortable
   without visually intruding. The handle highlights on hover so the user
   discovers it. */
.resize-handle {
  position: absolute;
  left: -3px;
  top: 0;
  bottom: 0;
  width: 6px;
  cursor: col-resize;
  z-index: 5;
  background: transparent;
  transition: background 0.15s;
}
.resize-handle:hover,
.resize-handle:focus {
  background: rgba(79, 142, 247, 0.5);
  outline: none;
}

.popout-btn {
  background: none; border: 1px solid transparent;
  color: var(--text3);
  padding: 3px 5px; border-radius: 4px;
  display: inline-flex; align-items: center;
  cursor: pointer;
  transition: all 0.15s;
}
.popout-btn:hover { color: var(--accent2); border-color: var(--border2); background: var(--bg3); }

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
.ai-block-label.collapsible, .ai-insight-title.collapsible { cursor: pointer; user-select: none; }
.ai-block-label.collapsible:hover, .ai-insight-title.collapsible:hover { color: var(--text2); }
.section-chevron { margin-left: auto; flex-shrink: 0; transition: transform 0.15s ease; }
.section-chevron.collapsed { transform: rotate(-90deg); }
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
.chat-content { color: var(--text2); word-break: break-word; }
.chat-text { /* markdown-body styles below */ }

/* Rendered markdown — keep type compact in the narrow right rail. */
.markdown-body :deep(p) { margin: 0 0 6px 0; }
.markdown-body :deep(p:last-child) { margin-bottom: 0; }
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4) {
  font-weight: 600; color: var(--text); margin: 8px 0 3px 0; line-height: 1.25;
}
.markdown-body :deep(h1) { font-size: 14px; }
.markdown-body :deep(h2) { font-size: 13.5px; }
.markdown-body :deep(h3) { font-size: 13px; }
.markdown-body :deep(h4) { font-size: 12.5px; }
.markdown-body :deep(ul),
.markdown-body :deep(ol) { padding-left: 16px; margin: 3px 0; }
.markdown-body :deep(li) { margin: 1px 0; }
.markdown-body :deep(strong) { font-weight: 600; color: var(--text); }
.markdown-body :deep(em) { font-style: italic; }
.markdown-body :deep(blockquote) {
  border-left: 2px solid var(--accent);
  padding: 2px 8px; margin: 4px 0;
  color: var(--text3); background: rgba(79,142,247,0.06);
}
.markdown-body :deep(code) {
  background: rgba(255,255,255,0.07);
  padding: 1px 4px; border-radius: 3px;
  font-family: var(--mono); font-size: 11px;
  color: #c9d1d9;
}
.markdown-body :deep(a) { color: var(--accent2); text-decoration: underline; }
.markdown-body :deep(hr) {
  border: none; border-top: 1px solid rgba(255,255,255,0.08); margin: 6px 0;
}
.markdown-body :deep(table) { border-collapse: collapse; margin: 4px 0; font-size: 11px; }
.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid rgba(255,255,255,0.1); padding: 3px 6px; text-align: left;
}
.markdown-body :deep(th) { background: rgba(255,255,255,0.04); font-weight: 600; }

.chat-error {
  display: flex; align-items: center; gap: 6px;
  margin-top: 8px;
  padding: 6px 8px;
  background: rgba(240, 84, 84, 0.1);
  border: 1px solid rgba(240, 84, 84, 0.25);
  border-radius: var(--r2);
  font-size: 11.5px;
  color: #f7c1c1;
}
.chat-error-icon {
  width: 14px; height: 14px;
  display: inline-flex; align-items: center; justify-content: center;
  background: rgba(240, 84, 84, 0.4);
  border-radius: 50%;
  color: #fff;
  font-weight: 700;
  font-size: 10px;
  flex-shrink: 0;
}
.chat-error-text { flex: 1; word-break: break-word; }
.chat-error-close {
  background: none; border: none; color: var(--text2); font-size: 14px;
  cursor: pointer; line-height: 1;
}
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

.ai-context-chip {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 10px;
  background: rgba(167, 139, 250, 0.08);
  border-top: 1px solid rgba(167, 139, 250, 0.25);
  font-size: 11px; color: #c4b3fd;
}
.ai-context-kind {
  text-transform: uppercase; letter-spacing: 0.05em; font-weight: 600;
  font-size: 9.5px; padding: 1px 5px;
  background: rgba(167, 139, 250, 0.18); border-radius: 3px;
}
.ai-context-label {
  flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  color: var(--text);
}
.ai-context-close {
  background: none; border: none; color: var(--text2); cursor: pointer;
  font-size: 13px; line-height: 1; padding: 0 4px; border-radius: 3px;
}
.ai-context-close:hover { background: rgba(255, 255, 255, 0.06); color: #fff; }

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
