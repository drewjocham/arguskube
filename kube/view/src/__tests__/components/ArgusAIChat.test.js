import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import ArgusAIChat from '../../components/ai/ArgusAIChat.vue'

const mockHistory = ref([])
const mockSending = ref(false)
const mockSendMessage = vi.fn(async () => 'reply')
const mockRefreshHistory = vi.fn(async () => {})

vi.mock('../../composables/useWails', () => ({
  useChat: vi.fn(() => ({
    history: mockHistory,
    sending: mockSending,
    sendMessage: mockSendMessage,
    refreshHistory: mockRefreshHistory,
  })),
}))

function createWrapper() {
  return mount(ArgusAIChat)
}

describe('ArgusAIChat.vue', () => {
  beforeEach(() => {
    mockHistory.value = []
    mockSending.value = false
    mockSendMessage.mockClear()
    mockRefreshHistory.mockClear()
  })

  it('renders the header and empty-state suggestions when no history', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    expect(wrapper.text()).toContain('Argus AI')
    expect(wrapper.find('.empty-state').exists()).toBe(true)
    expect(wrapper.findAll('.suggestion-btn').length).toBeGreaterThan(0)
  })

  it('refreshes history with the global alert id on mount', async () => {
    createWrapper()
    await flushPromises()
    expect(mockRefreshHistory).toHaveBeenCalledWith('global')
  })

  it('hides system messages and shows user/assistant roles', async () => {
    mockHistory.value = [
      { role: 'system', content: 'system prompt', timestamp: Date.now() },
      { role: 'user', content: 'hello', timestamp: Date.now() },
      { role: 'assistant', content: 'world', timestamp: Date.now() },
    ]
    const wrapper = createWrapper()
    await flushPromises()
    expect(wrapper.text()).not.toContain('system prompt')
    expect(wrapper.text()).toContain('hello')
    expect(wrapper.text()).toContain('world')
    expect(wrapper.findAll('.message.user').length).toBe(1)
    expect(wrapper.findAll('.message.assistant').length).toBe(1)
  })

  it('clicking a suggestion fills the textarea but does not send', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    const firstSuggestion = wrapper.find('.suggestion-btn')
    await firstSuggestion.trigger('click')
    const ta = wrapper.find('textarea.composer-input')
    expect(ta.element.value.length).toBeGreaterThan(0)
    expect(mockSendMessage).not.toHaveBeenCalled()
  })

  it('clicking Send dispatches sendMessage with the typed question and clears the input', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    const ta = wrapper.find('textarea.composer-input')
    await ta.setValue('what is wrong?')
    await wrapper.find('.send-btn').trigger('click')
    await flushPromises()
    expect(mockSendMessage).toHaveBeenCalledWith('global', 'what is wrong?')
    expect(ta.element.value).toBe('')
  })

  it('Enter sends, Shift+Enter inserts newline', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    const ta = wrapper.find('textarea.composer-input')
    await ta.setValue('hi')
    await ta.trigger('keydown', { key: 'Enter', shiftKey: true })
    expect(mockSendMessage).not.toHaveBeenCalled()
    await ta.trigger('keydown', { key: 'Enter', shiftKey: false })
    await flushPromises()
    expect(mockSendMessage).toHaveBeenCalledWith('global', 'hi')
  })

  it('disables Send while sending or when input is empty', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    const sendBtn = wrapper.find('.send-btn')
    expect(sendBtn.attributes('disabled')).toBeDefined()

    await wrapper.find('textarea.composer-input').setValue('q')
    expect(sendBtn.attributes('disabled')).toBeUndefined()

    mockSending.value = true
    await wrapper.vm.$nextTick()
    expect(sendBtn.attributes('disabled')).toBeDefined()
  })

  it('shows the typing indicator when sending', async () => {
    mockHistory.value = [
      { role: 'user', content: 'q', timestamp: Date.now() },
    ]
    mockSending.value = true
    const wrapper = createWrapper()
    await flushPromises()
    expect(wrapper.find('.message.typing').exists()).toBe(true)
  })

  it('surfaces the error message when sendMessage rejects', async () => {
    mockSendMessage.mockRejectedValueOnce(new Error('AI agent not configured — set DEEPSEEK_API_KEY'))
    const wrapper = createWrapper()
    await flushPromises()
    await wrapper.find('textarea.composer-input').setValue('q')
    await wrapper.find('.send-btn').trigger('click')
    await flushPromises()
    expect(wrapper.find('.error-banner').exists()).toBe(true)
    expect(wrapper.find('.error-banner').text()).toContain('DEEPSEEK_API_KEY')
  })
})
