<script setup>
import { ref, computed } from 'vue'
import MetricsRow from './MetricsRow.vue'
import AlertList from './AlertList.vue'
import LogStream from './LogStream.vue'
import TopologyMap from './TopologyMap.vue'
import ResourceTable from '../resources/ResourceTable.vue'
import ResourceDetail from '../resources/ResourceDetail.vue'
import RunbooksView from '../operations/RunbooksView.vue'
import IncidentLog from '../operations/IncidentLog.vue'
import ConfigAudit from '../operations/ConfigAudit.vue'

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

// Resource detail selection state.
const selectedResource = ref(null)

function onResourceSelect(resource) {
  selectedResource.value = resource
}

function closeDetail() {
  selectedResource.value = null
}

// Monitoring views.
const monitoringViews = ['alerts', 'topology', 'logs']

// Resource browser views — these use the generic ResourceTable.
const resourceViews = [
  'pods', 'deployments', 'statefulsets', 'daemonsets', 'replicasets', 'jobs', 'cronjobs',
  'services', 'endpoints', 'ingresses', 'networkpolicies',
  'configmaps', 'secrets', 'hpas',
  'pvcs', 'pvs', 'storageclasses',
  'nodes', 'namespaces', 'events',
]

// Operations views.
const operationViews = ['runbooks', 'incidents', 'audit']

const isMonitoring = computed(() => monitoringViews.includes(props.activeNav))
const isResource = computed(() => resourceViews.includes(props.activeNav))
const isOperations = computed(() => operationViews.includes(props.activeNav))
</script>

<template>
  <div class="content">
    <!-- Monitoring: Alerts overview -->
    <template v-if="isMonitoring">
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
        <div class="toolbar-btn primary">Diagnose All</div>
      </div>

      <div class="ctx-strip">
        <div class="ctx-label">Context</div>
        <div class="ctx-chip"><div class="ctx-dot" style="background: var(--green);"></div>live cluster</div>
        <div class="ctx-chip"><div class="ctx-dot" style="background: var(--amber);"></div>{{ alerts.length }} alerts</div>
        <div class="ctx-chip"><div class="ctx-dot" style="background: var(--accent);"></div>DECISION_LOG.md</div>
      </div>

      <div class="scroll">
        <MetricsRow :metrics="metrics" />
        <AlertList :alerts="alerts" :selectedAlert="selectedAlert" @select="emit('select-alert', $event)" />
        <LogStream :alerts="alerts" :externalLines="logLines" />
        <TopologyMap :alerts="alerts" />
      </div>
    </template>

    <!-- Resource browser -->
    <template v-else-if="isResource">
      <div class="resource-layout">
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
          {{ activeNav === 'runbooks' ? 'Runbooks' : activeNav === 'incidents' ? 'Incident Log' : 'Config Audit' }}
        </div>
      </div>

      <div class="scroll ops-scroll">
        <RunbooksView v-if="activeNav === 'runbooks'" />
        <IncidentLog v-if="activeNav === 'incidents'" />
        <ConfigAudit v-if="activeNav === 'audit'" />
      </div>
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
