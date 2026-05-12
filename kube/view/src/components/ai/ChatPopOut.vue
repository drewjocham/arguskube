<script setup>
import { onMounted, onUnmounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useUIPrefsStore } from '../../stores/uiPrefs'
import { useArgusContextStore } from '../../stores/argusContext'
import ArgusAIChat from './ArgusAIChat.vue'

const uiPrefs = useUIPrefsStore()
const argusContext = useArgusContextStore()
const { investigating, investigatingLabel } = storeToRefs(argusContext)

function close() {
  uiPrefs.closeChatPopOut()
}

function onKeydown(e) {
  if (e.key === 'Escape') close()
}

onMounted(() => {
  document.addEventListener('keydown', onKeydown)
})
onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div class="chat-popout-overlay" @click.self="close">
    <div class="chat-popout-window">
      <div class="chat-popout-titlebar" :class="{ investigating }">
        <div class="chat-popout-title">
          <span class="chat-popout-icon" :class="{ pulsing: investigating }">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="9"/>
              <path d="M9 11h.01"/>
              <path d="M15 11h.01"/>
              <path d="M9 15c1 1 2 1.5 3 1.5s2-.5 3-1.5"/>
            </svg>
          </span>
          Argus AI
          <span v-if="investigating" class="chat-popout-status" :title="investigatingLabel">
            · investigating<span v-if="investigatingLabel"> {{ investigatingLabel }}</span>
          </span>
        </div>
        <button class="chat-popout-close" @click="close" title="Close (Esc)">
          <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
            <path d="M7.116 8l-4.558 4.558.884.884L8 8.884l4.558 4.558.884-.884L8.884 8l4.558-4.558-.884-.884L8 7.116 3.442 2.558l-.884.884L7.116 8z"/>
          </svg>
        </button>
      </div>
      <div class="chat-popout-body">
        <ArgusAIChat />
      </div>
    </div>
  </div>
</template>

<style scoped>
.chat-popout-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(6px);
  z-index: 1200;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  animation: fade-in 0.15s ease-out;
}
@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }

.chat-popout-window {
  background: #16171a;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  box-shadow: 0 30px 80px rgba(0, 0, 0, 0.6);
  width: min(960px, 100%);
  height: min(720px, 100%);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.chat-popout-titlebar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  background: #1a1b1e;
  flex-shrink: 0;
}
.chat-popout-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: #e8eaec;
}
.chat-popout-title svg { color: #a78bfa; }
.chat-popout-icon { display: inline-flex; align-items: center; line-height: 0; }
/* Subtle ring pulse when the agent is investigating — visible enough that
   the user sees activity, soft enough that it doesn't compete with the
   composer once a reply is being typed. Animation stops when the
   investigating flag clears. */
.chat-popout-icon.pulsing {
  border-radius: 50%;
  animation: chat-popout-ring 1.6s ease-in-out infinite;
}
.chat-popout-titlebar.investigating {
  border-bottom-color: rgba(167, 139, 250, 0.35);
}
.chat-popout-status {
  font-weight: 400;
  font-size: 11.5px;
  color: #c4b3fd;
  font-family: var(--mono);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 320px;
}
@keyframes chat-popout-ring {
  0%, 100% { box-shadow: 0 0 0 0 rgba(167, 139, 250, 0); }
  50%      { box-shadow: 0 0 0 4px rgba(167, 139, 250, 0.22); }
}

.chat-popout-close {
  background: none;
  border: 1px solid transparent;
  color: #8b8f96;
  padding: 4px 6px;
  border-radius: 4px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  transition: all 0.15s;
}
.chat-popout-close:hover { color: #fff; background: rgba(255, 255, 255, 0.06); border-color: rgba(255, 255, 255, 0.08); }

.chat-popout-body {
  flex: 1;
  min-height: 0;
  display: flex;
}
.chat-popout-body :deep(.argus-ai-view) { flex: 1; }
</style>
