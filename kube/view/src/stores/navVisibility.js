import { defineStore } from 'pinia'
import { SECTIONS, SECTION_ORDER } from '../lib/sectionTabs'

// navVisibility — which sidebar sections are visible. The "clean
// defaults" principle from the master plan: on first launch, show
// the 5 core sections most users need every day. The 4 optional
// sections (config, storage, knowledge, admin) appear automatically
// when the backend reports that the matching subsystem is configured;
// otherwise they stay hidden until the user enables them in Settings.
//
// State shape: { visible: { sectionId: true|false } }
//
// Two layers:
//
//   1. STATIC: the CORE_SECTIONS list is always visible on first run.
//
//   2. DYNAMIC: initialize() probes the backend for the optional
//      subsystems. The probe result is OR'd into visibility — once
//      true, it sticks (we never auto-hide a section the user has
//      seen). The user's explicit toggles always win over both layers.

const STORAGE_KEY = 'argus.navVisibility.v1'

// Sections that should always show by default. The other 4 (config,
// storage, knowledge, admin) are opt-in either via a probe or via the
// Settings → Navigation panel.
// Admin is core because it owns Setup + Settings — surfaces the user
// needs to reach to configure ANY optional section. Hiding it by
// default created a discovery dead-end where the only path to enable
// other sections was the right-click menu or Cmd+K. The right fix is
// to keep it visible; the right-click "Open settings" path is a
// fallback for users who already navigated away from it.
const CORE_SECTIONS = Object.freeze([
  'monitoring',
  'cluster',
  'workloads',
  'network',
  'operations',
  'admin',
])

const OPTIONAL_SECTIONS = Object.freeze(
  SECTION_ORDER.filter((id) => !CORE_SECTIONS.includes(id)),
)

// Display hints for the Settings → Navigation toggle list. Each entry
// explains *why* a user might want to enable an optional section.
const SECTION_HINTS = Object.freeze({
  config: 'Config Maps, Secrets, HPAs',
  storage: 'PVCs, PVs, Storage Classes — useful for stateful workloads',
  knowledge: 'Local documents + S3-backed notebooks',
  workspace: 'Google Workspace integrations (Phase 1A: connections only)',
  admin: 'Setup & Tools and Settings',
})

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

function saveToStorage(state) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // Best-effort.
  }
}

function defaultVisibility() {
  const out = {}
  for (const id of SECTION_ORDER) {
    out[id] = CORE_SECTIONS.includes(id)
  }
  return out
}

function buildInitialVisibility() {
  const persisted = loadFromStorage()
  if (!persisted || typeof persisted.visible !== 'object') {
    return defaultVisibility()
  }
  // Merge persisted into the canonical section list so new sections
  // shipped in a later release pick up their default visibility instead
  // of being silently hidden.
  const out = defaultVisibility()
  for (const id of Object.keys(persisted.visible)) {
    if (SECTIONS[id]) out[id] = !!persisted.visible[id]
  }
  // Core sections are ALWAYS visible. Earlier builds let users hide
  // them (including admin, which owns Settings) and lock themselves
  // out. Enforce visibility here so a stale persisted "admin: false"
  // doesn't survive the next launch.
  for (const id of CORE_SECTIONS) {
    out[id] = true
  }
  return out
}

export const useNavVisibilityStore = defineStore('navVisibility', {
  state: () => ({
    visible: buildInitialVisibility(),
    initialized: false, // flips true after initialize() resolves
  }),

  getters: {
    // ordered list of currently-visible section ids
    visibleOrder: (state) =>
      SECTION_ORDER.filter((id) => state.visible[id]),

    sections: () =>
      SECTION_ORDER.map((id) => ({
        id,
        label: SECTIONS[id].label,
        core: CORE_SECTIONS.includes(id),
        hint: SECTION_HINTS[id] || '',
      })),
  },

  actions: {
    isVisible(sectionId) {
      return !!this.visible[sectionId]
    },

    /** Toggle a section. Core sections cannot be hidden — they're
     *  the surfaces every user needs (including Admin → Settings). */
    toggle(sectionId) {
      if (!SECTIONS[sectionId]) return
      if (CORE_SECTIONS.includes(sectionId) && this.visible[sectionId]) {
        // Toggling a visible core section off is suppressed.
        return
      }
      this.visible = { ...this.visible, [sectionId]: !this.visible[sectionId] }
      saveToStorage({ visible: this.visible })
    },

    show(sectionId) {
      if (!SECTIONS[sectionId] || this.visible[sectionId]) return
      this.visible = { ...this.visible, [sectionId]: true }
      saveToStorage({ visible: this.visible })
    },

    hide(sectionId) {
      if (!SECTIONS[sectionId] || !this.visible[sectionId]) return
      if (CORE_SECTIONS.includes(sectionId)) return // core can't be hidden
      this.visible = { ...this.visible, [sectionId]: false }
      saveToStorage({ visible: this.visible })
    },

    /** Reset to the default "5 core sections only" first-launch view. */
    resetToDefaults() {
      this.visible = defaultVisibility()
      saveToStorage({ visible: this.visible })
    },

    /**
     * Probe-aware initialization. Run once after auth lands. Each
     * probe is non-blocking — a failure leaves the section in its
     * current state (hidden, unless the user has already enabled
     * it). We never auto-HIDE: once a section is visible, only the
     * user can hide it.
     *
     * The `probes` argument lets the caller inject lightweight
     * callable probes for testability. Each probe returns a
     * promise that resolves to a truthy value when the matching
     * section should be revealed.
     */
    async initialize(probes = {}) {
      // Default no-op probes: the producer wiring in App.vue will
      // overlay real callGo() calls. Until then we just mark the
      // store initialized so the UI doesn't keep re-trying.
      const tasks = OPTIONAL_SECTIONS.map(async (id) => {
        const probe = probes[id]
        if (typeof probe !== 'function') return
        if (this.visible[id]) return // user already opted in
        try {
          const reveal = await probe()
          if (reveal) this.show(id)
        } catch {
          // Probe failure — leave the section hidden. The user can
          // still enable it manually in Settings.
        }
      })
      await Promise.all(tasks)
      this.initialized = true
    },
  },
})

export const __test = {
  STORAGE_KEY,
  CORE_SECTIONS,
  OPTIONAL_SECTIONS,
  SECTION_HINTS,
  defaultVisibility,
  buildInitialVisibility,
}
