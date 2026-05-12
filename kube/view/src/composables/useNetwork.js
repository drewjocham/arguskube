import { ref } from 'vue'
import { callGo, cachedCallGo, DEFAULT_TTL } from './useBridge'

/**
 * Composable for listing pods backing a Service.
 * Looks up endpointslices or endpoints to find matching pods.
 */
export function useServicePods() {
  const pods = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function fetchServicePods(namespace, serviceName) {
    loading.value = true
    error.value = null
    try {
      const result = await cachedCallGo('GetServicePods', [namespace, serviceName], DEFAULT_TTL)
      pods.value = result && Array.isArray(result) ? result : []
    } catch (e) {
      error.value = e?.message || String(e)
      pods.value = []
    } finally {
      loading.value = false
    }
  }

  return { pods, loading, error, fetchServicePods }
}
