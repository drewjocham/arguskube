
import { ref, onMounted, onUnmounted } from 'vue'
import * as backend from '../wailsjs/go/pkg/App'

/**
 * Bridge to Wails Go bindings.
 * In Wails mode uses direct Go bindings; otherwise falls back to REST API on :8080.
 */
export const isWails = () => typeof window !== 'undefined' && !!window.go

// ---------------------------------------------------------------------------
// Global response cache — prevents redundant backend calls when switching
// between views. Each entry is keyed by "method:arg1:arg2:..." and stores
// { data, ts }. Composables call cachedCallGo() instead of callGo() for
// read-only fetches.
// ---------------------------------------------------------------------------
const _cache = new Map()
const DEFAULT_TTL = 30_000  // 30 seconds for most resources
const FAST_TTL    = 5_000   // 5 seconds for metrics/alerts

function _cacheKey(method, args) {
  return args.length ? `${method}:${args.join(':')}` : method
}

/**
 * Cached version of callGo for read-only fetches.
 * Returns cached data instantly if within TTL; fetches fresh otherwise.
 * @param {number} ttl - cache TTL in ms
 */
export async function cachedCallGo(method, args = [], ttl = DEFAULT_TTL) {
  const key = _cacheKey(method, args)
  const cached = _cache.get(key)
  if (cached && (Date.now() - cached.ts) < ttl) {
    return cached.data
  }
  const result = await callGo(method, ...args)
  _cache.set(key, { data: result, ts: Date.now() })
  return result
}

/** Invalidate a specific cache entry (e.g. after a write/mutation). */
export function invalidateCache(method, ...args) {
  const key = _cacheKey(method, args)
  _cache.delete(key)
}

/** Invalidate all entries whose key starts with a prefix. */
export function invalidateCachePrefix(prefix) {
  for (const key of _cache.keys()) {
    if (key.startsWith(prefix)) _cache.delete(key)
  }
}

export async function callGo(method, ...args) {
  if (isWails()) {
    try {
      return await backend[method](...args)
    } catch (err) {
      console.error(`[wails] ${method} failed:`, err)
      throw err
    }
  }

  // Fallback to REST API for SaaS mode
  try {
    const apiBase = window.__KUBEWATCHER_API_BASE__ || 'http://localhost:8080'
    const res = await fetch(`${apiBase}/api/${method}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ args })
    })
    
    if (!res.ok) {
      throw new Error(`HTTP error! status: ${res.status}`)
    }
    
    const data = await res.json()
    if (data.error) {
      throw new Error(data.error)
    }
    return data.result
  } catch (err) {
    console.warn(`[saas-api] ${method} fallback failed (is the backend running on :8080?):`, err)
    // Components handle empty state on failure
    throw err
  }
}

/**
 * Composable for App Mode (e.g., 'dashboard' or 'terminal')
 */
export function useAppMode() {
  const mode = ref('dashboard')
  
  async function fetch() {
    try {
      const res = await callGo('GetAppMode')
      if (res) mode.value = res
    } catch (e) {
      console.warn('[app-mode] failed to fetch mode, using dashboard', e)
    }
  }

  onMounted(fetch)
  return { mode }
}

/**
 * Composable for cluster info.
 */
export function useClusterInfo() {
  const info = ref(null)
  const loading = ref(true)
  const error = ref(null)

  async function fetch() {
    loading.value = info.value === null // only show spinner on cold start
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
      // Invalidate caches that depend on the active context.
      invalidateCachePrefix('ListContexts')
      invalidateCachePrefix('GetClusterInfo')
      invalidateCachePrefix('ListResources')
      invalidateCachePrefix('GetMetrics')
      invalidateCachePrefix('GetAlerts')
      invalidateCachePrefix('EstimateCosts')
      invalidateCachePrefix('GetTopology')
      // Mark the new active context locally.
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

  return { metrics, loading, refresh: fetch, queryMetrics }
}

/**
 * Composable for time-series metric queries (no polling, on-demand only).
 */
export function useTimeSeriesMetrics() {
  async function queryMetrics(query, timeRange) {
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

/**
 * Composable for feature gate checks.
 */
export function useFeatures() {
  // Default: enable ALL features so the desktop app is fully functional
  // even before the backend responds or if the call fails.
  const defaultFeatures = {
    alerts: true,
    cluster_view: true,
    log_stream: true,
    topology: true,
    cascade_correlation: true,
    anomstack_anomaly: true,
    ai_diagnostics: true,
    runbook_automation: true,
    decision_log_context: true,
    multi_cluster: true,
    extended_history: true,
    custom_runbooks: true,
    arguscd: true,
  }

  const features = ref({ ...defaultFeatures })
  const tier = ref('pro')

  async function fetch() {
    try {
      const result = await cachedCallGo('GetFeatures', [], DEFAULT_TTL)
      if (result && Object.keys(result).length > 0) {
        features.value = result
      }
      const t = await cachedCallGo('GetTier', [], DEFAULT_TTL)
      if (t) tier.value = t
    } catch (e) {
      console.warn('[features] backend unavailable, using default (all enabled):', e.message || e)
      // Keep defaults — desktop app gets full access.
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
 * Composable for Argus Scan cluster scan.
 */
export function useArgusScan() {
  const report = ref(null)
  const loading = ref(false)
  const error = ref(null)

  async function runScan() {
    loading.value = true
    error.value = null
    try {
      report.value = await callGo('RunArgusScan')
    } catch (e) {
      error.value = e?.message || String(e)
      report.value = null
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

  async function listResources(kind, namespace, force = false) {
    const ns = namespace || '_all'
    loading.value = result.value === null // only spinner on cold start
    error.value = null
    try {
      if (force) invalidateCache('ListResources', kind, ns)
      result.value = await cachedCallGo('ListResources', [kind, ns], DEFAULT_TTL)
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  async function getResourceDetail(kind, namespace, name) {
    detailLoading.value = true
    try {
      detail.value = await cachedCallGo('GetResourceDetail', [kind, namespace || '', name], DEFAULT_TTL)
    } catch (e) {
      console.error('[resource-detail]', e)
    } finally {
      detailLoading.value = false
    }
  }

  async function listNamespaces() {
    try {
      const res = await cachedCallGo('ListAllNamespaces', [], DEFAULT_TTL)
      if (res) namespaces.value = res
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
 * Composable for interactive pod exec (kubectl exec -it).
 */
export function usePodExec() {
  const connected = ref(false)
  const error = ref(null)

  async function startExec(namespace, podName, container, rows, cols) {
    error.value = null
    try {
      await callGo('ExecPodShell', namespace, podName, container || '', rows, cols)
      connected.value = true
    } catch (e) {
      error.value = e?.message || String(e)
      console.error('[exec]', e)
    }
  }

  async function sendInput(data) {
    try {
      await callGo('SendExecInput', data)
    } catch (e) {
      console.error('[exec-input]', e)
    }
  }

  async function resizeExec(rows, cols) {
    try {
      await callGo('ResizeExec', rows, cols)
    } catch (e) {
      console.error('[exec-resize]', e)
    }
  }

  async function closeExec() {
    try {
      await callGo('CloseExecSession')
    } catch (e) {
      console.error('[exec-close]', e)
    }
    connected.value = false
  }

  return { connected, error, startExec, sendInput, resizeExec, closeExec }
}

/**
 * Composable for resolving service → backing pods.
 */
export function useServicePods() {
  const pods = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function fetchServicePods(namespace, serviceName) {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('GetServicePods', namespace, serviceName)
      pods.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
      pods.value = []
    } finally {
      loading.value = false
    }
  }

  return { pods, loading, error, fetchServicePods }
}


/**
 * Composable for S3-backed notebooks.
 */
export function useNotebooks() {
  const files = ref([])
  const loading = ref(false)
  const saving = ref(false)
  const synced = ref(false)
  const error = ref(null)

  async function listFiles() {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ListNotebooks')
      files.value = result || []
      synced.value = true
    } catch (e) {
      error.value = e?.message || String(e)
      files.value = []
    } finally {
      loading.value = false
    }
  }

  async function getFile(path) {
    try {
      return (await callGo('GetNotebook', path)) || ''
    } catch (e) {
      console.error('[notebooks] getFile failed:', e)
      return ''
    }
  }

  async function saveFile(path, content) {
    saving.value = true
    try {
      await callGo('SaveNotebook', path, content)
      synced.value = true
    } catch (e) {
      synced.value = false
      console.error('[notebooks] saveFile failed:', e)
    } finally {
      saving.value = false
    }
  }

  async function deleteFile(path) {
    try {
      await callGo('DeleteNotebook', path)
    } catch (e) {
      console.error('[notebooks] deleteFile failed:', e)
    }
    files.value = removeFromTree(files.value, path)
  }

  async function createFolder(path) {
    try {
      await callGo('CreateNotebookFolder', path)
      await listFiles()
    } catch (e) {
      console.error('[notebooks] createFolder failed:', e)
    }
  }

  async function testConnection() {
    try {
      await callGo('TestS3Connection')
      return { ok: true }
    } catch (e) {
      return { ok: false, error: e?.message || String(e) }
    }
  }

  /**
   * Add a new file entry to the tree — refreshes from backend.
   */
  function addFileToTree(path, name) {
    listFiles()
  }

  async function moveFile(oldPath, newPath) {
    try {
      await callGo('MoveNotebook', oldPath, newPath)
      await listFiles()
    } catch (e) {
      console.error('[notebooks] moveFile failed:', e)
    }
  }

  return { files, loading, saving, synced, error, listFiles, getFile, saveFile, deleteFile, createFolder, testConnection, addFileToTree, moveFile }
}

/**
 * Remove a file from a nested tree by path.
 */
function removeFromTree(tree, path) {
  return tree
    .filter(item => item.path !== path)
    .map(item => {
      if (item.children) {
        return { ...item, children: removeFromTree(item.children, path) }
      }
      return item
    })
}


/**
 * Composable for runbook CRUD.
 */
export function useRunbooks() {
  const runbooks = ref([])
  const loading = ref(false)
  const saving = ref(false)
  const error = ref(null)
  async function listRunbooks() {
    loading.value = runbooks.value.length === 0
    error.value = null
    try {
      const result = await cachedCallGo('ListRunbooks', [], DEFAULT_TTL)
      runbooks.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
      runbooks.value = []
    } finally {
      loading.value = false
    }
  }

  async function getRunbook(id) {
    try {
      return (await cachedCallGo('GetRunbook', [id], DEFAULT_TTL)) || ''
    } catch (e) {
      console.error('[runbooks] getRunbook failed:', e)
      return ''
    }
  }

  async function saveRunbook(id, content) {
    saving.value = true
    try {
      await callGo('SaveRunbook', id, content)
      invalidateCachePrefix('ListRunbooks')
      invalidateCache('GetRunbook', id)
    } catch (e) {
      console.error('[runbooks] saveRunbook failed:', e)
    } finally {
      saving.value = false
    }
  }

  async function deleteRunbook(id) {
    try {
      await callGo('DeleteRunbook', id)
      invalidateCachePrefix('ListRunbooks')
    } catch (e) {
      console.error('[runbooks] deleteRunbook failed:', e)
    }
    runbooks.value = runbooks.value.filter(rb => rb.id !== id)
  }

  async function createRunbook(name, trigger) {
    try {
      const rb = await callGo('CreateRunbook', name, trigger)
      if (rb) {
        invalidateCachePrefix('ListRunbooks')
        runbooks.value = [...runbooks.value, rb]
        return rb
      }
    } catch (e) {
      console.error('[runbooks] createRunbook failed:', e)
    }
    return null
  }

  return { runbooks, loading, saving, error, listRunbooks, getRunbook, saveRunbook, deleteRunbook, createRunbook }
}

/**
 * Composable for one-click tool setup.
 */
export function useSetup() {
  const tools = ref([])
  const loading = ref(false)
  const actionLoading = ref(null) // which tool is being installed

  async function checkTools() {
    loading.value = tools.value.length === 0
    try {
      const result = await cachedCallGo('CheckToolStatus', [], DEFAULT_TTL)
      tools.value = result || []
    } catch (e) {
      console.error('[setup]', e)
      tools.value = []
    } finally {
      loading.value = false
    }
  }

  async function installArgusScan() {
    actionLoading.value = 'argusScan'
    try {
      const result = await callGo('InstallArgusScan')
      await checkTools()
      return result
    } catch (e) {
      console.error('[setup] installArgusScan:', e)
      return { success: false, message: e?.message || String(e) }
    } finally {
      actionLoading.value = null
    }
  }

  async function deployAgent(namespace) {
    actionLoading.value = 'kubewatcher-agent'
    try {
      const result = await callGo('DeployAgent', namespace || 'kubewatcher')
      await checkTools()
      return result
    } catch (e) {
      console.error('[setup] deployAgent:', e)
      return { success: false, message: e?.message || String(e) }
    } finally {
      actionLoading.value = null
    }
  }

  async function undeployAgent(namespace) {
    actionLoading.value = 'kubewatcher-agent'
    try {
      const result = await callGo('UndeployAgent', namespace || 'kubewatcher')
      await checkTools()
      return result
    } catch (e) {
      console.error('[setup] undeployAgent:', e)
      return { success: false, message: e?.message || String(e) }
    } finally {
      actionLoading.value = null
    }
  }

  return { tools, loading, actionLoading, checkTools, installArgusScan, deployAgent, undeployAgent }
}

/**
 * Composable for Anomaly Agent connectivity.
 */
export function useAnomaly() {
  const anomalies = ref([])
  const settings = ref(null)
  const rules = ref([])
  const jobs = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function connectAgent(namespace = 'all') {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ConnectToAgent', namespace)
      anomalies.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
      anomalies.value = []
    } finally {
      loading.value = false
    }
  }

  async function getSettings() {
    try {
      settings.value = await cachedCallGo('GetAnomalySettings', [], 30_000)
    } catch (e) {
      error.value = e?.message || String(e)
    }
  }

  async function saveSettings(s) {
    try {
      await callGo('SaveAnomalySettings', s)
      settings.value = s
    } catch (e) {
      error.value = e?.message || String(e)
      throw e
    }
  }

  async function getRules() {
    try {
      rules.value = (await callGo('GetAnomalyRules')) || []
    } catch (e) {
      error.value = e?.message || String(e)
    }
  }

  async function saveRule(rule) {
    try {
      await callGo('SaveAnomalyRule', rule)
      await getRules()
    } catch (e) {
      error.value = e?.message || String(e)
      throw e
    }
  }

  async function toggleRule(id) {
    try {
      const newState = await callGo('ToggleAnomalyRule', id)
      const r = rules.value.find(r => r.id === id)
      if (r) r.enabled = newState
    } catch (e) {
      error.value = e?.message || String(e)
    }
  }

  async function deleteRule(id) {
    try {
      await callGo('DeleteAnomalyRule', id)
      rules.value = rules.value.filter(r => r.id !== id)
    } catch (e) {
      error.value = e?.message || String(e)
    }
  }

  async function getJobs() {
    try {
      jobs.value = (await callGo('GetAnomalyJobs')) || []
    } catch (e) {
      // Swallow — might be pro-only or detector not configured.
      jobs.value = []
    }
  }

  return {
    anomalies, settings, rules, jobs, loading, error,
    connectAgent, getSettings, saveSettings,
    getRules, saveRule, toggleRule, deleteRule, getJobs,
  }
}

/**
 * Composable for log querying.
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
 * Composable for incident CRUD.
 */
export function useIncidents() {
  const incidents = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function listIncidents() {
    loading.value = incidents.value.length === 0
    error.value = null
    try {
      const result = await cachedCallGo('ListIncidents', [], DEFAULT_TTL)
      incidents.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
      incidents.value = []
    } finally {
      loading.value = false
    }
  }

  async function createIncident(title, severity, type, description, namespace) {
    try {
      const inc = await callGo('CreateIncident', title, severity || 'info', type || 'alert', description || '', namespace || '')
      if (inc) {
        invalidateCachePrefix('ListIncidents')
        incidents.value = [inc, ...incidents.value]
        return inc
      }
    } catch (e) {
      console.error('[incidents] create:', e)
    }
    return null
  }

  async function updateIncident(id, status, description) {
    try {
      const updated = await callGo('UpdateIncident', id, status || '', description || '')
      if (updated) {
        invalidateCachePrefix('ListIncidents')
        incidents.value = incidents.value.map(i => i.id === id ? updated : i)
        return updated
      }
    } catch (e) {
      console.error('[incidents] update:', e)
    }
  }

  async function deleteIncident(id) {
    try {
      await callGo('DeleteIncident', id)
      invalidateCachePrefix('ListIncidents')
    } catch (e) {
      console.error('[incidents] delete:', e)
    }
    incidents.value = incidents.value.filter(i => i.id !== id)
  }

  return { incidents, loading, error, listIncidents, createIncident, updateIncident, deleteIncident }
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
      console.warn('[arguscd] getApp fallback — using list data')
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

/**
 * Composable for vulnerabilities.
 */
export function useVulnerabilities() {
  const images = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function listVulnerabilities() {
    loading.value = images.value.length === 0
    error.value = null
    try {
      const result = await cachedCallGo('ListVulnerabilities', [], DEFAULT_TTL)
      images.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
      images.value = []
    } finally {
      loading.value = false
    }
  }

  async function scanImage(image, engine) {
    try {
      const result = await callGo('ScanImage', image, engine)
      invalidateCachePrefix('ListVulnerabilities')
      await listVulnerabilities()
      return result
    } catch (e) {
      console.error('[vulnerabilities] scan:', e)
      return 'Scan failed'
    }
  }

  async function scanAllImages(namespace = '') {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ScanAllImages', namespace)
      invalidateCachePrefix('ListVulnerabilities')
      images.value = result || []
      return result
    } catch (e) {
      error.value = e?.message || String(e)
      return null
    } finally {
      loading.value = false
    }
  }

  return { images, loading, error, listVulnerabilities, scanImage, scanAllImages }
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

/**
 * Composable for FinOps cost estimation based on pod resource requests.
 * Supports multi-provider pricing (aws, gcp, azure, digitalocean).
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

/**
 * Composable for node-level system service logs (kubelet, containerd, kube-proxy).
 * Uses the kubelet proxy API to fetch real journal entries from cluster nodes.
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

/**
 * Composable for code blocks and sandboxing.
 */
export function useCodeBlock() {
  const isRunning = ref(false)
  const output = ref('')
  const isAnalyzing = ref(false)
  const suggestion = ref('')

  async function runCode(code, language) {
    isRunning.value = true
    output.value = ''
    try {
      const result = await callGo('RunCodeSandbox', code, language)
      output.value = result || 'No output'
    } catch (e) {
      output.value = e?.message || String(e)
    } finally {
      isRunning.value = false
    }
  }

  async function getAiSuggestion(code, language) {
    isAnalyzing.value = true
    suggestion.value = ''
    try {
      const result = await callGo('GetCodeSuggestion', code, language)
      suggestion.value = result || 'No suggestion available'
    } catch (e) {
      suggestion.value = e?.message || String(e)
    } finally {
      isAnalyzing.value = false
    }
  }

  return { isRunning, output, isAnalyzing, suggestion, runCode, getAiSuggestion }
}

/**
 * Composable for workflow CRUD.
 */
export function useWorkflows() {
  const workflows = ref([])
  const current = ref(null)
  const loading = ref(false)
  const saving = ref(false)
  const error = ref(null)

  async function listWorkflows() {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ListWorkflows')
      workflows.value = result || []
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  async function getWorkflow(id) {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('GetWorkflow', id)
      if (result) current.value = result
      return result
    } catch (e) {
      error.value = e?.message || String(e)
      return null
    } finally {
      loading.value = false
    }
  }

  async function saveWorkflow(wf) {
    saving.value = true
    error.value = null
    try {
      const result = await callGo('SaveWorkflow', wf)
      if (result) {
        current.value = result
        // Update the list entry or add new.
        const idx = workflows.value.findIndex(w => w.id === result.id)
        const summary = { id: result.id, title: result.title, stepCount: (result.steps || []).length, createdAt: result.createdAt, updatedAt: result.updatedAt }
        if (idx >= 0) {
          workflows.value[idx] = summary
        } else {
          workflows.value.unshift(summary)
        }
      }
      return result
    } catch (e) {
      error.value = e?.message || String(e)
      return null
    } finally {
      saving.value = false
    }
  }

  async function deleteWorkflow(id) {
    try {
      await callGo('DeleteWorkflow', id)
      workflows.value = workflows.value.filter(w => w.id !== id)
      if (current.value?.id === id) current.value = null
    } catch (e) {
      error.value = e?.message || String(e)
    }
  }

  return { workflows, current, loading, saving, error, listWorkflows, getWorkflow, saveWorkflow, deleteWorkflow }
}
