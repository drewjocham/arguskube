import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'

// Mock xterm and xterm-addon-fit.
const mockTerminalDispose = vi.fn()
const mockTerminalWrite = vi.fn()
const mockTerminalOpen = vi.fn()
const mockTerminalFocus = vi.fn()
let mockOnDataCallback = null
let mockTerminalColumns = 80
let mockTerminalRows = 24
const mockFit = vi.fn()

vi.mock('xterm', () => ({
  Terminal: vi.fn(() => ({
    loadAddon: vi.fn(),
    open: mockTerminalOpen,
    write: mockTerminalWrite,
    focus: mockTerminalFocus,
    dispose: mockTerminalDispose,
    onData: vi.fn((cb) => { mockOnDataCallback = cb }),
    get rows() { return mockTerminalRows },
    get cols() { return mockTerminalColumns },
  })),
}))

vi.mock('xterm-addon-fit', () => ({
  FitAddon: vi.fn(() => ({ fit: mockFit })),
}))

// Mocks for composables.
const mockCreateSession = vi.fn()
const mockSendSessionInput = vi.fn()
const mockResizeSession = vi.fn()
const mockCloseSession = vi.fn()
const mockRefreshSessions = vi.fn()
const mockExplainOutput = vi.fn()
const mockGenerateCommand = vi.fn()
const mockCopilotClear = vi.fn()
const mockFetchPods = vi.fn()
const mockFetchContexts = vi.fn()

function mockDomainIcon(d) {
  const icons = { default: '>', k8s: '\u2388', kafka: 'K', cloud: '\u2601' }
  return icons[d] || '>'
}
function mockDomainLabel(d) {
  const labels = { default: 'Shell', k8s: 'K8s', kafka: 'Kafka', cloud: 'Cloud' }
  return labels[d] || d
}

vi.mock('../../composables/useWails', () => ({
  useTerminal: vi.fn(() => ({
    startTerminal: vi.fn(), sendInput: vi.fn(), resizeTerminal: vi.fn(),
  })),
  useTerminalSession: vi.fn(() => ({
    sessions: [],
    domains: [
      { id: 'default', label: 'Shell', icon: '>' },
      { id: 'k8s', label: 'K8s', icon: '\u2388' },
      { id: 'kafka', label: 'Kafka', icon: 'K' },
      { id: 'cloud', label: 'Cloud', icon: '\u2601' },
    ],
    createSession: mockCreateSession,
    sendSessionInput: mockSendSessionInput,
    resizeSession: mockResizeSession,
    closeSession: mockCloseSession,
    refreshSessions: mockRefreshSessions,
    domainLabel: mockDomainLabel,
    domainIcon: mockDomainIcon,
  })),
  useTerminalCopilot: vi.fn(() => ({
    loading: { value: false }, result: { value: null }, error: { value: null },
    explainOutput: mockExplainOutput, generateCommand: mockGenerateCommand, clear: mockCopilotClear,
  })),
  usePods: vi.fn(() => ({
    pods: [], loading: false, fetchPods: mockFetchPods,
  })),
  useContexts: vi.fn(() => ({
    contexts: [], fetchContexts: mockFetchContexts,
  })),
}))

// Mock bus.
let terminalOutputCallback = null
vi.mock('../../lib/bus', () => ({
  bus: {
    useWailsEvent: vi.fn((eventName, callback) => {
      if (eventName === 'terminal:output') terminalOutputCallback = callback
    }),
    on: vi.fn(), off: vi.fn(), emit: vi.fn(), onWails: vi.fn(), useEvent: vi.fn(),
  },
}))

let TerminalView

async function createWrapper(props = {}) {
  if (!TerminalView) TerminalView = (await import('../../components/terminal/TerminalView.vue')).default
  return mount(TerminalView, { props: { visible: false, ...props }, attachTo: document.body })
}

describe('TerminalView.vue — Integration', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
    mockOnDataCallback = null; terminalOutputCallback = null
    mockTerminalColumns = 80; mockTerminalRows = 24
  })

  it('is hidden when visible prop is false', async () => {
    const wrapper = await createWrapper({ visible: false })
    expect(wrapper.find('.terminal-container').attributes('style')).toContain('display: none')
  })

  it('is visible when visible prop is true', async () => {
    const wrapper = await createWrapper({ visible: true })
    const style = wrapper.find('.terminal-container').attributes('style')
    expect(style === undefined || !style.includes('display: none')).toBe(true)
  })

  it('renders the terminal element', async () => {
    const wrapper = await createWrapper({ visible: true })
    expect(wrapper.find('.terminal-element').exists()).toBe(true)
  })

  it('calls createSession on first init', async () => {
    mockTerminalRows = 24; mockTerminalColumns = 80
    const wrapper = await createWrapper({ visible: false })
    await wrapper.setProps({ visible: true })
    await nextTick(); await flushPromises(); await nextTick()

    expect(mockTerminalOpen).toHaveBeenCalled()
    expect(mockFit).toHaveBeenCalled()
    expect(mockCreateSession).toHaveBeenCalledWith('default', 'default', 'Shell', 24, 80)
  })

  it('writes data to terminal on terminal:output event', async () => {
    const wrapper = await createWrapper({ visible: false })
    await wrapper.setProps({ visible: true })
    await nextTick(); await flushPromises(); await nextTick()

    expect(mockTerminalOpen).toHaveBeenCalled()
    if (terminalOutputCallback) terminalOutputCallback({ sessionId: 'default', data: 'hello' })
    await flushPromises()
    expect(mockTerminalWrite).toHaveBeenCalledWith('hello')
  })

  it('handles bare string terminal:output backward compat', async () => {
    const wrapper = await createWrapper({ visible: true })
    await nextTick(); await flushPromises(); await nextTick()
    if (terminalOutputCallback) terminalOutputCallback('raw output')
    await flushPromises()
    expect(mockTerminalWrite).toHaveBeenCalledWith('raw output')
  })

  it('calls fitAddon.fit on resize', async () => {
    const wrapper = await createWrapper({ visible: true })
    await nextTick(); await flushPromises(); await nextTick()
    window.dispatchEvent(new Event('resize'))
    expect(mockFit).toHaveBeenCalled()
  })

  it('disposes terminal on unmount', async () => {
    const wrapper = await createWrapper({ visible: false })
    await wrapper.setProps({ visible: true })
    await nextTick(); await flushPromises(); await nextTick()
    expect(mockTerminalOpen).toHaveBeenCalled()
    wrapper.unmount()
    expect(mockTerminalDispose).toHaveBeenCalled()
  })

  it('initializes terminal when mounted with visible=true', async () => {
    const wrapper = await createWrapper({ visible: true })
    await nextTick(); await flushPromises(); await nextTick()
    expect(mockTerminalOpen).toHaveBeenCalled()
    expect(mockCreateSession).toHaveBeenCalled()
  })

  it('flushes a queued command after mount-with-visible init', async () => {
    const { useTerminalDispatchStore } = await import('../../stores/terminalDispatch')
    const dispatch = useTerminalDispatchStore()
    dispatch.sendToTerminal('kubectl get pods')

    const wrapper = await createWrapper({ visible: true })
    await nextTick(); await flushPromises(); await nextTick(); await flushPromises()

    expect(mockSendSessionInput).toHaveBeenCalledWith('default', 'kubectl get pods')
    expect(dispatch.pendingCommand).toBeNull()
  })

  it('renders tab bar', async () => {
    const wrapper = await createWrapper({ visible: true })
    await nextTick()
    expect(wrapper.find('.terminal-tabs').exists()).toBe(true)
  })
})
