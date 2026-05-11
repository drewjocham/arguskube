<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { storeToRefs } from 'pinia'
import { useChat } from '../../composables/useWails'
import { parseCodeBlocks } from '../../utils/parseCodeBlocks'
import { renderMarkdown } from '../../utils/renderMarkdown'
import { useArgusContextStore } from '../../stores/argusContext'
import { useChatSessionsStore } from '../../stores/chatSessions'
import CodeBlock from './CodeBlock.vue'

const argusContext = useArgusContextStore()
const { pending: pendingContext } = storeToRefs(argusContext)

// Multi-session chat: each session is its own backend history thread
// keyed by id. The store keeps frontend metadata (title, last activity,
// counts) and tells us which session is "active". Switching sessions
// is just refreshing history with a new id.
const sessions = useChatSessionsStore()
const { activeId, sortedSessions } = storeToRefs(sessions)
const sessionsListOpen = ref(false)

const { history, sending, sendMessage, refreshHistory } = useChat()

const question = ref('')
const chatScrollEl = ref(null)
const errorMessage = ref(null)

const visibleHistory = computed(() =>
  (history.value || []).filter(m => m.role !== 'system')
)

const suggestions = [
  'What workloads are unhealthy in my cluster?',
  'Why are pods restarting in the kube-system namespace?',
  'Summarize current alerts and recent events.',
  'Recommend resource limits for high-restart pods.',
]

onMounted(() => {
  refreshHistory(activeId.value)
})

// Refetch history when the user switches sessions. Each session has
// its own backend thread, so this swap is a clean cut — old messages
// don't leak into the new view.
watch(activeId, (id) => {
  if (id) refreshHistory(id)
})

function switchSession(id) {
  sessions.setActive(id)
  sessionsListOpen.value = false
}

function newSession() {
  sessions.create()
  sessionsListOpen.value = false
}

function deleteSession(id) {
  sessions.remove(id)
}

function renameSession(s) {
  const next = window.prompt('Rename session', s.title)
  if (next != null) sessions.rename(s.id, next)
}

watch(history, async () => {
  await nextTick()
  if (chatScrollEl.value) {
    chatScrollEl.value.scrollTop = chatScrollEl.value.scrollHeight
  }
}, { deep: true })

async function onSend() {
  const val = question.value.trim()
  if (!val || sending.value) return
  question.value = ''
  errorMessage.value = null
  // Prepend any pending Argus context (a selected finding, network policy,
  // etc.) so the agent has scope without forcing the user to re-type it.
  const ctx = argusContext.consumeForSend()
  const payload = ctx ? `${ctx}\n\nUser question: ${val}` : val
  // Auto-name the session from the first user message. Subsequent
  // messages don't change the title.
  sessions.autoTitleFromFirstMessage(activeId.value, val)
  try {
    await sendMessage(activeId.value, payload)
    sessions.recordMessage(activeId.value)
  } catch (e) {
    errorMessage.value = e?.message || String(e)
  }
}

function onKeydown(e) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    onSend()
  }
}

function fillSuggestion(text) {
  question.value = text
}

function formatTime(ts) {
  if (!ts) return ''
  return new Date(ts).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
}
</script>

<template>
  <div class="argus-ai-view">
    <div class="header">
      <div class="header-icon" aria-hidden="true">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="9"/>
          <path d="M9 11h.01"/>
          <path d="M15 11h.01"/>
          <path d="M9 15c1 1 2 1.5 3 1.5s2-.5 3-1.5"/>
        </svg>
      </div>
      <div class="header-text">
        <div class="title">Argus AI</div>
        <div class="subtitle">Conversational assistant for cluster diagnostics, anomaly investigation, and config recommendations.</div>
      </div>
      <!-- Session switcher: dropdown of active threads, current title
           inline, "New session" creates a fresh thread on the same
           agent. The agent's history is keyed by session id on the
           backend, so switching is a clean cut. -->
      <div class="session-switcher">
        <button class="session-current" @click="sessionsListOpen = !sessionsListOpen" :title="sessions.activeSession?.title">
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"></path>
          </svg>
          <span class="session-current-label">{{ sessions.activeSession?.title || 'Session' }}</span>
          <span class="session-current-arrow" :class="{ open: sessionsListOpen }">▾</span>
        </button>
        <div v-if="sessionsListOpen" class="session-menu">
          <button class="session-action" @click="newSession">
            <span class="session-action-plus">+</span> New session
          </button>
          <div class="session-divider"></div>
          <div class="session-list">
            <div
              v-for="s in sortedSessions"
              :key="s.id"
              class="session-item"
              :class="{ active: s.id === activeId }"
              @click="switchSession(s.id)"
            >
              <div class="session-item-main">
                <div class="session-item-title">
                  <span v-if="s.pinned" class="session-pin" title="Pinned (cannot be deleted)">📌</span>
                  {{ s.title }}
                </div>
                <div class="session-item-meta">
                  {{ s.messageCount || 0 }} msg · {{ formatTime(s.lastActivity) }}
                </div>
              </div>
              <div class="session-item-actions" @click.stop>
                <button
                  class="session-icon-btn"
                  @click="renameSession(s)"
                  title="Rename"
                >✎</button>
                <button
                  v-if="!s.pinned"
                  class="session-icon-btn danger"
                  @click="deleteSession(s.id)"
                  title="Delete session"
                >×</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="chat-panel">
      <div ref="chatScrollEl" class="chat-scroll" data-test="chat-scroll">
        <div v-if="visibleHistory.length === 0" class="empty-state">
          <div class="empty-title">Start a conversation</div>
          <div class="empty-sub">Argus AI has live access to your cluster metrics, recent events, and alerts.</div>
          <div class="suggestions">
            <button
              v-for="s in suggestions"
              :key="s"
              class="suggestion-btn"
              @click="fillSuggestion(s)"
            >{{ s }}</button>
          </div>
        </div>

        <div v-else class="messages">
          <div
            v-for="(msg, i) in visibleHistory"
            :key="i"
            class="message"
            :class="msg.role"
          >
            <div class="message-meta">
              <span class="message-author">{{ msg.role === 'assistant' ? 'Argus AI' : 'You' }}</span>
              <span class="message-time">{{ formatTime(msg.timestamp) }}</span>
            </div>
            <div class="message-body">
              <template v-for="(seg, segIdx) in parseCodeBlocks(msg.content)" :key="segIdx">
                <div
                  v-if="seg.type === 'text'"
                  class="message-text markdown-body"
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
          </div>
          <div v-if="sending" class="message assistant typing">
            <div class="message-meta">
              <span class="message-author">Argus AI</span>
            </div>
            <div class="message-body">
              <span class="dot"></span><span class="dot"></span><span class="dot"></span>
            </div>
          </div>
        </div>
      </div>

      <div v-if="errorMessage" class="error-banner">
        <span class="error-icon">!</span>
        <span class="error-text">{{ errorMessage }}</span>
        <button class="error-close" @click="errorMessage = null">×</button>
      </div>

      <div v-if="pendingContext" class="context-chip">
        <span class="context-chip-kind">{{ pendingContext.kind }}</span>
        <span class="context-chip-label" :title="pendingContext.label">{{ pendingContext.label }}</span>
        <button class="context-chip-close" @click="argusContext.clearContext()" title="Detach this context">×</button>
      </div>

      <div class="composer">
        <textarea
          class="composer-input font-mono"
          v-model="question"
          @keydown="onKeydown"
          placeholder="Ask Argus AI about your cluster… (Enter to send, Shift+Enter for newline)"
          rows="2"
          spellcheck="false"
        ></textarea>
        <button
          class="send-btn"
          :disabled="sending || !question.trim()"
          @click="onSend"
        >
          {{ sending ? 'Sending…' : 'Send' }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.argus-ai-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.header {
  display: flex; align-items: center; gap: 12px;
  flex-shrink: 0;
}
.header-icon {
  width: 36px; height: 36px;
  display: flex; align-items: center; justify-content: center;
  border-radius: 8px;
  background: rgba(167, 139, 250, 0.15);
  color: #a78bfa;
}
.header-text { display: flex; flex-direction: column; gap: 2px; flex: 1; min-width: 0; }
.title { font-size: 20px; font-weight: 500; color: #fff; }
.subtitle { font-size: 13px; color: #8b8f96; max-width: 720px; }

/* --- Session switcher ------------------------------------------------- */

.session-switcher { position: relative; flex-shrink: 0; }
.session-current {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 6px 10px; border-radius: 6px;
  background: var(--bg3); border: 1px solid var(--border);
  color: var(--text); font: inherit; font-size: 12px;
  max-width: 220px; cursor: pointer; transition: all 0.15s;
}
.session-current:hover { background: var(--bg4); border-color: var(--border2); }
.session-current svg { flex-shrink: 0; color: #a78bfa; }
.session-current-label {
  flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.session-current-arrow { color: var(--text3); transition: transform 0.15s; }
.session-current-arrow.open { transform: rotate(180deg); }

.session-menu {
  position: absolute; top: calc(100% + 6px); right: 0;
  width: 320px; max-height: 400px; overflow: hidden;
  background: var(--bg2); border: 1px solid var(--border2); border-radius: 8px;
  box-shadow: var(--shadow2);
  z-index: 50;
  display: flex; flex-direction: column;
}
.session-action {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 12px; border: 0; background: transparent;
  color: var(--text); font: inherit; font-size: 12.5px;
  cursor: pointer; text-align: left; width: 100%;
}
.session-action:hover { background: var(--bg3); }
.session-action-plus {
  display: inline-flex; width: 18px; height: 18px;
  align-items: center; justify-content: center;
  background: rgba(79,142,247,0.15); color: var(--accent2);
  border-radius: 50%; font-weight: 600;
}
.session-divider { height: 1px; background: var(--border); }
.session-list { overflow-y: auto; flex: 1; }
.session-item {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 12px; cursor: pointer;
  border-bottom: 1px solid var(--border);
  transition: background 0.15s;
}
.session-item:last-child { border-bottom: 0; }
.session-item:hover { background: var(--bg3); }
.session-item.active {
  background: rgba(79,142,247,0.08);
}
.session-item-main { flex: 1; min-width: 0; }
.session-item-title {
  font-size: 12.5px; color: var(--text);
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.session-pin { font-size: 9px; margin-right: 4px; }
.session-item-meta {
  font-size: 10.5px; color: var(--text3);
  font-family: var(--mono);
  margin-top: 2px;
}
.session-item-actions { display: flex; gap: 2px; flex-shrink: 0; }
.session-icon-btn {
  width: 22px; height: 22px;
  background: transparent; border: 0;
  color: var(--text3); font: inherit; font-size: 13px;
  cursor: pointer; border-radius: 4px;
  display: flex; align-items: center; justify-content: center;
}
.session-icon-btn:hover { background: var(--bg4); color: var(--text); }
.session-icon-btn.danger:hover { background: rgba(240,84,84,0.15); color: var(--red2); }

.chat-panel {
  flex: 1;
  min-height: 0;
  display: flex; flex-direction: column;
  background: #1a1b1e;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  overflow: hidden;
}

.chat-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px 20px;
}

.empty-state {
  display: flex; flex-direction: column;
  align-items: center; justify-content: center;
  text-align: center;
  height: 100%;
  gap: 12px;
  color: #b0b4ba;
  padding: 40px 16px;
}
.empty-title { font-size: 15px; font-weight: 600; color: #e8eaec; }
.empty-sub { font-size: 13px; color: #8b8f96; max-width: 480px; }
.suggestions {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 8px;
  margin-top: 12px;
  width: 100%;
  max-width: 720px;
}
.suggestion-btn {
  background: rgba(167, 139, 250, 0.08);
  border: 1px solid rgba(167, 139, 250, 0.2);
  color: #c9d1d9;
  border-radius: 8px;
  padding: 10px 14px;
  font-size: 12.5px;
  cursor: pointer;
  text-align: left;
  transition: all 0.15s;
}
.suggestion-btn:hover {
  background: rgba(167, 139, 250, 0.15);
  border-color: rgba(167, 139, 250, 0.4);
  color: #fff;
}

.messages {
  display: flex; flex-direction: column;
  gap: 16px;
}
.message {
  display: flex; flex-direction: column;
  gap: 4px;
  max-width: 78%;
}
.message.user {
  align-self: flex-end;
  align-items: flex-end;
}
.message.assistant { align-self: flex-start; }
.message-meta {
  display: flex; gap: 8px; align-items: center;
  font-size: 11px;
  color: #6b7078;
}
.message-author { font-weight: 600; color: #8b8f96; }
.message-time { font-family: var(--mono); }
.message-body {
  padding: 10px 14px;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.55;
  word-break: break-word;
}
.markdown-body :deep(p) { margin: 0 0 8px 0; }
.markdown-body :deep(p:last-child) { margin-bottom: 0; }
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3),
.markdown-body :deep(h4) {
  font-weight: 600; color: #fff; margin: 10px 0 4px 0; line-height: 1.25;
}
.markdown-body :deep(h1) { font-size: 16px; }
.markdown-body :deep(h2) { font-size: 15px; }
.markdown-body :deep(h3) { font-size: 14px; }
.markdown-body :deep(h4) { font-size: 13px; }
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: 18px; margin: 4px 0;
}
.markdown-body :deep(li) { margin: 2px 0; }
.markdown-body :deep(strong) { font-weight: 600; color: #fff; }
.markdown-body :deep(em) { font-style: italic; }
.markdown-body :deep(blockquote) {
  border-left: 2px solid rgba(167, 139, 250, 0.5);
  padding: 2px 10px; margin: 6px 0;
  color: #b0b4ba; background: rgba(167, 139, 250, 0.06);
}
.markdown-body :deep(code) {
  background: rgba(255, 255, 255, 0.07);
  padding: 1px 5px; border-radius: 3px;
  font-family: var(--mono); font-size: 11.5px;
  color: #c9d1d9;
}
.markdown-body :deep(a) {
  color: #6ba3f9; text-decoration: underline;
}
.markdown-body :deep(a:hover) { color: #8dbafd; }
.markdown-body :deep(hr) {
  border: none; border-top: 1px solid rgba(255, 255, 255, 0.08); margin: 8px 0;
}
.markdown-body :deep(table) {
  border-collapse: collapse; margin: 6px 0; font-size: 12px;
}
.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid rgba(255, 255, 255, 0.1); padding: 4px 8px; text-align: left;
}
.markdown-body :deep(th) { background: rgba(255, 255, 255, 0.04); font-weight: 600; }
.message.user .message-body {
  background: rgba(79, 142, 247, 0.18);
  border: 1px solid rgba(79, 142, 247, 0.3);
  color: #e8eaec;
}
.message.assistant .message-body {
  background: rgba(167, 139, 250, 0.1);
  border: 1px solid rgba(167, 139, 250, 0.25);
  color: #e8eaec;
}
.message.typing .message-body {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 14px;
}
.message.typing .dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  background: #a78bfa;
  animation: blink 1.2s infinite;
}
.message.typing .dot:nth-child(2) { animation-delay: 0.2s; }
.message.typing .dot:nth-child(3) { animation-delay: 0.4s; }
@keyframes blink {
  0%, 80%, 100% { opacity: 0.25; }
  40% { opacity: 1; }
}

.error-banner {
  display: flex; align-items: center; gap: 10px;
  padding: 10px 16px;
  background: rgba(240, 84, 84, 0.12);
  border-top: 1px solid rgba(240, 84, 84, 0.3);
  color: #f7c1c1;
  font-size: 12.5px;
}
.error-icon {
  width: 18px; height: 18px;
  display: inline-flex; align-items: center; justify-content: center;
  border-radius: 50%;
  background: rgba(240, 84, 84, 0.4);
  color: #fff;
  font-weight: 700;
  font-size: 11px;
  flex-shrink: 0;
}
.error-text { flex: 1; }
.error-close {
  background: none; border: none; color: #c9d1d9;
  font-size: 16px; cursor: pointer; line-height: 1;
}

/* Context chip — when another view (Config Audit finding, network policy, etc.)
   has set a pending context, this sits just above the composer so the user
   sees what scope Argus has. Click × to detach. */
.context-chip {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  background: rgba(167, 139, 250, 0.08);
  border-top: 1px solid rgba(167, 139, 250, 0.25);
  font-size: 11.5px;
  color: #c4b3fd;
}
.context-chip-kind {
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 600;
  font-size: 10px;
  padding: 2px 6px;
  background: rgba(167, 139, 250, 0.18);
  border-radius: 3px;
}
.context-chip-label {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: #e8eaec;
}
.context-chip-close {
  background: none;
  border: none;
  color: #b0b4ba;
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  padding: 0 4px;
  border-radius: 3px;
}
.context-chip-close:hover { background: rgba(255, 255, 255, 0.08); color: #fff; }

.composer {
  display: flex;
  gap: 10px;
  padding: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  background: #141517;
  flex-shrink: 0;
}
.composer-input {
  flex: 1;
  resize: vertical;
  min-height: 44px;
  max-height: 200px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: #0f1012;
  color: #e8eaec;
  font-size: 13px;
  line-height: 1.5;
  font-family: var(--mono);
  outline: none;
}
.composer-input:focus { border-color: rgba(167, 139, 250, 0.5); }
.send-btn {
  background: rgba(167, 139, 250, 0.2);
  border: 1px solid rgba(167, 139, 250, 0.4);
  color: #a78bfa;
  border-radius: 8px;
  padding: 0 18px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s;
  align-self: stretch;
}
.send-btn:hover { background: rgba(167, 139, 250, 0.3); color: #fff; }
.send-btn:disabled { opacity: 0.4; cursor: not-allowed; }
</style>
