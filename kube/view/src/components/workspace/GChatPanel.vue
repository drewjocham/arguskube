<script setup>
// Google Chat panel — outbound messaging into a Chat space. Shares
// the `service=google` connection with Docs/Sheets/Tasks; the user
// connects Google once via the Connections panel.
//
// Existing pre-Phase-3 Google connections won't have the chat scopes;
// the panel surfaces a "reconnect to grant Chat permissions" hint
// when ListChannels returns a 403/permission error.

import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'
import { invalidateCache } from '../../composables/useBridge'
import Select from '../common/Select.vue'
import GoogleAccountHeader from './GoogleAccountHeader.vue'

const emit = defineEmits(['switch-tab'])

const store = useWorkspaceStore()
const { googleConnections, gchatSpaces, gchatLoading, gchatSendError, gchatSendStatus } = storeToRefs(store)

const activeID = ref('')
const selectedSpace = ref('')
const draftText = ref('')

// History strip — last 5 sends in this session. Not persisted.
const history = ref([])

const spaces = computed(() => gchatSpaces.value?.[activeID.value] || [])
const spaceOptions = computed(() =>
  spaces.value.map((s) => ({ value: s.id, label: s.name })),
)
const charCount = computed(() => draftText.value.length)
const overLimit = computed(() => charCount.value > 4096)
const hasActiveConnection = computed(() => !!activeID.value && googleConnections.value.some((c) => c.id === activeID.value))
const canSend = computed(
  () => !gchatLoading.value && hasActiveConnection.value && !!selectedSpace.value && draftText.value.trim().length > 0 && !overLimit.value,
)

// "permission denied" likely means the pre-Phase-3 grant doesn't have
// the chat.spaces.readonly scope. We surface a specific message rather
// than the raw API error.
const needsReconnect = computed(() => {
  const e = gchatSendError.value || ''
  return /permission denied|insufficient|scope/i.test(e)
})

async function refreshSpaces() {
  if (!activeID.value) return
  invalidateCache('ListGoogleChatSpaces')
  await store.loadGChatSpaces(activeID.value)
}

async function send() {
  if (!canSend.value) return
  try {
    await store.sendGChatMessage(activeID.value, selectedSpace.value, draftText.value)
    history.value = [
      { spaceID: selectedSpace.value, text: draftText.value, at: Date.now() },
      ...history.value,
    ].slice(0, 5)
    draftText.value = ''
  } catch {
    /* surfaced via gchatSendError */
  }
}

function spaceName(id) {
  return spaces.value.find((s) => s.id === id)?.name || id
}

watch(activeID, (id) => {
  if (id) store.loadGChatSpaces(id)
  selectedSpace.value = ''
}, { immediate: false })

onMounted(() => {
  if (googleConnections.value.length === 1) {
    activeID.value = googleConnections.value[0].id
  }
  if (activeID.value) store.loadGChatSpaces(activeID.value)
})

onBeforeUnmount(() => store.clearGChatSendStatus())
</script>

<template>
  <div class="gchat-panel">
    <GoogleAccountHeader v-model="activeID" @switch-tab="emit('switch-tab', $event)" />

    <div v-if="hasActiveConnection" class="composer-wrap">
      <div v-if="needsReconnect" class="warn-banner" data-testid="gchat-reconnect-hint">
        Your Google connection was made before Chat scopes were added.
        Disconnect and reconnect Google to grant Chat permissions.
      </div>

      <div class="row">
        <label class="field-label" for="gchat-space-select-wrap">Space</label>
        <div id="gchat-space-select-wrap" class="grow">
          <Select
            v-model="selectedSpace"
            :options="spaceOptions"
            :disabled="gchatLoading"
            width="100%"
            testid="gchat-space-select"
            aria-label="Space"
          />
        </div>
        <button class="btn-ghost" :disabled="gchatLoading" @click="refreshSpaces" data-testid="gchat-refresh">
          Refresh
        </button>
      </div>

      <textarea
        v-model="draftText"
        class="composer"
        rows="8"
        placeholder="Type a message…"
        data-testid="gchat-composer"
      />
      <div class="counter" :class="{ over: overLimit }">
        {{ charCount.toLocaleString() }} / 4,096
      </div>

      <div class="row footer">
        <button class="btn-primary" :disabled="!canSend" @click="send" data-testid="gchat-send">
          Send
        </button>
        <span v-if="gchatSendStatus" class="status-ok">Sent ✓</span>
        <span v-if="gchatSendError && !needsReconnect" class="status-err">{{ gchatSendError }}</span>
      </div>

      <div v-if="history.length" class="history">
        <div class="history-label">Recent (this session)</div>
        <ul>
          <li v-for="(h, i) in history" :key="i">
            <span class="h-chan">{{ spaceName(h.spaceID) }}</span>
            <span class="h-text">{{ h.text.length > 60 ? h.text.slice(0, 60) + '…' : h.text }}</span>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<style scoped>
.gchat-panel { padding: 1rem; display: flex; flex-direction: column; gap: 0.75rem; }
.composer-wrap { display: flex; flex-direction: column; gap: 0.5rem; }
.row { display: flex; gap: 0.5rem; align-items: center; }
.row .grow { flex: 1; }
.row.footer { margin-top: 0.5rem; }
.field-label { font-size: 12px; color: var(--text2); white-space: nowrap; }
.composer { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; color: var(--text); padding: 8px 10px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; resize: vertical; }
.composer:focus { outline: 2px solid var(--accent, #4f8cff); outline-offset: 1px; }
.counter { font-size: 11px; color: var(--text3); align-self: flex-end; }
.counter.over { color: #ef4444; font-weight: 600; }
.btn-primary { background: var(--accent, #2563eb); color: #fff; border: none; padding: 0.45rem 0.9rem; border-radius: 4px; cursor: pointer; font-size: 13px; font-weight: 600; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-ghost { background: transparent; border: 1px solid var(--border); padding: 0.3rem 0.6rem; border-radius: 4px; cursor: pointer; color: var(--text); font-size: 12px; }
.btn-ghost:disabled { opacity: 0.5; cursor: not-allowed; }
.status-ok { color: #10b981; font-size: 12px; }
.status-err { color: #ef4444; font-size: 12px; }
.warn-banner { background: rgba(251, 191, 36, 0.1); border: 1px solid #f59e0b; color: #fde68a; padding: 0.5rem 0.75rem; border-radius: 4px; font-size: 12px; line-height: 1.4; }
.history { margin-top: 0.75rem; padding-top: 0.5rem; border-top: 1px solid var(--border); }
.history-label { font-size: 11px; color: var(--text3); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 0.25rem; }
.history ul { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 0.2rem; }
.history li { display: flex; gap: 0.5rem; font-size: 12px; align-items: baseline; }
.h-chan { color: var(--text2); flex: 0 0 auto; }
.h-text { color: var(--text); opacity: 0.85; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
