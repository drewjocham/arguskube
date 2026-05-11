// credentialAlerts — fan a Vault probe outcome out to the user when a token
// transitions into a bad state (expired, invalid, error). One single Pinia
// store so the manual "Test" button in Settings and the background monitor
// loop both go through the same dedupe + delivery path.
//
// Two delivery channels:
//
//   1. The persistent bell panel via useNotificationsStore.add() — these
//      are findable later, even after the toast self-disappears.
//   2. The transparent top-right SaveToastStack via the
//      `argus:save` bus event — it already renders error-styled toasts.
//
// Dedupe is per-provider + per-status: once we've fired an alert for
// "github:expired", a follow-up probe that returns the same status is
// silent. Recovery (e.g. github goes from `expired` → `valid` after the
// user pastes a new token) clears the dedupe key so a future regression
// re-fires. Cooldown caps re-fires per status to once per RE_FIRE_MIN.

import { defineStore } from 'pinia'
import { ref } from 'vue'
import { bus } from '../lib/bus'
import { useNotificationsStore } from './notifications'

const ALERT_STATUSES = new Set(['expired', 'invalid', 'error'])
const RE_FIRE_MIN = 60 * 60 * 1000 // 1 h — don't badger about the same problem more than hourly

const STATUS_TITLE = {
  expired: 'token expired',
  invalid: 'token rejected',
  error:   'check failed',
}

export const useCredentialAlertsStore = defineStore('credentialAlerts', () => {
  // Per-provider state for dedupe + recovery detection.
  //   { lastStatus: 'valid' | 'expired' | …, firedAt: epoch_ms_or_0 }
  const seen = ref({})

  function fireToast(detail) {
    try {
      bus.emit('argus:save', detail)
    } catch {
      // silently no-op in test environments
    }
  }

  // observe takes a VaultEntry-shaped object: { id, label, status, message }.
  // Call this after every manual or background probe — the store decides
  // whether to actually fire.
  function observe(entry) {
    if (!entry || !entry.id) return false
    const id = entry.id
    const status = entry.status || ''
    const previous = seen.value[id] || { lastStatus: '', firedAt: 0 }

    // Recovery: status flipped back to a healthy state. Clear the dedupe
    // marker so a future regression isn't silently swallowed, and emit a
    // recovery toast so the user knows the previous alert is resolved.
    if (status === 'valid' && ALERT_STATUSES.has(previous.lastStatus)) {
      seen.value = { ...seen.value, [id]: { lastStatus: status, firedAt: 0 } }
      try {
        const notif = useNotificationsStore()
        notif.add({
          kind: 'info',
          title: `${entry.label || id} — credential restored`,
          body: 'A previously-failing credential is now valid.',
        })
      } catch { /* notifications optional */ }
      fireToast({
        method: 'CredentialAlert:' + id,
        label:  `${entry.label || id} credential restored`,
        status: 'ok',
        durationMs: 0,
        error: '',
      })
      return false
    }

    if (!ALERT_STATUSES.has(status)) {
      // Healthy / present / missing — no alert today.
      seen.value = { ...seen.value, [id]: { lastStatus: status, firedAt: previous.firedAt } }
      return false
    }

    // Bad status. Skip if we just fired for the same status recently.
    const now = Date.now()
    const sameStatus = previous.lastStatus === status
    if (sameStatus && previous.firedAt && now - previous.firedAt < RE_FIRE_MIN) {
      return false
    }

    const titleSuffix = STATUS_TITLE[status] || status
    const title = `${entry.label || id} ${titleSuffix}`
    const body = entry.message || `${entry.label || id}: ${status}`

    try {
      const notif = useNotificationsStore()
      notif.add({
        kind: status === 'error' ? 'warn' : 'error',
        title,
        body,
      })
    } catch { /* notifications optional */ }

    // Re-using the SaveToastStack channel. We hand it our own pre-built
    // label so the toast reads "GitHub token expired" instead of the
    // method-name derivation it'd otherwise apply.
    fireToast({
      method: 'CredentialAlert:' + id,
      label:  title,
      status: 'error',
      durationMs: 0,
      error: body,
    })

    seen.value = { ...seen.value, [id]: { lastStatus: status, firedAt: now } }
    return true
  }

  // Reset for tests / sign-out. Cooldown resets too.
  function reset() {
    seen.value = {}
  }

  return { seen, observe, reset }
})
