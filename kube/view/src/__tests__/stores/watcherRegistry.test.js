import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useWatcherRegistryStore } from '../../stores/watcherRegistry'

// Patch localStorage with a working in-memory mock (jsdom in this repo
// ships without functional getItem/setItem).
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

const REGISTRY_KEY = 'kw-watcher-registry/v1'

function makeDescriptor(overrides = {}) {
  return {
    id: 'credential:github',
    label: 'GitHub PAT',
    kind: 'credential',
    intervalMs: 60_000,
    check: vi.fn().mockResolvedValue({ status: 'ok' }),
    configureAnchor: 'vault',
    enabled: true,
    ...overrides,
  }
}

describe('watcherRegistry store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const k of Object.keys(memory)) delete memory[k]
  })

  it('register() adds a descriptor and exposes it via list (sorted by label)', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor({ id: 'a', label: 'Zeta' }))
    r.register(makeDescriptor({ id: 'b', label: 'Alpha' }))
    expect(r.list.map((w) => w.id)).toEqual(['b', 'a'])
  })

  it('register() throws if id or check is missing', () => {
    const r = useWatcherRegistryStore()
    expect(() => r.register({})).toThrow()
    expect(() => r.register({ id: 'x' })).toThrow()
    expect(() => r.register({ id: 'x', check: 'not-a-fn' })).toThrow()
  })

  it('unregister() removes the descriptor; no-op when missing', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor())
    r.unregister('credential:github')
    expect(r.list).toHaveLength(0)
    r.unregister('does-not-exist') // no throw
  })

  it('effectiveDescriptor() returns null for unknown ids', () => {
    const r = useWatcherRegistryStore()
    expect(r.effectiveDescriptor('missing')).toBeNull()
  })

  it('setInterval() clamps to [60s, 24h] and persists the override', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor())
    r.setInterval('credential:github', 100)            // too small
    expect(r.effectiveDescriptor('credential:github').intervalMs).toBe(60_000)
    r.setInterval('credential:github', 9_999_999_999)  // too big
    expect(r.effectiveDescriptor('credential:github').intervalMs).toBe(24 * 60 * 60 * 1000)
    // Persisted to localStorage.
    const raw = localStorage.getItem(REGISTRY_KEY)
    const parsed = JSON.parse(raw)
    expect(parsed['credential:github'].intervalMs).toBe(24 * 60 * 60 * 1000)
  })

  it('setInterval() handles non-finite values by clamping to MIN', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor())
    r.setInterval('credential:github', NaN)
    expect(r.effectiveDescriptor('credential:github').intervalMs).toBe(60_000)
  })

  it('setEnabled() flips the override and is observable via effectiveDescriptor', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor({ enabled: true }))
    r.setEnabled('credential:github', false)
    expect(r.effectiveDescriptor('credential:github').enabled).toBe(false)
  })

  it('setEnabled() / setInterval() no-op for unknown ids', () => {
    const r = useWatcherRegistryStore()
    r.setEnabled('x', false)
    r.setInterval('x', 60_000)
    // Without the descriptor we can't observe the override; just ensure no throw.
    expect(true).toBe(true)
  })

  it('overrides survive a fresh pinia load via localStorage', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor())
    r.setEnabled('credential:github', false)

    // Simulate a reload by creating a fresh pinia and re-registering the
    // descriptor (functions don't serialize, so registrations happen at
    // boot time on every page load).
    setActivePinia(createPinia())
    const r2 = useWatcherRegistryStore()
    r2.register(makeDescriptor())
    expect(r2.effectiveDescriptor('credential:github').enabled).toBe(false)
  })

  it('recordResult() updates results + lastCheckedAt', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor())
    const before = Date.now()
    r.recordResult('credential:github', { status: 'ok', message: 'green' })
    expect(r.results['credential:github']).toEqual({ status: 'ok', message: 'green' })
    expect(r.lastCheckedAt['credential:github']).toBeGreaterThanOrEqual(before)
  })

  it('dueAt() returns lastCheckedAt + intervalMs for known ids', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor({ intervalMs: 60_000 }))
    r.recordResult('credential:github', { status: 'ok' })
    const due = r.dueAt('credential:github')
    expect(due - r.lastCheckedAt['credential:github']).toBe(60_000)
  })

  it('dueAt() returns Infinity for unknown ids (so the loop skips them)', () => {
    const r = useWatcherRegistryStore()
    expect(r.dueAt('nope')).toBe(Number.POSITIVE_INFINITY)
  })

  it('snapshotForArgus() exposes a stable shape including lastResult/lastCheckedAt', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor())
    r.recordResult('credential:github', { status: 'ok', message: 'green' })
    const snap = r.snapshotForArgus()
    expect(Array.isArray(snap)).toBe(true)
    expect(snap[0]).toMatchObject({
      id: 'credential:github',
      label: 'GitHub PAT',
      kind: 'credential',
      enabled: true,
      configureAnchor: 'vault',
      lastResult: { status: 'ok', message: 'green' },
    })
    expect(snap[0].lastCheckedAt).toMatch(/T/) // ISO string
  })

  it('snapshotForArgus() returns null lastResult / lastCheckedAt when never checked', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor())
    const snap = r.snapshotForArgus()
    expect(snap[0].lastResult).toBeNull()
    expect(snap[0].lastCheckedAt).toBeNull()
  })

  it('register() overwrites an existing descriptor with the same id (e.g. label change)', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor({ label: 'GitHub PAT v1' }))
    r.register(makeDescriptor({ label: 'GitHub PAT v2' }))
    expect(r.list).toHaveLength(1)
    expect(r.list[0].label).toBe('GitHub PAT v2')
  })

  it('effectiveDescriptor falls back to base.enabled when no override is set', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor({ enabled: false })) // explicit base off
    expect(r.effectiveDescriptor('credential:github').enabled).toBe(false)
  })

  it('list filters out null effective descriptors gracefully', () => {
    const r = useWatcherRegistryStore()
    r.register(makeDescriptor())
    // Mutate watchers ref to insert a bogus entry; effectiveDescriptor
    // would return null which list() must drop.
    r.watchers.bogus = null
    expect(r.list.find((w) => !w)).toBeUndefined()
  })
})
