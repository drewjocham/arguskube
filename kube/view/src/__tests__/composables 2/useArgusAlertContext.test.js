import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { useArgusAlertContext } from '../../composables/useArgusAlertContext'
import { useNotificationGuardStore } from '../../stores/notificationGuard'
import { useWatcherRegistryStore } from '../../stores/watcherRegistry'

// Mock the engine helpers — runWatcher / runDueNow are imported by the
// composable and would otherwise execute real probes.
vi.mock('../../composables/useWatcherEngine', () => ({
  runDueNow: vi.fn().mockResolvedValue(undefined),
  runWatcherById: vi.fn().mockResolvedValue({ status: 'ok', message: 'fine' }),
}))
import { runDueNow, runWatcherById } from '../../composables/useWatcherEngine'

// Local-storage shim (jsdom in this repo doesn't expose functional methods)
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

// A tiny host component that mounts the composable so onMounted runs and
// the global handle gets attached. Unmounting it detaches.
const Host = defineComponent({
  setup() {
    useArgusAlertContext()
    return () => h('div')
  },
})

describe('useArgusAlertContext', () => {
  let guard, registry
  beforeEach(() => {
    setActivePinia(createPinia())
    guard = useNotificationGuardStore()
    registry = useWatcherRegistryStore()
    runDueNow.mockClear()
    runWatcherById.mockClear()
    // Make sure no stale handle persists between tests.
    delete window.argusAlertContext
  })
  afterEach(() => {
    delete window.argusAlertContext
  })

  function mountHost() {
    return mount(Host, { attachTo: document.body })
  }

  it('attaches window.argusAlertContext on mount and detaches on unmount', () => {
    expect(window.argusAlertContext).toBeUndefined()
    const w = mountHost()
    expect(window.argusAlertContext).toBeDefined()
    expect(typeof window.argusAlertContext.read).toBe('function')
    expect(window.argusAlertContext.schemaVersion).toBe(1)
    w.unmount()
    expect(window.argusAlertContext).toBeUndefined()
  })

  it('read() returns a JSON-friendly snapshot of watchers + silences + settings', () => {
    registry.register({
      id: 'cred:gh', label: 'GitHub PAT', kind: 'credential',
      intervalMs: 60_000,
      check: async () => ({ status: 'ok' }),
      configureAnchor: 'vault',
    })
    guard.silence('cred:gh', 60_000, { label: 'GitHub PAT' })

    mountHost()
    const snap = window.argusAlertContext.read()
    expect(snap).toHaveProperty('schemaVersion', 1)
    expect(snap).toHaveProperty('generatedAt')
    expect(Array.isArray(snap.watchers)).toBe(true)
    expect(snap.watchers[0].id).toBe('cred:gh')
    expect(Array.isArray(snap.silences)).toBe(true)
    expect(snap.silences[0].source).toBe('cred:gh')
    expect(snap.settings).toMatchObject({ spamThreshold: expect.any(Number) })
  })

  it('silence("src") creates a silence with reason="argus" and the default duration', () => {
    registry.register({
      id: 'cred:gh', label: 'GitHub PAT', kind: 'credential',
      intervalMs: 60_000,
      check: async () => ({ status: 'ok' }),
      configureAnchor: 'vault',
    })
    mountHost()
    const res = window.argusAlertContext.silence('cred:gh')
    expect(res.ok).toBe(true)
    expect(res.source).toBe('cred:gh')
    expect(guard.silences['cred:gh'].reason).toBe('argus')
  })

  it('silence("src", duration) clamps the duration to the 24h hard cap', () => {
    mountHost()
    const FAR_FUTURE = 999 * 24 * 3600 * 1000
    const res = window.argusAlertContext.silence('x', FAR_FUTURE)
    expect(res.ok).toBe(true)
    expect(res.silencedFor).toBeLessThanOrEqual(24 * 60 * 60 * 1000)
  })

  it('silence("src", duration) clamps the duration to at least 60s', () => {
    mountHost()
    const res = window.argusAlertContext.silence('x', 100)
    expect(res.silencedFor).toBeGreaterThanOrEqual(60_000)
  })

  it('silence() with no source returns { ok: false }', () => {
    mountHost()
    expect(window.argusAlertContext.silence('').ok).toBe(false)
  })

  it('unsilence() removes an active silence', () => {
    guard.silence('cred:gh', 60_000, { label: 'GH' })
    mountHost()
    const res = window.argusAlertContext.unsilence('cred:gh')
    expect(res.ok).toBe(true)
    expect(guard.silences['cred:gh']).toBeUndefined()
  })

  it('unsilence() returns { ok: false } if no active silence', () => {
    mountHost()
    const res = window.argusAlertContext.unsilence('nope')
    expect(res.ok).toBe(false)
    expect(res.error).toMatch(/no active silence/)
  })

  it('unsilence() with no source returns { ok: false }', () => {
    mountHost()
    expect(window.argusAlertContext.unsilence('').ok).toBe(false)
  })

  it('runWatcher(id) calls runWatcherById from the engine module', async () => {
    mountHost()
    const res = await window.argusAlertContext.runWatcher('cred:gh')
    expect(runWatcherById).toHaveBeenCalledWith('cred:gh')
    expect(res.ok).toBe(true)
    expect(res.id).toBe('cred:gh')
  })

  it('runWatcher() with no id returns { ok: false }', async () => {
    mountHost()
    expect((await window.argusAlertContext.runWatcher('')).ok).toBe(false)
  })

  it('runDueNow() calls the engine with force:true', async () => {
    mountHost()
    const res = await window.argusAlertContext.runDueNow()
    expect(runDueNow).toHaveBeenCalledWith({ force: true })
    expect(res.ok).toBe(true)
  })

  it('detach does not remove a handle owned by another schema version', () => {
    const w = mountHost()
    // Simulate a foreign attach overwriting our handle (e.g. a hot-reload
    // race or a future schema version coming online concurrently).
    window.argusAlertContext = { schemaVersion: 999 }
    w.unmount()
    // The detach guard only nukes its own schemaVersion — the foreign
    // handle should still be present after our unmount.
    expect(window.argusAlertContext).toBeDefined()
    expect(window.argusAlertContext.schemaVersion).toBe(999)
  })
})
