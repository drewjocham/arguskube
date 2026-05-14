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

// ── sectionTabs store stub ────────────────────────────────────────────────────
// DistLoadHistory imports useSectionTabsStore but never calls it in its logic.
// Provide a minimal pinia-based stub so the component mounts cleanly.
vi.mock('../../stores/sectionTabs', () => ({
  useSectionTabsStore: () => ({ setTab: vi.fn(), activeTab: vi.fn(() => null) }),
}))

import DistLoadHistory from '../../components/loadtest/DistLoadHistory.vue'

const SAMPLE_RUNS = [
  {
    runId: 'run-aaa',
    name: 'Smoke test',
    state: 'done',
    startedAt: '2026-01-10T10:00:00Z',
    finishedAt: '2026-01-10T10:05:00Z',
    creditsUsed: 12.5,
    summary: { totalSent: 5000, totalAcked: 4990, totalErrors: 10, throughput: 250.0, p50LatencyMs: 6.0, p95LatencyMs: 14.0 },
  },
  {
    runId: 'run-bbb',
    name: 'Stress test',
    state: 'error',
    startedAt: '2026-01-11T12:00:00Z',
    finishedAt: '2026-01-11T12:03:00Z',
    creditsUsed: 8.0,
    error: 'Region us-west-2 failed to provision',
    summary: null,
  },
]

function mountHistory() {
  return mount(DistLoadHistory, { attachTo: document.body })
}

describe('DistLoadHistory.vue', () => {
  beforeEach(() => {
    for (const k of Object.keys(memStorage)) delete memStorage[k]
    setActivePinia(createPinia())
    mockCallGo.mockReset()
    mockCachedCallGo.mockReset()
    document.body.innerHTML = ''
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // ── Loading state ──────────────────────────────────────────────────────────

  it('renders loading state while historyLoading is true and runHistory is empty', async () => {
    mockCallGo.mockImplementation(() => new Promise(() => {}))
    const wrapper = mountHistory()
    await flushPromises()

    expect(wrapper.find('.loading-state').exists()).toBe(true)
    expect(wrapper.text()).toContain('Loading history')
  })

  // ── Empty state ────────────────────────────────────────────────────────────

  it('renders empty state when runHistory is empty after load', async () => {
    mockCallGo.mockResolvedValueOnce([])
    const wrapper = mountHistory()
    await flushPromises()

    expect(wrapper.find('.empty-state').exists()).toBe(true)
    expect(wrapper.text()).toContain('No Load Tests Yet')
    expect(wrapper.find('.history-table').exists()).toBe(false)
  })

  // ── Populated list ─────────────────────────────────────────────────────────

  it('lists runs with their summary fields when runHistory is non-empty', async () => {
    mockCallGo.mockResolvedValueOnce(SAMPLE_RUNS)
    const wrapper = mountHistory()
    await flushPromises()

    expect(wrapper.find('.history-table').exists()).toBe(true)
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(2)

    // First row: smoke test
    expect(rows[0].text()).toContain('Smoke test')
    expect(rows[0].text()).toContain('done')
    expect(rows[0].text()).toContain('12.5')

    // Second row: stress test with error state badge
    expect(rows[1].text()).toContain('Stress test')
    expect(rows[1].text()).toContain('error')
  })

  // ── Click row opens detail panel ──────────────────────────────────────────

  it('clicking a table row shows the run detail panel', async () => {
    mockCallGo.mockResolvedValueOnce(SAMPLE_RUNS)
    const wrapper = mountHistory()
    await flushPromises()

    expect(wrapper.find('.detail-panel').exists()).toBe(false)

    const firstRow = wrapper.find('tbody tr')
    await firstRow.trigger('click')

    expect(wrapper.find('.detail-panel').exists()).toBe(true)
    expect(wrapper.find('.detail-panel').text()).toContain('run-aaa')
    expect(wrapper.find('.detail-panel').text()).toContain('done')
  })

  // ── Detail panel: Back button closes detail ───────────────────────────────

  it('clicking Back in the detail panel returns to the table', async () => {
    mockCallGo.mockResolvedValueOnce(SAMPLE_RUNS)
    const wrapper = mountHistory()
    await flushPromises()

    await wrapper.find('tbody tr').trigger('click')
    expect(wrapper.find('.detail-panel').exists()).toBe(true)

    // "Back" button is the btn-sm inside detail-header
    const backBtn = wrapper.find('.detail-header .btn-sm')
    await backBtn.trigger('click')

    expect(wrapper.find('.detail-panel').exists()).toBe(false)
    expect(wrapper.find('.history-table').exists()).toBe(true)
  })

  // ── onMounted calls loadHistory ────────────────────────────────────────────

  it('calls GetDistLoadHistory on mount', async () => {
    mockCallGo.mockResolvedValueOnce([])
    mountHistory()
    await flushPromises()

    const calls = mockCallGo.mock.calls.filter(c => c[0] === 'GetDistLoadHistory')
    expect(calls.length).toBeGreaterThanOrEqual(1)
  })

  // ── Error state ────────────────────────────────────────────────────────────

  it('renders error banner when store.error is set', async () => {
    mockCallGo.mockRejectedValueOnce(new Error('SaaS history unavailable'))
    const wrapper = mountHistory()
    await flushPromises()

    expect(wrapper.find('.error-banner').exists()).toBe(true)
    expect(wrapper.text()).toContain('SaaS history unavailable')
  })
})
