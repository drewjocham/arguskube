import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useMetrics, useCostEstimate } from '../useMetrics'

describe('useMetrics', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns metrics, loading, refresh refs', () => {
    const { metrics, loading, refresh } = useMetrics(60000)
    expect(metrics.value).toBe(null)
    expect(loading.value).toBe(true)
    expect(typeof refresh).toBe('function')
  })

  it('fetches metrics via refresh', async () => {
    const metricsData = {
      cpuUsage: 0.45,
      memoryUsage: 0.62,
      podCount: 12,
    }
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: metricsData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { metrics, loading, refresh } = useMetrics(60000)

    await refresh()

    expect(metrics.value).toEqual(metricsData)
    expect(loading.value).toBe(false)
  })

  it('handles fetch errors gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Metrics unavailable')))

    const { metrics, loading, refresh } = useMetrics(60000)

    await refresh()

    expect(metrics.value).toBe(null)
    expect(loading.value).toBe(false)
  })
})

describe('useCostEstimate', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns report, loading, provider refs', () => {
    const { report, loading, provider } = useCostEstimate()
    expect(report.value).toBe(null)
    expect(loading.value).toBe(false)
    expect(provider.value).toBe('aws')
  })

  it('fetches cost data for default provider', async () => {
    const costData = { totalMonthly: 1250, breakdown: [] }
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: costData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { report, fetchCosts } = useCostEstimate()
    await fetchCosts()

    expect(report.value).toEqual(costData)
  })

  it('uses provider override when specified', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { fetchCosts } = useCostEstimate()
    await fetchCosts('gcp')

    const callBody = JSON.parse(mockFetch.mock.calls[0][1].body)
    expect(callBody.args[0]).toBe('gcp')
  })
})
