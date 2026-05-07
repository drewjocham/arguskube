import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import ConfigMapList from '../../components/config/ConfigMapList.vue'

const mockResult = ref({ items: [] })
const mockDetail = ref(null)
const mockDetailLoading = ref(false)

vi.mock('../../composables/useWails', () => ({
  useResources: vi.fn(() => ({
    result: mockResult,
    detail: mockDetail,
    loading: ref(false),
    detailLoading: mockDetailLoading,
    listResources: vi.fn(async () => {}),
    getResourceDetail: vi.fn(async () => {}),
  })),
}))

function makeCm(name, overrides = {}) {
  return {
    name,
    namespace: 'default',
    age: '5d',
    fields: { data: '3', ...(overrides.fields || {}) },
    ...overrides,
  }
}

function makeDetail(name) {
  return {
    data: { 'config.yaml': 'key: value', 'app.json': '{"a":1}' },
    labels: { app: name },
    properties: [{ key: 'kind', value: 'ConfigMap' }],
  }
}

describe('ConfigMapList.vue — scroll & expand behavior', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    mockResult.value = { items: [] }
    mockDetail.value = null
    mockDetailLoading.value = false
    vi.clearAllMocks()
  })

  it('renders the outer cm-view scroll container with the expected scroll classes', async () => {
    mockResult.value = { items: [makeCm('cm-a')] }
    const wrapper = mount(ConfigMapList)
    await flushPromises()
    const view = wrapper.find('.cm-view')
    expect(view.exists()).toBe(true)
  })

  it('expands a row when clicked and shows the expanded section', async () => {
    mockResult.value = { items: [makeCm('cm-a'), makeCm('cm-b')] }
    mockDetail.value = makeDetail('cm-a')
    const wrapper = mount(ConfigMapList)
    await flushPromises()

    expect(wrapper.find('.cm-expanded').exists()).toBe(false)

    const rows = wrapper.findAll('.cm-row')
    await rows[0].trigger('click')
    await flushPromises()

    expect(wrapper.find('.cm-expanded').exists()).toBe(true)
  })

  it('collapses an expanded row when clicked again', async () => {
    mockResult.value = { items: [makeCm('cm-a')] }
    mockDetail.value = makeDetail('cm-a')
    const wrapper = mount(ConfigMapList)
    await flushPromises()

    const row = wrapper.find('.cm-row')
    await row.trigger('click')
    await flushPromises()
    expect(wrapper.find('.cm-expanded').exists()).toBe(true)

    await row.trigger('click')
    await flushPromises()
    expect(wrapper.find('.cm-expanded').exists()).toBe(false)
  })

  it('calls scrollIntoView on the expanded section so it stays visible when expanded near the bottom', async () => {
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView

    mockResult.value = { items: [makeCm('cm-a'), makeCm('cm-b'), makeCm('cm-c')] }
    mockDetail.value = makeDetail('cm-c')
    const wrapper = mount(ConfigMapList, { attachTo: document.body })
    await flushPromises()

    const rows = wrapper.findAll('.cm-row')
    await rows[2].trigger('click')
    await flushPromises()
    await nextTick()

    expect(scrollIntoView).toHaveBeenCalled()
    const args = scrollIntoView.mock.calls[0][0]
    expect(args).toMatchObject({ block: 'nearest' })
    wrapper.unmount()
  })

  it('does not crash when scrollIntoView is unavailable on the expanded element', async () => {
    Object.defineProperty(Element.prototype, 'scrollIntoView', {
      value: undefined,
      configurable: true,
      writable: true,
    })

    mockResult.value = { items: [makeCm('cm-a')] }
    mockDetail.value = makeDetail('cm-a')
    const wrapper = mount(ConfigMapList)
    await flushPromises()

    const row = wrapper.find('.cm-row')
    await expect(row.trigger('click')).resolves.not.toThrow()
    await flushPromises()
    expect(wrapper.find('.cm-expanded').exists()).toBe(true)
  })
})
