import { defineStore } from 'pinia'

// setupChecklist — the single source of truth for the "Get Argus ready"
// section in Settings. Producers (composables that own a probe) call
// upsert() when their state changes; the panel reads `items` and renders
// one row per entry. When everything is `ok`, the UI collapses to a
// single "Argus is ready" pill.
//
// Design intent:
//
//   * The store does NOT run probes itself. Probes register their result.
//     This keeps the store thin and lets producers own their own lifecycle
//     (some are watchers, some are periodic, some are one-shot).
//
//   * Items are sorted by `status` first (blockers first), then by
//     `priority` (lower = earlier), then alphabetically by id. The status
//     weights are baked in so producers don't have to think about it.
//
//   * The action is optional. Rows without an action are passive (just
//     informational). Rows with one render a single right-aligned button.
//     The button can either fire a callback OR emit an actionId that the
//     parent component dispatches — both shapes are supported so probes
//     don't have to import view code.

// Status weight controls sort order. Smaller = closer to the top.
const STATUS_WEIGHT = {
  error: 0,    // critical blocker
  todo: 1,     // user must do something
  warn: 2,     // working but degraded
  running: 3,  // probe in flight
  ok: 4,       // everything's fine
}

const VALID_STATUS = new Set(Object.keys(STATUS_WEIGHT))

function normalize(raw) {
  if (!raw || typeof raw !== 'object' || !raw.id) return null
  const status = VALID_STATUS.has(raw.status) ? raw.status : 'todo'
  const item = {
    id: String(raw.id),
    title: String(raw.title || raw.id),
    status,
    priority: Number.isFinite(raw.priority) ? raw.priority : 100,
    detail: raw.detail ? String(raw.detail) : '',
    source: raw.source ? String(raw.source) : '',
    action: null,
  }
  if (raw.action && typeof raw.action === 'object') {
    const { label, dispatch, actionId } = raw.action
    if (label && (typeof dispatch === 'function' || actionId)) {
      item.action = {
        label: String(label),
        dispatch: typeof dispatch === 'function' ? dispatch : null,
        actionId: actionId ? String(actionId) : '',
      }
    }
  }
  return item
}

export const useSetupChecklistStore = defineStore('setupChecklist', {
  state: () => ({
    // Map keyed by id so upserts replace cleanly. We expose a derived
    // array via the items getter.
    _byId: {},
  }),

  getters: {
    items: (s) => {
      const arr = Object.values(s._byId)
      arr.sort((a, b) => {
        const dw = STATUS_WEIGHT[a.status] - STATUS_WEIGHT[b.status]
        if (dw !== 0) return dw
        const dp = a.priority - b.priority
        if (dp !== 0) return dp
        return a.id.localeCompare(b.id)
      })
      return arr
    },
    blockerCount: (s) => Object.values(s._byId).filter(i => i.status === 'error' || i.status === 'todo').length,
    warnCount: (s) => Object.values(s._byId).filter(i => i.status === 'warn').length,
    allGreen: (s) => {
      const arr = Object.values(s._byId)
      if (arr.length === 0) return false
      return arr.every(i => i.status === 'ok')
    },
  },

  actions: {
    upsert(raw) {
      const next = normalize(raw)
      if (!next) return
      this._byId = { ...this._byId, [next.id]: next }
    },
    remove(id) {
      if (!id || !(id in this._byId)) return
      const next = { ...this._byId }
      delete next[id]
      this._byId = next
    },
    clear() {
      this._byId = {}
    },
  },
})

export const __test = {
  STATUS_WEIGHT,
  VALID_STATUS,
}
