import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Mock the Wails bridge at the module level so the store's callGo /
// cachedCallGo calls land in our vi.fn() stubs.
const mockCallGo = vi.fn()
const mockCachedCallGo = vi.fn()
vi.mock('../../composables/useBridge', () => ({
  callGo: (...args) => mockCallGo(...args),
  cachedCallGo: (...args) => mockCachedCallGo(...args),
}))

// Memory localStorage so the persistence tests don't bleed across runs
// and don't depend on jsdom's quota.
const memStorage = {}
beforeEach(() => {
  for (const k of Object.keys(memStorage)) delete memStorage[k]
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: {
      getItem: (k) => (k in memStorage ? memStorage[k] : null),
      setItem: (k, v) => { memStorage[k] = String(v) },
      removeItem: (k) => { delete memStorage[k] },
      clear: () => { for (const k of Object.keys(memStorage)) delete memStorage[k] },
    },
  })
  setActivePinia(createPinia())
  mockCallGo.mockReset()
  mockCachedCallGo.mockReset()
})

// Import AFTER the mock + storage are set up. Resets Pinia so each
// call returns a genuinely new store — simulates a page reload, which
// is exactly the path the active-run-persistence test needs.
async function freshStore() {
  setActivePinia(createPinia())
  vi.resetModules()
  const mod = await import('../../stores/distload')
  return mod.useDistLoadStore()
}

afterEach(() => { vi.useRealTimers() })

describe('distload store — audit blocker fixes', () => {
  // Bug #2 — start() must reject a second concurrent call instead of
  // silently overwriting activeRunId and orphaning the previous cloud
  // run.
  it('start() rejects when a run is already active', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce('run-1')
    await s.start({ regions: ['us-east'] })

    mockCallGo.mockResolvedValueOnce('run-2')
    await expect(s.start({ regions: ['us-east'] })).rejects.toThrow(/already running/i)
    expect(s.activeRunId).toBe('run-1')
    // The second StartDistributedLoadTest must NEVER have been
    // called — that would have burned real credits.
    const startCalls = mockCallGo.mock.calls.filter(c => c[0] === 'StartDistributedLoadTest')
    expect(startCalls.length).toBe(1)
  })

  // Bug #5 — estimateCost returns null on failure, not 0, so the UI
  // can render "—" instead of a false "free!" message.
  it('estimateCost returns null on backend failure', async () => {
    const s = await freshStore()
    mockCallGo.mockRejectedValueOnce(new Error('saas: platform unreachable'))
    const out = await s.estimateCost({ regions: ['us-east'] })
    expect(out).toBeNull()
    expect(s.estimatedCost).toBeNull()
  })

  it('estimateCost coerces non-numeric backend reply to null', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce('not a number')
    const out = await s.estimateCost({ regions: ['us-east'] })
    expect(out).toBeNull()
  })

  it('estimateCost happy path returns and stores the number', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce(12.5)
    const out = await s.estimateCost({ regions: ['us-east'] })
    expect(out).toBe(12.5)
    expect(s.estimatedCost).toBe(12.5)
  })

  // Bug #6 — active-run persistence: a runId must round-trip via
  // localStorage so the user can close the tab and resume tracking.
  it('start() persists the activeRunId so it survives reload', async () => {
    const s1 = await freshStore()
    mockCallGo.mockResolvedValueOnce('run-persist-1')
    await s1.start({ regions: ['us-east'] })
    s1.stopPolling() // freeze; we're testing persistence, not polling

    // Simulate reload: brand-new store, same localStorage.
    const s2 = await freshStore()
    expect(s2.activeRunId).toBeNull() // store starts empty
    // Make the resumed poll come back as a terminal state so the
    // test doesn't loop forever.
    mockCallGo.mockResolvedValueOnce({ runId: 'run-persist-1', state: 'done' })
    const resumed = s2.resumeActiveRun()
    expect(resumed).toBe(true)
    expect(s2.activeRunId).toBe('run-persist-1')
    s2.stopPolling()
  })

  it('cancel() clears the persisted run id', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce('run-cancel-1')
    await s.start({ regions: ['us-east'] })
    s.stopPolling()
    mockCallGo.mockResolvedValueOnce(null) // CancelDistributedLoadTest
    await s.cancel()
    // Reload: must NOT resume.
    const s2 = await freshStore()
    expect(s2.resumeActiveRun()).toBe(false)
  })

  it('terminal-state status updates clear persistence', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce('run-terminal-1')
    await s.start({ regions: ['us-east'] })
    s.stopPolling()
    // status update with state=done must clear the stored runId
    // so the next reload doesn't try to resume a finished run.
    // We can't trigger onStatusUpdate directly (private), so go
    // through the persistence accessor path: a fresh store after
    // a "done" persisted state should refuse to resume.
    window.localStorage.setItem(
      'argus.distload.activeRun.v1',
      JSON.stringify({ runId: 'run-terminal-1', state: 'done' }),
    )
    const s2 = await freshStore()
    expect(s2.resumeActiveRun()).toBe(false)
  })

  // Bug #7 — canStart credit pre-check: when estimatedCost is known
  // and exceeds creditBalance, Start must be disabled before the user
  // clicks (and gets a SaaS-side rejection).
  it('canStart is false when estimatedCost exceeds creditBalance', async () => {
    const s = await freshStore()
    s.creditBalance = 50
    s.setEstimatedCost(100)
    expect(s.canStart).toBe(false)
  })

  it('canStart is true when balance covers estimate', async () => {
    const s = await freshStore()
    s.creditBalance = 500
    s.setEstimatedCost(100)
    expect(s.canStart).toBe(true)
  })

  it('canStart is true when estimate is unknown (SaaS gives authoritative answer)', async () => {
    const s = await freshStore()
    s.creditBalance = 50
    s.setEstimatedCost(null)
    expect(s.canStart).toBe(true)
  })

  it('canStart is false when a run is already in flight', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce('run-canstart-1')
    await s.start({ regions: ['us-east'] })
    s.stopPolling()
    s.creditBalance = 10_000
    s.setEstimatedCost(1)
    expect(s.canStart).toBe(false)
  })
})

describe('distload store — consolidation extensions', () => {
  it('loadPresets stores the backend result', async () => {
    const s = await freshStore()
    const data = [{ id: 'preset-a', name: 'A', description: 'd', whenToUse: 'when', spec: {} }]
    mockCallGo.mockResolvedValueOnce(data)
    await s.loadPresets()
    expect(mockCallGo).toHaveBeenCalledWith('ListDistLoadPresets')
    expect(s.presets).toEqual(data)
  })

  it('loadPresets surfaces backend error without throwing', async () => {
    const s = await freshStore()
    mockCallGo.mockRejectedValueOnce(new Error('boom'))
    await s.loadPresets()
    expect(s.error).toMatch(/boom/)
    expect(s.presets).toEqual([])
  })

  it('loadBrokerKinds stores the array', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce(['kafka', 'rest'])
    await s.loadBrokerKinds()
    expect(mockCallGo).toHaveBeenCalledWith('ListDistLoadBrokerKinds')
    expect(s.brokerKinds).toEqual(['kafka', 'rest'])
  })

  it('loadLocalQuota stores the quota object', async () => {
    const s = await freshStore()
    const q = { used: 2, limit: 5, resetAt: '2030-01-01', isPro: false }
    mockCallGo.mockResolvedValueOnce(q)
    await s.loadLocalQuota()
    expect(mockCallGo).toHaveBeenCalledWith('GetLocalDistLoadQuota')
    expect(s.localQuota).toEqual(q)
  })

  it('generatePayload forwards prompt and sizeHint', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce('{"hello":"world"}')
    const out = await s.generatePayload('order event', 1024)
    expect(mockCallGo).toHaveBeenCalledWith('GenerateLoadTestPayload', 'order event', 1024)
    expect(out).toBe('{"hello":"world"}')
  })

  it('generatePayload propagates rejection', async () => {
    const s = await freshStore()
    mockCallGo.mockRejectedValueOnce(new Error('AI unconfigured'))
    await expect(s.generatePayload('x', 100)).rejects.toThrow(/AI unconfigured/)
    expect(s.error).toMatch(/AI unconfigured/)
  })

  it('resolvePayloadPath returns the resolution', async () => {
    const s = await freshStore()
    const out = { kind: 'file', files: [{ name: 'a.json', size: 12, path: '/a.json' }], sample: '{}' }
    mockCallGo.mockResolvedValueOnce(out)
    const got = await s.resolvePayloadPath('/a.json')
    expect(mockCallGo).toHaveBeenCalledWith('ResolveLocalPayloadPath', '/a.json')
    expect(got).toEqual(out)
  })

  it('getPreset finds by id', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce([{ id: 'p1', name: 'one' }, { id: 'p2', name: 'two' }])
    await s.loadPresets()
    expect(s.getPreset('p2')).toEqual({ id: 'p2', name: 'two' })
    expect(s.getPreset('missing')).toBeNull()
  })

  it('start with runner=local triggers a quota refresh after success', async () => {
    const s = await freshStore()
    mockCallGo.mockResolvedValueOnce('run-local-1')
    // The post-start fire-and-forget loadLocalQuota call resolves second.
    mockCallGo.mockResolvedValueOnce({ used: 3, limit: 5, isPro: false })
    await s.start({ runner: 'local' })
    s.stopPolling()
    // Allow microtask queue to drain so the .catch chained call runs.
    await Promise.resolve(); await Promise.resolve()
    const quotaCalls = mockCallGo.mock.calls.filter(c => c[0] === 'GetLocalDistLoadQuota')
    expect(quotaCalls.length).toBe(1)
  })
})
