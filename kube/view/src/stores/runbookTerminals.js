import { defineStore } from 'pinia'

// runbookTerminals tracks which terminal session a code block in a runbook
// should target. The user's spec:
//
//   - Code blocks under the same heading default to the same session.
//   - Code blocks under a NEW heading default to a new session for that
//     section.
//   - Per-document, the user can PIN one session and every block in the
//     doc routes to that pinned session instead.
//   - Per-block, the user can override the target session manually.
//   - Pin state persists. It scoped to the document (runbookId) — the
//     same pin doesn't bleed across runbooks.
//
// We persist pin state and per-block overrides in localStorage so the
// user's chosen routing survives reloads.

const STORAGE_KEY = 'argus.runbookTerminals.v1'

function loadFromStorage() {
  try {
    const raw = window.localStorage?.getItem(STORAGE_KEY)
    if (!raw) return { pinned: {}, overrides: {} }
    const parsed = JSON.parse(raw)
    return {
      pinned: parsed?.pinned && typeof parsed.pinned === 'object' ? parsed.pinned : {},
      overrides: parsed?.overrides && typeof parsed.overrides === 'object' ? parsed.overrides : {},
    }
  } catch (_) { return { pinned: {}, overrides: {} } }
}

function persist(state) {
  try {
    window.localStorage?.setItem(STORAGE_KEY, JSON.stringify({
      pinned: state.pinned,
      overrides: state.overrides,
    }))
  } catch (_) { /* best effort */ }
}

// A session id is namespaced by runbookId so two runbooks can each have
// e.g. a "verify-pods" section without colliding.
function buildSessionId(runbookId, scope) {
  return `${runbookId || 'untitled'}::${scope}`
}

export const useRunbookTerminalsStore = defineStore('runbookTerminals', {
  // state() reads localStorage each time a fresh Pinia instantiates this
  // store, rather than relying on a module-level snapshot captured at
  // first import. That keeps tests honest under setActivePinia(createPinia())
  // and means a popped-out terminal window picks up dashboard pins on its
  // first state init without a full module reload.
  state: () => {
    const persisted = loadFromStorage()
    return {
      pinned: { ...persisted.pinned },       // { [runbookId]: 'pin' | sectionId }
      overrides: { ...persisted.overrides }, // { [`${runbookId}::block:${codeIndex}`]: targetSessionId }
    }
  },

  getters: {
    isPinned: (s) => (runbookId) => s.pinned[runbookId] != null,
    pinnedSessionFor: (s) => (runbookId) => {
      const raw = s.pinned[runbookId]
      if (!raw) return null
      return buildSessionId(runbookId, raw === 'pin' ? 'pin' : raw)
    },
  },

  actions: {
    // resolveTarget: returns the sessionId a code block should run against.
    // Order:
    //   1. Per-block override (user clicked "send to <session>" on this block)
    //   2. Per-document pin (user pinned a session for the whole runbook)
    //   3. Section default (one session per heading)
    resolveTarget(runbookId, sectionId, codeIndex) {
      const overrideKey = `${runbookId}::block:${codeIndex}`
      if (this.overrides[overrideKey]) return this.overrides[overrideKey]
      if (this.pinned[runbookId]) {
        return buildSessionId(runbookId, this.pinned[runbookId] === 'pin' ? 'pin' : this.pinned[runbookId])
      }
      return buildSessionId(runbookId, sectionId || 'default')
    },

    pinDocument(runbookId, sectionId) {
      // Use 'pin' as the marker when pinning to a document-wide session.
      // When pinning to a SPECIFIC section, store that sectionId so later
      // blocks in different sections still land on the pinned section's
      // session.
      this.pinned[runbookId] = sectionId || 'pin'
      persist(this.$state)
    },

    unpinDocument(runbookId) {
      delete this.pinned[runbookId]
      persist(this.$state)
    },

    setBlockOverride(runbookId, codeIndex, sessionId) {
      const key = `${runbookId}::block:${codeIndex}`
      if (sessionId) {
        this.overrides[key] = sessionId
      } else {
        delete this.overrides[key]
      }
      persist(this.$state)
    },

    clearDocument(runbookId) {
      // Wipe pin + every block override for this runbook. Used when the user
      // wants a fresh start on a doc.
      delete this.pinned[runbookId]
      for (const k of Object.keys(this.overrides)) {
        if (k.startsWith(`${runbookId}::`)) delete this.overrides[k]
      }
      persist(this.$state)
    },
  },
})

export const __test = { STORAGE_KEY, buildSessionId }
