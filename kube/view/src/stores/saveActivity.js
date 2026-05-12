// saveActivity — a single global feed of save/mutation outcomes across the
// app. The bridge layer (composables/useBridge.js) emits an `argus:save`
// bus event for every mutating callGo it sends; this store listens for it
// and forwards to two destinations:
//
//   1. An in-memory ring of recent events (drives the SaveToastStack — the
//      transparent top-right notifications that auto-disappear).
//   2. The persistent notifications store — so every save also lands in the
//      bell panel and can be scrolled back through ("findable").
//
// Pinia handles state persistence; the event bus (lib/bus.ts) handles
// cross-component delivery without crowding the store with event logic.

import { defineStore } from 'pinia'
import { ref } from 'vue'
import { bus } from '../lib/bus'
import { useNotificationsStore } from './notifications'

const RING_MAX = 12
let nextLocal = 1

// CamelCase → space-separated, with the leading verb past-tensed when it
// matches a known mutating prefix. "SaveAnomalyRule" → "Saved Anomaly Rule".
const PAST_TENSE = {
  Save: 'Saved',
  Update: 'Updated',
  Create: 'Created',
  Delete: 'Deleted',
  Apply: 'Applied',
  Move: 'Moved',
  Toggle: 'Toggled',
  Sync: 'Synced',
  Refresh: 'Refreshed',
  Rollback: 'Rolled back',
  Restart: 'Restarted',
  Scale: 'Scaled',
  Set: 'Set',
}

export function deriveActivityLabel(method, status) {
  if (!method) return status === 'error' ? 'Save failed' : 'Save complete'
  // Split on uppercase boundaries: "SaveAnomalyRule" → ["Save","Anomaly","Rule"]
  const parts = String(method).match(/[A-Z][a-z0-9]*/g) || [String(method)]
  const verb = parts[0]
  const rest = parts.slice(1).join(' ')
  const tensed = PAST_TENSE[verb] || verb
  if (status === 'error') {
    return rest ? `Failed to ${verb.toLowerCase()} ${rest}` : `${verb} failed`
  }
  return rest ? `${tensed} ${rest}` : tensed
}

export const useSaveActivityStore = defineStore('saveActivity', () => {
  const events = ref([])
  let listening = false

  function record(input) {
    const now = Date.now()
    // Callers (e.g. credentialAlerts) may hand us a pre-built label — for
    // synthetic events the derived "SaveAnomalyRule" → "Saved Anomaly Rule"
    // grammar doesn't apply ("Failed to credential alert" reads badly).
    const label = input?.label
      ? String(input.label)
      : deriveActivityLabel(input?.method, input?.status)
    const entry = {
      // Stable monotonic id — Date.now() is not unique enough during bursts.
      id: 'sa-' + now.toString(36) + '-' + (nextLocal++).toString(36),
      method: String(input?.method || ''),
      status: input?.status === 'error' ? 'error' : 'ok',
      durationMs: Number.isFinite(input?.durationMs) ? Math.max(0, input.durationMs) : 0,
      error: input?.error ? String(input.error) : '',
      label,
      createdAt: new Date(now).toISOString(),
    }

    // In-memory ring — drives the toast stack. Newest first.
    events.value = [entry, ...events.value].slice(0, RING_MAX)

    // Persist into the bell-panel feed so the user can find this later.
    try {
      const notif = useNotificationsStore()
      notif.add({
        kind: entry.status === 'error' ? 'error' : 'save',
        title: entry.label,
        body: entry.status === 'error'
          ? entry.error
          : entry.durationMs ? `Completed in ${entry.durationMs} ms` : 'Completed',
      })
    } catch {
      // notifications store may not be available in some test contexts;
      // the toast still works without persistence.
    }

    return entry
  }

  function dismiss(id) {
    events.value = events.value.filter((e) => e.id !== id)
  }

  // Idempotent — App.vue calls this on mount. Stays attached for the
  // lifetime of the page; the bridge fires events directly on `window` so
  // there's no per-store cleanup to worry about.
  function attach() {
    if (listening || typeof window === 'undefined') return
    listening = true
    bus.on('argus:save', (detail) => {
      record(detail || {})
    })
  }

  return { events, record, dismiss, attach }
})
