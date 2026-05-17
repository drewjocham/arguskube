import { ref } from 'vue'
import { callGo, isWails } from './useBridge'
import type { DomainId, Domain } from '../types/terminal'

export class TerminalUnavailableError extends Error {
  code: string

  constructor(message: string) {
    super(message)
    this.name = 'TerminalUnavailableError'
    this.code = 'TERMINAL_UNAVAILABLE'
  }
}

const TERMINAL_UNAVAILABLE_MSG =
  'The terminal needs the Argus desktop app — it spawns a local shell ' +
  'process, which the web build cannot do. Open Argus on your desktop ' +
  'to use this feature.'

export function classifyTerminalError(err: unknown): unknown {
  if (err instanceof TerminalUnavailableError) return err
  const msg = (err as Record<string, unknown>)?.message as string | undefined || (err ? String(err as never) : '')
  if (/status: 403\b/.test(msg) || /not exposed via HTTP/i.test(msg)) {
    return new TerminalUnavailableError(TERMINAL_UNAVAILABLE_MSG)
  }
  return err
}

function guardTerminalAvailable(): void {
  if (!isWails()) {
    throw new TerminalUnavailableError(TERMINAL_UNAVAILABLE_MSG)
  }
}

export function useTerminal() {
  async function startTerminal(rows: number, cols: number): Promise<void> {
    guardTerminalAvailable()
    try {
      await callGo('StartTerminal', rows, cols)
    } catch (e: unknown) {
      const classified = classifyTerminalError(e)
      if ((classified as TerminalUnavailableError).code !== 'TERMINAL_UNAVAILABLE') {
        console.error('[terminal]', e)
      }
      throw classified
    }
  }

  async function sendInput(data: string): Promise<void> {
    try {
      await callGo('SendTerminalInput', data)
    } catch (e: unknown) {
      console.error('[terminal-input]', e)
    }
  }

  async function resizeTerminal(rows: number, cols: number): Promise<void> {
    try {
      await callGo('ResizeTerminal', rows, cols)
    } catch (e: unknown) {
      console.error('[terminal-resize]', e)
    }
  }

  return { startTerminal, sendInput, resizeTerminal }
}

export function usePodExec() {
  const connected = ref(false)
  const error = ref<string | null>(null)

  async function startExec(namespace: string, podName: string, container: string, rows: number, cols: number): Promise<void> {
    error.value = null
    try {
      await callGo('ExecPodShell', namespace, podName, container || '', rows, cols)
      connected.value = true
    } catch (e: unknown) {
      error.value = String((e as Record<string, unknown>)?.message ?? e ?? '')
      connected.value = false
      console.error('[exec]', e)
    }
  }

  async function sendInput(data: string): Promise<void> {
    try {
      await callGo('SendExecInput', data)
    } catch (e: unknown) {
      if (connected.value) {
        error.value = `Pod shell input failed: ${String((e as Record<string, unknown>)?.message ?? e ?? '')}`
        connected.value = false
      }
      console.error('[exec-input]', e)
    }
  }

  async function resizeExec(rows: number, cols: number): Promise<void> {
    try {
      await callGo('ResizeExec', rows, cols)
    } catch (e: unknown) {
      console.error('[exec-resize]', e)
    }
  }

  async function closeExec(): Promise<void> {
    try {
      await callGo('CloseExecSession')
    } catch (e: unknown) {
      console.error('[exec-close]', e)
    }
    connected.value = false
  }

  return { connected, error, startExec, sendInput, resizeExec, closeExec }
}

export function useTerminalSession() {
  const sessions = ref<unknown[]>([])

  async function createSession(sessionId: string, domain: string, label: string, rows: number, cols: number): Promise<void> {
    guardTerminalAvailable()
    try {
      await callGo('StartTerminalSession', sessionId, domain, label, rows, cols)
    } catch (e: unknown) {
      throw classifyTerminalError(e)
    }
    await refreshSessions()
  }

  async function sendSessionInput(sessionId: string, data: string): Promise<void> {
    try {
      await callGo('SendTerminalSessionInput', sessionId, data)
    } catch (e: unknown) {
      console.error('[terminal-session-input]', e)
    }
  }

  async function resizeSession(sessionId: string, rows: number, cols: number): Promise<void> {
    try {
      await callGo('ResizeTerminalSession', sessionId, rows, cols)
    } catch (e: unknown) {
      console.error('[terminal-session-resize]', e)
    }
  }

  async function closeSession(sessionId: string): Promise<void> {
    await callGo('CloseTerminalSession', sessionId)
    await refreshSessions()
  }

  async function refreshSessions(): Promise<void> {
    try {
      sessions.value = await callGo('ListTerminalSessions') || []
    } catch (e: unknown) {
      console.error('[terminal-sessions]', e)
    }
  }

  const domains: Domain[] = [
    { id: 'default' as DomainId, label: 'Shell', icon: '>' },
    { id: 'k8s' as DomainId, label: 'K8s', icon: '\u2388' },
    { id: 'kafka' as DomainId, label: 'Kafka', icon: 'K' },
    { id: 'cloud' as DomainId, label: 'Cloud', icon: '\u2601' },
  ]

  function domainLabel(domain: string): string {
    const d = domains.find(d => d.id === domain)
    return d ? d.label : domain
  }

  function domainIcon(domain: string): string {
    const d = domains.find(d => d.id === domain)
    return d ? d.icon : '>'
  }

  return {
    sessions, domains, createSession, sendSessionInput,
    resizeSession, closeSession, refreshSessions,
    domainLabel, domainIcon,
  }
}

export function useTerminalCopilot() {
  const loading = ref(false)
  const result = ref<string | null>(null)
  const error = ref<string | null>(null)

  async function explainOutput(output: string, domain: string): Promise<unknown> {
    loading.value = true
    error.value = null
    result.value = null
    try {
      const resp = await callGo('ExplainTerminalOutput', output, domain)
      result.value = resp as string
      return resp
    } catch (e: unknown) {
      error.value = String((e as Record<string, unknown>)?.message ?? e ?? '')
      return null
    } finally {
      loading.value = false
    }
  }

  async function generateCommand(prompt: string, domain: string): Promise<unknown> {
    loading.value = true
    error.value = null
    result.value = null
    try {
      const resp = await callGo('GenerateCommand', prompt, domain)
      result.value = resp as string
      return resp
    } catch (e: unknown) {
      error.value = String((e as Record<string, unknown>)?.message ?? e ?? '')
      return null
    } finally {
      loading.value = false
    }
  }

  function clear(): void {
    result.value = null
    error.value = null
  }

  return { loading, result, error, explainOutput, generateCommand, clear }
}
