import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { runDueNow, runWatcherById } from '../../composables/useWatcherEngine'
import { useWatcherRegistryStore } from '../../stores/watcherRegistry'
import { useNotificationGuardStore } from '../../stores/notificationGuard'

// Local-storage shim
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

function registerWatcher(reg, id, opts = {}) {
  reg.register({
    id, label: id, kind: 'credential', intervalMs: opts.intervalMs ?? 60_000,
    check: opts.check ?? vi.fn().mockResolvedValue({ status: 'ok', message: 'fine' }),
    configureAnchor: 'vault',
    enabled: opts.enabled !== false,
  })
}

describe('useWatcherEngine module-level helpers', () => {
  let registry, guard
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const k of Object.keys(memory)) delete memory[k]
    registry = useWatcherRegistryStore()
    guard = useNotificationGuardStore()
  })

  // ---- runWatcherById ---------------------------------------------------

  it('runWatcherById returns null for unknown ids', async () => {
    const result = await runWatcherById('missing')
    expect(result).toBeNull()
  })

  it('runWatcherById skips disabled watchers', async () => {
    const check = vi.fn()
    registerWatcher(registry, 'a', { enabled: false, check })
    const result = await runWatcherById('a')
    expect(result).toBeNull()
    expect(check).not.toHaveBeenCalled()
  })

  it('runWatcherById invokes check(), records the result, and notifies the guard', async () => {
    const check = vi.fn().mockResolvedValue({ status: 'expired', message: 'token died' })
    registerWatcher(registry, 'a', { check })
    const result = await runWatcherById('a')
    expect(check).toHaveBeenCalledTimes(1)
    expect(result.status).toBe('expired')
    expect(registry.results.a).toEqual({ status: 'expired', message: 'token died' })
    // The guard should have observed it and emitted an alert.
    expect(guard.sources.a).toBeTruthy()
    expect(guard.sources.a.lastStatus).toBe('expired')
  })

  it('runWatcherById coerces a thrown error into a {status:"error"} result', async () => {
    const check = vi.fn().mockRejectedValue(new Error('boom'))
    registerWatcher(registry, 'a', { check })
    const result = await runWatcherById('a')
    expect(result).toEqual({ status: 'error', message: 'boom' })
    expect(registry.results.a.status).toBe('error')
  })

  it('runWatcherById coerces a null/invalid result into {status:"error"}', async () => {
    const check = vi.fn().mockResolvedValue(null)
    registerWatcher(registry, 'a', { check })
    const result = await runWatcherById('a')
    expect(result.status).toBe('error')
  })

  // ---- runDueNow --------------------------------------------------------

  it('runDueNow runs every enabled, due watcher exactly once', async () => {
    const c1 = vi.fn().mockResolvedValue({ status: 'ok' })
    const c2 = vi.fn().mockResolvedValue({ status: 'ok' })
    registerWatcher(registry, 'a', { check: c1 })
    registerWatcher(registry, 'b', { check: c2 })
    await runDueNow({ force: true })
    expect(c1).toHaveBeenCalledTimes(1)
    expect(c2).toHaveBeenCalledTimes(1)
  })

  it('runDueNow respects per-watcher dueAt without force', async () => {
    const c1 = vi.fn().mockResolvedValue({ status: 'ok' })
    registerWatcher(registry, 'a', { intervalMs: 10 * 60_000, check: c1 })
    // Pretend we checked it 1 sec ago — still inside its 10 min window.
    registry.recordResult('a', { status: 'ok' })
    await runDueNow() // not force
    expect(c1).not.toHaveBeenCalled()
  })

  it('runDueNow with force=true ignores dueAt and re-runs everything', async () => {
    const c1 = vi.fn().mockResolvedValue({ status: 'ok' })
    registerWatcher(registry, 'a', { intervalMs: 10 * 60_000, check: c1 })
    registry.recordResult('a', { status: 'ok' })
    await runDueNow({ force: true })
    expect(c1).toHaveBeenCalledTimes(1)
  })

  it('runDueNow skips disabled watchers', async () => {
    const c1 = vi.fn().mockResolvedValue({ status: 'ok' })
    registerWatcher(registry, 'a', { enabled: false, check: c1 })
    await runDueNow({ force: true })
    expect(c1).not.toHaveBeenCalled()
  })

  it('runDueNow does not re-enter while already in flight', async () => {
    // c1 takes a tick to resolve; the second runDueNow call should short-circuit.
    let resolveFirst
    const slow = new Promise((r) => { resolveFirst = r })
    const c1 = vi.fn().mockImplementation(() => slow.then(() => ({ status: 'ok' })))
    registerWatcher(registry, 'a', { check: c1 })

    const p1 = runDueNow({ force: true })
    const p2 = runDueNow({ force: true })
    // Resolve the first watcher's check; both runs should be finished now.
    resolveFirst()
    await p1
    await p2
    expect(c1).toHaveBeenCalledTimes(1)
  })
})
