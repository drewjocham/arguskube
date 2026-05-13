import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import Select from '../Select.vue'

function mk(props = {}) {
  return mount(Select, {
    props: { options: ['a', 'b', 'c'], modelValue: 'a', ...props },
    attachTo: document.body,
  })
}

describe('Select.vue', () => {
  it('renders the current value', () => {
    const w = mk({ modelValue: 'b' })
    expect(w.find('.value').text()).toBe('b')
  })

  it('falls back to placeholder when value is null', () => {
    const w = mk({ modelValue: null, placeholder: 'Pick one' })
    expect(w.find('.value').classes()).toContain('placeholder')
    expect(w.find('.value').text()).toBe('Pick one')
  })

  it('opens on click and shows all options', async () => {
    const w = mk()
    expect(w.find('.panel').exists()).toBe(false)
    await w.find('.trigger').trigger('click')
    const opts = w.findAll('.option')
    expect(opts.length).toBe(3)
    expect(opts.map((o) => o.text())).toEqual(['a', 'b', 'c'])
  })

  it('emits update:modelValue + change on option pick', async () => {
    const w = mk()
    await w.find('.trigger').trigger('click')
    await w.findAll('.option')[2].trigger('mousedown')
    expect(w.emitted('update:modelValue')[0]).toEqual(['c'])
    expect(w.emitted('change')[0]).toEqual(['c'])
  })

  it('accepts {value,label} option objects', async () => {
    const w = mk({
      options: [{ value: 1, label: 'One' }, { value: 2, label: 'Two' }],
      modelValue: 2,
    })
    expect(w.find('.value').text()).toBe('Two')
    await w.find('.trigger').trigger('click')
    expect(w.findAll('.option').map((o) => o.text())).toEqual(['One', 'Two'])
  })

  it('ignores clicks on disabled options', async () => {
    const w = mk({
      options: [{ value: 'a', label: 'A' }, { value: 'b', label: 'B', disabled: true }],
      modelValue: 'a',
    })
    await w.find('.trigger').trigger('click')
    await w.findAll('.option')[1].trigger('mousedown')
    expect(w.emitted('update:modelValue')).toBeFalsy()
  })

  it('does not open when disabled', async () => {
    const w = mk({ disabled: true })
    await w.find('.trigger').trigger('click')
    expect(w.find('.panel').exists()).toBe(false)
  })

  it('shows "No options" when options array is empty', async () => {
    const w = mk({ options: [], modelValue: null })
    await w.find('.trigger').trigger('click')
    expect(w.find('.empty').exists()).toBe(true)
  })
})
