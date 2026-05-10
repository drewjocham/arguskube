import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import DiagnosticsPanel from '../../components/diagnostics/DiagnosticsPanel.vue'

const mockHistory = ref([])
const mockSending = ref(false)
const mockAutoSummary = ref(null)
const mockEventLog = ref([])

const mockSendMessage = vi.fn(async () => 'reply')
const mockRefreshHistory = vi.fn(async () => {})
const mockFetchAutoSummary = vi.fn(async () => {})
const mockFetchEventLog = vi.fn(async () => {})

vi.mock('../../composables/useWails', () => ({
  useChat: vi.fn(() => ({
    history: mockHistory,
    sending: mockSending,
    autoSummary: mockAutoSummary,
    eventLog: mockEventLog,
    sendMessage: mockSendMessage,
    refreshHistory: mockRefreshHistory,
    fetchAutoSummary: mockFetchAutoSummary,
    fetchEventLog: mockFetchEventLog,
  })),
}))

function makeWrapper(props = {}) {
  return mount(DiagnosticsPanel, {
    props,
    global: {
      provide: {
        isAllowed: () => true,
        tier: 'pro',
      },
    },
  })
}

describe('DiagnosticsPanel.vue — Argus right rail', () => {
  beforeEach(() => {
    mockHistory.value = []
    mockSending.value = false
    mockAutoSummary.value = null
    mockEventLog.value = []
    mockSendMessage.mockClear()
    mockRefreshHistory.mockClear()
    mockFetchAutoSummary.mockClear()
    mockFetchEventLog.mockClear()
  })

  it('defaults to the chat tab and accepts input when no alert is selected', async () => {
    const wrapper = makeWrapper({ selectedAlert: null })
    await flushPromises()
    expect(wrapper.find('.panel-tab.active').text()).toContain('Chat')
    const ta = wrapper.find('textarea.ai-input')
    expect(ta.attributes('disabled')).toBeUndefined()
  })

  it('refreshes the global chat history on mount when no alert is selected', async () => {
    makeWrapper({ selectedAlert: null })
    await flushPromises()
    expect(mockRefreshHistory).toHaveBeenCalledWith('global')
  })

  it('switches to the diagnostics tab when an alert is selected', async () => {
    const wrapper = makeWrapper({ selectedAlert: null })
    await flushPromises()
    await wrapper.setProps({ selectedAlert: { id: 'a-1', name: 'Pod CrashLoop', severity: 'critical', namespace: 'prod', restartCount: 3 } })
    await flushPromises()
    expect(wrapper.find('.panel-tab.active').text()).toContain('Diagnostics')
    expect(mockRefreshHistory).toHaveBeenCalledWith('a-1')
    expect(mockFetchAutoSummary).toHaveBeenCalledWith('a-1')
  })

  it('sendMessage uses the "global" alert id when nothing is selected', async () => {
    const wrapper = makeWrapper({ selectedAlert: null })
    await flushPromises()
    await wrapper.find('textarea.ai-input').setValue('what is wrong with my cluster?')
    await wrapper.find('button.ai-send').trigger('click')
    await flushPromises()
    expect(mockSendMessage).toHaveBeenCalledWith('global', 'what is wrong with my cluster?')
  })

  it('sendMessage uses the alert id when one is selected', async () => {
    const wrapper = makeWrapper({ selectedAlert: { id: 'alert-7', name: 'OOM', severity: 'critical', namespace: 'prod', restartCount: 5 } })
    await flushPromises()
    // Click the chat tab so the input flow targets it.
    const chatTab = wrapper.findAll('.panel-tab').find(t => t.text().includes('Chat'))
    await chatTab.trigger('click')
    await wrapper.find('textarea.ai-input').setValue('summarize')
    await wrapper.find('button.ai-send').trigger('click')
    await flushPromises()
    expect(mockSendMessage).toHaveBeenCalledWith('alert-7', 'summarize')
  })

  it('renders code blocks in assistant messages', async () => {
    mockHistory.value = [
      {
        role: 'assistant',
        content: 'Try this:\n```bash\nkubectl get pods -A\n```\nthen verify.',
        timestamp: Date.now(),
      },
    ]
    const wrapper = makeWrapper({ selectedAlert: null })
    await flushPromises()
    // Focus the chat tab (default is chat now, but be explicit).
    const chatTab = wrapper.findAll('.panel-tab').find(t => t.text().includes('Chat'))
    await chatTab.trigger('click')
    await nextTick()

    expect(wrapper.find('.code-block').exists()).toBe(true)
    expect(wrapper.find('.code-content').text()).toBe('kubectl get pods -A')
    expect(wrapper.find('button.run-btn').exists()).toBe(true)
    expect(wrapper.find('button.copy-btn').exists()).toBe(true)
  })

  it('surfaces the send error when sendMessage rejects', async () => {
    mockSendMessage.mockRejectedValueOnce(new Error('AI agent not configured — set the DeepSeek API key'))
    const wrapper = makeWrapper({ selectedAlert: null })
    await flushPromises()
    await wrapper.find('textarea.ai-input').setValue('q')
    await wrapper.find('button.ai-send').trigger('click')
    await flushPromises()
    expect(wrapper.find('.chat-error').exists()).toBe(true)
    expect(wrapper.find('.chat-error').text()).toContain('DeepSeek API key')
  })

  it('disables Send while sending or when input is empty', async () => {
    const wrapper = makeWrapper({ selectedAlert: null })
    await flushPromises()
    const send = wrapper.find('button.ai-send')
    expect(send.attributes('disabled')).toBeDefined()

    await wrapper.find('textarea.ai-input').setValue('hello')
    expect(send.attributes('disabled')).toBeUndefined()

    mockSending.value = true
    await wrapper.vm.$nextTick()
    expect(send.attributes('disabled')).toBeDefined()
  })
})
