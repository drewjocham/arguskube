import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useArgusCD } from '../useArgusCD'
import { invalidateCachePrefix } from '../useBridge'

describe('useArgusCD', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('GetArgusCDStatus')
    invalidateCachePrefix('ListArgusCDApps')
    invalidateCachePrefix('ListArgusCDProjects')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns all expected refs and methods', () => {
    const result = useArgusCD()
    expect(result.apps.value).toEqual([])
    expect(result.loading.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(result.status.value).toBe(null)
    expect(typeof result.listApps).toBe('function')
    expect(typeof result.fetchStatus).toBe('function')
    expect(typeof result.syncApp).toBe('function')
    expect(typeof result.testConnection).toBe('function')
  })

  it('fetchStatus calls GetArgusCDStatus and sets status', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: { connected: true, url: 'https://argocd.example.com' } }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { status, fetchStatus } = useArgusCD()

    await fetchStatus()

    expect(status.value).toEqual({ connected: true, url: 'https://argocd.example.com' })
  })

  it('fetchStatus falls back on error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Backend down')))

    const { status, fetchStatus } = useArgusCD()

    await fetchStatus()

    expect(status.value).toBeTruthy()
    expect(status.value.connected).toBe(false)
  })

  it('listApps calls ListArgusCDApps and populates apps', async () => {
    const appData = [
      { name: 'guestbook', syncStatus: 'Synced', healthStatus: 'Healthy' },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: appData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { apps, loading, listApps } = useArgusCD()

    await listApps()

    expect(apps.value).toEqual(appData)
    expect(loading.value).toBe(false)
  })

  it('listApps handles errors and sets error and empty apps', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('API error')))

    const { apps, error, loading, listApps } = useArgusCD()

    await listApps()

    expect(apps.value).toEqual([])
    expect(error.value).toBeTruthy()
    expect(loading.value).toBe(false)
  })

  it('syncApp calls SyncArgusCDApp', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: { phase: 'Succeeded', message: 'synced' } }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { syncApp } = useArgusCD()

    const result = await syncApp('guestbook')

    expect(mockFetch.mock.calls[0][0]).toContain('/api/SyncArgusCDApp')
  })

  it('testConnection calls TestArgusCDConnection', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { testConnection } = useArgusCD()

    const result = await testConnection()

    expect(mockFetch.mock.calls[0][0]).toContain('/api/TestArgusCDConnection')
    expect(result.success).toBe(true)
  })

  it('listProjects calls ListArgusCDProjects and populates projects', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: ['default', 'platform'] }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { projects, listProjects } = useArgusCD()
    await listProjects()

    expect(mockFetch.mock.calls[0][0]).toContain('/api/ListArgusCDProjects')
    expect(projects.value).toEqual(['default', 'platform'])
  })

  it('listProjects falls back to an empty array on error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('boom')))
    const { projects, listProjects } = useArgusCD()
    await listProjects()
    expect(projects.value).toEqual([])
  })

  it('rollbackApp invokes RollbackArgusCDApp with the supplied id', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)
    const { rollbackApp } = useArgusCD()
    await rollbackApp('guestbook', 7)
    expect(mockFetch.mock.calls.some(call => String(call[0]).includes('/api/RollbackArgusCDApp'))).toBe(true)
  })
})
