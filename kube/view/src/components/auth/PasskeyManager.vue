<script setup>
// PasskeyManager — small management UI for the user's registered
// passkeys. Lives in a modal-ish overlay opened from the user menu
// dropdown (Titlebar.vue). Keeps its own loading/error state so it
// can be mounted ad-hoc without coupling to a parent.
import { ref, onMounted } from 'vue'
import { useAuthStore } from '../../stores/auth'

const auth = useAuthStore()
const emit = defineEmits(['close'])

const credentials = ref([])
const loading = ref(false)
const errorMsg = ref('')
const newName = ref('')
const busy = ref(false)

async function refresh() {
  loading.value = true
  errorMsg.value = ''
  try {
    credentials.value = await auth.listPasskeys()
  } catch (err) {
    errorMsg.value = err?.message || 'Failed to load passkeys'
  } finally {
    loading.value = false
  }
}

async function add() {
  errorMsg.value = ''
  busy.value = true
  try {
    await auth.registerPasskey(newName.value || '')
    newName.value = ''
    await refresh()
  } catch (err) {
    errorMsg.value = err?.message || 'Failed to register passkey'
  } finally {
    busy.value = false
  }
}

async function revoke(id) {
  errorMsg.value = ''
  try {
    await auth.revokePasskey(id)
    await refresh()
  } catch (err) {
    errorMsg.value = err?.message || 'Revoke failed'
  }
}

function fmtDate(unix) {
  if (!unix) return '—'
  try {
    return new Date(unix * 1000).toLocaleString()
  } catch {
    return '—'
  }
}

onMounted(refresh)
</script>

<template>
  <div class="pk-shell" role="dialog" aria-modal="true">
    <div class="pk-card">
      <header class="pk-head">
        <h2>Passkeys</h2>
        <button class="pk-close" aria-label="Close" @click="emit('close')">×</button>
      </header>
      <p class="pk-intro">
        Passkeys let you sign in with Touch ID, Face ID, or a hardware key — no password required.
      </p>

      <div v-if="!auth.passkeyEnabled" class="pk-warn">
        Passkeys are not enabled on this server. Ask your administrator to set
        <code>ARGUS_PASSKEY_ENABLED=true</code>.
      </div>

      <template v-else>
        <form class="pk-add" @submit.prevent="add">
          <input
            v-model="newName"
            type="text"
            placeholder="Name (e.g. MacBook Touch ID)"
            :disabled="busy"
            data-test="passkey-name"
          />
          <button type="submit" class="pk-add-btn" :disabled="busy" data-test="passkey-register">
            {{ busy ? 'Working…' : 'Add passkey' }}
          </button>
        </form>

        <p v-if="errorMsg" class="pk-error">{{ errorMsg }}</p>

        <div v-if="loading" class="pk-loading">Loading…</div>
        <ul v-else-if="credentials.length" class="pk-list">
          <li v-for="c in credentials" :key="c.id" class="pk-item">
            <div class="pk-item-meta">
              <div class="pk-item-name">{{ c.name || 'Unnamed key' }}</div>
              <div class="pk-item-detail">
                Added {{ fmtDate(c.createdAt) }} · Last used {{ fmtDate(c.lastUsedAt) }}
              </div>
            </div>
            <button class="pk-revoke" @click="revoke(c.id)" :data-test="`passkey-revoke-${c.id}`">
              Remove
            </button>
          </li>
        </ul>
        <p v-else class="pk-empty">No passkeys yet — add your first one above.</p>
      </template>
    </div>
  </div>
</template>

<style scoped>
.pk-shell {
  position: fixed;
  inset: 0;
  background: rgba(0,0,0,.5);
  display: grid;
  place-items: center;
  z-index: 100;
}
.pk-card {
  width: 100%;
  max-width: 32rem;
  background: var(--bg2);
  border: 1px solid var(--border);
  border-radius: var(--r);
  padding: 1.5rem;
  box-shadow: var(--shadow2);
  color: var(--text);
}
.pk-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: .75rem; }
.pk-head h2 { font-size: 1.15rem; margin: 0; }
.pk-close {
  background: transparent; border: 0; color: var(--text2);
  font-size: 1.3rem; cursor: pointer;
}
.pk-intro { color: var(--text2); font-size: .85rem; margin-bottom: 1rem; }
.pk-warn {
  background: rgba(240,180,80,.1);
  border: 1px solid rgba(240,180,80,.3);
  padding: .6rem .75rem;
  border-radius: var(--r2);
  font-size: .85rem;
}
.pk-add { display: flex; gap: .5rem; margin-bottom: 1rem; }
.pk-add input {
  flex: 1; background: var(--bg3); border: 1px solid var(--border2);
  border-radius: var(--r2); padding: .55rem .7rem; color: var(--text); font: inherit;
}
.pk-add-btn {
  background: var(--accent); color: #fff; border: 0;
  padding: .55rem 1rem; border-radius: var(--r2); cursor: pointer; font: inherit;
}
.pk-add-btn:disabled { opacity: .55; cursor: not-allowed; }
.pk-error {
  background: rgba(240,84,84,.1); border: 1px solid rgba(240,84,84,.3);
  color: var(--red2); padding: .55rem .75rem; border-radius: var(--r2);
  font-size: .85rem; margin-bottom: .75rem;
}
.pk-loading, .pk-empty { color: var(--text2); font-size: .9rem; padding: .5rem 0; }
.pk-list { list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: .5rem; }
.pk-item {
  display: flex; align-items: center; justify-content: space-between;
  background: var(--bg3); border: 1px solid var(--border2);
  padding: .65rem .8rem; border-radius: var(--r2);
}
.pk-item-name { font-weight: 500; font-size: .9rem; }
.pk-item-detail { color: var(--text3); font-size: .75rem; margin-top: .15rem; }
.pk-revoke {
  background: transparent; border: 1px solid var(--border2);
  color: var(--text2); padding: .35rem .65rem; border-radius: var(--r2);
  cursor: pointer; font: inherit; font-size: .8rem;
}
.pk-revoke:hover { background: rgba(240,84,84,.1); border-color: rgba(240,84,84,.3); color: var(--red2); }
</style>
