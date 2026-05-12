import { ref } from 'vue'
import { callGo, cachedCallGo, DEFAULT_TTL } from './useBridge'

/**
 * Composable for one-click tool setup.
 */
export function useSetup() {
  const tools = ref([])
  const loading = ref(false)
  const actionLoading = ref(null)

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
      jobs.value = []
    }
  }

  return {
    anomalies, settings, rules, jobs, loading, error,
    connectAgent, getSettings, saveSettings,
    getRules, saveRule, toggleRule, deleteRule, getJobs,
  }
}
