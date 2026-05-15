<script setup>
// SlackEventsPanel — read-only stream of inbound Slack events +
// slash-command invocations. Polls ListSlackEvents every 3 seconds
// while the tab is active. Empty buffer renders the setup checklist
// (signing secret + public URL + Slack-app event subscriptions).
//
// SaaS-only: when the backend has no signing secret wired in, the
// Wails call returns an empty slice and we surface the setup hint.

import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { callGo } from '../../composables/useBridge'

const events = ref([])
const error = ref(null)
const loading = ref(false)
const expanded = ref(new Set())
let pollTimer = null

const SETUP_HINTS = [
  'Set ARGUS_SLACK_SIGNING_SECRET to your Slack app\'s "Signing Secret".',
  'Expose Argus on a public URL via ARGUS_AUTH_BASE_URL.',
  'In the Slack app config, set Event Subscriptions → Request URL to <base>/workspace/slack/events.',
  'For slash commands, set the command\'s Request URL to <base>/workspace/slack/commands. Built-in: /argus-ping.',
]

function sessionToken() {
  try {
    const raw = localStorage.getItem('argus.auth.session')
    if (!raw) return ''
    const parsed = JSON.parse(raw)
    return parsed?.token || ''
  } catch {
    return ''
  }
}

async function refresh() {
  loading.value = true
  try {
    const out = await callGo('ListSlackEvents', sessionToken())
    events.value = Array.isArray(out) ? out : []
    error.value = null
  } catch (e) {
    error.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}

function toggle(idx) {
  const s = new Set(expanded.value)
  if (s.has(idx)) s.delete(idx)
  else s.add(idx)
  expanded.value = s
}

function fmtTime(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const isEmpty = computed(() => !loading.value && events.value.length === 0)

onMounted(() => {
  refresh()
  pollTimer = setInterval(refresh, 3000)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <div class="events-panel">
    <header class="header">
      <h2>Slack — Inbound events</h2>
      <button class="btn-ghost" :disabled="loading" @click="refresh">Refresh</button>
    </header>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div v-if="isEmpty" class="empty">
      <p>No inbound events yet.</p>
      <p class="hint">
        Inbound events require a SaaS-mode deployment with a public webhook URL.
        Once configured, this view shows the last 50 events + slash-command invocations.
      </p>
      <ol class="setup">
        <li v-for="(h, i) in SETUP_HINTS" :key="i">{{ h }}</li>
      </ol>
    </div>

    <ul v-else class="events" data-testid="slack-events-list">
      <li v-for="(e, i) in events" :key="i" :class="['event', e.kind]">
        <div class="row" @click="toggle(i)">
          <span class="ts">{{ fmtTime(e.received_at) }}</span>
          <span :class="['kind-pill', e.kind]">{{ e.kind }}</span>
          <span class="subtype">{{ e.subtype || '—' }}</span>
          <span v-if="e.channel" class="meta">#{{ e.channel }}</span>
          <span v-if="e.user_id" class="meta">@{{ e.user_id }}</span>
          <span class="text">{{ e.text }}</span>
        </div>
        <pre v-if="expanded.has(i)" class="raw">{{ JSON.stringify(e.raw, null, 2) }}</pre>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.events-panel { padding: 1rem; display: flex; flex-direction: column; gap: 0.75rem; }
.header { display: flex; justify-content: space-between; align-items: center; }
.header h2 { margin: 0; font-size: 1.1rem; }
.btn-ghost { background: transparent; border: 1px solid var(--border); padding: 0.3rem 0.6rem; border-radius: 4px; color: var(--text); cursor: pointer; font-size: 12px; }
.btn-ghost:disabled { opacity: 0.5; }
.error-banner { background: rgba(239, 68, 68, 0.1); border: 1px solid #ef4444; color: #fca5a5; padding: 0.5rem; border-radius: 4px; font-size: 12px; }
.empty { padding: 1.5rem; border: 1px dashed var(--border); border-radius: 6px; color: var(--text2); font-size: 13px; }
.empty p { margin: 0 0 0.5rem; }
.empty .hint { opacity: 0.7; font-size: 12px; }
.empty .setup { font-size: 12px; margin: 0.5rem 0 0; padding-left: 1.2rem; }
.empty .setup li { margin: 0.2rem 0; }
.events { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 0.25rem; max-height: 60vh; overflow-y: auto; }
.event { background: var(--bg3); border: 1px solid var(--border); border-radius: 4px; padding: 0.4rem 0.6rem; font-size: 12px; }
.row { display: flex; gap: 0.5rem; align-items: baseline; cursor: pointer; }
.ts { color: var(--text3); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.kind-pill { font-size: 10px; text-transform: uppercase; padding: 0.05rem 0.4rem; border-radius: 8px; background: rgba(79, 140, 255, 0.15); color: var(--accent, #4f8cff); }
.kind-pill.slash_command { background: rgba(16, 185, 129, 0.15); color: #10b981; }
.subtype { color: var(--text2); font-style: italic; }
.meta { color: var(--text3); }
.text { color: var(--text); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.raw { background: var(--bg2); border-radius: 4px; padding: 0.5rem; margin: 0.4rem 0 0; font-size: 11px; color: var(--text); white-space: pre-wrap; word-break: break-all; max-height: 240px; overflow-y: auto; }
</style>
