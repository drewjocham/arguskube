import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import ChatPopOut from '../../components/ai/ChatPopOut.vue'
import { useUIPrefsStore } from '../../stores/uiPrefs'

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

describe('ChatPopOut.vue', () => {
  beforeEach(() => {
    mockHistory.value = []
    mockSending.value = false
    mockSendMessage.mockClear()
    mockRefreshHistory.mockClear()
  })

  it('renders a fullscreen overlay with the Argus AI title', () => {
    const wrapper = mount(ChatPopOut)
    expect(wrapper.find('.chat-popout-overlay').exists()).toBe(true)
    expect(wrapper.find('.chat-popout-title').text()).toContain('Argus AI')
  })

  it('embeds the ArgusAIChat component', async () => {
    const wrapper = mount(ChatPopOut)
    await flushPromises()
    expect(wrapper.find('.argus-ai-view').exists()).toBe(true)
  })

  it('clicking the overlay outside the panel closes the popout', async () => {
    const ui = useUIPrefsStore()
    ui.openChatPopOut()
    expect(ui.chatPopOutOpen).toBe(true)

    const wrapper = mount(ChatPopOut)
    await wrapper.find('.chat-popout-overlay').trigger('click')
    expect(ui.chatPopOutOpen).toBe(false)
  })

  it('clicking the close button closes the popout', async () => {
    const ui = useUIPrefsStore()
    ui.openChatPopOut()

    const wrapper = mount(ChatPopOut)
    await wrapper.find('.chat-popout-close').trigger('click')
    expect(ui.chatPopOutOpen).toBe(false)
  })

  it('clicking inside the panel does NOT close the popout', async () => {
    const ui = useUIPrefsStore()
    ui.openChatPopOut()

    const wrapper = mount(ChatPopOut)
    await wrapper.find('.chat-popout-window').trigger('click')
    expect(ui.chatPopOutOpen).toBe(true)
  })

  it('Escape closes the popout', async () => {
    const ui = useUIPrefsStore()
    ui.openChatPopOut()

    mount(ChatPopOut, { attachTo: document.body })
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(ui.chatPopOutOpen).toBe(false)
  })
})
