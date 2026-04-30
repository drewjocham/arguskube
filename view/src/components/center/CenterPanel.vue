<script setup>
import { ref, computed, reactive } from 'vue'
import MetricsRow from './MetricsRow.vue'
import AlertList from './AlertList.vue'
import LogStream from './LogStream.vue'
import TopologyMap from './TopologyMap.vue'
import ResourceTable from '../resources/ResourceTable.vue'
import ResourceDetail from '../resources/ResourceDetail.vue'
import RunbooksView from '../operations/RunbooksView.vue'
import IncidentLog from '../operations/IncidentLog.vue'
import ConfigAudit from '../operations/ConfigAudit.vue'
import WorkflowEditor from '../operations/WorkflowEditor.vue'
import LogExplorer from './LogExplorer.vue'
import AnomalyDetection from './AnomalyDetection.vue'
import MetricsExplorer from './MetricsExplorer.vue'
import S3Notebook from './S3Notebook.vue'
import NodeList from '../cluster/NodeList.vue'
import NamespaceList from '../cluster/NamespaceList.vue'
import EventStream from '../cluster/EventStream.vue'
import PodList from '../workloads/PodList.vue'
import DeploymentList from '../workloads/DeploymentList.vue'
import ConfigMapList from '../config/ConfigMapList.vue'
import ServiceList from '../network/ServiceList.vue'
import VolumeList from '../storage/VolumeList.vue'
import JobCronJobList from '../workloads/JobCronJobList.vue'
import StatefulDaemonSetList from '../workloads/StatefulDaemonSetList.vue'
import HpaList from '../config/HpaList.vue'
import NetworkPolicyList from '../network/NetworkPolicyList.vue'
import PopeyeReport from './PopeyeReport.vue'

const props = defineProps({
  metrics: { type: Object, default: null },
  alerts: { type: Array, default: () => [] },
  selectedAlert: { type: Object, default: null },
  logLines: { type: Array, default: () => [] },
  activeNav: { type: String, default: 'alerts' },
})

const emit = defineEmits(['select-alert'])

const criticalCount = computed(() => props.alerts.filter(a => a.severity === 'critical').length)
const warningCount = computed(() => props.alerts.filter(a => a.severity === 'warning').length)

const selectedResource = ref(null)

function onResourceSelect(resource) {
  selectedResource.value = resource
}

function closeDetail() {
  selectedResource.value = null
}

// Customizable layout state
const editMode = ref(false)
const widgetOrder = ref(['metrics', 'alerts', 'logs', 'topology'])

function moveUp(index) {
  if (index > 0) {
    const temp = widgetOrder.value[index]
    widgetOrder.value[index] = widgetOrder.value[index - 1]
    widgetOrder.value[index - 1] = temp
  }
}

function moveDown(index) {
  if (index < widgetOrder.value.length - 1) {
    const temp = widgetOrder.value[index]
    widgetOrder.value[index] = widgetOrder.value[index + 1]
    widgetOrder.value[index + 1] = temp
  }
}

// Monitoring views.
const monitoringViews = ['metrics', 'alerts', 'topology', 'logs', 'anomalies', 'analysis']

// Resource browser views — these use the generic ResourceTable.
const resourceViews = [
  'pods', 'deployments', 'statefulsets', 'daemonsets', 'replicasets', 'jobs', 'cronjobs',
  'services', 'endpoints', 'ingresses', 'networkpolicies',
  'configmaps', 'secrets', 'hpas',
  'pvcs', 'pvs', 'storageclasses',
  'nodes', 'namespaces', 'events',
]

// Operations views.
const operationViews = ['runbooks', 'incidents', 'audit', 'workflows']

// Knowledge views.
const knowledgeViews = ['notebooks']

const isMonitoring = computed(() => monitoringViews.includes(props.activeNav))
const isResource = computed(() => resourceViews.includes(props.activeNav))
const isOperations = computed(() => operationViews.includes(props.activeNav))
const isKnowledge = computed(() => knowledgeViews.includes(props.activeNav))

// Popeye logic
const reportData = ref(null)
const loadingReport = ref(false)

function runPopeyeScan() {
  loadingReport.value = true
  reportData.value = null
  
  setTimeout(() => {
    loadingReport.value = false
    reportData.value = {
      grade: 'B',
      score: 84,
      scanTimeMs: 1420,
      totalError: 2,
      totalWarn: 3,
      totalInfo: 5,
      totalOk: 42,
      findings: [
        {
          id: 1, severity: 'error', name: 'Root user allowed',
          resource: 'Deployment', namespace: 'default',
          message: 'Container runs as root',
          explanation: 'Running containers as root grants them excessive privileges on the host system. If a container is compromised, the attacker can break out and access the host.',
          fix: 'Set securityContext.runAsNonRoot = true in the pod spec. Ensure your Dockerfile switches to a non-root USER before the entrypoint.',
          command: 'kubectl patch deploy web-app --type="json" -p=\'[{"op": "add", "path": "/spec/template/spec/securityContext", "value": {"runAsNonRoot": true}}]\''
        },
        {
          id: 2, severity: 'error', name: 'Missing Probes',
          resource: 'StatefulSet', namespace: 'database',
          message: 'Liveness probe not defined',
          explanation: 'Without a liveness probe, Kubernetes cannot know if your application is stuck or deadlocked. It will not automatically restart a hanging pod.',
          fix: 'Define a livenessProbe in the container spec. Use an HTTP GET, TCP Socket, or Exec action to verify health.',
          command: null
        },
        {
          id: 3, severity: 'warning', name: 'CPU Limits',
          resource: 'DaemonSet', namespace: 'kube-system',
          message: 'No CPU limit configured',
          explanation: 'A container without CPU limits can consume all available CPU on a node, potentially starving other critical workloads.',
          fix: 'Specify resources.limits.cpu for the container.',
          command: 'kubectl set resources daemonset fluent-bit -c fluent-bit --limits=cpu=200m'
        }
      ]
    }
  }, 1500)
}
</script>

<template>
  <div class="content">
    <!-- Monitoring: Alerts overview -->
    <template v-if="isMonitoring">
      <template v-if="activeNav === 'metrics'">
        <MetricsExplorer />
      </template>
      <template v-else-if="activeNav === 'logs'">
        <LogExplorer />
      </template>
      <template v-else-if="activeNav === 'anomalies'">
        <AnomalyDetection />
      </template>
      <template v-else-if="activeNav === 'analysis'">
        <PopeyeReport :report="reportData" :loading="loadingReport" @run-scan="runPopeyeScan" />
      </template>
      <template v-else>
        <div class="tabs">
          <div class="tab active">Overview</div>
          <div class="tab" v-if="criticalCount > 0">
            <span class="tab-dot" style="background: var(--red);"></span>
            Critical ({{ criticalCount }})
          </div>
          <div class="tab" v-if="warningCount > 0">
            <span class="tab-dot" style="background: var(--amber);"></span>
            Warnings ({{ warningCount }})
          </div>
          <div class="tab">All Events</div>
          <div class="tab-spacer"></div>
          <div class="toolbar-btn">30m</div>
          <div class="toolbar-btn" :class="{ primary: editMode }" @click="editMode = !editMode">
            {{ editMode ? 'Done Editing' : 'Customize' }}
          </div>
          <div class="toolbar-btn primary">Diagnose All</div>
        </div>

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
              <AlertList v-else-if="widget === 'alerts'" :alerts="alerts" :selectedAlert="selectedAlert" @select="emit('select-alert', $event)" />
              <LogStream v-else-if="widget === 'logs'" :alerts="alerts" :externalLines="logLines" />
              <TopologyMap v-else-if="widget === 'topology'" :alerts="alerts" />
            </div>
          </div>
        </div>
      </template>
    </template>

    <!-- Resource browser -->
    <template v-else-if="isResource">
      <NodeList v-if="activeNav === 'nodes'" />
      <NamespaceList v-else-if="activeNav === 'namespaces'" />
      <EventStream v-else-if="activeNav === 'events'" />
      <PodList v-else-if="activeNav === 'pods'" />
      <DeploymentList v-else-if="activeNav === 'deployments'" />
      <JobCronJobList v-else-if="activeNav === 'jobs' || activeNav === 'cronjobs'" :type="activeNav" />
      <StatefulDaemonSetList v-else-if="activeNav === 'statefulsets' || activeNav === 'daemonsets' || activeNav === 'replicasets'" :type="activeNav" />
      
      <ConfigMapList v-else-if="activeNav === 'configmaps' || activeNav === 'secrets'" />
      <HpaList v-else-if="activeNav === 'hpas'" />
      
      <ServiceList v-else-if="activeNav === 'services' || activeNav === 'ingresses'" />
      <NetworkPolicyList v-else-if="activeNav === 'networkpolicies' || activeNav === 'endpoints'" :type="activeNav" />
      
      <VolumeList v-else-if="activeNav === 'pvcs' || activeNav === 'pvs'" />
      
      <div v-else class="resource-layout">
        <ResourceTable
          :resourceKind="activeNav"
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
    </template>

    <!-- Operations views -->
    <template v-else-if="isOperations">
      <div class="ops-header">
        <div class="ops-title">
          {{ activeNav === 'runbooks' ? 'Runbooks' : activeNav === 'incidents' ? 'Incident Log' : activeNav === 'audit' ? 'Config Audit' : 'Workflows' }}
        </div>
      </div>

      <div class="scroll ops-scroll">
        <RunbooksView v-if="activeNav === 'runbooks'" />
        <IncidentLog v-if="activeNav === 'incidents'" />
        <ConfigAudit v-if="activeNav === 'audit'" />
        <WorkflowEditor v-if="activeNav === 'workflows'" />
      </div>
    </template>

    <!-- Knowledge / S3 views -->
    <template v-else-if="isKnowledge">
      <S3Notebook v-if="activeNav === 'notebooks'" />
    </template>
  </div>
</template>

<style scoped>
.content { flex: 1; display: flex; flex-direction: column; overflow: hidden; }

.tabs {
  display: flex; align-items: center; height: 38px;
  border-bottom: 1px solid var(--border); background: var(--bg2);
  padding: 0 16px; gap: 2px; flex-shrink: 0;
}
.tab {
  padding: 5px 12px; font-size: 12.5px; font-weight: 400; color: var(--text2);
  cursor: pointer; border-radius: 6px; transition: all 0.1s; white-space: nowrap;
}
.tab:hover { background: var(--bg3); color: var(--text); }
.tab.active { background: rgba(79,142,247,0.12); color: var(--accent2); font-weight: 500; }
.tab-dot { display: inline-block; width: 5px; height: 5px; border-radius: 50%; margin-right: 5px; vertical-align: middle; position: relative; top: -1px; }
.tab-spacer { flex: 1; }

.toolbar-btn {
  display: flex; align-items: center; gap: 5px; padding: 5px 10px;
  border-radius: 6px; font-size: 12px; font-weight: 500; cursor: pointer;
  border: 1px solid var(--border2); background: var(--bg3); color: var(--text2);
  transition: all 0.1s; margin-left: 6px;
}
.toolbar-btn:hover { background: var(--bg4); color: var(--text); }
.toolbar-btn.primary { background: rgba(79,142,247,0.15); color: var(--accent2); border-color: rgba(79,142,247,0.3); }
.toolbar-btn.primary:hover { background: rgba(79,142,247,0.25); }

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

/* Customization Mode */
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
.editing-dim { opacity: 0.6; pointer-events: none; border: 1px dashed var(--border); border-radius: var(--r); }

/* Resource layout — table + optional detail panel */
.resource-layout {
  flex: 1;
  display: flex;
  overflow: hidden;
}

/* Operations */
.ops-header {
  height: 38px; display: flex; align-items: center; padding: 0 16px;
  border-bottom: 1px solid var(--border); background: var(--bg2); flex-shrink: 0;
}
.ops-title { font-size: 13px; font-weight: 500; color: var(--text); }

.ops-scroll { flex: 1; overflow-y: auto; padding: 14px; }
</style>
