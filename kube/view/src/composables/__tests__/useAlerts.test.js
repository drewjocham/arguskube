import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useAlerts, useDiagnostics } from '../useAlerts'

describe('useAlerts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns alerts, loading, refresh', () => {
    const { alerts, loading, refresh } = useAlerts(5000)
    expect(alerts.value).toEqual([])
    expect(loading.value).toBe(true)
    expect(typeof refresh).toBe('function')
  })

  it('refresh fetches alerts and updates state', async () => {
    const alertData = [
      { id: 'alert-1', severity: 'warning', message: 'Pod Pending' },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: alertData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { alerts, loading, refresh } = useAlerts(5000)

    await refresh()

    expect(alerts.value).toEqual(alertData)
    expect(loading.value).toBe(false)
  })

  it('refresh handles error gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Backend down')))

    const { alerts, loading, refresh } = useAlerts(5000)

    await refresh()

    expect(alerts.value).toEqual([])
    expect(loading.value).toBe(false)
  })
})

describe('useDiagnostics', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns bundle, loading, error, diagnose', () => {
    const { bundle, loading, error, diagnose } = useDiagnostics()
    expect(bundle.value).toBe(null)
    expect(loading.value).toBe(false)
    expect(error.value).toBe(null)
    expect(typeof diagnose).toBe('function')
  })

  it('diagnose(alertId) calls DiagnoseAlert and sets bundle on success', async () => {
    const bundleData = { summary: 'High memory pressure', details: '...' }
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: bundleData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { bundle, loading, diagnose } = useDiagnostics()

    await diagnose('alert-1')

    expect(mockFetch.mock.calls[0][0]).toContain('/api/DiagnoseAlert')
    expect(bundle.value).toEqual(bundleData)
    expect(loading.value).toBe(false)
  })

  it('diagnose handles errors gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Diagnostics unavailable')))

    const { bundle, error, loading, diagnose } = useDiagnostics()

    await diagnose('alert-1')

    expect(error.value).toBeTruthy()
    expect(bundle.value).toBe(null)
    expect(loading.value).toBe(false)
  })
})
