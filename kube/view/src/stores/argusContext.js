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

    // pendingPrompt is a one-shot send request from a non-chat view (Config
    // Audit, NetworkPolicy review, ...). When set, ArgusAIChat picks it up,
    // pushes it as the next user message in the active session, and clears
    // it. Routing through the chat panel — instead of letting each view
    // call sendMessage('global', …) on its own useChat instance — ensures
    // the user sees the question, the typing indicator, and the reply in
    // the panel they're actually looking at.
    pendingPrompt: null, // { body, label, sourceId, queuedAt }

    // investigating is true while the agent is processing any request
    // (whether the user typed in chat or another view enqueued a prompt).
    // Views that surface an "Ask Argus" button read this to render the
    // pulse so the user knows work is in flight even after navigating
    // away from the chat panel.
    investigating: false,
    investigatingLabel: '',
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

    // Queue a prompt for the chat panel to send on the user's behalf.
    // Caller is expected to also open the chat pop-out so the message
    // becomes visible.
    enqueuePrompt({ body, label, sourceId } = {}) {
      const text = String(body || '').trim()
      if (!text) return
      this.pendingPrompt = {
        body: text,
        label: String(label || ''),
        sourceId: sourceId || null,
        queuedAt: new Date().toISOString(),
      }
    },

    consumePendingPrompt() {
      const p = this.pendingPrompt
      this.pendingPrompt = null
      return p
    },

    setInvestigating(label) {
      this.investigating = true
      this.investigatingLabel = String(label || '')
    },

    clearInvestigating() {
      this.investigating = false
      this.investigatingLabel = ''
    },
  },
})
