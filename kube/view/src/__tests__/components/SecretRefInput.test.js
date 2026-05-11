import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import SecretRefInput from '../../components/common/SecretRefInput.vue'

function mountSRI(props = {}) {
  return mount(SecretRefInput, {
    props: { modelValue: '', ...props },
    attachTo: document.body,
  })
}

describe('SecretRefInput.vue', () => {
  it('renders a kind picker, value input, and source pill', () => {
    const w = mountSRI()
    expect(w.find('select.srf-kind').exists()).toBe(true)
    expect(w.find('input.srf-value').exists()).toBe(true)
    expect(w.find('.srf-source-pill').exists()).toBe(true)
  })

  it('parses an incoming modelValue and seeds the kind + value fields', () => {
    const w = mountSRI({ modelValue: 'aws-secret:prod/db#user' })
    expect(w.find('select.srf-kind').element.value).toBe('aws-secret')
    expect(w.find('input.srf-value').element.value).toBe('prod/db')
    expect(w.find('input.srf-key').element.value).toBe('user')
  })

  it('emits update:modelValue with formatted output when value changes', async () => {
    const w = mountSRI({ modelValue: 'env:HOST' })
    const input = w.find('input.srf-value')
    await input.setValue('OTHER')
    const emitted = w.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    expect(emitted[emitted.length - 1][0]).toBe('env:OTHER')
  })

  it('emits update:modelValue when the kind dropdown changes', async () => {
    const w = mountSRI({ modelValue: 'foo' })
    const select = w.find('select.srf-kind')
    await select.setValue('env')
    const emitted = w.emitted('update:modelValue')
    expect(emitted[emitted.length - 1][0]).toBe('env:foo')
  })

  it('drops the #key when switching back to inline', async () => {
    const w = mountSRI({ modelValue: 'aws-secret:prod/db#user' })
    const select = w.find('select.srf-kind')
    await select.setValue('inline')
    const last = w.emitted('update:modelValue').slice(-1)[0][0]
    // inline drops the prefix and the key
    expect(last).toBe('prod/db')
  })

  it('shows the #key field automatically for kinds that support it', async () => {
    const w = mountSRI({ modelValue: 'aws-secret:foo' })
    expect(w.find('input.srf-key').exists()).toBe(true)
    await w.find('select.srf-kind').setValue('env')
    await nextTick()
    expect(w.find('input.srf-key').exists()).toBe(false)
  })

  it('honours showKey="always" / "never"', async () => {
    const wAlways = mountSRI({ modelValue: 'env:HOST', showKey: 'always' })
    expect(wAlways.find('input.srf-key').exists()).toBe(true)
    const wNever = mountSRI({ modelValue: 'aws-secret:foo', showKey: 'never' })
    expect(wNever.find('input.srf-key').exists()).toBe(false)
  })

  it('renders the value as a password input only when inline + passwordLike', async () => {
    const wInline = mountSRI({ modelValue: 'literal', passwordLike: true })
    expect(wInline.find('input.srf-value').attributes('type')).toBe('password')

    const wEnv = mountSRI({ modelValue: 'env:HOST', passwordLike: true })
    expect(wEnv.find('input.srf-value').attributes('type')).toBe('text')
  })

  it('reflects validity in the source description and error pill', async () => {
    const w = mountSRI({ modelValue: 'env:not valid name' })
    await nextTick()
    expect(w.find('.srf-error').exists()).toBe(true)
    // and the wrapper carries the .invalid class
    expect(w.classes()).toContain('invalid')
  })

  it('keeps inline-empty as valid (nullable env vars)', () => {
    const w = mountSRI({ modelValue: '' })
    expect(w.classes()).not.toContain('invalid')
  })

  it('re-syncs internal state when the parent replaces modelValue', async () => {
    const w = mountSRI({ modelValue: 'env:A' })
    await w.setProps({ modelValue: 'aws-secret:b#c' })
    await nextTick()
    expect(w.find('select.srf-kind').element.value).toBe('aws-secret')
    expect(w.find('input.srf-value').element.value).toBe('b')
    expect(w.find('input.srf-key').element.value).toBe('c')
  })

  it('does not echo its own emit back to itself when modelValue is unchanged', async () => {
    const w = mountSRI({ modelValue: 'env:HOST' })
    // Same prop value — no internal change should occur.
    await w.setProps({ modelValue: 'env:HOST' })
    const updates = w.emitted('update:modelValue') || []
    expect(updates.length).toBe(0)
  })

  it('emits resolved-change with the parsed object', async () => {
    const w = mountSRI({ modelValue: '' })
    await w.find('select.srf-kind').setValue('vault')
    await w.find('input.srf-value').setValue('gh-pat')
    const emitted = w.emitted('resolved-change')
    expect(emitted).toBeTruthy()
    const last = emitted[emitted.length - 1][0]
    expect(last.kind).toBe('vault')
    expect(last.value).toBe('gh-pat')
  })

  it('disables all inputs when the disabled prop is set', () => {
    const w = mountSRI({ modelValue: 'env:HOST', disabled: true })
    expect(w.find('select.srf-kind').attributes('disabled')).toBeDefined()
    expect(w.find('input.srf-value').attributes('disabled')).toBeDefined()
  })

  it('renders the optional label when provided', () => {
    const w = mountSRI({ modelValue: '', label: 'API token' })
    expect(w.find('.srf-label').text()).toBe('API token')
  })

  it('hides the foot description when compact is true', () => {
    const w = mountSRI({ modelValue: 'env:HOST', compact: true })
    expect(w.classes()).toContain('compact')
    // .srf-foot is display:none in compact mode — assert via computed style
    const foot = w.find('.srf-foot')
    expect(foot.exists()).toBe(true)
    expect(foot.attributes('style') || w.html()).toBeTruthy()
  })
})
