import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ManifestEditPopup from '../../components/common/ManifestEditPopup.vue'

function createWrapper(overrides = {}) {
  return mount(ManifestEditPopup, {
    props: {
      open: true,
      loading: false,
      applying: false,
      editing: false,
      content: 'apiVersion: v1\nkind: Pod',
      kind: 'Pod',
      name: 'web-1',
      namespace: 'default',
      ...overrides,
    },
  })
}

describe('ManifestEditPopup.vue', () => {
  it('does not render when open is false', () => {
    const wrapper = mount(ManifestEditPopup, {
      props: { open: false, content: '' },
    })
    expect(wrapper.find('.popup-overlay').exists()).toBe(false)
  })

  it('renders the kind, name, and namespace in the header', () => {
    const wrapper = createWrapper()
    expect(wrapper.find('.popup-kind').text()).toBe('Pod')
    expect(wrapper.find('.popup-name').text()).toBe('web-1')
    expect(wrapper.find('.popup-ns').text()).toBe('default')
  })

  it('shows the viewer (pre) by default and the editor (textarea) when editing', async () => {
    const wrapper = createWrapper({ editing: false })
    expect(wrapper.find('pre.manifest-viewer').exists()).toBe(true)
    expect(wrapper.find('textarea.manifest-editor').exists()).toBe(false)

    await wrapper.setProps({ editing: true })
    expect(wrapper.find('pre.manifest-viewer').exists()).toBe(false)
    expect(wrapper.find('textarea.manifest-editor').exists()).toBe(true)
  })

  it('shows the loading indicator when loading is true', () => {
    const wrapper = createWrapper({ loading: true })
    expect(wrapper.find('.manifest-loading').exists()).toBe(true)
    expect(wrapper.find('pre.manifest-viewer').exists()).toBe(false)
    expect(wrapper.find('textarea.manifest-editor').exists()).toBe(false)
  })

  it('emits update:editing(true) when Edit is clicked', async () => {
    const wrapper = createWrapper({ editing: false })
    const editBtn = wrapper.findAll('button').find(b => b.text().includes('Edit'))
    await editBtn.trigger('click')
    expect(wrapper.emitted('update:editing')).toBeTruthy()
    expect(wrapper.emitted('update:editing')[0]).toEqual([true])
  })

  it('emits update:editing(false) when View is clicked while editing', async () => {
    const wrapper = createWrapper({ editing: true })
    const viewBtn = wrapper.findAll('button').find(b => b.text().includes('View'))
    await viewBtn.trigger('click')
    expect(wrapper.emitted('update:editing')[0]).toEqual([false])
  })

  it('emits update:content as the textarea is typed into', async () => {
    const wrapper = createWrapper({ editing: true })
    const ta = wrapper.find('textarea.manifest-editor')
    await ta.setValue('apiVersion: v2')
    expect(wrapper.emitted('update:content')).toBeTruthy()
    expect(wrapper.emitted('update:content').slice(-1)[0]).toEqual(['apiVersion: v2'])
  })

  it('emits apply when Redeploy is clicked', async () => {
    const wrapper = createWrapper()
    const apply = wrapper.findAll('button').find(b => b.text().includes('Redeploy'))
    await apply.trigger('click')
    expect(wrapper.emitted('apply')).toBeTruthy()
  })

  it('disables the Redeploy button when applying or content is empty', async () => {
    const wrapperApplying = createWrapper({ applying: true })
    const applyBtnA = wrapperApplying.findAll('button').find(b => b.text().includes('Applying'))
    expect(applyBtnA.attributes('disabled')).toBeDefined()

    const wrapperEmpty = createWrapper({ content: '   ' })
    const applyBtnB = wrapperEmpty.findAll('button').find(b => b.text().includes('Redeploy'))
    expect(applyBtnB.attributes('disabled')).toBeDefined()
  })

  it('emits close when the close (✕) button is clicked', async () => {
    const wrapper = createWrapper()
    await wrapper.find('.action-btn.close').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('emits close when the overlay is clicked outside the panel', async () => {
    const wrapper = createWrapper()
    await wrapper.find('.popup-overlay').trigger('click')
    expect(wrapper.emitted('close')).toBeTruthy()
  })

  it('does NOT emit close when the panel itself is clicked', async () => {
    const wrapper = createWrapper()
    await wrapper.find('.popup-panel').trigger('click')
    expect(wrapper.emitted('close')).toBeFalsy()
  })
})
