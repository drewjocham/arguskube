<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useTerminal } from '../../composables/useWails'
import { useWailsEvent } from '../../composables/useEvents'

const props = defineProps({
  visible: { type: Boolean, default: false },
})

const termRef = ref(null)
const { startTerminal, sendInput, resizeTerminal } = useTerminal()

let term = null
let fitAddon = null
let started = false

async function initTerminal() {
  if (term || !termRef.value) return

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
    await startTerminal(term.rows, term.cols)
  }
}

useWailsEvent('terminal:output', (data) => {
  if (term && data) {
    term.write(data)
  }
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
  }
})

onMounted(() => {
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver.value = new ResizeObserver(() => {
      handleResize()
    })
    if (termRef.value) {
      resizeObserver.value.observe(termRef.value)
    }
  }

  window.addEventListener('resize', handleResize)
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
    <div ref="termRef" class="terminal-element"></div>
  </div>
</template>

<style scoped>
.terminal-container {
  width: 100%;
  height: 100%;
  background: var(--bg);
  overflow: hidden;
}

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
