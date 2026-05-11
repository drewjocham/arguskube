import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import CenterPanel from '../../components/center/CenterPanel.vue'

// Mock every composable that CenterPanel imports from useWails.
vi.mock('../../composables/useWails', () => ({
  useArgusScan: vi.fn(() => ({
    report: vi.fn(() => null),
    loading: vi.fn(() => false),
    error: vi.fn(() => null),
    runScan: vi.fn(),
  })),
}))

vi.mock('../../composables/useBackgroundTasks', () => ({
  useBackgroundTasks: vi.fn(() => ({
    tasks: {},
    startTask: vi.fn(),
    completeTask: vi.fn(),
    failTask: vi.fn(),
    getTask: vi.fn(() => null),
    clearTask: vi.fn(),
    hasTask: vi.fn(() => false),
    isRunning: vi.fn(() => false),
    lastResult: vi.fn(() => null),
  })),
}))

// Also mock all the child components CenterPanel conditionally renders, so we
// don't need to mock their internal composables.
vi.mock('../../components/center/MetricsRow.vue', () => ({
  default: { template: '<div class="mock-metrics-row">MetricsRow</div>' },
}))
vi.mock('../../components/center/AlertList.vue', () => ({
  default: { template: '<div class="mock-alert-list">AlertList</div>' },
}))
vi.mock('../../components/center/LogStream.vue', () => ({
  default: { template: '<div class="mock-log-stream">LogStream</div>' },
}))
vi.mock('../../components/center/TopologyMap.vue', () => ({
  default: { template: '<div class="mock-topology-map">TopologyMap</div>' },
}))
vi.mock('../../components/center/MetricsExplorer.vue', () => ({
  default: { template: '<div class="mock-metrics-explorer">MetricsExplorer</div>' },
}))
vi.mock('../../components/center/VulnerabilityList.vue', () => ({
  default: { template: '<div class="mock-vulnerability-list">VulnerabilityList</div>' },
}))
vi.mock('../../components/center/LogExplorer.vue', () => ({
  default: { template: '<div class="mock-log-explorer">LogExplorer</div>' },
}))
vi.mock('../../components/center/AnomalyDetection.vue', () => ({
  default: { template: '<div class="mock-anomaly-detection">AnomalyDetection</div>' },
}))
vi.mock('../../components/center/ArgusScanReport.vue', () => ({
  default: { template: '<div class="mock-argus-scan-report">ArgusScanReport</div>' },
}))
vi.mock('../../components/center/FinOpsView.vue', () => ({
  default: { template: '<div class="mock-finops-view">FinOpsView</div>' },
}))
vi.mock('../../components/cluster/NodeList.vue', () => ({
  default: { template: '<div class="mock-node-list">NodeList</div>' },
}))
vi.mock('../../components/cluster/NamespaceList.vue', () => ({
  default: { template: '<div class="mock-namespace-list">NamespaceList</div>' },
}))
vi.mock('../../components/cluster/EventStream.vue', () => ({
  default: { template: '<div class="mock-event-stream">EventStream</div>' },
}))
vi.mock('../../components/workloads/PodList.vue', () => ({
  default: { template: '<div class="mock-pod-list">PodList</div>' },
}))
vi.mock('../../components/workloads/DeploymentList.vue', () => ({
  default: { template: '<div class="mock-deployment-list">DeploymentList</div>' },
}))
vi.mock('../../components/workloads/JobCronJobList.vue', () => ({
  default: { template: '<div class="mock-job-cronjob-list">JobCronJobList</div>' },
}))
vi.mock('../../components/workloads/StatefulDaemonSetList.vue', () => ({
  default: { template: '<div class="mock-stateful-daemonset-list">StatefulDaemonSetList</div>' },
}))
vi.mock('../../components/config/ConfigMapList.vue', () => ({
  default: { template: '<div class="mock-configmap-list">ConfigMapList</div>' },
}))
vi.mock('../../components/config/HpaList.vue', () => ({
  default: { template: '<div class="mock-hpa-list">HpaList</div>' },
}))
vi.mock('../../components/network/ServiceList.vue', () => ({
  default: { template: '<div class="mock-service-list">ServiceList</div>' },
}))
vi.mock('../../components/network/NetworkPolicyList.vue', () => ({
  default: { template: '<div class="mock-network-policy-list">NetworkPolicyList</div>' },
}))
vi.mock('../../components/storage/VolumeList.vue', () => ({
  default: { template: '<div class="mock-volume-list">VolumeList</div>' },
}))
vi.mock('../../components/resources/ResourceTable.vue', () => ({
  default: { template: '<div class="mock-resource-table">ResourceTable</div>' },
}))
vi.mock('../../components/resources/ResourceDetail.vue', () => ({
  default: { template: '<div class="mock-resource-detail">ResourceDetail</div>' },
}))
vi.mock('../../components/operations/RunbooksView.vue', () => ({
  default: { template: '<div class="mock-runbooks-view">RunbooksView</div>' },
}))
vi.mock('../../components/operations/IncidentLog.vue', () => ({
  default: { template: '<div class="mock-incident-log">IncidentLog</div>' },
}))
vi.mock('../../components/operations/ConfigAudit.vue', () => ({
  default: { template: '<div class="mock-config-audit">ConfigAudit</div>' },
}))
vi.mock('../../components/operations/ArgusCDList.vue', () => ({
  default: { template: '<div class="mock-arguscd-list">ArgusCDList</div>' },
}))
vi.mock('../../components/center/S3Notebook.vue', () => ({
  default: { template: '<div class="mock-s3-notebook">S3Notebook</div>' },
}))
vi.mock('../../components/setup/SetupPanel.vue', () => ({
  default: { template: '<div class="mock-setup-panel">SetupPanel</div>' },
}))
vi.mock('../../components/setup/SettingsPanel.vue', () => ({
  default: { template: '<div class="mock-settings-panel">SettingsPanel</div>' },
}))

function createWrapper(props = {}) {
  return mount(CenterPanel, {
    props: {
      metrics: null,
      alerts: [],
      selectedAlert: null,
      logLines: [],
      activeNav: 'alerts',
      ...props,
    },
  })
}

describe('CenterPanel.vue — Integration', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('renders the alert overview when activeNav is "alerts"', () => {
    const wrapper = createWrapper({ activeNav: 'alerts' })
    // The "Overview" tab should be rendered.
    expect(wrapper.text()).toContain('Overview')
    // The MetricsRow widget should be in the default widget order.
    expect(wrapper.find('.mock-metrics-row').exists()).toBe(true)
    expect(wrapper.find('.mock-alert-list').exists()).toBe(true)
  })

  it('routes activeNav="nodes" to NodeList component', () => {
    const wrapper = createWrapper({ activeNav: 'nodes' })
    expect(wrapper.find('.mock-node-list').exists()).toBe(true)
  })

  it('routes activeNav="metrics" to MetricsExplorer', () => {
    const wrapper = createWrapper({ activeNav: 'metrics' })
    expect(wrapper.find('.mock-metrics-explorer').exists()).toBe(true)
  })

  it('routes activeNav="pods" to PodList', () => {
    const wrapper = createWrapper({ activeNav: 'pods' })
    expect(wrapper.find('.mock-pod-list').exists()).toBe(true)
  })

  it('routes activeNav="deployments" to DeploymentList', () => {
    const wrapper = createWrapper({ activeNav: 'deployments' })
    expect(wrapper.find('.mock-deployment-list').exists()).toBe(true)
  })

  it('routes activeNav="jobs" to JobCronJobList', () => {
    const wrapper = createWrapper({ activeNav: 'jobs' })
    expect(wrapper.find('.mock-job-cronjob-list').exists()).toBe(true)
  })

  it('routes activeNav="cronjobs" to JobCronJobList', () => {
    const wrapper = createWrapper({ activeNav: 'cronjobs' })
    expect(wrapper.find('.mock-job-cronjob-list').exists()).toBe(true)
  })

  it('routes activeNav="statefulsets" to StatefulDaemonSetList', () => {
    const wrapper = createWrapper({ activeNav: 'statefulsets' })
    expect(wrapper.find('.mock-stateful-daemonset-list').exists()).toBe(true)
  })

  it('routes activeNav="configmaps" to ConfigMapList', () => {
    const wrapper = createWrapper({ activeNav: 'configmaps' })
    expect(wrapper.find('.mock-configmap-list').exists()).toBe(true)
  })

  it('routes activeNav="services" to ServiceList', () => {
    const wrapper = createWrapper({ activeNav: 'services' })
    expect(wrapper.find('.mock-service-list').exists()).toBe(true)
  })

  it('routes activeNav="networkpolicies" to NetworkPolicyList', () => {
    const wrapper = createWrapper({ activeNav: 'networkpolicies' })
    expect(wrapper.find('.mock-network-policy-list').exists()).toBe(true)
  })

  it('routes activeNav="pvcs" to VolumeList', () => {
    const wrapper = createWrapper({ activeNav: 'pvcs' })
    expect(wrapper.find('.mock-volume-list').exists()).toBe(true)
  })

  it('routes activeNav="runbooks" to RunbooksView', () => {
    const wrapper = createWrapper({ activeNav: 'runbooks' })
    expect(wrapper.find('.mock-runbooks-view').exists()).toBe(true)
  })

  it('routes activeNav="incidents" to IncidentLog', () => {
    const wrapper = createWrapper({ activeNav: 'incidents' })
    expect(wrapper.find('.mock-incident-log').exists()).toBe(true)
  })

  it('routes activeNav="audit" to ConfigAudit', () => {
    const wrapper = createWrapper({ activeNav: 'audit' })
    expect(wrapper.find('.mock-config-audit').exists()).toBe(true)
  })

  it('routes activeNav="arguscd" to ArgusCDList', () => {
    const wrapper = createWrapper({ activeNav: 'arguscd' })
    expect(wrapper.find('.mock-arguscd-list').exists()).toBe(true)
  })

  it('routes activeNav="notebooks" to S3Notebook', () => {
    const wrapper = createWrapper({ activeNav: 'notebooks' })
    expect(wrapper.find('.mock-s3-notebook').exists()).toBe(true)
  })

  it('routes activeNav="setup" to SetupPanel', () => {
    const wrapper = createWrapper({ activeNav: 'setup' })
    expect(wrapper.find('.mock-setup-panel').exists()).toBe(true)
  })

  it('routes activeNav="settings" to SettingsPanel', () => {
    const wrapper = createWrapper({ activeNav: 'settings' })
    expect(wrapper.find('.mock-settings-panel').exists()).toBe(true)
  })

  it('routes activeNav="topology" to TopologyMap with alerts', () => {
    const wrapper = createWrapper({ activeNav: 'topology' })
    expect(wrapper.text()).toContain('Service Topology')
    // TopologyMap mock is rendered.
    expect(wrapper.find('.mock-topology-map').exists()).toBe(true)
  })

  it('routes activeNav="analysis" to ArgusScanReport', () => {
    const wrapper = createWrapper({ activeNav: 'analysis' })
    expect(wrapper.find('.mock-argus-scan-report').exists()).toBe(true)
  })

  it('routes activeNav="finops" to FinOpsView', () => {
    const wrapper = createWrapper({ activeNav: 'finops' })
    expect(wrapper.find('.mock-finops-view').exists()).toBe(true)
  })

  it('renders admin-scroll wrapper for admin views', () => {
    const wrapper = createWrapper({ activeNav: 'setup' })
    expect(wrapper.find('.admin-scroll').exists()).toBe(true)
  })

  it('renders ops-scroll for operations views', () => {
    const wrapper = createWrapper({ activeNav: 'runbooks' })
    expect(wrapper.find('.ops-scroll').exists()).toBe(true)
  })
})
