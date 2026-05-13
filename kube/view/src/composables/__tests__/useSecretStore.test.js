import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useSecretStore, __test } from '../useSecretStore'

// Stub the Wails bridge object the composable looks for. window.go is
// populated by Wails at runtime; in tests we mount a fake with just the
// methods the composable uses.
function stubWails(overrides = {}) {
  window.go = {
    api: {
      pkg: {
        App: {
          GetSecretStoreInfo: vi.fn().mockResolvedValue({ backend: 'macOS Keychain', available: true }),
          SetSessionToken: vi.fn().mockResolvedValue(undefined),
          GetSessionToken: vi.fn().mockResolvedValue(''),
          ClearSessionToken: vi.fn().mockResolvedValue(undefined),
          ...overrides,
        },
      },
    },
  }
}

const memoryStorage = {}
beforeEach(() => {
  for (const k of Object.keys(memoryStorage)) delete memoryStorage[k]
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: {
      getItem: (k) => (k in memoryStorage ? memoryStorage[k] : null),
      setItem: (k, v) => { memoryStorage[k] = String(v) },
      removeItem: (k) => { delete memoryStorage[k] },
      clear: () => { for (const k of Object.keys(memoryStorage)) delete memoryStorage[k] },
    },
  })
  delete window.go
})

afterEach(() => {
  vi.restoreAllMocks()
  delete window.go
})

describe('useSecretStore — no Wails bridge', () => {
  it('reports keychain unavailable when window.go is missing', () => {
    const s = useSecretStore()
    expect(s.isKeychainAvailable()).toBe(false)
  })

  it('setSessionToken is a no-op when keychain is unavailable', async () => {
    const s = useSecretStore()
    await expect(s.setSessionToken('abc')).resolves.toBeUndefined()
  })

  it('getSessionToken returns empty string when keychain is unavailable', async () => {
    const s = useSecretStore()
    expect(await s.getSessionToken()).toBe('')
  })

  it('clearSessionToken is a no-op when keychain is unavailable', async () => {
    const s = useSecretStore()
    await expect(s.clearSessionToken()).resolves.toBeUndefined()
  })

  it('refreshInfo reports browser localStorage when no bridge', async () => {
    const s = useSecretStore()
    await s.refreshInfo()
    expect(s.backend.value).toBe('browser localStorage')
    expect(s.available.value).toBe(false)
  })

  it('migrateLegacyToken is a no-op when keychain is unavailable', async () => {
    memoryStorage[__test.LEGACY_STORAGE_KEY] = JSON.stringify({ token: 'legacy', expiresAt: 9999 })
    const s = useSecretStore()
    expect(await s.migrateLegacyToken()).toBe(false)
  })
})

describe('useSecretStore — Wails bridge present', () => {
  it('reports keychain available when SetSessionToken binding exists', () => {
    stubWails()
    const s = useSecretStore()
    expect(s.isKeychainAvailable()).toBe(true)
  })

  it('setSessionToken routes through the Wails binding', async () => {
    stubWails()
    const s = useSecretStore()
    await s.setSessionToken('abc123')
    expect(window.go.api.pkg.App.SetSessionToken).toHaveBeenCalledWith('abc123')
  })

  it('getSessionToken returns the value from the binding', async () => {
    stubWails({ GetSessionToken: vi.fn().mockResolvedValue('persisted-token') })
    const s = useSecretStore()
    expect(await s.getSessionToken()).toBe('persisted-token')
  })

  it('getSessionToken returns "" when the binding throws', async () => {
    stubWails({ GetSessionToken: vi.fn().mockRejectedValue(new Error('boom')) })
    const s = useSecretStore()
    expect(await s.getSessionToken()).toBe('')
  })

  it('refreshInfo populates the backend label', async () => {
    stubWails()
    const s = useSecretStore()
    await s.refreshInfo()
    expect(s.backend.value).toBe('macOS Keychain')
    expect(s.available.value).toBe(true)
  })

  it('migrateLegacyToken copies localStorage→Keychain when Keychain is empty', async () => {
    memoryStorage[__test.LEGACY_STORAGE_KEY] = JSON.stringify({ token: 'leg', expiresAt: 9999 })
    stubWails()
    const s = useSecretStore()
    const moved = await s.migrateLegacyToken()
    expect(moved).toBe(true)
    expect(window.go.api.pkg.App.SetSessionToken).toHaveBeenCalledWith('leg')
  })

  it('migrateLegacyToken is a no-op when Keychain already has a token', async () => {
    memoryStorage[__test.LEGACY_STORAGE_KEY] = JSON.stringify({ token: 'leg', expiresAt: 9999 })
    stubWails({ GetSessionToken: vi.fn().mockResolvedValue('already-there') })
    const s = useSecretStore()
    expect(await s.migrateLegacyToken()).toBe(false)
    expect(window.go.api.pkg.App.SetSessionToken).not.toHaveBeenCalled()
  })

  it('migrateLegacyToken is a no-op when localStorage has no legacy token', async () => {
    stubWails()
    const s = useSecretStore()
    expect(await s.migrateLegacyToken()).toBe(false)
    expect(window.go.api.pkg.App.SetSessionToken).not.toHaveBeenCalled()
  })

  it('setSessionToken swallows backend errors so logout/login never break', async () => {
    stubWails({ SetSessionToken: vi.fn().mockRejectedValue(new Error('keychain locked')) })
    const s = useSecretStore()
    await expect(s.setSessionToken('abc')).resolves.toBeUndefined()
  })
})
