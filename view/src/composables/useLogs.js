import { ref } from 'vue'
import { callGo, cachedCallGo, DEFAULT_TTL } from './useBridge'

/**
 * Composable for pod logs (one-shot fetch).
 *
 * Same monotonic-token guard as useLogStream so a slow fetch for the
 * previous pod doesn't overwrite the freshly-displayed logs of the
 * pod the user just switched to. Without this, switching pods rapidly
 * could surface logs from the wrong pod.
 */
export function usePodLogs() {
  const logs = ref([])
  const loading = ref(false)
  let activeToken = 0

  async function fetch(namespace, podName, tailLines = 50) {
    const myToken = ++activeToken
    loading.value = true
    try {
      const result = await callGo('GetPodLogs', namespace, podName, tailLines)
      if (myToken !== activeToken) return
      logs.value = Array.isArray(result) ? result : []
    } catch (e) {
      if (myToken !== activeToken) return
      console.error('[logs]', e)
    } finally {
      if (myToken === activeToken) loading.value = false
    }
  }

  return { logs, loading, fetch }
}

/**
 * Composable for live pod log streaming with follow mode.
 *
 * Two bugs the previous version had, both fixed here:
 *
 *   - "logs doubled": the backend StreamPodLogsFollow returns []string,
 *     usePodLogs returns []LogLine (objects with .message). The two
 *     shapes were ending up in the same component state when the user
 *     toggled follow mode, so the template rendered twice (once for
 *     each shape). Both paths are normalised to LogLine-shaped objects
 *     here so the consumer never has to branch.
 *
 *   - "logs disappear during streaming": stale fetches finishing AFTER
 *     a fresh one would clobber the UI with empty-or-old data. We tag
 *     each call with a monotonic token and drop late results.
 */
export function useLogStream() {
  const lines = ref([])
  const streaming = ref(false)
  const error = ref(null)

  // Monotonic ID — only the most recent startStream() owns the
  // visible lines.value. Stale resolutions are dropped.
  let activeToken = 0

  function asLogLine(raw, podName) {
    if (!raw) return null
    if (typeof raw === 'string') {
      return {
        timestamp: new Date().toISOString(),
        source: podName ? `[${podName}]` : '',
        level: inferLevel(raw),
        message: raw,
      }
    }
    if (typeof raw === 'object') {
      return {
        timestamp: raw.timestamp || raw.Timestamp || new Date().toISOString(),
        source: raw.source || raw.Source || '',
        level: raw.level || raw.Level || inferLevel(raw.message || raw.Message || ''),
        message: raw.message || raw.Message || '',
      }
    }
    return null
  }

  function inferLevel(s) {
    const lower = String(s || '').toLowerCase()
    if (lower.includes('fatal') || lower.includes('panic')) return 'fatal'
    if (lower.includes('error') || /\berr\b/.test(lower)) return 'error'
    if (lower.includes('warn')) return 'warning'
    if (lower.includes('debug') || lower.includes('trace')) return 'debug'
    return 'info'
  }

  async function startStream(namespace, podName, container = '', tailLines = 100) {
    const myToken = ++activeToken
    streaming.value = true
    error.value = null
    lines.value = []
    try {
      const result = await callGo('StreamPodLogsFollow', namespace, podName, container, tailLines)
      // Drop the result if a fresher call has started since.
      if (myToken !== activeToken) return

      let normalized = []
      if (Array.isArray(result)) {
        normalized = result.map(r => asLogLine(r, podName)).filter(Boolean)
      } else if (typeof result === 'string') {
        normalized = result
          .split('\n')
          .filter(Boolean)
          .map(line => asLogLine(line, podName))
          .filter(Boolean)
      }
      lines.value = normalized
    } catch (e) {
      if (myToken !== activeToken) return
      error.value = e?.message || String(e)
    } finally {
      if (myToken === activeToken) streaming.value = false
    }
  }

  function clear() {
    activeToken++ // invalidate any in-flight calls
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
