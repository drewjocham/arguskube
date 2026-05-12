import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import RevealableInput from '../../components/common/RevealableInput.vue'

function mountRI(props = {}) {
  return mount(RevealableInput, {
    props: { modelValue: '', ...props },
    attachTo: document.body,
  })
}

describe('RevealableInput.vue', () => {
  it('starts as a password input by default', () => {
    const w = mountRI()
    expect(w.find('input.ri-input').attributes('type')).toBe('password')
  })

  it('starts revealed when initiallyRevealed=true', () => {
    const w = mountRI({ initiallyRevealed: true })
    expect(w.find('input.ri-input').attributes('type')).toBe('text')
  })

  it('renders an eye toggle button with the right aria state when masked', () => {
    const w = mountRI()
    const btn = w.find('button.ri-toggle')
    expect(btn.exists()).toBe(true)
    expect(btn.attributes('aria-label')).toBe('Show value')
    expect(btn.attributes('aria-pressed')).toBe('false')
  })

  it('flips to text + emits reveal-change when the eye is clicked', async () => {
    const w = mountRI({ modelValue: 'secret' })
    const btn = w.find('button.ri-toggle')
    await btn.trigger('click')
    expect(w.find('input.ri-input').attributes('type')).toBe('text')
    expect(btn.attributes('aria-pressed')).toBe('true')
    expect(btn.attributes('aria-label')).toBe('Hide value')
    const events = w.emitted('reveal-change')
    expect(events).toBeTruthy()
    expect(events[0][0]).toBe(true)
  })

  it('toggles back to masked on a second click', async () => {
    const w = mountRI({ modelValue: 'secret' })
    const btn = w.find('button.ri-toggle')
    await btn.trigger('click')
    await btn.trigger('click')
    expect(w.find('input.ri-input').attributes('type')).toBe('password')
    const events = w.emitted('reveal-change')
    expect(events.length).toBe(2)
    expect(events[1][0]).toBe(false)
  })

  it('emits update:modelValue when the user types', async () => {
    const w = mountRI({ modelValue: '' })
    await w.find('input.ri-input').setValue('hunter2')
    const emitted = w.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    expect(emitted[emitted.length - 1][0]).toBe('hunter2')
  })

  it('forwards the modelValue prop into the input element', async () => {
    const w = mountRI({ modelValue: 'preset' })
    expect(w.find('input.ri-input').element.value).toBe('preset')
    await w.setProps({ modelValue: 'changed' })
    expect(w.find('input.ri-input').element.value).toBe('changed')
  })

  it('forwards id, placeholder, autocomplete, and aria-label', () => {
    const w = mountRI({
      id: 'pw',
      placeholder: 'Type here',
      autocomplete: 'current-password',
      ariaLabel: 'Account password',
    })
    const input = w.find('input.ri-input')
    expect(input.attributes('id')).toBe('pw')
    expect(input.attributes('placeholder')).toBe('Type here')
    expect(input.attributes('autocomplete')).toBe('current-password')
    expect(input.attributes('aria-label')).toBe('Account password')
  })

  it('applies inputClass onto the inner input for consumer styling', () => {
    const w = mountRI({ inputClass: 'input mono' })
    const input = w.find('input.ri-input')
    expect(input.classes()).toContain('input')
    expect(input.classes()).toContain('mono')
  })

  it('disables BOTH the input and the eye when disabled=true', async () => {
    const w = mountRI({ disabled: true })
    expect(w.find('input.ri-input').attributes('disabled')).toBeDefined()
    expect(w.find('button.ri-toggle').attributes('disabled')).toBeDefined()
    // Clicking the eye while disabled should not flip state.
    await w.find('button.ri-toggle').trigger('click')
    expect(w.find('input.ri-input').attributes('type')).toBe('password')
    expect(w.emitted('reveal-change')).toBeFalsy()
  })

  it('honours required + minlength attrs for form validation', () => {
    const w = mountRI({ required: true, minlength: 12 })
    const input = w.find('input.ri-input')
    expect(input.attributes('required')).toBeDefined()
    expect(input.attributes('minlength')).toBe('12')
  })

  it('omits minlength attribute when minlength=0', () => {
    const w = mountRI({ minlength: 0 })
    expect(w.find('input.ri-input').attributes('minlength')).toBeUndefined()
  })

  it('exposes revealed + toggleReveal for programmatic control', async () => {
    const w = mountRI()
    expect(w.vm.revealed).toBe(false)
    w.vm.toggleReveal()
    await nextTick()
    expect(w.vm.revealed).toBe(true)
  })

  it('eye button has a tooltip matching its state', async () => {
    const w = mountRI()
    expect(w.find('button.ri-toggle').attributes('title')).toBe('Show value')
    await w.find('button.ri-toggle').trigger('click')
    expect(w.find('button.ri-toggle').attributes('title')).toBe('Hide value')
  })

  it('button is type="button" so Enter inside a form does not submit', () => {
    const w = mountRI()
    expect(w.find('button.ri-toggle').attributes('type')).toBe('button')
  })

  it('renders the eye-open SVG when masked, eye-off SVG when revealed', async () => {
    const w = mountRI()
    // 2 path elements (eye + circle) when masked
    expect(w.find('button.ri-toggle svg circle').exists()).toBe(true)
    await w.find('button.ri-toggle').trigger('click')
    // 1 line element (the slash) appears when revealed
    expect(w.find('button.ri-toggle svg line').exists()).toBe(true)
  })

  it('marks the wrapper .disabled when disabled prop is true', () => {
    const w = mountRI({ disabled: true })
    expect(w.classes()).toContain('disabled')
  })
})
