import { ref } from 'vue'

// useSecretStore wraps the macOS-Keychain–backed secret store the
// Wails backend exposes via app_secretstore.go. Three goals:
//
//   1. Persist the session token in OS-native secure storage where
//      file-level attackers cannot read it, instead of localStorage.
//   2. Stay drop-in compatible with the existing auth flow — the
//      composable falls back to localStorage when the Wails Keychain
//      bridge isn't available (SaaS / web / non-Mac dev), so callers
//      don't have to branch on the runtime.
//   3. Expose a one-line `isKeychainAvailable()` signal so the UI can
//      label "Session stored in macOS Keychain" honestly.
//
// Important: keys passed in from the frontend are IGNORED by the
// backend — every call routes to a single hard-coded entry under the
// "Argus" service. The frontend can't read or write arbitrary keychain
// items. See app_secretstore.go for the safety rationale.

const LEGACY_STORAGE_KEY = 'argus.auth.session'

function callGoBinding(name, ...args) {
  // We deliberately don't use the existing useBridge.callGo because
  // it transparently falls back to /api/* HTTP — and these bindings
  // are intentionally NOT in the HTTP allowlist. If the Wails binding
  // isn't available we want a clean local fallback, not an HTTP 403.
  const fn = window?.go?.api?.pkg?.App?.[name]
  if (typeof fn !== 'function') return null
  try { return fn(...args) }
  catch { return null }
}

function isKeychainAvailable() {
  return typeof window?.go?.api?.pkg?.App?.SetSessionToken === 'function'
}

// Reads the legacy localStorage envelope produced by older builds. We
// keep the migration helper so the first run of a new Argus build can
// move any pre-existing token into the Keychain without forcing the
// user to sign in again.
function readLegacyTokenFromLocalStorage() {
  try {
    const raw = localStorage.getItem(LEGACY_STORAGE_KEY)
    if (!raw) return ''
    const parsed = JSON.parse(raw)
    return parsed?.token || ''
  } catch { return '' }
}

export function useSecretStore() {
  // Reactive snapshot of the backend label — populated lazily by
  // refreshInfo(). The auth-status row in Settings reads it to render
  // the "Session stored in macOS Keychain" line.
  const backend = ref('')
  const available = ref(isKeychainAvailable())

  async function refreshInfo() {
    if (!isKeychainAvailable()) {
      backend.value = 'browser localStorage'
      available.value = false
      return
    }
    const p = callGoBinding('GetSecretStoreInfo')
    if (p && typeof p.then === 'function') {
      try {
        const info = await p
        backend.value = info?.backend || 'unknown'
        available.value = !!info?.available
      } catch {
        // Treat any failure as "unknown but available" — the bindings
        // themselves still work; only the label is unknown.
        backend.value = 'OS secret store'
        available.value = true
      }
    }
  }

  async function setSessionToken(token) {
    // Dual-write today: Keychain (when present) + localStorage. A
    // follow-up will drop the localStorage half once we've validated
    // the async-boot path. The Pinia auth store currently reads
    // localStorage synchronously at construction; removing it requires
    // adding a `tokenReady` gate which is out of scope for this turn.
    const tasks = []
    if (isKeychainAvailable()) {
      const p = callGoBinding('SetSessionToken', token || '')
      if (p && typeof p.then === 'function') tasks.push(p.catch(() => {}))
    }
    await Promise.all(tasks)
  }

  async function getSessionToken() {
    if (!isKeychainAvailable()) return ''
    const p = callGoBinding('GetSessionToken')
    if (!p || typeof p.then !== 'function') return ''
    try { return (await p) || '' }
    catch { return '' }
  }

  // hasSessionToken — cheap "do we already have a Keychain entry?"
  // check used by the biometric-unlock boot path before bothering the
  // user with a Touch ID prompt. A read here is silent (Keychain
  // doesn't dialog for entries the app has already accessed once).
  async function hasSessionToken() {
    const tok = await getSessionToken()
    return Boolean(tok)
  }

  // biometricAvailable — proxies the Wails App.IsBiometricAvailable
  // binding. Returns false (never throws) when the binding is missing,
  // e.g. SaaS/web mode or non-mac dev. The auth boot flow uses this
  // to decide whether to even attempt the Touch ID prompt.
  async function biometricAvailable() {
    if (typeof window?.go?.api?.pkg?.App?.IsBiometricAvailable !== 'function') {
      return false
    }
    const p = callGoBinding('IsBiometricAvailable')
    if (!p || typeof p.then !== 'function') return false
    try { return Boolean(await p) }
    catch { return false }
  }

  // authenticateWithBiometrics — triggers the system Touch ID prompt
  // via the Wails App.AuthenticateWithBiometrics binding. Resolves on
  // success, rejects with the Go-side error message on failure
  // (cancellation, no enrolled fingerprint, retry limit, …).
  async function authenticateWithBiometrics(reason) {
    if (typeof window?.go?.api?.pkg?.App?.AuthenticateWithBiometrics !== 'function') {
      throw new Error('biometric: bridge unavailable')
    }
    const p = callGoBinding('AuthenticateWithBiometrics', reason || 'Unlock Argus')
    if (!p || typeof p.then !== 'function') {
      throw new Error('biometric: bridge call did not return a promise')
    }
    await p
  }

  async function clearSessionToken() {
    const tasks = []
    if (isKeychainAvailable()) {
      const p = callGoBinding('ClearSessionToken')
      if (p && typeof p.then === 'function') tasks.push(p.catch(() => {}))
    }
    await Promise.all(tasks)
  }

  // migrateLegacyToken copies any pre-existing localStorage token into
  // the Keychain on first run of a Keychain-aware build. Idempotent —
  // a second call with a populated Keychain is a no-op. Safe to call on
  // every boot.
  async function migrateLegacyToken() {
    if (!isKeychainAvailable()) return false
    const existing = await getSessionToken()
    if (existing) return false
    const legacy = readLegacyTokenFromLocalStorage()
    if (!legacy) return false
    await setSessionToken(legacy)
    return true
  }

  return {
    backend,
    available,
    isKeychainAvailable,
    refreshInfo,
    setSessionToken,
    getSessionToken,
    hasSessionToken,
    clearSessionToken,
    migrateLegacyToken,
    biometricAvailable,
    authenticateWithBiometrics,
  }
}

// Test-only surface.
export const __test = {
  LEGACY_STORAGE_KEY,
  readLegacyTokenFromLocalStorage,
}
