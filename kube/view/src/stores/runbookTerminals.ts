import { defineStore } from 'pinia'

const STORAGE_KEY = 'argus.runbookTerminals.v1'

interface PersistedState {
  pinned: Record<string, string>
  overrides: Record<string, string>
}

interface RunbookTerminalsState {
  pinned: Record<string, string>
  overrides: Record<string, string>
}

function loadFromStorage(): PersistedState {
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

function persist(state: RunbookTerminalsState) {
  try {
    window.localStorage?.setItem(STORAGE_KEY, JSON.stringify({
      pinned: state.pinned,
      overrides: state.overrides,
    }))
  } catch (_) { /* best effort */ }
}

function buildSessionId(runbookId: string, scope: string): string {
  return `${runbookId || 'untitled'}::${scope}`
}

export const useRunbookTerminalsStore = defineStore('runbookTerminals', {
  state: (): RunbookTerminalsState => {
    const persisted = loadFromStorage()
    return {
      pinned: { ...persisted.pinned },
      overrides: { ...persisted.overrides },
    }
  },

  getters: {
    isPinned: (s) => (runbookId: string): boolean => s.pinned[runbookId] != null,
    pinnedSessionFor: (s) => (runbookId: string): string | null => {
      const raw = s.pinned[runbookId]
      if (!raw) return null
      return buildSessionId(runbookId, raw === 'pin' ? 'pin' : raw)
    },
  },

  actions: {
    resolveTarget(runbookId: string, sectionId: string, codeIndex: number): string {
      const overrideKey = `${runbookId}::block:${codeIndex}`
      if (this.overrides[overrideKey]) return this.overrides[overrideKey]
      if (this.pinned[runbookId]) {
        return buildSessionId(runbookId, this.pinned[runbookId] === 'pin' ? 'pin' : this.pinned[runbookId])
      }
      return buildSessionId(runbookId, sectionId || 'default')
    },

    pinDocument(runbookId: string, sectionId?: string) {
      this.pinned[runbookId] = sectionId || 'pin'
      persist(this.$state)
    },

    unpinDocument(runbookId: string) {
      delete this.pinned[runbookId]
      persist(this.$state)
    },

    setBlockOverride(runbookId: string, codeIndex: number, sessionId: string | null) {
      const key = `${runbookId}::block:${codeIndex}`
      if (sessionId) {
        this.overrides[key] = sessionId
      } else {
        delete this.overrides[key]
      }
      persist(this.$state)
    },

    clearDocument(runbookId: string) {
      delete this.pinned[runbookId]
      for (const k of Object.keys(this.overrides)) {
        if (k.startsWith(`${runbookId}::`)) delete this.overrides[k]
      }
      persist(this.$state)
    },
  },
})

export const __test = { STORAGE_KEY, buildSessionId }
