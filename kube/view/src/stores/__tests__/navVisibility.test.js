import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useNavVisibilityStore, __test } from '../navVisibility'

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

describe('navVisibility store', () => {
  beforeEach(() => {
    for (const k of Object.keys(memory)) delete memory[k]
    setActivePinia(createPinia())
  })

  it('starts with only the 5 core sections visible', () => {
    const s = useNavVisibilityStore()
    for (const id of __test.CORE_SECTIONS) {
      expect(s.visible[id]).toBe(true)
    }
    for (const id of __test.OPTIONAL_SECTIONS) {
      expect(s.visible[id]).toBe(false)
    }
  })

  it('visibleOrder preserves SECTION_ORDER and reflects toggles', () => {
    const s = useNavVisibilityStore()
    const before = s.visibleOrder
    expect(before).toEqual(__test.CORE_SECTIONS.slice())
    s.show('storage')
    expect(s.visibleOrder).toContain('storage')
    // Order check: storage sits AFTER network but BEFORE operations.
    const order = s.visibleOrder
    expect(order.indexOf('storage')).toBeGreaterThan(order.indexOf('network'))
    expect(order.indexOf('storage')).toBeLessThan(order.indexOf('operations'))
  })

  it('toggle flips visibility and persists', () => {
    const s = useNavVisibilityStore()
    s.toggle('storage')
    expect(s.visible.storage).toBe(true)
    s.toggle('storage')
    expect(s.visible.storage).toBe(false)
    // Reload simulation
    setActivePinia(createPinia())
    const s2 = useNavVisibilityStore()
    expect(s2.visible.storage).toBe(false)
  })

  it('toggle ignores unknown section ids', () => {
    const s = useNavVisibilityStore()
    s.toggle('nonexistent')
    expect(s.visible.nonexistent).toBeUndefined()
  })

  it('show/hide are idempotent no-ops when state already matches', () => {
    const s = useNavVisibilityStore()
    s.show('monitoring') // already true
    expect(s.visible.monitoring).toBe(true)
    s.hide('storage') // already false
    expect(s.visible.storage).toBe(false)
  })

  it('resetToDefaults restores the first-launch shape', () => {
    const s = useNavVisibilityStore()
    s.show('storage')
    s.hide('monitoring')
    s.resetToDefaults()
    for (const id of __test.CORE_SECTIONS) {
      expect(s.visible[id]).toBe(true)
    }
    for (const id of __test.OPTIONAL_SECTIONS) {
      expect(s.visible[id]).toBe(false)
    }
  })

  it('merges persisted state into the canonical section map', () => {
    // Persisted state from an older build that only knew 4 sections.
    memory[__test.STORAGE_KEY] = JSON.stringify({
      visible: { monitoring: true, storage: true },
    })
    setActivePinia(createPinia())
    const s = useNavVisibilityStore()
    // New core sections (workloads, network, etc.) still show.
    expect(s.visible.workloads).toBe(true)
    expect(s.visible.network).toBe(true)
    // Persisted preference honored.
    expect(s.visible.storage).toBe(true)
  })

  it('initialize() reveals optional sections when their probe resolves true', async () => {
    const s = useNavVisibilityStore()
    expect(s.initialized).toBe(false)
    await s.initialize({
      storage: () => Promise.resolve(true),
      knowledge: () => Promise.resolve(false),
      config: () => Promise.resolve(true),
      // admin is no longer optional — it's a core section so the
      // probe value is moot; visibility stays true.
    })
    expect(s.initialized).toBe(true)
    expect(s.visible.storage).toBe(true)
    expect(s.visible.config).toBe(true)
    expect(s.visible.knowledge).toBe(false)
    // admin is core; always visible regardless of probes
    expect(s.visible.admin).toBe(true)
  })

  it('initialize() does not hide sections — probes can only reveal', async () => {
    const s = useNavVisibilityStore()
    s.show('storage')
    await s.initialize({
      storage: () => Promise.resolve(false),
    })
    expect(s.visible.storage).toBe(true) // user opt-in survives probe
  })

  it('initialize() swallows probe errors without flipping visibility', async () => {
    const s = useNavVisibilityStore()
    await s.initialize({
      storage: () => Promise.reject(new Error('backend down')),
    })
    expect(s.visible.storage).toBe(false)
    expect(s.initialized).toBe(true)
  })

  it('sections getter shapes per-section metadata for Settings', () => {
    const s = useNavVisibilityStore()
    const list = s.sections
    expect(list.length).toBe(11)
    const monitoring = list.find((x) => x.id === 'monitoring')
    expect(monitoring.core).toBe(true)
    const knowledge = list.find((x) => x.id === 'knowledge')
    expect(knowledge.core).toBe(false)
    expect(knowledge.hint).toContain('S3')
  })

  it('exports a stable CORE/OPTIONAL split', () => {
    expect(__test.CORE_SECTIONS).toContain('monitoring')
    expect(__test.OPTIONAL_SECTIONS).not.toContain('monitoring')
    expect(__test.CORE_SECTIONS.length + __test.OPTIONAL_SECTIONS.length).toBe(11)
  })
})
