import { ref, onMounted, onUnmounted } from 'vue'
import { callGo, cachedCallGo, DEFAULT_TTL } from './useBridge'

/**
 * Composable for pod listing.
 */
export function usePods(intervalMs = 10000) {
  const pods = ref([])
  const loading = ref(true)
  const error = ref(null)
  const currentNamespace = ref('_all')
  let timer = null

  async function listPods(namespace = '_all') {
    currentNamespace.value = namespace
    loading.value = pods.value.length === 0
    error.value = null
    try {
      const result = await cachedCallGo('ListResources', ['pods', namespace], DEFAULT_TTL)
      pods.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
      pods.value = []
    } finally {
      loading.value = false
    }
  }

  async function getPodLogs(namespace, podName, tailLines = 50) {
    try {
      return await callGo('GetPodLogs', namespace, podName, tailLines)
    } catch (e) {
      console.error('[pods] getPodLogs:', e)
      return null
    }
  }

  onMounted(() => {
    // timer = setInterval(() => listPods(currentNamespace.value), intervalMs)
  })
  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  return { pods, loading, error, listPods, getPodLogs }
}

/**
 * Composable for deployment revision history (rollout timeline).
 */
export function useDeploymentRevisions() {
  const revisions = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function fetchRevisions(namespace, deploymentName, limit = 25) {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('GetDeploymentRevisions', namespace, deploymentName, limit)
      revisions.value = result && Array.isArray(result) ? result : []
    } catch (e) {
      error.value = e?.message || String(e)
      revisions.value = []
    } finally {
      loading.value = false
    }
  }

  return { revisions, loading, error, fetchRevisions }
}

/**
 * Composable for VPA (Vertical Pod Autoscaler) recommendations.
 */
export function useVPARecommendations() {
  const vpas = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function fetchVPAs(namespace = '') {
    loading.value = vpas.value.length === 0
    error.value = null
    try {
      const result = await cachedCallGo('GetVPARecommendations', [namespace], DEFAULT_TTL)
      vpas.value = result && Array.isArray(result) ? result : []
    } catch (e) {
      error.value = e?.message || String(e)
      vpas.value = []
    } finally {
      loading.value = false
    }
  }

  return { vpas, loading, error, fetchVPAs }
}
