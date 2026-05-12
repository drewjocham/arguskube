export interface AlertItem {
  id: string
  kind?: string
  labels?: Record<string, string>
  severity?: string
  message?: string
  active?: boolean
  startsAt?: string
}

export interface MetricsFrame {
  cpu?: number
  memory?: number
  network?: { rx: number; tx: number }
  timestamp?: number
}

export interface LogLine {
  line: string
  source?: string
  timestamp?: string
}

export interface SpotCheckActive {
  checkName: string
  description?: string
}

export interface AgentAnalysisStart {
  runId: string
  lookingAt: string
}

export interface AgentAnalysisComplete {
  runId: string
  text: string
}

export interface TerminalOutput {
  data: string
  source?: string
}

export interface ExecOutput {
  data: string
  pod?: string
  container?: string
}

export interface ExecExit {
  code: number
  pod?: string
}

export interface DeepLink {
  path: string
}

export interface ArgusNotification {
  kind: string
  title: string
  body?: string
  rerunnable?: boolean
  rerunPayload?: any
  meta?: any
}

export interface SaveActivity {
  method: string
  status: 'ok' | 'error'
  label?: string
  durationMs?: number
  error?: string
}

export interface ToastMessage {
  id?: string
  message: string
  kind?: 'info' | 'success' | 'error' | 'warning'
  duration?: number
}

export interface NavRequest {
  navId: string
  anchor?: string
}

export interface NavReturnTo {
  navId: string
  label: string
  anchor?: string
}

export interface SessionExpired {}

export type WailsBackendEvents = {
  'alert:update': { alerts?: AlertItem[] }
  'metrics:update': { metrics?: MetricsFrame }
  'log:line': LogLine
  'argus:notification': ArgusNotification
  'argus:spotcheck:active': SpotCheckActive | null
  'agent:analysis:start': AgentAnalysisStart
  'agent:analysis:complete': AgentAnalysisComplete
  'terminal:output': TerminalOutput
  'exec:output': ExecOutput
  'exec:exit': ExecExit
  'deep-link': DeepLink
}

export type AppLocalEvents = {
  'argus:save': SaveActivity
  'argus:session-expired': SessionExpired
  'toast:show': ToastMessage
  'toast:dismiss': { id: string }
  'nav:request': NavRequest
  'nav:return-to': NavReturnTo
  'spotcheck:run': {}
  'credential:alert': SaveActivity
  // Generic watcher framework: anything that can expire (credentials, certs,
  // licenses, OAuth refresh tokens, …) registers itself and routes status
  // updates through the same guard → bus → toast/bell pipeline.
  'watcher:status': WatcherStatus
  // The notification guard fires this when it detects spam from a single
  // source and replaces further fires with a silence-acknowledgement.
  'watcher:silenced': WatcherSilenced
}

export interface WatcherStatus {
  watcherId: string
  label: string
  kind: string                 // 'credential' | 'volume' | 'cert' | 'license' | …
  status: string               // 'ok' | 'warn' | 'expired' | 'invalid' | 'error'
  message?: string
  expiresAt?: string           // RFC3339 timestamp when the thing expires (if known)
  configureAnchor?: string     // deep-link target for the "Configure" CTA
  // Caller can pass a pre-built display label; otherwise the guard derives one.
  displayLabel?: string
}

export interface WatcherSilenced {
  source: string               // watcher id or generic source key
  count: number                // how many notifications were collapsed
  silencedUntil: string        // RFC3339 — when auto-resume kicks in
  acknowledgeable: boolean
}

export interface TaskMap extends WailsBackendEvents, AppLocalEvents {}

export type TaskName = keyof TaskMap

export type TaskPayload<T extends TaskName> = TaskMap[T]
