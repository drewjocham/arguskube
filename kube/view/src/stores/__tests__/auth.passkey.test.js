// Passkey-specific store tests. The WebAuthn calls themselves are
// faked at the @simplewebauthn/browser boundary so we can exercise the
// full begin → ceremony → finish round-trip without a real
// authenticator. The browser WebAuthn API is also stubbed for the
// conditional-mediation path.
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const memory = {}
Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: (k) => (k in memory ? memory[k] : null),
    setItem: (k, v) => { memory[k] = String(v) },
    removeItem: (k) => { delete memory[k] },
    clear: () => { for (const k of Object.keys(memory)) delete memory[k] },
  },
  writable: true, configurable: true,
})

const API_BASE = 'http://127.0.0.1:8080'

vi.mock('../../composables/useBridge', () => ({ apiBase: API_BASE }))

// Stub @simplewebauthn/browser. The dynamic import in the store will
// pick this up because vitest hoists vi.mock calls.
const swaCalls = { startAuthentication: 0, startRegistration: 0 }
vi.mock('@simplewebauthn/browser', () => ({
  startAuthentication: vi.fn(async () => {
    swaCalls.startAuthentication++
    return { id: 'cred-id', response: {}, type: 'public-key' }
  }),
  startRegistration: vi.fn(async () => {
    swaCalls.startRegistration++
    return { id: 'new-cred', response: {}, type: 'public-key' }
  }),
}))

describe('auth store — passkeys', () => {
  let store

  beforeEach(async () => {
    for (const k of Object.keys(memory)) delete memory[k]
    swaCalls.startAuthentication = 0
    swaCalls.startRegistration = 0
    vi.resetModules()
    setActivePinia(createPinia())
    const mod = await import('../auth')
    store = mod.useAuthStore()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  // mkFetch returns a fetch stub that dispatches by URL substring.
  function mkFetch(routes) {
    return vi.fn((url, opts = {}) => {
      for (const [pat, resp] of Object.entries(routes)) {
        if (String(url).includes(pat)) {
          const body = typeof resp === 'function' ? resp(opts) : resp
          return Promise.resolve({
            ok: true,
            status: 200,
            text: () => Promise.resolve(JSON.stringify(body)),
            json: () => Promise.resolve(body),
          })
        }
      }
      return Promise.reject(new Error(`unmocked: ${url}`))
    })
  }

  it('reads passkeyEnabled from /auth/providers', async () => {
    vi.stubGlobal('fetch', mkFetch({
      '/auth/providers': { providers: [], allowSignup: true, authDisabled: false, passkeyEnabled: true },
    }))
    await store.loadProviders()
    expect(store.passkeyEnabled).toBe(true)
  })

  it('defaults passkeyEnabled false when omitted', async () => {
    vi.stubGlobal('fetch', mkFetch({
      '/auth/providers': { providers: [], allowSignup: true },
    }))
    await store.loadProviders()
    expect(store.passkeyEnabled).toBe(false)
  })

  it('loginWithPasskey runs the begin→authenticator→finish ceremony and adopts the session', async () => {
    store.passkeyEnabled = true
    vi.stubGlobal('fetch', mkFetch({
      '/auth/passkey/login/begin': { publicKey: { challenge: 'abc' }, state: 'state-1' },
      '/auth/passkey/login/finish': {
        token: 't1',
        user: { id: 'u1', email: 'me@x.com', provider: 'local' },
        expiresAt: 9999999999,
      },
    }))
    await store.loginWithPasskey()
    expect(swaCalls.startAuthentication).toBe(1)
    expect(store.token).toBe('t1')
    expect(store.user.email).toBe('me@x.com')
    expect(store.lastUsedMethod).toEqual({ kind: 'passkey' })
  })

  it('loginWithPasskey throws when feature disabled', async () => {
    store.passkeyEnabled = false
    await expect(store.loginWithPasskey()).rejects.toThrow(/not enabled/)
  })

  it('registerPasskey posts begin then finish with the chosen name', async () => {
    store.passkeyEnabled = true
    let finishBody = null
    vi.stubGlobal('fetch', mkFetch({
      '/auth/passkey/register/begin': { publicKey: { challenge: 'r' }, state: 'r-state' },
      '/auth/passkey/register/finish': (opts) => {
        finishBody = JSON.parse(opts.body)
        return { id: 7, name: 'YubiKey', createdAt: 0, lastUsedAt: 0 }
      },
    }))
    const res = await store.registerPasskey('YubiKey')
    expect(swaCalls.startRegistration).toBe(1)
    expect(finishBody.state).toBe('r-state')
    expect(finishBody.name).toBe('YubiKey')
    expect(res.id).toBe(7)
  })

  it('listPasskeys returns the credentials array', async () => {
    vi.stubGlobal('fetch', mkFetch({
      '/auth/passkey/list': { credentials: [{ id: 1, name: 'Key A' }] },
    }))
    const out = await store.listPasskeys()
    expect(out).toEqual([{ id: 1, name: 'Key A' }])
  })

  it('revokePasskey issues a DELETE', async () => {
    let captured = null
    vi.stubGlobal('fetch', vi.fn((url, opts) => {
      captured = { url, method: opts.method }
      return Promise.resolve({ ok: true, status: 204, text: () => Promise.resolve('') })
    }))
    await store.revokePasskey(42)
    expect(captured.method).toBe('DELETE')
    expect(captured.url).toContain('/auth/passkey/42')
  })

  it('login records lastUsedMethod for the local path too', async () => {
    vi.stubGlobal('fetch', mkFetch({
      '/auth/login': { token: 't', user: { id: 'u', email: 'a@b.com' }, expiresAt: 9999999999 },
    }))
    await store.login('a@b.com', 'pw')
    expect(store.lastUsedMethod.kind).toBe('local')
    expect(store.lastUsedMethod.email).toBe('a@b.com')
  })
})
