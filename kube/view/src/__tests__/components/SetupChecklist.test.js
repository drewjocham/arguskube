import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import SetupChecklist from '../../components/setup/SetupChecklist.vue'
import ChecklistRow from '../../components/setup/ChecklistRow.vue'
import { useSetupChecklistStore } from '../../stores/setupChecklist'

describe('SetupChecklist + ChecklistRow', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders the empty hint when no probes have reported yet', () => {
    const wrapper = mount(SetupChecklist)
    expect(wrapper.text()).toContain('checking your setup')
    expect(wrapper.text()).toContain('Get Argus ready')
  })

  it('renders one row per checklist item', () => {
    const store = useSetupChecklistStore()
    store.upsert({ id: 'a', title: 'Probe A', status: 'todo' })
    store.upsert({ id: 'b', title: 'Probe B', status: 'ok' })
    const wrapper = mount(SetupChecklist)
    const rows = wrapper.findAllComponents(ChecklistRow)
    expect(rows).toHaveLength(2)
    expect(wrapper.text()).toContain('Probe A')
    expect(wrapper.text()).toContain('Probe B')
  })

  it('sorts blockers first', () => {
    const store = useSetupChecklistStore()
    store.upsert({ id: 'ok', title: 'OK', status: 'ok' })
    store.upsert({ id: 'todo', title: 'TODO', status: 'todo' })
    store.upsert({ id: 'err', title: 'ERR', status: 'error' })
    const wrapper = mount(SetupChecklist)
    const titles = wrapper.findAllComponents(ChecklistRow).map(r => r.props('item').id)
    expect(titles).toEqual(['err', 'todo', 'ok'])
  })

  it('collapses to "Argus is ready" when all items are ok', () => {
    const store = useSetupChecklistStore()
    store.upsert({ id: 'a', title: 'A', status: 'ok' })
    store.upsert({ id: 'b', title: 'B', status: 'ok' })
    const wrapper = mount(SetupChecklist)
    expect(wrapper.find('[data-testid="checklist-summary"]').text()).toBe('Argus is ready')
  })

  it('shows the blocker count when there are todos', () => {
    const store = useSetupChecklistStore()
    store.upsert({ id: 'a', title: 'A', status: 'todo' })
    store.upsert({ id: 'b', title: 'B', status: 'todo' })
    store.upsert({ id: 'c', title: 'C', status: 'ok' })
    const wrapper = mount(SetupChecklist)
    expect(wrapper.find('[data-testid="checklist-summary"]').text()).toContain('2 setup items')
  })

  it('row click invokes the item dispatch and toggles busy state', async () => {
    const store = useSetupChecklistStore()
    let release
    const dispatch = vi.fn(() => new Promise((r) => { release = r }))
    store.upsert({
      id: 'fix-me', title: 'Fix me', status: 'todo',
      action: { label: 'Fix', dispatch },
    })
    const wrapper = mount(SetupChecklist)
    const btn = wrapper.find('[data-testid="checklist-action-fix-me"]')
    await btn.trigger('click')
    expect(dispatch).toHaveBeenCalledOnce()
    expect(btn.text()).toBe('Working…')
    expect(btn.attributes('disabled')).toBeDefined()
    release()
    await flushPromises()
    expect(btn.text()).toBe('Fix')
  })

  it('does not render the action button for ok rows', () => {
    const store = useSetupChecklistStore()
    store.upsert({
      id: 'done', title: 'Done', status: 'ok',
      action: { label: 'Re-run', dispatch: () => {} },
    })
    const wrapper = mount(SetupChecklist)
    expect(wrapper.find('[data-testid="checklist-action-done"]').exists()).toBe(false)
  })

  it('collapse toggle hides the rows but keeps the header', async () => {
    const store = useSetupChecklistStore()
    store.upsert({ id: 'a', title: 'A', status: 'todo' })
    const wrapper = mount(SetupChecklist)
    expect(wrapper.find('[data-testid="checklist-rows"]').exists()).toBe(true)
    await wrapper.find('.toggle').trigger('click')
    expect(wrapper.find('[data-testid="checklist-rows"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('Get Argus ready')
  })

  it('rows carry the data-status attribute for color theming', () => {
    const store = useSetupChecklistStore()
    store.upsert({ id: 'e', title: 'Err', status: 'error' })
    store.upsert({ id: 't', title: 'Todo', status: 'todo' })
    const wrapper = mount(SetupChecklist)
    const errRow = wrapper.find('[data-testid="checklist-row-e"]')
    const todoRow = wrapper.find('[data-testid="checklist-row-t"]')
    expect(errRow.attributes('data-status')).toBe('error')
    expect(todoRow.attributes('data-status')).toBe('todo')
  })
})
