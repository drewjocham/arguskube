import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSetupChecklistStore, __test } from '../setupChecklist'

describe('setupChecklist store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts empty and reports not-all-green for an empty list', () => {
    const s = useSetupChecklistStore()
    expect(s.items).toEqual([])
    // An empty list is NOT "all green" — we don't want the panel to claim
    // readiness before any probe has reported.
    expect(s.allGreen).toBe(false)
    expect(s.blockerCount).toBe(0)
  })

  it('upsert() inserts a normalized item with sane defaults', () => {
    const s = useSetupChecklistStore()
    s.upsert({ id: 'x', title: 'Probe X' })
    expect(s.items).toHaveLength(1)
    expect(s.items[0].status).toBe('todo')
    expect(s.items[0].priority).toBe(100)
    expect(s.items[0].action).toBeNull()
  })

  it('upsert() rejects unknown statuses and falls back to todo', () => {
    const s = useSetupChecklistStore()
    s.upsert({ id: 'x', title: 'X', status: 'banana' })
    expect(s.items[0].status).toBe('todo')
  })

  it('upsert() replaces an existing row in place', () => {
    const s = useSetupChecklistStore()
    s.upsert({ id: 'x', title: 'one', status: 'todo' })
    s.upsert({ id: 'x', title: 'two', status: 'ok' })
    expect(s.items).toHaveLength(1)
    expect(s.items[0].title).toBe('two')
    expect(s.items[0].status).toBe('ok')
  })

  it('sorts by status weight, then priority, then id', () => {
    const s = useSetupChecklistStore()
    s.upsert({ id: 'b-ok', title: 'b', status: 'ok' })
    s.upsert({ id: 'a-warn', title: 'a', status: 'warn', priority: 50 })
    s.upsert({ id: 'b-warn', title: 'b', status: 'warn', priority: 50 })
    s.upsert({ id: 'a-error', title: 'a', status: 'error' })
    s.upsert({ id: 'a-todo', title: 'a', status: 'todo', priority: 10 })
    expect(s.items.map(i => i.id)).toEqual([
      'a-error', 'a-todo', 'a-warn', 'b-warn', 'b-ok',
    ])
  })

  it('drops upserts without an id', () => {
    const s = useSetupChecklistStore()
    s.upsert({ title: 'no id' })
    s.upsert(null)
    s.upsert('not an object')
    expect(s.items).toEqual([])
  })

  it('action is kept when label + dispatch are both present', () => {
    const s = useSetupChecklistStore()
    const fn = vi.fn()
    s.upsert({
      id: 'x', title: 'X', status: 'todo',
      action: { label: 'Fix', dispatch: fn },
    })
    expect(s.items[0].action.label).toBe('Fix')
    expect(s.items[0].action.dispatch).toBe(fn)
  })

  it('action is dropped when label or dispatch is missing', () => {
    const s = useSetupChecklistStore()
    s.upsert({ id: 'x', title: 'X', action: { label: 'Fix' } })
    expect(s.items[0].action).toBeNull()
    s.upsert({ id: 'x', title: 'X', action: { dispatch: () => {} } })
    expect(s.items[0].action).toBeNull()
  })

  it('action accepts actionId-only form for parent-routed dispatch', () => {
    const s = useSetupChecklistStore()
    s.upsert({ id: 'x', title: 'X', action: { label: 'Apply', actionId: 'auto.apply-proxy' } })
    expect(s.items[0].action.label).toBe('Apply')
    expect(s.items[0].action.actionId).toBe('auto.apply-proxy')
    expect(s.items[0].action.dispatch).toBeNull()
  })

  it('remove() drops a row by id', () => {
    const s = useSetupChecklistStore()
    s.upsert({ id: 'x', title: 'X' })
    s.upsert({ id: 'y', title: 'Y' })
    s.remove('x')
    expect(s.items.map(i => i.id)).toEqual(['y'])
  })

  it('allGreen flips when every row is ok', () => {
    const s = useSetupChecklistStore()
    s.upsert({ id: 'a', title: 'A', status: 'ok' })
    s.upsert({ id: 'b', title: 'B', status: 'ok' })
    expect(s.allGreen).toBe(true)
    s.upsert({ id: 'c', title: 'C', status: 'warn' })
    expect(s.allGreen).toBe(false)
  })

  it('blockerCount counts error + todo only', () => {
    const s = useSetupChecklistStore()
    s.upsert({ id: 'a', title: 'A', status: 'error' })
    s.upsert({ id: 'b', title: 'B', status: 'todo' })
    s.upsert({ id: 'c', title: 'C', status: 'warn' })
    s.upsert({ id: 'd', title: 'D', status: 'ok' })
    expect(s.blockerCount).toBe(2)
    expect(s.warnCount).toBe(1)
  })

  it('exposes STATUS_WEIGHT for documentation', () => {
    expect(__test.STATUS_WEIGHT.error).toBeLessThan(__test.STATUS_WEIGHT.todo)
    expect(__test.STATUS_WEIGHT.todo).toBeLessThan(__test.STATUS_WEIGHT.warn)
    expect(__test.STATUS_WEIGHT.warn).toBeLessThan(__test.STATUS_WEIGHT.ok)
  })
})
