<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useTerminal } from '../../composables/useWails'
import { useWailsEvent } from '../../composables/useEvents'
import { useTerminalDispatch } from '../../composables/useTerminalDispatch'
import { useOutputCaptureStore } from '../../stores/outputCapture'

const props = defineProps({
  visible: { type: Boolean, default: false },
})

const termRef = ref(null)
const { startTerminal, sendInput, resizeTerminal } = useTerminal()

let term = null
let fitAddon = null
let started = false

// Visible error surface — when StartTerminal on the backend fails (PTY can't
// allocate, shell binary missing, etc.), we now show what went wrong inside
// the panel instead of leaving the user with a silent black box.
const initError = ref(null)

async function initTerminal() {
  if (term || !termRef.value) return
  initError.value = null

  const { Terminal } = await import('xterm')
  const { FitAddon } = await import('xterm-addon-fit')

  term = new Terminal({
    fontFamily: "'Cascadia Mono', 'Cascadia Code', 'SF Mono', Consolas, monospace",
    fontSize: 12,
    lineHeight: 1.35,
    cursorBlink: true,
    cursorStyle: 'bar',
    theme: {
      background: '#1a1c1e',
      foreground: '#e8eaec',
      cursor: '#4f8ef7',
      cursorAccent: '#1a1c1e',
      selectionBackground: 'rgba(79,142,247,0.25)',
      black: '#1a1c1e',
      red: '#f05454',
      green: '#3ecf8e',
      yellow: '#f5a623',
      blue: '#4f8ef7',
      magenta: '#a78bfa',
      cyan: '#2dd4bf',
      white: '#e8eaec',
      brightBlack: '#5c6168',
      brightRed: '#ff7575',
      brightGreen: '#5edba6',
      brightYellow: '#ffc04d',
      brightBlue: '#6ba3f9',
      brightMagenta: '#c4b3fd',
      brightCyan: '#5ee8d4',
      brightWhite: '#ffffff',
    },
  })

  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(termRef.value)

  await nextTick()
  fitAddon.fit()

  term.onData((data) => {
    sendInput(data)
  })

  if (!started) {
    started = true
    try {
      await startTerminal(term.rows, term.cols)
    } catch (e) {
      // Surface the failure so the user can see what's broken instead of
      // staring at an empty box. Reset `started` so the Retry button can
      // re-attempt cleanly.
      initError.value = e?.message || String(e)
      started = false
      term?.dispose()
      term = null
      fitAddon = null
    }
  }
}

async function retryInit() {
  initError.value = null
  if (props.visible) {
    await nextTick()
    await initTerminal()
    term?.focus()
    flushPendingCommand()
  }
}

const captureStore = useOutputCaptureStore()
useWailsEvent('terminal:output', (data) => {
  if (term && data) {
    term.write(data)
  }
  // Forward to whichever CodeBlock is currently capturing terminal output.
  // The store is a no-op when no block has called startCapture().
  captureStore.appendOutput(data)
})

// Handle resize.
const resizeObserver = ref(null)

function handleResize() {
  if (fitAddon && term && props.visible) {
    fitAddon.fit()
    resizeTerminal(term.rows, term.cols)
  }
}

watch(() => props.visible, async (visible) => {
  if (visible) {
    await nextTick()
    if (!term) {
      await initTerminal()
    } else {
      fitAddon?.fit()
    }
    term?.focus()
    flushPendingCommand()
  }
})

const { pendingCommand, consumePendingCommand, peekPendingCommand } = useTerminalDispatch()

// Flush any command queued by another view (Argus AI chat) into the terminal
// session. We deliberately do NOT append a trailing newline — the user must
// press Enter to execute, which is the safety boundary against auto-running
// LLM-suggested shell commands.
//
// Important: only consume the queued command AFTER verifying the xterm
// session is fully ready. Earlier versions consumed-then-checked, dropping
// commands queued before the panel finished initializing.
// `lastSessionId` tracks which logical session the most-recent dispatch
// belonged to. When a new dispatch arrives from a DIFFERENT session, we
// prepend a one-line `# session: <label>` comment so the user can see
// which runbook section a command came from. This is a stopgap until the
// multi-PTY backend lands and sessionId actually routes to a separate PTY.
let lastSessionId = null

function flushPendingCommand() {
  if (!term || !started || !props.visible) return
  if (!peekPendingCommand()) return
  const queued = consumePendingCommand()
  if (!queued) return

  if (queued.sessionId && queued.sessionId !== lastSessionId) {
    const label = queued.sectionLabel || queued.sessionId
    // The shell ignores comment lines starting with `#`, so this is safe
    // to write even though it'd execute as a no-op if the user pressed
    // Enter. We DON'T include a newline — the user still has to press
    // Enter to send anything to the shell.
    sendInput(`# session: ${label}\n`)
    lastSessionId = queued.sessionId
  } else if (!queued.sessionId) {
    lastSessionId = null
  }

  sendInput(queued.text)
  term.focus()
}

watch(pendingCommand, (val) => {
  if (val) {
    flushPendingCommand()
  }
})

onMounted(async () => {
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver.value = new ResizeObserver(() => {
      handleResize()
    })
    if (termRef.value) {
      resizeObserver.value.observe(termRef.value)
    }
  }

  window.addEventListener('resize', handleResize)

  // The parent gates this component behind v-if, so it always mounts with
  // visible=true. Vue's watch on props.visible doesn't fire on the initial
  // value, so we have to kick off init here. Without this, the xterm UI
  // appears but no PTY ever starts and the terminal stays blank.
  if (props.visible) {
    await nextTick()
    if (!term) {
      await initTerminal()
    }
    term?.focus()
    flushPendingCommand()
  }
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  resizeObserver.value?.disconnect()
  term?.dispose()
  term = null
})
</script>

<template>
  <div class="terminal-container" v-show="visible">
    <div v-if="initError" class="terminal-error">
      <div class="terminal-error-icon">!</div>
      <div class="terminal-error-body">
        <div class="terminal-error-title">Terminal session failed to start</div>
        <div class="terminal-error-msg">{{ initError }}</div>
        <button class="terminal-error-retry" @click="retryInit">Retry</button>
      </div>
    </div>
    <div ref="termRef" class="terminal-element" v-show="!initError"></div>
  </div>
</template>

<style scoped>
.terminal-container {
  width: 100%;
  height: 100%;
  background: var(--bg);
  overflow: hidden;
  position: relative;
}

.terminal-error {
  position: absolute; inset: 0;
  display: flex; align-items: center; justify-content: center; gap: 14px;
  padding: 24px;
  background: rgba(240, 84, 84, 0.04);
  z-index: 5;
}
.terminal-error-icon {
  flex-shrink: 0;
  width: 32px; height: 32px;
  display: inline-flex; align-items: center; justify-content: center;
  border-radius: 50%;
  background: rgba(240, 84, 84, 0.18);
  color: #f05454;
  font-weight: 700; font-size: 16px;
}
.terminal-error-body { display: flex; flex-direction: column; gap: 4px; max-width: 520px; }
.terminal-error-title { font-size: 13px; font-weight: 600; color: var(--text); }
.terminal-error-msg { font-size: 12px; color: var(--text2); font-family: var(--mono); word-break: break-word; }
.terminal-error-retry {
  align-self: flex-start;
  margin-top: 6px;
  background: rgba(79, 142, 247, 0.15);
  border: 1px solid rgba(79, 142, 247, 0.3);
  color: var(--accent2);
  padding: 5px 14px; border-radius: 4px;
  font-size: 12px; cursor: pointer;
}
.terminal-error-retry:hover { background: rgba(79, 142, 247, 0.25); color: #fff; }

.terminal-element {
  width: 100%;
  height: 100%;
  padding: 4px 8px;
}

:deep(.xterm) {
  padding: 0;
}

:deep(.xterm-viewport) {
  overflow-y: auto !important;
}

:deep(.xterm-viewport::-webkit-scrollbar) {
  width: 5px;
}

:deep(.xterm-viewport::-webkit-scrollbar-thumb) {
  background: rgba(255,255,255,0.1);
  border-radius: 3px;
}
</style>
