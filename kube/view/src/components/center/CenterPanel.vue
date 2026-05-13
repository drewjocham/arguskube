<script setup>
import { ref, computed, watch } from 'vue'
import { storeToRefs } from 'pinia'
import Select from '../common/Select.vue'
import MetricsRow from './MetricsRow.vue'
import AlertList from './AlertList.vue'
import LogStream from './LogStream.vue'
import ResourceTable from '../resources/ResourceTable.vue'
import ResourceDetail from '../resources/ResourceDetail.vue'
import RunbooksView from '../operations/RunbooksView.vue'
import IncidentLog from '../operations/IncidentLog.vue'
import ConfigAudit from '../operations/ConfigAudit.vue'
import ArgusCDList from '../operations/ArgusCDList.vue'
import PipelinesView from '../operations/PipelinesView.vue'
import LogExplorer from './LogExplorer.vue'
import AnomalyDetection from './AnomalyDetection.vue'
import MetricsExplorer from './MetricsExplorer.vue'
import VulnerabilityList from './VulnerabilityList.vue'
import S3Notebook from './S3Notebook.vue'
import AlertsView from '../alerts/AlertsView.vue'
import DocumentsView from '../knowledge/DocumentsView.vue'
import NodeList from '../cluster/NodeList.vue'
import NamespaceList from '../cluster/NamespaceList.vue'
import EventStream from '../cluster/EventStream.vue'
import PodList from '../workloads/PodList.vue'
import DeploymentList from '../workloads/DeploymentList.vue'
import ConfigMapList from '../config/ConfigMapList.vue'
import SecretsView from '../config/SecretsView.vue'
import ServiceList from '../network/ServiceList.vue'
import VolumeList from '../storage/VolumeList.vue'
import JobCronJobList from '../workloads/JobCronJobList.vue'
import StatefulDaemonSetList from '../workloads/StatefulDaemonSetList.vue'
import WorkloadBuilder from '../workloads/WorkloadBuilder.vue'
import HpaList from '../config/HpaList.vue'
import NetworkPolicyList from '../network/NetworkPolicyList.vue'
import ExternalBridge from '../network/ExternalBridge.vue'
import LabelMatcher from '../network/LabelMatcher.vue'
import EndpointTopology from '../network/EndpointTopology.vue'
import RouteTopologyGraph from '../gateway/RouteTopologyGraph.vue'
import StatusDashboard from '../gateway/StatusDashboard.vue'
import IngressMigration from '../gateway/IngressMigration.vue'
import TrafficSplitter from '../gateway/TrafficSplitter.vue'
import ArgusScanReport from './ArgusScanReport.vue'
import FinOpsView from './FinOpsView.vue'
import LogEventCorrelator from '../diagnostics/LogEventCorrelator.vue'
import WasteHeatmap from './WasteHeatmap.vue'
import RBACView from './RBACView.vue'
import SetupPanel from '../setup/SetupPanel.vue'
import SettingsPanel from '../setup/SettingsPanel.vue'
import ArgusAIChat from '../ai/ArgusAIChat.vue'
import SectionTabs from '../shared/SectionTabs.vue'
import { useArgusScan } from '../../composables/useWails'
import { useBackgroundTasks } from '../../composables/useBackgroundTasks'
import { useAppNavStore } from '../../stores/appNav'
import { useSectionTabsStore } from '../../stores/sectionTabs'
import { SECTIONS } from '../../lib/sectionTabs'

// CenterPanel — section-based router. Receives `activeNav` (a section
// id) and renders a <SectionTabs> bar at the top plus the matching
// content based on the section's saved active tab.
//
// The section dispatcher is a single v-if/v-else-if chain on
// `activeNav`. Inside each section the per-tab dispatch is its own
// v-if chain on `sectionTabs.activeTab(sectionId)`. This is the same
// shape as before — only the OUTER grouping (5 wide buckets →
// 9 sections) and the new tab bar are new.
//
// The "Alerts" tab inside Monitoring keeps its OWN inner sub-tabs
// (Cluster Alerts vs Manage Alerts) because that's two distinct
// dashboards over the same data and adding them as siblings of the
// other Monitoring tabs would confuse the model. One layer of
// nesting is OK; we just don't do more.

const { report: argusScanReport, loading: argusScanLoading, error: argusScanError, runScan: runArgusScanReal } = useArgusScan()
const { startTask, completeTask, failTask, getTask } = useBackgroundTasks()
const ARGUS_SCAN_KEY = 'argus-scan'

const props = defineProps({
  metrics: { type: Object, default: null },
  alerts: { type: Array, default: () => [] },
  selectedAlert: { type: Object, default: null },
  logLines: { type: Array, default: () => [] },
  // activeNav is now a SECTION id (monitoring | cluster | workloads | …).
  activeNav: { type: String, default: 'monitoring' },
})

const emit = defineEmits(['select-alert', 'diagnose-all'])

// Alerts dashboard time window (was a hard-coded "30m" label).
const alertsRange = ref('30m')

// "Diagnose All" — fire-and-forget bulk diagnosis. The parent owns
// the AI agent client; we just emit progress so the button is honest
// about what's happening.
const diagnosingAll = ref(false)
const diagnoseProgress = ref('')

async function onDiagnoseAll() {
  if (!props.alerts?.length || diagnosingAll.value) return
  diagnosingAll.value = true
  diagnoseProgress.value = `0/${props.alerts.length}`
  try {
    // App.vue listens for this and awaits its diagnose flow per alert;
    // we just expose progress via the prop callback shape the existing
    // diagnose() supports. We don't await here — the parent's queue
    // handles it, but we surface a count so the user has feedback.
    let i = 0
    for (const a of props.alerts) {
      emit('diagnose-all', a)
      i++
      diagnoseProgress.value = `${i}/${props.alerts.length}`
      // Tiny yield so the parent's diagnose loop can interleave + the
      // UI updates the counter instead of locking.
      await new Promise(r => setTimeout(r, 50))
    }
  } finally {
    diagnosingAll.value = false
    diagnoseProgress.value = ''
  }
}

const criticalCount = computed(() => props.alerts.filter(a => a.severity === 'critical').length)
const warningCount = computed(() => props.alerts.filter(a => a.severity === 'warning').length)

const sectionTabsStore = useSectionTabsStore()
const { tabs: sectionTabValues } = storeToRefs(sectionTabsStore)

// Helper: which tab is active for the current section?
const currentTab = computed(() => sectionTabValues.value[props.activeNav] || '')

function setTab(sectionId, tabId) {
  sectionTabsStore.setTab(sectionId, tabId)
}

// Resource detail side panel (used by the generic-table fallback for
// storage classes and any future kinds not covered by a dedicated list).
const selectedResource = ref(null)
function onResourceSelect(resource) { selectedResource.value = resource }
function closeDetail() { selectedResource.value = null }

// Customizable Alerts dashboard layout.
const editMode = ref(false)
const widgetOrder = ref(['metrics', 'alerts', 'logs'])
function moveUp(index) {
  if (index > 0) {
    const tmp = widgetOrder.value[index]
    widgetOrder.value[index] = widgetOrder.value[index - 1]
    widgetOrder.value[index - 1] = tmp
  }
}
function moveDown(index) {
  if (index < widgetOrder.value.length - 1) {
    const tmp = widgetOrder.value[index]
    widgetOrder.value[index] = widgetOrder.value[index + 1]
    widgetOrder.value[index + 1] = tmp
  }
}

async function runArgusScan() {
  startTask(ARGUS_SCAN_KEY)
  try {
    await runArgusScanReal()
    if (argusScanReport.value) completeTask(ARGUS_SCAN_KEY, argusScanReport.value)
  } catch (e) {
    failTask(ARGUS_SCAN_KEY, e?.message || String(e))
  }
}

const reportData = computed(() => {
  if (argusScanReport.value) return argusScanReport.value
  const stored = getTask(ARGUS_SCAN_KEY)
  return stored?.status === 'completed' ? stored.result : null
})
const loadingReport = computed(() => argusScanLoading.value)

// "Alerts" tab → two sub-tabs. Default to watchers (managed alerts) so
// the first thing users see is what's been silenced/escalating, not the
// raw cluster overview.
const alertsTab = ref('watchers')
const appNav = useAppNavStore()
watch(() => appNav.pending, (req) => {
  // External requests pointing at an alerts silence/watcher anchor
  // should also switch the Monitoring section's active tab to alerts.
  if (!req) return
  if (req.navId === 'alerts' || (req.anchor && (req.anchor.startsWith('silence:') || req.anchor.startsWith('watcher:')))) {
    if (props.activeNav === 'monitoring') {
      sectionTabsStore.setTab('monitoring', 'alerts')
    }
    if (req.anchor && (req.anchor.startsWith('silence:') || req.anchor.startsWith('watcher:'))) {
      alertsTab.value = 'watchers'
    }
  }
}, { immediate: true })

// --- Section-tab helpers exposed to the template
const monitoringTabs = SECTIONS.monitoring.tabs
const clusterTabs = SECTIONS.cluster.tabs
const workloadsTabs = SECTIONS.workloads.tabs
const configTabs = SECTIONS.config.tabs
const networkTabs = SECTIONS.network.tabs
const gatewayTabs = SECTIONS.gateway.tabs
const storageTabs = SECTIONS.storage.tabs
const operationsTabs = SECTIONS.operations.tabs
const knowledgeTabs = SECTIONS.knowledge.tabs
const adminTabs = SECTIONS.admin.tabs
</script>

<template>
  <div class="content">
    <!-- ============ MONITORING ============ -->
    <template v-if="activeNav === 'monitoring'">
      <SectionTabs
        :tabs="monitoringTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('monitoring', $event)"
      />
      <ArgusAIChat v-if="currentTab === 'argusai'" />
      <MetricsExplorer v-else-if="currentTab === 'metrics'" />
      <VulnerabilityList v-else-if="currentTab === 'vulnerabilities'" />
      <LogExplorer v-else-if="currentTab === 'logs'" />
      <AnomalyDetection v-else-if="currentTab === 'anomalies'" />
      <ArgusScanReport
        v-else-if="currentTab === 'analysis'"
        :report="reportData"
        :loading="loadingReport"
        :error="argusScanError"
        @run-scan="runArgusScan"
      />
      <FinOpsView v-else-if="currentTab === 'finops'" />
      <LogEventCorrelator v-else-if="currentTab === 'correlator'" />
      <WasteHeatmap v-else-if="currentTab === 'waste'" />
      <!-- Alerts tab → sub-tabs (cluster overview vs managed watchers) -->
      <template v-else-if="currentTab === 'alerts'">
        <div class="tabs sub-tabs">
          <div class="tab" :class="{ active: alertsTab === 'cluster' }" @click="alertsTab = 'cluster'">
            Cluster Alerts
          </div>
          <div class="tab" :class="{ active: alertsTab === 'watchers' }" @click="alertsTab = 'watchers'">
            Manage Alerts
          </div>
          <div class="tab-spacer"></div>
          <template v-if="alertsTab === 'cluster'">
            <div class="tab-meta" v-if="criticalCount > 0">
              <span class="tab-dot" style="background: var(--red);"></span>
              {{ criticalCount }} critical
            </div>
            <div class="tab-meta" v-if="warningCount > 0">
              <span class="tab-dot" style="background: var(--amber);"></span>
              {{ warningCount }} warning
            </div>
            <!-- Time-range selector (was an inert "30m" label) -->
            <Select
              v-model="alertsRange"
              :options="[{value:'15m',label:'15m'},{value:'30m',label:'30m'},{value:'1h',label:'1h'},{value:'6h',label:'6h'},{value:'24h',label:'24h'}]"
              size="sm"
              aria-label="Alerts time window"
            />
            <button
              type="button"
              class="toolbar-btn"
              :class="{ primary: editMode }"
              @click="editMode = !editMode"
            >
              {{ editMode ? 'Done Editing' : 'Customize' }}
            </button>
            <!-- Diagnose All — emits up so App.vue can drive AI
                 diagnosis across every visible alert. -->
            <button
              type="button"
              class="toolbar-btn primary"
              :disabled="!alerts.length || diagnosingAll"
              :title="alerts.length ? 'Run AI diagnostics on every visible alert' : 'No alerts to diagnose'"
              @click="onDiagnoseAll"
            >
              {{ diagnosingAll ? `Diagnosing… (${diagnoseProgress})` : 'Diagnose All' }}
            </button>
          </template>
        </div>
        <template v-if="alertsTab === 'cluster'">
          <div class="ctx-strip">
            <div class="ctx-label">Context</div>
            <div class="ctx-chip"><div class="ctx-dot" style="background: var(--green);"></div>live cluster</div>
            <div class="ctx-chip"><div class="ctx-dot" style="background: var(--amber);"></div>{{ alerts.length }} alerts</div>
            <div class="ctx-chip"><div class="ctx-dot" style="background: var(--accent);"></div>DECISION_LOG.md</div>
          </div>
          <div class="scroll" :class="{ 'is-editing': editMode }">
            <div v-for="(widget, index) in widgetOrder" :key="widget" class="widget-wrapper">
              <div v-if="editMode" class="widget-controls">
                <span class="widget-name">{{ widget.toUpperCase() }}</span>
                <div class="widget-actions">
                  <button class="ctrl-btn" @click="moveUp(index)" :disabled="index === 0">↑</button>
                  <button class="ctrl-btn" @click="moveDown(index)" :disabled="index === widgetOrder.length - 1">↓</button>
                </div>
              </div>
              <div class="widget-content" :class="{ 'editing-dim': editMode }">
                <MetricsRow v-if="widget === 'metrics'" :metrics="metrics" />
                <AlertList
                  v-else-if="widget === 'alerts'"
                  :alerts="alerts"
                  :selectedAlert="selectedAlert"
                  @select="emit('select-alert', $event)"
                />
                <LogStream
                  v-else-if="widget === 'logs'"
                  :alerts="alerts"
                  :externalLines="logLines"
                />
              </div>
            </div>
          </div>
        </template>
        <template v-else-if="alertsTab === 'watchers'">
          <AlertsView />
        </template>
      </template>
    </template>

    <!-- ============ CLUSTER ============ -->
    <template v-else-if="activeNav === 'cluster'">
      <SectionTabs
        :tabs="clusterTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('cluster', $event)"
      />
      <div class="resource-scroll-area">
        <NodeList v-if="currentTab === 'nodes'" />
        <NamespaceList v-else-if="currentTab === 'namespaces'" />
        <EventStream v-else-if="currentTab === 'events'" />
      </div>
    </template>

    <!-- ============ WORKLOADS ============ -->
    <template v-else-if="activeNav === 'workloads'">
      <SectionTabs
        :tabs="workloadsTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('workloads', $event)"
      />
      <div class="resource-scroll-area">
        <PodList v-if="currentTab === 'pods'" />
        <DeploymentList v-else-if="currentTab === 'deployments'" />
        <StatefulDaemonSetList
          v-else-if="currentTab === 'statefulsets' || currentTab === 'daemonsets' || currentTab === 'replicasets'"
          :type="currentTab"
        />
        <JobCronJobList
          v-else-if="currentTab === 'jobs' || currentTab === 'cronjobs'"
          :type="currentTab"
        />
        <WorkloadBuilder v-else-if="currentTab === 'builder'" />
      </div>
    </template>

    <!-- ============ CONFIG ============ -->
    <template v-else-if="activeNav === 'config'">
      <SectionTabs
        :tabs="configTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('config', $event)"
      />
      <div class="resource-scroll-area">
        <ConfigMapList
          v-if="currentTab === 'configmaps'"
          :type="currentTab"
        />
        <SecretsView v-else-if="currentTab === 'secrets'" />
        <HpaList v-else-if="currentTab === 'hpas'" />
      </div>
    </template>

    <!-- ============ NETWORK ============ -->
    <template v-else-if="activeNav === 'network'">
      <SectionTabs
        :tabs="networkTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('network', $event)"
      />
      <div class="resource-scroll-area">
        <ServiceList
          v-if="currentTab === 'services' || currentTab === 'ingresses'"
          :type="currentTab"
        />
        <NetworkPolicyList
          v-else-if="currentTab === 'networkpolicies' || currentTab === 'endpoints'"
          :type="currentTab"
        />
        <ExternalBridge v-else-if="currentTab === 'external-bridges'" />
        <LabelMatcher v-else-if="currentTab === 'label-matcher'" />
        <EndpointTopology v-else-if="currentTab === 'endpoint-topology'" />
      </div>
    </template>

    <!-- ============ GATEWAY ============ -->
    <template v-else-if="activeNav === 'gateway'">
      <SectionTabs
        :tabs="gatewayTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('gateway', $event)"
      />
      <div class="resource-scroll-area">
        <RouteTopologyGraph v-if="currentTab === 'topology'" />
        <StatusDashboard v-else-if="currentTab === 'status'" />
        <IngressMigration v-else-if="currentTab === 'migration'" />
        <TrafficSplitter v-else-if="currentTab === 'traffic'" />
      </div>
    </template>

    <!-- ============ STORAGE ============ -->
    <template v-else-if="activeNav === 'storage'">
      <SectionTabs
        :tabs="storageTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('storage', $event)"
      />
      <div class="resource-scroll-area">
        <VolumeList
          v-if="currentTab === 'pvcs' || currentTab === 'pvs'"
          :type="currentTab"
        />
        <div v-else-if="currentTab === 'storageclasses'" class="resource-layout">
          <ResourceTable
            resource-kind="storageclasses"
            @select="onResourceSelect"
          />
          <ResourceDetail
            v-if="selectedResource"
            :kind="selectedResource.kind"
            :namespace="selectedResource.namespace"
            :name="selectedResource.name"
            @close="closeDetail"
          />
        </div>
      </div>
    </template>

    <!-- ============ OPERATIONS ============ -->
    <template v-else-if="activeNav === 'operations'">
      <SectionTabs
        :tabs="operationsTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('operations', $event)"
      />
      <div class="scroll ops-scroll">
        <RunbooksView v-if="currentTab === 'runbooks'" />
        <IncidentLog v-else-if="currentTab === 'incidents'" />
        <ConfigAudit v-else-if="currentTab === 'audit'" />
        <ArgusCDList v-else-if="currentTab === 'arguscd'" />
        <PipelinesView v-else-if="currentTab === 'pipelines'" />
      </div>
    </template>

    <!-- ============ KNOWLEDGE ============ -->
    <template v-else-if="activeNav === 'knowledge'">
      <SectionTabs
        :tabs="knowledgeTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('knowledge', $event)"
      />
      <S3Notebook v-if="currentTab === 'notebooks'" />
      <DocumentsView v-else-if="currentTab === 'documents'" />
    </template>

    <!-- ============ ADMIN ============ -->
    <template v-else-if="activeNav === 'admin'">
      <SectionTabs
        :tabs="adminTabs"
        :active-tab="currentTab"
        @update:active-tab="setTab('admin', $event)"
      />
      <div class="admin-scroll">
        <SetupPanel v-if="currentTab === 'setup'" />
        <SettingsPanel v-else-if="currentTab === 'settings'" />
        <RBACView v-else-if="currentTab === 'rbac'" />
      </div>
    </template>
  </div>
</template>

<style scoped>
/* min-height: 0 lets sections with internal scroll (ArgusAIChat, LogExplorer
   etc.) actually scroll. Without it, a tall child stretches this column
   and the descendant overflow-y never engages. */
.content { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }

/* Sub-tabs row used inside the Monitoring → Alerts tab. Same metrics
   as the SectionTabs bar so the visual rhythm matches. */
.tabs.sub-tabs {
  display: flex; align-items: center; height: 38px;
  border-bottom: 1px solid var(--border); background: var(--bg2);
  padding: 0 16px; gap: 2px; flex-shrink: 0;
}
.tabs.sub-tabs .tab {
  padding: 5px 12px; font-size: 12.5px; font-weight: 400; color: var(--text2);
  cursor: pointer; border-radius: 6px; transition: all 0.1s; white-space: nowrap;
}
.tabs.sub-tabs .tab:hover { background: var(--bg3); color: var(--text); }
.tabs.sub-tabs .tab.active { background: rgba(79,142,247,0.12); color: var(--accent2); font-weight: 500; }
.tabs.sub-tabs .tab-dot { display: inline-block; width: 5px; height: 5px; border-radius: 50%; margin-right: 5px; vertical-align: middle; position: relative; top: -1px; }
.tabs.sub-tabs .tab-spacer { flex: 1; }
.tabs.sub-tabs .tab-meta {
  display: flex; align-items: center;
  font-size: 11px; color: var(--text3);
  margin: 0 6px;
  white-space: nowrap;
}

.toolbar-btn {
  display: flex; align-items: center; gap: 5px; padding: 5px 10px;
  border-radius: 6px; font-size: 12px; font-weight: 500; cursor: pointer;
  border: 1px solid var(--border2); background: var(--bg3); color: var(--text2);
  transition: all 0.1s; margin-left: 6px;
}
.toolbar-btn:hover { background: var(--bg4); color: var(--text); }
.toolbar-btn.primary { background: rgba(79,142,247,0.15); color: var(--accent2); border-color: rgba(79,142,247,0.3); }
.toolbar-btn.primary:hover { background: rgba(79,142,247,0.25); }
.toolbar-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.toolbar-btn:disabled:hover { background: var(--bg3); color: var(--text2); }

.ctx-strip {
  padding: 6px 12px; border-bottom: 1px solid var(--border);
  display: flex; gap: 5px; flex-wrap: wrap; align-items: center;
}
.ctx-label { font-size: 10px; color: var(--text3); margin-right: 2px; text-transform: uppercase; letter-spacing: 0.05em; }
.ctx-chip {
  display: flex; align-items: center; gap: 4px; padding: 2px 8px;
  border-radius: 20px; font-size: 10.5px;
  border: 1px solid var(--border2); background: var(--bg3); color: var(--text2);
}
.ctx-dot { width: 5px; height: 5px; border-radius: 50%; }

.scroll { flex: 1; overflow-y: auto; padding: 14px; display: flex; flex-direction: column; gap: 12px; }

.widget-wrapper { position: relative; display: flex; flex-direction: column; gap: 6px; }
.widget-controls {
  display: flex; align-items: center; justify-content: space-between;
  background: var(--bg3); border: 1px dashed var(--accent); border-radius: 6px;
  padding: 6px 12px;
}
.widget-name { font-size: 11px; font-weight: 600; color: var(--accent2); letter-spacing: 0.05em; }
.widget-actions { display: flex; gap: 4px; }
.ctrl-btn {
  background: var(--bg2); border: 1px solid var(--border); color: var(--text2);
  width: 24px; height: 24px; border-radius: 4px; cursor: pointer; display: flex; align-items: center; justify-content: center;
  transition: all 0.1s;
}
.ctrl-btn:hover:not(:disabled) { background: var(--accent); color: white; border-color: var(--accent); }
.ctrl-btn:disabled { opacity: 0.3; cursor: not-allowed; }

.admin-scroll { flex: 1; overflow-y: auto; display: flex; flex-direction: column; min-height: 0; }
.editing-dim { opacity: 0.6; pointer-events: none; border: 1px dashed var(--border); border-radius: var(--r); }

.resource-scroll-area { flex: 1; overflow-y: auto; min-height: 0; display: flex; flex-direction: column; }
.resource-layout { flex: 1; display: flex; overflow: hidden; }

.ops-scroll { flex: 1; overflow-y: auto; padding: 14px; }
</style>
