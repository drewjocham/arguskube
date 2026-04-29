import { ref, onMounted, onUnmounted } from 'vue'

/**
 * Bridge to Wails Go bindings.
 * In dev mode (no window.go), falls back to mock data.
 */
const isWails = () => typeof window !== 'undefined' && window.go

/**
 * Calls a Go method via Wails bindings.
 * @param {string} method - Method name on the App struct (e.g., 'GetAlerts')
 * @param  {...any} args - Arguments to pass
 */
async function callGo(method, ...args) {
  if (!isWails()) {
    console.warn(`[wails] No Go runtime — mock mode for: ${method}`)
    return null
  }
  try {
    // Wails bindings path: api/pkg → window.go.api.pkg
    return await window.go.api.pkg.App[method](...args)
  } catch (err) {
    console.error(`[wails] ${method} failed:`, err)
    throw err
  }
}

/**
 * Composable for cluster info.
 */
export function useClusterInfo() {
  const info = ref(null)
  const loading = ref(true)
  const error = ref(null)

  async function fetch() {
    loading.value = true
    try {
      info.value = await callGo('GetClusterInfo')
    } catch (e) {
      error.value = e
    } finally {
      loading.value = false
    }
  }

  onMounted(fetch)
  return { info, loading, error, refresh: fetch }
}

/**
 * Composable for cluster metrics with auto-refresh.
 */
export function useMetrics(intervalMs = 5000) {
  const metrics = ref(null)
  const loading = ref(true)
  let timer = null

  async function fetch() {
    try {
      metrics.value = await callGo('GetMetrics')
    } catch (e) {
      console.error('[metrics]', e)
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    fetch()
    timer = setInterval(fetch, intervalMs)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  return { metrics, loading, refresh: fetch }
}

/**
 * Composable for alerts with auto-refresh.
 */
export function useAlerts(intervalMs = 5000) {
  const alerts = ref([])
  const loading = ref(true)
  let timer = null

  async function fetch() {
    try {
      const result = await callGo('GetAlerts')
      if (result) alerts.value = result
    } catch (e) {
      console.error('[alerts]', e)
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    fetch()
    timer = setInterval(fetch, intervalMs)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  return { alerts, loading, refresh: fetch }
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

/**
 * Composable for feature gate checks.
 */
export function useFeatures() {
  const features = ref({})
  const tier = ref('free')

  async function fetch() {
    try {
      features.value = (await callGo('GetFeatures')) || {}
      tier.value = (await callGo('GetTier')) || 'free'
    } catch (e) {
      console.error('[features]', e)
    }
  }

  onMounted(fetch)

  function isAllowed(feature) {
    return features.value[feature] === true
  }

  return { features, tier, isAllowed, refresh: fetch }
}

/**
 * Composable for pod logs.
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
 * Composable for AI agent chat.
 */
export function useChat() {
  const history = ref([])
  const sending = ref(false)
  const autoSummary = ref(null)
  const eventLog = ref([])

  async function sendMessage(alertId, message) {
    sending.value = true
    try {
      const response = await callGo('SendChatMessage', alertId, message)
      // Refresh history after send.
      await refreshHistory(alertId)
      return response
    } catch (e) {
      console.error('[chat]', e)
      throw e
    } finally {
      sending.value = false
    }
  }

  async function refreshHistory(alertId) {
    try {
      const result = await callGo('GetChatHistory', alertId)
      if (result) history.value = result
    } catch (e) {
      console.error('[chat history]', e)
    }
  }

  async function fetchAutoSummary(alertId) {
    try {
      autoSummary.value = await callGo('GetAutoSummary', alertId)
    } catch (e) {
      console.error('[auto-summary]', e)
    }
  }

  async function fetchEventLog() {
    try {
      const result = await callGo('GetAgentEventLog')
      if (result) eventLog.value = result
    } catch (e) {
      console.error('[event-log]', e)
    }
  }

  return { history, sending, autoSummary, eventLog, sendMessage, refreshHistory, fetchAutoSummary, fetchEventLog }
}

/**
 * Composable for Popeye cluster scan.
 */
export function usePopeye() {
  const report = ref(null)
  const loading = ref(false)
  const error = ref(null)

  async function runScan() {
    loading.value = true
    error.value = null
    try {
      report.value = await callGo('RunPopeye')
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  return { report, loading, error, runScan }
}

/**
 * Composable for the resource browser.
 */
export function useResources() {
  const result = ref(null)
  const detail = ref(null)
  const namespaces = ref([])
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref(null)

  async function listResources(kind, namespace) {
    loading.value = true
    error.value = null
    try {
      result.value = await callGo('ListResources', kind, namespace || '')
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  async function getResourceDetail(kind, namespace, name) {
    detailLoading.value = true
    try {
      detail.value = await callGo('GetResourceDetail', kind, namespace || '', name)
    } catch (e) {
      console.error('[resource-detail]', e)
    } finally {
      detailLoading.value = false
    }
  }

  async function listNamespaces() {
    try {
      const result = await callGo('ListAllNamespaces')
      if (result) namespaces.value = result
    } catch (e) {
      console.error('[namespaces]', e)
    }
  }

  return { result, detail, namespaces, loading, detailLoading, error, listResources, getResourceDetail, listNamespaces }
}

/**
 * Composable for the embedded terminal.
 */
export function useTerminal() {
  async function startTerminal(rows, cols) {
    try {
      await callGo('StartTerminal', rows, cols)
    } catch (e) {
      console.error('[terminal]', e)
    }
  }

  async function sendInput(data) {
    try {
      await callGo('SendTerminalInput', data)
    } catch (e) {
      console.error('[terminal-input]', e)
    }
  }

  async function resizeTerminal(rows, cols) {
    try {
      await callGo('ResizeTerminal', rows, cols)
    } catch (e) {
      console.error('[terminal-resize]', e)
    }
  }

  return { startTerminal, sendInput, resizeTerminal }
}

/**
 * Composable for Anomaly Agent connectivity.
 */
export function useAnomaly() {
  const anomalies = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function connectAgent(namespace = 'all') {
    loading.value = true
    error.value = null
    try {
      anomalies.value = await callGo('ConnectToAgent', namespace)
    } catch (e) {
      error.value = e.message || 'Failed to connect to agent'
    } finally {
      loading.value = false
    }
  }

  return { anomalies, loading, error, connectAgent }
}
