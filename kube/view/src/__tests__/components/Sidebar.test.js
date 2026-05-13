import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { setActivePinia, createPinia } from 'pinia'
import Sidebar from '../../components/sidebar/Sidebar.vue'
import { useNavVisibilityStore } from '../../stores/navVisibility'
import { useSectionTabsStore } from '../../stores/sectionTabs'
import { SECTION_ORDER } from '../../lib/sectionTabs'

// useContexts is mocked at the barrel level so the Sidebar's
// onMounted(listContexts) doesn't try to hit Wails.
vi.mock('../../composables/useWails', () => ({
  useContexts: vi.fn(() => ({
    contexts: [],
    loading: false,
    switching: false,
    listContexts: vi.fn(),
    switchContext: vi.fn(),
  })),
}))

const defaultProvide = { isAllowed: () => true }

// Memory-backed localStorage so each test gets a clean state for the
// per-section tab store + navVisibility store.
const memory = {}
Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: {
    getItem: (k) => (k in memory ? memory[k] : null),
    setItem: (k, v) => { memory[k] = String(v) },
    removeItem: (k) => { delete memory[k] },
    clear: () => { for (const k of Object.keys(memory)) delete memory[k] },
  },
})

function createWrapper(props = {}, provide = {}) {
  return mount(Sidebar, {
    props: {
      clusterInfo: null,
      alerts: [],
      activeNav: 'monitoring',
      ...props,
    },
    global: { provide: { ...defaultProvide, ...provide } },
    attachTo: document.body,
  })
}

describe('Sidebar.vue — section navigation model', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    for (const k of Object.keys(memory)) delete memory[k]
    setActivePinia(createPinia())
  })

  it('renders one row per visible section (5 by default — core only)', () => {
    const wrapper = createWrapper()
    const rows = wrapper.findAll('[data-testid^="sidebar-section-"]')
    // Core sections per navVisibility defaults.
    expect(rows.length).toBe(5)
  })

  it('shows additional sections after the user enables them', async () => {
    const wrapper = createWrapper()
    const vis = useNavVisibilityStore()
    vis.show('storage')
    vis.show('knowledge')
    await nextTick()
    const rows = wrapper.findAll('[data-testid^="sidebar-section-"]')
    expect(rows.length).toBe(7)
    expect(wrapper.find('[data-testid="sidebar-section-storage"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="sidebar-section-knowledge"]').exists()).toBe(true)
  })

  it('emits update:activeNav with the section id when a row is clicked', async () => {
    const wrapper = createWrapper()
    await wrapper.find('[data-testid="sidebar-section-workloads"]').trigger('click')
    const events = wrapper.emitted('update:activeNav')
    expect(events).toBeTruthy()
    expect(events[0]).toEqual(['workloads'])
  })

  it('marks the active section row', () => {
    const wrapper = createWrapper({ activeNav: 'workloads' })
    const active = wrapper.find('[data-testid="sidebar-section-workloads"]')
    expect(active.classes()).toContain('active')
    const inactive = wrapper.find('[data-testid="sidebar-section-monitoring"]')
    expect(inactive.classes()).not.toContain('active')
  })

  it('renders the alert badge on the monitoring row (critical wins over warning)', () => {
    const wrapper = createWrapper({
      alerts: [
        { severity: 'critical' },
        { severity: 'warning' },
        { severity: 'warning' },
      ],
    })
    const monitoring = wrapper.find('[data-testid="sidebar-section-monitoring"]')
    const red = monitoring.find('.badge-red')
    expect(red.exists()).toBe(true)
    expect(red.text()).toBe('1')
  })

  it('renders amber badge when only warnings', () => {
    const wrapper = createWrapper({
      alerts: [
        { severity: 'warning' },
        { severity: 'warning' },
      ],
    })
    const amber = wrapper.find('[data-testid="sidebar-section-monitoring"] .badge-amber')
    expect(amber.exists()).toBe(true)
    expect(amber.text()).toBe('2')
  })

  it('no badge appears on non-monitoring sections', () => {
    const wrapper = createWrapper({
      alerts: [{ severity: 'critical' }],
    })
    expect(wrapper.find('[data-testid="sidebar-section-workloads"] .badge').exists()).toBe(false)
  })

  it('preserves the cluster selector', () => {
    const wrapper = createWrapper({
      clusterInfo: { name: 'prod', nodeCount: 12, k8sVersion: 'v1.29.5' },
    })
    const selector = wrapper.find('.cluster-selector')
    expect(selector.exists()).toBe(true)
    expect(selector.text()).toContain('prod')
    expect(selector.text()).toContain('12 nodes')
  })

  it('preserves the AI context card', () => {
    const wrapper = createWrapper()
    expect(wrapper.find('.ai-context-card').exists()).toBe(true)
    expect(wrapper.find('.ai-context-card').text()).toContain('Argus Context')
  })

  it('clicking a search-result tab hit sets the section tab + emits the section', async () => {
    const wrapper = createWrapper()
    // Programmatically activate the nav-search store; the actual search
    // input lives in the titlebar, but the store is the single source of
    // truth Sidebar reads from.
    const { useNavSearchStore } = await import('../../stores/navSearch')
    const search = useNavSearchStore()
    search.setQuery('Pods')
    await nextTick()
    const hit = wrapper.find('[data-testid="sidebar-hit-workloads-pods"]')
    expect(hit.exists()).toBe(true)
    await hit.trigger('click')

    const tabs = useSectionTabsStore()
    expect(tabs.activeTab('workloads')).toBe('pods')
    expect(wrapper.emitted('update:activeNav')[0]).toEqual(['workloads'])
  })

  it('SECTION_ORDER is the source of truth for row order', async () => {
    const vis = useNavVisibilityStore()
    for (const id of SECTION_ORDER) vis.show(id)
    const wrapper = createWrapper()
    await nextTick()
    const rows = wrapper.findAll('[data-testid^="sidebar-section-"]')
    const renderedOrder = rows.map((r) => r.attributes('data-testid').replace('sidebar-section-', ''))
    expect(renderedOrder).toEqual([...SECTION_ORDER])
  })
})
