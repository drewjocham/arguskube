<script setup>
import { ref, onMounted } from 'vue'
import { useResources, useDeploymentRevisions } from '../../composables/useWails'

const { result, detail, loading, detailLoading, listResources, getResourceDetail } = useResources()
const { revisions, loading: revisionsLoading, error: revisionsError, fetchRevisions } = useDeploymentRevisions()

const mockDeployments = [
  { name: 'web-app', namespace: 'default', ready: 3, desired: 3, upToDate: 3, available: 3, image: 'web-app:v1.2.4', age: '14d' },
  { name: 'worker', namespace: 'default', ready: 5, desired: 5, upToDate: 5, available: 5, image: 'worker:latest', age: '14d' },
  { name: 'payment-service', namespace: 'finance', ready: 1, desired: 2, upToDate: 2, available: 1, image: 'payment:v2.0', age: '2h' },
  { name: 'nginx-ingress', namespace: 'kube-system', ready: 2, desired: 2, upToDate: 2, available: 2, image: 'ingress-nginx:v1.9.0', age: '145d' }
]

const deployments = ref([])
const depDetail = ref(null)
const expandedDep = ref(null)
const activeDetailTab = ref('details')

onMounted(async () => {
  await listResources('deployments', '')
  if (result.value && result.value.items && result.value.items.length > 0) {
    deployments.value = result.value.items.map(item => {
      const readyParts = (item.fields?.ready || '0/0').split('/')
      return {
        name: item.name,
        namespace: item.namespace,
        status: item.status,
        statusColor: item.statusColor,
        ready: parseInt(readyParts[0]) || 0,
        desired: parseInt(readyParts[1]) || 0,
        upToDate: parseInt(item.fields?.up_to_date || '0'),
        available: parseInt(item.fields?.available || '0'),
        image: item.fields?.containers || '—',
        age: item.age || '—'
      }
    })
  } else {
    deployments.value = mockDeployments
  }
})

async function toggleExpand(depName) {
  if (expandedDep.value === depName) {
    expandedDep.value = null
    depDetail.value = null
    activeDetailTab.value = 'details'
  } else {
    expandedDep.value = depName
    activeDetailTab.value = 'details'
    const dep = deployments.value.find(d => d.name === depName)
    if (dep) {
      await getResourceDetail('deployments', dep.namespace, depName)
      if (detail.value) {
        depDetail.value = detail.value
      }
      // Also fetch revision history in parallel.
      fetchRevisions(dep.namespace, dep.name, 25)
    }
  }
}

function switchDetailTab(tab) {
  activeDetailTab.value = tab
}
</script>

<template>
  <div class="deployments-view">
    <div class="header">
      <div class="title">Deployments</div>
      <div class="subtitle">Declarative updates for Pods and ReplicaSets</div>
    </div>

    <div class="deployments-grid" :class="{ 'has-expanded': expandedDep !== null }">
      <div v-for="d in deployments" :key="d.name" 
           class="dep-card" 
           :class="{ 
             'degraded': d.ready < d.desired,
             'is-expanded': expandedDep === d.name,
             'is-hidden': expandedDep !== null && expandedDep !== d.name
           }">
        
        <!-- Main Card Content -->
        <div class="dep-main">
          <div class="dep-header">
            <div style="display:flex; align-items:center; gap:8px;">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #a78bfa;">
                <rect x="4" y="4" width="16" height="16" rx="2" ry="2"></rect>
                <rect x="9" y="9" width="6" height="6"></rect>
                <line x1="9" y1="1" x2="9" y2="4"></line>
                <line x1="15" y1="1" x2="15" y2="4"></line>
                <line x1="9" y1="20" x2="9" y2="23"></line>
                <line x1="15" y1="20" x2="15" y2="23"></line>
                <line x1="20" y1="9" x2="23" y2="9"></line>
                <line x1="20" y1="14" x2="23" y2="14"></line>
                <line x1="1" y1="9" x2="4" y2="9"></line>
                <line x1="1" y1="14" x2="4" y2="14"></line>
              </svg>
              <span class="dep-name">{{ d.name }}</span>
            </div>
            <div class="dep-ns font-mono">{{ d.namespace }}</div>
          </div>

          <div class="dep-image">
            <span class="image-icon">📦</span> <span class="font-mono">{{ d.image }}</span>
          </div>

          <div class="dep-replicas">
            <div class="replica-stats">
              <div class="stat-box">
                <div class="stat-val" :class="{'error': d.ready < d.desired}">{{ d.ready }} / {{ d.desired }}</div>
                <div class="stat-lbl">Ready</div>
              </div>
              <div class="stat-box">
                <div class="stat-val">{{ d.upToDate }}</div>
                <div class="stat-lbl">Up-to-date</div>
              </div>
              <div class="stat-box">
                <div class="stat-val">{{ d.available }}</div>
                <div class="stat-lbl">Available</div>
              </div>
            </div>
            <div class="replica-bar">
              <div class="replica-fill" :style="{ width: (d.ready / d.desired * 100) + '%', background: d.ready < d.desired ? '#f5a623' : '#3ecf8e' }"></div>
            </div>
          </div>

          <!-- Detail Expand Toggle -->
          <div class="manifest-tracker" @click="toggleExpand(d.name)">
            <div class="tracker-label" :class="d.ready < d.desired ? 'has-changes' : 'no-changes'">
              <svg v-if="d.ready >= d.desired" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"></polyline></svg>
              <svg v-else width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line></svg>
              {{ d.ready >= d.desired ? 'All replicas ready' : (d.desired - d.ready) + ' replicas pending' }}
            </div>
            <svg class="chevron" :class="{ open: expandedDep === d.name }" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded Detail -->
        <div class="dep-expanded" v-if="expandedDep === d.name">
          <div class="timeline-header">
            <div class="dep-tabs">
              <button class="dep-tab" :class="{ active: activeDetailTab === 'details' }" @click="switchDetailTab('details')">Details</button>
              <button class="dep-tab" :class="{ active: activeDetailTab === 'revisions' }" @click="switchDetailTab('revisions')">
                Revisions
                <span class="rev-count" v-if="revisions.length">{{ revisions.length }}</span>
              </button>
            </div>
            <button class="close-btn" @click="toggleExpand(d.name)">Close</button>
          </div>

          <!-- Details Tab -->
          <div v-if="activeDetailTab === 'details'">
            <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
            <div v-else-if="depDetail" class="detail-panels">
              <!-- Properties -->
              <div class="detail-section" v-if="depDetail.properties && depDetail.properties.length">
                <h5 class="section-title">Properties</h5>
                <div class="props-grid">
                  <div class="prop-row" v-for="prop in depDetail.properties" :key="prop.key">
                    <span class="prop-label">{{ prop.key }}</span>
                    <span class="prop-value font-mono">{{ prop.value }}</span>
                  </div>
                </div>
              </div>

              <!-- Conditions -->
              <div class="detail-section" v-if="depDetail.conditions && depDetail.conditions.length">
                <h5 class="section-title">Conditions</h5>
                <div class="conditions-list">
                  <div class="condition-row" v-for="c in depDetail.conditions" :key="c.type">
                    <span class="cond-type font-mono">{{ c.type }}</span>
                    <span class="cond-status" :class="c.status === 'True' ? 'ok' : 'fail'">{{ c.status }}</span>
                    <span class="cond-reason font-mono" v-if="c.reason">{{ c.reason }}</span>
                  </div>
                </div>
              </div>

              <!-- Labels -->
              <div class="detail-section" v-if="depDetail.labels && Object.keys(depDetail.labels).length">
                <h5 class="section-title">Labels</h5>
                <div class="labels-grid">
                  <span class="label-chip" v-for="(v, k) in depDetail.labels" :key="k">{{ k }}={{ v }}</span>
                </div>
              </div>

              <!-- Events -->
              <div class="detail-section" v-if="depDetail.events && depDetail.events.length">
                <h5 class="section-title">Recent Events</h5>
                <div class="events-mini">
                  <div class="event-mini-row" v-for="(ev, i) in depDetail.events" :key="i" :class="ev.type?.toLowerCase()">
                    <span class="ev-type">{{ ev.type }}</span>
                    <span class="ev-reason font-mono">{{ ev.reason }}</span>
                    <span class="ev-msg">{{ ev.message }}</span>
                    <span class="ev-age font-mono">{{ ev.age }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Revisions Tab -->
          <div v-if="activeDetailTab === 'revisions'" class="revisions-panel">
            <div v-if="revisionsLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading revision history…</div>
            <div v-else-if="revisionsError" style="color:#f5a623; font-size:13px; padding:12px;">{{ revisionsError }}</div>
            <div v-else-if="revisions.length === 0" class="empty-revisions">
              No revision history available. The deployment may not have been rolled out yet.
            </div>
            <div v-else class="revision-timeline">
              <div v-for="(rev, idx) in revisions" :key="rev.revision"
                   class="revision-entry" :class="{ active: rev.active, latest: idx === 0 }">
                <div class="rev-marker">
                  <div class="rev-dot" :class="{ active: rev.active }"></div>
                  <div class="rev-line" v-if="idx < revisions.length - 1"></div>
                </div>
                <div class="rev-content">
                  <div class="rev-header">
                    <span class="rev-number">Revision #{{ rev.revision }}</span>
                    <span class="rev-active-badge" v-if="rev.active">ACTIVE</span>
                    <span class="rev-replicas font-mono">{{ rev.readyReplicas || 0 }}/{{ rev.replicas || 0 }} replicas</span>
                  </div>
                  <div class="rev-image font-mono">{{ rev.image || '—' }}</div>
                  <div class="rev-meta">
                    <span class="rev-rs font-mono" v-if="rev.replicaSet">RS: {{ rev.replicaSet }}</span>
                    <span class="rev-time" v-if="rev.createdAt">{{ new Date(rev.createdAt).toLocaleString() }}</span>
                  </div>
                  <div class="rev-cause" v-if="rev.changeCause">
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
                    {{ rev.changeCause }}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.deployments-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.deployments-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
}
.deployments-grid.has-expanded {
  display: flex;
  flex-direction: column;
}

.dep-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease;
}
.dep-card.is-hidden { display: none; }
.dep-card.is-expanded {
  flex-direction: row;
  align-items: flex-start;
  border-color: #a78bfa;
}

.dep-main {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
}
.dep-card.is-expanded .dep-main {
  max-width: 400px;
  border-right: 1px solid rgba(255,255,255,0.05);
}

.dep-card.degraded { border-color: rgba(245, 166, 35, 0.3); }

.dep-header { display: flex; justify-content: space-between; align-items: center; }
.dep-name { font-size: 15px; font-weight: 600; color: #e8eaec; }
.dep-ns { font-size: 11px; padding: 2px 6px; background: rgba(255,255,255,0.05); border-radius: 4px; color: #b0b4ba; }

.dep-image { display: flex; align-items: center; gap: 6px; background: rgba(0,0,0,0.2); padding: 8px 10px; border-radius: 6px; border: 1px solid rgba(255,255,255,0.03); }
.image-icon { font-size: 12px; }

.dep-replicas { display: flex; flex-direction: column; gap: 12px; }
.replica-stats { display: flex; justify-content: space-between; }
.stat-box { display: flex; flex-direction: column; align-items: center; gap: 4px; }
.stat-val { font-size: 16px; font-weight: 600; color: #fff; }
.stat-val.error { color: #f5a623; }
.stat-lbl { font-size: 10px; text-transform: uppercase; letter-spacing: 0.05em; color: #8b8f96; }

.replica-bar { width: 100%; height: 6px; background: rgba(255, 255, 255, 0.06); border-radius: 3px; overflow: hidden; }
.replica-fill { height: 100%; transition: width 0.3s ease; }

.font-mono { font-family: 'SF Mono', Consolas, monospace; font-size: 12px; color: #a78bfa; }

/* Manifest Tracker Label */
.manifest-tracker {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 4px;
  padding: 8px 12px;
  background: #141517;
  border-radius: 6px;
  border: 1px solid rgba(255,255,255,0.05);
  cursor: pointer;
  transition: background 0.2s;
}
.manifest-tracker:hover { background: rgba(255,255,255,0.02); }

.tracker-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  font-weight: 600;
}
.tracker-label.no-changes { color: #3ecf8e; }
.tracker-label.has-changes { color: #f5a623; }

.chevron { color: #6b7078; transition: transform 0.2s; }
.chevron.open { transform: rotate(-90deg); }

/* Expanded Timeline */
.dep-expanded {
  flex: 2;
  padding: 24px;
  background: #141517;
  border-radius: 0 8px 8px 0;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.timeline-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.timeline-header h4 { margin: 0; font-size: 14px; color: #fff; }
.close-btn { background: transparent; border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; padding: 4px 12px; border-radius: 4px; cursor: pointer; font-size: 12px; }
.close-btn:hover { background: rgba(255,255,255,0.05); }

/* Detail Panels */
.detail-panels { display: flex; flex-direction: column; gap: 16px; }
.detail-section { display: flex; flex-direction: column; gap: 8px; }
.section-title { margin: 0; font-size: 12px; font-weight: 600; color: #a5d6ff; text-transform: uppercase; letter-spacing: 0.05em; }

.props-grid { display: flex; flex-direction: column; gap: 4px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }

.conditions-list { display: flex; flex-direction: column; gap: 4px; }
.condition-row { display: flex; align-items: center; gap: 12px; font-size: 12px; }
.cond-type { color: #e8eaec; min-width: 120px; }
.cond-status { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; }
.cond-status.ok { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.cond-status.fail { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.cond-reason { color: #8b8f96; }

.labels-grid { display: flex; flex-wrap: wrap; gap: 6px; }
.label-chip { background: rgba(55, 148, 255, 0.1); color: #3794ff; font-size: 11px; padding: 3px 8px; border-radius: 4px; font-family: 'SF Mono', Consolas, monospace; }

.events-mini { display: flex; flex-direction: column; gap: 4px; max-height: 200px; overflow-y: auto; }
.event-mini-row { display: grid; grid-template-columns: 60px 120px 1fr 50px; gap: 8px; font-size: 11px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); align-items: center; }
.event-mini-row.warning { color: #f5a623; }
.event-mini-row.normal { color: #b0b4ba; }
.ev-type { font-weight: 600; }
.ev-reason { color: #a78bfa; }
.ev-msg { color: #8b8f96; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ev-age { color: #6b7078; text-align: right; }

/* Deployment Tabs */
.dep-tabs { display: flex; gap: 4px; }
.dep-tab {
  background: transparent; border: none; color: #6b7078; padding: 6px 14px;
  font-size: 12px; font-weight: 500; cursor: pointer; border-radius: 4px; transition: all 0.2s;
  display: flex; align-items: center; gap: 6px;
}
.dep-tab:hover { color: #b0b4ba; background: rgba(255,255,255,0.04); }
.dep-tab.active { color: #fff; background: rgba(255,255,255,0.08); }
.rev-count {
  background: rgba(167, 139, 250, 0.2); color: #a78bfa;
  font-size: 10px; padding: 1px 5px; border-radius: 8px; font-weight: 600;
}

/* Revisions Panel */
.revisions-panel { padding: 16px; }
.empty-revisions { color: #6b7078; font-size: 13px; text-align: center; padding: 32px; }

.revision-timeline { display: flex; flex-direction: column; gap: 0; }

.revision-entry {
  display: flex; gap: 16px; padding: 0;
}

.rev-marker {
  display: flex; flex-direction: column; align-items: center; flex-shrink: 0; width: 20px;
}
.rev-dot {
  width: 10px; height: 10px; border-radius: 50%;
  background: #4b5058; border: 2px solid #2a2d31;
  flex-shrink: 0; margin-top: 4px;
}
.rev-dot.active { background: #3ecf8e; border-color: rgba(62, 207, 142, 0.3); box-shadow: 0 0 8px rgba(62, 207, 142, 0.3); }
.rev-line {
  width: 2px; flex: 1; background: rgba(255, 255, 255, 0.06); min-height: 12px;
}

.rev-content {
  flex: 1; display: flex; flex-direction: column; gap: 4px;
  padding-bottom: 16px; border-bottom: 1px solid rgba(255,255,255,0.03);
}
.revision-entry:last-child .rev-content { border-bottom: none; }

.rev-header { display: flex; align-items: center; gap: 10px; }
.rev-number { font-size: 13px; font-weight: 600; color: #e8eaec; }
.rev-active-badge {
  font-size: 9px; padding: 2px 6px; border-radius: 3px; font-weight: 700;
  background: rgba(62, 207, 142, 0.15); color: #3ecf8e; letter-spacing: 0.05em;
}
.rev-replicas { color: #8b8f96; font-size: 11px; margin-left: auto; }

.rev-image { font-size: 12px; color: #a78bfa; }

.rev-meta { display: flex; gap: 16px; font-size: 11px; }
.rev-rs { color: #6b7078; }
.rev-time { color: #6b7078; }

.rev-cause {
  display: flex; align-items: center; gap: 6px;
  font-size: 11px; color: #8b8f96; margin-top: 2px;
  padding: 4px 8px; background: rgba(255,255,255,0.02); border-radius: 4px;
}
.rev-cause svg { color: #6b7078; flex-shrink: 0; }
</style>
