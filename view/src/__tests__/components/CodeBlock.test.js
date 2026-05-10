import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import CodeBlock from '../../components/ai/CodeBlock.vue'
import { useTerminalDispatch } from '../../composables/useTerminalDispatch'

describe('CodeBlock.vue', () => {
  beforeEach(() => {
    // Drain shared dispatch state between tests.
    useTerminalDispatch().consumePendingCommand()
  })

  it('renders the code body and language label', () => {
    const wrapper = mount(CodeBlock, {
      props: { code: 'kubectl get pods', language: 'bash' },
    })
    expect(wrapper.find('code.code-content').text()).toBe('kubectl get pods')
    expect(wrapper.find('.code-lang').text()).toBe('bash')
  })

  it('falls back to "text" label when no language is provided', () => {
    const wrapper = mount(CodeBlock, { props: { code: 'plain text', language: '' } })
    expect(wrapper.find('.code-lang').text()).toBe('text')
  })

  it('shows Run button only for shell-ish languages', () => {
    const shell = mount(CodeBlock, { props: { code: 'ls', language: 'bash' } })
    expect(shell.find('button.run-btn').exists()).toBe(true)

    const yaml = mount(CodeBlock, { props: { code: 'a: 1', language: 'yaml' } })
    expect(yaml.find('button.run-btn').exists()).toBe(false)
  })

  it('hides Run button when allow-run is explicitly false', () => {
    const wrapper = mount(CodeBlock, {
      props: { code: 'ls', language: 'bash', allowRun: false },
    })
    expect(wrapper.find('button.run-btn').exists()).toBe(false)
  })

  it('clicking Copy writes to navigator.clipboard and toggles "Copied" feedback', async () => {
    const writeText = vi.fn().mockResolvedValue()
    Object.assign(navigator, { clipboard: { writeText } })

    const wrapper = mount(CodeBlock, { props: { code: 'kubectl get ns', language: 'bash' } })
    await wrapper.find('button.copy-btn').trigger('click')
    await flushPromises()

    expect(writeText).toHaveBeenCalledWith('kubectl get ns')
    expect(wrapper.find('button.copy-btn').text()).toContain('Copied')
  })

  it('clicking Run sends the command to the terminal dispatcher', async () => {
    const wrapper = mount(CodeBlock, { props: { code: 'kubectl top nodes', language: 'sh' } })
    const dispatch = useTerminalDispatch()
    const before = dispatch.requestOpen.value

    await wrapper.find('button.run-btn').trigger('click')

    expect(dispatch.pendingCommand.value).not.toBeNull()
    expect(dispatch.pendingCommand.value.text).toBe('kubectl top nodes')
    expect(dispatch.requestOpen.value).toBe(before + 1)
    expect(wrapper.find('button.run-btn').text()).toContain('Sent')
  })

  it('Run button does NOT append a newline (user must press Enter to execute)', async () => {
    const wrapper = mount(CodeBlock, { props: { code: 'rm -rf /', language: 'bash' } })
    const dispatch = useTerminalDispatch()
    await wrapper.find('button.run-btn').trigger('click')
    expect(dispatch.pendingCommand.value.text).toBe('rm -rf /')
    expect(dispatch.pendingCommand.value.text).not.toContain('\n')
  })

  it('Copy gracefully reports an error when clipboard is unavailable', async () => {
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockRejectedValue(new Error('blocked')) } })
    // Prevent the textarea fallback from succeeding either.
    const origExec = document.execCommand
    document.execCommand = vi.fn().mockReturnValue(false)

    const wrapper = mount(CodeBlock, { props: { code: 'cmd', language: 'bash' } })
    await wrapper.find('button.copy-btn').trigger('click')
    await flushPromises()
    expect(wrapper.find('.code-error').exists()).toBe(true)

    document.execCommand = origExec
  })
})
