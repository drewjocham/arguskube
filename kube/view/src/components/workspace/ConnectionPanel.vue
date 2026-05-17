<script setup>
// ConnectionPanel — Phase 1A workspace integrations view. Renders one
// tile per service the backend has a Provider for, and lets the user
// connect / disconnect via the OAuth popup flow owned by the store.
//
// Per-service control panels (channels, doc pickers, etc.) live in
// their own tabs in 1B+; this tile is intentionally minimal.
//
// Inline OAuth credential editor: each Google-family tile exposes an
// expander that shows the workspaceGoogle* values from settings via
// LockableField. Saved values render LOCKED with a placeholder, no
// raw value in the DOM, and require ticking "Unlock to edit" to be
// modified. The expander lives here (not in Settings) so the user
// can set up credentials WITHOUT navigating away from the page that
// gates on them — the previous flow was "see the Connect button is
// dead → click 'Open Settings' link → scroll to OAuth section → save
// → go back". That trip is the entire reason this expander exists.

import { onMounted, ref, computed } from 'vue'
import { callGo } from '../../composables/useBridge'
import { useWorkspaceStore } from '../../stores/workspace'
import { useAppNavStore } from '../../stores/appNav'
import LockableField from '../common/LockableField.vue'

const store = useWorkspaceStore()
const appNav = useAppNavStore()
const connecting = ref({})  // service -> bool

// OAuth-credential form state, loaded lazily from GetSettings the
// first time the user expands a tile. Holds ALL settings so a save
// can round-trip through UpdateSettings without nulling out unrelated
// fields. settingsLoaded gates the expander UI so the user doesn't
// see empty inputs before the backend round-trip completes.
const settingsForm = ref(null)
const settingsLoaded = ref(false)
const settingsLoadError = ref('')
const settingsSaving = ref(false)
const settingsSavedAt = ref(0)
// Tracks which tiles have their credentials expander open. Keyed by
// service so multiple tiles can be open simultaneously.
const credsExpanded = ref({})

async function loadSettingsOnce() {
  if (settingsLoaded.value) return
  try {
    const result = await callGo('GetSettings')
    settingsForm.value = result || {}
    settingsLoaded.value = true
  } catch (err) {
    settingsLoadError.value = err?.message || String(err)
  }
}

async function toggleCreds(service) {
  credsExpanded.value = { ...credsExpanded.value, [service]: !credsExpanded.value[service] }
  if (credsExpanded.value[service]) {
    await loadSettingsOnce()
  }
}

// saveCredentials sends the full settings object back so the backend
// keeps every other field intact. The user only edits OAuth values
// here; the rest of the form is whatever we loaded.
async function saveCredentials() {
  if (!settingsForm.value) return
  settingsSaving.value = true
  try {
    await callGo('UpdateSettings', settingsForm.value)
    settingsSavedAt.value = Date.now()
    // Re-load services because a new Google client ID could enable
    // a Provider that wasn't available before.
    await store.loadServices()
  } catch (err) {
    settingsLoadError.value = err?.message || String(err)
  } finally {
    settingsSaving.value = false
  }
}

// Per-service field map. Only google + slack have OAuth credentials
// the user can configure inline today; the other tiles fall back to
// the "open Settings" link.
const CRED_FIELDS = {
  google: [
    { key: 'workspaceGoogleClientId',     label: 'Google client ID' },
    { key: 'workspaceGoogleClientSecret', label: 'Google client secret' },
  ],
  gchat: [
    { key: 'workspaceGoogleClientId',     label: 'Google client ID' },
    { key: 'workspaceGoogleClientSecret', label: 'Google client secret' },
  ],
  gdocs:   null,  // share the google entry above; no separate tile creds
  gsheets: null,
  gtasks:  null,
  slack: [
    { key: 'slackClientId',     label: 'Slack client ID' },
    { key: 'slackClientSecret', label: 'Slack client secret' },
  ],
}
function credFieldsFor(service) {
  return CRED_FIELDS[service] || null
}

// Empty-state CTA: jump to the new "Sign-in & integrations" section in
// Settings so the user can paste OAuth client credentials without
// editing env vars + restarting the backend. The same anchor is what
// SettingsPanel's onMounted handler scrolls to.
function openSettings() {
  appNav.requestNav({ navId: 'settings', anchor: 'sign-in-integrations' })
}

// Service metadata is local — the backend's authoritative list is
// service *ids* only; labels/colors/icons are presentation concerns.
// Keep this in sync with kube/backend/internal/workspace/types.go —
// any Service constant the backend exposes via AvailableServices()
// should have a matching entry here, otherwise the tile is filtered
// out and the user sees the "no integrations wired" empty state.
const SERVICE_META = {
  slack:   { label: 'Slack',         color: '#4A154B', letter: 'S' },
  gchat:   { label: 'Google Chat',   color: '#0F9D58', letter: 'C' },
  google:  { label: 'Google Workspace', color: '#4285F4', letter: 'G' },
  gdocs:   { label: 'Google Docs',   color: '#4285F4', letter: 'D' },
  gsheets: { label: 'Google Sheets', color: '#0F9D58', letter: 'S' },
  gtasks:  { label: 'Google Tasks',  color: '#4285F4', letter: 'T' },
}

onMounted(async () => {
  await store.loadServices()
  await store.loadConnections()
})

const visibleServices = computed(() => store.services.filter(s => SERVICE_META[s]))

async function onConnect(service) {
  connecting.value = { ...connecting.value, [service]: true }
  try {
    await store.startConnect(service)
  } catch {
    /* error already surfaced via store.error */
  } finally {
    connecting.value = { ...connecting.value, [service]: false }
  }
}

async function onDisconnect(conn) {
  if (!window.confirm(`Disconnect ${conn.display_name || conn.service}? Tokens will be erased.`)) return
  await store.disconnect(conn.id)
}

function avatarStyle(meta) {
  return { background: meta.color }
}
</script>

<template>
  <div class="ws-panel">
    <header>
      <h2>Workspace integrations</h2>
      <p class="hint">
        Connect Argus to the tools you already use. OAuth tokens are stored
        encrypted on this machine, keyed off the same secret-store entry
        that protects your sign-in session — Argus never sends your tokens
        to a third party.
      </p>
    </header>

    <div v-if="store.error" class="error">{{ store.error }}</div>

    <div v-if="!visibleServices.length" class="empty">
      <p>No workspace integrations are available.</p>
      <p class="hint">
        Paste your Slack and Google OAuth client credentials in Settings to
        enable the Connect buttons — no restart needed.
      </p>
      <button class="open-settings-btn" type="button" @click="openSettings">
        Open sign-in &amp; integrations settings
      </button>
    </div>

    <ul v-else class="tiles">
      <li v-for="svc in visibleServices" :key="svc" class="tile">
        <div class="tile-head">
          <div class="avatar" :style="avatarStyle(SERVICE_META[svc])">
            {{ SERVICE_META[svc].letter }}
          </div>
          <div class="tile-meta">
            <div class="tile-title">{{ SERVICE_META[svc].label }}</div>
            <div class="tile-sub">
              <template v-if="store.connectionsByService[svc]?.length">
                {{ store.connectionsByService[svc].length }}
                workspace{{ store.connectionsByService[svc].length === 1 ? '' : 's' }} connected
              </template>
              <template v-else>Not connected</template>
            </div>
          </div>
          <button
            v-if="credFieldsFor(svc)"
            class="creds-btn"
            type="button"
            :aria-expanded="!!credsExpanded[svc]"
            :title="credsExpanded[svc] ? 'Hide OAuth credentials' : 'Configure OAuth credentials'"
            @click="toggleCreds(svc)"
          >{{ credsExpanded[svc] ? 'Hide credentials' : 'Credentials' }}</button>
          <button
            class="connect-btn"
            :disabled="connecting[svc]"
            @click="onConnect(svc)"
          >
            {{ connecting[svc] ? 'Connecting…' : (store.connectionsByService[svc]?.length ? 'Add another' : 'Connect') }}
          </button>
        </div>

        <!-- OAuth credential editor.
             Locked by default when a saved value comes back from
             GetSettings — LockableField renders the placeholder, NOT
             the raw value, so the secret never reaches the DOM until
             the user explicitly unlocks. -->
        <div v-if="credsExpanded[svc] && credFieldsFor(svc)" class="creds-editor">
          <div v-if="settingsLoadError" class="error" style="margin: 0 0 10px;">{{ settingsLoadError }}</div>
          <div v-else-if="!settingsLoaded" class="hint">Loading current credentials…</div>
          <template v-else>
            <p class="hint" style="margin: 0 0 8px;">
              Saved values are hidden until you unlock the field. The
              same values are also editable in
              <a href="#" @click.prevent="openSettings">Settings → Sign-in &amp; integrations</a>.
            </p>
            <div
              v-for="f in credFieldsFor(svc)"
              :key="f.key"
              class="cred-field"
            >
              <label class="cred-label">{{ f.label }}</label>
              <LockableField
                v-model="settingsForm[f.key]"
                input-class="creds-input"
              />
            </div>
            <div class="cred-actions">
              <button
                class="save-btn"
                type="button"
                :disabled="settingsSaving"
                @click="saveCredentials"
              >{{ settingsSaving ? 'Saving…' : 'Save credentials' }}</button>
              <span v-if="settingsSavedAt" class="save-ok">Saved ✓</span>
            </div>
          </template>
        </div>

        <ul v-if="store.connectionsByService[svc]?.length" class="conns">
          <li
            v-for="c in store.connectionsByService[svc]"
            :key="c.id"
            class="conn"
          >
            <img v-if="c.avatar_url" :src="c.avatar_url" alt="" class="conn-avatar" />
            <div v-else class="conn-avatar" :style="avatarStyle(SERVICE_META[svc])">
              {{ SERVICE_META[svc].letter }}
            </div>
            <div class="conn-meta">
              <div class="conn-name">{{ c.display_name || '(unnamed workspace)' }}</div>
              <div v-if="c.email" class="conn-email">{{ c.email }}</div>
            </div>
            <button class="disconnect-btn" @click="onDisconnect(c)">Disconnect</button>
          </li>
        </ul>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.ws-panel { padding: 18px 22px; overflow-y: auto; flex: 1; min-height: 0; }
header h2 { margin: 0 0 6px; font-size: 16px; font-weight: 600; color: var(--text); }
.hint { margin: 0; font-size: 12px; color: var(--text3); max-width: 640px; line-height: 1.45; }

.error {
  margin: 14px 0;
  padding: 10px 12px;
  background: rgba(220, 80, 80, 0.10);
  border: 1px solid rgba(220, 80, 80, 0.35);
  border-radius: 6px;
  color: var(--text);
  font-size: 12.5px;
}

.empty {
  margin-top: 36px;
  text-align: center;
  color: var(--text2);
  padding: 30px;
  border: 1px dashed var(--border);
  border-radius: 8px;
}
.empty p { margin: 4px 0; }
.open-settings-btn {
  margin-top: 14px;
  padding: 7px 16px;
  background: var(--accent);
  color: #fff;
  border: 0;
  border-radius: 6px;
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
}
.open-settings-btn:hover { filter: brightness(1.06); }

.tiles { list-style: none; padding: 0; margin: 18px 0 0; display: flex; flex-direction: column; gap: 12px; }

.tile {
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 14px;
}
.tile-head { display: flex; align-items: center; gap: 12px; }
.tile-meta { flex: 1; min-width: 0; }
.tile-title { font-size: 14px; font-weight: 600; color: var(--text); }
.tile-sub { font-size: 11.5px; color: var(--text3); margin-top: 2px; }

.avatar {
  width: 36px; height: 36px; border-radius: 8px;
  display: flex; align-items: center; justify-content: center;
  color: white; font-weight: 700; font-size: 15px;
}

.connect-btn {
  padding: 6px 14px;
  border-radius: 6px;
  border: 1px solid var(--border2);
  background: var(--accent);
  color: white;
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  transition: opacity 0.1s;
}
.connect-btn:hover:not(:disabled) { opacity: 0.85; }
.connect-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.creds-btn {
  padding: 6px 12px;
  border-radius: 6px;
  border: 1px solid var(--border2);
  background: transparent;
  color: var(--text2);
  font-size: 12px;
  cursor: pointer;
}
.creds-btn:hover { background: var(--bg3); color: var(--text); }
.creds-btn[aria-expanded="true"] {
  background: var(--bg3);
  color: var(--text);
  border-color: var(--accent);
}

.creds-editor {
  margin-top: 12px;
  padding: 12px;
  border: 1px dashed var(--border);
  border-radius: 6px;
  background: var(--bg3);
}
.cred-field {
  margin-bottom: 10px;
}
.cred-field:last-of-type { margin-bottom: 12px; }
.cred-label {
  display: block;
  font-size: 11.5px;
  color: var(--text3);
  margin-bottom: 4px;
}
:deep(.creds-input) {
  width: 100%;
  padding: 5px 8px;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--text);
  font-family: var(--mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 12px;
}
:deep(.creds-input:focus) {
  outline: none;
  border-color: var(--accent);
}
.cred-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}
.save-btn {
  padding: 5px 14px;
  border-radius: 5px;
  border: 1px solid var(--border2);
  background: var(--accent);
  color: white;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}
.save-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.save-ok { font-size: 11.5px; color: var(--green, #5ec76d); }

.conns {
  list-style: none;
  margin: 12px 0 0;
  padding: 10px 0 0;
  border-top: 1px solid var(--border);
  display: flex; flex-direction: column; gap: 8px;
}
.conn {
  display: flex; align-items: center; gap: 10px;
  padding: 6px 8px; border-radius: 6px;
  background: var(--bg3);
}
.conn-avatar {
  width: 28px; height: 28px; border-radius: 6px;
  display: flex; align-items: center; justify-content: center;
  color: white; font-weight: 600; font-size: 12px; flex-shrink: 0;
}
.conn-meta { flex: 1; min-width: 0; }
.conn-name { font-size: 13px; color: var(--text); }
.conn-email { font-size: 11px; color: var(--text3); }
.disconnect-btn {
  padding: 4px 10px;
  border-radius: 5px;
  border: 1px solid var(--border2);
  background: transparent;
  color: var(--text2);
  font-size: 11.5px;
  cursor: pointer;
}
.disconnect-btn:hover { background: var(--bg4); color: var(--text); border-color: rgba(220, 80, 80, 0.5); }
</style>
