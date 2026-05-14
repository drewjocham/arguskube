// DBAgent composable — wraps the Wails bindings exposed by
// kube/backend/api/pkg/app_dbagent.go. Mirrors the shape of the other
// domain composables (useAlerts, useResources): a ref-based state
// blob, plus async actions that the components call.

import { ref } from 'vue'
import { callGo, invalidateCache } from './useBridge'

export function useDBAgent() {
  const connections = ref([])
  const loading = ref(false)
  const error = ref(null)
  const lastAnalysis = ref(null)

  async function list() {
    loading.value = true
    error.value = null
    try {
      connections.value = (await callGo('ListDBConnections')) || []
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  // upsert returns the saved view so the form can pick up the assigned ID.
  async function upsert(input) {
    error.value = null
    try {
      const saved = await callGo('UpsertDBConnection', input)
      invalidateCache('ListDBConnections')
      await list()
      return saved
    } catch (e) {
      error.value = e?.message || String(e)
      throw e
    }
  }

  async function remove(id) {
    error.value = null
    try {
      await callGo('DeleteDBConnection', id)
      invalidateCache('ListDBConnections')
      await list()
    } catch (e) {
      error.value = e?.message || String(e)
    }
  }

  // testConn returns { ok, message, latency_ms } — no throw on driver
  // errors; the caller renders the message inline.
  async function testConn(id) {
    return callGo('TestDBConnection', id)
  }

  async function analyze(id, section = 'overview') {
    error.value = null
    try {
      lastAnalysis.value = await callGo('AnalyzeDB', id, section)
      return lastAnalysis.value
    } catch (e) {
      error.value = e?.message || String(e)
      throw e
    }
  }

  return { connections, loading, error, lastAnalysis, list, upsert, remove, testConn, analyze }
}
