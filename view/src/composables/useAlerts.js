import { ref, onMounted, onUnmounted } from 'vue'
import { callGo, cachedCallGo, invalidateCache, FAST_TTL, DEFAULT_TTL } from './useBridge'

/**
 * Composable for alerts with auto-refresh.
 */
export function useAlerts(intervalMs = 5000) {
  const alerts = ref([])
  const loading = ref(true)
  let timer = null

  async function fetch() {
    try {
      const result = await cachedCallGo('GetAlerts', [], FAST_TTL)
      if (result) alerts.value = result
    } catch (e) {
      console.warn('[alerts] backend unavailable')
      if (alerts.value.length === 0) alerts.value = []
    } finally {
      loading.value = false
    }
  }

  async function hardFetch() {
    invalidateCache('GetAlerts')
    return fetch()
  }

  onMounted(() => {
    fetch()
    timer = setInterval(hardFetch, intervalMs)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  return { alerts, loading, refresh: hardFetch }
}

/**
 * Composable for AI diagnostics.
 */
export function useDiagnostics() {
  const bundle = ref(null)
  const loading = ref(false)
  const error = ref(null)

  async function diagnose(alertId) {
    loading.value = true
    error.value = null
    try {
      bundle.value = await callGo('DiagnoseAlert', alertId)
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  return { bundle, loading, error, diagnose }
}
