// notificationGuard — a single traffic-cop in front of every "something is
// wrong" notification this app fires. Three responsibilities:
//
//   1. Dedupe — collapse identical (source + status) repeats into a single
//      pulse. Caller hands us an event, we decide whether to actually emit.
//   2. Spam detection — count fires per source per rolling window. If a
//      source fires more than `spamThreshold` times in `spamWindowMs`, we
//      silence it: the user gets ONE static warning that needs to be
//      acknowledged, the noisy source goes quiet for `silenceMs` (capped
//      at 24 h), and a recovery message fires when silence ends.
//   3. Manual silence/unsilence — the user (or Argus, via the chat) can
//      silence a source explicitly through this same store. Same plumbing,
//      same guarantees.
//
// All settings (thresholds, default silence duration, max silence) are
// persisted to localStorage and exposed in Settings → Watchers &
// Notifications. Every silenced banner carries a deep-link anchor so the
// "configure this" CTA jumps the user straight to the relevant setting.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { bus } from '../lib/bus'
import { useNotificationsStore } from './notifications'

const STORAGE_KEY = 'kw-notification-guard/v1'
const MAX_SILENCE_MS = 24 * 60 * 60 * 1000   // hard cap — never silence longer than 24 h

const DEFAULTS = {
  spamThreshold: 5,             // > this many fires from one source...
  spamWindowMs: 5 * 60 * 1000,  // ...inside this rolling window...
  defaultSilenceMs: 60 * 60 * 1000, // ...silences the source for an hour by default
  rememberAcksAcrossReloads: true,
  enabled: true,
}

function loadSettings() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return { ...DEFAULTS }
    const parsed = JSON.parse(raw)
    return { ...DEFAULTS, ...(parsed || {}) }
  } catch { return { ...DEFAULTS } }
}

function persistSettings(s) {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(s)) } catch { /* ignore */ }
}

const STATUS_TITLES = {
  expired: 'token expired',
  invalid: 'token rejected',
  error:   'check failed',
  warn:    'warning',
  fail:    'failed',
}

function clampSilence(ms, settings) {
  const cap = Math.min(MAX_SILENCE_MS, settings?.defaultSilenceMs || MAX_SILENCE_MS)
  return Math.max(60_000, Math.min(MAX_SILENCE_MS, Number.isFinite(ms) ? ms : cap))
}

export const useNotificationGuardStore = defineStore('notificationGuard', () => {
  const settings = ref(loadSettings())

  // Per-source rolling counters and dedupe state.
  //   { lastStatus, firedAt, recentFires: number[] (timestamps) }
  const sources = ref({})

  // Active silences. Keyed by source.
  //   { until: epoch_ms, reason: 'spam' | 'manual' | 'argus',
  //     pendingCount: number, acknowledged: bool, anchor?: string,
  //     label: string }
  const silences = ref({})

  // ---- Settings management ----

  function setSettings(patch) {
    settings.value = { ...settings.value, ...patch }
    if (Number.isFinite(settings.value.defaultSilenceMs)) {
      settings.value.defaultSilenceMs = Math.min(MAX_SILENCE_MS, settings.value.defaultSilenceMs)
    }
    persistSettings(settings.value)
  }

  // ---- Public API used by callers (watcher results, ad-hoc alerts) ----

  // observe(event) — primary entry point. Returns true if the alert was
  // emitted to the user, false if it was suppressed (deduped or silenced
  // or replaced with a spam warning).
  //
  // event shape: {
  //   source: string,            // unique id (watcher id or feature key)
  //   label: string,             // human title for "Source X"
  //   status: string,            // 'ok' | 'warn' | 'expired' | 'invalid' | 'error'
  //   message?: string,          // body text shown in toast + bell
  //   anchor?: string,           // settings deep-link (configureAnchor)
  //   recoveryStatuses?: string[]  // statuses that count as "recovered"
  // }
  function observe(event) {
    if (!event || !event.source || !settings.value.enabled) return false
    const source = event.source
    const status = event.status || 'warn'
    const recoveryStatuses = event.recoveryStatuses || ['ok', 'valid', 'present']

    const prev = sources.value[source] || { lastStatus: '', firedAt: 0, recentFires: [] }

    // Recovery flow: previously bad → now good. Clear silence + emit a
    // green "all clear" toast so the user knows the warning is resolved.
    if (recoveryStatuses.includes(status) && prev.lastStatus && !recoveryStatuses.includes(prev.lastStatus)) {
      sources.value = {
        ...sources.value,
        [source]: { lastStatus: status, firedAt: 0, recentFires: [] },
      }
      const sil = silences.value[source]
      if (sil) {
        const next = { ...silences.value }
        delete next[source]
        silences.value = next
      }
      _fireRecovery(source, event)
      return true
    }

    if (recoveryStatuses.includes(status)) {
      // Still healthy. Just update the last-known status.
      sources.value = {
        ...sources.value,
        [source]: { ...prev, lastStatus: status },
      }
      return false
    }

    const now = Date.now()

    // Currently silenced? Increment the pending counter and drop.
    const sil = silences.value[source]
    if (sil && sil.until > now) {
      silences.value = {
        ...silences.value,
        [source]: { ...sil, pendingCount: (sil.pendingCount || 0) + 1 },
      }
      return false
    }
    if (sil && sil.until <= now) {
      // Silence expired — clear it and let the rest of the function fire
      // the alert normally so the user knows the source is back.
      const next = { ...silences.value }
      delete next[source]
      silences.value = next
      _fireRecovery(source, {
        ...event,
        status: 'ok',
        message: `Auto-resumed after silence (${sil.count || 0} suppressed).`,
      })
    }

    // Dedupe — same status in a row, no recovery in between, suppress.
    if (prev.lastStatus === status) {
      const updated = _trackFire(prev, now)
      // Spam check still runs even on dedupe — repeated fires of the
      // *same* alert can trip the spam guard too.
      if (_isSpam(updated.recentFires)) {
        _silenceForSpam(source, event, updated.recentFires.length)
        sources.value = { ...sources.value, [source]: { ...updated, lastStatus: status } }
        return false
      }
      sources.value = { ...sources.value, [source]: { ...updated, lastStatus: status } }
      return false
    }

    // Spam check on transition fires too.
    const updated = _trackFire(prev, now)
    if (_isSpam(updated.recentFires)) {
      _silenceForSpam(source, event, updated.recentFires.length)
      sources.value = { ...sources.value, [source]: { ...updated, lastStatus: status, firedAt: now } }
      return false
    }

    // Normal fire path. Routes to bell + SaveToastStack.
    _emitAlert(source, event, status)
    sources.value = {
      ...sources.value,
      [source]: { ...updated, lastStatus: status, firedAt: now },
    }
    return true
  }

  // Manually silence a source. Used by the Settings UI and by Argus chat.
  function silence(source, durationMs, opts = {}) {
    if (!source) return
    const dur = clampSilence(durationMs ?? settings.value.defaultSilenceMs, settings.value)
    silences.value = {
      ...silences.value,
      [source]: {
        until: Date.now() + dur,
        reason: opts.reason || 'manual',
        pendingCount: 0,
        acknowledged: true,    // user-initiated silences don't need an ack
        anchor: opts.anchor || '',
        label: opts.label || source,
        count: 0,
      },
    }
  }

  // Lift an active silence early.
  function unsilence(source) {
    if (!silences.value[source]) return
    const next = { ...silences.value }
    delete next[source]
    silences.value = next
  }

  function acknowledge(source) {
    const sil = silences.value[source]
    if (!sil) return
    silences.value = {
      ...silences.value,
      [source]: { ...sil, acknowledged: true },
    }
  }

  // ---- View helpers (used by Settings + AlertsView + Argus context) ----

  const activeSilences = computed(() =>
    Object.entries(silences.value)
      .map(([source, s]) => ({ source, ...s }))
      .sort((a, b) => a.until - b.until)
  )

  const pendingAcks = computed(() =>
    activeSilences.value.filter((s) => !s.acknowledged)
  )

  function snapshotForArgus() {
    return {
      generatedAt: new Date().toISOString(),
      settings: { ...settings.value },
      silences: activeSilences.value.map((s) => ({
        source: s.source,
        label:  s.label,
        until:  new Date(s.until).toISOString(),
        reason: s.reason,
        suppressedSinceLastFire: s.pendingCount,
      })),
      sources: Object.entries(sources.value).map(([id, s]) => ({
        id,
        lastStatus: s.lastStatus,
        recentFireCount: s.recentFires?.length || 0,
      })),
    }
  }

  // ---- Internals ----

  function _trackFire(prev, now) {
    const window = settings.value.spamWindowMs
    const recent = (prev.recentFires || []).filter((t) => now - t < window)
    recent.push(now)
    return { ...prev, recentFires: recent }
  }

  function _isSpam(recentFires) {
    return (recentFires?.length || 0) > settings.value.spamThreshold
  }

  function _silenceForSpam(source, event, count) {
    const dur = clampSilence(settings.value.defaultSilenceMs, settings.value)
    silences.value = {
      ...silences.value,
      [source]: {
        until: Date.now() + dur,
        reason: 'spam',
        pendingCount: 0,
        acknowledged: false,    // spam-driven silences DO need user ack
        anchor: event.anchor || 'watchers-notifications',
        label: event.label || source,
        count,
      },
    }

    // One static "X notifications silenced" message via the bell + a
    // dedicated bus event the SaveToastStack renders as an
    // acknowledgeable banner.
    const untilISO = new Date(silences.value[source].until).toISOString()
    try {
      const notif = useNotificationsStore()
      notif.add({
        kind: 'warn',
        title: `${event.label || source} silenced`,
        body: `Detected ${count} notifications in a short window. Suppressed until ${formatLocal(silences.value[source].until)} (max 24 h). Click the bell entry to acknowledge.`,
      })
    } catch { /* notifications store optional in tests */ }

    bus.emit('watcher:silenced', {
      source,
      count,
      silencedUntil: untilISO,
      acknowledgeable: true,
    })
  }

  function _emitAlert(source, event, status) {
    const titleSuffix = STATUS_TITLES[status] || status
    const title = event.displayLabel || `${event.label || source} ${titleSuffix}`
    const body  = event.message || `${event.label || source}: ${status}`

    try {
      const notif = useNotificationsStore()
      notif.add({
        kind: status === 'warn' ? 'warn' : 'error',
        title,
        body,
      })
    } catch { /* optional */ }

    bus.emit('argus:save', {
      method: 'WatcherAlert:' + source,
      label: title,
      status: 'error',
      durationMs: 0,
      error: body,
    })
  }

  function _fireRecovery(source, event) {
    try {
      const notif = useNotificationsStore()
      notif.add({
        kind: 'info',
        title: `${event.label || source} recovered`,
        body: event.message || 'Previously failing source is healthy again.',
      })
    } catch { /* optional */ }
    bus.emit('argus:save', {
      method: 'WatcherAlert:' + source,
      label: `${event.label || source} recovered`,
      status: 'ok',
      durationMs: 0,
      error: '',
    })
  }

  return {
    settings, sources, silences,
    activeSilences, pendingAcks,
    setSettings, observe, silence, unsilence, acknowledge,
    snapshotForArgus,
  }
})

function formatLocal(epochMs) {
  try { return new Date(epochMs).toLocaleString() } catch { return String(epochMs) }
}
