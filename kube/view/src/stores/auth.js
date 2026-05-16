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
const LAST_METHOD_KEY = 'argus.auth.lastMethod'

// readLastMethod / writeLastMethod surface the "Continue with X" CTA
// on the LoginView. The shape is { kind, email?, provider? } where
// kind ∈ {'local','oauth','passkey'}. We treat any malformed read as
// "no preference" so a corrupted localStorage just falls back to the
// full picker rather than crashing the boot path.
function readLastMethod() {
  try {
    const raw = localStorage.getItem(LAST_METHOD_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed || !parsed.kind) return null
    if (!['local', 'oauth', 'passkey'].includes(parsed.kind)) return null
    return parsed
  } catch {
    return null
  }
}

function writeLastMethod(payload) {
  try {
    if (payload && payload.kind) {
      localStorage.setItem(LAST_METHOD_KEY, JSON.stringify(payload))
    } else {
      localStorage.removeItem(LAST_METHOD_KEY)
    }
  } catch {
    // localStorage private-mode failure: not worth surfacing.
  }
}

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
  // passkeyEnabled mirrors /auth/providers.passkeyEnabled. When false
  // the LoginView hides the passkey CTA and PasskeyManager.vue
  // refuses to register. We default to false so a /auth/providers
  // failure doesn't accidentally surface UI that will error on use.
  const passkeyEnabled = ref(false)
  // lastUsedMethod drives the one-tap "Continue with X" CTA on the
  // LoginView. See readLastMethod for the shape.
  const lastUsedMethod = ref(readLastMethod())
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
      passkeyEnabled.value = r.passkeyEnabled === true
      // Visible breadcrumb so the operator can confirm in dev tools
      // exactly what landed when `make no-auth-run` mysteriously
      // doesn't bypass the gate. Mirror the backend's startup banner.
      if (authDisabled.value) {
        console.info('[auth] dev-mode ON — gate bypassed (server reports authDisabled=true)')
      } else {
        console.info('[auth] dev-mode OFF — login required',
          { providers: providers.value.map((p) => p.name), allowSignup: allowSignup.value })
      }
    } catch (err) {
      // Network failure: keep secure defaults so the local form still renders.
      providers.value = []
      allowSignup.value = true
      authDisabled.value = false
      passkeyEnabled.value = false
      console.error('[auth] /auth/providers failed — falling back to login screen. Check that the backend is up at',
        apiBase, 'and that ARGUS_AUTH_DISABLED reached it:', err)
    }
  }

  function _recordLastMethod(payload) {
    lastUsedMethod.value = payload
    writeLastMethod(payload)
  }

  async function login(email, password) {
    const r = await _post('/auth/login', { email, password })
    _adopt(r)
    _recordLastMethod({ kind: 'local', email })
    return user.value
  }

  async function register(email, name, password) {
    const r = await _post('/auth/register', { email, name, password })
    _adopt(r)
    _recordLastMethod({ kind: 'local', email })
    return user.value
  }

  // ---- Passkeys (WebAuthn) ---------------------------------------------
  //
  // We import @simplewebauthn/browser lazily so the bundle stays slim
  // for deployments that disable the feature. The helper handles the
  // base64url ↔ ArrayBuffer ceremony that browsers insist on; rolling
  // our own is the #1 source of "InvalidStateError" bugs in WebAuthn
  // code.

  async function _swaBrowser() {
    return import('@simplewebauthn/browser')
  }

  // loginWithPasskey runs a discoverable (usernameless) login. When
  // mediation === 'conditional' the browser surfaces the passkey in the
  // email field's autocomplete pop-up instead of a modal; the LoginView
  // kicks one of these off on mount so users with a passkey don't even
  // see the form.
  async function loginWithPasskey({ mediation } = {}) {
    if (!passkeyEnabled.value) throw new Error('Passkeys are not enabled on this server')
    const swa = await _swaBrowser()
    const begin = await _post('/auth/passkey/login/begin', null)
    const credential = await swa.startAuthentication({
      optionsJSON: begin.publicKey,
      useBrowserAutofill: mediation === 'conditional',
    })
    const r = await _post('/auth/passkey/login/finish', { state: begin.state, credential })
    _adopt(r)
    _recordLastMethod({ kind: 'passkey' })
    return user.value
  }

  // registerPasskey requires an authenticated session — you can only
  // add a passkey to your own account. The optional `name` is the
  // human-facing label (e.g. "MacBook Touch ID") shown in the
  // management UI; blank gets a fallback on the server.
  async function registerPasskey(name = '') {
    if (!passkeyEnabled.value) throw new Error('Passkeys are not enabled on this server')
    const swa = await _swaBrowser()
    const begin = await _post('/auth/passkey/register/begin', null)
    const credential = await swa.startRegistration({ optionsJSON: begin.publicKey })
    return _post('/auth/passkey/register/finish', { state: begin.state, name, credential })
  }

  async function listPasskeys() {
    const r = await _get('/auth/passkey/list')
    return r.credentials || []
  }

  async function revokePasskey(id) {
    const res = await fetch(`${apiBase}/auth/passkey/${encodeURIComponent(id)}`, {
      method: 'DELETE',
      headers: authHeaders(),
    })
    if (!res.ok) {
      throw new Error((await res.text()) || `HTTP ${res.status}`)
    }
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
      _recordLastMethod({ kind: 'oauth', provider: r.user?.provider || '' })
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
    passkeyEnabled,
    lastUsedMethod,
    isAuthenticated,
    authHeaders,
    loadProviders,
    login,
    register,
    logout,
    restoreSession,
    startOAuth,
    pollOAuth,
    loginWithPasskey,
    registerPasskey,
    listPasskeys,
    revokePasskey,
  }
})
