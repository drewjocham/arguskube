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

  it('renders one row per visible section (6 by default — core only)', () => {
    const wrapper = createWrapper()
    const rows = wrapper.findAll('[data-testid^="sidebar-section-"]')
    // Core sections per navVisibility defaults: monitoring, cluster,
    // workloads, network, operations, admin. Admin is core because it
    // owns Settings — hiding it would lock users out.
    expect(rows.length).toBe(6)
  })

  it('shows additional sections after the user enables them', async () => {
    const wrapper = createWrapper()
    const vis = useNavVisibilityStore()
    vis.show('storage')
    vis.show('knowledge')
    await nextTick()
    const rows = wrapper.findAll('[data-testid^="sidebar-section-"]')
    // 6 core + 2 newly enabled.
    expect(rows.length).toBe(8)
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

  // --- §C3 right-click quick-toggle menu ---

  it('right-click on an optional section reveals Hide + Show all + Open settings', async () => {
    const vis = useNavVisibilityStore()
    vis.show('storage')
    const wrapper = createWrapper()
    await nextTick()
    const row = wrapper.find('[data-testid="sidebar-section-storage"]')
    await row.trigger('contextmenu', { clientX: 100, clientY: 200 })
    expect(document.querySelector('[data-testid="sidebar-section-menu-hide"]')).toBeTruthy()
    expect(document.querySelector('[data-testid="sidebar-section-menu-show-all"]')).toBeTruthy()
    expect(document.querySelector('[data-testid="sidebar-section-menu-open-settings"]')).toBeTruthy()
  })

  it('right-click on a core section omits the Hide option', async () => {
    const wrapper = createWrapper()
    await wrapper.find('[data-testid="sidebar-section-monitoring"]').trigger('contextmenu')
    expect(document.querySelector('[data-testid="sidebar-section-menu-hide"]')).toBeNull()
    expect(document.querySelector('[data-testid="sidebar-section-menu-show-all"]')).toBeTruthy()
  })

  it('Hide menu item hides the section', async () => {
    const vis = useNavVisibilityStore()
    vis.show('storage')
    const wrapper = createWrapper()
    await nextTick()
    await wrapper.find('[data-testid="sidebar-section-storage"]').trigger('contextmenu')
    const hide = document.querySelector('[data-testid="sidebar-section-menu-hide"]')
    hide.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(vis.isVisible('storage')).toBe(false)
  })

  it('Show all menu item reveals every section', async () => {
    const wrapper = createWrapper()
    await wrapper.find('[data-testid="sidebar-section-monitoring"]').trigger('contextmenu')
    const showAll = document.querySelector('[data-testid="sidebar-section-menu-show-all"]')
    showAll.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    const vis = useNavVisibilityStore()
    for (const id of SECTION_ORDER) expect(vis.isVisible(id)).toBe(true)
  })

  it('Open settings menu item navigates to admin/settings + reveals admin', async () => {
    const wrapper = createWrapper()
    await wrapper.find('[data-testid="sidebar-section-monitoring"]').trigger('contextmenu')
    const open = document.querySelector('[data-testid="sidebar-section-menu-open-settings"]')
    open.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    const vis = useNavVisibilityStore()
    expect(vis.isVisible('admin')).toBe(true)
    const { useSectionTabsStore } = await import('../../stores/sectionTabs')
    expect(useSectionTabsStore().activeTab('admin')).toBe('settings')
    expect(wrapper.emitted('update:activeNav').at(-1)).toEqual(['admin'])
  })

  // --- §C4 Density quick-pick in the sidebar footer ---

  it('density footer renders three buttons with the current selection marked active', () => {
    const wrapper = createWrapper()
    const compact = wrapper.find('[data-testid="density-compact"]')
    const normal = wrapper.find('[data-testid="density-normal"]')
    const comfortable = wrapper.find('[data-testid="density-comfortable"]')
    expect(compact.exists()).toBe(true)
    expect(normal.exists()).toBe(true)
    expect(comfortable.exists()).toBe(true)
    expect(normal.classes()).toContain('active')
  })

  it('clicking a density button updates the appearance store', async () => {
    const wrapper = createWrapper()
    const { useAppearanceStore } = await import('../../stores/appearance')
    const app = useAppearanceStore()
    await wrapper.find('[data-testid="density-compact"]').trigger('click')
    expect(app.density).toBe('compact')
  })

  it('hides the density footer when the sidebar is collapsed', async () => {
    const wrapper = createWrapper()
    await wrapper.find('.collapse-toggle').trigger('click')
    expect(wrapper.find('[data-testid="sidebar-footer"]').exists()).toBe(false)
  })
})
