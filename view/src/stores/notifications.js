import { defineStore } from 'pinia'

// Argus notifications — periodic spot-check findings, sync results, scan
// outcomes, anything the user should be able to scroll back through. Capped
// at `maxItems` (default 500, configurable in Settings) — when full, the
// oldest entry is evicted. Persisted to localStorage so the list survives
// reloads. NOT acknowledged via per-item dismissal: the user can clear one,
// clear all, or just leave them and let the cap roll.

const STORAGE_KEY = 'kubewatcher.notifications.v1'
const SETTINGS_KEY = 'kubewatcher.notifications.settings.v1'
const DEFAULT_MAX = 500
const ABSOLUTE_MAX = 5000 // safety: localStorage isn't infinite

function nowISO() {
  return new Date().toISOString()
}

function newId() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  return `n-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

function load(key, fallback) {
  try {
    const raw = window.localStorage?.getItem(key)
    if (!raw) return fallback
    const parsed = JSON.parse(raw)
    return parsed ?? fallback
  } catch (_) {
    return fallback
  }
}

function save(key, value) {
  try {
    window.localStorage?.setItem(key, JSON.stringify(value))
  } catch (_) { /* best effort */ }
}

function clampMax(n) {
  const v = Number(n)
  if (!Number.isFinite(v)) return DEFAULT_MAX
  return Math.min(ABSOLUTE_MAX, Math.max(1, Math.floor(v)))
}

function loadSettings() {
  const stored = load(SETTINGS_KEY, null)
  if (!stored || typeof stored !== 'object') return { maxItems: DEFAULT_MAX }
  return { maxItems: clampMax(stored.maxItems ?? DEFAULT_MAX) }
}

function loadItems() {
  const stored = load(STORAGE_KEY, [])
  return Array.isArray(stored) ? stored : []
}

export const useNotificationsStore = defineStore('notifications', {
  state: () => ({
    items: loadItems(),
    settings: loadSettings(),
    panelOpen: false,
  }),

  getters: {
    unreadCount: (s) => s.items.filter(n => !n.read).length,
    sortedItems: (s) => s.items.slice().sort((a, b) => (b.createdAt || '').localeCompare(a.createdAt || '')),
  },

  actions: {
    add(notification) {
      const n = {
        id: newId(),
        kind: notification?.kind || 'info', // 'info' | 'spot-check' | 'scan' | 'alert' | 'warn' | 'error'
        title: String(notification?.title || 'Argus'),
        body: String(notification?.body || ''),
        createdAt: nowISO(),
        read: false,
        rerunnable: !!notification?.rerunnable,
        rerunPayload: notification?.rerunPayload || null,
        meta: notification?.meta || {},
      }
      this.items.push(n)
      // Evict oldest above the cap.
      while (this.items.length > this.settings.maxItems) {
        // We added newest at the end, so shift() drops the oldest.
        this.items.shift()
      }
      this._persist()
      return n.id
    },

    remove(id) {
      const idx = this.items.findIndex(n => n.id === id)
      if (idx >= 0) {
        this.items.splice(idx, 1)
        this._persist()
      }
    },

    clearAll() {
      this.items = []
      this._persist()
    },

    markRead(id) {
      const n = this.items.find(x => x.id === id)
      if (n && !n.read) {
        n.read = true
        this._persist()
      }
    },

    markAllRead() {
      let changed = false
      for (const n of this.items) {
        if (!n.read) { n.read = true; changed = true }
      }
      if (changed) this._persist()
    },

    setMaxItems(value) {
      const next = clampMax(value)
      if (next === this.settings.maxItems) return
      this.settings.maxItems = next
      // Trim immediately if the new cap is lower than the current length.
      while (this.items.length > next) this.items.shift()
      save(SETTINGS_KEY, this.settings)
      this._persist()
    },

    openPanel() { this.panelOpen = true },
    closePanel() { this.panelOpen = false },
    togglePanel() { this.panelOpen = !this.panelOpen },

    _persist() {
      save(STORAGE_KEY, this.items)
    },
  },
})

export const __test = {
  STORAGE_KEY,
  SETTINGS_KEY,
  DEFAULT_MAX,
  ABSOLUTE_MAX,
}
