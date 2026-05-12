import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'

// Mock xterm and xterm-addon-fit — TerminalView dynamically imports these.
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
  FitAddon: vi.fn(() => ({
    fit: mockFit,
  })),
}))

// Mock useTerminal from useShell.
const mockStartTerminal = vi.fn()
const mockSendInput = vi.fn()
const mockResizeTerminal = vi.fn()

vi.mock('../../composables/useWails', () => ({
  useTerminal: vi.fn(() => ({
    startTerminal: mockStartTerminal,
    sendInput: mockSendInput,
    resizeTerminal: mockResizeTerminal,
  })),
}))

// Mock useWailsEvent — need to capture callback for terminal:output.
let terminalOutputCallback = null
vi.mock('../../composables/useEvents', () => ({
  useWailsEvent: vi.fn((eventName, callback) => {
    if (eventName === 'terminal:output') {
      terminalOutputCallback = callback
    }
  }),
}))

let TerminalView

async function createWrapper(props = {}) {
  if (!TerminalView) {
    TerminalView = (await import('../../components/terminal/TerminalView.vue')).default
  }
  return mount(TerminalView, {
    props: {
      visible: false,
      ...props,
    },
    attachTo: document.body,
  })
}

describe('TerminalView.vue — Integration', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
    mockOnDataCallback = null
    terminalOutputCallback = null
    mockTerminalColumns = 80
    mockTerminalRows = 24
  })

  it('is hidden when visible prop is false', async () => {
    const wrapper = await createWrapper({ visible: false })
    const container = wrapper.find('.terminal-container')
    expect(container.exists()).toBe(true)
    // v-show toggles display: none.
    expect(container.attributes('style')).toContain('display: none')
  })

  it('is visible when visible prop is true', async () => {
    const wrapper = await createWrapper({ visible: true })
    const container = wrapper.find('.terminal-container')
    // v-show removes inline style when visible (no inline display:none)
    const style = container.attributes('style')
    expect(style === undefined || !style.includes('display: none')).toBe(true)
  })

  it('renders the terminal element ref', async () => {
    const wrapper = await createWrapper({ visible: true })
    const termEl = wrapper.find('.terminal-element')
    expect(termEl.exists()).toBe(true)
  })

  it('initializes terminal when becoming visible', async () => {
    const wrapper = await createWrapper({ visible: false })
    // No terminal yet.
    expect(mockStartTerminal).not.toHaveBeenCalled()

    // Now show it.
    await wrapper.setProps({ visible: true })
    await nextTick()
    // Dynamic imports take a tick via flushPromises.
    await flushPromises()
    await nextTick()

    // Terminal should have been created.
    expect(mockTerminalOpen).toHaveBeenCalled()
    // FitAddon.fit should have been called.
    expect(mockFit).toHaveBeenCalled()
  })

  it('calls startTerminal with correct rows and cols on first init', async () => {
    mockTerminalRows = 24
    mockTerminalColumns = 80
    const wrapper = await createWrapper({ visible: false })
    await wrapper.setProps({ visible: true })
    await nextTick()
    await flushPromises()
    await nextTick()

    expect(mockStartTerminal).toHaveBeenCalledWith(24, 80)
  })

  it('writes data to terminal when terminal:output event fires', async () => {
    const wrapper = await createWrapper({ visible: false })
    // Become visible to trigger initialization (component caches term, so run this first)
    await wrapper.setProps({ visible: true })
    await nextTick()
    await flushPromises()
    await nextTick()

    // Verify terminal was initialized
    expect(mockTerminalOpen).toHaveBeenCalled()

    // Simulate an output event.
    if (terminalOutputCallback) {
      terminalOutputCallback('hello from backend')
    }

    await flushPromises()
    expect(mockTerminalWrite).toHaveBeenCalledWith('hello from backend')
  })

  it('calls fitAddon.fit on resize', async () => {
    const wrapper = await createWrapper({ visible: true })
    await nextTick()
    await flushPromises()
    await nextTick()

    // Simulate resize.
    window.dispatchEvent(new Event('resize'))

    expect(mockFit).toHaveBeenCalled()
  })

  it('disposes terminal on unmount', async () => {
    const wrapper = await createWrapper({ visible: false })
    await wrapper.setProps({ visible: true })
    await nextTick()
    await flushPromises()
    await nextTick()

    expect(mockTerminalOpen).toHaveBeenCalled()

    wrapper.unmount()
    expect(mockTerminalDispose).toHaveBeenCalled()
  })
})
