import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useRunbookTerminalsStore, __test } from '../runbookTerminals'

const memory = {}
Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: (k) => (k in memory ? memory[k] : null),
    setItem: (k, v) => { memory[k] = String(v) },
    removeItem: (k) => { delete memory[k] },
  },
  writable: true, configurable: true,
})

describe('runbookTerminals store', () => {
  beforeEach(() => {
    for (const k of Object.keys(memory)) delete memory[k]
    vi.resetModules()
    setActivePinia(createPinia())
  })

  async function fresh() {
    setActivePinia(createPinia())
    const mod = await import('../runbookTerminals.js')
    return mod.useRunbookTerminalsStore()
  }

  it('starts with no pins or overrides', async () => {
    const s = await fresh()
    expect(s.pinned).toEqual({})
    expect(s.overrides).toEqual({})
  })

  it('resolveTarget falls back to the section default when nothing is pinned', async () => {
    const s = await fresh()
    expect(s.resolveTarget('rb-1', 'verify-pods', 0)).toBe('rb-1::verify-pods')
  })

  it('resolveTarget uses the pinned section after pinDocument', async () => {
    const s = await fresh()
    s.pinDocument('rb-1', 'verify-pods')
    // Even when querying a different sectionId, the resolver returns the pin.
    expect(s.resolveTarget('rb-1', 'somewhere-else', 5)).toBe('rb-1::verify-pods')
    expect(s.isPinned('rb-1')).toBe(true)
  })

  it('pin scope is per-document — other runbooks are unaffected', async () => {
    const s = await fresh()
    s.pinDocument('rb-1', 'verify-pods')
    expect(s.resolveTarget('rb-2', 'restart', 0)).toBe('rb-2::restart')
    expect(s.isPinned('rb-2')).toBe(false)
  })

  it('unpinDocument removes the pin', async () => {
    const s = await fresh()
    s.pinDocument('rb-1', 'verify-pods')
    s.unpinDocument('rb-1')
    expect(s.isPinned('rb-1')).toBe(false)
    expect(s.resolveTarget('rb-1', 'restart', 0)).toBe('rb-1::restart')
  })

  it('per-block override beats both pin and section default', async () => {
    const s = await fresh()
    s.pinDocument('rb-1', 'verify-pods')
    s.setBlockOverride('rb-1', 3, 'my-custom-session')
    expect(s.resolveTarget('rb-1', 'verify-pods', 3)).toBe('my-custom-session')
    // A different block under the same doc still respects the pin.
    expect(s.resolveTarget('rb-1', 'verify-pods', 4)).toBe('rb-1::verify-pods')
  })

  it('setBlockOverride(null) clears the override', async () => {
    const s = await fresh()
    s.setBlockOverride('rb-1', 3, 'custom')
    s.setBlockOverride('rb-1', 3, null)
    expect(s.resolveTarget('rb-1', 'restart', 3)).toBe('rb-1::restart')
  })

  it('clearDocument wipes pin and every block override for that runbook only', async () => {
    const s = await fresh()
    s.pinDocument('rb-1', 'sec-a')
    s.setBlockOverride('rb-1', 0, 'cust-a')
    s.setBlockOverride('rb-1', 1, 'cust-b')
    s.setBlockOverride('rb-2', 0, 'cust-other')

    s.clearDocument('rb-1')
    expect(s.isPinned('rb-1')).toBe(false)
    expect(s.overrides['rb-1::block:0']).toBeUndefined()
    expect(s.overrides['rb-1::block:1']).toBeUndefined()
    expect(s.overrides['rb-2::block:0']).toBe('cust-other')
  })

  it('persists pin + overrides across a fresh load', async () => {
    const a = await fresh()
    a.pinDocument('rb-1', 'sec-a')
    a.setBlockOverride('rb-1', 2, 'cust')
    const b = await fresh()
    expect(b.isPinned('rb-1')).toBe(true)
    expect(b.overrides['rb-1::block:2']).toBe('cust')
  })

  it('pinnedSessionFor returns null when not pinned, sessionId when pinned', async () => {
    const s = await fresh()
    expect(s.pinnedSessionFor('rb-1')).toBeNull()
    s.pinDocument('rb-1', 'verify')
    expect(s.pinnedSessionFor('rb-1')).toBe('rb-1::verify')
  })
})

describe('buildSessionId (internal)', () => {
  it('namespaces sessionId by runbookId', () => {
    expect(__test.buildSessionId('rb-1', 'verify')).toBe('rb-1::verify')
    expect(__test.buildSessionId('', 'verify')).toBe('untitled::verify')
  })
})
