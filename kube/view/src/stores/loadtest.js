import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { callGo } from '../composables/useBridge'

// Maximum samples we keep live in the ring buffer. Older entries are
// dropped so the chart never holds more than ~2000 data points —
// enough for 500 seconds at 4 fps without unbounded memory growth.
const SAMPLE_CAP = 2000

export const useLoadTestStore = defineStore('loadtest', () => {
  // ── static data loaded once on mount ──────────────────────────────
  /** @type {import('vue').Ref<Array<{id:string,name:string,description:string,whenToUse:string,spec:object}>>} */
  const presets = ref([])
  /** @type {import('vue').Ref<string[]>} */
  const kinds = ref([])

  // ── active run state ───────────────────────────────────────────────
  const activeRunId = ref(/** @type {string|null} */ (null))
  /** @type {import('vue').Ref<object|null>} full LoadTestStatus shape from Go */
  const status = ref(null)
  /** @type {import('vue').Ref<Array<{at:string,ackLatencyNs:number,ok:boolean,err?:string}>>} */
  const samplesBuffer = ref([])
  /** @type {import('vue').Ref<Array<{at:string,phase:string,replicas:number,ready:number}>>} */
  const scaleLogBuffer = ref([])

  const loading = ref(false)
  const error = ref(/** @type {string|null} */ (null))

  // ── derived ────────────────────────────────────────────────────────
  const isRunning = computed(() => {
    const s = status.value?.state
    return s === 'pending' || s === 'running'
  })

  const isDone = computed(() => {
    const s = status.value?.state
    return s === 'done' || s === 'canceled' || s === 'error'
  })

  // ── throughput series for the SVG chart ───────────────────────────
  // Returns [{elapsedSec, msgsPerSec}] bucketed by second.
  const throughputSeries = computed(() => {
    const buf = samplesBuffer.value
    if (!buf.length) return []
    const bySecond = new Map()
    const t0 = new Date(buf[0].at).getTime()
    for (const s of buf) {
      const sec = Math.floor((new Date(s.at).getTime() - t0) / 1000)
      bySecond.set(sec, (bySecond.get(sec) ?? 0) + 1)
    }
    const pairs = [...bySecond.entries()].sort((a, b) => a[0] - b[0])
    return pairs.map(([sec, count]) => ({ elapsedSec: sec, msgsPerSec: count }))
  })

  // ── actions ────────────────────────────────────────────────────────
  async function loadPresets() {
    try {
      const result = await callGo('ListLoadTestPresets')
      presets.value = result ?? []
    } catch (e) {
      error.value = e.message ?? String(e)
    }
  }

  async function loadKinds() {
    try {
      const result = await callGo('ListBrokerKinds')
      kinds.value = result ?? []
    } catch (e) {
      error.value = e.message ?? String(e)
    }
  }

  /** Copy preset RunSpec fields into the form spec. Returns the preset or null. */
  function getPreset(id) {
    return presets.value.find((p) => p.id === id) ?? null
  }

  /**
   * Kick off a run. Returns the runId string from the backend.
   * @param {object} spec  RunSpec-shaped object (broker, payload, count, ramp, scale…)
   */
  async function start(spec) {
    error.value = null
    loading.value = true
    try {
      const runId = await callGo('StartLoadTest', spec)
      activeRunId.value = runId
      status.value = { runId, state: 'pending', summary: {} }
      samplesBuffer.value = []
      scaleLogBuffer.value = []
      return runId
    } catch (e) {
      error.value = e.message ?? String(e)
      throw e
    } finally {
      loading.value = false
    }
  }

  async function cancel() {
    if (!activeRunId.value) return
    try {
      await callGo('CancelLoadTest', activeRunId.value)
    } catch (e) {
      error.value = e.message ?? String(e)
    }
  }

  /**
   * Handle argus:loadtest:progress payload.
   * payload: { runId, samples, scaleLog, emittedAt }
   */
  function onProgress(payload) {
    if (!payload) return
    // Append samples, then trim to ring cap.
    if (Array.isArray(payload.samples) && payload.samples.length) {
      samplesBuffer.value = [...samplesBuffer.value, ...payload.samples].slice(-SAMPLE_CAP)
    }
    if (Array.isArray(payload.scaleLog) && payload.scaleLog.length) {
      scaleLogBuffer.value = [...scaleLogBuffer.value, ...payload.scaleLog]
    }
  }

  /**
   * Handle argus:loadtest:done payload (full LoadTestStatus).
   */
  function onDone(payload) {
    if (!payload) return
    status.value = payload
    // If the backend sends a final flush of samples inside the done
    // status, we don't need to re-append — summary is in payload.summary.
  }

  function reset() {
    activeRunId.value = null
    status.value = null
    samplesBuffer.value = []
    scaleLogBuffer.value = []
    error.value = null
    loading.value = false
  }

  return {
    // state
    presets,
    kinds,
    activeRunId,
    status,
    samplesBuffer,
    scaleLogBuffer,
    loading,
    error,
    // computed
    isRunning,
    isDone,
    throughputSeries,
    // actions
    loadPresets,
    loadKinds,
    getPreset,
    start,
    cancel,
    onProgress,
    onDone,
    reset,
  }
})
