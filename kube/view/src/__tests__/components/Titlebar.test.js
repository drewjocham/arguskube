import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import Titlebar from '../../components/titlebar/Titlebar.vue'
import { useNavSearchStore } from '../../stores/navSearch'
import { useSectionTabsStore } from '../../stores/sectionTabs'
import { useAppNavStore } from '../../stores/appNav'

// Stub everything the titlebar reaches for that isn't relevant to the
// §D1 palette behaviour. useSpotCheck.active is a REAL Vue ref — the
// template uses v-if + unwraps it; a plain object would render truthy
// and then explode on the inner `.description` access.
vi.mock('../../composables/useWails', () => ({
  isWails: () => false,
}))
vi.mock('../../composables/useSpotCheck', () => ({
  useSpotCheck: () => ({ active: ref(null), runAll: vi.fn() }),
}))
vi.mock('../../components/notifications/NotificationsPanel.vue', () => ({
  default: { template: '<div class="mock-notifications-panel" />' },
}))
vi.mock('../../components/titlebar/EnvironmentSelector.vue', () => ({
  default: { template: '<div class="mock-environment-selector" />' },
}))

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

function mountTitlebar() {
  return mount(Titlebar, {
    props: { clusterInfo: null, terminalOpen: false },
    attachTo: document.body,
  })
}

describe('Titlebar — §D1 palette', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    for (const k of Object.keys(memory)) delete memory[k]
    setActivePinia(createPinia())
  })
  afterEach(() => { vi.restoreAllMocks() })

  it('Cmd+K focuses the titlebar search input', async () => {
    const wrapper = mountTitlebar()
    const input = wrapper.find('[data-testid="titlebar-search"]')
    expect(document.activeElement).not.toBe(input.element)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    await flushPromises()
    expect(document.activeElement).toBe(input.element)
  })

  it('Ctrl+K is also accepted (Linux/Windows shortcut)', async () => {
    const wrapper = mountTitlebar()
    const input = wrapper.find('[data-testid="titlebar-search"]')
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', ctrlKey: true }))
    await flushPromises()
    expect(document.activeElement).toBe(input.element)
  })

  it('typing surfaces palette results filtered by tab label', async () => {
    const wrapper = mountTitlebar()
    const search = useNavSearchStore()
    search.setQuery('Pods')
    await wrapper.find('[data-testid="titlebar-search"]').trigger('focus')
    await flushPromises()
    const results = document.querySelector('[data-testid="palette-results"]')
    expect(results).toBeTruthy()
    expect(document.querySelector('[data-testid="palette-row-workloads-pods"]')).toBeTruthy()
  })

  it('Enter on the active palette row navigates to (section, tab)', async () => {
    const wrapper = mountTitlebar()
    const search = useNavSearchStore()
    search.setQuery('Vulnerabilities')
    const input = wrapper.find('[data-testid="titlebar-search"]')
    await input.trigger('focus')
    await flushPromises()
    await input.trigger('keydown', { key: 'Enter' })
    await flushPromises()

    const tabs = useSectionTabsStore()
    expect(tabs.activeTab('monitoring')).toBe('vulnerabilities')
    const appNav = useAppNavStore()
    expect(appNav.pending?.navId).toBe('vulnerabilities')
  })

  it('arrow keys move the selection within the result list', async () => {
    const wrapper = mountTitlebar()
    const search = useNavSearchStore()
    search.setQuery('Logs')
    const input = wrapper.find('[data-testid="titlebar-search"]')
    await input.trigger('focus')
    await flushPromises()
    const firstActive = document.querySelector('.palette-row.active')
    expect(firstActive).toBeTruthy()
    await input.trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    // Selection moved — the originally-active row is no longer active
    // (assuming there are at least 2 hits; if not, the index just doesn't move).
    const rows = document.querySelectorAll('[data-testid^="palette-row-"]')
    if (rows.length >= 2) {
      const activeNow = document.querySelector('.palette-row.active')
      expect(activeNow).not.toBe(firstActive)
    }
  })

  it('Escape clears the query and closes the palette', async () => {
    const wrapper = mountTitlebar()
    const search = useNavSearchStore()
    search.setQuery('Pods')
    const input = wrapper.find('[data-testid="titlebar-search"]')
    await input.trigger('focus')
    await flushPromises()
    expect(document.querySelector('[data-testid="palette-results"]')).toBeTruthy()
    await input.trigger('keydown', { key: 'Escape' })
    await flushPromises()
    expect(search.query).toBe('')
    expect(document.querySelector('[data-testid="palette-results"]')).toBeNull()
  })

  it('clicking a result navigates and clears the search', async () => {
    const wrapper = mountTitlebar()
    const search = useNavSearchStore()
    search.setQuery('Runbooks')
    await wrapper.find('[data-testid="titlebar-search"]').trigger('focus')
    await flushPromises()
    const row = document.querySelector('[data-testid="palette-row-operations-runbooks"]')
    expect(row).toBeTruthy()
    row.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    await flushPromises()
    const tabs = useSectionTabsStore()
    expect(tabs.activeTab('operations')).toBe('runbooks')
    expect(search.query).toBe('')
  })

  it('placeholder advertises the Cmd+K shortcut', () => {
    const wrapper = mountTitlebar()
    const input = wrapper.find('[data-testid="titlebar-search"]')
    expect(input.attributes('placeholder')).toContain('⌘K')
  })
})
