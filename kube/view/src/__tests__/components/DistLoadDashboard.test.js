import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { useDistLoadStore } from '../../stores/distload'

// ── Bridge mock ──────────────────────────────────────────────────────────────
const mockCallGo = vi.fn()
const mockCachedCallGo = vi.fn()
vi.mock('../../composables/useBridge', () => ({
  callGo: (...args) => mockCallGo(...args),
  cachedCallGo: (...args) => mockCachedCallGo(...args),
}))

// ── Memory localStorage ──────────────────────────────────────────────────────
const memStorage = {}
Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: {
    getItem: (k) => (k in memStorage ? memStorage[k] : null),
    setItem: (k, v) => { memStorage[k] = String(v) },
    removeItem: (k) => { delete memStorage[k] },
    clear: () => { for (const k of Object.keys(memStorage)) delete memStorage[k] },
  },
})

import DistLoadDashboard from '../../components/loadtest/DistLoadDashboard.vue'

function mountDashboard() {
  return mount(DistLoadDashboard, { attachTo: document.body })
}

describe('DistLoadDashboard.vue', () => {
  beforeEach(() => {
    for (const k of Object.keys(memStorage)) delete memStorage[k]
    setActivePinia(createPinia())
    mockCallGo.mockReset()
    mockCachedCallGo.mockReset()
    document.body.innerHTML = ''
    // Prevent onMounted polling from interfering unless explicitly tested.
    mockCallGo.mockResolvedValue(null)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  // ── Empty state (no activeRunId) ────────────────────────────────────────────

  it('renders "No Active Test" when activeRunId is null', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.text()).toContain('No Active Test')
  })

  // ── AUDIT GAP-STATE FIX: activeRunId set but status=null → Initializing ────

  it('renders "Initializing…" (not "No Active Test") when activeRunId is set but status is null', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-abc123'
    store.status = null
    await flushPromises()

    expect(wrapper.text()).not.toContain('No Active Test')
    expect(wrapper.text()).toContain('Initializing')
    // Spinner icon span must exist inside the empty-state div.
    expect(wrapper.find('.spinner-icon').exists()).toBe(true)
  })

  // ── Provisioning state ─────────────────────────────────────────────────────

  it('renders first stage as active in provisioning state', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-prov'
    store.status = { runId: 'run-prov', state: 'provisioning' }
    await flushPromises()

    const steps = wrapper.findAll('.stage-step')
    expect(steps[0].classes()).toContain('active')
    expect(steps[1].classes()).not.toContain('active')
    expect(steps[1].classes()).not.toContain('done')
  })

  // ── Running state ──────────────────────────────────────────────────────────

  it('renders first stage as done and second as active in running state', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-run'
    store.status = { runId: 'run-run', state: 'running' }
    await flushPromises()

    const steps = wrapper.findAll('.stage-step')
    expect(steps[0].classes()).toContain('done')
    expect(steps[1].classes()).toContain('active')
  })

  // ── Done state ─────────────────────────────────────────────────────────────

  it('renders all stages done when state=done', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-done'
    store.status = {
      runId: 'run-done',
      state: 'done',
      summary: { totalSent: 1000, totalAcked: 990, totalErrors: 10, throughput: 100.5, p50LatencyMs: 5.0, p95LatencyMs: 12.0 },
    }
    await flushPromises()

    // With state=done, currentStageIndex=3 → stages 0,1,2 are done, stage 3 is active
    const steps = wrapper.findAll('.stage-step')
    expect(steps[0].classes()).toContain('done')
    expect(steps[1].classes()).toContain('done')
    expect(steps[2].classes()).toContain('done')
  })

  // ── Error state ────────────────────────────────────────────────────────────

  it('renders error banner when store.error is set', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-err'
    store.status = { runId: 'run-err', state: 'error' }
    store.error = 'SaaS platform rejected the run'
    await flushPromises()

    expect(wrapper.find('.error-banner').exists()).toBe(true)
    expect(wrapper.text()).toContain('SaaS platform rejected the run')
  })

  // ── Canceled state ─────────────────────────────────────────────────────────
  // NOTE: currentStageIndex returns -1 for state='canceled' (no branch handles
  // it), so the template condition `canceled: status?.state === 'canceled' &&
  // currentStageIndex >= i` evaluates to false for every step (-1 >= 0 is
  // false). The component still renders the stage-bar and a "Canceled" stage
  // label via stageLabel. This test pins the actual rendered behaviour.

  it('renders the stage-bar and shows the canceled run id when state=canceled', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-cxl'
    store.status = { runId: 'run-cxl', state: 'canceled' }
    await flushPromises()

    // The stage-bar renders (not the empty-state)
    expect(wrapper.find('.stage-bar').exists()).toBe(true)
    // The run id is shown (first 8 chars)
    expect(wrapper.find('.run-id').text()).toContain('run-cxl')
    // stageLabel for 'canceled' falls through to the `return s` branch → "canceled"
    expect(wrapper.find('.stage-title').text().toLowerCase()).toContain('canceled')
  })

  // ── AUDIT CANCEL-CONFIRM: confirm → true → store.cancel called ─────────────

  it('calls store.cancel when user confirms the cancel dialog', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)

    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-cxl2'
    store.status = { runId: 'run-cxl2', state: 'running' }
    // isRunning computed needs store state flags consistent
    vi.spyOn(store, 'cancel').mockResolvedValue(undefined)
    await flushPromises()

    const cancelBtn = wrapper.find('.btn-cancel')
    expect(cancelBtn.exists()).toBe(true)
    await cancelBtn.trigger('click')

    expect(window.confirm).toHaveBeenCalledOnce()
    expect(store.cancel).toHaveBeenCalledOnce()
  })

  // ── AUDIT CANCEL-CONFIRM: confirm → false → store.cancel NOT called ─────────

  it('does NOT call store.cancel when user declines the cancel dialog', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false)

    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-cxl3'
    store.status = { runId: 'run-cxl3', state: 'running' }
    const cancelSpy = vi.spyOn(store, 'cancel').mockResolvedValue(undefined)
    await flushPromises()

    const cancelBtn = wrapper.find('.btn-cancel')
    expect(cancelBtn.exists()).toBe(true)
    await cancelBtn.trigger('click')

    expect(window.confirm).toHaveBeenCalledOnce()
    expect(cancelSpy).not.toHaveBeenCalled()
  })

  // ── onMounted: calls resumeActiveRun when no activeRunId ──────────────────

  it('calls store.resumeActiveRun on mount when activeRunId is null', async () => {
    const store = useDistLoadStore()
    const resumeSpy = vi.spyOn(store, 'resumeActiveRun').mockReturnValue(false)

    mountDashboard()
    await flushPromises()

    expect(resumeSpy).toHaveBeenCalledOnce()
  })

  // ── onUnmounted: calls stopPolling ────────────────────────────────────────

  it('calls store.stopPolling on unmount', async () => {
    const store = useDistLoadStore()
    const stopSpy = vi.spyOn(store, 'stopPolling')

    const wrapper = mountDashboard()
    await flushPromises()
    wrapper.unmount()

    expect(stopSpy).toHaveBeenCalled()
  })

  // ── Per-region worker status rows ─────────────────────────────────────────

  it('renders region cards when status.workers is non-empty', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-workers'
    store.status = {
      runId: 'run-workers',
      state: 'running',
      workers: [
        { region: 'us-east-1', state: 'running', sent: 1000, acked: 990, errors: 10, throughput: 100.0, p50Ms: 5.0, p95Ms: 12.0, p99Ms: 20.0 },
        { region: 'eu-west-1', state: 'running', sent: 500, acked: 495, errors: 5, throughput: 50.0, p50Ms: 7.0, p95Ms: 15.0, p99Ms: 25.0 },
      ],
    }
    await flushPromises()

    const cards = wrapper.findAll('.region-card')
    expect(cards.length).toBe(2)
    expect(cards[0].text()).toContain('us-east-1')
    expect(cards[1].text()).toContain('eu-west-1')
  })

  // ── Per-region provision rows ─────────────────────────────────────────────

  it('renders region cards with VM counts when status.provisionProgress is non-empty', async () => {
    const wrapper = mountDashboard()
    await flushPromises()

    const store = useDistLoadStore()
    store.activeRunId = 'run-prov2'
    store.status = {
      runId: 'run-prov2',
      state: 'provisioning',
      provisionProgress: [
        { region: 'ap-southeast-1', state: 'provisioning', vmsSpec: 4, vmsReady: 2, errorMessage: null },
      ],
    }
    await flushPromises()

    const cards = wrapper.findAll('.region-card')
    expect(cards.length).toBe(1)
    expect(cards[0].text()).toContain('ap-southeast-1')
    expect(cards[0].text()).toContain('2/4')
  })
})
