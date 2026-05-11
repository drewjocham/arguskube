// watcherRegistry — every "thing that can expire" lives here.
//
// Anything from credentials to certs to license keys to OAuth refresh
// tokens registers itself with a small descriptor and an async check()
// function. The watcher engine (composables/useWatcherEngine.js) drives
// them all on a single shared schedule and routes results through the
// notificationGuard.
//
// Why a registry instead of one-off per feature: a single registry means
//   • one place to enable/disable watchers
//   • one place to override schedules ("re-check GitHub every 5 min")
//   • one place for Argus chat to read state and silence/unsilence
//   • automatic dedupe + spam protection inherited for free
//
// Watcher descriptor:
//   {
//     id: string,                 // unique key
//     label: string,              // human-readable
//     kind: string,               // 'credential' | 'cert' | 'license' | 'volume' | …
//     intervalMs: number,         // poll cadence (clamped to [60s, 24h])
//     check: async () => {        // called by the engine on each tick
//       status: string,           // 'ok' | 'warn' | 'expired' | 'invalid' | 'error'
//       message?: string,
//       expiresAt?: string,       // RFC3339
//     },
//     configureAnchor?: string,   // settings deep-link
//     enabled?: boolean,          // user toggle (defaults true)
//   }
//
// Per-watcher overrides (interval, enabled flag) persist to localStorage
// keyed by id so the user's preferences survive reloads even though the
// watcher functions themselves re-register on every page load.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const STORAGE_KEY = 'kw-watcher-registry/v1'
const MIN_INTERVAL_MS = 60 * 1000
const MAX_INTERVAL_MS = 24 * 60 * 60 * 1000

function loadOverrides() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch { return {} }
}

function persistOverrides(map) {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(map)) } catch { /* ignore */ }
}

function clampInterval(ms) {
  if (!Number.isFinite(ms)) return MIN_INTERVAL_MS
  return Math.max(MIN_INTERVAL_MS, Math.min(MAX_INTERVAL_MS, Math.floor(ms)))
}

export const useWatcherRegistryStore = defineStore('watcherRegistry', () => {
  // Descriptors (functions live here too; not serialized).
  const watchers = ref({})
  // Persisted user overrides per watcher id.
  const overrides = ref(loadOverrides())
  // Last-known result per watcher id, in-memory only.
  const results = ref({})
  // Last successful check timestamp per id.
  const lastCheckedAt = ref({})

  function effectiveDescriptor(id) {
    const base = watchers.value[id]
    if (!base) return null
    const ov = overrides.value[id] || {}
    return {
      ...base,
      intervalMs: clampInterval(ov.intervalMs ?? base.intervalMs),
      enabled: ov.enabled !== undefined ? !!ov.enabled : (base.enabled !== false),
    }
  }

  // Returns a snapshot list — what UI/Argus iterate over.
  const list = computed(() =>
    Object.keys(watchers.value)
      .map(effectiveDescriptor)
      .filter(Boolean)
      .sort((a, b) => a.label.localeCompare(b.label))
  )

  function register(descriptor) {
    if (!descriptor || !descriptor.id || typeof descriptor.check !== 'function') {
      throw new Error('watcherRegistry: register() requires { id, check() }')
    }
    watchers.value = { ...watchers.value, [descriptor.id]: { ...descriptor } }
  }

  function unregister(id) {
    if (!watchers.value[id]) return
    const next = { ...watchers.value }
    delete next[id]
    watchers.value = next
  }

  function setOverride(id, patch) {
    if (!watchers.value[id]) return
    overrides.value = {
      ...overrides.value,
      [id]: { ...(overrides.value[id] || {}), ...patch },
    }
    persistOverrides(overrides.value)
  }

  function setEnabled(id, enabled) { setOverride(id, { enabled: !!enabled }) }
  function setInterval(id, ms)     { setOverride(id, { intervalMs: clampInterval(ms) }) }

  function recordResult(id, result) {
    results.value = { ...results.value, [id]: result }
    lastCheckedAt.value = { ...lastCheckedAt.value, [id]: Date.now() }
  }

  function dueAt(id) {
    const last = lastCheckedAt.value[id] || 0
    const desc = effectiveDescriptor(id)
    if (!desc) return Number.POSITIVE_INFINITY
    return last + desc.intervalMs
  }

  function snapshotForArgus() {
    return list.value.map((w) => ({
      id: w.id,
      label: w.label,
      kind: w.kind,
      intervalMs: w.intervalMs,
      enabled: w.enabled,
      configureAnchor: w.configureAnchor || '',
      lastResult: results.value[w.id] || null,
      lastCheckedAt: lastCheckedAt.value[w.id]
        ? new Date(lastCheckedAt.value[w.id]).toISOString()
        : null,
    }))
  }

  return {
    watchers, overrides, results, lastCheckedAt,
    list,
    register, unregister, setOverride, setEnabled, setInterval,
    recordResult, dueAt, effectiveDescriptor, snapshotForArgus,
  }
})
