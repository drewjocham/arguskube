import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useLoadTestStore } from '../../stores/loadtest'

// Mock callGo so the store never hits the real bridge.
vi.mock('../../composables/useBridge', () => ({
  callGo: vi.fn(),
}))

import { callGo } from '../../composables/useBridge'

const SAMPLE_CAP = 2000

function makeSample(overrides = {}) {
  return {
    at: new Date().toISOString(),
    ackLatencyNs: 1_000_000,
    ok: true,
    ...overrides,
  }
}

describe('loadtest store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  // ── loadPresets ──────────────────────────────────────────────────
  it('loadPresets populates store.presets from callGo', async () => {
    const fakePresets = [
      { id: 'smoke', name: 'Smoke', description: 'Quick', whenToUse: 'Dev', spec: { count: 1000, ramp: { kind: 'constant', rate: 50 }, workers: 10 } },
    ]
    callGo.mockResolvedValueOnce(fakePresets)
    const store = useLoadTestStore()
    await store.loadPresets()
    expect(store.presets).toHaveLength(1)
    expect(store.presets[0].id).toBe('smoke')
  })

  it('loadPresets sets error on callGo failure', async () => {
    callGo.mockRejectedValueOnce(new Error('network error'))
    const store = useLoadTestStore()
    await store.loadPresets()
    expect(store.error).toBe('network error')
    expect(store.presets).toHaveLength(0)
  })

  // ── loadKinds ────────────────────────────────────────────────────
  it('loadKinds populates store.kinds', async () => {
    callGo.mockResolvedValueOnce(['pubsub', 'nats', 'kafka', 'rabbitmq', 'amqp1'])
    const store = useLoadTestStore()
    await store.loadKinds()
    expect(store.kinds).toHaveLength(5)
    expect(store.kinds).toContain('kafka')
  })

  // ── applyPreset ──────────────────────────────────────────────────
  it('getPreset(id) returns the matching preset', async () => {
    const presets = [
      { id: 'smoke', name: 'Smoke', description: '', whenToUse: '', spec: { count: 1000, ramp: { kind: 'constant', rate: 50 }, workers: 10 } },
      { id: 'soak', name: 'Soak', description: '', whenToUse: '', spec: { count: 100_000, ramp: { kind: 'constant', rate: 100 }, workers: 50 } },
    ]
    callGo.mockResolvedValueOnce(presets)
    const store = useLoadTestStore()
    await store.loadPresets()
    const p = store.getPreset('soak')
    expect(p).not.toBeNull()
    expect(p.spec.count).toBe(100_000)
    expect(p.spec.workers).toBe(50)
    expect(p.spec.ramp.kind).toBe('constant')
  })

  it('getPreset(unknown) returns null', () => {
    const store = useLoadTestStore()
    expect(store.getPreset('does-not-exist')).toBeNull()
  })

  // ── start / cancel ───────────────────────────────────────────────
  it('start() calls StartLoadTest with the spec and sets activeRunId', async () => {
    callGo.mockResolvedValueOnce('run-abc-123')
    const store = useLoadTestStore()
    const spec = { broker: { kind: 'kafka', kafka: {} }, destination: 'test', payload: { kind: 'pasted', bytes: [], size: 2 }, count: 100, workers: 5, ramp: { kind: 'constant', rate: 10 } }
    const runId = await store.start(spec)
    expect(callGo).toHaveBeenCalledWith('StartLoadTest', spec)
    expect(runId).toBe('run-abc-123')
    expect(store.activeRunId).toBe('run-abc-123')
    expect(store.status.state).toBe('pending')
  })

  it('start() sets error and rethrows on failure', async () => {
    callGo.mockRejectedValueOnce(new Error('invalid spec'))
    const store = useLoadTestStore()
    await expect(store.start({})).rejects.toThrow('invalid spec')
    expect(store.error).toBe('invalid spec')
  })

  it('cancel() calls CancelLoadTest with activeRunId', async () => {
    callGo.mockResolvedValueOnce('run-xyz')
    const store = useLoadTestStore()
    store.activeRunId = 'run-xyz'
    callGo.mockResolvedValueOnce(undefined)
    await store.cancel()
    expect(callGo).toHaveBeenCalledWith('CancelLoadTest', 'run-xyz')
  })

  // ── onProgress ring buffer ───────────────────────────────────────
  it('onProgress appends samples to the buffer', () => {
    const store = useLoadTestStore()
    const samples = [makeSample(), makeSample(), makeSample()]
    store.onProgress({ samples, scaleLog: [] })
    expect(store.samplesBuffer).toHaveLength(3)
  })

  it('onProgress caps buffer at SAMPLE_CAP and drops oldest entries', () => {
    const store = useLoadTestStore()
    // Pre-fill to capacity.
    const initial = Array.from({ length: SAMPLE_CAP }, (_, i) =>
      makeSample({ at: new Date(i * 1000).toISOString() })
    )
    store.onProgress({ samples: initial, scaleLog: [] })
    expect(store.samplesBuffer).toHaveLength(SAMPLE_CAP)

    // Adding 50 more should still cap at SAMPLE_CAP and drop the earliest.
    const newer = Array.from({ length: 50 }, (_, i) =>
      makeSample({ at: new Date((SAMPLE_CAP + i) * 1000).toISOString() })
    )
    store.onProgress({ samples: newer, scaleLog: [] })
    expect(store.samplesBuffer).toHaveLength(SAMPLE_CAP)
    // The last item in the buffer should be the newest sample.
    const lastAt = new Date(store.samplesBuffer[SAMPLE_CAP - 1].at).getTime()
    expect(lastAt).toBe((SAMPLE_CAP + 49) * 1000)
  })

  it('onProgress appends scaleLog events', () => {
    const store = useLoadTestStore()
    const scaleLog = [
      { at: new Date().toISOString(), phase: 'pre-scale', replicas: 0, ready: 0 },
    ]
    store.onProgress({ samples: [], scaleLog })
    expect(store.scaleLogBuffer).toHaveLength(1)
    expect(store.scaleLogBuffer[0].phase).toBe('pre-scale')
  })

  it('onProgress is a no-op for null payload', () => {
    const store = useLoadTestStore()
    expect(() => store.onProgress(null)).not.toThrow()
    expect(store.samplesBuffer).toHaveLength(0)
  })

  // ── onDone ───────────────────────────────────────────────────────
  it('onDone stores the full LoadTestStatus', () => {
    const store = useLoadTestStore()
    const donePayload = {
      runId: 'run-1',
      state: 'done',
      startedAt: new Date().toISOString(),
      finishedAt: new Date().toISOString(),
      summary: { sent: 1000, acked: 998, errors: 2, throughputPerSec: 49.9 },
    }
    store.onDone(donePayload)
    expect(store.status.state).toBe('done')
    expect(store.status.summary.sent).toBe(1000)
  })

  // ── isRunning / isDone computed ───────────────────────────────────
  it('isRunning is true while state is running or pending', () => {
    const store = useLoadTestStore()
    store.status = { state: 'running', summary: {} }
    expect(store.isRunning).toBe(true)
    store.status = { state: 'pending', summary: {} }
    expect(store.isRunning).toBe(true)
    store.status = { state: 'done', summary: {} }
    expect(store.isRunning).toBe(false)
  })

  it('isDone is true for done/canceled/error states', () => {
    const store = useLoadTestStore()
    for (const state of ['done', 'canceled', 'error']) {
      store.status = { state, summary: {} }
      expect(store.isDone).toBe(true)
    }
    store.status = { state: 'running', summary: {} }
    expect(store.isDone).toBe(false)
  })

  // ── throughputSeries ──────────────────────────────────────────────
  it('throughputSeries returns empty array when buffer is empty', () => {
    const store = useLoadTestStore()
    expect(store.throughputSeries).toHaveLength(0)
  })

  it('throughputSeries groups samples by second', () => {
    const store = useLoadTestStore()
    const t0 = new Date('2024-01-01T00:00:00.000Z').getTime()
    store.samplesBuffer = [
      { at: new Date(t0).toISOString(), ok: true },
      { at: new Date(t0 + 100).toISOString(), ok: true },
      { at: new Date(t0 + 1200).toISOString(), ok: true },
    ]
    const series = store.throughputSeries
    // Two distinct seconds: 0 and 1
    expect(series.length).toBeGreaterThanOrEqual(2)
    expect(series[0].elapsedSec).toBe(0)
    expect(series[0].msgsPerSec).toBe(2)
    expect(series[1].elapsedSec).toBe(1)
    expect(series[1].msgsPerSec).toBe(1)
  })

  // ── reset ────────────────────────────────────────────────────────
  it('reset clears all run state', () => {
    const store = useLoadTestStore()
    store.activeRunId = 'x'
    store.status = { state: 'done' }
    store.samplesBuffer = [makeSample()]
    store.scaleLogBuffer = [{ at: new Date().toISOString(), phase: 'done', replicas: 2, ready: 2 }]
    store.reset()
    expect(store.activeRunId).toBeNull()
    expect(store.status).toBeNull()
    expect(store.samplesBuffer).toHaveLength(0)
    expect(store.scaleLogBuffer).toHaveLength(0)
  })
})
