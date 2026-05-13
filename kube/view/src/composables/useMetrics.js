import { ref, onMounted, onUnmounted } from 'vue'
import { callGo, cachedCallGo, invalidateCache, FAST_TTL, DEFAULT_TTL } from './useBridge'

/**
 * Composable for cluster metrics with auto-refresh.
 */
export function useMetrics(intervalMs = 5000) {
  const metrics = ref(null)
  const loading = ref(true)
  let timer = null

  async function fetch() {
    try {
      const result = await cachedCallGo('GetMetrics', [], FAST_TTL)
      if (result) metrics.value = result
    } catch (e) {
      console.warn('[metrics] backend unavailable')
      if (!metrics.value) metrics.value = null
    } finally {
      loading.value = false
    }
  }

  async function hardFetch() {
    invalidateCache('GetMetrics')
    return fetch()
  }

  onMounted(() => {
    fetch()
    timer = setInterval(hardFetch, intervalMs)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  async function queryMetrics(query, timeRange) {
    try {
      return await callGo('QueryTimeSeriesMetrics', query, timeRange)
    } catch (e) {
      console.error('[metrics] queryTimeSeries:', e)
      return null
    }
  }

  return { metrics, loading, refresh: hardFetch, queryMetrics }
}

/**
 * Composable for time-series metric queries (no polling, on-demand only).
 */
export function useTimeSeriesMetrics() {
  // namespace is accepted for API symmetry with the rest of the
  // metrics surface but is NOT forwarded — the Go binding
  // QueryTimeSeriesMetrics(query, timeRange) takes 2 args and Wails
  // rejects the call (with a silent log + null result) if we send 3.
  // The previous "drop-in upgrade" comment promised a backend change
  // that never landed; until it does, we ignore namespace here so the
  // Cluster Overview's CPU/RAM/Network charts actually render data.
  async function queryMetrics(query, timeRange /*, namespace */) {
    try {
      return await callGo('QueryTimeSeriesMetrics', query, timeRange)
    } catch (e) {
      console.error('[metrics] queryTimeSeries:', e)
      return null
    }
  }
  return { queryMetrics }
}

/**
 * Composable for FinOps cost estimation based on pod resource requests.
 */
export function useCostEstimate() {
  const report = ref(null)
  const loading = ref(false)
  const error = ref(null)
  const provider = ref('aws')

  async function fetchCosts(providerOverride) {
    const p = providerOverride || provider.value
    loading.value = report.value === null
    error.value = null
    try {
      const result = await cachedCallGo('EstimateCosts', [p], DEFAULT_TTL)
      report.value = result || null
    } catch (e) {
      console.warn('[finops] backend unavailable')
      report.value = null
    } finally {
      loading.value = false
    }
  }

  return { report, loading, error, provider, fetchCosts }
}
