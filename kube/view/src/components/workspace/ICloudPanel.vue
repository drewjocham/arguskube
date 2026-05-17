<script setup>
// ICloudPanel — iCloud integration: notes, reminders, calendar.
// Notes/Reminders use macOS CLI tools (memo, remindctl); Calendar uses
// CalDAV. Credentials are app-specific passwords, stored encrypted.

import { computed, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'

const store = useWorkspaceStore()
const {
  icloudConnections, icloudLoading, icloudError, icloudStatus,
  icloudNotes, icloudReminders,
} = storeToRefs(store)

const appleID = ref('')
const appPassword = ref('')
const activeTab = ref('notes')

const connected = computed(() => icloudConnections.value.length > 0)

const canConnect = computed(
  () => !icloudLoading.value && appleID.value.trim() && appPassword.value.trim(),
)

async function doConnect() {
  if (!canConnect.value) return
  await store.connectICloud(appleID.value, appPassword.value)
  appPassword.value = ''
}
</script>

<template>
  <div class="panel icloud-panel">
    <h3>iCloud</h3>

    <div v-if="icloudError" class="status error">{{ icloudError }}</div>
    <div v-if="icloudStatus" class="status ok">{{ icloudStatus.op }}</div>

    <!-- Connection form (shown when no connections) -->
    <div v-if="!connected" class="card">
      <h4>Connect iCloud</h4>
      <p class="muted">
        You need an <strong>app-specific password</strong> for Argus.
        Generate one at
        <a href="https://appleid.apple.com" target="_blank">appleid.apple.com</a>
        → Sign-In and Security → App-Specific Passwords.
      </p>
      <div class="form-row">
        <input
          v-model="appleID"
          placeholder="Apple ID (email)"
          class="input"
          type="email"
          autocomplete="username"
        />
      </div>
      <div class="form-row">
        <input
          v-model="appPassword"
          placeholder="App-specific password"
          class="input"
          type="password"
          autocomplete="off"
        />
      </div>
      <button :disabled="!canConnect" @click="doConnect" class="btn primary">
        {{ icloudLoading ? 'Connecting…' : 'Connect' }}
      </button>
    </div>

    <!-- Connected view -->
    <div v-if="connected" class="card">
      <div class="connected-info">
        <strong>Connected as:</strong> {{ icloudConnections[0]?.display_name }}
      </div>

      <!-- Tab bar for capability views -->
      <nav class="sub-tabs">
        <button
          v-for="t in ['notes', 'reminders', 'calendar']"
          :key="t"
          class="sub-tab"
          :class="{ active: activeTab === t }"
          @click="activeTab = t"
        >{{ t.charAt(0).toUpperCase() + t.slice(1) }}</button>
      </nav>

      <!-- Notes (macOS CLI bridge) -->
      <div v-if="activeTab === 'notes'" class="tab-content">
        <h4>Notes</h4>
        <p class="muted">
          Notes are accessed via the <code>memo</code> CLI on macOS.
          Install the apple-notes Hermes skill for CLI access, or use
          the Notes app directly.
        </p>
        <div v-if="icloudNotes.length">
          <div v-for="n in icloudNotes" :key="n.id" class="item-row">
            <strong>{{ n.title }}</strong>
          </div>
        </div>
        <div v-else class="muted">No notes loaded. Run "memo list" in Terminal to verify setup.</div>
      </div>

      <!-- Reminders (macOS CLI bridge) -->
      <div v-if="activeTab === 'reminders'" class="tab-content">
        <h4>Reminders</h4>
        <p class="muted">
          Reminders are accessed via the <code>remindctl</code> CLI on macOS.
          Install the apple-reminders Hermes skill for CLI access.
        </p>
        <div v-if="icloudReminders.length">
          <div v-for="r in icloudReminders" :key="r.id" class="item-row">
            <strong>{{ r.title }}</strong>
          </div>
        </div>
        <div v-else class="muted">No reminders loaded. Run "remindctl list" in Terminal to verify setup.</div>
      </div>

      <!-- Calendar (CalDAV stub) -->
      <div v-if="activeTab === 'calendar'" class="tab-content">
        <h4>Calendar</h4>
        <p class="muted">
          iCloud Calendar via CalDAV is in progress. For now, use the
          native Calendar app on macOS or the Google Calendar integration
          for cloud calendar access.
        </p>
      </div>
    </div>
  </div>
</template>

<style scoped>
.icloud-panel { padding: 12px; }
.card { margin-bottom: 12px; padding: 12px; border: 1px solid var(--border); border-radius: 6px; }
.card h4 { margin: 0 0 8px; }
.form-row { margin-bottom: 8px; }
.input { width: 100%; padding: 6px 8px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg); color: var(--fg); }
.btn { padding: 6px 12px; border: 1px solid var(--border); border-radius: 4px; cursor: pointer; background: var(--bg); color: var(--fg); margin-top: 8px; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn.primary { background: var(--accent); color: white; border-color: var(--accent); }
.status { padding: 6px 12px; margin-bottom: 8px; border-radius: 4px; font-size: 0.9em; }
.status.error { background: var(--danger-bg); color: var(--danger); }
.status.ok { background: var(--ok-bg); color: var(--ok); }
.muted { color: var(--muted); font-size: 0.9em; }
.connected-info { margin-bottom: 12px; }
.sub-tabs { display: flex; gap: 4px; margin-bottom: 12px; border-bottom: 1px solid var(--border); }
.sub-tab { padding: 6px 12px; border: none; background: none; cursor: pointer; color: var(--muted); font-size: 0.9em; }
.sub-tab.active { color: var(--accent); border-bottom: 2px solid var(--accent); font-weight: 600; }
.tab-content { padding: 8px 0; }
.item-row { padding: 4px 0; }
</style>
