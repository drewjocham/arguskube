<script setup>
import { ref, onMounted, onUnmounted, nextTick, computed } from 'vue'
import { useTerminal } from '../../composables/useWails'
import { bus } from '../../lib/bus'

// ProDesktopApp — the in-app pop-out terminal "window" rendered as a
// movable floating panel on top of the dashboard. It is NOT a real OS
// window (LaunchPopOutTerminal handles that path); this is the fallback
// for web/SaaS mode where the user can't get a true second OS window.
//
// Two prior complaints fixed here:
//   1. It looked like "a terminal inside a terminal" — fake macOS chrome
//      sitting in the middle of the dashboard with no movement, so it
//      visually competed with the embedded bottom terminal. Reworked the
//      titlebar so it reads as a tool window (drag grip + label + close
//      only) instead of mimicking the real OS frame.
//   2. It was not draggable — the previous version relied on the Wails
//      `--wails-draggable: drag` style that only moves the native window;
//      a positioned div in the renderer ignores it. Replaced with
//      explicit mousedown/move/up handlers.

const props = defineProps({
  standalone: { type: Boolean, default: false },
})

const emit = defineEmits(['close'])

const termRef = ref(null)
const windowEl = ref(null)
const { startTerminal, sendInput, resizeTerminal } = useTerminal()

let term = null
let fitAddon = null
let started = false
let resizeObs = null

// ── Floating-window position + size ─────────────────────────────
// State lives in pixels relative to the viewport. We seed from the
// middle of the screen on mount; users can drag and resize freely.
const DEFAULT_W = 880
const DEFAULT_H = 520
const winLeft = ref(0)
const winTop = ref(0)
const winWidth = ref(DEFAULT_W)
const winHeight = ref(DEFAULT_H)
const isMaximized = ref(false)
// Saved geometry so "restore" returns to the user's prior position
// instead of resetting to center. Captured on every maximize.
let savedGeometry = null

function seedPosition() {
  // Initial spawn: roughly centered, but offset slightly up so the
  // dashboard's embedded terminal at the bottom of the viewport stays
  // visible. We don't use full-center because that conflicts with the
  // pattern the user already has open at the bottom of the screen.
  const vw = window.innerWidth
  const vh = window.innerHeight
  winWidth.value = Math.min(DEFAULT_W, Math.max(480, vw - 80))
  winHeight.value = Math.min(DEFAULT_H, Math.max(320, vh - 120))
  winLeft.value = Math.max(20, Math.round((vw - winWidth.value) / 2))
  winTop.value = Math.max(20, Math.round((vh - winHeight.value) / 2) - 40)
}

// ── Dragging ─────────────────────────────────────────────────────
let dragOffsetX = 0
let dragOffsetY = 0
let dragging = false

function onTitlebarMouseDown(e) {
  // Ignore drags that start on an interactive titlebar element (close
  // button, maximize) so those still receive their click.
  if (e.target.closest('.no-drag')) return
  if (isMaximized.value) return
  dragging = true
  dragOffsetX = e.clientX - winLeft.value
  dragOffsetY = e.clientY - winTop.value
  document.addEventListener('mousemove', onDragMove)
  document.addEventListener('mouseup', onDragEnd)
  e.preventDefault()
}
function onDragMove(e) {
  if (!dragging) return
  const vw = window.innerWidth
  const vh = window.innerHeight
  let nl = e.clientX - dragOffsetX
  let nt = e.clientY - dragOffsetY
  // Keep at least 60px of the titlebar reachable so the user can never
  // throw the window off-screen and lose it.
  nl = Math.max(60 - winWidth.value, Math.min(vw - 60, nl))
  nt = Math.max(0, Math.min(vh - 40, nt))
  winLeft.value = nl
  winTop.value = nt
}
function onDragEnd() {
  dragging = false
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
}

// ── Resizing ─────────────────────────────────────────────────────
// Single corner handle on the bottom-right. Min size big enough that the
// xterm grid still has room to render legibly.
const MIN_W = 360
const MIN_H = 220
let resizing = false
let resizeStartX = 0
let resizeStartY = 0
let resizeStartW = 0
let resizeStartH = 0

function onResizeMouseDown(e) {
  if (isMaximized.value) return
  resizing = true
  resizeStartX = e.clientX
  resizeStartY = e.clientY
  resizeStartW = winWidth.value
  resizeStartH = winHeight.value
  document.addEventListener('mousemove', onResizeMove)
  document.addEventListener('mouseup', onResizeEnd)
  e.preventDefault()
  e.stopPropagation()
}
function onResizeMove(e) {
  if (!resizing) return
  const vw = window.innerWidth
  const vh = window.innerHeight
  winWidth.value = Math.max(MIN_W, Math.min(vw - winLeft.value - 10, resizeStartW + (e.clientX - resizeStartX)))
  winHeight.value = Math.max(MIN_H, Math.min(vh - winTop.value - 10, resizeStartH + (e.clientY - resizeStartY)))
}
function onResizeEnd() {
  resizing = false
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
}

// ── Maximize ─────────────────────────────────────────────────────
function toggleMaximize() {
  if (isMaximized.value) {
    if (savedGeometry) {
      winLeft.value = savedGeometry.left
      winTop.value = savedGeometry.top
      winWidth.value = savedGeometry.width
      winHeight.value = savedGeometry.height
    }
    isMaximized.value = false
  } else {
    savedGeometry = {
      left: winLeft.value, top: winTop.value,
      width: winWidth.value, height: winHeight.value,
    }
    isMaximized.value = true
  }
}

const winStyle = computed(() => {
  if (isMaximized.value) {
    return { left: '12px', top: '12px', width: 'calc(100vw - 24px)', height: 'calc(100vh - 24px)' }
  }
  return {
    left: winLeft.value + 'px',
    top: winTop.value + 'px',
    width: winWidth.value + 'px',
    height: winHeight.value + 'px',
  }
})

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

function onKeydown(e) {
  if (e.key === 'Escape' && !props.standalone) {
    handleClose()
  }
}

onMounted(async () => {
  seedPosition()
  window.addEventListener('keydown', onKeydown)
  await nextTick()
  await initTerminal()
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
  resizeObs?.disconnect()
  term?.dispose()
  term = null
})
</script>

<template>
  <!-- No backdrop dim — this is a floating tool window, not a modal.
       Click-through to the rest of the dashboard is intentional so the
       user can keep working while watching command output. -->
  <div
    ref="windowEl"
    class="popout-window"
    :class="{ maximized: isMaximized, dragging }"
    :style="winStyle"
    role="dialog"
    aria-label="Argus pop-out terminal"
  >
    <!-- Drag grip + label + window controls. The grip dots make the
         affordance obvious; the whole bar is the drag target except
         elements marked .no-drag. -->
    <div class="popout-titlebar" @mousedown="onTitlebarMouseDown" @dblclick="toggleMaximize">
      <div class="grip" aria-hidden="true">
        <span /><span /><span />
        <span /><span /><span />
      </div>
      <div class="title">Terminal</div>
      <div class="ctrls no-drag">
        <button class="ctrl-btn" @click="toggleMaximize" :title="isMaximized ? 'Restore' : 'Maximize'">
          <svg v-if="!isMaximized" width="12" height="12" viewBox="0 0 12 12"><rect x="1.5" y="1.5" width="9" height="9" fill="none" stroke="currentColor" stroke-width="1.2" /></svg>
          <svg v-else width="12" height="12" viewBox="0 0 12 12"><rect x="1.5" y="3.5" width="7" height="7" fill="none" stroke="currentColor" stroke-width="1.2" /><rect x="3.5" y="1.5" width="7" height="7" fill="none" stroke="currentColor" stroke-width="1.2" /></svg>
        </button>
        <button class="ctrl-btn close" @click="handleClose" title="Close (Esc)">
          <svg width="12" height="12" viewBox="0 0 12 12"><path d="M2 2l8 8M10 2l-8 8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" fill="none"/></svg>
        </button>
      </div>
    </div>

    <div class="popout-body">
      <div ref="termRef" class="popout-term"></div>
    </div>

    <div v-if="!isMaximized" class="resize-handle" @mousedown="onResizeMouseDown" aria-hidden="true">
      <svg width="14" height="14" viewBox="0 0 14 14"><path d="M13 5 5 13M13 9 9 13M13 13 13 13" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" fill="none"/></svg>
    </div>
  </div>
</template>

<style scoped>
.popout-window {
  position: fixed;
  background: #0d0d0d;
  border-radius: 10px;
  border: 1px solid rgba(255,255,255,0.14);
  box-shadow:
    0 24px 60px rgba(0,0,0,0.55),
    0 4px 14px rgba(0,0,0,0.4),
    0 0 0 1px rgba(255,255,255,0.04) inset;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  z-index: 10000;
  animation: pop-in 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}
.popout-window.dragging { transition: none; user-select: none; }
.popout-window.maximized { border-radius: 6px; }

@keyframes pop-in {
  from { opacity: 0; transform: scale(0.97); }
  to { opacity: 1; transform: scale(1); }
}

/* Drag handle bar — taller and more obviously interactive than the
   previous "fake macOS frame" version, so the user reads it as a
   tool-window header rather than as part of the embedded terminal. */
.popout-titlebar {
  height: 34px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 8px 0 10px;
  background: linear-gradient(180deg, #232629 0%, #1c1f22 100%);
  border-bottom: 1px solid rgba(255,255,255,0.06);
  cursor: grab;
  user-select: none;
}
.popout-window.dragging .popout-titlebar { cursor: grabbing; }

.grip {
  display: grid;
  grid-template-columns: repeat(2, 3px);
  grid-template-rows: repeat(3, 3px);
  gap: 2px;
  margin-right: 2px;
  flex-shrink: 0;
}
.grip span {
  width: 3px; height: 3px;
  background: rgba(255,255,255,0.32);
  border-radius: 50%;
}

.title {
  flex: 1;
  font-size: 12.5px;
  font-weight: 500;
  color: var(--text, #e8eaec);
  letter-spacing: 0.01em;
  pointer-events: none;
}

.ctrls { display: flex; gap: 4px; align-items: center; }
.ctrl-btn {
  background: none;
  border: none;
  color: var(--text3, #8b8f96);
  cursor: pointer;
  padding: 4px 5px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  transition: background 0.12s, color 0.12s;
}
.ctrl-btn:hover { background: rgba(255,255,255,0.08); color: var(--text, #e8eaec); }
.ctrl-btn.close:hover { background: rgba(240,84,84,0.85); color: #fff; }

.popout-body {
  flex: 1;
  overflow: hidden;
  background: #0d0d0d;
}
.popout-term {
  width: 100%;
  height: 100%;
  padding: 6px 10px;
}

.resize-handle {
  position: absolute;
  right: 0; bottom: 0;
  width: 16px; height: 16px;
  cursor: nwse-resize;
  display: flex;
  align-items: center;
  justify-content: center;
  color: rgba(255,255,255,0.25);
}
.resize-handle:hover { color: rgba(255,255,255,0.55); }

:deep(.xterm) { padding: 0; }
:deep(.xterm-viewport) { overflow-y: auto !important; }
:deep(.xterm-viewport::-webkit-scrollbar) { width: 5px; }
:deep(.xterm-viewport::-webkit-scrollbar-thumb) {
  background: rgba(255,255,255,0.1);
  border-radius: 3px;
}
</style>
