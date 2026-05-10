import { defineStore } from 'pinia'

// argusContext is the bridge between "the user selected something on screen"
// and "the AI agent should know what they mean by 'this'." When a view
// surfaces a focused entity (a Config Audit finding, a network policy, an
// alert, etc.) it calls setContext({ kind, label, body, sourceId }). The
// chat UI shows a chip above the input — the user can clear it with × or
// type a question and the body is prepended to the next message.
//
// Only one context is active at a time; setting a new one replaces the
// previous. clearContext() detaches without sending anything.

export const useArgusContextStore = defineStore('argusContext', {
  state: () => ({
    pending: null, // { kind, label, body, sourceId, setAt }
  }),

  getters: {
    hasContext: (s) => s.pending !== null,
    label: (s) => s.pending?.label || '',
  },

  actions: {
    setContext(ctx) {
      if (!ctx || typeof ctx !== 'object') {
        this.pending = null
        return
      }
      this.pending = {
        kind: String(ctx.kind || 'item'),
        label: String(ctx.label || ''),
        body: String(ctx.body || ''),
        sourceId: ctx.sourceId || null,
        setAt: new Date().toISOString(),
      }
    },

    clearContext() {
      this.pending = null
    },

    // Called by the chat composer when the user sends a message. Returns
    // the body to prepend (or '' if no context) and clears the pending
    // context so it doesn't get re-attached to follow-ups.
    consumeForSend() {
      if (!this.pending) return ''
      const body = this.pending.body || ''
      this.pending = null
      return body
    },
  },
})
