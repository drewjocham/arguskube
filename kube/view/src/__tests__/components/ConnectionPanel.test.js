import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'

const mockCallGo = vi.fn()
const mockCachedCallGo = vi.fn()
vi.mock('../../composables/useBridge', () => ({
  callGo: (...a) => mockCallGo(...a),
  cachedCallGo: (...a) => mockCachedCallGo(...a),
  invalidateCache: vi.fn(),
  FAST_TTL: 5_000,
}))

import ConnectionPanel from '../../components/workspace/ConnectionPanel.vue'

beforeEach(() => {
  setActivePinia(createPinia())
  mockCallGo.mockReset()
  mockCachedCallGo.mockReset()
})

describe('ConnectionPanel.vue', () => {
  it('renders the empty state when no services are wired', async () => {
    mockCachedCallGo.mockResolvedValueOnce([])
    mockCallGo.mockResolvedValueOnce([])
    const w = mount(ConnectionPanel)
    await flushPromises()
    expect(w.text()).toContain('No workspace integrations are wired')
  })

  it('renders a tile with a Connect button when no connections exist', async () => {
    mockCachedCallGo.mockResolvedValueOnce(['slack'])
    mockCallGo.mockResolvedValueOnce([])
    const w = mount(ConnectionPanel)
    await flushPromises()
    expect(w.text()).toContain('Slack')
    expect(w.text()).toContain('Not connected')
    const connectBtn = w.find('.connect-btn')
    expect(connectBtn.exists()).toBe(true)
    expect(connectBtn.text()).toContain('Connect')
  })

  it('renders one row per connection with a Disconnect button', async () => {
    mockCachedCallGo.mockResolvedValueOnce(['slack'])
    mockCallGo.mockResolvedValueOnce([
      { id: 'a', service: 'slack', display_name: 'Acme HQ' },
      { id: 'b', service: 'slack', display_name: 'Beta Corp' },
    ])
    const w = mount(ConnectionPanel)
    await flushPromises()
    const conns = w.findAll('.conn')
    expect(conns.length).toBe(2)
    expect(w.text()).toContain('Acme HQ')
    expect(w.text()).toContain('Beta Corp')
    expect(w.text()).toContain('2 workspaces connected')
    const disconnectBtns = w.findAll('.disconnect-btn')
    expect(disconnectBtns.length).toBe(2)
  })
})
