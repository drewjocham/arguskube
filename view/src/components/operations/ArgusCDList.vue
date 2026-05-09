<script setup>
import { ref, computed, onMounted, inject, watch } from 'vue'
import { useArgusCD } from '../../composables/useWails'
import ProGateOverlay from '../shared/ProGateOverlay.vue'

const isAllowed = inject('isAllowed')

const {
  apps, selectedApp, resources, diffs, projects, status, loading, error,
  fetchStatus, listApps, listProjects, getApp, getResources, getDiffs,
  syncApp, refreshApp, rollbackApp, testConnection,
} = useArgusCD()

const syncing = ref(null)
const rollingBack = ref(null)
const notification = ref(null)
const treeCollapsed = ref(false)
const driftCollapsed = ref(false)
const historyCollapsed = ref(false)
const projectFilter = ref('')

onMounted(async () => {
  await fetchStatus()
  await Promise.all([listApps(projectFilter.value), listProjects()])
})

watch(projectFilter, async (val) => {
  clearSelection()
  await listApps(val)
})

function syncStatusColor(s) {
  return s === 'Synced' ? '#10b981' : s === 'OutOfSync' ? '#f5a623' : '#8b8f96'
}

function healthStatusColor(s) {
  switch (s) {
    case 'Healthy': return '#10b981'
    case 'Degraded': return '#ef4444'
    case 'Progressing': return '#3b82f6'
    case 'Missing': return '#f5a623'
    default: return '#8b8f96'
  }
}

function resourceIcon(kind) {
  const map = {
    Deployment: '⚙️', Service: '🌐', Pod: '🚀', ReplicaSet: '📦',
    ConfigMap: '📋', Secret: '🔒', Ingress: '🔀', StatefulSet: '🗄️',
    DaemonSet: '🔄', Job: '⏱️', CronJob: '🕐', PersistentVolumeClaim: '💾',
    ServiceAccount: '👤', HorizontalPodAutoscaler: '📈', NetworkPolicy: '🛡️',
  }
  return map[kind] || '📄'
}

async function onSelectApp(app) {
  selectedApp.value = app
  // Load resources and diffs in parallel.
  await Promise.all([
    getResources(app.name),
    getDiffs(app.name),
  ])
}

function clearSelection() {
  selectedApp.value = null
  resources.value = []
  diffs.value = []
}

async function onSync(app, event) {
  if (event) event.stopPropagation()
  syncing.value = app.name
  try {
    const result = await syncApp(app.name)
    notification.value = { type: 'success', text: `${app.name}: ${result?.message || 'Sync triggered'}` }
  } catch (e) {
    notification.value = { type: 'error', text: `Sync failed: ${e?.message || e}` }
  } finally {
    syncing.value = null
    setTimeout(() => { notification.value = null }, 4000)
  }
}

async function onRefresh(app) {
  await refreshApp(app.name, false)
}

async function onRollback(app, entry) {
  if (!entry || typeof entry.id !== 'number') {
    notification.value = { type: 'error', text: 'Selected revision has no ID — cannot rollback.' }
    setTimeout(() => { notification.value = null }, 4000)
    return
  }
  const shortRev = entry.revision ? entry.revision.slice(0, 8) : `id ${entry.id}`
  if (!confirm(`Rollback ${app.name} to revision ${shortRev}?`)) return

  rollingBack.value = entry.id
  try {
    await rollbackApp(app.name, entry.id)
    notification.value = { type: 'success', text: `${app.name}: rollback to ${shortRev} triggered` }
    if (selectedApp.value && selectedApp.value.name === app.name) {
      await getApp(app.name)
    }
  } catch (e) {
    notification.value = { type: 'error', text: `Rollback failed: ${e?.message || e}` }
  } finally {
    rollingBack.value = null
    setTimeout(() => { notification.value = null }, 4000)
  }
}

function formatHistoryDate(ts) {
  if (!ts) return '—'
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ts
  return d.toLocaleString()
}

const filteredResources = computed(() => {
  if (!resources.value) return []
  // Group by kind.
  const groups = {}
  for (const r of resources.value) {
    if (!groups[r.kind]) groups[r.kind] = []
    groups[r.kind].push(r)
  }
  return groups
})

const outOfSyncCount = computed(() => {
  return apps.value.filter(a => a.syncStatus === 'OutOfSync').length
})
const degradedCount = computed(() => {
  return apps.value.filter(a => a.healthStatus === 'Degraded').length
})
</script>

<template>
  <div class="argus-view" style="position: relative;">
    <ProGateOverlay
      v-if="!isAllowed('arguscd')"
      feature="arguscd"
      title="ArgusCD"
      description="Continuous deployment management is available with KubeWatcher Pro. Deploy, sync, and monitor your applications from a single pane."
    />

    <!-- Notification -->
    <div v-if="notification" class="notification" :class="notification.type">
      {{ notification.text }}
    </div>

    <div class="header">
      <div class="title-row">
        <div>
          <div class="title">ArgusCD</div>
          <div class="subtitle">
            <template v-if="status?.connected">
              Connected to <span class="mono">{{ status.url }}</span>
            </template>
            <template v-else>
              {{ status?.message || 'Checking Argo CD connection…' }}
            </template>
          </div>
        </div>
        <div class="header-actions">
          <div class="stats" v-if="apps.length > 0">
            <span class="stat">{{ apps.length }} apps</span>
            <span class="stat warn" v-if="outOfSyncCount > 0">{{ outOfSyncCount }} out of sync</span>
            <span class="stat crit" v-if="degradedCount > 0">{{ degradedCount }} degraded</span>
          </div>
          <select
            v-if="projects && projects.length > 0"
            v-model="projectFilter"
            class="project-filter"
            :title="'Filter by Argo CD project'"
          >
            <option value="">All projects</option>
            <option v-for="p in projects" :key="p" :value="p">{{ p }}</option>
          </select>
          <button class="refresh-btn" @click="listApps(projectFilter)" :disabled="loading">
            {{ loading ? '…' : '↻' }} Refresh
          </button>
        </div>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading && apps.length === 0" class="empty-state">Loading applications…</div>

    <!-- Error -->
    <div v-else-if="error" class="empty-state error">{{ error }}</div>

    <!-- No apps -->
    <div v-else-if="apps.length === 0 && !loading" class="empty-state">
      <div class="empty-icon">📦</div>
      <div>No applications found</div>
      <div class="empty-hint" v-if="!status?.connected">
        Configure Argo CD connection in <strong>Settings → ArgusCD</strong> to see your applications.
        <br>Without Argo CD, this view shows Kubernetes deployments as applications.
      </div>
    </div>

    <!-- App Grid -->
    <div v-else-if="!selectedApp" class="app-grid">
      <div v-for="app in apps" :key="app.name" class="app-card" @click="onSelectApp(app)">
        <div class="card-header">
          <div class="app-info">
            <svg class="app-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><circle cx="8.5" cy="8.5" r="1.5"></circle><polyline points="21 15 16 10 5 21"></polyline></svg>
            <div>
              <div class="app-name">{{ app.name }}</div>
              <div class="app-project">{{ app.project || app.namespace }}</div>
            </div>
          </div>

          <button class="sync-action-btn" @click="onSync(app, $event)" :disabled="syncing === app.name">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"></polyline><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path></svg>
            {{ syncing === app.name ? 'SYNCING…' : 'SYNC' }}
          </button>
        </div>

        <div class="status-row">
          <div class="status-box">
            <div class="status-label">SYNC STATUS</div>
            <div class="status-val" :style="{ color: syncStatusColor(app.syncStatus) }">
              <span class="status-dot" :style="{ background: syncStatusColor(app.syncStatus) }"></span>
              {{ app.syncStatus || 'Unknown' }}
            </div>
          </div>
          <div class="status-box">
            <div class="status-label">HEALTH STATUS</div>
            <div class="status-val" :style="{ color: healthStatusColor(app.healthStatus) }">
               <span class="status-icon">
                 <svg v-if="app.healthStatus === 'Healthy'" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="20 6 9 17 4 12"></polyline></svg>
                 <svg v-else-if="app.healthStatus === 'Degraded'" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
                 <svg v-else width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline></svg>
               </span>
              {{ app.healthStatus || 'Unknown' }}
            </div>
          </div>
        </div>

        <div class="meta-grid">
          <div class="meta-item" v-if="app.repoUrl">
            <div class="meta-icon">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"></path></svg>
            </div>
            <div class="meta-content">
              <div class="meta-val repo-url">{{ app.repoUrl }}</div>
              <div class="meta-path" v-if="app.path">{{ app.path }} @ {{ app.targetRevision || 'HEAD' }}</div>
            </div>
          </div>

          <div class="meta-item">
            <div class="meta-icon">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path></svg>
            </div>
            <div class="meta-content">
              <div class="meta-val">{{ app.destServer || 'in-cluster' }}</div>
              <div class="meta-path">ns: {{ app.destNamespace || app.namespace }}</div>
            </div>
          </div>

          <div class="meta-item" v-if="app.image">
            <div class="meta-icon">🐳</div>
            <div class="meta-content">
              <div class="meta-val mono">{{ app.image }}</div>
            </div>
          </div>
        </div>

        <div class="card-footer">
          <div>Last Sync: <span style="color: #ccc;">{{ app.lastSync || '—' }}</span></div>
          <div v-if="app.replicas !== undefined">{{ app.readyReplicas }}/{{ app.replicas }} ready</div>
        </div>
      </div>
    </div>

    <!-- Application Detail View -->
    <div v-else class="app-detail">
      <div class="detail-header">
        <button class="back-btn" @click="clearSelection()">← Back</button>
        <div class="detail-title">
          <span class="detail-name">{{ selectedApp.name }}</span>
          <span class="status-dot" :style="{ background: syncStatusColor(selectedApp.syncStatus), marginLeft: '12px' }"></span>
          <span style="font-size: 13px; font-weight: 600;" :style="{ color: syncStatusColor(selectedApp.syncStatus) }">{{ selectedApp.syncStatus }}</span>
          <span class="status-dot" :style="{ background: healthStatusColor(selectedApp.healthStatus), marginLeft: '12px' }"></span>
          <span style="font-size: 13px; font-weight: 600;" :style="{ color: healthStatusColor(selectedApp.healthStatus) }">{{ selectedApp.healthStatus }}</span>
        </div>
        <div class="detail-actions">
          <button class="sync-action-btn" @click="onRefresh(selectedApp)">↻ REFRESH</button>
          <button class="sync-action-btn primary" @click="onSync(selectedApp, null)" :disabled="syncing === selectedApp.name">
            {{ syncing === selectedApp.name ? 'SYNCING…' : 'SYNC' }}
          </button>
        </div>
      </div>

      <!-- Content Split: Resource Tree & Drift View -->
      <div class="detail-content">
        <!-- Resource Tree (Left) -->
        <div class="tree-panel" :class="{ collapsed: treeCollapsed }">
          <div class="panel-header clickable" @click="treeCollapsed = !treeCollapsed">
            <svg class="collapse-chevron" :class="{ rotated: treeCollapsed }" width="10" height="10" viewBox="0 0 10 10"><polyline points="2 3 5 7 8 3" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
            Resource Tree <span class="badge" v-if="resources.length">{{ resources.length }}</span>
          </div>
          <template v-if="!treeCollapsed">
          <div class="resource-tree" v-if="resources.length > 0">
            <template v-for="(items, kind) in filteredResources" :key="kind">
              <div class="tree-group">
                <div class="tree-group-header">{{ kind }} ({{ items.length }})</div>
                <div v-for="r in items" :key="r.name" class="tree-resource">
                  <span class="tree-icon">{{ resourceIcon(r.kind) }}</span>
                  <span class="tree-name">{{ r.namespace ? r.namespace + '/' : '' }}{{ r.name }}</span>
                  <span class="tree-health" :style="{ color: healthStatusColor(r.health || 'Unknown') }">
                    {{ r.health || '—' }}
                  </span>
                  <span class="tree-sync" v-if="r.status" :style="{ color: syncStatusColor(r.status) }">
                    {{ r.status }}
                  </span>
                </div>
              </div>
            </template>
          </div>
          <div class="tree-canvas" v-else>
            <!-- Fallback visual tree for non-Argo CD mode -->
            <div class="tree-node root-node">
              <div class="node-icon">📦</div>
              <div class="node-text">{{ selectedApp.name }}</div>
            </div>
            <div class="tree-lines vertical-line"></div>
            <div class="tree-level">
              <div class="tree-branch">
                <div class="tree-node service-node">
                  <div class="node-icon">🌐</div>
                  <div class="node-text">Service</div>
                  <div class="node-status healthy"></div>
                </div>
              </div>
              <div class="tree-branch">
                <div class="tree-node deploy-node" :class="{ 'drifted': selectedApp.syncStatus === 'OutOfSync' }">
                  <div class="node-icon">⚙️</div>
                  <div class="node-text">Deployment</div>
                  <div class="node-status" :class="selectedApp.syncStatus === 'OutOfSync' ? 'degraded' : 'healthy'"></div>
                </div>
                <div class="tree-lines vertical-line"></div>
                <div class="tree-level pods-level">
                  <div class="tree-node pod-node" v-for="i in Math.min(selectedApp.replicas || 2, 4)" :key="i">
                    <div class="node-icon">🚀</div>
                    <div class="node-text">Pod-{{ i }}</div>
                    <div class="node-status" :class="i <= (selectedApp.readyReplicas || 0) ? 'healthy' : 'degraded'"></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          </template>
        </div>

        <!-- Drift Detection Panel (Right) -->
        <div class="drift-panel" :class="{ collapsed: driftCollapsed }">
          <div class="panel-header clickable" :class="{ 'drift-alert': diffs.length > 0 || selectedApp.syncStatus === 'OutOfSync' }" @click="driftCollapsed = !driftCollapsed">
            <svg class="collapse-chevron" :class="{ rotated: driftCollapsed }" width="10" height="10" viewBox="0 0 10 10"><polyline points="2 3 5 7 8 3" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
            Drift Detection
            <span v-if="diffs.length > 0" class="drift-badge">{{ diffs.length }} Drift{{ diffs.length > 1 ? 's' : '' }} Detected</span>
            <span v-else-if="selectedApp.syncStatus === 'OutOfSync'" class="drift-badge">Out of Sync</span>
          </div>

          <template v-if="!driftCollapsed">
          <!-- Real diffs from Argo CD -->
          <template v-if="diffs.length > 0">
            <div v-for="(d, idx) in diffs" :key="idx" class="diff-viewer">
              <div class="diff-file">{{ d.resource }}</div>
              <div class="diff-content">
                <pre class="diff-pre">{{ d.diff || '(full diff not available — live ≠ target)' }}</pre>
              </div>
            </div>
            <div class="drift-explanation">
              <strong>Live state differs from Git!</strong><br>
              Click <strong>SYNC</strong> to perform self-healing and revert the cluster back to the Git source of truth.
            </div>
          </template>

          <!-- Out of sync but no detailed diffs (fallback mode) -->
          <template v-else-if="selectedApp.syncStatus === 'OutOfSync'">
            <div class="diff-viewer">
              <div class="diff-file">deployment.yaml</div>
              <div class="diff-content">
                <div class="diff-line unchanged">  spec:</div>
                <div class="diff-line unchanged">    containers:</div>
                <div class="diff-line minus">-     replicas: {{ selectedApp.readyReplicas }}</div>
                <div class="diff-line plus">+     replicas: {{ selectedApp.replicas }}</div>
              </div>
            </div>
            <div class="drift-explanation">
              <strong>Deployment not fully rolled out.</strong><br>
              {{ selectedApp.readyReplicas }}/{{ selectedApp.replicas }} replicas are ready.
              Click <strong>SYNC</strong> to restart the rollout.
            </div>
          </template>

          <!-- All synced -->
          <div class="diff-viewer empty" v-else>
            <div class="empty-icon">✓</div>
            <div>Cluster is fully synced{{ status?.connected ? ' with Git' : '' }}.</div>
            <div style="font-size: 11px; color: var(--text3); margin-top: 4px;">No configuration drift detected.</div>
          </div>
          </template>
        </div>
      </div>

      <!-- Revision history & rollback -->
      <div class="history-panel" :class="{ collapsed: historyCollapsed }">
        <div class="panel-header clickable" @click="historyCollapsed = !historyCollapsed">
          <svg class="collapse-chevron" :class="{ rotated: historyCollapsed }" width="10" height="10" viewBox="0 0 10 10"><polyline points="2 3 5 7 8 3" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
          Revision History
          <span class="badge" v-if="selectedApp.history && selectedApp.history.length">{{ selectedApp.history.length }}</span>
        </div>
        <template v-if="!historyCollapsed">
          <div v-if="selectedApp.history && selectedApp.history.length > 0" class="history-list">
            <div
              v-for="(h, idx) in selectedApp.history"
              :key="h.id"
              class="history-entry"
            >
              <div class="history-meta">
                <div class="history-rev mono">{{ (h.revision || '').slice(0, 12) || '—' }}</div>
                <div class="history-time">{{ formatHistoryDate(h.deployedAt) }}</div>
                <div class="history-source mono" v-if="h.source">{{ h.source }}</div>
              </div>
              <div class="history-actions">
                <span v-if="idx === 0" class="current-tag">Current</span>
                <button
                  v-else
                  class="rollback-btn"
                  :disabled="rollingBack === h.id"
                  @click="onRollback(selectedApp, h)"
                >
                  {{ rollingBack === h.id ? 'Rolling back…' : 'Rollback to this' }}
                </button>
              </div>
            </div>
          </div>
          <div v-else class="history-empty">
            No revision history available{{ status?.connected ? '' : ' — connect ArgusCD to see deploy history' }}.
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.argus-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title-row { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 4px; }
.header .title { font-size: 24px; font-weight: 600; color: #fff; letter-spacing: -0.02em; }
.header .subtitle { font-size: 13px; color: #8b8f96; margin-top: 2px; }
.header .subtitle .mono { font-family: var(--mono); color: var(--text2); }

.header-actions { display: flex; align-items: center; gap: 12px; }
.stats { display: flex; gap: 8px; }
.stat { font-size: 11px; padding: 3px 8px; border-radius: 4px; background: rgba(255,255,255,0.06); color: var(--text2); }
.stat.warn { background: rgba(245,166,35,0.15); color: #f5a623; }
.stat.crit { background: rgba(239,68,68,0.15); color: #ef4444; }

.refresh-btn {
  background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: var(--text2);
  padding: 6px 12px; border-radius: 6px; font-size: 12px; cursor: pointer; transition: all 0.15s;
}
.refresh-btn:hover { background: rgba(255,255,255,0.12); color: var(--text); }

.project-filter {
  background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1);
  color: var(--text2);
  padding: 6px 28px 6px 10px; border-radius: 6px;
  font-size: 12px; cursor: pointer;
  transition: all 0.15s;
  appearance: none;
  background-image: url("data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' width='10' height='10' viewBox='0 0 10 10' fill='none' stroke='%238b8f96' stroke-width='1.4'><polyline points='2 3 5 7 8 3' stroke-linecap='round' stroke-linejoin='round'/></svg>");
  background-repeat: no-repeat;
  background-position: right 8px center;
}
.project-filter:hover { background-color: rgba(255,255,255,0.12); color: var(--text); }
.project-filter:focus { outline: none; border-color: rgba(167,139,250,0.5); }

/* History panel */
.history-panel {
  margin-top: 16px;
  background: #1e2023;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: 8px;
  overflow: hidden;
}
.history-panel .panel-header {
  padding: 10px 14px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text2);
  display: flex; align-items: center; gap: 8px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
}
.history-panel .panel-header.clickable { cursor: pointer; user-select: none; }
.history-panel .panel-header.clickable:hover { background: rgba(255,255,255,0.03); color: var(--text); }
.history-panel .badge {
  background: rgba(167,139,250,0.15); color: #a78bfa;
  border-radius: 10px;
  padding: 1px 8px; font-size: 11px;
}
.history-list { display: flex; flex-direction: column; }
.history-entry {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 14px;
  border-bottom: 1px solid rgba(255,255,255,0.04);
  gap: 12px;
}
.history-entry:last-child { border-bottom: none; }
.history-meta {
  display: flex; flex-direction: column; gap: 2px;
  flex: 1; min-width: 0;
}
.history-rev { font-size: 12px; color: #e8eaec; font-weight: 500; }
.history-time { font-size: 11px; color: #8b8f96; }
.history-source { font-size: 11px; color: #6b7078; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.history-actions { display: flex; align-items: center; gap: 8px; flex-shrink: 0; }
.current-tag {
  font-size: 10px; text-transform: uppercase; letter-spacing: 0.04em;
  background: rgba(16,185,129,0.15); color: #10b981;
  border: 1px solid rgba(16,185,129,0.25);
  padding: 3px 8px; border-radius: 4px;
}
.rollback-btn {
  background: rgba(245,166,35,0.12);
  border: 1px solid rgba(245,166,35,0.3);
  color: #f5a623;
  padding: 5px 10px; border-radius: 5px; cursor: pointer;
  font-size: 11px; font-weight: 500;
  transition: all 0.15s;
}
.rollback-btn:hover { background: rgba(245,166,35,0.2); color: #fbbf24; }
.rollback-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.history-empty {
  padding: 16px; color: #8b8f96; font-size: 13px; text-align: center;
}

.notification {
  position: fixed; top: 60px; right: 20px; z-index: 100;
  padding: 10px 16px; border-radius: 8px; font-size: 12px; font-weight: 500;
  animation: slideIn 0.2s ease-out;
}
.notification.success { background: rgba(16,185,129,0.15); border: 1px solid rgba(16,185,129,0.3); color: #10b981; }
.notification.error { background: rgba(239,68,68,0.15); border: 1px solid rgba(239,68,68,0.3); color: #ef4444; }
@keyframes slideIn { from { transform: translateX(20px); opacity: 0; } to { transform: translateX(0); opacity: 1; } }

.empty-state { flex: 1; display: flex; flex-direction: column; align-items: center; justify-content: center; color: var(--text3); font-size: 14px; gap: 8px; }
.empty-state.error { color: var(--red); }
.empty-icon { font-size: 32px; margin-bottom: 8px; }
.empty-hint { font-size: 12px; color: var(--text3); text-align: center; line-height: 1.5; margin-top: 4px; }

.app-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 16px; }

.app-card {
  background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px;
  display: flex; flex-direction: column; overflow: hidden; transition: transform 0.15s, box-shadow 0.15s; cursor: pointer;
}
.app-card:hover { transform: translateY(-2px); box-shadow: 0 8px 16px rgba(0,0,0,0.4); border-color: rgba(255,255,255,0.15); }

.card-header { display: flex; justify-content: space-between; align-items: flex-start; padding: 16px; border-bottom: 1px solid rgba(255,255,255,0.04); }
.app-info { display: flex; gap: 12px; align-items: center; }
.app-icon { width: 32px; height: 32px; color: #a78bfa; background: rgba(167, 139, 250, 0.1); padding: 6px; border-radius: 6px; }
.app-name { font-size: 15px; font-weight: 600; color: #fff; margin-bottom: 2px; }
.app-project { font-size: 11px; color: #8b8f96; font-family: var(--mono); text-transform: uppercase; }

.sync-action-btn {
  display: flex; align-items: center; gap: 4px; padding: 4px 8px; border-radius: 4px;
  background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #e8eaec;
  font-size: 10px; font-weight: 700; letter-spacing: 0.05em; cursor: pointer; transition: background 0.15s;
}
.sync-action-btn:hover { background: rgba(255,255,255,0.12); }
.sync-action-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.status-row { display: grid; grid-template-columns: 1fr 1fr; border-bottom: 1px solid rgba(255,255,255,0.04); }
.status-box { padding: 12px 16px; display: flex; flex-direction: column; gap: 6px; }
.status-box:first-child { border-right: 1px solid rgba(255,255,255,0.04); }
.status-label { font-size: 10px; color: #8b8f96; font-weight: 600; letter-spacing: 0.05em; }
.status-val { font-size: 13px; font-weight: 600; display: flex; align-items: center; gap: 6px; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; display: inline-block; }
.status-icon { display: flex; align-items: center; }

.meta-grid { display: flex; flex-direction: column; padding: 16px; gap: 12px; }
.meta-item { display: flex; gap: 10px; align-items: flex-start; }
.meta-icon { color: #8b8f96; margin-top: 2px; }
.meta-content { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.meta-val { font-size: 12px; color: #e8eaec; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.meta-val.mono { font-family: var(--mono); font-size: 11px; }
.repo-url { color: #60a5fa; font-family: var(--mono); font-size: 11px; }
.meta-path { font-size: 11px; color: #8b8f96; }

.card-footer { padding: 10px 16px; background: rgba(0,0,0,0.2); font-size: 11px; color: #8b8f96; display: flex; justify-content: space-between; border-top: 1px solid rgba(255,255,255,0.04); }

/* Detail View */
.app-detail { display: flex; flex-direction: column; height: 100%; gap: 16px; }
.detail-header { display: flex; align-items: center; padding: 16px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.back-btn { background: transparent; border: 1px solid rgba(255,255,255,0.1); color: #fff; padding: 6px 12px; border-radius: 6px; cursor: pointer; font-size: 13px; font-weight: 500; margin-right: 20px; transition: background 0.15s; }
.back-btn:hover { background: rgba(255,255,255,0.05); }
.detail-title { flex: 1; display: flex; align-items: center; }
.detail-name { font-size: 20px; font-weight: 600; color: #fff; }
.detail-actions { display: flex; gap: 8px; }
.sync-action-btn.primary { background: rgba(16, 185, 129, 0.15); border-color: rgba(16, 185, 129, 0.4); color: #10b981; padding: 8px 16px; font-size: 12px; }
.sync-action-btn.primary:hover { background: rgba(16, 185, 129, 0.25); }

.detail-content { display: flex; gap: 16px; flex: 1; min-height: 0; }
.tree-panel, .drift-panel { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; flex: 1; display: flex; flex-direction: column; overflow: hidden; transition: flex 0.2s ease; }
.tree-panel.collapsed, .drift-panel.collapsed { flex: 0 0 auto; }
.panel-header { padding: 12px 16px; font-size: 13px; font-weight: 600; border-bottom: 1px solid rgba(255,255,255,0.04); color: #fff; letter-spacing: 0.02em; display: flex; align-items: center; gap: 8px; }
.panel-header.clickable { cursor: pointer; transition: background 0.15s; }
.panel-header.clickable:hover { background: rgba(255,255,255,0.04); }
.collapse-chevron { flex-shrink: 0; transition: transform 0.2s ease; color: var(--text3); }
.collapse-chevron.rotated { transform: rotate(-90deg); }
.panel-header.drift-alert { background: rgba(245, 166, 35, 0.1); color: #f5a623; display: flex; justify-content: space-between; align-items: center; }
.drift-badge { font-size: 10px; padding: 2px 6px; background: rgba(245, 166, 35, 0.2); border-radius: 4px; color: #f5a623; }
.badge { font-size: 10px; padding: 1px 6px; background: rgba(255,255,255,0.1); border-radius: 4px; color: var(--text2); }

/* Resource Tree */
.resource-tree { flex: 1; overflow-y: auto; padding: 12px; }
.tree-group { margin-bottom: 12px; }
.tree-group-header { font-size: 11px; font-weight: 600; color: var(--text3); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 6px; padding: 0 4px; }
.tree-resource { display: flex; align-items: center; gap: 8px; padding: 6px 8px; border-radius: 4px; transition: background 0.1s; }
.tree-resource:hover { background: rgba(255,255,255,0.04); }
.tree-icon { font-size: 14px; width: 20px; text-align: center; }
.tree-name { flex: 1; font-size: 12px; color: var(--text); font-family: var(--mono); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.tree-health { font-size: 10px; font-weight: 600; }
.tree-sync { font-size: 10px; font-weight: 600; }

/* Fallback visual tree */
.tree-canvas { padding: 40px; display: flex; flex-direction: column; align-items: center; flex: 1; overflow: auto; background: var(--bg2); }
.tree-node { background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.1); padding: 10px 16px; border-radius: 6px; display: flex; align-items: center; gap: 8px; position: relative; width: 140px; box-shadow: 0 4px 6px rgba(0,0,0,0.2); }
.tree-node.root-node { border-color: #a78bfa; background: rgba(167, 139, 250, 0.05); }
.tree-node.drifted { border-color: #f5a623; background: rgba(245, 166, 35, 0.05); }
.node-icon { font-size: 16px; }
.node-text { font-size: 12px; font-weight: 500; color: #e8eaec; flex: 1; text-align: left; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.node-status { width: 8px; height: 8px; border-radius: 50%; }
.node-status.healthy { background: #10b981; box-shadow: 0 0 8px rgba(16,185,129,0.5); }
.node-status.degraded { background: #f5a623; box-shadow: 0 0 8px rgba(245,166,35,0.5); }
.tree-lines { width: 2px; background: rgba(255,255,255,0.1); }
.vertical-line { height: 30px; }
.tree-level { display: flex; gap: 40px; }
.tree-branch { display: flex; flex-direction: column; align-items: center; }
.pods-level { gap: 10px; }

/* Drift Panel */
.diff-viewer { flex: 1; display: flex; flex-direction: column; }
.diff-viewer.empty { align-items: center; justify-content: center; color: #10b981; font-weight: 500; font-size: 14px; }
.diff-file { padding: 8px 16px; font-family: var(--mono); font-size: 11px; color: #b0b4ba; border-bottom: 1px solid rgba(255,255,255,0.04); background: rgba(0,0,0,0.2); }
.diff-content { flex: 1; padding: 12px 16px; font-family: var(--mono); font-size: 12px; line-height: 1.6; overflow-y: auto; background: #111; }
.diff-pre { margin: 0; white-space: pre-wrap; color: var(--text2); }
.diff-line { padding: 0 4px; white-space: pre; }
.diff-line.minus { background: rgba(239, 68, 68, 0.15); color: #ef4444; }
.diff-line.plus { background: rgba(16, 185, 129, 0.15); color: #10b981; }
.diff-line.unchanged { color: #8b8f96; }
.drift-explanation { padding: 16px; background: rgba(245, 166, 35, 0.05); border-top: 1px solid rgba(245, 166, 35, 0.1); font-size: 13px; color: #e8eaec; line-height: 1.5; }
</style>
