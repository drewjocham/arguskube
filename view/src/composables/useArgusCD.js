import { ref } from 'vue'
import { callGo, cachedCallGo, invalidateCache, invalidateCachePrefix, DEFAULT_TTL } from './useBridge'

/**
 * Composable for ArgusCD — real Argo CD integration with k8s deployment fallback.
 */
export function useArgusCD() {
  const apps = ref([])
  const selectedApp = ref(null)
  const resources = ref([])
  const diffs = ref([])
  const status = ref(null)
  const loading = ref(false)
  const error = ref(null)

  async function fetchStatus() {
    try {
      status.value = await cachedCallGo('GetArgusCDStatus', [], DEFAULT_TTL)
    } catch (e) {
      status.value = { connected: false, message: e?.message || 'Failed to check status' }
    }
  }

  async function listApps(project = '') {
    loading.value = apps.value.length === 0
    error.value = null
    try {
      const result = await cachedCallGo('ListArgusCDApps', [project], DEFAULT_TTL)
      apps.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
      apps.value = []
    } finally {
      loading.value = false
    }
  }

  async function getApp(name) {
    try {
      selectedApp.value = await cachedCallGo('GetArgusCDApp', [name], DEFAULT_TTL)
    } catch (e) {
      console.warn('[arguscd] getApp — using list data')
      selectedApp.value = apps.value.find(a => a.name === name) || null
    }
  }

  async function getResources(name) {
    try {
      resources.value = await cachedCallGo('GetArgusCDResources', [name], DEFAULT_TTL)
    } catch (e) {
      resources.value = []
    }
  }

  async function getDiffs(name) {
    try {
      diffs.value = await cachedCallGo('GetArgusCDDiffs', [name], DEFAULT_TTL)
    } catch (e) {
      diffs.value = []
    }
  }

  async function syncApp(name) {
    try {
      const result = await callGo('SyncArgusCDApp', name)
      invalidateCachePrefix('ListArgusCDApps')
      invalidateCachePrefix('GetArgusCDApp')
      await listApps()
      return result
    } catch (e) {
      console.error('[arguscd] sync:', e)
      throw e
    }
  }

  async function refreshApp(name, hard = false) {
    try {
      await callGo('RefreshArgusCDApp', name, hard)
      invalidateCachePrefix('ListArgusCDApps')
      invalidateCachePrefix('GetArgusCDApp')
      await listApps()
    } catch (e) {
      console.error('[arguscd] refresh:', e)
    }
  }

  async function rollbackApp(name, revisionID) {
    try {
      await callGo('RollbackArgusCDApp', name, revisionID)
      invalidateCachePrefix('ListArgusCDApps')
      invalidateCachePrefix('GetArgusCDApp')
      await listApps()
    } catch (e) {
      console.error('[arguscd] rollback:', e)
      throw e
    }
  }

  async function testConnection() {
    try {
      await callGo('TestArgusCDConnection')
      return { success: true }
    } catch (e) {
      return { success: false, error: e?.message || String(e) }
    }
  }

  return {
    apps, selectedApp, resources, diffs, status, loading, error,
    fetchStatus, listApps, getApp, getResources, getDiffs,
    syncApp, refreshApp, rollbackApp, testConnection,
  }
}

// Legacy alias for backward compat.
export function useApplications() {
  const { apps: applications, loading, error, listApps, syncApp } = useArgusCD()
  return {
    applications, loading, error,
    listApplications: listApps,
    syncApplication: (ns, name) => syncApp(name),
  }
}
