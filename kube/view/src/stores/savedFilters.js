import { defineStore } from 'pinia'

// Saved log-explorer filter sets. Each entry is a named bundle of
// { query, filters: [{field, value}, …], limit } the user can recall later.
// Persisted to localStorage so they survive reloads. Capped at 50 to keep
// the dropdown manageable; users hitting the cap can delete old entries.

const STORAGE_KEY = 'argus.savedFilters.v1'
const MAX_ENTRIES = 50

function newId() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  return `f-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

function loadFromStorage() {
  try {
    const raw = window.localStorage?.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch (_) { return [] }
}

function persist(list) {
  try {
    window.localStorage?.setItem(STORAGE_KEY, JSON.stringify(list))
  } catch (_) { /* best effort */ }
}

export const useSavedFiltersStore = defineStore('savedFilters', {
  state: () => ({
    entries: loadFromStorage(),
  }),

  getters: {
    sortedEntries: (s) => s.entries.slice().sort((a, b) => (b.createdAt || '').localeCompare(a.createdAt || '')),
    findByName: (s) => (name) => s.entries.find(e => e.name === name) || null,
  },

  actions: {
    save(name, payload) {
      const trimmed = String(name || '').trim()
      if (!trimmed) return null
      const existing = this.entries.find(e => e.name === trimmed)
      const data = {
        name: trimmed,
        query: String(payload?.query || ''),
        filters: Array.isArray(payload?.filters) ? payload.filters.map(f => ({ field: String(f.field || ''), value: String(f.value || '') })) : [],
        limit: Number.isFinite(Number(payload?.limit)) ? Number(payload.limit) : null,
        updatedAt: new Date().toISOString(),
      }
      if (existing) {
        Object.assign(existing, data)
        persist(this.entries)
        return existing.id
      }
      const entry = { id: newId(), createdAt: data.updatedAt, ...data }
      this.entries.push(entry)
      while (this.entries.length > MAX_ENTRIES) this.entries.shift()
      persist(this.entries)
      return entry.id
    },

    remove(id) {
      const idx = this.entries.findIndex(e => e.id === id)
      if (idx >= 0) {
        this.entries.splice(idx, 1)
        persist(this.entries)
      }
    },

    clear() {
      this.entries = []
      persist(this.entries)
    },
  },
})

export const __test = { STORAGE_KEY, MAX_ENTRIES }
