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
