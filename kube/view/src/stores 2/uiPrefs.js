import { defineStore } from 'pinia'

// UI preferences that should survive across navigation and (where useful)
// across reloads. The right-rail width is the canonical example: the user
// resizes the Argus panel, and that width persists.

const STORAGE_KEY = 'argus.uiPrefs.v1'

const DEFAULTS = {
  rightPanelWidth: 340,
  rightPanelMin: 280,
  rightPanelMax: 720,
  chatPopOutOpen: false,
}

function loadFromStorage() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return null
    return parsed
  } catch (e) {
    return null
  }
}

function saveToStorage(state) {
  try {
    const subset = {
      rightPanelWidth: state.rightPanelWidth,
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(subset))
  } catch (e) {
    // Best-effort; localStorage may be unavailable in tests/jsdom edge cases.
  }
}

export const useUIPrefsStore = defineStore('uiPrefs', {
  state: () => {
    const persisted = loadFromStorage() || {}
    return {
      rightPanelWidth: clampWidth(persisted.rightPanelWidth ?? DEFAULTS.rightPanelWidth),
      rightPanelMin: DEFAULTS.rightPanelMin,
      rightPanelMax: DEFAULTS.rightPanelMax,
      chatPopOutOpen: false, // never persist — pop-out is a session-level state
    }
  },

  actions: {
    setRightPanelWidth(px) {
      const next = clampWidth(px)
      if (next === this.rightPanelWidth) return
      this.rightPanelWidth = next
      saveToStorage(this.$state)
    },

    openChatPopOut() {
      this.chatPopOutOpen = true
    },

    closeChatPopOut() {
      this.chatPopOutOpen = false
    },
  },
})

function clampWidth(px) {
  const n = Number(px)
  if (!Number.isFinite(n)) return DEFAULTS.rightPanelWidth
  return Math.max(DEFAULTS.rightPanelMin, Math.min(DEFAULTS.rightPanelMax, n))
}
