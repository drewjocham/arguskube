import { ref } from 'vue'
import { callGo } from './useBridge'

/**
 * Composable for the embedded terminal.
 */
export function useTerminal() {
  // startTerminal RETHROWS so the caller (TerminalView) can surface the
  // failure in the UI. Previously this silently logged and returned, which
  // left the user staring at an empty black box with no clue what failed.
  async function startTerminal(rows, cols) {
    try {
      await callGo('StartTerminal', rows, cols)
    } catch (e) {
      console.error('[terminal]', e)
      throw e
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
    await callGo('StartTerminalSession', sessionId, domain, label, rows, cols)
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
