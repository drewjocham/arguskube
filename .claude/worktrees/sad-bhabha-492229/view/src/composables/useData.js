import { ref } from 'vue'
import { callGo, cachedCallGo, invalidateCache, invalidateCachePrefix, DEFAULT_TTL } from './useBridge'

/**
 * Composable for feature gate checks.
 */
export function useFeatures() {
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
    }
  }

  function isAllowed(feature) {
    return features.value[feature] === true
  }

  return { features, tier, isAllowed, refresh: fetch }
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

function removeFromTree(tree, path) {
  return tree
    .filter(item => item.path !== path)
    .map(item => {
      if (item.children) return { ...item, children: removeFromTree(item.children, path) }
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
