<script setup>
import { ref, computed, inject, onMounted, onUnmounted } from 'vue'
import { useContexts } from '../../composables/useWails'

const props = defineProps({
  clusterInfo: { type: Object, default: null },
  alerts: { type: Array, default: () => [] },
  activeNav: { type: String, default: 'alerts' },
})

const emit = defineEmits(['update:activeNav', 'context-switched'])

const isAllowed = inject('isAllowed')

// Context switching.
const { contexts, loading: ctxLoading, switching, listContexts, switchContext } = useContexts()
const ctxDropdownOpen = ref(false)

onMounted(() => {
  listContexts()
})

async function onSwitchContext(name) {
  await switchContext(name)
  ctxDropdownOpen.value = false
  emit('context-switched', name)
}

function toggleCtxDropdown() {
  ctxDropdownOpen.value = !ctxDropdownOpen.value
  if (ctxDropdownOpen.value) listContexts()
}

// Close dropdown on outside click.
function onDocClick(e) {
  if (ctxDropdownOpen.value && !e.target.closest('.cluster-area')) {
    ctxDropdownOpen.value = false
  }
}
onMounted(() => document.addEventListener('click', onDocClick))
onUnmounted(() => document.removeEventListener('click', onDocClick))

// Track which sections are collapsed.
const collapsed = ref({})

function toggleSection(id) {
  collapsed.value[id] = !collapsed.value[id]
}

function isCollapsed(id) {
  return !!collapsed.value[id]
}

// Navigation tree definition.
const navTree = [
  {
    id: 'monitoring',
    label: 'Monitoring',
    items: [
      { id: 'metrics', label: 'Metrics Explorer' },
      { id: 'alerts', label: 'Alerts' },
      { id: 'topology', label: 'Topology' },
      { id: 'vulnerabilities', label: 'Vulnerabilities' },
      { id: 'logs', label: 'Logs' },
      { id: 'anomalies', label: 'Anomaly Detection' },
      { id: 'analysis', label: 'Analysis' },
    ],
  },
  {
    id: 'cluster',
    label: 'Cluster',
    items: [
      { id: 'nodes', label: 'Nodes' },
      { id: 'namespaces', label: 'Namespaces' },
      { id: 'events', label: 'Events' },
    ],
  },
  {
    id: 'workloads',
    label: 'Workloads',
    items: [
      { id: 'pods', label: 'Pods' },
      { id: 'deployments', label: 'Deployments' },
      { id: 'statefulsets', label: 'StatefulSets' },
      { id: 'daemonsets', label: 'DaemonSets' },
      { id: 'replicasets', label: 'ReplicaSets' },
      { id: 'jobs', label: 'Jobs' },
      { id: 'cronjobs', label: 'Cron Jobs' },
    ],
  },
  {
    id: 'config',
    label: 'Config',
    items: [
      { id: 'configmaps', label: 'Config Maps' },
      { id: 'secrets', label: 'Secrets' },
      { id: 'hpas', label: 'HPAs' },
    ],
  },
  {
    id: 'network',
    label: 'Network',
    items: [
      { id: 'services', label: 'Services' },
      { id: 'endpoints', label: 'Endpoints' },
      { id: 'ingresses', label: 'Ingresses' },
      { id: 'networkpolicies', label: 'Network Policies' },
    ],
  },
  {
    id: 'storage',
    label: 'Storage',
    items: [
      { id: 'pvcs', label: 'Volume Claims' },
      { id: 'pvs', label: 'Volumes' },
      { id: 'storageclasses', label: 'Storage Classes' },
    ],
  },
  {
    id: 'operations',
    label: 'Operations',
    items: [
      { id: 'runbooks', label: 'Runbooks' },
      { id: 'incidents', label: 'Incident Log' },
      { id: 'audit', label: 'Config Audit' },
      { id: 'arguscd', label: 'ArgusCD', pro: true },
    ],
  },
  {
    id: 'knowledge',
    label: 'Knowledge',
    items: [
      { id: 'notebooks', label: 'Notebooks & S3' },
    ],
  },
  {
    id: 'admin',
    label: 'Admin',
    items: [
      { id: 'setup', label: 'Setup & Tools' },
    ],
  },
]

const criticalCount = computed(() =>
  props.alerts.filter(a => a.severity === 'critical').length
)
const warningCount = computed(() =>
  props.alerts.filter(a => a.severity === 'warning').length
)
</script>

<template>
  <div class="sidebar">
    <!-- Cluster selector -->
    <div class="cluster-area">
      <div class="cluster-selector" @click.stop="toggleCtxDropdown" :class="{ open: ctxDropdownOpen }">
        <div class="cluster-icon">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
            <circle cx="7" cy="7" r="5" stroke="white" stroke-width="1.5"/>
            <circle cx="7" cy="7" r="2" fill="white"/>
          </svg>
        </div>
        <div class="cluster-info">
          <div class="cluster-name">{{ clusterInfo?.name || '—' }}</div>
          <div class="cluster-sub">{{ clusterInfo?.nodeCount || 0 }} nodes · {{ clusterInfo?.k8sVersion || '—' }}</div>
        </div>
        <svg class="chevron-down" :class="{ flipped: ctxDropdownOpen }" width="10" height="10" viewBox="0 0 10 10">
          <path d="M3 4l2 2.5L7 4" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linecap="round"/>
        </svg>
      </div>

      <!-- Context dropdown -->
      <div v-if="ctxDropdownOpen" class="ctx-dropdown">
        <div class="ctx-dropdown-header">Kubernetes Contexts</div>
        <div v-if="ctxLoading" class="ctx-loading">Loading…</div>
        <div v-else class="ctx-list">
          <div
            v-for="ctx in contexts"
            :key="ctx.name"
            class="ctx-item"
            :class="{ active: ctx.active, switching: switching }"
            @click.stop="onSwitchContext(ctx.name)"
          >
            <div class="ctx-dot" :class="{ active: ctx.active }"></div>
            <div class="ctx-details">
              <div class="ctx-name">{{ ctx.name }}</div>
              <div class="ctx-cluster">{{ ctx.cluster }}</div>
            </div>
            <div v-if="ctx.active" class="ctx-badge">active</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Navigation tree -->
    <div class="nav-scroll">
      <template v-for="section in navTree" :key="section.id">
        <div class="section-header" @click="toggleSection(section.id)">
          <svg class="section-chevron" :class="{ open: !isCollapsed(section.id) }" width="8" height="8" viewBox="0 0 8 8">
            <path d="M2 1.5l3 2.5-3 2.5" stroke="currentColor" stroke-width="1.2" fill="none" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
          <span class="section-label">{{ section.label }}</span>
        </div>

        <div v-show="!isCollapsed(section.id)" class="section-items">
          <template v-for="item in section.items" :key="item.id">
            <!-- Pro-gated items: show with lock badge if not allowed -->
            <div
              v-if="!item.pro || isAllowed(item.id)"
              class="nav-item"
              :class="{ active: activeNav === item.id }"
              @click="emit('update:activeNav', item.id)"
            >
              <div class="nav-dot" :class="{ active: activeNav === item.id }"></div>
              <span class="nav-label">{{ item.label }}</span>
              <span v-if="item.id === 'alerts' && criticalCount > 0" class="badge badge-red">{{ criticalCount }}</span>
              <span v-if="item.id === 'alerts' && warningCount > 0 && criticalCount === 0" class="badge badge-amber">{{ warningCount }}</span>
            </div>
            <div
              v-else
              class="nav-item pro-locked"
            >
              <div class="nav-dot"></div>
              <span class="nav-label">{{ item.label }}</span>
              <span class="pro-badge">PRO</span>
            </div>
          </template>
        </div>
      </template>
    </div>

    <!-- AI Context card -->
    <div class="ai-context-card">
      <div class="ai-context-header">
        <div class="ai-dot"></div>
        Argus Context
      </div>
      <div class="ai-context-body">
        {{ alerts.length }} active alerts · 12h window
      </div>
      <div class="ai-context-action" v-if="isAllowed('runbook_automation')">
        Attach runbook →
      </div>
      <div class="ai-context-action pro-label" v-else>
        PRO: Attach runbook
      </div>
    </div>
  </div>
</template>

<style scoped>
.sidebar {
  width: 230px;
  background: var(--bg2);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow: hidden;
}

.cluster-area { padding: 10px 10px 6px; }

.cluster-selector {
  padding: 8px 10px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: var(--r2);
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: border-color 0.15s, background 0.15s;
}
.cluster-selector:hover { border-color: rgba(255,255,255,0.18); background: var(--bg4); }

.cluster-icon {
  width: 24px; height: 24px; border-radius: 6px;
  background: linear-gradient(135deg, var(--accent) 0%, var(--purple) 100%);
  display: flex; align-items: center; justify-content: center; flex-shrink: 0;
}
.cluster-info { flex: 1; min-width: 0; }
.cluster-name { font-size: 12px; font-weight: 500; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.cluster-sub { font-size: 10px; color: var(--text3); margin-top: 1px; }
.chevron-down { color: var(--text3); flex-shrink: 0; transition: transform 0.2s ease; }
.chevron-down.flipped { transform: rotate(180deg); }

.cluster-selector.open { border-color: var(--accent); background: var(--bg4); }

/* Context dropdown */
.cluster-area { position: relative; }

.ctx-dropdown {
  position: absolute;
  top: calc(100% + 2px);
  left: 10px;
  right: 10px;
  background: var(--bg3);
  border: 1px solid var(--border2);
  border-radius: var(--r2);
  box-shadow: 0 8px 24px rgba(0,0,0,0.4);
  z-index: 100;
  overflow: hidden;
  animation: ctx-slide 0.15s ease-out;
}
@keyframes ctx-slide { from { opacity: 0; transform: translateY(-4px); } to { opacity: 1; transform: translateY(0); } }

.ctx-dropdown-header {
  padding: 8px 12px 6px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text3);
  border-bottom: 1px solid var(--border);
}
.ctx-loading { padding: 12px; font-size: 11px; color: var(--text3); }

.ctx-list { max-height: 200px; overflow-y: auto; }

.ctx-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  transition: background 0.1s;
}
.ctx-item:hover { background: var(--bg4); }
.ctx-item.active { background: rgba(79,142,247,0.08); }
.ctx-item.switching { opacity: 0.5; pointer-events: none; }

.ctx-dot { width: 6px; height: 6px; border-radius: 50%; background: var(--text3); flex-shrink: 0; }
.ctx-dot.active { background: var(--accent); }

.ctx-details { flex: 1; min-width: 0; }
.ctx-name { font-size: 12px; color: var(--text); font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ctx-cluster { font-size: 10px; color: var(--text3); margin-top: 1px; }

.ctx-badge {
  font-size: 9px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--accent);
  background: rgba(79,142,247,0.1);
  padding: 2px 6px;
  border-radius: 4px;
  flex-shrink: 0;
}

/* Scrollable nav */
.nav-scroll {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 4px 0 8px;
}

/* Section headers */
.section-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px 4px;
  cursor: pointer;
  user-select: none;
}
.section-header:hover .section-label { color: var(--text2); }

.section-label {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.06em;
  color: var(--text3);
  text-transform: uppercase;
  transition: color 0.15s;
}

.section-chevron {
  color: var(--text3);
  transition: transform 0.2s ease;
  flex-shrink: 0;
}
.section-chevron.open { transform: rotate(90deg); }

/* Nav items */
.nav-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 12px 5px 26px;
  cursor: pointer;
  transition: background 0.1s, color 0.1s;
  color: var(--text2);
  font-size: 12.5px;
  font-weight: 400;
  position: relative;
}
.nav-item:hover { background: var(--bg3); color: var(--text); }
.nav-item.active { background: rgba(79,142,247,0.08); color: var(--accent2); }
.nav-item.active::before {
  content: '';
  position: absolute;
  left: 0; top: 3px; bottom: 3px;
  width: 2px;
  background: var(--accent);
  border-radius: 0 2px 2px 0;
}

.nav-dot {
  width: 4px; height: 4px; border-radius: 50%;
  background: var(--text3);
  flex-shrink: 0;
  transition: background 0.15s;
}
.nav-dot.active { background: var(--accent); }

.nav-label {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* AI Context */
.ai-context-card {
  margin: 6px 10px 10px;
  background: rgba(79,142,247,0.06);
  border: 1px solid rgba(79,142,247,0.15);
  border-radius: 8px;
  padding: 9px 10px;
  flex-shrink: 0;
}
.ai-context-header { font-size: 10px; font-weight: 600; color: var(--accent2); margin-bottom: 3px; display: flex; align-items: center; gap: 4px; }
.ai-dot { width: 5px; height: 5px; border-radius: 50%; background: var(--accent); flex-shrink: 0; }
.ai-context-body { font-size: 10.5px; color: var(--text3); line-height: 1.5; }
.ai-context-action { margin-top: 5px; font-size: 10.5px; color: var(--accent2); cursor: pointer; }
.ai-context-action.pro-label { color: var(--purple); opacity: 0.5; cursor: default; }

/* Pro-locked nav items */
.nav-item.pro-locked { opacity: 0.4; cursor: default; }
.nav-item.pro-locked:hover { background: transparent; color: var(--text2); }
.pro-badge {
  font-size: 8px; font-weight: 700; letter-spacing: 0.06em;
  color: var(--purple); background: rgba(167, 139, 250, 0.12);
  padding: 1px 4px; border-radius: 3px; margin-left: auto;
}
</style>
