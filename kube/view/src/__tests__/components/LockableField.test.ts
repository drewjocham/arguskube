/**
 * LockableField — pins the security guarantee.
 *
 * The whole reason this component exists is so a saved credential
 * doesn't render to the DOM until the user explicitly asks for it.
 * The unit tests below assert that property directly: mount with a
 * sensitive value, inspect the rendered HTML, confirm the value is
 * absent. Without this test, a refactor could silently start using
 * an <input :value="modelValue"> for the locked branch and leak the
 * credential into devtools / screenshots.
 */
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LockableField from '../../components/common/LockableField.vue'

const SECRET = 'sk-this-must-not-leak-to-the-dom'

describe('LockableField — locked branch', () => {
  it('starts locked when modelValue is non-empty', () => {
    const w = mount(LockableField, { props: { modelValue: SECRET } })
    expect(w.find('.lf-locked').exists()).toBe(true)
    expect(w.find('input[type="text"].lf-input').exists()).toBe(false)
  })

  it('does NOT render the secret value anywhere in the DOM when locked', () => {
    const w = mount(LockableField, { props: { modelValue: SECRET } })
    const html = w.html()
    expect(html).not.toContain(SECRET)
    // Sanity: the placeholder bullets DO render.
    expect(w.find('.lf-locked').text()).toMatch(/^•+$/)
  })

  it('does not put the secret on any HTMLInputElement.value', () => {
    const w = mount(LockableField, { props: { modelValue: SECRET } })
    // Only the unlock CHECKBOX should be in the DOM; no text input.
    const inputs = w.findAll('input')
    const textOrPassword = inputs.filter(i => {
      const t = (i.element as HTMLInputElement).type
      return t === 'text' || t === 'password'
    })
    expect(textOrPassword).toHaveLength(0)
    // The checkbox's value attribute is harmless ("on") but assert
    // none of the inputs carry the secret regardless.
    for (const i of inputs) {
      expect((i.element as HTMLInputElement).value).not.toBe(SECRET)
    }
  })
})

describe('LockableField — unlocked branch', () => {
  it('starts unlocked when modelValue is empty (first-time setup)', () => {
    const w = mount(LockableField, { props: { modelValue: '' } })
    expect(w.find('.lf-locked').exists()).toBe(false)
    expect(w.find('input[type="text"].lf-input').exists()).toBe(true)
  })

  it('renders the value once the user unlocks', async () => {
    const w = mount(LockableField, { props: { modelValue: SECRET } })
    const unlockBox = w.find('.lf-unlock input[type="checkbox"]')
    await unlockBox.setValue(true)
    expect(w.find('.lf-locked').exists()).toBe(false)
    const input = w.find<HTMLInputElement>('input[type="text"].lf-input')
    expect(input.exists()).toBe(true)
    expect(input.element.value).toBe(SECRET)
  })

  it('emits update:modelValue on typing', async () => {
    const w = mount(LockableField, { props: { modelValue: '' } })
    const input = w.find('input[type="text"].lf-input')
    await input.setValue('new-value')
    const events = w.emitted('update:modelValue') as unknown[][]
    expect(events).toBeTruthy()
    expect(events[events.length - 1]).toEqual(['new-value'])
  })

  it('preserves modelValue on re-lock (does NOT clear unsaved edits)', async () => {
    // Earlier draft cleared the value on re-lock, which erased
    // unsaved edits. The header comment in LockableField calls out
    // the policy reversal — this test pins it.
    const w = mount(LockableField, { props: { modelValue: SECRET } })
    const unlockBox = w.find('.lf-unlock input[type="checkbox"]')
    await unlockBox.setValue(true)
    // Re-lock.
    await unlockBox.setValue(false)
    // No update:modelValue with empty string should have fired.
    const events = (w.emitted('update:modelValue') as unknown[][]) || []
    for (const ev of events) {
      expect(ev[0]).not.toBe('')
    }
  })

  it('emits lock-change with the new state', async () => {
    const w = mount(LockableField, { props: { modelValue: SECRET } })
    const unlockBox = w.find('.lf-unlock input[type="checkbox"]')
    await unlockBox.setValue(true)
    await unlockBox.setValue(false)
    const events = w.emitted('lock-change') as unknown[][]
    expect(events).toEqual([[false], [true]])
  })
})

describe('LockableField — disabled', () => {
  it('does not toggle when disabled', async () => {
    const w = mount(LockableField, {
      props: { modelValue: SECRET, disabled: true },
    })
    const unlockBox = w.find('.lf-unlock input[type="checkbox"]')
    await unlockBox.setValue(true)
    // Component should still be locked.
    expect(w.find('.lf-locked').exists()).toBe(true)
  })
})
