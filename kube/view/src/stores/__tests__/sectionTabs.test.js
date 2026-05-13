import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSectionTabsStore, __test } from '../sectionTabs'
import { DEFAULT_TABS, SECTION_ORDER, SECTIONS } from '../../lib/sectionTabs'

// Use the project's setup file pattern: a memory-backed localStorage
// so each test gets a clean slate.
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

describe('sectionTabs store', () => {
  beforeEach(() => {
    for (const k of Object.keys(memory)) delete memory[k]
    setActivePinia(createPinia())
  })

  it('initializes with DEFAULT_TABS when nothing is persisted', () => {
    const s = useSectionTabsStore()
    for (const sectionId of SECTION_ORDER) {
      expect(s.tabs[sectionId]).toBe(DEFAULT_TABS[sectionId])
    }
  })

  it('activeTab(sectionId) returns the live value', () => {
    const s = useSectionTabsStore()
    expect(s.activeTab('workloads')).toBe('pods')
    s.setTab('workloads', 'deployments')
    expect(s.activeTab('workloads')).toBe('deployments')
  })

  it('setTab persists across reload', () => {
    const s = useSectionTabsStore()
    s.setTab('monitoring', 'logs')
    // New Pinia instance simulates a reload — the persisted value
    // should be read back.
    setActivePinia(createPinia())
    const s2 = useSectionTabsStore()
    expect(s2.tabs.monitoring).toBe('logs')
  })

  it('setTab rejects unknown sections', () => {
    const s = useSectionTabsStore()
    s.setTab('nonexistent', 'pods')
    expect(s.tabs.nonexistent).toBeUndefined()
  })

  it('setTab rejects tab ids that do not belong to the section', () => {
    const s = useSectionTabsStore()
    // 'pods' is a valid tab, but it's under workloads, not monitoring.
    s.setTab('monitoring', 'pods')
    expect(s.tabs.monitoring).toBe(DEFAULT_TABS.monitoring)
  })

  it('setTab on the active tab is a no-op (no localStorage write)', () => {
    const s = useSectionTabsStore()
    memory[__test.STORAGE_KEY] = JSON.stringify({ monitoring: 'alerts' })
    // Touch the storage write tracker
    let writes = 0
    const origSet = window.localStorage.setItem
    window.localStorage.setItem = (k, v) => { writes++; memory[k] = String(v) }
    s.setTab('monitoring', s.tabs.monitoring)
    window.localStorage.setItem = origSet
    expect(writes).toBe(0)
  })

  it('drops persisted values that no longer correspond to known tabs', () => {
    memory[__test.STORAGE_KEY] = JSON.stringify({
      monitoring: 'this-tab-does-not-exist',
      workloads: 'pods',
    })
    setActivePinia(createPinia())
    const s = useSectionTabsStore()
    expect(s.tabs.monitoring).toBe(DEFAULT_TABS.monitoring)
    expect(s.tabs.workloads).toBe('pods')
  })

  it('resetAll() restores every section to its DEFAULT_TABS choice', () => {
    const s = useSectionTabsStore()
    s.setTab('monitoring', 'logs')
    s.setTab('workloads', 'deployments')
    s.resetAll()
    for (const sectionId of SECTION_ORDER) {
      expect(s.tabs[sectionId]).toBe(DEFAULT_TABS[sectionId])
    }
  })

  it('every section in SECTIONS has at least one tab', () => {
    // Coverage guard for the catalog itself — a section without tabs
    // would render an empty tab bar.
    for (const id of SECTION_ORDER) {
      expect(SECTIONS[id].tabs.length).toBeGreaterThan(0)
    }
  })

  it('every DEFAULT_TABS entry points to a real tab', () => {
    for (const id of SECTION_ORDER) {
      const def = DEFAULT_TABS[id]
      const hasTab = SECTIONS[id].tabs.some((t) => t.id === def)
      expect(hasTab).toBe(true)
    }
  })
})
