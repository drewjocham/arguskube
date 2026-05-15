import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

const mockCallGo = vi.fn()
const mockCachedCallGo = vi.fn()
const mockInvalidate = vi.fn()
vi.mock('../../composables/useBridge', () => ({
  callGo: (...a) => mockCallGo(...a),
  cachedCallGo: (...a) => mockCachedCallGo(...a),
  invalidateCache: (...a) => mockInvalidate(...a),
  FAST_TTL: 5_000,
}))

import TasksPanel from '../../components/workspace/TasksPanel.vue'
import { useWorkspaceStore } from '../../stores/workspace'

beforeEach(() => {
  setActivePinia(createPinia())
  mockCallGo.mockReset()
  mockCachedCallGo.mockReset()
  mockInvalidate.mockReset()
})

async function mountWithSeed(seedTasks) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const store = useWorkspaceStore()
  store.connections = [{ id: 'g', service: 'google', email: 'me@x' }]
  store.taskLists = { g: [{ id: 'L1', title: 'My Tasks' }] }
  // Bridge mocks return the seed for any list call so the watcher-triggered
  // loadTasks doesn't replace our seeded state with [].
  mockCachedCallGo.mockImplementation((method) => {
    if (method === 'ListGoogleTasks') return Promise.resolve(seedTasks)
    if (method === 'ListGoogleTaskLists') return Promise.resolve([{ id: 'L1', title: 'My Tasks' }])
    return Promise.resolve([])
  })
  mockCallGo.mockResolvedValue([])
  const w = mount(TasksPanel, { global: { plugins: [pinia] } })
  await flushPromises()
  w.vm.listID = 'L1'
  await flushPromises()
  return { w, store }
}

describe('TasksPanel.vue', () => {
  it('Active filter hides completed tasks', async () => {
    const { w } = await mountWithSeed([
      { id: 'T1', title: 'open one',  status: 'needsAction' },
      { id: 'T2', title: 'done one',  status: 'completed' },
    ])
    // Default filter is 'active' per the panel.
    expect(w.text()).toContain('open one')
    expect(w.text()).not.toContain('done one')
    // Flip to All — both visible.
    await w.find('[data-testid="tasks-filter-all"]').trigger('click')
    expect(w.text()).toContain('done one')
  })

  it('toggling the checkbox calls updateTask with status flipped', async () => {
    const { w, store } = await mountWithSeed([
      { id: 'T1', title: 'x', status: 'needsAction' },
    ])
    const spy = vi.spyOn(store, 'updateTask').mockResolvedValue({
      id: 'T1', status: 'completed', title: 'x',
    })
    await w.find('[data-testid="tasks-toggle-T1"]').trigger('change')
    await flushPromises()
    expect(spy).toHaveBeenCalledWith('g', 'L1', 'T1', { status: 'completed' })
  })
})
