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

import GChatPanel from '../../components/workspace/GChatPanel.vue'
import { useWorkspaceStore } from '../../stores/workspace'

beforeEach(() => {
  setActivePinia(createPinia())
  mockCallGo.mockReset()
  mockCachedCallGo.mockReset()
  mockInvalidate.mockReset()
})

describe('GChatPanel.vue', () => {
  it('renders empty state when no Google connection exists', async () => {
    const w = mount(GChatPanel)
    await flushPromises()
    // GoogleAccountHeader handles the empty state — the panel just hosts it
    // and shouldn't render a composer until a connection exists.
    expect(w.find('[data-testid="gchat-composer"]').exists()).toBe(false)
  })

  it('disables Send when text is blank', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useWorkspaceStore()
    store.connections = [
      { id: 'g1', service: 'google', display_name: 'Acme', email: 'a@b' },
    ]
    mockCachedCallGo.mockResolvedValue([{ id: 'spaces/A', name: 'Eng' }])

    const w = mount(GChatPanel, { global: { plugins: [pinia] } })
    await flushPromises()
    const sendBtn = w.find('[data-testid="gchat-send"]')
    expect(sendBtn.attributes('disabled')).toBeDefined()
  })

  it('flags reconnect when send error mentions permission denied', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useWorkspaceStore()
    store.connections = [
      { id: 'g1', service: 'google', display_name: 'Acme', email: 'a@b' },
    ]
    store.gchatSendError = 'gchat: messages.create 403: permission denied'
    mockCachedCallGo.mockResolvedValue([])

    const w = mount(GChatPanel, { global: { plugins: [pinia] } })
    await flushPromises()
    expect(w.find('[data-testid="gchat-reconnect-hint"]').exists()).toBe(true)
  })
})
