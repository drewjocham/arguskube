import { ref } from 'vue'
import { callGo, cachedCallGo, DEFAULT_TTL } from './useBridge'

/**
 * Composable for pod logs (one-shot fetch).
 */
export function usePodLogs() {
  const logs = ref([])
  const loading = ref(false)

  async function fetch(namespace, podName, tailLines = 50) {
    loading.value = true
    try {
      const result = await callGo('GetPodLogs', namespace, podName, tailLines)
      if (result) logs.value = result
    } catch (e) {
      console.error('[logs]', e)
    } finally {
      loading.value = false
    }
  }

  return { logs, loading, fetch }
}

/**
 * Composable for live pod log streaming with follow mode.
 * Uses StreamPodLogsFollow binding for real-time tailing.
 */
export function useLogStream() {
  const lines = ref([])
  const streaming = ref(false)
  const error = ref(null)

  async function startStream(namespace, podName, container = '', tailLines = 100) {
    streaming.value = true
    error.value = null
    lines.value = []
    try {
      const result = await callGo('StreamPodLogsFollow', namespace, podName, container, tailLines)
      if (result && Array.isArray(result)) {
        lines.value = result
      } else if (typeof result === 'string') {
        lines.value = result.split('\n').filter(Boolean).map(line => ({ message: line }))
      }
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      streaming.value = false
    }
  }

  function clear() {
    lines.value = []
    error.value = null
  }

  return { lines, streaming, error, startStream, clear }
}

/**
 * Composable for log querying (log explorer/filter).
 */
export function useLogs() {
  const entries = ref([])
  const histogram = ref([])
  const fields = ref([])
  const total = ref(0)
  const loading = ref(false)
  const queryTime = ref(0)
  const error = ref(null)

  async function queryLogs(query = '*', namespace = '', limit = 100) {
    loading.value = true
    error.value = null
    const start = performance.now()
    try {
      const result = await callGo('QueryLogs', query, namespace, limit)
      queryTime.value = Math.round(performance.now() - start)
      if (result) {
        entries.value = result.entries || []
        histogram.value = result.histogram || []
        fields.value = result.fields || []
        total.value = result.total || 0
      }
    } catch (e) {
      error.value = e?.message || String(e)
      queryTime.value = Math.round(performance.now() - start)
    } finally {
      loading.value = false
    }
  }

  return { entries, histogram, fields, total, loading, queryTime, error, queryLogs }
}

/**
 * Composable for node-level system service logs (kubelet, containerd, kube-proxy).
 */
export function useNodeLogs() {
  const logs = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function fetchNodeLogs(nodeName, tailLines = 100) {
    loading.value = logs.value.length === 0
    error.value = null
    try {
      const result = await cachedCallGo('GetNodeLogs', [nodeName, tailLines], DEFAULT_TTL)
      logs.value = result && Array.isArray(result) ? result : []
    } catch (e) {
      console.warn('[node-logs] backend unavailable')
      logs.value = []
    } finally {
      loading.value = false
    }
  }

  function clear() {
    logs.value = []
    error.value = null
  }

  return { logs, loading, error, fetchNodeLogs, clear }
}
