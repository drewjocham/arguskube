// useArgusAlertContext — exposes the live watcher + silence state to the
// Argus AI chat through a stable global handle. Mounted once in App.vue;
// the chat (or any tool that wants to know "what's currently alerting"
// or "silence GitHub for an hour") reads/writes via window.argusAlertContext.
//
// Why a window-level handle instead of importing the stores: the chat
// runs through a string-typed tool-call interface (and may eventually
// run inside an iframe / web worker), so a small JSON-shaped surface is
// easier to expose than a Vue/Pinia binding. The shape is stable and
// versioned — adding a field is fine; removing one breaks the chat tool.
//
// Surface:
//   window.argusAlertContext.read()
//     → { generatedAt, watchers: [...], silences: [...], guardSettings }
//   window.argusAlertContext.silence(source, durationMs?)
//   window.argusAlertContext.unsilence(source)
//   window.argusAlertContext.runWatcher(id)
//   window.argusAlertContext.runDueNow()
//
// Every action returns a small JSON ack so the chat can render a
// confirmation back to the user.

import { onMounted, onBeforeUnmount } from 'vue'
import { useWatcherRegistryStore } from '../stores/watcherRegistry'
import { useNotificationGuardStore } from '../stores/notificationGuard'
import { runDueNow, runWatcherById } from './useWatcherEngine'

const HANDLE_KEY = 'argusAlertContext'
const SCHEMA_VERSION = 1
const MAX_SILENCE_MS = 24 * 60 * 60 * 1000

export function useArgusAlertContext() {
  const registry = useWatcherRegistryStore()
  const guard = useNotificationGuardStore()

  function read() {
    return {
      schemaVersion: SCHEMA_VERSION,
      generatedAt:   new Date().toISOString(),
      watchers:      registry.snapshotForArgus(),
      ...guard.snapshotForArgus(),
      // ↑ snapshotForArgus already includes generatedAt + silences +
      // sources + settings; spreading at the end so its generatedAt wins.
    }
  }

  function silence(source, durationMs) {
    if (!source) return { ok: false, error: 'source is required' }
    const dur = Math.max(60_000, Math.min(MAX_SILENCE_MS,
      Number.isFinite(durationMs) ? durationMs : guard.settings.defaultSilenceMs))
    const desc = registry.effectiveDescriptor(source)
    guard.silence(source, dur, {
      label: desc?.label || source,
      anchor: desc?.configureAnchor || 'watchers-notifications',
      reason: 'argus',
    })
    return { ok: true, source, silencedFor: dur, until: new Date(Date.now() + dur).toISOString() }
  }

  function unsilence(source) {
    if (!source) return { ok: false, error: 'source is required' }
    if (!guard.silences[source]) return { ok: false, error: 'no active silence for that source' }
    guard.unsilence(source)
    return { ok: true, source }
  }

  async function runOne(id) {
    if (!id) return { ok: false, error: 'id is required' }
    const result = await runWatcherById(id)
    return { ok: !!result, id, result }
  }

  async function runAll() {
    await runDueNow({ force: true })
    return { ok: true }
  }

  function attach() {
    if (typeof window === 'undefined') return
    window[HANDLE_KEY] = {
      schemaVersion: SCHEMA_VERSION,
      read,
      silence,
      unsilence,
      runWatcher: runOne,
      runDueNow: runAll,
    }
  }
  function detach() {
    if (typeof window === 'undefined') return
    if (window[HANDLE_KEY] && window[HANDLE_KEY].schemaVersion === SCHEMA_VERSION) {
      try { delete window[HANDLE_KEY] } catch { window[HANDLE_KEY] = null }
    }
  }

  onMounted(attach)
  onBeforeUnmount(detach)

  return { read, silence, unsilence, runOne, runAll }
}
