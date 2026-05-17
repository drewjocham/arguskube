export interface TerminalDomain {
  id: string
  label: string
  icon: string
}

export interface TabState {
  sessionId: string
  domain: string
  label: string
  initError: string | null
  started: boolean
  term: any | null
  fitAddon: any | null
}

export interface PendingCommand {
  text: string
  requestedAt: number
  sessionId?: string
  sectionLabel?: string
}

export interface CommandMeta {
  sessionId?: string
  sectionLabel?: string
}

export type TerminalOutputPayload =
  | string
  | { sessionId?: string; data: string; source?: string }
