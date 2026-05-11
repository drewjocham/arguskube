import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'

const mockListResources = vi.fn(async () => [])
const mockGetDetail = vi.fn(async () => ({ properties: [] }))
const mockCallGo = vi.fn(async () => '')

vi.mock('../../composables/useBridge', () => ({
  callGo: (...args) => mockCallGo(...args),
  cachedCallGo: vi.fn(async (method, args) => {
    if (method === 'ListResources') {
      return {
        items: [
          { name: 'web-stateful-1', namespace: 'default', status: 'Running', statusColor: 'green', fields: { ready: '3/3', desired: '3', current: '3' }, age: '5d' },
          { name: 'web-stateful-2', namespace: 'default', status: 'Running', statusColor: 'green', fields: { ready: '2/2', desired: '2', current: '2' }, age: '5d' },
          { name: 'web-stateful-3', namespace: 'default', status: 'Running', statusColor: 'green', fields: { ready: '1/1', desired: '1', current: '1' }, age: '5d' },
        ],
      }
    }
    if (method === 'GetResourceDetail') {
      return mockGetDetail(args)
    }
    return null
  }),
  invalidateCache: vi.fn(),
  DEFAULT_TTL: 0,
}))

let StatefulDaemonSetList

describe('StatefulDaemonSetList.vue — scroll behaviour', () => {
  beforeEach(async () => {
    document.body.innerHTML = ''
    vi.clearAllMocks()
    StatefulDaemonSetList = (await import('../../components/workloads/StatefulDaemonSetList.vue')).default
  })

  it('renders the outer ws-view scroll container', async () => {
    const wrapper = mount(StatefulDaemonSetList, { props: { type: 'statefulsets' } })
    await flushPromises()
    expect(wrapper.find('.ws-view').exists()).toBe(true)
  })

  it('expanding a row calls scrollIntoView on the expanded section so it stays visible', async () => {
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView

    mockGetDetail.mockResolvedValue({
      properties: [{ key: 'kind', value: 'StatefulSet' }],
      conditions: [],
      labels: {},
      events: [],
    })
    const wrapper = mount(StatefulDaemonSetList, {
      props: { type: 'statefulsets' },
      attachTo: document.body,
    })
    await flushPromises()

    // Click the third row — this is the bottom row, the most likely to fall
    // off the visible area without auto-scroll.
    const rows = wrapper.findAll('.ws-row')
    expect(rows.length).toBe(3)
    await rows[2].trigger('click')
    await flushPromises()
    await nextTick()

    expect(wrapper.find('.ws-expanded').exists()).toBe(true)
    expect(scrollIntoView).toHaveBeenCalled()
    const args = scrollIntoView.mock.calls[0][0]
    expect(args).toMatchObject({ block: 'nearest' })
    wrapper.unmount()
  })

  it('collapsing a row clears wsDetail and does not call scrollIntoView again', async () => {
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView

    mockGetDetail.mockResolvedValue({ properties: [], conditions: [], labels: {}, events: [] })
    const wrapper = mount(StatefulDaemonSetList, {
      props: { type: 'statefulsets' },
      attachTo: document.body,
    })
    await flushPromises()
    const row = wrapper.find('.ws-row')
    await row.trigger('click') // expand
    await flushPromises()
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    await row.trigger('click') // collapse
    await flushPromises()
    expect(wrapper.find('.ws-expanded').exists()).toBe(false)
    // No additional scroll call on collapse.
    expect(scrollIntoView).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('does not throw when scrollIntoView is unavailable on the expanded element', async () => {
    Object.defineProperty(Element.prototype, 'scrollIntoView', {
      value: undefined,
      configurable: true,
      writable: true,
    })

    mockGetDetail.mockResolvedValue({ properties: [], conditions: [], labels: {}, events: [] })
    const wrapper = mount(StatefulDaemonSetList, { props: { type: 'statefulsets' } })
    await flushPromises()
    const row = wrapper.find('.ws-row')
    await expect(row.trigger('click')).resolves.not.toThrow()
    await flushPromises()
    expect(wrapper.find('.ws-expanded').exists()).toBe(true)
  })
})
