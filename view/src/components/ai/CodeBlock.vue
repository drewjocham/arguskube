<script setup>
import { ref, computed, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { isShellLanguage } from '../../utils/parseCodeBlocks'
import { useTerminalDispatchStore } from '../../stores/terminalDispatch'
import { useOutputCaptureStore } from '../../stores/outputCapture'

const props = defineProps({
  code: { type: String, required: true },
  language: { type: String, default: '' },
  // When false, hide the "Send to terminal" button regardless of language.
  allowRun: { type: Boolean, default: true },
})

const dispatch = useTerminalDispatchStore()
const capture = useOutputCaptureStore()
const { activeBlockId, buffers } = storeToRefs(capture)

// Each CodeBlock instance owns a stable id so we can route captured terminal
// output back into the correct block. crypto.randomUUID is available in
// Wails (Chromium) and modern browsers.
const blockId = (typeof crypto !== 'undefined' && crypto.randomUUID)
  ? crypto.randomUUID()
  : `cb-${Math.random().toString(36).slice(2)}-${Date.now()}`

const copied = ref(false)
const sent = ref(false)
const copyError = ref(null)

const displayLanguage = computed(() => props.language || 'text')
const canRun = computed(() => props.allowRun && isShellLanguage(props.language))
const outputText = computed(() => buffers.value[blockId] || '')
const isCapturing = computed(() => activeBlockId.value === blockId)
const showOutput = computed(() => outputText.value.length > 0 || isCapturing.value)

async function copy() {
  copyError.value = null
  try {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(props.code)
    } else {
      const ta = document.createElement('textarea')
      ta.value = props.code
      ta.setAttribute('readonly', '')
      ta.style.position = 'absolute'
      ta.style.left = '-9999px'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    copied.value = true
    setTimeout(() => { copied.value = false }, 1500)
  } catch (e) {
    copyError.value = e?.message || 'Copy failed'
    setTimeout(() => { copyError.value = null }, 2500)
  }
}

function run() {
  // Take ownership of terminal-output capture; previous block's buffer is
  // preserved but no new chunks land there.
  capture.startCapture(blockId)
  dispatch.sendToTerminal(props.code)
  sent.value = true
  setTimeout(() => { sent.value = false }, 1500)
}

function stopCapture() {
  capture.stopCapture(blockId)
}

function clearOutput() {
  capture.clearBuffer(blockId)
}

onUnmounted(() => {
  if (isCapturing.value) {
    capture.stopCapture(blockId)
  }
})
</script>

<template>
  <div class="code-block" :class="{ 'is-shell': canRun }">
    <div class="code-toolbar">
      <span class="code-lang">{{ displayLanguage }}</span>
      <div class="code-actions">
        <button
          v-if="canRun"
          class="code-btn run-btn"
          :title="'Open the terminal panel and paste this command (Enter to execute)'"
          @click="run"
        >
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
          {{ sent ? 'Sent →' : 'Run in terminal' }}
        </button>
        <button class="code-btn copy-btn" :title="'Copy to clipboard'" @click="copy">
          <svg v-if="!copied" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          <svg v-else width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>
          {{ copied ? 'Copied' : 'Copy' }}
        </button>
      </div>
    </div>
    <pre class="code-pre"><code class="code-content font-mono">{{ code }}</code></pre>
    <div v-if="copyError" class="code-error">{{ copyError }}</div>

    <div v-if="showOutput" class="code-output">
      <div class="output-header">
        <span class="output-label">
          <span class="output-dot" :class="{ live: isCapturing }"></span>
          {{ isCapturing ? 'Capturing terminal output' : 'Captured output' }}
        </span>
        <div class="output-actions">
          <button v-if="isCapturing" class="output-btn" @click="stopCapture">Stop</button>
          <button v-if="outputText" class="output-btn" @click="clearOutput">Clear</button>
        </div>
      </div>
      <pre class="output-pre"><code class="output-content font-mono">{{ outputText || 'Waiting for output… press Enter in the terminal to execute.' }}</code></pre>
    </div>
  </div>
</template>

<style scoped>
.code-block {
  background: #0f1012;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  margin: 8px 0;
  overflow: hidden;
}
.code-block.is-shell { border-left: 2px solid #a78bfa; }

.code-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}
.code-lang {
  font-family: var(--mono);
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #6b7078;
}
.code-actions { display: flex; gap: 4px; }

.code-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #b0b4ba;
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s;
}
.code-btn:hover { background: rgba(255, 255, 255, 0.12); color: #e8eaec; }
.code-btn.run-btn { color: #a78bfa; border-color: rgba(167, 139, 250, 0.3); }
.code-btn.run-btn:hover { background: rgba(167, 139, 250, 0.18); color: #fff; }
.code-btn.copy-btn:hover { color: #4f8ef7; border-color: rgba(79, 142, 247, 0.3); }

.code-pre {
  margin: 0;
  padding: 10px 12px;
  overflow: auto;
  /* Show roughly 10 lines of code (12px font * 1.5 line-height = 18px) before
     the user has to scroll. The block also caps at ~50% viewport so a giant
     manifest doesn't take over the chat. */
  max-height: min(50vh, 200px);
  background: #0f1012;
}
.code-content {
  display: block;
  font-size: 12px;
  line-height: 1.5;
  color: #c9d1d9;
  white-space: pre;
  tab-size: 2;
}

/* Inline output panel, populated when the user clicks Run. Captures the
   terminal:output stream while this block is the active target. */
.code-output {
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  background: #0d0d0d;
}
.output-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 4px 10px;
  font-size: 10.5px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #8b8f96;
  background: rgba(255, 255, 255, 0.02);
}
.output-label { display: inline-flex; align-items: center; gap: 6px; font-weight: 600; }
.output-dot {
  width: 6px; height: 6px; border-radius: 50%;
  background: #6b7078;
}
.output-dot.live {
  background: #3ecf8e;
  animation: capture-pulse 1.4s ease-in-out infinite;
}
@keyframes capture-pulse {
  0%, 100% { opacity: 1; }
  50%      { opacity: 0.35; }
}
.output-actions { display: flex; gap: 4px; }
.output-btn {
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #b0b4ba;
  padding: 1px 7px;
  border-radius: 3px;
  font-size: 10px;
  cursor: pointer;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.output-btn:hover { background: rgba(255, 255, 255, 0.08); color: #fff; }

.output-pre {
  margin: 0;
  padding: 8px 12px;
  overflow: auto;
  max-height: min(50vh, 200px);
  background: #0d0d0d;
}
.output-content {
  display: block;
  font-size: 11.5px;
  line-height: 1.45;
  color: #b0b4ba;
  white-space: pre-wrap;
  word-break: break-word;
}
.code-error {
  padding: 4px 12px;
  font-size: 11px;
  color: #f05454;
  background: rgba(240, 84, 84, 0.08);
  border-top: 1px solid rgba(240, 84, 84, 0.2);
}
</style>
