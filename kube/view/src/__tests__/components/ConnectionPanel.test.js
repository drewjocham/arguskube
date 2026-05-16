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
    expect(w.text()).toContain('No workspace integrations are available')
  })

  it('empty state surfaces a button that navigates to the sign-in-integrations Settings anchor', async () => {
    mockCachedCallGo.mockResolvedValueOnce([])
    mockCallGo.mockResolvedValueOnce([])
    const w = mount(ConnectionPanel)
    await flushPromises()
    const btn = w.find('.open-settings-btn')
    expect(btn.exists()).toBe(true)
    expect(btn.text()).toMatch(/sign-in/i)
    // Click queues a nav request through the appNav store; lazily
    // import after the panel mounts so we observe the same store
    // Pinia hands to the component.
    const { useAppNavStore } = await import('../../stores/appNav')
    const appNav = useAppNavStore()
    await btn.trigger('click')
    expect(appNav.pending).toEqual({ navId: 'settings', anchor: 'sign-in-integrations' })
    // No env-var hint anymore.
    expect(w.text()).not.toContain('ARGUS_SLACK_CLIENT_ID')
  })

  it('renders a tile with a Connect button when no connections exist', async () => {
    mockCachedCallGo.mockResolvedValueOnce(['gdocs'])
    mockCallGo.mockResolvedValueOnce([])
    const w = mount(ConnectionPanel)
    await flushPromises()
    expect(w.text()).toContain('Google Docs')
    expect(w.text()).toContain('Not connected')
    const connectBtn = w.find('.connect-btn')
    expect(connectBtn.exists()).toBe(true)
    expect(connectBtn.text()).toContain('Connect')
  })

  it('renders Slack and Google Workspace tiles when the backend reports them', async () => {
    mockCachedCallGo.mockResolvedValueOnce(['slack', 'google', 'gchat'])
    mockCallGo.mockResolvedValueOnce([])
    const w = mount(ConnectionPanel)
    await flushPromises()
    expect(w.text()).toContain('Slack')
    expect(w.text()).toContain('Google Workspace')
    expect(w.text()).toContain('Google Chat')
    // One Connect button per service tile.
    expect(w.findAll('.connect-btn').length).toBe(3)
  })

  it('renders one row per connection with a Disconnect button', async () => {
    mockCachedCallGo.mockResolvedValueOnce(['gdocs'])
    mockCallGo.mockResolvedValueOnce([
      { id: 'a', service: 'gdocs', display_name: 'Acme HQ' },
      { id: 'b', service: 'gdocs', display_name: 'Beta Corp' },
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
