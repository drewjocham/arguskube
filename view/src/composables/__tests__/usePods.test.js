import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { usePods, useDeploymentRevisions, useVPARecommendations } from '../usePods'
import { invalidateCachePrefix } from '../useBridge'

describe('usePods', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('ListResources')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns pods, loading, error, listPods, getPodLogs', () => {
    const { pods, loading, error, listPods, getPodLogs } = usePods(10000)
    expect(pods.value).toEqual([])
    expect(loading.value).toBe(true)
    expect(error.value).toBe(null)
    expect(typeof listPods).toBe('function')
    expect(typeof getPodLogs).toBe('function')
  })

  it('listPods populates pods from API', async () => {
    const podsData = [
      { name: 'nginx-abc123', namespace: 'default', status: 'Running' },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: podsData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { pods, listPods } = usePods(10000)

    await listPods('default')

    expect(mockFetch.mock.calls[0][0]).toContain('/api/ListResources')
    const body = JSON.parse(mockFetch.mock.calls[0][1].body)
    expect(body.args).toEqual(['pods', 'default'])
    expect(pods.value).toEqual(podsData)
  })

  it('listPods handles errors gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('API error')))

    const { pods, error, listPods } = usePods(10000)

    await listPods('default')

    expect(error.value).toBeTruthy()
    expect(pods.value).toEqual([])
  })

  it('getPodLogs calls GetPodLogs and returns logs', async () => {
    const logData = [
      { timestamp: '2024-01-01T00:00:00Z', message: 'Server started' },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: logData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { getPodLogs } = usePods(10000)

    const result = await getPodLogs('default', 'nginx-abc123', 50)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/GetPodLogs')
    expect(result).toEqual(logData)
  })

  it('getPodLogs returns null on error', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Logs error')))

    const { getPodLogs } = usePods(10000)

    const result = await getPodLogs('default', 'nginx-abc123')

    expect(result).toBe(null)
  })
})

describe('useDeploymentRevisions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns revisions, loading, error, fetchRevisions', () => {
    const { revisions, loading, error, fetchRevisions } = useDeploymentRevisions()
    expect(revisions.value).toEqual([])
    expect(loading.value).toBe(false)
    expect(error.value).toBe(null)
    expect(typeof fetchRevisions).toBe('function')
  })

  it('fetchRevisions calls GetDeploymentRevisions and populates revisions', async () => {
    const revData = [
      { revision: 1, deployed: '2024-01-01T00:00:00Z' },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: revData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { revisions, fetchRevisions } = useDeploymentRevisions()

    await fetchRevisions('default', 'nginx', 25)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/GetDeploymentRevisions')
    expect(revisions.value).toEqual(revData)
  })

  it('fetchRevisions handles errors gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Revisions unavailable')))

    const { revisions, error, fetchRevisions } = useDeploymentRevisions()

    await fetchRevisions('default', 'nginx')

    expect(error.value).toBeTruthy()
    expect(revisions.value).toEqual([])
  })
})

describe('useVPARecommendations', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('GetVPARecommendations')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns vpas, loading, error, fetchVPAs', () => {
    const { vpas, loading, error, fetchVPAs } = useVPARecommendations()
    expect(vpas.value).toEqual([])
    expect(loading.value).toBe(false)
    expect(error.value).toBe(null)
    expect(typeof fetchVPAs).toBe('function')
  })

  it('fetchVPAs calls GetVPARecommendations and populates vpas', async () => {
    const vpaData = [
      { name: 'nginx-vpa', target: { cpu: '250m', memory: '512Mi' } },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: vpaData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { vpas, fetchVPAs } = useVPARecommendations()

    await fetchVPAs('default')

    expect(mockFetch.mock.calls[0][0]).toContain('/api/GetVPARecommendations')
    expect(vpas.value).toEqual(vpaData)
  })

  it('fetchVPAs handles errors gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('VPA error')))

    const { vpas, error, fetchVPAs } = useVPARecommendations()

    await fetchVPAs('default')

    expect(error.value).toBeTruthy()
    expect(vpas.value).toEqual([])
  })
})
