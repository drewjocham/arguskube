<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useChat } from '../../composables/useWails'

const GLOBAL_ALERT_ID = 'global'

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
  refreshHistory(GLOBAL_ALERT_ID)
})

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
  try {
    await sendMessage(GLOBAL_ALERT_ID, val)
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
            <div class="message-body">{{ msg.content }}</div>
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
.header-text { display: flex; flex-direction: column; gap: 2px; }
.title { font-size: 20px; font-weight: 500; color: #fff; }
.subtitle { font-size: 13px; color: #8b8f96; max-width: 720px; }

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
  white-space: pre-wrap;
  word-break: break-word;
}
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
