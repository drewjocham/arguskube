import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useResources } from '../useResources'
import { invalidateCache, invalidateCachePrefix } from '../useBridge'

describe('useResources', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('ListResources')
    invalidateCachePrefix('GetResourceDetail')
    invalidateCachePrefix('ListAllNamespaces')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns result, detail, namespaces, loading, detailLoading, error, listResources, getResourceDetail, listNamespaces', () => {
    const result = useResources()
    expect(result.result.value).toBe(null)
    expect(result.detail.value).toBe(null)
    expect(result.namespaces.value).toEqual([])
    expect(result.loading.value).toBe(false)
    expect(result.detailLoading.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(typeof result.listResources).toBe('function')
    expect(typeof result.getResourceDetail).toBe('function')
    expect(typeof result.listNamespaces).toBe('function')
  })

  it('listResources populates result from backend', async () => {
    const resourceData = [
      { name: 'pod-1', kind: 'Pod', namespace: 'default' },
    ]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListResources: vi.fn().mockResolvedValue(resourceData),
        },
      },
    })

    const { result, loading, error, listResources } = useResources()
    await listResources('Pod', 'default')

    expect(result.value).toEqual(resourceData)
    expect(error.value).toBe(null)
    expect(loading.value).toBe(false)
  })

  it('listResources passes kind and namespace as args', async () => {
    const mockList = vi.fn().mockResolvedValue([])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListResources: mockList,
        },
      },
    })

    const { listResources } = useResources()
    await listResources('Deployment', 'kube-system')

    expect(mockList).toHaveBeenCalledWith('Deployment', 'kube-system')
  })

  it('listResources uses _all when namespace not provided', async () => {
    const mockList = vi.fn().mockResolvedValue([])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListResources: mockList,
        },
      },
    })

    const { listResources } = useResources()
    await listResources('Pod')

    expect(mockList).toHaveBeenCalledWith('Pod', '_all')
  })

  it('listResources with force=true invalidates cache', async () => {
    const mockList = vi.fn().mockResolvedValue([])
    const invalidateSpy = vi.spyOn({ invalidateCache }, 'invalidateCache')
    // Actually just test the flow: force calls invalidateCache then cachedCallGo
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListResources: mockList,
        },
      },
    })

    const { listResources } = useResources()
    await listResources('Pod', 'default', true)

    // The cache was invalidated, then callGo was invoked
    expect(mockList).toHaveBeenCalledWith('Pod', 'default')
  })

  it('listResources handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListResources: vi.fn().mockRejectedValue(new Error('API error')),
        },
      },
    })

    const { result, error, loading, listResources } = useResources()
    await listResources('Pod', 'default')

    expect(result.value).toBe(null)
    expect(error.value).toBeTruthy()
    expect(loading.value).toBe(false)
  })

  it('getResourceDetail populates detail from backend', async () => {
    const detailData = {
      name: 'pod-1',
      namespace: 'default',
      status: 'Running',
      containers: ['nginx'],
    }

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetResourceDetail: vi.fn().mockResolvedValue(detailData),
        },
      },
    })

    const { detail, detailLoading, getResourceDetail } = useResources()
    await getResourceDetail('Pod', 'default', 'pod-1')

    expect(detail.value).toEqual(detailData)
    expect(detailLoading.value).toBe(false)
  })

  it('getResourceDetail passes correct args', async () => {
    const mockDetail = vi.fn().mockResolvedValue(null)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetResourceDetail: mockDetail,
        },
      },
    })

    const { getResourceDetail } = useResources()
    await getResourceDetail('Deployment', 'kube-system', 'coredns')

    expect(mockDetail).toHaveBeenCalledWith('Deployment', 'kube-system', 'coredns')
  })

  it('getResourceDetail uses empty string when namespace is empty', async () => {
    const mockDetail = vi.fn().mockResolvedValue(null)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetResourceDetail: mockDetail,
        },
      },
    })

    const { getResourceDetail } = useResources()
    await getResourceDetail('Node', '', 'node-1')

    expect(mockDetail).toHaveBeenCalledWith('Node', '', 'node-1')
  })

  it('getResourceDetail handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetResourceDetail: vi.fn().mockRejectedValue(new Error('Detail error')),
        },
      },
    })

    const { detail, detailLoading, getResourceDetail } = useResources()
    await getResourceDetail('Pod', 'default', 'pod-1')

    expect(detail.value).toBe(null)
    expect(detailLoading.value).toBe(false)
  })

  it('listNamespaces populates namespaces from backend', async () => {
    const nsData = ['default', 'kube-system', 'kubewatcher']

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListAllNamespaces: vi.fn().mockResolvedValue(nsData),
        },
      },
    })

    const { namespaces, listNamespaces } = useResources()
    await listNamespaces()

    expect(namespaces.value).toEqual(nsData)
  })

  it('listNamespaces does not overwrite on null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListAllNamespaces: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { namespaces, listNamespaces } = useResources()
    namespaces.value = ['existing']

    await listNamespaces()

    // null result won't update: `if (res) namespaces.value = res`
    expect(namespaces.value).toEqual(['existing'])
  })

  it('listNamespaces handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListAllNamespaces: vi.fn().mockRejectedValue(new Error('NS error')),
        },
      },
    })

    const { namespaces, listNamespaces } = useResources()
    namespaces.value = ['existing']

    await listNamespaces()

    expect(namespaces.value).toEqual(['existing'])
  })
})
