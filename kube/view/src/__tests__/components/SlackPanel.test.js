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

import SlackPanel from '../../components/workspace/SlackPanel.vue'
import { useWorkspaceStore } from '../../stores/workspace'

beforeEach(() => {
  setActivePinia(createPinia())
  mockCallGo.mockReset()
  mockCachedCallGo.mockReset()
  mockInvalidate.mockReset()
})

describe('SlackPanel.vue', () => {
  it('renders empty state when no Slack connections', async () => {
    mockCachedCallGo.mockResolvedValueOnce([])   // ListWorkspaceServices
    mockCallGo.mockResolvedValueOnce([])         // ListWorkspaceConnections
    const w = mount(SlackPanel)
    await flushPromises()
    expect(w.text()).toContain('No Slack workspace connected')
    expect(w.find('.empty').exists()).toBe(true)
  })

  it('emits switch-tab when "Go to Connections" clicked', async () => {
    mockCachedCallGo.mockResolvedValueOnce([])
    mockCallGo.mockResolvedValueOnce([])
    const w = mount(SlackPanel)
    await flushPromises()
    await w.find('.empty button').trigger('click')
    expect(w.emitted('switch-tab')?.[0]).toEqual(['connections'])
  })

  it('populates channel dropdown from store cache', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useWorkspaceStore()
    store.connections = [
      { id: 'conn-a', service: 'slack', display_name: 'Acme' },
    ]
    // Mount calls ListWorkspaceServices then ListSlackChannels (cached).
    mockCachedCallGo
      .mockResolvedValueOnce([])  // services
      .mockResolvedValue([        // any subsequent ListSlackChannels
        { id: 'C1', name: 'general' },
        { id: 'C2', name: 'random' },
      ])
    mockCallGo.mockResolvedValue([])

    const w = mount(SlackPanel, { global: { plugins: [pinia] } })
    await flushPromises()
    // Open the channel select
    const selects = w.findAllComponents({ name: 'Select' })
    // First (and only, single-conn case) is the channel picker.
    const channelSelect = selects[0]
    expect(channelSelect.exists()).toBe(true)
    // Options should include both channel labels.
    const opts = channelSelect.props('options')
    expect(opts.map((o) => o.label)).toEqual(['#general', '#random'])
  })

  it('Send button is disabled when text is blank', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useWorkspaceStore()
    store.connections = [{ id: 'c', service: 'slack', display_name: 'Acme' }]
    store.slackChannels = { c: [{ id: 'C1', name: 'general' }] }
    mockCachedCallGo.mockResolvedValue([])
    mockCallGo.mockResolvedValue([])

    const w = mount(SlackPanel, { global: { plugins: [pinia] } })
    await flushPromises()
    const sendBtn = w.find('[data-testid="slack-send"]')
    expect(sendBtn.attributes('disabled')).toBeDefined()
  })

  it('marks char count as over when text exceeds 40k', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useWorkspaceStore()
    store.connections = [{ id: 'c', service: 'slack', display_name: 'Acme' }]
    mockCachedCallGo.mockResolvedValue([])
    mockCallGo.mockResolvedValue([])

    const w = mount(SlackPanel, { global: { plugins: [pinia] } })
    await flushPromises()
    const ta = w.find('textarea')
    await ta.setValue('x'.repeat(40001))
    expect(w.find('.char-count.over').exists()).toBe(true)
  })

  it('clicking Send calls store.sendSlackMessage with the right args', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const store = useWorkspaceStore()
    store.connections = [{ id: 'conn-a', service: 'slack', display_name: 'Acme' }]
    const spy = vi.spyOn(store, 'sendSlackMessage').mockResolvedValue(true)
    mockCachedCallGo
      .mockResolvedValueOnce([])  // services
      .mockResolvedValue([{ id: 'C1', name: 'general' }])
    mockCallGo.mockResolvedValue([])

    const w = mount(SlackPanel, { global: { plugins: [pinia] } })
    await flushPromises()
    // Seed component state: pick channel + type text.
    w.vm.selectedChannelID = 'C1'
    w.vm.messageText = 'hello world'
    await flushPromises()
    await w.find('[data-testid="slack-send"]').trigger('click')
    await flushPromises()
    expect(spy).toHaveBeenCalledWith('conn-a', 'C1', 'hello world')
  })
})
