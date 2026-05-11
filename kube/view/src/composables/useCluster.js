import { ref, onMounted } from 'vue'
import { callGo, cachedCallGo, invalidateCache, invalidateCachePrefix, DEFAULT_TTL } from './useBridge'

/**
 * Composable for App Mode (e.g., 'dashboard' or 'terminal')
 */
export function useAppMode() {
  const mode = ref('dashboard')
  async function fetchMode() {
    try {
      const res = await callGo('GetAppMode')
      if (res) mode.value = res
    } catch (e) {
      console.warn('[app-mode] failed to fetch mode, using dashboard', e)
    }
  }
  onMounted(fetchMode)
  return { mode, fetchMode }
}

/**
 * Composable for cluster info.
 */
export function useClusterInfo() {
  const info = ref(null)
  const loading = ref(true)
  const error = ref(null)

  async function fetch() {
    loading.value = info.value === null
    try {
      info.value = await cachedCallGo('GetClusterInfo', [], DEFAULT_TTL)
    } catch (e) {
      error.value = e
      info.value = null
    } finally {
      loading.value = false
    }
  }

  async function hardRefresh() {
    invalidateCache('GetClusterInfo')
    return fetch()
  }

  onMounted(fetch)
  return { info, loading, error, refresh: hardRefresh }
}

/**
 * Composable for kubeconfig context listing and switching.
 */
export function useContexts() {
  const contexts = ref([])
  const loading = ref(false)
  const switching = ref(false)
  const error = ref(null)

  async function listContexts() {
    loading.value = contexts.value.length === 0
    error.value = null
    try {
      const result = await cachedCallGo('ListContexts', [], DEFAULT_TTL)
      contexts.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
      contexts.value = []
    } finally {
      loading.value = false
    }
  }

  async function switchContext(name) {
    switching.value = true
    error.value = null
    try {
      await callGo('SwitchContext', name)
      invalidateCachePrefix('ListContexts')
      invalidateCachePrefix('GetClusterInfo')
      invalidateCachePrefix('ListResources')
      invalidateCachePrefix('GetMetrics')
      invalidateCachePrefix('GetAlerts')
      invalidateCachePrefix('EstimateCosts')
      invalidateCachePrefix('GetTopology')
      contexts.value = contexts.value.map(c => ({ ...c, active: c.name === name }))
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      switching.value = false
    }
  }

  return { contexts, loading, switching, error, listContexts, switchContext }
}

/**
 * Composable for topology graph.
 */
export function useTopology() {
  const topology = ref(null)
  const loading = ref(false)
  const error = ref(null)

  async function fetchTopology(namespace = '') {
    loading.value = topology.value === null
    error.value = null
    try {
      topology.value = await cachedCallGo('GetTopology', [namespace], DEFAULT_TTL)
    } catch (e) {
      console.warn('[topology] backend unavailable')
      topology.value = null
    } finally {
      loading.value = false
    }
  }

  return { topology, loading, error, fetchTopology }
}
