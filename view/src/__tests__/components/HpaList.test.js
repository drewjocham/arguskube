import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import HpaList from '../../components/config/HpaList.vue'

const mockResult = ref({ items: [] })
const mockDetail = ref(null)
const mockDetailLoading = ref(false)
const mockVpas = ref([])
const mockVpasLoading = ref(false)
const mockVpasError = ref(null)

vi.mock('../../composables/useWails', () => ({
  useResources: vi.fn(() => ({
    result: mockResult,
    detail: mockDetail,
    loading: ref(false),
    detailLoading: mockDetailLoading,
    listResources: vi.fn(async () => {}),
    getResourceDetail: vi.fn(async () => {}),
  })),
  useVPARecommendations: vi.fn(() => ({
    vpas: mockVpas,
    loading: mockVpasLoading,
    error: mockVpasError,
    fetchVPAs: vi.fn(async () => {}),
  })),
}))

function makeHpa(name, overrides = {}) {
  return {
    name,
    namespace: 'default',
    age: '2d',
    fields: {
      reference: 'Deployment/api',
      targets: '40% / 80%',
      min_pods: '2',
      max_pods: '10',
      replicas: '3',
      ...(overrides.fields || {}),
    },
    ...overrides,
  }
}

function makeDetail() {
  return {
    properties: [{ key: 'kind', value: 'HorizontalPodAutoscaler' }],
    conditions: [{ type: 'AbleToScale', status: 'True', reason: 'ReadyForNewScale' }],
    events: [],
  }
}

describe('HpaList.vue — scroll & expand behavior', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    mockResult.value = { items: [] }
    mockDetail.value = null
    mockDetailLoading.value = false
    mockVpas.value = []
    mockVpasLoading.value = false
    mockVpasError.value = null
    vi.clearAllMocks()
  })

  it('renders the outer hpa-view scroll container', async () => {
    mockResult.value = { items: [makeHpa('hpa-a')] }
    const wrapper = mount(HpaList)
    await flushPromises()
    const view = wrapper.find('.hpa-view')
    expect(view.exists()).toBe(true)
  })

  it('expands a row when clicked and shows the expanded section', async () => {
    mockResult.value = { items: [makeHpa('hpa-a'), makeHpa('hpa-b')] }
    mockDetail.value = makeDetail()
    const wrapper = mount(HpaList)
    await flushPromises()

    expect(wrapper.find('.hpa-expanded').exists()).toBe(false)

    const rows = wrapper.findAll('.hpa-row')
    await rows[0].trigger('click')
    await flushPromises()

    expect(wrapper.find('.hpa-expanded').exists()).toBe(true)
  })

  it('collapses an expanded row when clicked again', async () => {
    mockResult.value = { items: [makeHpa('hpa-a')] }
    mockDetail.value = makeDetail()
    const wrapper = mount(HpaList)
    await flushPromises()

    const row = wrapper.find('.hpa-row')
    await row.trigger('click')
    await flushPromises()
    expect(wrapper.find('.hpa-expanded').exists()).toBe(true)

    await row.trigger('click')
    await flushPromises()
    expect(wrapper.find('.hpa-expanded').exists()).toBe(false)
  })

  it('calls scrollIntoView on the expanded section so it stays visible when expanded near the bottom', async () => {
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView

    mockResult.value = { items: [makeHpa('hpa-a'), makeHpa('hpa-b'), makeHpa('hpa-c')] }
    mockDetail.value = makeDetail()
    const wrapper = mount(HpaList, { attachTo: document.body })
    await flushPromises()

    const rows = wrapper.findAll('.hpa-row')
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

    mockResult.value = { items: [makeHpa('hpa-a')] }
    mockDetail.value = makeDetail()
    const wrapper = mount(HpaList)
    await flushPromises()

    const row = wrapper.find('.hpa-row')
    await expect(row.trigger('click')).resolves.not.toThrow()
    await flushPromises()
    expect(wrapper.find('.hpa-expanded').exists()).toBe(true)
  })
})
