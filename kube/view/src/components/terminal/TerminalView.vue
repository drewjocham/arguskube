<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick, reactive } from 'vue'
import { useTerminal, useTerminalSession, useTerminalCopilot, usePods, useContexts } from '../../composables/useWails'
import { bus } from '../../lib/bus'
import { useTerminalDispatch } from '../../composables/useTerminalDispatch'
import { useOutputCaptureStore } from '../../stores/outputCapture'

const props = defineProps({
  visible: { type: Boolean, default: false },
})

const termRefs = ref({})
const activeSessionId = ref('default')
const { startTerminal, sendInput, resizeTerminal } = useTerminal()
const {
  sessions, domains, createSession, sendSessionInput,
  resizeSession, closeSession, refreshSessions,
  domainLabel, domainIcon,
} = useTerminalSession()

const tabs = reactive([
  { sessionId: 'default', domain: 'default', label: 'Shell', initError: null, started: false, term: null, fitAddon: null },
])

let busOff = null

function getTab(sessionId) {
  return tabs.find(t => t.sessionId === sessionId)
}

async function addTab(domain) {
  const existing = tabs.find(t => t.domain === domain)
  if (existing) { activeSessionId.value = existing.sessionId; return }
  const sessionId = domain
  tabs.push({ sessionId, domain, label: domainLabel(domain), initError: null, started: false, term: null, fitAddon: null })
  activeSessionId.value = sessionId
  await nextTick()
  await initSessionTab(sessionId)
}

async function closeTab(sessionId) {
  if (tabs.length <= 1) return
  const idx = tabs.findIndex(t => t.sessionId === sessionId)
  const tab = getTab(sessionId)
  if (tab) { tab.term?.dispose(); await closeSession(sessionId) }
  tabs.splice(idx, 1)
  if (activeSessionId.value === sessionId) {
    activeSessionId.value = tabs[Math.min(idx, tabs.length - 1)].sessionId
  }
}

async function initSessionTab(sessionId) {
  const tab = getTab(sessionId)
  if (!tab || tab.started) return
  tab.initError = null

  const el = termRefs.value[sessionId]
  if (!el) return

  const { Terminal } = await import('xterm')
  const { FitAddon } = await import('xterm-addon-fit')

  const term = new Terminal({
    fontFamily: "'Cascadia Mono', 'Cascadia Code', 'SF Mono', Consolas, monospace",
    fontSize: 12, lineHeight: 1.35, cursorBlink: true, cursorStyle: 'bar',
    theme: {
      background: '#1a1c1e', foreground: '#e8eaec', cursor: '#4f8ef7',
      cursorAccent: '#1a1c1e', selectionBackground: 'rgba(79,142,247,0.25)',
      black: '#1a1c1e', red: '#f05454', green: '#3ecf8e', yellow: '#f5a623',
      blue: '#4f8ef7', magenta: '#a78bfa', cyan: '#2dd4bf', white: '#e8eaec',
      brightBlack: '#5c6168', brightRed: '#ff7575', brightGreen: '#5edba6',
      brightYellow: '#ffc04d', brightBlue: '#6ba3f9', brightMagenta: '#c4b3fd',
      brightCyan: '#5ee8d4', brightWhite: '#ffffff',
    },
  })

  const fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(el)
  await nextTick()
  fitAddon.fit()

  term.onData((data) => { sendSessionInput(sessionId, data) })

  try {
    await createSession(sessionId, tab.domain, tab.label, term.rows, term.cols)
    tab.started = true
    tab.term = term
    tab.fitAddon = fitAddon
  } catch (e) {
    tab.initError = e?.message || String(e)
    tab.started = false
    term.dispose()
    tab.term = null; tab.fitAddon = null
  }
}

async function retryInit(sessionId) {
  const tab = getTab(sessionId)
  if (!tab) return
  tab.initError = null
  if (props.visible) { await nextTick(); await initSessionTab(sessionId); tab.term?.focus(); flushPendingCommand() }
}

const captureStore = useOutputCaptureStore()
const recentOutput = ref('')

function handleTerminalOutput(payload) {
  if (!payload) return
  let data
  if (typeof payload === 'string') { data = payload; const tab = getTab('default'); if (tab?.term) tab.term.write(data) }
  else { data = payload.data; const tab = getTab(payload.sessionId || 'default'); if (tab?.term) tab.term.write(data) }
  captureStore.appendOutput(data)
  if (data) { recentOutput.value = (recentOutput.value + data).slice(-4096) }
}

function handleResize() {
  for (const tab of tabs) {
    if (tab.fitAddon && tab.term && props.visible && tab.sessionId === activeSessionId.value) {
      tab.fitAddon.fit(); resizeSession(tab.sessionId, tab.term.rows, tab.term.cols)
    }
  }
}

watch(() => props.visible, async (visible) => {
  if (visible) {
    await nextTick()
    for (const tab of tabs) { if (!tab.term) await initSessionTab(tab.sessionId); else tab.fitAddon?.fit() }
    getTab(activeSessionId.value)?.term?.focus()
    flushPendingCommand()
  }
})

watch(activeSessionId, async () => {
  await nextTick()
  const tab = getTab(activeSessionId.value)
  if (tab) {
    if (!tab.term) await initSessionTab(tab.sessionId); else tab.fitAddon?.fit()
    tab.term?.focus()
    flushPendingCommand()
  }
})

const { pendingCommand, consumePendingCommand, peekPendingCommand } = useTerminalDispatch()
let lastSessionId = null

function flushPendingCommand() {
  const tab = getTab(activeSessionId.value)
  if (!tab?.term || !tab.started || !props.visible) return
  if (!peekPendingCommand()) return
  const queued = consumePendingCommand()
  if (!queued) return
  if (queued.sessionId && queued.sessionId !== lastSessionId) {
    sendSessionInput(activeSessionId.value, `# session: ${queued.sectionLabel || queued.sessionId}\n`)
    lastSessionId = queued.sessionId
  } else if (!queued.sessionId) { lastSessionId = null }
  sendSessionInput(activeSessionId.value, queued.text)
  tab.term.focus()
}

watch(pendingCommand, (val) => { if (val) flushPendingCommand() })

// Copilot
const copilot = useTerminalCopilot()
const copilotInput = ref('')
const copilotOpen = ref(false)
const copilotMode = ref('ask')

async function handleCopilotSubmit() {
  const input = copilotInput.value.trim()
  if (!input) return
  const tab = getTab(activeSessionId.value)
  const domain = tab?.domain || 'default'
  copilotOpen.value = true; copilotMode.value = 'ask'
  await copilot.generateCommand(input, domain)
  copilotInput.value = ''
}

async function handleExplain() {
  const tab = getTab(activeSessionId.value)
  const domain = tab?.domain || 'default'
  copilotOpen.value = true; copilotMode.value = 'explain'
  await copilot.explainOutput(recentOutput.value, domain)
}

function copilotResultText() {
  if (copilot.loading.value) return 'Thinking...'
  if (copilot.error.value) return copilot.error.value
  if (copilot.result.value) return copilot.result.value
  return ''
}

function closeCopilot() { copilotOpen.value = false; copilot.clear() }

// Pod exec (K8s tab)
const podPickerOpen = ref(false)
const { pods, loading: podsLoading, fetchPods } = usePods()

async function togglePodPicker() {
  podPickerOpen.value = !podPickerOpen.value
  if (podPickerOpen.value) await fetchPods()
}

function execIntoPod(podName, namespace) {
  const tab = getTab(activeSessionId.value)
  if (!tab?.term) return
  podPickerOpen.value = false
  sendSessionInput(activeSessionId.value, `kubectl exec -it -n ${namespace} ${podName} -- sh\n`)
}

// Context switcher (K8s tab)
const { contexts, fetchContexts } = useContexts()
const currentContext = ref('')
const ctxSwitcherOpen = ref(false)

async function refreshContext() {
  await fetchContexts()
  const active = contexts.value?.find(c => c.active)
  currentContext.value = active?.name || ''
}

async function toggleCtxSwitcher() {
  ctxSwitcherOpen.value = !ctxSwitcherOpen.value
  if (ctxSwitcherOpen.value) await refreshContext()
}

async function switchCtx(name) {
  ctxSwitcherOpen.value = false
  const tab = getTab(activeSessionId.value)
  if (!tab?.term) return
  sendSessionInput(activeSessionId.value, `kubectl config use-context ${name}\n`)
  currentContext.value = name
}

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  busOff = bus.useWailsEvent('terminal:output', handleTerminalOutput)
  if (props.visible) {
    await nextTick()
    const tab = getTab(activeSessionId.value)
    if (tab && !tab.term) await initSessionTab('default')
    getTab(activeSessionId.value)?.term?.focus()
    flushPendingCommand()
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  busOff?.()
  for (const tab of tabs) { tab.term?.dispose(); tab.term = null }
})
</script>

<template>
  <div class="terminal-container" v-show="visible">
    <div class="terminal-tabs">
      <button v-for="tab in tabs" :key="tab.sessionId" class="tab"
        :class="{ active: tab.sessionId === activeSessionId }"
        @click="activeSessionId = tab.sessionId">
        <span class="tab-icon">{{ domainIcon(tab.domain) }}</span>
        <span class="tab-label">{{ tab.label }}</span>
        <span v-if="tabs.length > 1" class="tab-close" @click.stop="closeTab(tab.sessionId)">&times;</span>
      </button>
      <div class="tab-add-wrapper">
        <button class="tab-add" title="New session">+</button>
        <div class="tab-add-menu">
          <button v-for="d in domains" :key="d.id" @click="addTab(d.id)"
            :disabled="tabs.some(t => t.domain === d.id)">
            {{ domainIcon(d.id) }} {{ d.label }}
          </button>
        </div>
      </div>
    </div>

    <!-- Pod picker -->
    <div v-if="podPickerOpen" class="overlay-panel" @click.stop>
      <div class="overlay-header"><span>Select a pod to exec into:</span><button class="overlay-close" @click="podPickerOpen = false">&times;</button></div>
      <div class="overlay-list">
        <div v-if="podsLoading" class="overlay-empty">Loading pods...</div>
        <div v-else-if="!pods?.length" class="overlay-empty">No pods found</div>
        <button v-for="pod in pods" :key="pod.metadata?.uid || pod.metadata?.name" class="overlay-item" @click="execIntoPod(pod.metadata?.name, pod.metadata?.namespace)">
          <span class="overlay-item-name">{{ pod.metadata?.name }}</span>
          <span class="overlay-item-ns">{{ pod.metadata?.namespace }}</span>
          <span class="overlay-item-status" :class="pod.status?.phase?.toLowerCase()">{{ pod.status?.phase }}</span>
        </button>
      </div>
    </div>

    <!-- Context switcher -->
    <div v-if="ctxSwitcherOpen" class="overlay-panel" @click.stop>
      <div class="overlay-header"><span>Switch Kubernetes context:</span><button class="overlay-close" @click="ctxSwitcherOpen = false">&times;</button></div>
      <div class="overlay-list">
        <div v-if="!contexts?.length" class="overlay-empty">No contexts found</div>
        <button v-for="ctx in contexts" :key="ctx.name" class="overlay-item" :class="{ 'ctx-active': ctx.active }" @click="switchCtx(ctx.name)">
          <span class="overlay-item-name">{{ ctx.name }}</span>
          <span class="overlay-item-status" v-if="ctx.active">active</span>
        </button>
      </div>
    </div>

    <!-- Sessions -->
    <div v-for="tab in tabs" :key="tab.sessionId" v-show="tab.sessionId === activeSessionId" class="terminal-session">
      <div v-if="tab.initError" class="terminal-error">
        <div class="terminal-error-icon">!</div>
        <div class="terminal-error-body">
          <div class="terminal-error-title">Terminal session failed to start</div>
          <div class="terminal-error-msg">{{ tab.initError }}</div>
          <button class="terminal-error-retry" @click="retryInit(tab.sessionId)">Retry</button>
        </div>
      </div>
      <div :ref="el => { if (el) termRefs[tab.sessionId] = el }" class="terminal-element" v-show="!tab.initError"></div>
    </div>

    <!-- Copilot response -->
    <div v-if="copilotOpen" class="copilot-response" @click="closeCopilot">
      <div class="copilot-response-header">
        <span class="copilot-response-label">{{ copilotMode === 'explain' ? 'Analysis' : 'Command' }}</span>
        <button class="overlay-close" @click.stop="closeCopilot">&times;</button>
      </div>
      <pre class="copilot-response-text">{{ copilotResultText() }}</pre>
    </div>

    <!-- Toolbar -->
    <div class="copilot-bar">
      <button v-if="getTab(activeSessionId)?.domain === 'k8s'" class="tool-btn k8s-btn" @click="togglePodPicker">Pod Exec</button>
      <button v-if="getTab(activeSessionId)?.domain === 'k8s'" class="tool-btn ctx-btn" @click="toggleCtxSwitcher">{{ currentContext || 'Context' }}</button>
      <button class="tool-btn explain-btn" @click="handleExplain">Explain</button>
      <input v-model="copilotInput" class="copilot-input" placeholder="Ask AI to generate a command..." @keydown.enter="handleCopilotSubmit" @focus="copilotOpen = true" />
      <button class="tool-btn" @click="handleCopilotSubmit" :disabled="!copilotInput.trim()">Go</button>
    </div>
  </div>
</template>

<style scoped>
.terminal-container { width: 100%; height: 100%; background: var(--bg); overflow: hidden; position: relative; display: flex; flex-direction: column; }
.terminal-tabs { display: flex; align-items: center; gap: 0; background: #151719; border-bottom: 1px solid rgba(255,255,255,0.06); flex-shrink: 0; padding: 0 4px; min-height: 32px; }
.tab { display: flex; align-items: center; gap: 5px; padding: 6px 10px; font-size: 11.5px; color: #8b8f96; background: none; border: none; border-bottom: 2px solid transparent; cursor: pointer; white-space: nowrap; font-family: inherit; }
.tab:hover { color: #e8eaec; background: rgba(255,255,255,0.04); }
.tab.active { color: #e8eaec; border-bottom-color: #4f8ef7; }
.tab-icon { font-size: 13px; }
.tab-close { margin-left: 4px; font-size: 14px; line-height: 1; color: #5c6168; padding: 0 2px; }
.tab-close:hover { color: #f05454; }
.tab-add-wrapper { position: relative; margin-left: auto; }
.tab-add { background: none; border: none; color: #5c6168; font-size: 16px; padding: 4px 8px; cursor: pointer; font-family: inherit; }
.tab-add:hover { color: #e8eaec; }
.tab-add-wrapper:hover .tab-add-menu { display: flex; }
.tab-add-menu { display: none; position: absolute; top: 100%; right: 0; flex-direction: column; background: #1c1f22; border: 1px solid rgba(255,255,255,0.1); border-radius: 6px; padding: 4px; z-index: 50; min-width: 120px; }
.tab-add-menu button { background: none; border: none; color: #e8eaec; padding: 6px 10px; font-size: 12px; text-align: left; cursor: pointer; border-radius: 4px; font-family: inherit; }
.tab-add-menu button:hover { background: rgba(79,142,247,0.15); }
.tab-add-menu button:disabled { opacity: 0.35; cursor: default; }

.terminal-session { flex: 1; overflow: hidden; position: relative; }
.terminal-error { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; gap: 14px; padding: 24px; background: rgba(240,84,84,0.04); z-index: 5; }
.terminal-error-icon { flex-shrink: 0; width: 32px; height: 32px; display: inline-flex; align-items: center; justify-content: center; border-radius: 50%; background: rgba(240,84,84,0.18); color: #f05454; font-weight: 700; font-size: 16px; }
.terminal-error-body { display: flex; flex-direction: column; gap: 4px; max-width: 520px; }
.terminal-error-title { font-size: 13px; font-weight: 600; color: var(--text); }
.terminal-error-msg { font-size: 12px; color: var(--text2); font-family: var(--mono); word-break: break-word; }
.terminal-error-retry { align-self: flex-start; margin-top: 6px; background: rgba(79,142,247,0.15); border: 1px solid rgba(79,142,247,0.3); color: var(--accent2); padding: 5px 14px; border-radius: 4px; font-size: 12px; cursor: pointer; }
.terminal-error-retry:hover { background: rgba(79,142,247,0.25); color: #fff; }
.terminal-element { width: 100%; height: 100%; padding: 4px 8px; }

.overlay-panel { position: absolute; bottom: 100%; left: 8px; right: 8px; max-height: 220px; overflow-y: auto; background: #1c1f22; border: 1px solid rgba(255,255,255,0.1); border-radius: 6px; z-index: 10; }
.overlay-header { display: flex; align-items: center; justify-content: space-between; padding: 6px 8px; font-size: 11px; color: #8b8f96; border-bottom: 1px solid rgba(255,255,255,0.06); }
.overlay-close { background: none; border: none; color: #5c6168; font-size: 16px; cursor: pointer; line-height: 1; padding: 0 2px; }
.overlay-close:hover { color: #f05454; }
.overlay-empty { padding: 12px; text-align: center; color: #5c6168; font-size: 12px; }
.overlay-list { padding: 4px; }
.overlay-item { display: flex; align-items: center; gap: 6px; width: 100%; padding: 5px 6px; background: none; border: none; border-radius: 4px; cursor: pointer; text-align: left; font-family: inherit; font-size: 11.5px; }
.overlay-item:hover { background: rgba(79,142,247,0.12); }
.overlay-item-name { color: #e8eaec; flex: 1; overflow: hidden; text-overflow: ellipsis; }
.overlay-item-ns { color: #5c6168; font-size: 10px; }
.overlay-item-status { font-size: 10px; padding: 1px 5px; border-radius: 3px; background: rgba(255,255,255,0.06); color: #8b8f96; }
.overlay-item-status.running { background: rgba(62,207,142,0.12); color: #3ecf8e; }
.overlay-item-status.pending { background: rgba(245,166,35,0.12); color: #f5a623; }
.overlay-item-status.failed { background: rgba(240,84,84,0.12); color: #f05454; }
.ctx-active { background: rgba(79,142,247,0.08); }
.ctx-active .overlay-item-name { color: #4f8ef7; }

.copilot-response { position: absolute; bottom: 100%; left: 8px; right: 8px; max-height: 200px; overflow-y: auto; background: #1c1f22; border: 1px solid rgba(255,255,255,0.1); border-radius: 6px; padding: 8px 10px; z-index: 10; cursor: pointer; }
.copilot-response-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px; }
.copilot-response-label { font-size: 10px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: #4f8ef7; }
.copilot-response-text { font-size: 12px; color: #e8eaec; font-family: var(--mono); white-space: pre-wrap; word-break: break-word; margin: 0; line-height: 1.4; }

.copilot-bar { display: flex; align-items: center; gap: 6px; padding: 6px 8px; background: #151719; border-top: 1px solid rgba(255,255,255,0.06); flex-shrink: 0; }
.tool-btn { display: inline-flex; align-items: center; gap: 4px; background: rgba(79,142,247,0.12); border: 1px solid rgba(79,142,247,0.2); color: #8b8f96; padding: 4px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; white-space: nowrap; font-family: inherit; }
.tool-btn:hover { background: rgba(79,142,247,0.2); color: #e8eaec; }
.tool-btn:disabled { opacity: 0.35; cursor: default; }
.explain-btn { background: rgba(240,84,84,0.1); border-color: rgba(240,84,84,0.2); }
.explain-btn:hover { background: rgba(240,84,84,0.2); color: #f05454; }
.k8s-btn { background: rgba(79,142,247,0.1); border-color: rgba(79,142,247,0.2); }
.ctx-btn { background: rgba(245,166,35,0.1); border-color: rgba(245,166,35,0.2); }
.ctx-btn:hover { background: rgba(245,166,35,0.2); color: #f5a623; }
.copilot-input { flex: 1; background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.08); border-radius: 4px; color: #e8eaec; padding: 5px 8px; font-size: 12px; font-family: var(--mono); outline: none; }
.copilot-input:focus { border-color: rgba(79,142,247,0.4); }
.copilot-input::placeholder { color: #5c6168; }

:deep(.xterm) { padding: 0; }
:deep(.xterm-viewport) { overflow-y: auto !important; }
:deep(.xterm-viewport::-webkit-scrollbar) { width: 5px; }
:deep(.xterm-viewport::-webkit-scrollbar-thumb) { background: rgba(255,255,255,0.1); border-radius: 3px; }
</style>
