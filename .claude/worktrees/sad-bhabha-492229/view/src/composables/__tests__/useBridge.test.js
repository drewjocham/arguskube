import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { ref } from 'vue'
import {
  callGo,
  isWails,
  invalidateCache,
  useAppMode,
  useClusterInfo,
  useContexts,
} from '../useWails'

describe('callGo', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Remove window.go if it exists
    if (window.go) {
      delete window.go
    }
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('when window.go exists (Wails mode)', () => {
    it('calls the Wails binding with correct method and args', async () => {
      const mockMethod = vi.fn().mockResolvedValue({ success: true })
      vi.stubGlobal('go', {
        pkg: {
          App: {
            GetAlerts: mockMethod,
          },
        },
      })

      const result = await callGo('GetAlerts')

      expect(mockMethod).toHaveBeenCalledOnce()
      expect(result).toEqual({ success: true })
    })

    it('passes multiple arguments to Wails binding', async () => {
      const mockMethod = vi.fn().mockResolvedValue('ok')
      vi.stubGlobal('go', {
        pkg: {
          App: {
            SwitchContext: mockMethod,
          },
        },
      })

      await callGo('SwitchContext', 'staging-eks', true)

      expect(mockMethod).toHaveBeenCalledWith('staging-eks', true)
    })

    it('re-throws Wails errors to caller', async () => {
      const testError = new Error('Wails binding failed')
      const mockMethod = vi.fn().mockRejectedValue(testError)
      vi.stubGlobal('go', {
        pkg: {
          App: {
            GetMetrics: mockMethod,
          },
        },
      })

      await expect(callGo('GetMetrics')).rejects.toThrow('Wails binding failed')
    })
  })

  describe('when window.go does not exist (SaaS mode)', () => {
    it('falls back to HTTP fetch', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: vi.fn().mockResolvedValue({ result: { cluster: 'test' } }),
      })
      vi.stubGlobal('fetch', mockFetch)

      const result = await callGo('GetClusterInfo')

      expect(mockFetch).toHaveBeenCalledOnce()
      expect(mockFetch).toHaveBeenCalledWith(
        'http://localhost:8080/api/GetClusterInfo',
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ args: [] }),
        })
      )
      expect(result).toEqual({ cluster: 'test' })
    })

    it('constructs correct URL with method name', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: vi.fn().mockResolvedValue({ result: null }),
      })
      vi.stubGlobal('fetch', mockFetch)

      await callGo('ListContexts')

      const callArgs = mockFetch.mock.calls[0]
      expect(callArgs[0]).toBe('http://localhost:8080/api/ListContexts')
    })

    it('passes arguments in request body', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: vi.fn().mockResolvedValue({ result: 'switched' }),
      })
      vi.stubGlobal('fetch', mockFetch)

      await callGo('SwitchContext', 'production-gke')

      const callArgs = mockFetch.mock.calls[0]
      expect(JSON.parse(callArgs[1].body)).toEqual({ args: ['production-gke'] })
    })

    it('throws when HTTP response is not ok', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
      })
      vi.stubGlobal('fetch', mockFetch)

      await expect(callGo('GetAlerts')).rejects.toThrow('HTTP error! status: 500')
    })

    it('throws when response JSON contains error field', async () => {
      const mockFetch = vi.fn().mockResolvedValue({
        ok: true,
        json: vi.fn().mockResolvedValue({ error: 'Backend error occurred' }),
      })
      vi.stubGlobal('fetch', mockFetch)

      await expect(callGo('GetMetrics')).rejects.toThrow('Backend error occurred')
    })

    it('throws when fetch itself fails', async () => {
      const mockFetch = vi.fn().mockRejectedValue(new Error('Network error'))
      vi.stubGlobal('fetch', mockFetch)

      await expect(callGo('GetClusterInfo')).rejects.toThrow('Network error')
    })
  })

  describe('isWails', () => {
    it('returns true when window.go exists', () => {
      vi.stubGlobal('go', {})
      expect(isWails()).toBe(true)
    })

    it('returns false when window.go does not exist', () => {
      if (window.go) delete window.go
      expect(isWails()).toBe(false)
    })

    it('handles undefined window gracefully', () => {
      // Should not throw
      expect(isWails()).toBe(false)
    })
  })
})

describe('useAppMode', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns mode ref with default value', () => {
    const { mode } = useAppMode()
    expect(mode.value).toBe('dashboard')
  })

  it('fetches and updates mode via fetchMode', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: 'terminal' }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { mode, fetchMode } = useAppMode()

    await fetchMode()

    expect(mode.value).toBe('terminal')
  })

  it('falls back to dashboard on fetch error', async () => {
    const mockFetch = vi.fn().mockRejectedValue(new Error('Network error'))
    vi.stubGlobal('fetch', mockFetch)

    const { mode, fetchMode } = useAppMode()

    await fetchMode()

    // Should keep default value on error
    expect(mode.value).toBe('dashboard')
  })

  it('calls GetAppMode via callGo', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: 'dashboard' }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { fetchMode } = useAppMode()

    await fetchMode()

    const callArgs = mockFetch.mock.calls[0]
    expect(callArgs[0]).toContain('/api/GetAppMode')
  })
})

describe('useClusterInfo', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns info, loading, error refs', () => {
    const { info, loading, error } = useClusterInfo()

    expect(info.value).toBe(null)
    expect(loading.value).toBe(true)
    expect(error.value).toBe(null)
  })

  it('fetches cluster info via refresh', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: { name: 'k3s-local', version: '1.28.0', nodes: 3 } }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { info, loading, refresh } = useClusterInfo()

    await refresh()

    expect(info.value).toEqual({ name: 'k3s-local', version: '1.28.0', nodes: 3 })
    expect(loading.value).toBe(false)
  })

  it('sets error and loading on fetch failure', async () => {
    const testError = new Error('Cluster unreachable')
    const mockFetch = vi.fn().mockRejectedValue(testError)
    vi.stubGlobal('fetch', mockFetch)

    const { error, loading, refresh } = useClusterInfo()

    await refresh()

    expect(error.value).toBeTruthy()
    expect(loading.value).toBe(false)
  })

  it('provides refresh function to re-fetch data', async () => {
    const clusterData = {
      name: 'k3s-local',
      version: '1.28.0',
      nodes: 3,
    }
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: clusterData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { info, loading, refresh } = useClusterInfo()

    await refresh()

    expect(info.value).toEqual(clusterData)
    expect(loading.value).toBe(false)
  })
})

describe('useContexts', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    // Clear any cached ListContexts from previous tests
    invalidateCache('ListContexts')
  })

  afterEach(() => {
    vi.restoreAllMocks()
    invalidateCache('ListContexts')
  })

  it('returns contexts, loading, switching, error refs and methods', () => {
    const composable = useContexts()

    expect(composable.contexts).toBeDefined()
    expect(composable.loading).toBeDefined()
    expect(composable.switching).toBeDefined()
    expect(composable.error).toBeDefined()
    expect(typeof composable.listContexts).toBe('function')
    expect(typeof composable.switchContext).toBe('function')
  })

  it('initializes with empty contexts array', () => {
    const { contexts } = useContexts()
    expect(contexts.value).toEqual([])
  })

  it('listContexts populates contexts from API', async () => {
    const contextData = [
      { name: 'local', cluster: 'local', active: true },
      { name: 'staging', cluster: 'staging', active: false },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: contextData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { contexts, listContexts } = useContexts()

    await listContexts()

    expect(contexts.value).toEqual(contextData)
  })

  it('returns empty array on API failure', async () => {
    const mockFetch = vi.fn().mockRejectedValue(new Error('API down'))
    vi.stubGlobal('fetch', mockFetch)

    const { contexts, listContexts } = useContexts()

    await listContexts()

    expect(contexts.value).toEqual([])
  })

  it('returns empty array when API returns empty', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: [] }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { contexts, listContexts } = useContexts()

    await listContexts()

    expect(contexts.value).toEqual([])
  })

  it('switchContext calls API with context name', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { switchContext, contexts } = useContexts()

    // Set up initial contexts
    contexts.value = [
      { name: 'local', cluster: 'local', active: true },
      { name: 'staging', cluster: 'staging', active: false },
    ]

    await switchContext('staging')

    const callArgs = mockFetch.mock.calls[0]
    expect(callArgs[0]).toContain('/api/SwitchContext')
    expect(JSON.parse(callArgs[1].body)).toEqual({ args: ['staging'] })
  })

  it('switchContext updates active context locally', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { switchContext, contexts } = useContexts()

    contexts.value = [
      { name: 'local', cluster: 'local', active: true },
      { name: 'staging', cluster: 'staging', active: false },
    ]

    await switchContext('staging')

    expect(contexts.value[0].active).toBe(false)
    expect(contexts.value[1].active).toBe(true)
  })

  it('switchContext handles API errors gracefully', async () => {
    const mockFetch = vi.fn().mockRejectedValue(new Error('Switch failed'))
    vi.stubGlobal('fetch', mockFetch)

    const { switchContext, error, switching } = useContexts()

    await switchContext('staging')

    expect(error.value).toBeTruthy()
    expect(switching.value).toBe(false)
  })

  it('sets switching state during context switch', async () => {
    const mockFetch = vi.fn().mockImplementation(() => {
      return new Promise(r => setTimeout(() => {
        r({
          ok: true,
          json: () => Promise.resolve({ result: null }),
        })
      }, 10))
    })
    vi.stubGlobal('fetch', mockFetch)

    const { switchContext, switching } = useContexts()

    const promise = switchContext('staging')
    expect(switching.value).toBe(true)

    await promise
    expect(switching.value).toBe(false)
  })

  it('listContexts sets loading state correctly', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: [] }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { listContexts, loading } = useContexts()

    const promise = listContexts()
    expect(loading.value).toBe(true)

    await promise
    expect(loading.value).toBe(false)
  })

  it('clears error on successful listContexts', async () => {
    const contextData = [{ name: 'test', cluster: 'test', active: true }]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: contextData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { listContexts, error } = useContexts()

    error.value = 'Previous error'
    await listContexts()

    expect(error.value).toBe(null)
  })
})
