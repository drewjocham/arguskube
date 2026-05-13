import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref, nextTick } from 'vue'

// Create mutable refs so tests can inject data into the composable.
const mockApps = ref([])
const mockSelectedApp = ref(null)
const mockResources = ref([])
const mockDiffs = ref([])
const mockProjects = ref([])
const mockStatus = ref(null)
const mockLoading = ref(false)
const mockError = ref(null)

const mockFetchStatus = vi.fn()
const mockListApps = vi.fn()
const mockListProjects = vi.fn()
const mockGetApp = vi.fn()
const mockGetResources = vi.fn()
const mockGetDiffs = vi.fn()
const mockSyncApp = vi.fn()
const mockRefreshApp = vi.fn()
const mockRollbackApp = vi.fn()
const mockTestConnection = vi.fn()

vi.mock('../../composables/useWails', () => ({
  useArgusCD: vi.fn(() => ({
    apps: mockApps,
    selectedApp: mockSelectedApp,
    resources: mockResources,
    diffs: mockDiffs,
    projects: mockProjects,
    status: mockStatus,
    loading: mockLoading,
    error: mockError,
    fetchStatus: mockFetchStatus,
    listApps: mockListApps,
    listProjects: mockListProjects,
    getApp: mockGetApp,
    getResources: mockGetResources,
    getDiffs: mockGetDiffs,
    syncApp: mockSyncApp,
    refreshApp: mockRefreshApp,
    rollbackApp: mockRollbackApp,
    testConnection: mockTestConnection,
  })),
}))



function makeApp(name, overrides = {}) {
  return {
    name,
    project: 'default',
    namespace: 'default',
    syncStatus: 'Synced',
    healthStatus: 'Healthy',
    repoUrl: 'https://github.com/example/app',
    path: 'deploy',
    targetRevision: 'main',
    destServer: 'https://kubernetes.default.svc',
    destNamespace: 'prod',
    lastSync: '2m ago',
    replicas: 3,
    readyReplicas: 3,
    image: 'nginx:1.25',
    ...overrides,
  }
}

function createWrapper(options = {}) {
  const { provide = {}, props = {} } = options
  return mount(ArgusCDList, {
    global: {
      provide: {
        isAllowed: () => true,
        ...provide,
      },
    },
    props,
  })
}

// Dynamic import so mocks apply.
let ArgusCDList

beforeEach(async () => {
  document.body.innerHTML = ''
  vi.clearAllMocks()
  mockApps.value = []
  mockSelectedApp.value = null
  mockResources.value = []
  mockDiffs.value = []
  mockStatus.value = null
  mockLoading.value = false
  mockError.value = null

  // Re-import to get fresh mocks.
  ArgusCDList = (await import('../../components/operations/ArgusCDList.vue')).default
})

describe('ArgusCDList.vue — Integration', () => {
  it('shows loading state when loading and no apps', async () => {
    mockLoading.value = true
    const wrapper = createWrapper()
    await nextTick()
    expect(wrapper.text()).toContain('Loading applications')
  })

  it('shows error state when error is set', async () => {
    mockError.value = 'Connection refused'
    const wrapper = createWrapper()
    await nextTick()
    expect(wrapper.text()).toContain('Connection refused')
    expect(wrapper.find('.empty-state.error').exists()).toBe(true)
  })

  it('shows empty state when no apps and not loading', async () => {
    const wrapper = createWrapper()
    await nextTick()
    expect(wrapper.text()).toContain('No applications found')
  })



  it('renders apps in grid when apps are populated', async () => {
    mockApps.value = [
      makeApp('app-one'),
      makeApp('app-two'),
    ]
    mockStatus.value = { connected: true, url: 'https://argocd.example.com' }
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    const cards = wrapper.findAll('.app-card')
    expect(cards.length).toBe(2)
    expect(wrapper.text()).toContain('app-one')
    expect(wrapper.text()).toContain('app-two')
  })

  it('renders header with title and refresh button', async () => {
    const wrapper = createWrapper()
    await nextTick()
    expect(wrapper.text()).toContain('ArgusCD')
    expect(wrapper.find('.refresh-btn').exists()).toBe(true)
  })

  it('shows connection subtitle when connected', async () => {
    mockStatus.value = { connected: true, url: 'https://argocd.example.com' }
    const wrapper = createWrapper()
    await nextTick()
    expect(wrapper.text()).toContain('Connected to')
    expect(wrapper.text()).toContain('https://argocd.example.com')
  })

  it('shows checking connection message when not connected', async () => {
    mockStatus.value = { connected: false, message: 'Checking...' }
    const wrapper = createWrapper()
    await nextTick()
    expect(wrapper.text()).toContain('Checking...')
  })

  it('shows app stats when apps exist', async () => {
    mockApps.value = [
      makeApp('app-1'),
      makeApp('app-2', { syncStatus: 'OutOfSync' }),
      makeApp('app-3', { healthStatus: 'Degraded' }),
    ]
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    expect(wrapper.text()).toContain('3 apps')
    expect(wrapper.text()).toContain('1 out of sync')
    expect(wrapper.text()).toContain('1 degraded')
  })

  it('renders app card with sync and health status', async () => {
    mockApps.value = [makeApp('my-app', { syncStatus: 'Synced', healthStatus: 'Healthy' })]
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    expect(wrapper.text()).toContain('Synced')
    expect(wrapper.text()).toContain('Healthy')
  })

  it('renders app card metadata (repo, dest, image)', async () => {
    // Use makeApp with destServer set to something short since it may be truncated
    const app = makeApp('my-app', { destServer: 'kubernetes.svc', repoUrl: 'https://github.com/example/app' })
    mockApps.value = [app]
    mockStatus.value = { connected: true, url: 'https://argocd.example.com' }
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('https://github.com/example/app')
    expect(text).toContain('nginx:1.25')
    expect(text).toContain('ns: prod')
  })

  it('renders card footer with last sync and replica count', async () => {
    mockApps.value = [makeApp('my-app')]
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    expect(wrapper.text()).toContain('Last Sync')
    expect(wrapper.text()).toContain('2m ago')
    expect(wrapper.text()).toContain('3/3 ready')
  })

  it('selects an app and shows detail view on card click', async () => {
    const app = makeApp('my-app')
    mockApps.value = [app]
    mockGetResources.mockResolvedValue([])
    mockGetDiffs.mockResolvedValue([])

    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    // Click the app card.
    await wrapper.find('.app-card').trigger('click')
    await nextTick()

    expect(mockGetResources).toHaveBeenCalledWith('my-app')
    expect(mockGetDiffs).toHaveBeenCalledWith('my-app')
  })

  it('triggers sync when sync button is clicked', async () => {
    const app = makeApp('my-app')
    mockApps.value = [app]
    mockSyncApp.mockResolvedValue({ message: 'Sync triggered' })

    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    // Click the sync-action-btn on the card.
    const syncBtn = wrapper.find('.sync-action-btn')
    expect(syncBtn.exists()).toBe(true)
    await syncBtn.trigger('click')
    await nextTick()

    expect(mockSyncApp).toHaveBeenCalledWith('my-app', expect.any(String))
  })

  it('shows success notification after sync', async () => {
    const app = makeApp('my-app')
    mockApps.value = [app]
    mockSyncApp.mockResolvedValue({ message: 'Sync triggered' })

    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    await wrapper.find('.sync-action-btn').trigger('click')
    await nextTick()

    const notification = wrapper.find('.notification')
    expect(notification.exists()).toBe(true)
    expect(notification.classes()).toContain('success')
    expect(wrapper.text()).toContain('my-app')
    expect(wrapper.text()).toContain('Sync triggered')
  })

  it('shows error notification when sync fails', async () => {
    const app = makeApp('my-app')
    mockApps.value = [app]
    mockSyncApp.mockRejectedValue(new Error('Connection timeout'))

    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    await wrapper.find('.sync-action-btn').trigger('click')
    await nextTick()

    const notification = wrapper.find('.notification')
    expect(notification.exists()).toBe(true)
    expect(notification.classes()).toContain('error')
    expect(wrapper.text()).toContain('Sync failed')
  })

  it('shows drift detection panel when app is OutOfSync', async () => {
    const app = makeApp('my-app', { syncStatus: 'OutOfSync', replicas: 3, readyReplicas: 1 })
    mockApps.value = [app]
    mockStatus.value = { connected: false, message: 'Checking...' }
    mockGetResources.mockResolvedValue([])
    mockGetDiffs.mockResolvedValue([])

    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    // Click the card to select (triggers onSelectApp which sets selectedApp)
    await wrapper.find('.app-card').trigger('click')
    await nextTick()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Drift Detection')
    // The drill panel shows the OutOfSync fallback diff with replica info
    expect(text).toContain('Out of Sync')
  })

  it('shows diff content in drift panel when diffs exist', async () => {
    const app = makeApp('my-app')
    mockApps.value = [app]
    mockSelectedApp.value = app
    mockStatus.value = { connected: true, url: 'https://argocd.example.com' }
    mockResources.value = [{ kind: 'Deployment', name: 'my-app', namespace: 'prod' }]
    mockDiffs.value = [{ resource: 'deployment.yaml', diff: '--- live\n+++ target\n' }]
    mockGetResources.mockResolvedValue([{ kind: 'Deployment', name: 'my-app', namespace: 'prod' }])
    mockGetDiffs.mockResolvedValue([{ resource: 'deployment.yaml', diff: '--- live\n+++ target\n' }])

    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Drift Detection')
    expect(text).toContain('1 Drift')
    expect(text).toContain('deployment.yaml')
  })

  it('hides the project filter when no projects are returned', async () => {
    mockApps.value = [makeApp('a')]
    mockProjects.value = []
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()
    expect(wrapper.find('.argus-select').exists()).toBe(false)
  })

  it('renders the project filter when projects exist and re-lists on change', async () => {
    mockApps.value = [makeApp('a')]
    mockProjects.value = ['default', 'platform']
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    // The native <select> is replaced by the argus-select component.
    const selectWrapper = wrapper.find('.argus-select')
    expect(selectWrapper.exists()).toBe(true)

    // Open the panel by clicking the trigger button.
    await selectWrapper.find('button.trigger').trigger('click')
    await nextTick()

    const panel = selectWrapper.find('.panel')
    expect(panel.exists()).toBe(true)
    const optionTexts = panel.findAll('.option').map(o => o.text())
    expect(optionTexts).toContain('All projects')
    expect(optionTexts).toContain('default')
    expect(optionTexts).toContain('platform')

    // Pick 'platform' — fires update:modelValue → projectFilter watcher → listApps.
    mockListApps.mockClear()
    const platformOption = panel.findAll('.option').find(o => o.text() === 'platform')
    await platformOption.trigger('mousedown')
    await flushPromises()
    expect(mockListApps).toHaveBeenCalledWith('platform')
  })

  it('shows revision history when selectedApp has history', async () => {
    const app = makeApp('my-app', {
      history: [
        { id: 3, revision: 'cccccccc', deployedAt: '2026-01-03T10:00:00Z' },
        { id: 2, revision: 'bbbbbbbb', deployedAt: '2026-01-02T10:00:00Z' },
        { id: 1, revision: 'aaaaaaaa', deployedAt: '2026-01-01T10:00:00Z' },
      ],
    })
    mockApps.value = [app]
    mockSelectedApp.value = app
    mockStatus.value = { connected: true, url: 'https://argocd.example.com' }
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('Revision History')
    const entries = wrapper.findAll('.history-entry')
    expect(entries.length).toBe(3)
    // First entry is "current" with no rollback button.
    expect(entries[0].text()).toContain('Current')
    expect(entries[0].find('button.rollback-btn').exists()).toBe(false)
    // Subsequent entries expose a rollback button.
    expect(entries[1].find('button.rollback-btn').exists()).toBe(true)
    expect(entries[2].find('button.rollback-btn').exists()).toBe(true)
  })

  it('rolling back invokes rollbackApp with the entry id when confirmed', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)
    mockRollbackApp.mockResolvedValue()
    const app = makeApp('my-app', {
      history: [
        { id: 3, revision: 'cccccccc', deployedAt: '2026-01-03T10:00:00Z' },
        { id: 2, revision: 'bbbbbbbb', deployedAt: '2026-01-02T10:00:00Z' },
      ],
    })
    mockApps.value = [app]
    mockSelectedApp.value = app
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    const rollbackBtn = wrapper.findAll('.history-entry')[1].find('button.rollback-btn')
    await rollbackBtn.trigger('click')
    await flushPromises()
    expect(mockRollbackApp).toHaveBeenCalledWith('my-app', 2)
    confirmSpy.mockRestore()
  })

  it('rolling back does NOT call rollbackApp when the user cancels the confirm', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const app = makeApp('my-app', {
      history: [
        { id: 3, revision: 'cccccccc', deployedAt: '2026-01-03T10:00:00Z' },
        { id: 2, revision: 'bbbbbbbb', deployedAt: '2026-01-02T10:00:00Z' },
      ],
    })
    mockApps.value = [app]
    mockSelectedApp.value = app
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()

    await wrapper.findAll('.history-entry')[1].find('button.rollback-btn').trigger('click')
    await flushPromises()
    expect(mockRollbackApp).not.toHaveBeenCalled()
    confirmSpy.mockRestore()
  })

  it('shows an empty-history message when selectedApp has no history', async () => {
    const app = makeApp('my-app', { history: [] })
    mockApps.value = [app]
    mockSelectedApp.value = app
    mockStatus.value = { connected: true, url: 'https://argocd.example.com' }
    const wrapper = createWrapper()
    await nextTick()
    await flushPromises()
    expect(wrapper.find('.history-empty').exists()).toBe(true)
  })

  it('shows ProGateOverlay when isAllowed returns false', async () => {
    mockStatus.value = { connected: false, message: 'Checking...' }
    const wrapper = mount(ArgusCDList, {
      global: {
        provide: {
          isAllowed: () => false,
        },
      },
    })
    await nextTick()
    await flushPromises()

    // The real ProGateOverlay renders these texts (custom description prop)
    expect(wrapper.text()).toContain('Argus Pro')
    expect(wrapper.find('.pro-gate-overlay').exists()).toBe(true)
  })
})
