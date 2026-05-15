<script setup>
// SlackPanel — Phase 1B outbound surface. Picks a Slack workspace
// (when the user has multiple), lists channels for it, and posts a
// chat.postMessage via the workspace store. Everything stays in
// memory — there's no thread history, no draft persistence — because
// the goal here is "send a message" not "be a Slack client".

import { computed, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'
import { invalidateCache } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const emit = defineEmits(['switch-tab'])

const store = useWorkspaceStore()
const {
  slackChannels,
  slackLoading,
  slackSendError,
  slackSendStatus,
  slackConnections,
} = storeToRefs(store)

// Active connection — defaults to the first Slack connection. Held
// separately from the store so the user can flip without the rest of
// the app caring.
const activeConnectionID = ref(null)
const selectedChannelID = ref(null)
const messageText = ref('')
// Last 5 sends, newest first. Local-only — wipes on unmount by design.
const recentSends = ref([])

const MAX_LEN = 40000

const connectionOptions = computed(() =>
  slackConnections.value.map((c) => ({
    value: c.id,
    label: c.display_name || '(unnamed workspace)',
  })),
)

const channelsForActive = computed(() => {
  const id = activeConnectionID.value
  if (!id) return []
  return slackChannels.value[id] || []
})

const channelOptions = computed(() =>
  channelsForActive.value.map((ch) => ({
    value: ch.id,
    label: `#${ch.name}`,
  })),
)

const charCount = computed(() => messageText.value.length)
const tooLong = computed(() => charCount.value > MAX_LEN)

const sendDisabled = computed(
  () =>
    slackLoading.value ||
    !activeConnectionID.value ||
    !selectedChannelID.value ||
    !messageText.value.trim() ||
    tooLong.value,
)

const activeConnection = computed(() =>
  slackConnections.value.find((c) => c.id === activeConnectionID.value) || null,
)

onMounted(async () => {
  // Need services + connections to render; if Phase 1A loaded them
  // already these are cheap no-ops thanks to the cache.
  await store.loadServices()
  if (!slackConnections.value.length) await store.loadConnections()
  if (!activeConnectionID.value && slackConnections.value.length) {
    activeConnectionID.value = slackConnections.value[0].id
  }
})

onBeforeUnmount(() => {
  recentSends.value = []
})

// Re-fetch channels whenever the active workspace changes. The cache
// keeps a fast switcheroo from spamming the API.
watch(
  activeConnectionID,
  async (id) => {
    selectedChannelID.value = null
    if (id) await store.loadSlackChannels(id)
  },
  { immediate: true },
)

// If connections load after mount (e.g. user opened the panel before
// the network call returned), default to the first available one.
watch(slackConnections, (conns) => {
  if (!activeConnectionID.value && conns.length) {
    activeConnectionID.value = conns[0].id
  }
})

function onMessageInput() {
  // Typing should clear stale error banners so the user isn't left
  // staring at a red box from a failed attempt 10 seconds ago.
  if (slackSendError.value) store.clearSlackSendStatus()
}

async function refreshChannels() {
  if (!activeConnectionID.value) return
  invalidateCache('ListSlackChannels')
  await store.loadSlackChannels(activeConnectionID.value)
}

async function onSend() {
  if (sendDisabled.value) return
  const connID = activeConnectionID.value
  const chID = selectedChannelID.value
  const text = messageText.value
  const channelLabel =
    channelsForActive.value.find((c) => c.id === chID)?.name || chID
  try {
    await store.sendSlackMessage(connID, chID, text)
    messageText.value = ''
    recentSends.value = [
      { at: Date.now(), channel: channelLabel, text },
      ...recentSends.value,
    ].slice(0, 5)
  } catch {
    /* surfaced via store.slackSendError */
  }
}

function truncate(s, n = 80) {
  return s.length > n ? `${s.slice(0, n)}…` : s
}
function fmtTime(ms) {
  const d = new Date(ms)
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
function avatarLetter(name) {
  return (name || 'S').trim().charAt(0).toUpperCase() || 'S'
}
</script>

<template>
  <div class="slack-panel">
    <!-- Empty state: no Slack connection at all. We could hide the tab
         entirely but it's friendlier to land the user here with a hint. -->
    <div v-if="!slackConnections.length" class="empty">
      <div class="empty-icon" aria-hidden="true">#</div>
      <h3>No Slack workspace connected</h3>
      <p>Link a workspace from the Connections tab to start sending messages.</p>
      <button class="btn-primary" @click="emit('switch-tab', 'connections')">
        Go to Connections
      </button>
    </div>

    <template v-else>
      <header class="head">
        <div class="ws-info">
          <div class="ws-avatar" :title="activeConnection?.display_name || ''">
            <img
              v-if="activeConnection?.avatar_url"
              :src="activeConnection.avatar_url"
              alt=""
            />
            <span v-else>{{ avatarLetter(activeConnection?.display_name) }}</span>
          </div>
          <div class="ws-meta">
            <div class="ws-name">{{ activeConnection?.display_name || 'Slack' }}</div>
            <div v-if="activeConnection?.external_workspace_id" class="ws-id">
              {{ activeConnection.external_workspace_id }}
            </div>
          </div>
        </div>
        <div v-if="slackConnections.length > 1" class="ws-picker">
          <label>Workspace</label>
          <Select
            v-model="activeConnectionID"
            :options="connectionOptions"
            size="sm"
            width="220px"
            aria-label="Slack workspace"
          />
        </div>
      </header>

      <section class="row">
        <label>Channel</label>
        <Select
          v-model="selectedChannelID"
          :options="channelOptions"
          :disabled="slackLoading || !channelOptions.length"
          :placeholder="slackLoading ? 'Loading channels…' : 'Pick a channel'"
          width="280px"
          aria-label="Slack channel"
          testid="slack-channel-select"
        />
        <button class="btn-ghost" :disabled="slackLoading" @click="refreshChannels">
          Refresh
        </button>
      </section>

      <section class="composer">
        <textarea
          v-model="messageText"
          rows="8"
          placeholder="Type a message…"
          spellcheck="true"
          @input="onMessageInput"
        />
        <div class="composer-foot">
          <span class="char-count" :class="{ over: tooLong }">
            {{ charCount.toLocaleString() }} / {{ MAX_LEN.toLocaleString() }}
          </span>
          <span class="grow" />
          <span v-if="slackSendStatus" class="sent-badge">Sent ✓</span>
          <button
            class="btn-primary send-btn"
            :disabled="sendDisabled"
            data-testid="slack-send"
            @click="onSend"
          >
            Send
          </button>
        </div>
        <div v-if="slackSendError" class="err-banner">{{ slackSendError }}</div>
      </section>

      <section v-if="recentSends.length" class="history">
        <div class="history-head">Recent sends</div>
        <ul>
          <li v-for="r in recentSends" :key="r.at">
            <span class="hist-time">{{ fmtTime(r.at) }}</span>
            <span class="hist-ch">#{{ r.channel }}</span>
            <span class="hist-text">{{ truncate(r.text) }}</span>
          </li>
        </ul>
      </section>
    </template>
  </div>
</template>

<style scoped>
.slack-panel { padding: 18px 22px; overflow-y: auto; flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 14px; }

.empty {
  margin: 60px auto;
  max-width: 420px;
  text-align: center;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 32px;
}
.empty-icon {
  width: 48px; height: 48px;
  border-radius: 10px;
  background: #4A154B; color: white;
  display: flex; align-items: center; justify-content: center;
  font-size: 22px; font-weight: 700;
  margin: 0 auto 14px;
}
.empty h3 { margin: 0 0 6px; font-size: 15px; color: var(--text); }
.empty p { margin: 0 0 16px; font-size: 12.5px; color: var(--text2); }

.head {
  display: flex; align-items: center; justify-content: space-between; gap: 12px;
  padding: 12px 14px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
}
.ws-info { display: flex; align-items: center; gap: 10px; min-width: 0; }
.ws-avatar {
  width: 36px; height: 36px; border-radius: 8px;
  background: #4A154B; color: white;
  display: flex; align-items: center; justify-content: center;
  font-weight: 700; overflow: hidden; flex-shrink: 0;
}
.ws-avatar img { width: 100%; height: 100%; object-fit: cover; }
.ws-meta { min-width: 0; }
.ws-name { font-size: 14px; font-weight: 600; color: var(--text); }
.ws-id { font-size: 11px; color: var(--text3); }
.ws-picker { display: flex; align-items: center; gap: 8px; }
.ws-picker label { font-size: 11.5px; color: var(--text2); }

.row {
  display: flex; align-items: center; gap: 10px;
  padding: 0 2px;
}
.row label { font-size: 12px; color: var(--text2); min-width: 64px; }

.composer { display: flex; flex-direction: column; gap: 8px; }
.composer textarea {
  width: 100%;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
  color: var(--text);
  padding: 10px 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  line-height: 1.5;
  resize: vertical;
  min-height: 140px;
}
.composer textarea:focus { outline: none; border-color: var(--accent); }

.composer-foot {
  display: flex; align-items: center; gap: 10px;
}
.char-count { font-size: 11px; color: var(--text3); font-variant-numeric: tabular-nums; }
.char-count.over { color: #dc5050; font-weight: 600; }
.grow { flex: 1; }
.sent-badge {
  font-size: 11.5px; font-weight: 600;
  color: #4ade80;
  background: rgba(74, 222, 128, 0.10);
  padding: 3px 8px; border-radius: 999px;
  animation: fade-in 0.15s ease-out;
}

.btn-primary {
  padding: 7px 16px;
  border-radius: 6px;
  border: 1px solid var(--border2);
  background: var(--accent);
  color: white;
  font-size: 12.5px; font-weight: 500;
  cursor: pointer;
  transition: opacity 0.1s;
}
.btn-primary:hover:not(:disabled) { opacity: 0.88; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.btn-ghost {
  padding: 5px 12px;
  border-radius: 6px;
  border: 1px solid var(--border2);
  background: transparent;
  color: var(--text2);
  font-size: 12px;
  cursor: pointer;
}
.btn-ghost:hover:not(:disabled) { background: var(--bg3); color: var(--text); }
.btn-ghost:disabled { opacity: 0.5; cursor: not-allowed; }

.err-banner {
  padding: 8px 12px;
  background: rgba(220, 80, 80, 0.10);
  border: 1px solid rgba(220, 80, 80, 0.35);
  border-radius: 6px;
  color: var(--text);
  font-size: 12px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.history {
  margin-top: 4px;
  padding: 10px 12px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
}
.history-head { font-size: 11px; color: var(--text3); margin-bottom: 6px; text-transform: uppercase; letter-spacing: 0.5px; }
.history ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 4px; }
.history li {
  display: flex; gap: 10px; align-items: baseline;
  font-size: 12px; color: var(--text2);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.hist-time { color: var(--text3); flex-shrink: 0; }
.hist-ch { color: var(--accent2, var(--accent)); flex-shrink: 0; }
.hist-text { color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }
</style>
