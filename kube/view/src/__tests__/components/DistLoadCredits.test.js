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

// Import component after mocks are set up.
import DistLoadCredits from '../../components/loadtest/DistLoadCredits.vue'

function mountCredits() {
  return mount(DistLoadCredits, { attachTo: document.body })
}

describe('DistLoadCredits.vue', () => {
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

  it('shows loading state when creditsLoading=true and creditBalance=null', async () => {
    // loadCredits will never resolve so creditsLoading stays true and balance stays null.
    mockCallGo.mockImplementation(() => new Promise(() => {}))
    const wrapper = mountCredits()
    await flushPromises()

    expect(wrapper.text()).toContain('Loading credit info…')
    expect(wrapper.find('.credits-content').exists()).toBe(false)
  })

  // ── Loaded balance ─────────────────────────────────────────────────────────

  it('renders formatted balance when creditBalance is loaded', async () => {
    mockCallGo.mockResolvedValueOnce(250.0) // balance
    mockCallGo.mockResolvedValueOnce([])    // history
    const wrapper = mountCredits()
    await flushPromises()

    expect(wrapper.find('.balance-number').text()).toBe('250.0')
  })

  // ── AUDIT FINDING #4: null balance must NOT show low-credits warning ────────

  it('does not show low-credits warning when creditBalance is null (not loading)', async () => {
    // Simulate: load completes with null balance (store coerces to 0, but
    // component guards behind creditBalance != null). We directly set store state.
    mockCallGo.mockImplementation(() => new Promise(() => {}))
    const wrapper = mountCredits()

    // Reach into the store and force creditsLoading=false, creditBalance=null
    const store = useDistLoadStore()
    store.creditsLoading = false
    store.creditBalance = null
    await flushPromises()

    expect(wrapper.find('.balance-card').classes()).not.toContain('low')
    expect(wrapper.find('.balance-warning').exists()).toBe(false)
  })

  // ── Low balance ────────────────────────────────────────────────────────────

  it('applies .low class and shows warning when creditBalance < 100', async () => {
    mockCallGo.mockResolvedValueOnce(50)
    mockCallGo.mockResolvedValueOnce([])
    const wrapper = mountCredits()
    await flushPromises()

    expect(wrapper.find('.balance-card').classes()).toContain('low')
    expect(wrapper.find('.balance-warning').exists()).toBe(true)
  })

  // ── Adequate balance ───────────────────────────────────────────────────────

  it('does not apply .low class when creditBalance >= 100', async () => {
    mockCallGo.mockResolvedValueOnce(500)
    mockCallGo.mockResolvedValueOnce([])
    const wrapper = mountCredits()
    await flushPromises()

    expect(wrapper.find('.balance-card').classes()).not.toContain('low')
    expect(wrapper.find('.balance-warning').exists()).toBe(false)
  })

  // ── Transaction history ────────────────────────────────────────────────────

  it('renders transaction rows when creditHistory is non-empty', async () => {
    mockCallGo.mockResolvedValueOnce(200)
    mockCallGo.mockResolvedValueOnce([
      { id: 'txn-1', type: 'purchase', amount: 100, note: 'Top-up', createdAt: '2026-01-01T00:00:00Z' },
      { id: 'txn-2', type: 'usage', amount: -10, note: 'Run xyz', createdAt: '2026-01-02T00:00:00Z' },
    ])
    const wrapper = mountCredits()
    await flushPromises()

    expect(wrapper.find('.transaction-table').exists()).toBe(true)
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBe(2)
    expect(rows[0].text()).toContain('purchase')
    expect(rows[1].text()).toContain('usage')
  })

  // ── Empty transaction history ──────────────────────────────────────────────

  it('renders the empty-state placeholder when creditHistory is empty', async () => {
    mockCallGo.mockResolvedValueOnce(300)
    mockCallGo.mockResolvedValueOnce([])
    const wrapper = mountCredits()
    await flushPromises()

    expect(wrapper.find('.empty-transactions').exists()).toBe(true)
    expect(wrapper.find('.transaction-table').exists()).toBe(false)
  })

  // ── onMounted calls loadCredits ────────────────────────────────────────────

  it('calls GetDistLoadCreditBalance and GetDistLoadCreditHistory on mount', async () => {
    mockCallGo.mockResolvedValue(null)
    mountCredits()
    await flushPromises()

    const balanceCalls = mockCallGo.mock.calls.filter(c => c[0] === 'GetDistLoadCreditBalance')
    const historyCalls = mockCallGo.mock.calls.filter(c => c[0] === 'GetDistLoadCreditHistory')
    expect(balanceCalls.length).toBeGreaterThanOrEqual(1)
    expect(historyCalls.length).toBeGreaterThanOrEqual(1)
  })

  // ── Boundary: exactly 100 credits is not "low" ────────────────────────────

  it('does not show low-credits warning when creditBalance is exactly 100', async () => {
    mockCallGo.mockResolvedValueOnce(100)
    mockCallGo.mockResolvedValueOnce([])
    const wrapper = mountCredits()
    await flushPromises()

    expect(wrapper.find('.balance-card').classes()).not.toContain('low')
    expect(wrapper.find('.balance-warning').exists()).toBe(false)
  })

  // ── Debit / credit cell class ──────────────────────────────────────────────

  it('applies txn-debit class for negative amounts and txn-credit for positive', async () => {
    mockCallGo.mockResolvedValueOnce(500)
    mockCallGo.mockResolvedValueOnce([
      { id: 'txn-d', type: 'usage', amount: -5, note: '', createdAt: null },
      { id: 'txn-c', type: 'purchase', amount: 50, note: '', createdAt: null },
    ])
    const wrapper = mountCredits()
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].find('.txn-debit').exists()).toBe(true)
    expect(rows[1].find('.txn-credit').exists()).toBe(true)
  })
})
