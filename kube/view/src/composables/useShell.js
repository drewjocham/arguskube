import { ref } from 'vue'
import { callGo, isWails } from './useBridge'

/**
 * TerminalUnavailableError signals "we deliberately can't run a
 * terminal in this environment" — distinct from a real backend bug.
 * Callers (TerminalView, App.vue:openPopOut) branch on
 * err.code === 'TERMINAL_UNAVAILABLE' to render an actionable UX
 * instead of the scary "HTTP error! status: 403" string the user
 * was seeing before.
 *
 * The condition: terminal Wails-bindings (StartTerminal,
 * StartTerminalSession, SendTerminalInput, ResizeTerminal,
 * LaunchPopOutTerminal) spawn local OS processes. They're intentionally
 * NOT in api/pkg/server.go's httpExposedMethods allowlist, so the HTTP
 * fallback returns 403 in SaaS / browser mode. The fix is not "expose
 * them over HTTP" (a non-starter security-wise) but "detect the
 * environment up front and surface the right message."
 */
export class TerminalUnavailableError extends Error {
  constructor(message) {
    super(message)
    this.name = 'TerminalUnavailableError'
    this.code = 'TERMINAL_UNAVAILABLE'
  }
}

// Default human-facing message reused by every terminal entry point.
const TERMINAL_UNAVAILABLE_MSG =
  'The terminal needs the Argus desktop app — it spawns a local shell ' +
  'process, which the web build cannot do. Open Argus on your desktop ' +
  'to use this feature.'

// classifyTerminalError converts a known-bad backend response into a
// TerminalUnavailableError so the UI can render the friendly variant.
// Anything else is passed through unchanged.
export function classifyTerminalError(err) {
  if (err instanceof TerminalUnavailableError) return err
  const msg = err?.message || String(err || '')
  // Both shapes that indicate the method isn't HTTP-exposed:
  //   "HTTP error! status: 403"  (callGo wrapper)
  //   "method not exposed via HTTP API" (raw server body)
  if (/status: 403\b/.test(msg) || /not exposed via HTTP/i.test(msg)) {
    return new TerminalUnavailableError(TERMINAL_UNAVAILABLE_MSG)
  }
  return err
}

// guardTerminalAvailable short-circuits before the network attempt
// when we already know the environment can't host a terminal. Saves
// a 403 round-trip and a console error.
function guardTerminalAvailable() {
  if (!isWails()) {
    throw new TerminalUnavailableError(TERMINAL_UNAVAILABLE_MSG)
  }
}

/**
 * Composable for the embedded terminal.
 */
export function useTerminal() {
  // startTerminal RETHROWS so the caller (TerminalView) can surface the
  // failure in the UI. Previously this silently logged and returned, which
  // left the user staring at an empty black box with no clue what failed.
  async function startTerminal(rows, cols) {
    guardTerminalAvailable()
    try {
      await callGo('StartTerminal', rows, cols)
    } catch (e) {
      const classified = classifyTerminalError(e)
      if (classified.code !== 'TERMINAL_UNAVAILABLE') {
        // Real failure — log + rethrow so TerminalView shows the
        // error banner with retry.
        console.error('[terminal]', e)
      }
      throw classified
    }
  }

  async function sendInput(data) {
    try {
      await callGo('SendTerminalInput', data)
    } catch (e) {
      console.error('[terminal-input]', e)
    }
  }

  async function resizeTerminal(rows, cols) {
    try {
      await callGo('ResizeTerminal', rows, cols)
    } catch (e) {
      console.error('[terminal-resize]', e)
    }
  }

  return { startTerminal, sendInput, resizeTerminal }
}

/**
 * Composable for interactive pod exec (kubectl exec -it).
 *
 * Errors now flow into `error` so the caller can render them. The old
 * version swallowed sendInput / resizeExec errors with console.error,
 * which left the user staring at a non-responsive shell with no clue
 * why ("Shell in pods not working"). When the underlying RBAC drops,
 * the pod gets deleted, or the SPDY stream dies, the user sees a
 * specific message instead of silence.
 */
export function usePodExec() {
  const connected = ref(false)
  const error = ref(null)

  async function startExec(namespace, podName, container, rows, cols) {
    error.value = null
    try {
      await callGo('ExecPodShell', namespace, podName, container || '', rows, cols)
      connected.value = true
    } catch (e) {
      error.value = e?.message || String(e)
      connected.value = false
      console.error('[exec]', e)
    }
  }

  async function sendInput(data) {
    try {
      await callGo('SendExecInput', data)
    } catch (e) {
      if (connected.value) {
        error.value = `Pod shell input failed: ${e?.message || e}`
        connected.value = false
      }
      console.error('[exec-input]', e)
    }
  }

  async function resizeExec(rows, cols) {
    try {
      await callGo('ResizeExec', rows, cols)
    } catch (e) {
      console.error('[exec-resize]', e)
    }
  }

  async function closeExec() {
    try {
      await callGo('CloseExecSession')
    } catch (e) {
      console.error('[exec-close]', e)
    }
    connected.value = false
  }

  return { connected, error, startExec, sendInput, resizeExec, closeExec }
}

/**
 * Composable for multi-session terminal with domain support.
 */
export function useTerminalSession() {
  const sessions = ref([])

  async function createSession(sessionId, domain, label, rows, cols) {
    guardTerminalAvailable()
    try {
      await callGo('StartTerminalSession', sessionId, domain, label, rows, cols)
    } catch (e) {
      throw classifyTerminalError(e)
    }
    await refreshSessions()
  }

  async function sendSessionInput(sessionId, data) {
    try {
      await callGo('SendTerminalSessionInput', sessionId, data)
    } catch (e) {
      console.error('[terminal-session-input]', e)
    }
  }

  async function resizeSession(sessionId, rows, cols) {
    try {
      await callGo('ResizeTerminalSession', sessionId, rows, cols)
    } catch (e) {
      console.error('[terminal-session-resize]', e)
    }
  }

  async function closeSession(sessionId) {
    await callGo('CloseTerminalSession', sessionId)
    await refreshSessions()
  }

  async function refreshSessions() {
    try {
      sessions.value = await callGo('ListTerminalSessions') || []
    } catch (e) {
      console.error('[terminal-sessions]', e)
    }
  }

  const domains = [
    { id: 'default', label: 'Shell', icon: '>' },
    { id: 'k8s', label: 'K8s', icon: '\u2388' },
    { id: 'kafka', label: 'Kafka', icon: 'K' },
    { id: 'cloud', label: 'Cloud', icon: '\u2601' },
  ]

  function domainLabel(domain) {
    const d = domains.find(d => d.id === domain)
    return d ? d.label : domain
  }

  function domainIcon(domain) {
    const d = domains.find(d => d.id === domain)
    return d ? d.icon : '>'
  }

  return {
    sessions, domains, createSession, sendSessionInput,
    resizeSession, closeSession, refreshSessions,
    domainLabel, domainIcon,
  }
}

/**
 * Composable for the terminal AI copilot.
 */
export function useTerminalCopilot() {
  const loading = ref(false)
  const result = ref(null)
  const error = ref(null)

  async function explainOutput(output, domain) {
    loading.value = true
    error.value = null
    result.value = null
    try {
      const resp = await callGo('ExplainTerminalOutput', output, domain)
      result.value = resp
      return resp
    } catch (e) {
      error.value = e?.message || String(e)
      return null
    } finally {
      loading.value = false
    }
  }

  async function generateCommand(prompt, domain) {
    loading.value = true
    error.value = null
    result.value = null
    try {
      const resp = await callGo('GenerateCommand', prompt, domain)
      result.value = resp
      return resp
    } catch (e) {
      error.value = e?.message || String(e)
      return null
    } finally {
      loading.value = false
    }
  }

  function clear() {
    result.value = null
    error.value = null
  }

  return { loading, result, error, explainOutput, generateCommand, clear }
}
