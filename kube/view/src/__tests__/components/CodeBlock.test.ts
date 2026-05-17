import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import CodeBlock from '../../components/ai/CodeBlock.vue'
import { useTerminalDispatchStore } from '../../stores/terminalDispatch'

describe('CodeBlock.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
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

  it('clicking Run dispatches a command to the terminal dispatch store', async () => {
    const dispatch = useTerminalDispatchStore()
    const wrapper = mount(CodeBlock, {
      props: { code: 'kubectl get pods', language: 'bash' },
    })
    await wrapper.find('button.run-btn').trigger('click')
    expect(dispatch.pendingCommand).not.toBeNull()
    expect(dispatch.pendingCommand!.text).toBe('kubectl get pods')
  })

  it('Run button is hidden when allowRun is false', () => {
    const wrapper = mount(CodeBlock, {
      props: { code: 'ls', language: 'bash', allowRun: false },
    })
    expect(wrapper.find('button.run-btn').exists()).toBe(false)
  })

  it('Run button is hidden for non-shell languages', () => {
    const wrapper = mount(CodeBlock, {
      props: { code: 'name: foo', language: 'yaml' },
    })
    expect(wrapper.find('button.run-btn').exists()).toBe(false)
  })

  it('copy button copies code to clipboard', async () => {
    const writeText = vi.fn()
    Object.assign(navigator, { clipboard: { writeText } })
    const wrapper = mount(CodeBlock, { props: { code: 'kubectl get pods', language: 'bash' } })
    const copyBtn = wrapper.findAll('button').find((b) => b.text().includes('Copy'))
    await copyBtn?.trigger('click')
    expect(writeText).toHaveBeenCalledWith('kubectl get pods')
  })

  it('copy button shows "Copied" feedback temporarily', async () => {
    vi.useFakeTimers()
    const wrapper = mount(CodeBlock, { props: { code: 'ls', language: 'bash' } })
    const copyBtn = wrapper.findAll('button').find((b) => b.text().includes('Copy'))
    await copyBtn?.trigger('click')
    expect(wrapper.text()).toContain('Copied')
    vi.advanceTimersByTime(2000)
    await flushPromises()
    expect(wrapper.text()).not.toContain('Copied')
    vi.useRealTimers()
  })
})
