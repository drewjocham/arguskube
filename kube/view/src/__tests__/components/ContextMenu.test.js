import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ContextMenu from '../../components/shared/ContextMenu.vue'

function mountMenu(props = {}) {
  return mount(ContextMenu, {
    props: {
      x: 50,
      y: 100,
      items: [
        { id: 'hide', label: 'Hide section' },
        { id: 'show-all', label: 'Show all' },
        { id: 'danger', label: 'Drop', danger: true },
        { id: 'noop', label: 'Disabled', disabled: true },
      ],
      ...props,
    },
    attachTo: document.body,
  })
}

describe('ContextMenu', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('renders one button per item with the correct data-testid', () => {
    const wrapper = mountMenu()
    expect(document.querySelector('[data-testid="context-menu-hide"]')).toBeTruthy()
    expect(document.querySelector('[data-testid="context-menu-show-all"]')).toBeTruthy()
    expect(document.querySelector('[data-testid="context-menu-noop"]')).toBeTruthy()
  })

  it('positions the menu at the given x/y', () => {
    mountMenu({ x: 123, y: 456 })
    const menu = document.querySelector('[data-testid="context-menu"]')
    expect(menu.style.left).toBe('123px')
    expect(menu.style.top).toBe('456px')
  })

  it('emits select(id) on click + close', async () => {
    const wrapper = mountMenu()
    const btn = document.querySelector('[data-testid="context-menu-hide"]')
    btn.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('select')[0]).toEqual(['hide'])
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('disabled items neither emit select nor close', async () => {
    const wrapper = mountMenu()
    const btn = document.querySelector('[data-testid="context-menu-noop"]')
    btn.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('select')).toBeUndefined()
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('Escape key closes the menu', async () => {
    const wrapper = mountMenu()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('outside click closes the menu', async () => {
    const wrapper = mountMenu()
    // Dispatch a mousedown event whose target is NOT inside the menu.
    const evt = new MouseEvent('mousedown', { bubbles: true })
    document.body.dispatchEvent(evt)
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('danger items render the danger class', () => {
    mountMenu()
    const danger = document.querySelector('[data-testid="context-menu-danger"]')
    expect(danger.classList.contains('danger')).toBe(true)
  })
})
