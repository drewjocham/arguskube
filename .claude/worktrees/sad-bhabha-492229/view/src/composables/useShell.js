import { ref } from 'vue'
import { callGo } from './useBridge'

/**
 * Composable for the embedded terminal.
 */
export function useTerminal() {
  async function startTerminal(rows, cols) {
    try {
      await callGo('StartTerminal', rows, cols)
    } catch (e) {
      console.error('[terminal]', e)
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
      console.error('[exec]', e)
    }
  }

  async function sendInput(data) {
    try {
      await callGo('SendExecInput', data)
    } catch (e) {
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
