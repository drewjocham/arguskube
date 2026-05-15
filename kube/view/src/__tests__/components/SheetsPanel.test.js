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

import SheetsPanel from '../../components/workspace/SheetsPanel.vue'
import { useWorkspaceStore } from '../../stores/workspace'

beforeEach(() => {
  setActivePinia(createPinia())
  mockCallGo.mockReset()
  mockCachedCallGo.mockReset()
  mockInvalidate.mockReset()
})

describe('SheetsPanel.vue', () => {
  it('rejects garbage range and disables Read', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useWorkspaceStore()
    store.connections = [{ id: 'g', service: 'google', email: 'me@x' }]
    mockCachedCallGo.mockResolvedValue([])
    mockCallGo.mockResolvedValue([])

    const w = mount(SheetsPanel, { global: { plugins: [pinia] } })
    await flushPromises()
    // Seed a sheet ID so the only thing breaking Read is the range.
    w.vm.sheetID = 'sheet-123'
    w.vm.range = 'not a range!!'
    await flushPromises()
    const readBtn = w.find('[data-testid="sheets-read"]')
    expect(readBtn.attributes('disabled')).toBeDefined()
  })

  it('accepts Sheet1!A1:C10 and calls store.writeSheetRange with the matrix', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useWorkspaceStore()
    store.connections = [{ id: 'g', service: 'google', email: 'me@x' }]
    mockCachedCallGo.mockResolvedValue([])
    mockCallGo.mockResolvedValue([])
    const writeSpy = vi.spyOn(store, 'writeSheetRange').mockResolvedValue(undefined)

    const w = mount(SheetsPanel, { global: { plugins: [pinia] } })
    await flushPromises()
    w.vm.sheetID = 'sheet-123'
    w.vm.range = 'Sheet1!A1:C10'
    w.vm.matrix = [['a', 'b'], ['c', 'd']]
    await flushPromises()
    await w.find('[data-testid="sheets-write"]').trigger('click')
    await flushPromises()
    expect(writeSpy).toHaveBeenCalledWith('g', 'sheet-123', 'Sheet1!A1:C10', [['a', 'b'], ['c', 'd']])
  })
})
