import { defineStore } from 'pinia'

// Cross-component event bus for "send a command to the embedded terminal".
//
// Any view (Argus AI chat code blocks, the right-rail chat, etc.) calls
// `sendToTerminal(cmd)`. App.vue listens for openRequestId ticks and ensures
// the terminal panel is visible; TerminalView watches `pendingCommand` and
// writes the command to the active xterm session.
//
// Safety: commands are queued WITHOUT a trailing newline. The user has to
// press Enter in the terminal to actually execute. This avoids auto-running
// shell commands suggested by the LLM while still saving the
// "copy → switch to terminal → paste" round-trip.
export const useTerminalDispatchStore = defineStore('terminalDispatch', {
  state: () => ({
    pendingCommand: null, // { text, requestedAt, sessionId?, sectionLabel? } | null
    openRequestId: 0,     // increments each time a view requests the terminal panel be opened
  }),

  actions: {
    // sendToTerminal accepts an optional metadata bag. `sessionId` is used by
    // the upcoming multi-PTY backend to route to the right session; today
    // the embedded terminal is single-session so the metadata is also used
    // to prepend a "# section: …" header so the user sees which logical
    // session the command belongs to.
    sendToTerminal(command, meta) {
      if (typeof command !== 'string' || command.length === 0) return
      const queued = { text: command, requestedAt: Date.now() }
      if (meta && typeof meta === 'object') {
        if (typeof meta.sessionId === 'string') queued.sessionId = meta.sessionId
        if (typeof meta.sectionLabel === 'string') queued.sectionLabel = meta.sectionLabel
      }
      this.pendingCommand = queued
      this.openRequestId += 1
    },

    consumePendingCommand() {
      const value = this.pendingCommand
      this.pendingCommand = null
      return value
    },

    peekPendingCommand() {
      return this.pendingCommand
    },
  },
})
