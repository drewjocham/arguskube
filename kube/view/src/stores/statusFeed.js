import { defineStore } from 'pinia'

// statusFeed — the single producer surface for "Argus is currently doing X"
// events. Anything in the app (envprobe, k8s client, ai agent, setup, agent
// connection, ...) calls push() with a StatusEvent and it shows up in the
// bottom <StatusRibbon> and in the notifications panel scroll-back.
//
// Two contracts the rest of the app depends on:
//
//   1. push() is non-blocking and idempotent. Producers can fire freely
//      without worrying about backpressure or duplicates within a small
//      window (we collapse identical (source, message) pairs that arrive
//      within COLLAPSE_WINDOW_MS).
//
//   2. The ring buffer is bounded. The ribbon is best-effort — losing the
//      oldest event when we overflow is correct behavior, not an error.
//      Long-lived audit history lives in the notifications store.
//
// Severity is encoded as a 4px left strip in the ribbon (info gray, warn
// amber, error red) so the ribbon itself stays calm.

const RING_CAPACITY = 200
const COLLAPSE_WINDOW_MS = 1500

const VALID_SEVERITY = new Set(['info', 'warn', 'error'])
const VALID_SOURCE = new Set([
  'envprobe', 'k8s', 'argocd', 'agent', 'ai', 'setup', 'userprofile', 'system',
])

function nowMs() { return Date.now() }
function nowISO() { return new Date().toISOString() }

function newId() {
  if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
  return `s-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

function normalize(raw) {
  const severity = VALID_SEVERITY.has(raw?.severity) ? raw.severity : 'info'
  const source = VALID_SOURCE.has(raw?.source) ? raw.source : 'system'
  const message = String(raw?.message ?? '').trim()
  if (!message) return null
  return {
    id: raw?.id || newId(),
    ts: raw?.ts || nowISO(),
    severity,
    source,
    message,
    detail: raw?.detail ? String(raw.detail) : '',
    actionLabel: raw?.actionLabel ? String(raw.actionLabel) : '',
    actionId: raw?.actionId ? String(raw.actionId) : '',
  }
}

export const useStatusFeedStore = defineStore('statusFeed', {
  state: () => ({
    items: [],            // ring buffer, newest at the end
    pausedUntil: 0,       // ms epoch; ribbon scroll suspended until this time
    expanded: false,      // true when the user clicked the ribbon open
    _lastByKey: new Map(),// (source|message) -> last ms; used for collapse
  }),

  getters: {
    // The ribbon reads scrollItems left-to-right by ts. We give it newest
    // first because the marquee animation pops newest in from the right.
    scrollItems: (s) => s.items.slice().reverse(),
    latest: (s) => (s.items.length ? s.items[s.items.length - 1] : null),
    hasErrors: (s) => s.items.some(e => e.severity === 'error'),
  },

  actions: {
    push(raw) {
      const event = normalize(raw)
      if (!event) return null

      // Collapse a flood of identical (source, message) events. Producers
      // like envprobe re-run every 60s and we don't want the ribbon to
      // chant the same line.
      const key = `${event.source}|${event.message}`
      const last = this._lastByKey.get(key) || 0
      const now = nowMs()
      if (now - last < COLLAPSE_WINDOW_MS) {
        return null
      }
      this._lastByKey.set(key, now)

      this.items.push(event)
      while (this.items.length > RING_CAPACITY) this.items.shift()
      return event.id
    },

    // Producer convenience helpers. Same shape, different default severity.
    info(source, message, extras = {}) {
      return this.push({ ...extras, source, message, severity: 'info' })
    },
    warn(source, message, extras = {}) {
      return this.push({ ...extras, source, message, severity: 'warn' })
    },
    error(source, message, extras = {}) {
      return this.push({ ...extras, source, message, severity: 'error' })
    },

    // Ribbon hover-pause: freeze the scroll for `ms` milliseconds. Calling
    // it again with a later deadline extends; calling with `0` resumes now.
    pauseFor(ms) {
      if (!Number.isFinite(ms) || ms <= 0) {
        this.pausedUntil = 0
        return
      }
      const target = nowMs() + ms
      if (target > this.pausedUntil) this.pausedUntil = target
    },

    resume() { this.pausedUntil = 0 },

    isPaused() { return this.pausedUntil > nowMs() },

    setExpanded(v) { this.expanded = !!v },

    clear() {
      this.items = []
      this._lastByKey.clear()
    },
  },
})

export const __test = {
  RING_CAPACITY,
  COLLAPSE_WINDOW_MS,
  VALID_SEVERITY,
  VALID_SOURCE,
}
