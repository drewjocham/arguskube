import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useNotificationsStore, __test } from '../notifications'

const memory = {}
Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: (k) => (k in memory ? memory[k] : null),
    setItem: (k, v) => { memory[k] = String(v) },
    removeItem: (k) => { delete memory[k] },
    clear: () => { for (const k of Object.keys(memory)) delete memory[k] },
  },
  writable: true, configurable: true,
})

describe('notifications store', () => {
  beforeEach(() => {
    for (const k of Object.keys(memory)) delete memory[k]
    vi.resetModules()
    setActivePinia(createPinia())
  })

  async function load() {
    setActivePinia(createPinia())
    const mod = await import('../notifications.js')
    return mod.useNotificationsStore()
  }

  it('starts empty with the default cap', async () => {
    const s = await load()
    expect(s.items).toEqual([])
    expect(s.settings.maxItems).toBe(__test.DEFAULT_MAX)
    expect(s.unreadCount).toBe(0)
  })

  it('add() appends a notification with sane defaults and an id', async () => {
    const s = await load()
    const id = s.add({ title: 'Test' })
    expect(typeof id).toBe('string')
    expect(s.items).toHaveLength(1)
    expect(s.items[0].title).toBe('Test')
    expect(s.items[0].kind).toBe('info')
    expect(s.items[0].read).toBe(false)
    expect(typeof s.items[0].createdAt).toBe('string')
    expect(s.unreadCount).toBe(1)
  })

  it('persists items to localStorage and reloads on a fresh store', async () => {
    const a = await load()
    a.add({ title: 'persisted', body: 'survive me' })
    const b = await load()
    expect(b.items).toHaveLength(1)
    expect(b.items[0].title).toBe('persisted')
  })

  it('evicts the oldest entry once the cap is reached', async () => {
    const s = await load()
    s.setMaxItems(3)
    s.add({ title: 'first' })
    s.add({ title: 'second' })
    s.add({ title: 'third' })
    s.add({ title: 'fourth' })
    expect(s.items.map(n => n.title)).toEqual(['second', 'third', 'fourth'])
  })

  it('setMaxItems clamps invalid values', async () => {
    const s = await load()
    s.setMaxItems(0)
    expect(s.settings.maxItems).toBe(1)
    s.setMaxItems('not a number')
    // Stays at the previous valid value (1) — not reset to default since
    // we only re-default when not finite.
    expect(s.settings.maxItems).toBe(__test.DEFAULT_MAX)
    s.setMaxItems(__test.ABSOLUTE_MAX + 9999)
    expect(s.settings.maxItems).toBe(__test.ABSOLUTE_MAX)
  })

  it('lowering the cap trims existing items', async () => {
    const s = await load()
    s.add({ title: '1' })
    s.add({ title: '2' })
    s.add({ title: '3' })
    s.setMaxItems(2)
    expect(s.items.map(n => n.title)).toEqual(['2', '3'])
  })

  it('remove() drops the matching id', async () => {
    const s = await load()
    const id = s.add({ title: 'go away' })
    s.add({ title: 'stay' })
    s.remove(id)
    expect(s.items.map(n => n.title)).toEqual(['stay'])
  })

  it('clearAll() drops everything', async () => {
    const s = await load()
    s.add({ title: 'a' })
    s.add({ title: 'b' })
    s.clearAll()
    expect(s.items).toEqual([])
  })

  it('markRead and markAllRead update unreadCount', async () => {
    const s = await load()
    const a = s.add({ title: 'a' })
    s.add({ title: 'b' })
    expect(s.unreadCount).toBe(2)
    s.markRead(a)
    expect(s.unreadCount).toBe(1)
    s.markAllRead()
    expect(s.unreadCount).toBe(0)
  })

  it('sortedItems returns newest-first', async () => {
    const s = await load()
    s.add({ title: 'older' })
    // Force a different timestamp for the next entry.
    await new Promise(r => setTimeout(r, 5))
    s.add({ title: 'newer' })
    const titles = s.sortedItems.map(n => n.title)
    expect(titles[0]).toBe('newer')
  })

  it('togglePanel + open/close manage panelOpen state', async () => {
    const s = await load()
    expect(s.panelOpen).toBe(false)
    s.togglePanel()
    expect(s.panelOpen).toBe(true)
    s.closePanel()
    expect(s.panelOpen).toBe(false)
    s.openPanel()
    expect(s.panelOpen).toBe(true)
  })
})
