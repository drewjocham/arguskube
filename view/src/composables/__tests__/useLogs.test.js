import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { usePodLogs, useLogStream, useLogs, useNodeLogs } from '../useLogs'
import { invalidateCachePrefix } from '../useBridge'

describe('usePodLogs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns logs, loading, and fetch', () => {
    const { logs, loading, fetch } = usePodLogs()
    expect(logs.value).toEqual([])
    expect(loading.value).toBe(false)
    expect(typeof fetch).toBe('function')
  })

  it('fetch retrieves pod logs', async () => {
    const logData = [
      { timestamp: '2024-01-01T00:00:00Z', message: 'Server started' },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: logData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { logs, loading, fetch } = usePodLogs()

    await fetch('default', 'my-pod', 50)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/GetPodLogs')
    expect(logs.value).toEqual(logData)
    expect(loading.value).toBe(false)
  })

  it('fetch handles errors gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Logs unavailable')))

    const { logs, loading, fetch } = usePodLogs()

    await fetch('default', 'my-pod')

    expect(logs.value).toEqual([])
    expect(loading.value).toBe(false)
  })
})

describe('useLogStream', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns lines, streaming, error, startStream, clear', () => {
    const { lines, streaming, error, startStream, clear } = useLogStream()
    expect(lines.value).toEqual([])
    expect(streaming.value).toBe(false)
    expect(error.value).toBe(null)
    expect(typeof startStream).toBe('function')
    expect(typeof clear).toBe('function')
  })

  it('startStream calls StreamPodLogsFollow and populates lines', async () => {
    const logLines = [
      { message: 'line 1', timestamp: '2024-01-01T00:00:00Z' },
      { message: 'line 2', timestamp: '2024-01-01T00:00:01Z' },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: logLines }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { lines, streaming, startStream } = useLogStream()

    await startStream('default', 'my-pod', '', 100)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/StreamPodLogsFollow')
    expect(lines.value).toEqual(logLines)
    expect(streaming.value).toBe(false)
  })

  it('startStream handles string result splitting', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: 'line1\nline2\nline3' }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { lines, startStream } = useLogStream()

    await startStream('default', 'my-pod')

    expect(lines.value).toHaveLength(3)
    expect(lines.value[0]).toEqual({ message: 'line1' })
    expect(lines.value[2]).toEqual({ message: 'line3' })
  })

  it('startStream handles errors gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Stream error')))

    const { error, startStream } = useLogStream()

    await startStream('default', 'my-pod')

    expect(error.value).toBeTruthy()
  })

  it('clear resets lines and error', async () => {
    const { lines, error, clear } = useLogStream()
    lines.value = [{ message: 'test' }]
    error.value = 'some error'

    clear()

    expect(lines.value).toEqual([])
    expect(error.value).toBe(null)
  })
})

describe('useLogs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns entries, histogram, fields, total, loading, queryTime, error, queryLogs', () => {
    const {
      entries, histogram, fields, total,
      loading, queryTime, error, queryLogs,
    } = useLogs()
    expect(entries.value).toEqual([])
    expect(histogram.value).toEqual([])
    expect(fields.value).toEqual([])
    expect(total.value).toBe(0)
    expect(loading.value).toBe(false)
    expect(queryTime.value).toBe(0)
    expect(error.value).toBe(null)
    expect(typeof queryLogs).toBe('function')
  })

  it('queryLogs fetches and populates entries', async () => {
    const queryResult = {
      entries: [{ message: 'log entry', timestamp: '2024-01-01T00:00:00Z' }],
      histogram: [{ bucket: '2024-01-01T00:00:00Z', count: 5 }],
      fields: ['message', 'timestamp'],
      total: 1,
    }
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: queryResult }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { entries, histogram, fields, total, loading, queryLogs } = useLogs()

    await queryLogs('error', 'default', 100)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/QueryLogs')
    expect(entries.value).toEqual(queryResult.entries)
    expect(histogram.value).toEqual(queryResult.histogram)
    expect(fields.value).toEqual(queryResult.fields)
    expect(total.value).toBe(1)
    expect(loading.value).toBe(false)
  })

  it('queryLogs handles errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Query failed')))

    const { error, queryLogs } = useLogs()

    await queryLogs('*')

    expect(error.value).toBeTruthy()
  })
})

describe('useNodeLogs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('GetNodeLogs')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns logs, loading, error, fetchNodeLogs, clear', () => {
    const { logs, loading, error, fetchNodeLogs, clear } = useNodeLogs()
    expect(logs.value).toEqual([])
    expect(loading.value).toBe(false)
    expect(error.value).toBe(null)
    expect(typeof fetchNodeLogs).toBe('function')
    expect(typeof clear).toBe('function')
  })

  it('fetchNodeLogs calls GetNodeLogs and populates logs', async () => {
    const logData = [
      { timestamp: '2024-01-01T00:00:00Z', message: 'kubelet started' },
    ]
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: logData }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { logs, fetchNodeLogs } = useNodeLogs()

    await fetchNodeLogs('node-1', 100)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/GetNodeLogs')
    expect(logs.value).toEqual(logData)
  })

  it('fetchNodeLogs handles error gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Node logs unavailable')))

    const { logs, fetchNodeLogs } = useNodeLogs()

    await fetchNodeLogs('node-1')

    expect(logs.value).toEqual([])
  })

  it('clear resets logs and error', async () => {
    const { logs, error, clear } = useNodeLogs()
    logs.value = [{ message: 'test' }]
    error.value = 'some error'

    clear()

    expect(logs.value).toEqual([])
    expect(error.value).toBe(null)
  })
})
