import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useAutoContext, __test } from '../useAutoContext'

// useAutoContext routes through callGo, which prefers the HTTP fallback
// when window.go is absent. Stubbing global fetch is the most realistic
// path: we exercise the same JSON shape the real backend returns.
function stubFetch(payload, { ok = true, throws } = {}) {
  if (throws) {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(throws))
    return
  }
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok,
    json: vi.fn().mockResolvedValue({ result: payload }),
  }))
}

describe('useAutoContext', () => {
  beforeEach(() => {
    __test.reset()
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('exposes resolution, error, loading, resolve, reprobe', () => {
    const c = useAutoContext()
    expect(c.resolution.value).toBeNull()
    expect(c.error.value).toBeNull()
    expect(c.loading.value).toBe(false)
    expect(typeof c.resolve).toBe('function')
    expect(typeof c.reprobe).toBe('function')
  })

  it('resolve() populates resolution from the backend', async () => {
    stubFetch({
      chosen: 'prod',
      confidence: 'active-reachable',
      probes: [
        { name: 'prod', reachable: true, active: true, latencyMs: 120, serverVersion: 'v1.29.5' },
        { name: 'stage', reachable: true, active: false, latencyMs: 240, serverVersion: 'v1.29.5' },
      ],
      reachableCount: 2,
    })
    const c = useAutoContext()
    const r = await c.resolve()
    expect(r.chosen).toBe('prod')
    expect(r.confidence).toBe('active-reachable')
    expect(c.resolution.value).toEqual(r)
    expect(c.loading.value).toBe(false)
    expect(c.error.value).toBeNull()
  })

  it('resolve() is idempotent — second call does not re-probe', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({
        result: { chosen: 'prod', confidence: 'active-reachable', probes: [], reachableCount: 1 },
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const c = useAutoContext()
    await c.resolve()
    await c.resolve()
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('reprobe() always hits the backend even after a previous resolve', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({
        result: { chosen: 'prod', confidence: 'active-reachable', probes: [], reachableCount: 1 },
      }),
    })
    vi.stubGlobal('fetch', fetchMock)

    const c = useAutoContext()
    await c.resolve()
    await c.reprobe()
    await c.reprobe()
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('resolve() captures the error and clears resolution on failure', async () => {
    stubFetch(null, { throws: new Error('boom') })
    const c = useAutoContext()
    const r = await c.resolve()
    expect(r).toBeNull()
    expect(c.resolution.value).toBeNull()
    expect(c.error.value).toContain('boom')
    expect(c.loading.value).toBe(false)
  })

  it('loading flips true during the call and false after', async () => {
    let resolveFetch
    vi.stubGlobal('fetch', vi.fn().mockImplementation(() => new Promise((res) => {
      resolveFetch = () => res({
        ok: true,
        json: vi.fn().mockResolvedValue({
          result: { chosen: 'prod', confidence: 'active-reachable', probes: [], reachableCount: 1 },
        }),
      })
    })))

    const c = useAutoContext()
    const p = c.resolve()
    // Sync after kicking off — should be loading.
    expect(c.loading.value).toBe(true)
    resolveFetch()
    await p
    expect(c.loading.value).toBe(false)
  })

  it('returns the no-reachable resolution unchanged when backend reports none', async () => {
    stubFetch({
      chosen: 'prod',
      confidence: 'active-unreachable',
      probes: [{ name: 'prod', reachable: false, active: true, error: 'i/o timeout' }],
      reachableCount: 0,
    })
    const c = useAutoContext()
    const r = await c.resolve()
    expect(r.confidence).toBe('active-unreachable')
    expect(r.reachableCount).toBe(0)
  })
})
