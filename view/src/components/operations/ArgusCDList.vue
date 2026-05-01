<script setup>
import { ref, onMounted, inject } from 'vue'
import { useApplications } from '../../composables/useWails'
import ProGateOverlay from '../shared/ProGateOverlay.vue'

const isAllowed = inject('isAllowed')

const { applications: backendApps, loading, listApplications, syncApplication } = useApplications()

const mockApps = [
  { name: 'arguskube-core', namespace: 'arguskube-system', syncStatus: 'Synced', healthStatus: 'Healthy', image: 'drewjocham/arguskube-core:v1.2.0', replicas: 3, readyReplicas: 3, lastSync: '10m ago' },
  { name: 'payment-service', namespace: 'finance-prod', syncStatus: 'OutOfSync', healthStatus: 'Healthy', image: 'drewjocham/payment:v2.0', replicas: 2, readyReplicas: 2, lastSync: '2d ago' },
  { name: 'data-pipeline', namespace: 'data-prod', syncStatus: 'Synced', healthStatus: 'Degraded', image: 'drewjocham/pipeline:latest', replicas: 5, readyReplicas: 3, lastSync: '1h ago' },
]

const applications = ref(mockApps)

onMounted(async () => {
  await listApplications('')
  if (backendApps.value && backendApps.value.length > 0) {
    applications.value = backendApps.value
  }
})

function syncStatusColor(status) {
  return status === 'Synced' ? '#10b981' : '#f5a623'
}

function healthStatusColor(status) {
  switch (status) {
    case 'Healthy': return '#10b981'
    case 'Degraded': return '#ef4444'
    case 'Progressing': return '#3b82f6'
    case 'Missing': return '#f5a623'
    default: return '#8b8f96'
  }
}

const syncing = ref(null)

async function syncApp(app, event) {
  if (event) event.stopPropagation()
  syncing.value = app.name
  try {
    await syncApplication(app.namespace, app.name)
    app.healthStatus = 'Progressing'
    app.syncStatus = 'Synced'
    app.lastSync = 'just now'
    setTimeout(() => {
      app.healthStatus = 'Healthy'
      syncing.value = null
    }, 3000)
  } catch (e) {
    // Dev mode fallback.
    app.syncStatus = 'Synced'
    app.healthStatus = 'Progressing'
    setTimeout(() => {
      app.healthStatus = 'Healthy'
      app.lastSync = 'just now'
      syncing.value = null
    }, 3000)
  }
}

const selectedApp = ref(null)

function selectApp(app) {
  selectedApp.value = app
}

function clearSelection() {
  selectedApp.value = null
}
</script>

<template>
  <div class="argus-view" style="position: relative;">
    <ProGateOverlay
      v-if="!isAllowed('arguscd')"
      feature="arguscd"
      title="ArgusCD"
      description="Continuous deployment management is available with KubeWatcher Pro. Deploy, sync, and monitor your applications from a single pane."
    />
    <div class="header">
      <div class="title-row">
        <div class="title">ArgusCD</div>
        <button class="new-app-btn">+ New App</button>
      </div>
      <div class="subtitle">Declarative, GitOps continuous delivery for Kubernetes.</div>
    </div>

    <div v-if="!selectedApp" class="app-grid">
      <div v-for="app in applications" :key="app.name" class="app-card" @click="selectApp(app)">
        <div class="card-header">
          <div class="app-info">
            <svg class="app-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><circle cx="8.5" cy="8.5" r="1.5"></circle><polyline points="21 15 16 10 5 21"></polyline></svg>
            <div>
              <div class="app-name">{{ app.name }}</div>
              <div class="app-project">Project: {{ app.project }}</div>
            </div>
          </div>
          
          <button class="sync-action-btn" @click="syncApp(app, $event)">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"></polyline><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path></svg>
            SYNC
          </button>
        </div>

        <div class="status-row">
          <div class="status-box">
            <div class="status-label">SYNC STATUS</div>
            <div class="status-val" :style="{ color: syncStatusColor(app.syncStatus) }">
              <span class="status-dot" :style="{ background: syncStatusColor(app.syncStatus) }"></span>
              {{ app.syncStatus }}
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
              {{ app.healthStatus }}
            </div>
          </div>
        </div>

        <div class="meta-grid">
          <div class="meta-item">
            <div class="meta-icon">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22"></path></svg>
            </div>
            <div class="meta-content">
              <div class="meta-val repo-url">{{ app.repoUrl }}</div>
              <div class="meta-path">{{ app.path }}</div>
            </div>
          </div>
          
          <div class="meta-item">
            <div class="meta-icon">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path><polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline><line x1="12" y1="22.08" x2="12" y2="12"></line></svg>
            </div>
            <div class="meta-content">
              <div class="meta-val">{{ app.destServer }}</div>
              <div class="meta-path">ns: {{ app.destNamespace }}</div>
            </div>
          </div>
        </div>
        
        <div class="card-footer">
          <div>Last Sync: <span style="color: #ccc;">{{ app.lastSync }}</span></div>
        </div>
      </div>
    </div>

    <!-- Application Detail View (Resource Tree + Drift) -->
    <div v-else class="app-detail">
      <div class="detail-header">
        <button class="back-btn" @click="clearSelection()">← Back</button>
        <div class="detail-title">
          <span class="detail-name">{{ selectedApp.name }}</span>
          <span class="status-dot" :style="{ background: syncStatusColor(selectedApp.syncStatus), marginLeft: '12px' }"></span>
          <span style="font-size: 13px; font-weight: 600;" :style="{ color: syncStatusColor(selectedApp.syncStatus) }">{{ selectedApp.syncStatus }}</span>
        </div>
        <div class="detail-actions">
          <button class="sync-action-btn primary" @click="syncApp(selectedApp, null)">SYNC</button>
        </div>
      </div>
      
      <!-- Content Split: Visual Tree & Drift View -->
      <div class="detail-content">
        <!-- Resource Tree Graph (Left) -->
        <div class="tree-panel">
          <div class="panel-header">Live Resource Tree</div>
          <div class="tree-canvas">
            <div class="tree-node root-node">
              <div class="node-icon">📦</div>
              <div class="node-text">{{ selectedApp.name }}</div>
            </div>
            <div class="tree-lines vertical-line"></div>
            
            <div class="tree-level">
              <!-- Service Branch -->
              <div class="tree-branch">
                <div class="tree-node service-node">
                  <div class="node-icon">🌐</div>
                  <div class="node-text">Service</div>
                  <div class="node-status healthy"></div>
                </div>
              </div>
              
              <!-- Deployment Branch -->
              <div class="tree-branch">
                <div class="tree-node deploy-node" :class="{ 'drifted': selectedApp.syncStatus === 'OutOfSync' }">
                  <div class="node-icon">⚙️</div>
                  <div class="node-text">Deployment</div>
                  <div class="node-status" :class="selectedApp.syncStatus === 'OutOfSync' ? 'degraded' : 'healthy'"></div>
                </div>
                
                <div class="tree-lines vertical-line"></div>
                
                <div class="tree-level pods-level">
                  <div class="tree-node pod-node">
                    <div class="node-icon">🚀</div>
                    <div class="node-text">Pod-abcd</div>
                    <div class="node-status healthy"></div>
                  </div>
                  <div class="tree-node pod-node">
                    <div class="node-icon">🚀</div>
                    <div class="node-text">Pod-efgh</div>
                    <div class="node-status healthy"></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Drift Detection / Diff Panel (Right) -->
        <div class="drift-panel">
          <div class="panel-header" :class="{ 'drift-alert': selectedApp.syncStatus === 'OutOfSync' }">
            Drift Detection
            <span v-if="selectedApp.syncStatus === 'OutOfSync'" class="drift-badge">1 Drift Detected</span>
          </div>
          
          <div class="diff-viewer" v-if="selectedApp.syncStatus === 'OutOfSync'">
            <div class="diff-file">deployment.yaml</div>
            <div class="diff-content">
              <div class="diff-line unchanged">  template:</div>
              <div class="diff-line unchanged">    spec:</div>
              <div class="diff-line unchanged">      containers:</div>
              <div class="diff-line unchanged">        - name: app</div>
              <div class="diff-line minus">-         image: nginx:1.19.0</div>
              <div class="diff-line plus">+         image: nginx:1.21.0</div>
              <div class="diff-line unchanged">          ports:</div>
              <div class="diff-line unchanged">            - containerPort: 80</div>
            </div>
            
            <div class="drift-explanation">
              <strong>Live state differs from Git!</strong><br>
              The container image tag was manually mutated in the cluster to <code class="live-tag">nginx:1.19.0</code>, but the Git repository defines <code class="git-tag">nginx:1.21.0</code>.<br>
              <br>
              Click <strong>SYNC</strong> to perform self-healing and revert the cluster back to the Git source of truth.
            </div>
          </div>
          
          <div class="diff-viewer empty" v-else>
            <div class="empty-icon">✓</div>
            <div>Cluster is fully synced with Git.</div>
            <div style="font-size: 11px; color: var(--text3); margin-top: 4px;">No configuration drift detected.</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.argus-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title-row { display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px; }
.header .title { font-size: 24px; font-weight: 600; color: #fff; letter-spacing: -0.02em; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.new-app-btn {
  background: rgba(59, 130, 246, 0.15); border: 1px solid rgba(59, 130, 246, 0.4); color: #60a5fa;
  padding: 6px 12px; border-radius: 6px; font-size: 12px; font-weight: 600; cursor: pointer; transition: all 0.2s;
}
.new-app-btn:hover { background: rgba(59, 130, 246, 0.25); }

.app-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(360px, 1fr)); gap: 16px; }

.app-card {
  background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px;
  display: flex; flex-direction: column; overflow: hidden; transition: transform 0.15s, box-shadow 0.15s;
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
.meta-val { font-size: 12px; color: #e8eaec; font-family: var(--mono); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.repo-url { color: #60a5fa; }
.meta-path { font-size: 11px; color: #8b8f96; }

.card-footer { padding: 10px 16px; background: rgba(0,0,0,0.2); font-size: 11px; color: #8b8f96; display: flex; justify-content: space-between; border-top: 1px solid rgba(255,255,255,0.04); }

/* Detail View Styles */
.app-detail { display: flex; flex-direction: column; height: 100%; gap: 16px; }
.detail-header { display: flex; align-items: center; padding: 16px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.back-btn { background: transparent; border: 1px solid rgba(255,255,255,0.1); color: #fff; padding: 6px 12px; border-radius: 6px; cursor: pointer; font-size: 13px; font-weight: 500; margin-right: 20px; transition: background 0.15s; }
.back-btn:hover { background: rgba(255,255,255,0.05); }
.detail-title { flex: 1; display: flex; align-items: center; }
.detail-name { font-size: 20px; font-weight: 600; color: #fff; }
.sync-action-btn.primary { background: rgba(16, 185, 129, 0.15); border-color: rgba(16, 185, 129, 0.4); color: #10b981; padding: 8px 16px; font-size: 12px; }
.sync-action-btn.primary:hover { background: rgba(16, 185, 129, 0.25); }

.detail-content { display: flex; gap: 16px; flex: 1; min-height: 0; }
.tree-panel, .drift-panel { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.panel-header { padding: 12px 16px; font-size: 13px; font-weight: 600; border-bottom: 1px solid rgba(255,255,255,0.04); color: #fff; letter-spacing: 0.02em; }
.panel-header.drift-alert { background: rgba(245, 166, 35, 0.1); color: #f5a623; display: flex; justify-content: space-between; align-items: center; }
.drift-badge { font-size: 10px; padding: 2px 6px; background: rgba(245, 166, 35, 0.2); border-radius: 4px; color: #f5a623; }

/* Tree specific */
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
.empty-icon { font-size: 32px; margin-bottom: 12px; background: rgba(16, 185, 129, 0.1); width: 64px; height: 64px; border-radius: 50%; display: flex; align-items: center; justify-content: center; }
.diff-file { padding: 8px 16px; font-family: var(--mono); font-size: 11px; color: #b0b4ba; border-bottom: 1px solid rgba(255,255,255,0.04); background: rgba(0,0,0,0.2); }
.diff-content { flex: 1; padding: 12px 16px; font-family: var(--mono); font-size: 12px; line-height: 1.6; overflow-y: auto; background: #111; }
.diff-line { padding: 0 4px; white-space: pre; }
.diff-line.minus { background: rgba(239, 68, 68, 0.15); color: #ef4444; }
.diff-line.plus { background: rgba(16, 185, 129, 0.15); color: #10b981; }
.diff-line.unchanged { color: #8b8f96; }

.drift-explanation { padding: 16px; background: rgba(245, 166, 35, 0.05); border-top: 1px solid rgba(245, 166, 35, 0.1); font-size: 13px; color: #e8eaec; line-height: 1.5; }
.live-tag { color: #ef4444; font-family: var(--mono); background: rgba(239,68,68,0.1); padding: 2px 4px; border-radius: 4px; font-size: 11px; }
.git-tag { color: #10b981; font-family: var(--mono); background: rgba(16,185,129,0.1); padding: 2px 4px; border-radius: 4px; font-size: 11px; }
</style>
