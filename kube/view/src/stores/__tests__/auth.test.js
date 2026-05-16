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

vi.mock('../../composables/useBridge', () => ({
  apiBase: API_BASE,
}))

describe('auth store', () => {
  let store

  beforeEach(async () => {
    for (const k of Object.keys(memory)) delete memory[k]
    vi.resetModules()
    setActivePinia(createPinia())
    const mod = await import('../auth')
    store = mod.useAuthStore()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function mockFetch(status, body) {
    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve({
        ok: status >= 200 && status < 300,
        status,
        text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
        json: () => Promise.resolve(body),
      })
    ))
  }

  function mockFetchError(error) {
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(error)))
  }

  it('starts unauthenticated with empty state', () => {
    expect(store.token).toBe('')
    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
    expect(store.providers).toEqual([])
    expect(store.allowSignup).toBe(true)
    expect(store.authDisabled).toBe(false)
  })

  it('isAuthenticated returns true when authDisabled is true', () => {
    store.authDisabled = true
    expect(store.isAuthenticated).toBe(true)
  })

  it('isAuthenticated returns true when token and user are set', () => {
    store.token = 'test-token'
    store.user = { name: 'Test' }
    expect(store.isAuthenticated).toBe(true)
  })

  it('authHeaders adds Bearer token when token is present', () => {
    store.token = 'my-token'
    const headers = store.authHeaders({ 'X-Custom': 'val' })
    expect(headers.Authorization).toBe('Bearer my-token')
    expect(headers['X-Custom']).toBe('val')
  })

  it('authHeaders returns original headers when token is empty', () => {
    const headers = store.authHeaders({ 'X-Custom': 'val' })
    expect(headers.Authorization).toBeUndefined()
    expect(headers['X-Custom']).toBe('val')
  })

  it('loadProviders fetches providers and sets state', async () => {
    mockFetch(200, { providers: ['google', 'github'], allowSignup: false, authDisabled: false })
    await store.loadProviders()
    expect(store.providers).toEqual(['google', 'github'])
    expect(store.allowSignup).toBe(false)
    expect(store.authDisabled).toBe(false)
  })

  it('loadProviders detects authDisabled from backend', async () => {
    mockFetch(200, { providers: [], allowSignup: false, authDisabled: true })
    await store.loadProviders()
    expect(store.authDisabled).toBe(true)
  })

  it('loadProviders falls back to secure defaults on network failure', async () => {
    mockFetchError(new Error('network error'))
    await store.loadProviders()
    expect(store.providers).toEqual([])
    expect(store.authDisabled).toBe(false)
  })

  it('login calls _post and adopts the response', async () => {
    mockFetch(200, { token: 'abc', user: { email: 'a@b.com' } })
    const user = await store.login('a@b.com', 'password123!')
    expect(user.email).toBe('a@b.com')
    expect(store.token).toBe('abc')
    expect(store.isAuthenticated).toBe(true)
  })

  it('login throws on server error', async () => {
    mockFetch(401, { error: 'Invalid credentials' })
    await expect(store.login('a@b.com', 'wrong')).rejects.toThrow('Invalid credentials')
  })

  it('register calls _post and adopts the response', async () => {
    mockFetch(200, { token: 'def', user: { email: 'c@d.com', name: 'Test' } })
    const user = await store.register('c@d.com', 'Test', 'password123!')
    expect(user.email).toBe('c@d.com')
    expect(store.token).toBe('def')
  })

  it('register throws on server error', async () => {
    mockFetch(400, { error: 'Email taken' })
    await expect(store.register('c@d.com', 'Test', 'pw')).rejects.toThrow('Email taken')
  })

  it('logout clears local state', async () => {
    mockFetch(200, {})
    store.token = 'abc'
    store.user = { email: 'a@b.com' }
    store.expiresAt = Date.now() / 1000 + 3600
    await store.logout()
    expect(store.token).toBe('')
    expect(store.user).toBeNull()
    expect(store.expiresAt).toBe(0)
    expect(store.isAuthenticated).toBe(false)
  })

  it('logout does not throw when server revoke fails', async () => {
    mockFetch(500, 'server error')
    store.token = 'abc'
    store.user = { email: 'a@b.com' }
    await expect(store.logout()).resolves.toBeUndefined()
    expect(store.token).toBe('')
  })

  it('restoreSession verifies token with /auth/me', async () => {
    mockFetch(200, { email: 'a@b.com' })
    store.token = 'valid-token'
    const ok = await store.restoreSession()
    expect(ok).toBe(true)
    expect(store.user).toEqual({ email: 'a@b.com' })
  })

  it('restoreSession clears state on 401', async () => {
    mockFetch(401, 'unauthorized')
    store.token = 'expired-token'
    store.user = { email: 'a@b.com' }
    const ok = await store.restoreSession()
    expect(ok).toBe(false)
    expect(store.token).toBe('')
    expect(store.user).toBeNull()
  })

  it('restoreSession returns false when no token exists', async () => {
    const ok = await store.restoreSession()
    expect(ok).toBe(false)
  })

  it('startOAuth returns authUrl and state', async () => {
    mockFetch(200, { authUrl: 'https://accounts.google.com/o/oauth2/auth?...', state: 'xyz123' })
    const r = await store.startOAuth('google')
    expect(r.authUrl).toContain('google.com')
    expect(r.state).toBe('xyz123')
  })

  it('pollOAuth returns done=true when status is ok', async () => {
    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ status: 'ok', token: 'abc', user: { email: 'a@b.com' } }),
        text: () => Promise.resolve(''),
      })
    ))
    const r = await store.pollOAuth('state-123')
    expect(r.done).toBe(true)
    expect(store.token).toBe('abc')
  })

  it('pollOAuth returns done=false when still pending', async () => {
    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ status: 'pending' }),
        text: () => Promise.resolve(''),
      })
    ))
    const r = await store.pollOAuth('state-123')
    expect(r.done).toBe(false)
  })

  it('pollOAuth throws on error status', async () => {
    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ status: 'error', error: 'OAuth failed' }),
        text: () => Promise.resolve(''),
      })
    ))
    await expect(store.pollOAuth('state-123')).rejects.toThrow('OAuth failed')
  })

  it('pollOAuth throws on HTTP error', async () => {
    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve({
        ok: false,
        status: 400,
        text: () => Promise.resolve('bad request'),
      })
    ))
    await expect(store.pollOAuth('state-123')).rejects.toThrow('bad request')
  })

  it('restores session from localStorage on construction', async () => {
    const future = Math.floor(Date.now() / 1000) + 86400
    memory['argus.auth.session'] = JSON.stringify({
      token: 'stored-token',
      user: { email: 'stored@b.com' },
      expiresAt: future,
    })
    vi.resetModules()
    setActivePinia(createPinia())
    const mod = await import('../auth')
    const s = mod.useAuthStore()
    expect(s.token).toBe('stored-token')
    expect(s.user).toEqual({ email: 'stored@b.com' })
  })

  it('lastUsedMethod is null on first construction', () => {
    expect(store.lastUsedMethod).toBeNull()
  })

  it('login records a local lastUsedMethod with the email', async () => {
    mockFetch(200, { token: 'abc', user: { email: 'a@b.com' } })
    await store.login('a@b.com', 'password123!')
    expect(store.lastUsedMethod).toBeTruthy()
    expect(store.lastUsedMethod.kind).toBe('local')
    expect(store.lastUsedMethod.email).toBe('a@b.com')
    expect(typeof store.lastUsedMethod.at).toBe('number')
  })

  it('register records a local lastUsedMethod with the email', async () => {
    mockFetch(200, { token: 'def', user: { email: 'c@d.com', name: 'Test' } })
    await store.register('c@d.com', 'Test', 'password123!')
    expect(store.lastUsedMethod.kind).toBe('local')
    expect(store.lastUsedMethod.email).toBe('c@d.com')
  })

  it('logout leaves lastUsedMethod intact', async () => {
    mockFetch(200, { token: 'abc', user: { email: 'a@b.com' } })
    await store.login('a@b.com', 'pw12345678!!')
    expect(store.lastUsedMethod).toBeTruthy()
    // Now logout — separate fetch mock for the revoke call.
    mockFetch(200, {})
    await store.logout()
    expect(store.token).toBe('')
    // Affinity stays — that's the whole point.
    expect(store.lastUsedMethod).toBeTruthy()
    expect(store.lastUsedMethod.kind).toBe('local')
  })

  it('pollOAuth records an oauth lastUsedMethod with the provider', async () => {
    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({
          status: 'ok',
          token: 'abc',
          user: { email: 'a@b.com' },
          provider: 'google',
        }),
        text: () => Promise.resolve(''),
      })
    ))
    const r = await store.pollOAuth('state-123')
    expect(r.done).toBe(true)
    expect(store.lastUsedMethod.kind).toBe('oauth')
    expect(store.lastUsedMethod.provider).toBe('google')
  })

  it('pollOAuth records kind=apple when provider is apple', async () => {
    vi.stubGlobal('fetch', vi.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({
          status: 'ok',
          token: 'abc',
          user: { email: 'apple@user.com' },
          provider: 'apple',
        }),
        text: () => Promise.resolve(''),
      })
    ))
    await store.pollOAuth('state-123')
    expect(store.lastUsedMethod.kind).toBe('apple')
    expect(store.lastUsedMethod.provider).toBe('apple')
    expect(store.lastUsedMethod.email).toBe('apple@user.com')
  })

  it('loads a recent lastUsedMethod from localStorage on construction', async () => {
    const nowSec = Math.floor(Date.now() / 1000)
    memory['argus.auth.lastMethod'] = JSON.stringify({
      kind: 'oauth', provider: 'google', email: null, at: nowSec - 100,
    })
    vi.resetModules()
    setActivePinia(createPinia())
    const mod = await import('../auth')
    const s = mod.useAuthStore()
    expect(s.lastUsedMethod).toBeTruthy()
    expect(s.lastUsedMethod.provider).toBe('google')
  })

  it('treats a 91+ day old lastUsedMethod as null', async () => {
    const nowSec = Math.floor(Date.now() / 1000)
    const ancient = nowSec - 91 * 24 * 60 * 60
    memory['argus.auth.lastMethod'] = JSON.stringify({
      kind: 'oauth', provider: 'google', email: null, at: ancient,
    })
    vi.resetModules()
    setActivePinia(createPinia())
    const mod = await import('../auth')
    const s = mod.useAuthStore()
    expect(s.lastUsedMethod).toBeNull()
  })

  it('ignores expired localStorage sessions', async () => {
    const past = Math.floor(Date.now() / 1000) - 3600
    memory['argus.auth.session'] = JSON.stringify({
      token: 'expired',
      user: { email: 'old@b.com' },
      expiresAt: past,
    })
    vi.resetModules()
    setActivePinia(createPinia())
    const mod = await import('../auth')
    const s = mod.useAuthStore()
    expect(s.token).toBe('')
    expect(s.user).toBeNull()
  })
})
