// useWatcherEngine — the single, application-global tick that drives every
// registered watcher. Replaces per-feature polling loops (credentialMonitor,
// future cert-expiry-monitor, license-expiry-monitor, …) with one shared
// 30-second timer that scans the registry and runs whatever's due.
//
// Why one engine: it makes "the whole app probes external systems sanely"
// a property of the engine rather than something each feature has to
// re-implement (and re-misimplement) per loop. Specifically:
//
//   • respects per-watcher intervalMs and enabled flag from the registry
//   • routes every result through notificationGuard so dedupe + spam
//     protection apply uniformly
//   • pauses while document.hidden, fires a fresh probe pass on visibility
//     return — same behaviour the credential monitor used to do alone
//   • skips a tick if the previous one is still in flight
//
// API split:
//   useWatcherEngine() — call ONCE (App.vue) to start the timer.
//   runDueNow(), runWatcherById() — module-level helpers any component
//     can import to trigger an immediate run. They share the same
//     in-flight guard as the timer so a manual click never doubles up.

import { onMounted, onBeforeUnmount } from 'vue'
import { useWatcherRegistryStore } from '../stores/watcherRegistry'
import { useNotificationGuardStore } from '../stores/notificationGuard'

const TICK_MS = 30 * 1000
const FIRST_TICK_DELAY_MS = 5 * 1000
const PER_WATCHER_GAP_MS = 200

// Module-level state — there is only ever one engine, even if the
// composable is mounted twice by accident. The flag guards both the
// timer-driven runDueNow() and the manually-triggered ones.
let inFlight = false
let engineStopped = false

export async function runWatcherById(id) {
  const registry = useWatcherRegistryStore()
  const guard = useNotificationGuardStore()

  const desc = registry.effectiveDescriptor(id)
  if (!desc || !desc.enabled || typeof desc.check !== 'function') return null
  let result = null
  try {
    result = await desc.check()
  } catch (e) {
    result = { status: 'error', message: e?.message || String(e) }
  }
  if (!result || typeof result !== 'object') {
    result = { status: 'error', message: 'watcher returned no result' }
  }
  registry.recordResult(id, result)
  guard.observe({
    source: id,
    label: desc.label,
    status: result.status,
    message: result.message,
    anchor: desc.configureAnchor || '',
    recoveryStatuses: ['ok', 'valid', 'present', 'healthy'],
  })
  return result
}

export async function runDueNow({ force = false } = {}) {
  if (engineStopped) return
  if (inFlight) return
  inFlight = true
  try {
    const registry = useWatcherRegistryStore()
    const now = Date.now()
    for (const desc of registry.list) {
      if (engineStopped) break
      if (!desc.enabled) continue
      if (!force && registry.dueAt(desc.id) > now) continue
      await runWatcherById(desc.id)
      await new Promise((r) => setTimeout(r, PER_WATCHER_GAP_MS))
    }
  } finally {
    inFlight = false
  }
}

export function useWatcherEngine() {
  let firstTimer = null
  let interval = null

  function onVisibility() {
    if (engineStopped) return
    if (typeof document === 'undefined') return
    if (document.visibilityState === 'visible') {
      runDueNow().catch(() => {})
    }
  }

  onMounted(() => {
    engineStopped = false
    firstTimer = setTimeout(() => runDueNow().catch(() => {}), FIRST_TICK_DELAY_MS)
    interval = setInterval(() => runDueNow().catch(() => {}), TICK_MS)
    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', onVisibility)
    }
  })

  onBeforeUnmount(() => {
    engineStopped = true
    if (firstTimer) clearTimeout(firstTimer)
    if (interval) clearInterval(interval)
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', onVisibility)
    }
  })

  return { runDueNow, runWatcherById }
}
