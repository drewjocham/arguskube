import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSavedFiltersStore, __test } from '../savedFilters'

const memory = {}
Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: (k) => (k in memory ? memory[k] : null),
    setItem: (k, v) => { memory[k] = String(v) },
    removeItem: (k) => { delete memory[k] },
  },
  writable: true, configurable: true,
})

describe('savedFilters store', () => {
  beforeEach(() => {
    for (const k of Object.keys(memory)) delete memory[k]
    vi.resetModules()
    setActivePinia(createPinia())
  })

  async function fresh() {
    setActivePinia(createPinia())
    const mod = await import('../savedFilters.js')
    return mod.useSavedFiltersStore()
  }

  it('starts empty when nothing is persisted', async () => {
    const s = await fresh()
    expect(s.entries).toEqual([])
  })

  it('save() creates a new entry with id, name, query, filters, limit', async () => {
    const s = await fresh()
    const id = s.save('errors only', {
      query: '{level="error"}',
      filters: [{ field: 'kubernetes.pod_namespace', value: 'prod' }],
      limit: 200,
    })
    expect(id).toBeTypeOf('string')
    expect(s.entries).toHaveLength(1)
    const e = s.entries[0]
    expect(e.name).toBe('errors only')
    expect(e.query).toBe('{level="error"}')
    expect(e.filters).toEqual([{ field: 'kubernetes.pod_namespace', value: 'prod' }])
    expect(e.limit).toBe(200)
    expect(typeof e.createdAt).toBe('string')
  })

  it('save() trims whitespace and rejects empty names', async () => {
    const s = await fresh()
    expect(s.save('   ', { query: 'x' })).toBeNull()
    s.save('   spaced  ', { query: 'x' })
    expect(s.entries[0].name).toBe('spaced')
  })

  it('save() updates an existing entry by name in place (no duplicates)', async () => {
    const s = await fresh()
    const id1 = s.save('errors', { query: 'a', filters: [], limit: 50 })
    const id2 = s.save('errors', { query: 'b', filters: [], limit: 100 })
    expect(s.entries).toHaveLength(1)
    expect(id2).toBe(id1)
    expect(s.entries[0].query).toBe('b')
    expect(s.entries[0].limit).toBe(100)
  })

  it('persists to localStorage and reloads on a fresh store', async () => {
    const a = await fresh()
    a.save('persistent', { query: '{}', filters: [], limit: 100 })
    const b = await fresh()
    expect(b.entries).toHaveLength(1)
    expect(b.entries[0].name).toBe('persistent')
  })

  it('remove() drops the matching id', async () => {
    const s = await fresh()
    const id = s.save('a', { query: '1', filters: [] })
    s.save('b', { query: '2', filters: [] })
    s.remove(id)
    expect(s.entries.map(e => e.name)).toEqual(['b'])
  })

  it('clear() removes everything', async () => {
    const s = await fresh()
    s.save('a', { query: '1', filters: [] })
    s.save('b', { query: '2', filters: [] })
    s.clear()
    expect(s.entries).toEqual([])
  })

  it('caps to MAX_ENTRIES', async () => {
    const s = await fresh()
    for (let i = 0; i < __test.MAX_ENTRIES + 5; i++) {
      s.save(`set-${i}`, { query: `q${i}`, filters: [] })
    }
    expect(s.entries.length).toBe(__test.MAX_ENTRIES)
    // Oldest evicted; first remaining should NOT be set-0.
    expect(s.entries[0].name).not.toBe('set-0')
  })

  it('sortedEntries returns newest-first', async () => {
    const s = await fresh()
    s.save('older', { query: 'a', filters: [] })
    await new Promise(r => setTimeout(r, 5))
    s.save('newer', { query: 'b', filters: [] })
    expect(s.sortedEntries[0].name).toBe('newer')
  })

  it('findByName looks up by exact name', async () => {
    const s = await fresh()
    s.save('matchme', { query: 'x', filters: [] })
    expect(s.findByName('matchme')).not.toBeNull()
    expect(s.findByName('miss')).toBeNull()
  })
})
