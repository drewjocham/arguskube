import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { callGo, cachedCallGo } from '../composables/useBridge'

const CACHE_TTL = 30_000
const POLL_INTERVAL = 1500
const MAX_POLL_RETRIES = 3

// ACTIVE_RUN_KEY persists the in-flight run id across reload. The user
// closing the tab mid-run used to lose tracking — they had no way to
// rejoin a real cloud run that was still burning credits in the
// background. We persist runId + state on every status update; on
// store init we resume polling if a non-terminal run was active.
const ACTIVE_RUN_KEY = 'argus.distload.activeRun.v1'

const TERMINAL_STATES = new Set(['done', 'canceled', 'error'])

function loadPersistedActiveRun() {
  try {
    const raw = localStorage.getItem(ACTIVE_RUN_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed?.runId) return null
    if (TERMINAL_STATES.has(parsed.state)) return null
    return parsed
  } catch {
    return null
  }
}

function savePersistedActiveRun(payload) {
  try {
    if (!payload?.runId) {
      localStorage.removeItem(ACTIVE_RUN_KEY)
      return
    }
    localStorage.setItem(ACTIVE_RUN_KEY, JSON.stringify({
      runId: payload.runId,
      state: payload.state,
    }))
  } catch {
    /* best-effort — quota / SSR fallthrough */
  }
}

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

  // ── pre-Start gate ──────────────────────────────────────────────────
  // estimatedCost is set by the form's debounced EstimateDistLoadCost
  // call. canStart pulls credit + estimate together so the user gets
  // a single "insufficient credits" hint before they click Start
  // (instead of after, with a real cloud-side rejection).
  const estimatedCost = ref(null)

  function setEstimatedCost(v) {
    estimatedCost.value = typeof v === 'number' && Number.isFinite(v) ? v : null
  }

  const canStart = computed(() => {
    // Don't gate Start while still loading credit balance — that
    // would trap the user in a stuck Start button.
    if (creditsLoading.value) return false
    if (isRunning.value) return false
    if (loading.value) return false
    // Estimate not yet available → allow; the SaaS side gives the
    // authoritative answer when StartDistLoadTest is called.
    if (estimatedCost.value == null) return true
    const balance = creditBalance.value
    if (balance == null) return true
    return balance >= estimatedCost.value
  })

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
    // Concurrent-run guard: orphaning a real cloud run silently
    // would burn credits on VMs nobody is watching. Reject the
    // second Start so the user has to explicitly Cancel first.
    if (activeRunId.value && isRunning.value) {
      const msg = `A distributed load test is already running (${activeRunId.value}). Cancel it before starting another.`
      error.value = msg
      throw new Error(msg)
    }
    error.value = null
    loading.value = true
    try {
      const runId = await callGo('StartDistributedLoadTest', spec)
      activeRunId.value = runId
      status.value = { runId, state: 'provisioning', startedAt: new Date().toISOString() }
      savePersistedActiveRun(status.value)
      startPolling(runId)
      return runId
    } catch (e) {
      error.value = e.message ?? String(e)
      throw e
    } finally {
      loading.value = false
    }
  }

  // Polling deliberately uses a hand-rolled setTimeout chain rather
  // than VueUse's useIntervalFn. Two requirements force it:
  //
  //   1. Backoff on errors. The poll interval doubles on each
  //      consecutive failure (2s → 4s → 8s) up to MAX_POLL_RETRIES,
  //      then surfaces "Lost connection". useIntervalFn is fixed-
  //      interval; reconfiguring it on every error would be more
  //      code than the current chain.
  //   2. Self-terminating. The chain stops when status reaches a
  //      terminal state (done / canceled / error). useIntervalFn
  //      requires explicit pause(), which is more bookkeeping for
  //      no benefit here.
  //
  // The audit's concern — orphaned timers after unmount — is
  // addressed by DistLoadDashboard.vue's onUnmounted(stopPolling)
  // (landed in PR-24) and by resumeActiveRun()'s persistence so a
  // reload re-attaches instead of double-firing.
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
    if (TERMINAL_STATES.has(payload.state)) {
      savePersistedActiveRun(null)
    } else {
      savePersistedActiveRun(payload)
    }
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
      savePersistedActiveRun(null)
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
      const v = await callGo('EstimateDistLoadCost', spec)
      const n = typeof v === 'number' ? v : Number(v)
      const out = Number.isFinite(n) ? n : null
      setEstimatedCost(out)
      return out
    } catch (e) {
      // null (not 0) so the UI can render "—" instead of a false
      // "free!" estimate when SaaS is unreachable. The audit
      // specifically called this out as confusing.
      error.value = e.message ?? String(e)
      setEstimatedCost(null)
      return null
    }
  }

  function reset() {
    stopPolling()
    activeRunId.value = null
    status.value = null
    error.value = null
    loading.value = false
    savePersistedActiveRun(null)
  }

  // resumeActiveRun is called once at store init. If the user closed
  // the tab mid-run, restore the runId and re-attach polling so they
  // can see the cloud run finish.
  function resumeActiveRun() {
    const persisted = loadPersistedActiveRun()
    if (!persisted) return false
    activeRunId.value = persisted.runId
    status.value = { runId: persisted.runId, state: persisted.state || 'provisioning' }
    startPolling(persisted.runId)
    return true
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
    estimatedCost,
    canStart,
    setEstimatedCost,
    loadRegions,
    loadCredits,
    loadHistory,
    start,
    startPolling,
    stopPolling,
    cancel,
    fetchResult,
    estimateCost,
    resumeActiveRun,
    reset,
  }
})
