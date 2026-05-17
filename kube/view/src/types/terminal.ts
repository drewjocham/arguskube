import type { Terminal } from 'xterm'
import type { FitAddon } from 'xterm-addon-fit'

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

export type DomainId = 'default' | 'k8s' | 'kafka' | 'cloud'

export interface Domain {
  id: DomainId
  label: string
  icon: string
}

export interface TabState {
  sessionId: string
  domain: string
  label: string
  initError: string | null
  unavailable: boolean
  started: boolean
  term: Terminal | null
  fitAddon: FitAddon | null
}

export interface PinState {
  [runbookId: string]: string
}

export interface OverrideState {
  [key: string]: string
}

export interface CaptureBuffers {
  [blockId: string]: string
}

export interface CaptureTimestamps {
  [blockId: string]: number
}
