import { defineStore } from 'pinia'

export interface PendingCommand {
  text: string
  requestedAt: number
  sessionId?: string
  sectionLabel?: string
}

interface TerminalDispatchState {
  pendingCommand: PendingCommand | null
  openRequestId: number
}

export const useTerminalDispatchStore = defineStore('terminalDispatch', {
  state: (): TerminalDispatchState => ({
    pendingCommand: null,
    openRequestId: 0,
  }),

  actions: {
    sendToTerminal(command: string, meta?: Record<string, unknown> | null) {
      if (typeof command !== 'string' || command.length === 0) return
      const queued: PendingCommand = { text: command, requestedAt: Date.now() }
      if (meta && typeof meta === 'object') {
        if (typeof meta.sessionId === 'string') queued.sessionId = meta.sessionId
        if (typeof meta.sectionLabel === 'string') queued.sectionLabel = meta.sectionLabel
      }
      this.pendingCommand = queued
      this.openRequestId += 1
    },

    consumePendingCommand(): PendingCommand | null {
      const value = this.pendingCommand
      this.pendingCommand = null
      return value
    },

    peekPendingCommand(): PendingCommand | null {
      return this.pendingCommand
    },
  },
})
