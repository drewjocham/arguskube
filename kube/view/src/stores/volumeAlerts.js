// volumeAlerts — per-volume usage thresholds, persisted to localStorage.
//
// An alert config is keyed by `<scope>:<namespace>:<name>` so a PVC and a
// PV with the same name in different scopes are distinct entries. Threshold
// is either a percentage (0-100) or an absolute byte count below which the
// alert fires. The alert state ("yellow") is computed at the call site by
// joining this with notificationChannels.hasAny().

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const STORAGE_KEY = 'kw-volume-alerts/v1'

function loadInitial() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function persist(map) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(map))
  } catch {
    // localStorage may be disabled in private mode; the in-memory store
    // still works for the rest of the session.
  }
}

export function volumeKey(scope, namespace, name) {
  return [scope || 'pv', namespace || '', name || ''].join(':')
}

export const useVolumeAlertsStore = defineStore('volumeAlerts', () => {
  const alerts = ref(loadInitial())

  const all = computed(() => Object.values(alerts.value))

  function get(scope, namespace, name) {
    return alerts.value[volumeKey(scope, namespace, name)] || null
  }

  // Set or update an alert. mode = 'pct' | 'abs'. value is interpreted
  // accordingly: 0-100 for pct; bytes (positive integer) for abs.
  function set(cfg) {
    const k = volumeKey(cfg.scope, cfg.namespace, cfg.name)
    const next = {
      key: k,
      scope: cfg.scope,
      namespace: cfg.namespace || '',
      name: cfg.name,
      capacity: cfg.capacity || '',
      mode: cfg.mode === 'abs' ? 'abs' : 'pct',
      value: Math.max(0, Number(cfg.value) || 0),
      enabled: cfg.enabled !== false,
      updatedAt: Date.now(),
    }
    alerts.value = { ...alerts.value, [k]: next }
    persist(alerts.value)
    return next
  }

  function remove(scope, namespace, name) {
    const k = volumeKey(scope, namespace, name)
    if (!(k in alerts.value)) return
    const copy = { ...alerts.value }
    delete copy[k]
    alerts.value = copy
    persist(alerts.value)
  }

  return { alerts, all, get, set, remove }
})
