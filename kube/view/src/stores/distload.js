import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { callGo, cachedCallGo } from '../composables/useBridge'

const CACHE_TTL = 30_000
const POLL_INTERVAL = 1500
const MAX_POLL_RETRIES = 3

export const useDistLoadStore = defineStore('distload', () => {
  // ── static data loaded once ─────────────────────────────────────────
  const regions = ref([])
  const regionsLoading = ref(false)

  // ── active run state ────────────────────────────────────────────────
  const activeRunId = ref(null)
  const status = ref(null)
  const loading = ref(false)
  const error = ref(null)

  // ── history ─────────────────────────────────────────────────────────
  const runHistory = ref([])
  const historyLoading = ref(false)

  // ── credits ─────────────────────────────────────────────────────────
  const creditBalance = ref(null)
  const creditHistory = ref([])
  const creditsLoading = ref(false)

  // ── polling ─────────────────────────────────────────────────────────
  let pollTimer = null
  let pollRetries = 0
  const cancelledLocally = new Set()

  // ── computed ────────────────────────────────────────────────────────
  const isRunning = computed(() => {
    const s = status.value?.state
    return s === 'pending' || s === 'provisioning' || s === 'running'
  })

  const isDone = computed(() => {
    const s = status.value?.state
    return s === 'done' || s === 'collapsing' || s === 'canceled' || s === 'error'
  })

  const isTerminal = computed(() => {
    const s = status.value?.state
    return s === 'done' || s === 'canceled' || s === 'error'
  })

  const hasRegions = computed(() => regions.value.length > 0)

  // ── actions ─────────────────────────────────────────────────────────

  async function loadRegions() {
    regionsLoading.value = true
    try {
      const result = await cachedCallGo('ListDistLoadRegions', [], CACHE_TTL)
      regions.value = result ?? []
    } catch (e) {
      error.value = e.message ?? String(e)
    } finally {
      regionsLoading.value = false
    }
  }

  async function loadCredits() {
    creditsLoading.value = true
    try {
      const [balance, history] = await Promise.all([
        callGo('GetDistLoadCreditBalance'),
        callGo('GetDistLoadCreditHistory'),
      ])
      creditBalance.value = balance ?? 0
      creditHistory.value = history ?? []
    } catch (e) {
      error.value = e.message ?? String(e)
    } finally {
      creditsLoading.value = false
    }
  }

  async function loadHistory() {
    historyLoading.value = true
    try {
      const result = await callGo('GetDistLoadHistory')
      runHistory.value = result ?? []
    } catch (e) {
      error.value = e.message ?? String(e)
    } finally {
      historyLoading.value = false
    }
  }

  async function start(spec) {
    error.value = null
    loading.value = true
    try {
      const runId = await callGo('StartDistributedLoadTest', spec)
      activeRunId.value = runId
      status.value = { runId, state: 'provisioning', startedAt: new Date().toISOString() }
      startPolling(runId)
      return runId
    } catch (e) {
      error.value = e.message ?? String(e)
      throw e
    } finally {
      loading.value = false
    }
  }

  function startPolling(runId) {
    stopPolling()
    pollRetries = 0
    const poll = async () => {
      if (cancelledLocally.has(runId)) return
      try {
        const result = await callGo('GetDistributedLoadTestStatus', runId)
        pollRetries = 0
        onStatusUpdate(result)
        if (!isTerminal.value) {
          pollTimer = setTimeout(poll, POLL_INTERVAL)
        }
      } catch (e) {
        pollRetries++
        error.value = `Polling failed (${pollRetries}/${MAX_POLL_RETRIES}): ${e.message ?? String(e)}`
        if (pollRetries < MAX_POLL_RETRIES) {
          const backoff = 2000 * Math.pow(2, pollRetries)
          pollTimer = setTimeout(poll, backoff)
        } else {
          error.value = 'Lost connection to Argus SaaS platform. Check your network connection.'
        }
      }
    }
    poll()
  }

  function stopPolling() {
    if (pollTimer) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
  }

  function onStatusUpdate(payload) {
    if (!payload) return
    if (cancelledLocally.has(payload.runId)) return
    status.value = payload
  }

  async function cancel() {
    if (!activeRunId.value) return
    const runId = activeRunId.value
    cancelledLocally.add(runId)
    try {
      await callGo('CancelDistributedLoadTest', runId)
    } catch (e) {
      error.value = e.message ?? String(e)
    } finally {
      stopPolling()
      if (status.value) {
        status.value = { ...status.value, state: 'canceled' }
      }
    }
  }

  async function fetchResult(runId) {
    try {
      const result = await callGo('GetDistributedLoadTestResult', runId)
      return result
    } catch (e) {
      error.value = e.message ?? String(e)
      throw e
    }
  }

  async function estimateCost(spec) {
    try {
      return await callGo('EstimateDistLoadCost', spec)
    } catch (e) {
      error.value = e.message ?? String(e)
      return 0
    }
  }

  function reset() {
    stopPolling()
    activeRunId.value = null
    status.value = null
    error.value = null
    loading.value = false
  }

  return {
    regions,
    regionsLoading,
    activeRunId,
    status,
    loading,
    error,
    runHistory,
    historyLoading,
    creditBalance,
    creditHistory,
    creditsLoading,
    isRunning,
    isDone,
    isTerminal,
    hasRegions,
    loadRegions,
    loadCredits,
    loadHistory,
    start,
    startPolling,
    stopPolling,
    cancel,
    fetchResult,
    estimateCost,
    reset,
  }
})
