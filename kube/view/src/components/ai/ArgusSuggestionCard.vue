<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useUserProfile } from '../../composables/useUserProfile'
import { useAppNavStore } from '../../stores/appNav'

// ArgusSuggestionCard — the §6 user-visible surface. Pinned to the
// right panel header so it never obscures content. At most ONE card
// renders at any time; that's enforced by the suggester's daily-budget
// cap on the backend, AND by the local single-state `current` ref.
//
// Lifecycle:
//   - On mount and whenever the active view changes, poll the backend
//     for a suggestion.
//   - When a suggestion arrives, render it and start a 60s expiry.
//     The expiry is the user's "if you don't care, it goes away" promise.
//   - The card itself surfaces three actions: Accept (perform the
//     suggestion action), Mute (don't ask again), and a passive
//     dismiss-on-X (just close this one card).

const props = defineProps({
  activeView: { type: String, default: '' },
})
const emit = defineEmits(['navigate'])

const profile = useUserProfile()
const appNav = useAppNavStore()
const current = ref(null)
const expiringIn = ref(0)

let expiryTimer = null
let countdownTimer = null
let pollTimer = null

function clearExpiry() {
  if (expiryTimer) { clearTimeout(expiryTimer); expiryTimer = null }
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
  expiringIn.value = 0
}

function startExpiry(seconds) {
  clearExpiry()
  expiringIn.value = seconds
  countdownTimer = setInterval(() => {
    expiringIn.value = Math.max(0, expiringIn.value - 1)
  }, 1000)
  expiryTimer = setTimeout(() => {
    // Auto-dismiss without recording it as a mute — the user just
    // didn't engage. We DO record it as a dismiss so we can spot
    // patterns of ignored suggestions later.
    if (current.value) {
      profile.dismiss(current.value.muteKey, current.value.kind)
    }
    current.value = null
    clearExpiry()
  }, seconds * 1000)
}

async function maybeFetch() {
  const sg = await profile.pollSuggestion(props.activeView || '')
  if (sg && (!current.value || current.value.muteKey !== sg.muteKey)) {
    current.value = sg
    startExpiry(Math.max(15, Number(sg.expiresInS) || 60))
  } else if (!sg && current.value) {
    // Backend says nothing relevant any more (e.g. user navigated to
    // the suggested view themselves) — close the card cleanly.
    clearExpiry()
    current.value = null
  }
}

function onAccept() {
  if (!current.value) return
  const sg = current.value
  profile.accept(sg.muteKey, sg.kind)

  // Handle "open view X" suggestions.
  const m = (sg.actionId || '').match(/^userprofile\.open-view:(.+)$/)
  if (m) {
    appNav.requestNav({ navId: m[1] })
    emit('navigate', m[1])
    current.value = null
    clearExpiry()
    return
  }

  // Handle profile-related suggestions — open settings at the profiles section.
  const pm = (sg.actionId || '').match(/^profiles\.suggest:(.+)$/)
  if (pm) {
    appNav.requestNav({ navId: 'settings', anchor: 'profile-groups' })
    current.value = null
    clearExpiry()
    return
  }

  current.value = null
  clearExpiry()
}

function onMute() {
  if (!current.value) return
  profile.mute(current.value.muteKey)
  current.value = null
  clearExpiry()
}

function onClose() {
  if (!current.value) return
  profile.dismiss(current.value.muteKey, current.value.kind)
  current.value = null
  clearExpiry()
}

// Re-poll on view change. We deliberately fetch on every change rather
// than on a long-running timer to keep the surface tightly aligned with
// what the user is doing.
watch(() => props.activeView, () => { maybeFetch() })

onMounted(() => {
  maybeFetch()
  // Light idle poll: if nothing has happened for 10 minutes we ask
  // once more in case the suggester has new pattern data. We do NOT
  // poll aggressively — the backend's daily budget is the authority,
  // we just give it a chance to surface a different candidate.
  pollTimer = setInterval(() => {
    if (!current.value) maybeFetch()
  }, 10 * 60 * 1000)
})

onUnmounted(() => {
  clearExpiry()
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
})
</script>

<template>
  <div
    v-if="current"
    class="argus-suggestion"
    :data-kind="current.kind"
    data-testid="argus-suggestion-card"
    role="status"
    aria-live="polite"
  >
    <div class="suggestion-header">
      <span class="suggestion-label">Argus suggests</span>
      <button
        class="suggestion-close"
        :aria-label="'Dismiss suggestion'"
        data-testid="suggestion-close"
        @click="onClose"
      >×</button>
    </div>
    <div class="suggestion-title">{{ current.title }}</div>
    <div v-if="current.body" class="suggestion-body">{{ current.body }}</div>
    <div class="suggestion-actions">
      <button
        v-if="current.actionLabel"
        class="suggestion-accept"
        data-testid="suggestion-accept"
        @click="onAccept"
      >{{ current.actionLabel }}</button>
      <button
        class="suggestion-mute"
        data-testid="suggestion-mute"
        @click="onMute"
      >Don't ask again</button>
      <span v-if="expiringIn > 0" class="suggestion-expiry" aria-hidden="true">
        {{ expiringIn }}s
      </span>
    </div>
  </div>
</template>

<style scoped>
.argus-suggestion {
  margin: 6px;
  padding: 10px 12px;
  background: var(--bg2, #1a1a1a);
  border: 1px solid var(--accent, #4a9eff);
  border-radius: 6px;
  color: var(--text, #e5e5e5);
}
.suggestion-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.suggestion-label {
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--accent, #4a9eff);
}
.suggestion-close {
  background: none;
  border: none;
  color: var(--text3, #5a5a5a);
  cursor: pointer;
  font-size: 16px;
  line-height: 1;
  padding: 0 4px;
}
.suggestion-close:hover { color: var(--text, #e5e5e5); }
.suggestion-title {
  font-size: 13px;
  font-weight: 600;
  margin-bottom: 4px;
}
.suggestion-body {
  font-size: 12px;
  color: var(--text2, #b0b0b0);
  margin-bottom: 8px;
}
.suggestion-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.suggestion-accept {
  background: var(--accent, #4a9eff);
  color: #fff;
  border: none;
  border-radius: 4px;
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
}
.suggestion-accept:hover { filter: brightness(1.1); }
.suggestion-mute {
  background: none;
  border: 1px solid var(--border, #2a2a2a);
  color: var(--text2, #b0b0b0);
  border-radius: 4px;
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
}
.suggestion-mute:hover {
  border-color: var(--text3, #5a5a5a);
  color: var(--text, #e5e5e5);
}
.suggestion-expiry {
  margin-left: auto;
  font-size: 10px;
  color: var(--text3, #5a5a5a);
}
</style>
