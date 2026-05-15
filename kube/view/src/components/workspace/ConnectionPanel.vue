<script setup>
// ConnectionPanel — Phase 1A workspace integrations view. Renders one
// tile per service the backend has a Provider for, and lets the user
// connect / disconnect via the OAuth popup flow owned by the store.
//
// Per-service control panels (channels, doc pickers, etc.) live in
// their own tabs in 1B+; this tile is intentionally minimal.

import { onMounted, ref, computed } from 'vue'
import { useWorkspaceStore } from '../../stores/workspace'

const store = useWorkspaceStore()
const connecting = ref({})  // service -> bool

// Service metadata is local — the backend's authoritative list is
// service *ids* only; labels/colors/icons are presentation concerns.
const SERVICE_META = {
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
      <p>No workspace integrations are wired in this build.</p>
      <p class="hint">Google Docs, Sheets, and Tasks adapters land in Phase 1B.</p>
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
            class="connect-btn"
            :disabled="connecting[svc]"
            @click="onConnect(svc)"
          >
            {{ connecting[svc] ? 'Connecting…' : (store.connectionsByService[svc]?.length ? 'Add another' : 'Connect') }}
          </button>
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
