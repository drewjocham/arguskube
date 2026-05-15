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

import DocsPanel from '../../components/workspace/DocsPanel.vue'
import { useWorkspaceStore } from '../../stores/workspace'

beforeEach(() => {
  setActivePinia(createPinia())
  mockCallGo.mockReset()
  mockCachedCallGo.mockReset()
  mockInvalidate.mockReset()
})

describe('DocsPanel.vue', () => {
  it('renders empty state when no Google connections', async () => {
    mockCachedCallGo.mockResolvedValueOnce([])
    mockCallGo.mockResolvedValueOnce([])
    const w = mount(DocsPanel)
    await flushPromises()
    expect(w.text()).toContain('No Google account connected')
  })

  it('Create button disabled when title is blank', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useWorkspaceStore()
    store.connections = [{ id: 'g', service: 'google', email: 'me@x' }]
    mockCachedCallGo.mockResolvedValue([])
    mockCallGo.mockResolvedValue([])

    const w = mount(DocsPanel, { global: { plugins: [pinia] } })
    await flushPromises()
    const btn = w.find('[data-testid="docs-create"]')
    expect(btn.attributes('disabled')).toBeDefined()
  })
})
