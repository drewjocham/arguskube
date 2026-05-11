import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useServicePods } from '../useNetwork'
import { invalidateCachePrefix } from '../useBridge'

describe('useServicePods', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    // Clear module-level cache to prevent stale results across tests
    invalidateCachePrefix('')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns pods, loading, error, fetchServicePods', () => {
    const result = useServicePods()
    expect(result.pods.value).toEqual([])
    expect(result.loading.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(typeof result.fetchServicePods).toBe('function')
  })

  it('fetchServicePods populates pods from backend', async () => {
    const podData = [
      { name: 'svc-pod-1', ip: '10.0.0.1', ready: true },
      { name: 'svc-pod-2', ip: '10.0.0.2', ready: false },
    ]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetServicePods: vi.fn().mockResolvedValue(podData),
        },
      },
    })

    const { pods, loading, error, fetchServicePods } = useServicePods()
    await fetchServicePods('default', 'my-service')

    expect(pods.value).toEqual(podData)
    expect(error.value).toBe(null)
    expect(loading.value).toBe(false)
  })

  it('fetchServicePods returns empty array when result is null', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetServicePods: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { pods, fetchServicePods } = useServicePods()
    await fetchServicePods('default', 'my-service')

    expect(pods.value).toEqual([])
  })

  it('fetchServicePods returns empty array when result is not an array', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetServicePods: vi.fn().mockResolvedValue({ not: 'array' }),
        },
      },
    })

    const { pods, fetchServicePods } = useServicePods()
    await fetchServicePods('default', 'my-service')

    expect(pods.value).toEqual([])
  })

  it('fetchServicePods handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetServicePods: vi.fn().mockRejectedValue(new Error('Network error')),
        },
      },
    })

    const { pods, error, loading, fetchServicePods } = useServicePods()
    await fetchServicePods('default', 'my-service')

    expect(pods.value).toEqual([])
    expect(error.value).toBeTruthy()
    expect(loading.value).toBe(false)
  })

  it('fetchServicePods passes namespace and service name to backend', async () => {
    const mockGetServicePods = vi.fn().mockResolvedValue([])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetServicePods: mockGetServicePods,
        },
      },
    })

    const { fetchServicePods } = useServicePods()
    await fetchServicePods('production', 'nginx-service')

    expect(mockGetServicePods).toHaveBeenCalledWith('production', 'nginx-service')
  })
})
