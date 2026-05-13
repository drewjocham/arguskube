import { defineStore } from 'pinia'
import { DEFAULT_TABS, SECTIONS, SECTION_ORDER, isValidTab } from '../lib/sectionTabs'

// sectionTabs — Pinia store remembering which tab the user last
// opened inside each section. Persisted to localStorage so a reload
// lands the user back on the tab they were using, not the section
// default.
//
// State shape: { tabs: { sectionId: tabId } }
//
// Invariants:
//   * Every section has an entry (initialized from DEFAULT_TABS).
//   * tabId is always a known tab for its section. Persisted values
//     that fail isValidTab() are dropped on load — the user's
//     localStorage isn't authoritative against the live SECTIONS map.

const STORAGE_KEY = 'argus.sectionTabs.v1'

function loadFromStorage() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return null
    return parsed
  } catch {
    return null
  }
}

function saveToStorage(tabs) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(tabs))
  } catch {
    // Best-effort — private windows / quota errors don't fail nav.
  }
}

function buildInitialTabs() {
  const persisted = loadFromStorage() || {}
  const out = {}
  for (const sectionId of SECTION_ORDER) {
    const stored = persisted[sectionId]
    if (typeof stored === 'string' && isValidTab(sectionId, stored)) {
      out[sectionId] = stored
    } else {
      out[sectionId] = DEFAULT_TABS[sectionId] || SECTIONS[sectionId].tabs[0]?.id || ''
    }
  }
  return out
}

export const useSectionTabsStore = defineStore('sectionTabs', {
  state: () => ({
    tabs: buildInitialTabs(),
  }),

  getters: {
    /**
     * Returns the active tab id for a section. Falls back to the
     * default when the section id is unknown — defensive so a stale
     * caller can't crash navigation.
     */
    activeTab: (state) => (sectionId) => {
      if (!sectionId) return ''
      return state.tabs[sectionId] || DEFAULT_TABS[sectionId] || ''
    },
  },

  actions: {
    /**
     * Sets the active tab inside a section. Unknown sections or tabs
     * are rejected silently — the alternative (throwing) would break
     * a benign Cmd+K palette dispatch when the tab catalog drifts.
     */
    setTab(sectionId, tabId) {
      if (!sectionId || !tabId) return
      if (!isValidTab(sectionId, tabId)) return
      if (this.tabs[sectionId] === tabId) return
      this.tabs[sectionId] = tabId
      saveToStorage(this.tabs)
    },

    /**
     * Resets every section to its DEFAULT_TABS choice. Surfaced as a
     * "Reset tab memory" button in Settings (future C2 follow-up).
     */
    resetAll() {
      const defaults = {}
      for (const sectionId of SECTION_ORDER) {
        defaults[sectionId] = DEFAULT_TABS[sectionId] || SECTIONS[sectionId].tabs[0]?.id || ''
      }
      this.tabs = defaults
      saveToStorage(this.tabs)
    },
  },
})

// Test-only helpers.
export const __test = {
  STORAGE_KEY,
  loadFromStorage,
  buildInitialTabs,
}
