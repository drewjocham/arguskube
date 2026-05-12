<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { useTerminal } from '../../composables/useWails'
import { bus } from '../../lib/bus'

const props = defineProps({
  standalone: { type: Boolean, default: false },
})

const emit = defineEmits(['close'])

const termRef = ref(null)
const { startTerminal, sendInput, resizeTerminal } = useTerminal()

let term = null
let fitAddon = null
let started = false
let resizeObs = null

async function initTerminal() {
  if (term || !termRef.value) return

  const { Terminal } = await import('xterm')
  const { FitAddon } = await import('xterm-addon-fit')

  term = new Terminal({
    fontFamily: "var(--mono), 'Cascadia Mono', 'SF Mono', Consolas, monospace",
    fontSize: 13,
    lineHeight: 1.35,
    cursorBlink: true,
    cursorStyle: 'bar',
    theme: {
      background: '#0d0d0d',
      foreground: '#e8eaec',
      cursor: '#4f8ef7',
      cursorAccent: '#0d0d0d',
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
    await startTerminal(term.rows, term.cols)
  }

  term.focus()

  // Observe container resize for fit.
  if (typeof ResizeObserver !== 'undefined') {
    resizeObs = new ResizeObserver(() => {
      if (fitAddon && term) {
        fitAddon.fit()
        resizeTerminal(term.rows, term.cols)
      }
    })
    resizeObs.observe(termRef.value)
  }
}

bus.useWailsEvent('terminal:output', (data) => {
  if (term && data) {
    term.write(data)
  }
})

function handleClose() {
  emit('close')
}

// Keyboard shortcut: Escape to close (only in overlay mode).
function onKeydown(e) {
  if (e.key === 'Escape' && !props.standalone) {
    handleClose()
  }
}

onMounted(async () => {
  window.addEventListener('keydown', onKeydown)
  await nextTick()
  await initTerminal()
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  resizeObs?.disconnect()
  term?.dispose()
  term = null
})
</script>

<template>
  <div class="pro-desktop-overlay" @mousedown.self="handleClose">
    <div class="pro-desktop-window">
      <!-- Titlebar -->
      <div class="pro-window-titlebar" style="--wails-draggable: drag">
        <div class="traffic-lights">
          <div class="tl tl-r" @click="handleClose"></div>
          <div class="tl tl-y"></div>
          <div class="tl tl-g"></div>
        </div>
        <div class="window-title">
          <span>Argus</span> — Terminal
        </div>
        <div class="window-right">
          <button class="close-btn" @click="handleClose" title="Close (Esc)" style="--wails-draggable: no-drag">
            <svg width="12" height="12" viewBox="0 0 12 12">
              <path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            </svg>
          </button>
        </div>
      </div>

      <!-- Terminal area -->
      <div class="pro-terminal-area">
        <div ref="termRef" class="pro-terminal-element"></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pro-desktop-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.55);
  backdrop-filter: blur(6px);
  z-index: 10000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
}

.pro-desktop-window {
  width: 100%;
  max-width: 960px;
  height: 100%;
  max-height: 640px;
  background: #0d0d0d;
  border-radius: 10px;
  border: 1px solid rgba(255,255,255,0.1);
  box-shadow: 0 24px 60px rgba(0,0,0,0.5), 0 0 0 1px rgba(255,255,255,0.05) inset;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: pop-in 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes pop-in {
  from { opacity: 0; transform: scale(0.96) translateY(8px); }
  to { opacity: 1; transform: scale(1) translateY(0); }
}

.pro-window-titlebar {
  height: 40px;
  background: #1a1c1f;
  border-bottom: 1px solid rgba(255,255,255,0.06);
  display: flex;
  align-items: center;
  padding: 0 14px;
  justify-content: space-between;
  flex-shrink: 0;
}

.traffic-lights {
  display: flex;
  gap: 7px;
  align-items: center;
}
.tl { width: 12px; height: 12px; border-radius: 50%; cursor: pointer; }
.tl-r { background: #ff5f57; box-shadow: 0 0 0 0.5px rgba(0,0,0,0.3); }
.tl-y { background: #febc2e; box-shadow: 0 0 0 0.5px rgba(0,0,0,0.3); }
.tl-g { background: #28c840; box-shadow: 0 0 0 0.5px rgba(0,0,0,0.3); }

.window-title {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text3, #8b8f96);
}
.window-title span {
  color: var(--text, #e8eaec);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text3, #5c6168);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  transition: all 0.15s;
}
.close-btn:hover {
  background: rgba(255,255,255,0.08);
  color: var(--text, #e8eaec);
}

.pro-terminal-area {
  flex: 1;
  overflow: hidden;
}

.pro-terminal-element {
  width: 100%;
  height: 100%;
  padding: 6px 10px;
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
