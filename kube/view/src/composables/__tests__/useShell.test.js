import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useTerminal, usePodExec, classifyTerminalError, TerminalUnavailableError } from '../useShell'

// stubWails / stripWails swap the "is this a Wails build?" signal
// (isWails() in useBridge checks for window.go). Both code paths need
// driving without touching production code.
function stubWails() {
  // Minimal shape: isWails() only checks !!window.go.
  window.go = { pkg: { App: {} } }
}
function stripWails() {
  if (window.go) delete window.go
}

describe('useTerminal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    stripWails()
  })

  afterEach(() => {
    vi.restoreAllMocks()
    stripWails()
  })

  it('returns startTerminal, sendInput, resizeTerminal', () => {
    const { startTerminal, sendInput, resizeTerminal } = useTerminal()
    expect(typeof startTerminal).toBe('function')
    expect(typeof sendInput).toBe('function')
    expect(typeof resizeTerminal).toBe('function')
  })

  it('startTerminal calls StartTerminal with rows and cols', async () => {
    stubWails() // happy path runs against Wails (or its mock)
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { startTerminal } = useTerminal()

    await startTerminal(24, 80)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/StartTerminal')
    const body = JSON.parse(mockFetch.mock.calls[0][1].body)
    expect(body.args).toEqual([24, 80])
  })

  it('startTerminal RETHROWS errors so the UI can surface them', async () => {
    // Previously startTerminal swallowed errors silently, leaving the user
    // staring at an empty black box. Now it rethrows so TerminalView can
    // show a visible "session failed to start" panel with a Retry button.
    stubWails()
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Terminal error')))

    const { startTerminal } = useTerminal()
    await expect(startTerminal(24, 80)).rejects.toThrow('Terminal error')
  })

  it('startTerminal throws TerminalUnavailableError in SaaS/web mode (no Wails)', async () => {
    // Browser mode: the call used to round-trip and surface as a
    // scary "HTTP error! status: 403" banner. Now the guard
    // short-circuits BEFORE the network call with a typed error
    // (.code === 'TERMINAL_UNAVAILABLE') the UI branches on.
    stripWails()
    const mockFetch = vi.fn()
    vi.stubGlobal('fetch', mockFetch)

    const { startTerminal } = useTerminal()
    await expect(startTerminal(24, 80)).rejects.toBeInstanceOf(TerminalUnavailableError)
    expect(mockFetch).not.toHaveBeenCalled() // no 403 round-trip
  })

  it('sendInput calls SendTerminalInput with data', async () => {
    stubWails()
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { sendInput } = useTerminal()

    await sendInput('ls -la')

    expect(mockFetch.mock.calls[0][0]).toContain('/api/SendTerminalInput')
    const body = JSON.parse(mockFetch.mock.calls[0][1].body)
    expect(body.args).toEqual(['ls -la'])
  })

  it('resizeTerminal calls ResizeTerminal with rows and cols', async () => {
    stubWails()
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { resizeTerminal } = useTerminal()

    await resizeTerminal(24, 80)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/ResizeTerminal')
    const body = JSON.parse(mockFetch.mock.calls[0][1].body)
    expect(body.args).toEqual([24, 80])
  })
})

describe('classifyTerminalError', () => {
  it('passes TerminalUnavailableError through unchanged', () => {
    const err = new TerminalUnavailableError('already classified')
    expect(classifyTerminalError(err)).toBe(err)
  })

  it('reclassifies a 403 status string as TerminalUnavailableError', () => {
    const err = new Error('HTTP error! status: 403')
    const out = classifyTerminalError(err)
    expect(out).toBeInstanceOf(TerminalUnavailableError)
    expect(out.code).toBe('TERMINAL_UNAVAILABLE')
  })

  it('reclassifies "method not exposed via HTTP" as TerminalUnavailableError', () => {
    // The raw server body — passes through callGo wrapped in data.error.
    const err = new Error('method not exposed via HTTP API')
    expect(classifyTerminalError(err)).toBeInstanceOf(TerminalUnavailableError)
  })

  it('passes unrelated errors through unchanged', () => {
    const err = new Error('connection refused')
    expect(classifyTerminalError(err)).toBe(err)
  })

  it('handles null/undefined errors without throwing', () => {
    expect(classifyTerminalError(null)).toBe(null)
    expect(classifyTerminalError(undefined)).toBe(undefined)
  })
})

describe('usePodExec', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns connected, error, startExec, sendInput, resizeExec, closeExec', () => {
    const { connected, error, startExec, sendInput, resizeExec, closeExec } = usePodExec()
    expect(connected.value).toBe(false)
    expect(error.value).toBe(null)
    expect(typeof startExec).toBe('function')
    expect(typeof sendInput).toBe('function')
    expect(typeof resizeExec).toBe('function')
    expect(typeof closeExec).toBe('function')
  })

  it('startExec calls ExecPodShell and sets connected=true', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { connected, startExec } = usePodExec()

    await startExec('default', 'nginx-abc', 'nginx', 24, 80)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/ExecPodShell')
    const body = JSON.parse(mockFetch.mock.calls[0][1].body)
    expect(body.args).toEqual(['default', 'nginx-abc', 'nginx', 24, 80])
    expect(connected.value).toBe(true)
  })

  it('startExec handles errors gracefully', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('Exec failed')))

    const { connected, error, startExec } = usePodExec()

    await startExec('default', 'nginx-abc', '', 24, 80)

    expect(error.value).toBeTruthy()
    expect(connected.value).toBe(false)
  })

  it('sendInput calls SendExecInput with data', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { sendInput } = usePodExec()

    await sendInput('ls -la')

    expect(mockFetch.mock.calls[0][0]).toContain('/api/SendExecInput')
    const body = JSON.parse(mockFetch.mock.calls[0][1].body)
    expect(body.args).toEqual(['ls -la'])
  })

  it('resizeExec calls ResizeExec with rows and cols', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { resizeExec } = usePodExec()

    await resizeExec(24, 80)

    expect(mockFetch.mock.calls[0][0]).toContain('/api/ResizeExec')
    const body = JSON.parse(mockFetch.mock.calls[0][1].body)
    expect(body.args).toEqual([24, 80])
  })

  it('closeExec calls CloseExecSession and sets connected=false', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: vi.fn().mockResolvedValue({ result: null }),
    })
    vi.stubGlobal('fetch', mockFetch)

    const { connected, closeExec } = usePodExec()

    connected.value = true
    await closeExec()

    expect(mockFetch.mock.calls[0][0]).toContain('/api/CloseExecSession')
    expect(connected.value).toBe(false)
  })
})
