import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises, shallowMount } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import NodeList from '../../components/cluster/NodeList.vue'

// Use mutable refs so we can control mock data from tests.
const mockResult = ref({ items: [] })
const mockDetail = ref({ properties: [] })
const mockDetailLoading = ref(false)
const mockLogs = ref([])
const mockLogsLoading = ref(false)
const mockLogsError = ref(null)

vi.mock('../../composables/useWails', () => ({
  useResources: vi.fn(() => ({
    result: mockResult,
    detail: mockDetail,
    loading: ref(false),
    detailLoading: mockDetailLoading,
    listResources: vi.fn(async () => {}),
    getResourceDetail: vi.fn(async () => {}),
  })),
  useNodeLogs: vi.fn(() => ({
    logs: mockLogs,
    loading: mockLogsLoading,
    error: mockLogsError,
    fetchNodeLogs: vi.fn(async () => {}),
    clear: vi.fn(),
  })),
  callGo: vi.fn(async () => ({ datapoints: null })),
}))

// Helper to build a realistic node item.
function makeNode(name, status = 'Ready', overrides = {}) {
  return {
    name,
    status,
    age: '10d',
    fields: {
      roles: 'worker',
      version: 'v1.28.3',
      os_image: 'Ubuntu 22.04',
      cpu_capacity: '8',
      mem_capacity: '32Gi',
      internal_ip: '10.0.0.1',
      ...(overrides.fields || {}),
    },
    ...overrides,
  }
}

function createWrapper() {
  return mount(NodeList)
}

describe('NodeList.vue — Integration', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    // Reset mock data before each test.
    mockResult.value = { items: [] }
    mockDetail.value = { properties: [] }
    mockDetailLoading.value = false
    mockLogs.value = []
    mockLogsLoading.value = false
    mockLogsError.value = null
    vi.clearAllMocks()
  })

  it('shows the header with title and refresh button', () => {
    const wrapper = createWrapper()
    expect(wrapper.text()).toContain('Cluster Nodes')
    expect(wrapper.find('.refresh-btn').exists()).toBe(true)
  })

  it('renders node cards when nodes are populated', async () => {
    mockResult.value = {
      items: [
        makeNode('node-1'),
        makeNode('node-2', 'Ready'),
        makeNode('node-3', 'NotReady'),
      ],
    }
    const wrapper = createWrapper()
    await flushPromises()
    // Re-trigger reactivity by simulating what fetchNodes does.
    // The issue: onMounted calls listResources which is mocked to do nothing,
    // but the component then checks mockResult.value items directly.
    // Since mockResult is a ref now, reassignment triggers reactivity.
    // However, the component's fetchNodes function only runs once on mount.
    // We need fetchNodes to re-run. Let's trigger it via the refresh button.
    await wrapper.find('.refresh-btn').trigger('click')
    await flushPromises()
    const cards = wrapper.findAll('.node-card')
    expect(cards.length).toBe(3)
  })

  it('shows node name, status, roles, version, and age on each card', async () => {
    mockResult.value = {
      items: [
        makeNode('node-prod-1', 'Ready'),
      ],
    }
    const wrapper = createWrapper()
    await wrapper.find('.refresh-btn').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('node-prod-1')
    expect(wrapper.text()).toContain('Ready')
    expect(wrapper.text()).toContain('worker')
    expect(wrapper.text()).toContain('v1.28.3')
    expect(wrapper.text()).toContain('10d')
  })

  it('shows not-ready styling for nodes with status !== Ready', async () => {
    mockResult.value = {
      items: [
        makeNode('node-bad', 'NotReady'),
      ],
    }
    const wrapper = createWrapper()
    await wrapper.find('.refresh-btn').trigger('click')
    await flushPromises()
    const cards = wrapper.findAll('.node-card')
    expect(cards.length).toBe(1)
    expect(cards[0].classes()).toContain('not-ready')
  })

  it('shows "8" (cpuCapacity) and "32Gi" (memCapacity) on Ready nodes', async () => {
    mockResult.value = {
      items: [
        makeNode('node-a', 'Ready'),
      ],
    }
    const wrapper = createWrapper()
    await wrapper.find('.refresh-btn').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('8')
    expect(wrapper.text()).toContain('32Gi')
  })

  it('toggles expand on node click, showing expanded content', async () => {
    mockResult.value = {
      items: [
        makeNode('expand-me', 'Ready'),
      ],
    }
    const wrapper = createWrapper()
    await wrapper.find('.refresh-btn').trigger('click')
    await flushPromises()

    // Click the node name link to expand.
    const nodeName = wrapper.find('.node-name')
    expect(nodeName.exists()).toBe(true)
    await nodeName.trigger('click')
    await flushPromises()

    // After clicking, the expanded content should appear.
    const expandedContent = wrapper.find('.node-expanded-content')
    expect(expandedContent.exists()).toBe(true)
    // Collapse the node name link.
    await nodeName.trigger('click')
    await flushPromises()
    expect(wrapper.find('.node-expanded-content').exists()).toBe(false)
  })

  it('shows sparkline hint when no metrics data available', async () => {
    mockResult.value = {
      items: [
        makeNode('spark-test', 'Ready'),
      ],
    }
    const wrapper = createWrapper()
    await wrapper.find('.refresh-btn').trigger('click')
    await flushPromises()
    // Expand node.
    await wrapper.find('.node-name').trigger('click')
    await flushPromises()
    // Sparkline hint should be shown.
    const sparkHints = wrapper.findAll('.spark-hint')
    expect(sparkHints.length).toBeGreaterThanOrEqual(1)
    expect(sparkHints[0].text()).toBe('unavailable')
  })

  it('shows logs-empty message when no logs available', async () => {
    mockResult.value = {
      items: [
        makeNode('log-test', 'Ready'),
      ],
    }
    const wrapper = createWrapper()
    await wrapper.find('.refresh-btn').trigger('click')
    await flushPromises()
    // Expand node.
    await wrapper.find('.node-name').trigger('click')
    await flushPromises()
    const logsEmpty = wrapper.find('.logs-empty')
    expect(logsEmpty.exists()).toBe(true)
    expect(logsEmpty.text()).toContain('No logs available')
  })
})
