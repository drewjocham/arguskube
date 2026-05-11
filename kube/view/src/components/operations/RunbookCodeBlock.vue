<script setup>
import { ref, computed } from 'vue'
import { useRunbookTerminalsStore } from '../../stores/runbookTerminals'
import { useTerminalDispatchStore } from '../../stores/terminalDispatch'

const props = defineProps({
  code: { type: String, required: true },
  language: { type: String, default: '' },
  section: { type: String, default: 'Section' },
  sectionId: { type: String, default: 'default' },
  codeIndex: { type: Number, default: 0 },
  runbookId: { type: String, required: true },
})

const sessions = useRunbookTerminalsStore()
const dispatch = useTerminalDispatchStore()

const copied = ref(false)
const sent = ref(false)
const copyError = ref(null)

const targetSession = computed(() =>
  sessions.resolveTarget(props.runbookId, props.sectionId, props.codeIndex),
)

const isPinnedDoc = computed(() => sessions.isPinned(props.runbookId))

// A short, human-readable label for the session — strips the runbook
// prefix so the user sees "verify-pods" not "rb-7::verify-pods".
const targetLabel = computed(() => {
  const t = targetSession.value || ''
  const parts = t.split('::')
  const raw = parts[parts.length - 1] || 'default'
  if (raw === 'pin') return 'pinned'
  return raw
})

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
  // Dispatch the command to whichever session this block resolves to.
  // The terminal-dispatch store carries (text, sessionId); the embedded
  // TerminalView reads pendingCommand and writes it into the active xterm.
  // sessionId is forward-looking — when a multi-PTY backend lands, the
  // dispatch will route to the right session. Today the embedded terminal
  // is single-session so the sessionId becomes a label the user sees in a
  // header comment.
  dispatch.sendToTerminal(props.code, { sessionId: targetSession.value, sectionLabel: props.section })
  sent.value = true
  setTimeout(() => { sent.value = false }, 1500)
}

function togglePinDoc() {
  if (isPinnedDoc.value) {
    sessions.unpinDocument(props.runbookId)
  } else {
    sessions.pinDocument(props.runbookId, props.sectionId)
  }
}

const isShellLang = computed(() => {
  const lang = (props.language || '').toLowerCase()
  if (!lang) return true
  return ['sh', 'bash', 'zsh', 'shell', 'console', 'kubectl', 'fish'].includes(lang)
})
</script>

<template>
  <div class="runbook-code-block">
    <div class="rcb-toolbar">
      <span class="rcb-lang">{{ language || 'text' }}</span>
      <span class="rcb-session" :title="'Target terminal session: ' + targetSession">
        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
        {{ targetLabel }}
        <span v-if="isPinnedDoc" class="rcb-pin-marker" title="Document-wide pin is active">📌</span>
      </span>
      <div class="rcb-actions">
        <button
          class="rcb-btn"
          :class="{ active: isPinnedDoc }"
          @click="togglePinDoc"
          :title="isPinnedDoc ? 'Unpin: blocks return to per-section sessions' : 'Pin this section for the whole document'"
        >
          {{ isPinnedDoc ? 'Unpin doc' : 'Pin doc' }}
        </button>
        <button v-if="isShellLang" class="rcb-btn run-btn" @click="run" :title="'Send to terminal session: ' + targetSession">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
          {{ sent ? 'Sent →' : 'Run' }}
        </button>
        <button class="rcb-btn copy-btn" @click="copy" title="Copy to clipboard">
          <svg v-if="!copied" width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          <svg v-else width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>
          {{ copied ? 'Copied' : 'Copy' }}
        </button>
      </div>
    </div>
    <pre class="rcb-pre"><code class="rcb-code font-mono">{{ code }}</code></pre>
    <div v-if="copyError" class="rcb-error">{{ copyError }}</div>
  </div>
</template>

<style scoped>
.runbook-code-block {
  background: #0f1012;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  margin: 8px 0;
  overflow: hidden;
}
.rcb-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 8px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}
.rcb-lang {
  font-family: var(--mono);
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: #6b7078;
  flex-shrink: 0;
}
.rcb-session {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-family: var(--mono);
  font-size: 10.5px;
  color: #6ba3f9;
  background: rgba(79, 142, 247, 0.1);
  padding: 2px 8px;
  border-radius: 10px;
}
.rcb-pin-marker { font-size: 10px; }

.rcb-actions { margin-left: auto; display: flex; gap: 4px; }
.rcb-btn {
  display: inline-flex; align-items: center; gap: 4px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #b0b4ba;
  padding: 3px 8px;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s;
}
.rcb-btn:hover { background: rgba(255, 255, 255, 0.12); color: #e8eaec; }
.rcb-btn.active { background: rgba(245, 166, 35, 0.18); color: #f5a623; border-color: rgba(245, 166, 35, 0.4); }
.rcb-btn.run-btn { color: #a78bfa; border-color: rgba(167, 139, 250, 0.3); }
.rcb-btn.run-btn:hover { background: rgba(167, 139, 250, 0.18); color: #fff; }
.rcb-btn.copy-btn:hover { color: #4f8ef7; border-color: rgba(79, 142, 247, 0.3); }

.rcb-pre {
  margin: 0;
  padding: 10px 12px;
  overflow: auto;
  max-height: min(50vh, 240px);
  background: #0f1012;
}
.rcb-code {
  display: block;
  font-size: 12px;
  line-height: 1.5;
  color: #c9d1d9;
  white-space: pre;
  tab-size: 2;
}
.rcb-error {
  padding: 4px 12px;
  font-size: 11px;
  color: #f05454;
  background: rgba(240, 84, 84, 0.08);
  border-top: 1px solid rgba(240, 84, 84, 0.2);
}
</style>
