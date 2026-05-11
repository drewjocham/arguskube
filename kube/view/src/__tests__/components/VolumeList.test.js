import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import VolumeList from '../../components/storage/VolumeList.vue'

const mockResult = ref({ items: [] })
const mockDetail = ref(null)

vi.mock('../../composables/useWails', () => ({
  useResources: vi.fn(() => ({
    result: mockResult,
    detail: mockDetail,
    loading: ref(false),
    error: ref(null),
    detailLoading: ref(false),
    listResources: vi.fn(async () => {}),
    getResourceDetail: vi.fn(async () => {}),
  })),
}))

function makePVC(name, ns = 'default') {
  return {
    name, namespace: ns, status: 'Bound', statusColor: 'green',
    fields: { capacity: '10Gi', access_modes: 'RWO', storage_class: 'standard' },
    age: '5d',
  }
}
function makePV(name) {
  return {
    name, namespace: '', status: 'Bound', statusColor: 'green',
    fields: { capacity: '50Gi', access_modes: 'RWO', storage_class: 'fast-ssd' },
    age: '12d',
  }
}

describe('VolumeList.vue — PV vs PVC differentiation', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    mockResult.value = { items: [] }
    mockDetail.value = null
    vi.clearAllMocks()
  })

  it('renders the "Volume Claims" title and includes the namespace column for type=pvcs', async () => {
    mockResult.value = { items: [makePVC('data-claim', 'prod')] }
    const wrapper = mount(VolumeList, { props: { type: 'pvcs' } })
    await flushPromises()
    await nextTick()
    await flushPromises()

    expect(wrapper.find('.title').text()).toContain('Volume Claims')
    // Namespace column header should be present.
    const headers = wrapper.findAll('.vol-header-row > div').map(d => d.text())
    expect(headers).toContain('Namespace')
    // The list root should NOT carry the is-pv variant class.
    expect(wrapper.find('.vol-list').classes()).not.toContain('is-pv')
  })

  it('renders the "Persistent Volumes" title and hides the namespace column for type=pvs', async () => {
    mockResult.value = { items: [makePV('pv-1')] }
    const wrapper = mount(VolumeList, { props: { type: 'pvs' } })
    await flushPromises()
    await nextTick()
    await flushPromises()

    expect(wrapper.find('.title').text()).toContain('Persistent Volumes')
    const headers = wrapper.findAll('.vol-header-row > div').map(d => d.text())
    expect(headers).not.toContain('Namespace')
    expect(wrapper.find('.vol-list').classes()).toContain('is-pv')
    // No .col-ns cell rendered in the row either.
    expect(wrapper.find('.vol-row .col-ns').exists()).toBe(false)
  })

  it('uses a different empty-state message for PVs vs PVCs', async () => {
    const pvcWrapper = mount(VolumeList, { props: { type: 'pvcs' } })
    await flushPromises()
    expect(pvcWrapper.find('.state-box').text()).toContain('PersistentVolumeClaims')

    const pvWrapper = mount(VolumeList, { props: { type: 'pvs' } })
    await flushPromises()
    expect(pvWrapper.find('.state-box').text()).toContain('PersistentVolumes')
  })

  it('shows a "Cluster-scoped" chip for PVs and "Namespaced" chip for PVCs', async () => {
    const pvWrapper = mount(VolumeList, { props: { type: 'pvs' } })
    await flushPromises()
    const pvChip = pvWrapper.find('.scope-chip')
    expect(pvChip.exists()).toBe(true)
    expect(pvChip.text()).toBe('Cluster-scoped')
    expect(pvChip.classes()).toContain('scope-cluster')

    const pvcWrapper = mount(VolumeList, { props: { type: 'pvcs' } })
    await flushPromises()
    const pvcChip = pvcWrapper.find('.scope-chip')
    expect(pvcChip.text()).toBe('Namespaced')
    expect(pvcChip.classes()).toContain('scope-ns')
  })

  it('expanding a row scrolls it into view (so it does not fall off the screen)', async () => {
    const scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView

    mockResult.value = { items: [makePVC('a', 'ns'), makePVC('b', 'ns'), makePVC('c', 'ns')] }
    mockDetail.value = { properties: [{ key: 'kind', value: 'PVC' }], events: [], labels: {} }

    const wrapper = mount(VolumeList, { props: { type: 'pvcs' }, attachTo: document.body })
    await flushPromises()
    await nextTick()
    await flushPromises()

    const rows = wrapper.findAll('.vol-row')
    expect(rows.length).toBe(3)
    await rows[2].trigger('click')
    await flushPromises()
    await nextTick()

    expect(wrapper.find('.vol-expanded').exists()).toBe(true)
    expect(scrollIntoView).toHaveBeenCalled()
    expect(scrollIntoView.mock.calls[0][0]).toMatchObject({ block: 'nearest' })
    wrapper.unmount()
  })
})
