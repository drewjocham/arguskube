<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useAuthStore } from '../../stores/auth'

// OAuthLoginButton — a single button that exposes every configured OAuth
// provider behind one click. Replaces the previous "one button per
// provider" stack so the login screen stays compact regardless of how
// many providers the operator has enabled.
//
// Behaviour:
//   • Click: opens a popover listing every provider. The user picks one;
//     we kick off the auth flow via auth.startOAuth(name) and open the
//     authorization URL in the system browser. We then long-poll
//     auth.pollOAuth(state) until the backend confirms (or 5 min elapse).
//   • If only one provider is configured, click goes straight to it
//     instead of opening the menu — saves a tap.
//   • If zero providers are configured, the button is disabled and the
//     tooltip explains that no providers are set up yet.
//   • Keyboard: Escape closes the menu, ArrowUp/Down moves selection.
//
// This component does NOT own any persisted state — it just wraps the
// auth store's existing startOAuth / pollOAuth API. The login screen can
// either render this button alone or alongside the email/password form.

const props = defineProps({
  // Optional override for the button label. Defaults to "Continue with…"
  // when ≥1 providers, "OAuth login" when 0.
  label: { type: String, default: '' },
  // Optional override for the auth.startOAuth poll cap (ms).
  pollTimeoutMs: { type: Number, default: 5 * 60 * 1000 },
  // Tight presentation (smaller padding, no description); used in modals.
  compact: { type: Boolean, default: false },
  // If true, fetch /auth/providers on mount. Skip when the parent already
  // populated auth.providers (the LoginView does this already).
  loadOnMount: { type: Boolean, default: false },
})

const emit = defineEmits(['login', 'error', 'cancel'])

const auth = useAuthStore()
const open = ref(false)
const inFlight = ref('')         // provider name during a flow
const errorMsg = ref('')
const focusedIdx = ref(-1)
const triggerRef = ref(null)
const menuRef = ref(null)

onMounted(async () => {
  if (props.loadOnMount) {
    try { await auth.loadProviders() } catch { /* defaults */ }
  }
  document.addEventListener('mousedown', onDocClick)
  document.addEventListener('keydown', onKey)
})
onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocClick)
  document.removeEventListener('keydown', onKey)
})

function onDocClick(e) {
  if (!open.value) return
  if (triggerRef.value?.contains?.(e.target)) return
  if (menuRef.value?.contains?.(e.target)) return
  open.value = false
}

function onKey(e) {
  if (!open.value) return
  if (e.key === 'Escape') {
    open.value = false
    triggerRef.value?.focus?.()
  } else if (e.key === 'ArrowDown') {
    e.preventDefault()
    focusedIdx.value = Math.min(auth.providers.length - 1, focusedIdx.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    focusedIdx.value = Math.max(0, focusedIdx.value - 1)
  } else if (e.key === 'Enter' && focusedIdx.value >= 0) {
    e.preventDefault()
    const p = auth.providers[focusedIdx.value]
    if (p) startFlow(p.name)
  }
}

const buttonLabel = computed(() => {
  if (props.label) return props.label
  if (inFlight.value) return `Signing in with ${inFlight.value}…`
  if (!auth.providers.length) return 'OAuth login'
  if (auth.providers.length === 1) return `Continue with ${auth.providers[0].displayName || auth.providers[0].name}`
  return 'Continue with…'
})

const buttonDisabled = computed(() => Boolean(inFlight.value) || !auth.providers.length)

function toggleMenu() {
  if (buttonDisabled.value) return
  // Single-provider shortcut: skip the menu entirely.
  if (auth.providers.length === 1) {
    return startFlow(auth.providers[0].name)
  }
  open.value = !open.value
  if (open.value) {
    focusedIdx.value = 0
  }
}

function openInBrowser(url) {
  if (typeof window !== 'undefined' && window.runtime?.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(url)
  } else if (typeof window !== 'undefined') {
    window.open(url, '_blank', 'noopener,noreferrer')
  }
}

async function startFlow(provider) {
  if (inFlight.value) return
  errorMsg.value = ''
  open.value = false
  inFlight.value = provider
  let timer = null
  try {
    const { authUrl, state } = await auth.startOAuth(provider)
    if (!authUrl || !state) throw new Error('OAuth start did not return authUrl/state')
    openInBrowser(authUrl)
    const deadline = Date.now() + props.pollTimeoutMs
    while (Date.now() < deadline && inFlight.value === provider) {
      await new Promise((r) => { timer = setTimeout(r, 1500) })
      if (inFlight.value !== provider) break // user cancelled
      try {
        const res = await auth.pollOAuth(state)
        if (res?.done) {
          inFlight.value = ''
          emit('login', res.user || auth.user)
          return
        }
      } catch (err) {
        inFlight.value = ''
        errorMsg.value = err?.message || 'OAuth login failed'
        emit('error', errorMsg.value)
        return
      }
    }
    if (inFlight.value === provider) {
      inFlight.value = ''
      errorMsg.value = 'OAuth sign-in timed out'
      emit('error', errorMsg.value)
    }
  } catch (err) {
    inFlight.value = ''
    errorMsg.value = err?.message || 'Could not start OAuth flow'
    emit('error', errorMsg.value)
  } finally {
    if (timer) clearTimeout(timer)
  }
}

function cancel() {
  if (!inFlight.value) return
  const was = inFlight.value
  inFlight.value = ''
  emit('cancel', was)
}

// A single-letter provider mark for the tile icons. Doesn't have to be
// pretty — branded SVG marks would be better but adding a logo per
// provider bloats the bundle and the upstream brand kits aren't
// redistributable.
function providerMark(p) {
  const n = (p?.displayName || p?.name || '?').trim()
  return n[0]?.toUpperCase() || '?'
}

// Expose for tests and parent debugging.
defineExpose({ open, inFlight, startFlow, cancel })
</script>

<template>
  <div class="oauth-login-btn-wrap" :class="{ compact }">
    <button
      ref="triggerRef"
      type="button"
      class="oauth-login-trigger"
      :class="{ 'in-flight': inFlight, 'is-disabled': buttonDisabled }"
      :disabled="buttonDisabled"
      :title="buttonDisabled && !auth.providers.length
        ? 'No OAuth providers configured. Set them in Settings → Authentication.'
        : ''"
      :aria-haspopup="auth.providers.length > 1 ? 'menu' : 'false'"
      :aria-expanded="open"
      @click="toggleMenu"
    >
      <span v-if="inFlight" class="oauth-spinner" aria-hidden="true"></span>
      <svg v-else class="oauth-glyph" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="12" cy="12" r="9"></circle>
        <path d="M12 7v5l3 2"></path>
      </svg>
      <span class="oauth-label">{{ buttonLabel }}</span>
      <svg v-if="!inFlight && auth.providers.length > 1" class="oauth-chevron" :class="{ flipped: open }" width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
        <path d="M2 3l3 3 3-3" stroke="currentColor" stroke-width="1.4" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
      </svg>
    </button>

    <div v-if="inFlight" class="oauth-pending-row">
      <span>Finish signing in with <strong>{{ inFlight }}</strong> in your browser.</span>
      <button type="button" class="oauth-link" @click="cancel">Cancel</button>
    </div>

    <div
      v-if="open && auth.providers.length > 1"
      ref="menuRef"
      class="oauth-menu"
      role="menu"
    >
      <div class="oauth-menu-head">Choose a provider</div>
      <button
        v-for="(p, i) in auth.providers"
        :key="p.name"
        type="button"
        role="menuitem"
        class="oauth-menu-item"
        :class="{ focused: focusedIdx === i }"
        :data-provider="p.name"
        @click="startFlow(p.name)"
        @mouseenter="focusedIdx = i"
      >
        <span class="oauth-mark" :data-provider="p.name">{{ providerMark(p) }}</span>
        <span class="oauth-pname">{{ p.displayName || p.name }}</span>
        <span v-if="p.note" class="oauth-pnote">{{ p.note }}</span>
      </button>
    </div>

    <p v-if="errorMsg" class="oauth-err" role="alert">{{ errorMsg }}</p>
  </div>
</template>

<style scoped>
.oauth-login-btn-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.oauth-login-trigger {
  display: flex; align-items: center; gap: 8px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  color: var(--text);
  padding: 9px 14px;
  border-radius: 6px;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  transition: background 0.12s, border-color 0.12s;
  width: 100%;
}
.oauth-login-trigger:hover:not(:disabled) { background: var(--bg4); border-color: var(--accent); }
.oauth-login-trigger:disabled,
.oauth-login-trigger.is-disabled { opacity: 0.55; cursor: not-allowed; }
.oauth-login-trigger.in-flight { color: var(--text2); border-color: var(--accent); }

.oauth-glyph { color: var(--accent); flex-shrink: 0; }
.oauth-label { flex: 1; text-align: left; }
.oauth-chevron { color: var(--text3); transition: transform 0.15s ease; flex-shrink: 0; }
.oauth-chevron.flipped { transform: rotate(180deg); }

.oauth-spinner {
  width: 14px; height: 14px;
  border: 2px solid rgba(255,255,255,0.18);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: oauth-spin 0.85s linear infinite;
  flex-shrink: 0;
}
@keyframes oauth-spin { to { transform: rotate(360deg); } }

.oauth-pending-row {
  display: flex; align-items: center; gap: 8px;
  font-size: 11px;
  color: var(--text2);
  padding: 4px 4px 0;
}
.oauth-link {
  background: transparent; border: 0; color: var(--accent2);
  font: inherit; font-size: 11px; cursor: pointer; text-decoration: underline;
  padding: 0;
}
.oauth-link:hover { color: var(--accent); }

.oauth-menu {
  position: absolute;
  top: calc(100% + 4px);
  left: 0; right: 0;
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: 6px;
  box-shadow: 0 12px 32px rgba(0,0,0,0.4);
  z-index: 10;
  padding: 4px;
  animation: oauth-menu-in 0.12s ease-out;
}
@keyframes oauth-menu-in {
  from { opacity: 0; transform: translateY(-4px); }
  to   { opacity: 1; transform: translateY(0); }
}
.oauth-menu-head {
  padding: 6px 10px 4px;
  font-size: 9.5px;
  text-transform: uppercase;
  letter-spacing: 0.07em;
  color: var(--text3);
  font-weight: 600;
}
.oauth-menu-item {
  display: flex; align-items: center; gap: 9px;
  width: 100%;
  background: transparent;
  border: 0;
  color: var(--text);
  text-align: left;
  padding: 7px 10px;
  border-radius: 4px;
  font: inherit;
  font-size: 12.5px;
  cursor: pointer;
  transition: background 0.1s;
}
.oauth-menu-item:hover,
.oauth-menu-item.focused { background: var(--bg4); }
.oauth-menu-item:focus-visible { outline: 2px solid var(--accent); outline-offset: -2px; }

.oauth-mark {
  width: 22px; height: 22px;
  display: inline-flex; align-items: center; justify-content: center;
  border-radius: 4px;
  background: var(--bg2);
  color: var(--text2);
  font-weight: 600; font-size: 11px;
  flex-shrink: 0;
}
.oauth-mark[data-provider="google"]    { color: #4285F4; }
.oauth-mark[data-provider="github"]    { color: #f0f6fc; background: #24292e; }
.oauth-mark[data-provider="gitlab"]    { color: #FC6D26; }
.oauth-mark[data-provider="bitbucket"] { color: #0052CC; }
.oauth-mark[data-provider="microsoft"] { color: #00a4ef; }
.oauth-mark[data-provider="azure"]     { color: #00a4ef; }
.oauth-mark[data-provider="okta"]      { color: var(--purple); }
.oauth-mark[data-provider="oidc"]      { color: var(--purple); }

.oauth-pname { flex: 1; }
.oauth-pnote { font-size: 10.5px; color: var(--text3); }

.oauth-err {
  margin: 0;
  background: rgba(240,84,84,0.1);
  border: 1px solid rgba(240,84,84,0.3);
  color: var(--red2, #f05454);
  padding: 6px 10px;
  border-radius: 5px;
  font-size: 11.5px;
}

.oauth-login-btn-wrap.compact .oauth-login-trigger { padding: 6px 10px; font-size: 12px; }
.oauth-login-btn-wrap.compact .oauth-pending-row { font-size: 10.5px; }
</style>
