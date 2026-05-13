// Auth store — holds the current session token + user identity.
// The token is the only thing the rest of the app reads; everything
// else flows from /auth/me on demand. Persisted to localStorage so a
// reload doesn't kick the user back to the login screen.
//
// Wails bindings bypass HTTP, so this gate is frontend-only for those.
// The server-side /api/* gate (sessions in sqlite) is still authoritative
// for any out-of-process caller.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { apiBase } from '../composables/useBridge'
import { useSecretStore } from '../composables/useSecretStore'

const STORAGE_KEY = 'argus.auth.session'

function readPersisted() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed.token || !parsed.expiresAt) return null
    if (Date.now() / 1000 >= parsed.expiresAt) return null
    return parsed
  } catch {
    return null
  }
}

function writePersisted(payload) {
  try {
    if (payload) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(payload))
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
  } catch {
    // localStorage can throw in private mode; auth still works in-memory
    // for the current session.
  }
}

export const useAuthStore = defineStore('auth', () => {
  const persisted = readPersisted()
  const token = ref(persisted?.token || '')
  const user = ref(persisted?.user || null)
  const expiresAt = ref(persisted?.expiresAt || 0)

  // Provider list shown on the login screen — populated lazily by
  // loadProviders(). Always includes "local" as a virtual entry so the
  // form renders even if no OAuth is configured.
  const providers = ref([])
  const allowSignup = ref(true)
  // authDisabled is set by /auth/providers when the backend is running
  // with ARGUS_AUTH_DISABLED=true. App.vue uses it to skip the
  // LoginView gate entirely. We default to false so a network failure
  // on /auth/providers keeps the secure behavior.
  const authDisabled = ref(false)

  // The dashboard renders when either the user is signed in or auth is
  // disabled in dev mode.
  const isAuthenticated = computed(() => authDisabled.value || Boolean(token.value && user.value))

  function authHeaders(extra = {}) {
    if (!token.value) return extra
    return { ...extra, Authorization: `Bearer ${token.value}` }
  }

  async function _post(path, body) {
    const res = await fetch(`${apiBase}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authHeaders() },
      body: body ? JSON.stringify(body) : undefined,
    })
    const text = await res.text()
    let data = null
    if (text) {
      try {
        data = JSON.parse(text)
      } catch {
        // server returned plain-text error from http.Error; keep as message
        if (!res.ok) throw new Error(text || `HTTP ${res.status}`)
      }
    }
    if (!res.ok) {
      const msg = (data && data.error) || text || `HTTP ${res.status}`
      throw new Error(msg)
    }
    return data
  }

  async function _get(path) {
    const res = await fetch(`${apiBase}${path}`, {
      headers: authHeaders(),
    })
    if (!res.ok) {
      const t = await res.text()
      throw new Error(t || `HTTP ${res.status}`)
    }
    return res.json()
  }

  const secretStore = useSecretStore()

  function _adopt(loginPayload) {
    if (!loginPayload?.token || !loginPayload?.user) {
      throw new Error('login response missing token or user')
    }
    token.value = loginPayload.token
    user.value = loginPayload.user
    expiresAt.value = loginPayload.expiresAt || (Math.floor(Date.now() / 1000) + 14 * 24 * 60 * 60)
    writePersisted({ token: token.value, user: user.value, expiresAt: expiresAt.value })
    // Dual-write the token to the OS secret store when available. The
    // localStorage write above stays as the boot-time fast path; a
    // follow-up turn will drop it on macOS once the async-boot gate is
    // wired through App.vue. Fire-and-forget — a Keychain error never
    // fails an otherwise-good login.
    secretStore.setSessionToken(token.value).catch(() => {})
  }

  async function loadProviders() {
    try {
      const r = await _get('/auth/providers')
      providers.value = r.providers || []
      allowSignup.value = r.allowSignup !== false
      authDisabled.value = r.authDisabled === true
    } catch (err) {
      // Network failure: keep secure defaults so the local form still renders.
      providers.value = []
      allowSignup.value = true
      authDisabled.value = false
      console.warn('[auth] /auth/providers failed:', err)
    }
  }

  async function login(email, password) {
    const r = await _post('/auth/login', { email, password })
    _adopt(r)
    return user.value
  }

  async function register(email, name, password) {
    const r = await _post('/auth/register', { email, name, password })
    _adopt(r)
    return user.value
  }

  async function logout() {
    if (token.value) {
      try {
        await _post('/auth/logout', null)
      } catch {
        // Server-side revoke is best-effort; we still clear local state.
      }
    }
    token.value = ''
    user.value = null
    expiresAt.value = 0
    writePersisted(null)
    // Best-effort wipe of the OS secret store. Same fire-and-forget
    // pattern as _adopt — we don't want a Keychain hiccup to block
    // logout from completing on the UI side.
    secretStore.clearSessionToken().catch(() => {})
  }

  // Verify a persisted session is still good. Called on app boot — if
  // the token has been revoked server-side or the DB was wiped, we
  // bounce the user back to the login screen instead of letting every
  // /api call return 401 individually.
  async function restoreSession() {
    if (!token.value) return false
    try {
      const me = await _get('/auth/me')
      user.value = me
      writePersisted({ token: token.value, user: user.value, expiresAt: expiresAt.value })
      return true
    } catch {
      token.value = ''
      user.value = null
      expiresAt.value = 0
      writePersisted(null)
      return false
    }
  }

  // Kicks off an OAuth login. Returns the upstream auth URL + state
  // token; the caller is responsible for opening the URL in the
  // system browser, then calling pollOAuth(state) until it resolves.
  async function startOAuth(provider) {
    return _post('/auth/oauth/start', { provider })
  }

  async function pollOAuth(state) {
    const url = `${apiBase}/auth/oauth/poll?state=${encodeURIComponent(state)}`
    const res = await fetch(url, { headers: authHeaders() })
    if (!res.ok) {
      throw new Error(await res.text())
    }
    const r = await res.json()
    if (r.status === 'ok') {
      _adopt({ token: r.token, user: r.user, expiresAt: 0 })
      return { done: true, user: user.value }
    }
    if (r.status === 'error') {
      throw new Error(r.error || 'OAuth login failed')
    }
    return { done: false }
  }

  return {
    token,
    user,
    expiresAt,
    providers,
    allowSignup,
    authDisabled,
    isAuthenticated,
    authHeaders,
    loadProviders,
    login,
    register,
    logout,
    restoreSession,
    startOAuth,
    pollOAuth,
  }
})
