import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'

const mockTerminalDispose = vi.fn()
const mockTerminalWrite = vi.fn()
const mockTerminalOpen = vi.fn()
const mockTerminalFocus = vi.fn()
let mockOnDataCallback: ((data: string) => void) | null = null
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
    onData: vi.fn((cb: (data: string) => void) => { mockOnDataCallback = cb }),
    get rows() { return mockTerminalRows },
    get cols() { return mockTerminalColumns },
  })),
}))

vi.mock('xterm-addon-fit', () => ({
  FitAddon: vi.fn(() => ({ fit: mockFit })),
}))

const mockCreateSession = vi.fn()
const mockSendSessionInput = vi.fn()
const mockResizeSession = vi.fn()
const mockCloseSession = vi.fn()
const mockRefreshSessions = vi.fn()
const mockExplainOutput = vi.fn()
const mockGenerateCommand = vi.fn()
const mockCopilotClear = vi.fn()
const mockListPods = vi.fn()
const mockListContexts = vi.fn()

function mockDomainIcon(d: string) {
  const icons: Record<string, string> = { default: '>', k8s: '\u2388', kafka: 'K', cloud: '\u2601' }
  return icons[d] || '>'
}
function mockDomainLabel(d: string) {
  const labels: Record<string, string> = { default: 'Shell', k8s: 'K8s', kafka: 'Kafka', cloud: 'Cloud' }
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
    pods: [], loading: false, listPods: mockListPods,
  })),
  useContexts: vi.fn(() => ({
    contexts: [], listContexts: mockListContexts,
  })),
}))

let terminalOutputCallback: ((payload: any) => void) | null = null
vi.mock('../../lib/bus', () => ({
  bus: {
    useWailsEvent: vi.fn((eventName: string, callback: (payload: any) => void) => {
      if (eventName === 'terminal:output') terminalOutputCallback = callback
    }),
    on: vi.fn(), off: vi.fn(), emit: vi.fn(), onWails: vi.fn(), useEvent: vi.fn(),
  },
}))

let TerminalView: any

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
    // debounced — wait 200ms for the 100ms debounce + buffer
    await new Promise(r => setTimeout(r, 200))
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

  it('uses shared TERMINAL_FONT_FAMILY', async () => {
    const { TERMINAL_FONT_FAMILY } = await import('../../features/terminal/theme')
    expect(TERMINAL_FONT_FAMILY).toContain('ui-monospace')
    expect(TERMINAL_FONT_FAMILY).toContain('SF Mono')
  })
})
