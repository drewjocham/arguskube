import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useTopology } from '../useCluster'

import { invalidateCachePrefix } from '../useBridge'

describe('useTopology', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('GetTopology')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns topology, loading, error, fetchTopology', () => {
    const { topology, loading, error, fetchTopology } = useTopology()
    expect(topology.value).toBe(null)
    expect(loading.value).toBe(false)
    expect(error.value).toBe(null)
    expect(typeof fetchTopology).toBe('function')
  })

  it('fetchTopology populates topology and sets loading false', async () => {
    const topologyData = {
      nodes: [
        { id: 'node-1', label: 'k3s-control-plane', type: 'control-plane' },
        { id: 'node-2', label: 'k3s-worker-1', type: 'worker' },
      ],
      edges: [
        { source: 'node-1', target: 'node-2' },
      ],
    }
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: topologyData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { topology, loading, fetchTopology } = useTopology()

    await fetchTopology()

    expect(topology.value).toEqual(topologyData)
    expect(loading.value).toBe(false)
  })

  it('fetchTopology handles error gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Topology unavailable')))

    const { topology, loading, fetchTopology } = useTopology()

    await fetchTopology()

    expect(topology.value).toBe(null)
    expect(loading.value).toBe(false)
  })

  it('fetchTopology passes namespace argument', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { fetchTopology } = useTopology()

    await fetchTopology('kube-system')

    const body = JSON.parse(mockFetch.mock.calls[0][1].body)
    expect(body.args[0]).toBe('kube-system')
  })
})
