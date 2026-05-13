import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import CenterPanel from '../../components/center/CenterPanel.vue'
import { useSectionTabsStore } from '../../stores/sectionTabs'

// Mock every composable CenterPanel imports from useWails so the
// component mounts without trying to hit Wails.
vi.mock('../../composables/useWails', () => ({
  useArgusScan: vi.fn(() => ({
    report: { value: null }, loading: { value: false }, error: { value: null },
    runScan: vi.fn(),
  })),
}))

vi.mock('../../composables/useBackgroundTasks', () => ({
  useBackgroundTasks: vi.fn(() => ({
    tasks: {}, startTask: vi.fn(), completeTask: vi.fn(), failTask: vi.fn(),
    getTask: vi.fn(() => null), clearTask: vi.fn(),
    hasTask: vi.fn(() => false), isRunning: vi.fn(() => false), lastResult: vi.fn(() => null),
  })),
}))

// Child component stubs. vi.mock() requires statically analyzable
// arguments (hoisted to the top of the module by the test runner) so
// each path must be a literal string — no loops, no array iteration.
function stub(klass) {
  return { default: { name: klass, template: `<div class="mock-${klass}">${klass}</div>` } }
}
vi.mock('../../components/center/MetricsRow.vue', () => stub('metrics-row'))
vi.mock('../../components/center/AlertList.vue', () => stub('alert-list'))
vi.mock('../../components/center/LogStream.vue', () => stub('log-stream'))
vi.mock('../../components/center/MetricsExplorer.vue', () => stub('metrics-explorer'))
vi.mock('../../components/center/VulnerabilityList.vue', () => stub('vulnerability-list'))
vi.mock('../../components/center/LogExplorer.vue', () => stub('log-explorer'))
vi.mock('../../components/center/AnomalyDetection.vue', () => stub('anomaly-detection'))
vi.mock('../../components/center/ArgusScanReport.vue', () => stub('argus-scan-report'))
vi.mock('../../components/center/FinOpsView.vue', () => stub('finops-view'))
vi.mock('../../components/center/S3Notebook.vue', () => stub('s3-notebook'))
vi.mock('../../components/cluster/NodeList.vue', () => stub('node-list'))
vi.mock('../../components/cluster/NamespaceList.vue', () => stub('namespace-list'))
vi.mock('../../components/cluster/EventStream.vue', () => stub('event-stream'))
vi.mock('../../components/workloads/PodList.vue', () => stub('pod-list'))
vi.mock('../../components/workloads/DeploymentList.vue', () => stub('deployment-list'))
vi.mock('../../components/workloads/JobCronJobList.vue', () => stub('job-cronjob-list'))
vi.mock('../../components/workloads/StatefulDaemonSetList.vue', () => stub('stateful-daemonset-list'))
vi.mock('../../components/config/ConfigMapList.vue', () => stub('configmap-list'))
vi.mock('../../components/config/SecretsView.vue', () => stub('secrets-view'))
vi.mock('../../components/config/HpaList.vue', () => stub('hpa-list'))
vi.mock('../../components/network/ServiceList.vue', () => stub('service-list'))
vi.mock('../../components/network/NetworkPolicyList.vue', () => stub('network-policy-list'))
vi.mock('../../components/storage/VolumeList.vue', () => stub('volume-list'))
vi.mock('../../components/resources/ResourceTable.vue', () => stub('resource-table'))
vi.mock('../../components/resources/ResourceDetail.vue', () => stub('resource-detail'))
vi.mock('../../components/operations/RunbooksView.vue', () => stub('runbooks-view'))
vi.mock('../../components/operations/IncidentLog.vue', () => stub('incident-log'))
vi.mock('../../components/operations/ConfigAudit.vue', () => stub('config-audit'))
vi.mock('../../components/operations/ArgusCDList.vue', () => stub('arguscd-list'))
vi.mock('../../components/operations/PipelinesView.vue', () => stub('pipelines-view'))
vi.mock('../../components/knowledge/DocumentsView.vue', () => stub('documents-view'))
vi.mock('../../components/setup/SetupPanel.vue', () => stub('setup-panel'))
vi.mock('../../components/setup/SettingsPanel.vue', () => stub('settings-panel'))
vi.mock('../../components/ai/ArgusAIChat.vue', () => stub('argus-ai-chat'))
vi.mock('../../components/alerts/AlertsView.vue', () => stub('alerts-view'))

// Memory-backed localStorage for the sectionTabs store.
const memory = {}
Object.defineProperty(window, 'localStorage', {
  configurable: true,
  value: {
    getItem: (k) => (k in memory ? memory[k] : null),
    setItem: (k, v) => { memory[k] = String(v) },
    removeItem: (k) => { delete memory[k] },
    clear: () => { for (const k of Object.keys(memory)) delete memory[k] },
  },
})

// Helper to mount with a section + an explicit tab pre-seeded into the
// sectionTabs store. Returns the wrapper + the store so individual
// tests can also reach into it.
function mountSection(sectionId, tabId, extraProps = {}) {
  setActivePinia(createPinia())
  const tabs = useSectionTabsStore()
  tabs.setTab(sectionId, tabId)
  const wrapper = mount(CenterPanel, {
    props: {
      metrics: null,
      alerts: [],
      selectedAlert: null,
      logLines: [],
      activeNav: sectionId,
      ...extraProps,
    },
  })
  return { wrapper, tabs }
}

describe('CenterPanel.vue — section + tab routing', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    for (const k of Object.keys(memory)) delete memory[k]
    setActivePinia(createPinia())
  })

  // --- MONITORING ---

  it('renders SectionTabs for monitoring with all monitoring tabs', () => {
    const { wrapper } = mountSection('monitoring', 'metrics')
    expect(wrapper.find('[data-testid="section-tabs"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="section-tab-alerts"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="section-tab-metrics"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="section-tab-vulnerabilities"]').exists()).toBe(true)
  })

  it('clicking a section tab updates the store', async () => {
    const { wrapper, tabs } = mountSection('monitoring', 'metrics')
    expect(tabs.activeTab('monitoring')).toBe('metrics')
    await wrapper.find('[data-testid="section-tab-logs"]').trigger('click')
    expect(tabs.activeTab('monitoring')).toBe('logs')
  })

  it('monitoring/metrics renders MetricsExplorer', () => {
    const { wrapper } = mountSection('monitoring', 'metrics')
    expect(wrapper.find('.mock-metrics-explorer').exists()).toBe(true)
  })

  it('monitoring/vulnerabilities renders VulnerabilityList', () => {
    const { wrapper } = mountSection('monitoring', 'vulnerabilities')
    expect(wrapper.find('.mock-vulnerability-list').exists()).toBe(true)
  })

  it('monitoring/logs renders LogExplorer', () => {
    const { wrapper } = mountSection('monitoring', 'logs')
    expect(wrapper.find('.mock-log-explorer').exists()).toBe(true)
  })

  it('monitoring/anomalies renders AnomalyDetection', () => {
    const { wrapper } = mountSection('monitoring', 'anomalies')
    expect(wrapper.find('.mock-anomaly-detection').exists()).toBe(true)
  })

  it('monitoring/analysis renders ArgusScanReport', () => {
    const { wrapper } = mountSection('monitoring', 'analysis')
    expect(wrapper.find('.mock-argus-scan-report').exists()).toBe(true)
  })

  it('monitoring/finops renders FinOpsView', () => {
    const { wrapper } = mountSection('monitoring', 'finops')
    expect(wrapper.find('.mock-finops-view').exists()).toBe(true)
  })

  it('monitoring/argusai renders ArgusAIChat', () => {
    const { wrapper } = mountSection('monitoring', 'argusai')
    expect(wrapper.find('.mock-argus-ai-chat').exists()).toBe(true)
  })

  it('monitoring/alerts renders the inner sub-tabs (Cluster + Manage)', () => {
    const { wrapper } = mountSection('monitoring', 'alerts')
    expect(wrapper.text()).toContain('Cluster Alerts')
    expect(wrapper.text()).toContain('Manage Alerts')
  })

  // --- CLUSTER ---

  it('cluster/nodes renders NodeList', () => {
    const { wrapper } = mountSection('cluster', 'nodes')
    expect(wrapper.find('.mock-node-list').exists()).toBe(true)
  })
  it('cluster/namespaces renders NamespaceList', () => {
    const { wrapper } = mountSection('cluster', 'namespaces')
    expect(wrapper.find('.mock-namespace-list').exists()).toBe(true)
  })
  it('cluster/events renders EventStream', () => {
    const { wrapper } = mountSection('cluster', 'events')
    expect(wrapper.find('.mock-event-stream').exists()).toBe(true)
  })

  // --- WORKLOADS ---

  it('workloads/pods renders PodList', () => {
    const { wrapper } = mountSection('workloads', 'pods')
    expect(wrapper.find('.mock-pod-list').exists()).toBe(true)
  })
  it('workloads/deployments renders DeploymentList', () => {
    const { wrapper } = mountSection('workloads', 'deployments')
    expect(wrapper.find('.mock-deployment-list').exists()).toBe(true)
  })
  it('workloads/jobs renders JobCronJobList', () => {
    const { wrapper } = mountSection('workloads', 'jobs')
    expect(wrapper.find('.mock-job-cronjob-list').exists()).toBe(true)
  })
  it('workloads/cronjobs renders JobCronJobList', () => {
    const { wrapper } = mountSection('workloads', 'cronjobs')
    expect(wrapper.find('.mock-job-cronjob-list').exists()).toBe(true)
  })
  it('workloads/statefulsets renders StatefulDaemonSetList', () => {
    const { wrapper } = mountSection('workloads', 'statefulsets')
    expect(wrapper.find('.mock-stateful-daemonset-list').exists()).toBe(true)
  })

  // --- CONFIG ---

  it('config/configmaps renders ConfigMapList', () => {
    const { wrapper } = mountSection('config', 'configmaps')
    expect(wrapper.find('.mock-configmap-list').exists()).toBe(true)
  })
  it('config/secrets renders SecretsView', () => {
    const { wrapper } = mountSection('config', 'secrets')
    expect(wrapper.find('.mock-secrets-view').exists()).toBe(true)
  })
  it('config/hpas renders HpaList', () => {
    const { wrapper } = mountSection('config', 'hpas')
    expect(wrapper.find('.mock-hpa-list').exists()).toBe(true)
  })

  // --- NETWORK ---

  it('network/services renders ServiceList', () => {
    const { wrapper } = mountSection('network', 'services')
    expect(wrapper.find('.mock-service-list').exists()).toBe(true)
  })
  it('network/ingresses renders ServiceList', () => {
    const { wrapper } = mountSection('network', 'ingresses')
    expect(wrapper.find('.mock-service-list').exists()).toBe(true)
  })
  it('network/networkpolicies renders NetworkPolicyList', () => {
    const { wrapper } = mountSection('network', 'networkpolicies')
    expect(wrapper.find('.mock-network-policy-list').exists()).toBe(true)
  })

  // --- STORAGE ---

  it('storage/pvcs renders VolumeList', () => {
    const { wrapper } = mountSection('storage', 'pvcs')
    expect(wrapper.find('.mock-volume-list').exists()).toBe(true)
  })
  it('storage/storageclasses falls back to generic ResourceTable', () => {
    const { wrapper } = mountSection('storage', 'storageclasses')
    expect(wrapper.find('.mock-resource-table').exists()).toBe(true)
  })

  // --- OPERATIONS ---

  it('operations/runbooks renders RunbooksView', () => {
    const { wrapper } = mountSection('operations', 'runbooks')
    expect(wrapper.find('.mock-runbooks-view').exists()).toBe(true)
  })
  it('operations/incidents renders IncidentLog', () => {
    const { wrapper } = mountSection('operations', 'incidents')
    expect(wrapper.find('.mock-incident-log').exists()).toBe(true)
  })
  it('operations/audit renders ConfigAudit', () => {
    const { wrapper } = mountSection('operations', 'audit')
    expect(wrapper.find('.mock-config-audit').exists()).toBe(true)
  })
  it('operations/arguscd renders ArgusCDList', () => {
    const { wrapper } = mountSection('operations', 'arguscd')
    expect(wrapper.find('.mock-arguscd-list').exists()).toBe(true)
  })
  it('operations/pipelines renders PipelinesView', () => {
    const { wrapper } = mountSection('operations', 'pipelines')
    expect(wrapper.find('.mock-pipelines-view').exists()).toBe(true)
  })

  // --- KNOWLEDGE ---

  it('knowledge/notebooks renders S3Notebook', () => {
    const { wrapper } = mountSection('knowledge', 'notebooks')
    expect(wrapper.find('.mock-s3-notebook').exists()).toBe(true)
  })
  it('knowledge/documents renders DocumentsView', () => {
    const { wrapper } = mountSection('knowledge', 'documents')
    expect(wrapper.find('.mock-documents-view').exists()).toBe(true)
  })

  // --- ADMIN ---

  it('admin/setup renders SetupPanel inside .admin-scroll', () => {
    const { wrapper } = mountSection('admin', 'setup')
    expect(wrapper.find('.admin-scroll .mock-setup-panel').exists()).toBe(true)
  })
  it('admin/settings renders SettingsPanel inside .admin-scroll', () => {
    const { wrapper } = mountSection('admin', 'settings')
    expect(wrapper.find('.admin-scroll .mock-settings-panel').exists()).toBe(true)
  })

  // --- General invariants ---

  it('section-tabs bar appears for every section', () => {
    for (const id of ['monitoring', 'cluster', 'workloads', 'config', 'network', 'storage', 'operations', 'knowledge', 'admin']) {
      const { wrapper } = mountSection(id, undefined)
      expect(wrapper.find('[data-testid="section-tabs"]').exists()).toBe(true)
    }
  })

  it('operations section wraps content in .ops-scroll', () => {
    const { wrapper } = mountSection('operations', 'runbooks')
    expect(wrapper.find('.ops-scroll').exists()).toBe(true)
  })
})
