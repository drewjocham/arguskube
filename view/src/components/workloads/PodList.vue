<script setup>
import { ref, onMounted } from 'vue'
import { useResources } from '../../composables/useWails'

const { result, detail, loading, detailLoading, listResources, getResourceDetail } = useResources()

const mockPods = [
  { name: 'web-app-58589998-f8q2b', namespace: 'default', status: 'Running', restarts: '0', node: 'ip-10-0-1-143', cpu: '120m', mem: '256Mi', age: '2d', controlledBy: 'ReplicaSet', qos: 'Burstable' },
  { name: 'worker-6b5d9db9f8-t29zj', namespace: 'default', status: 'Running', restarts: '2', node: 'ip-10-0-2-45', cpu: '45m', mem: '128Mi', age: '14h', controlledBy: 'ReplicaSet', qos: 'Burstable' },
  { name: 'redis-master-0', namespace: 'database', status: 'Running', restarts: '0', node: 'ip-10-0-2-89', cpu: '800m', mem: '2.4Gi', age: '14d', controlledBy: 'StatefulSet', qos: 'Guaranteed' },
  { name: 'failed-job-abcde', namespace: 'default', status: 'Error', restarts: '5', node: 'ip-10-0-2-45', cpu: '—', mem: '—', age: '2m', controlledBy: 'Job', qos: 'Burstable' }
]

const pods = ref([])
const podDetail = ref(null)
const expandedPod = ref(null)
const notification = ref(null)

onMounted(async () => {
  await listResources('pods', '')
  if (result.value && result.value.items && result.value.items.length > 0) {
    pods.value = result.value.items.map(item => ({
      name: item.name,
      namespace: item.namespace,
      status: item.status,
      statusColor: item.statusColor,
      restarts: item.fields?.restarts || '0',
      node: item.fields?.node || '—',
      cpu: item.fields?.cpu || '—',
      mem: item.fields?.memory || '—',
      age: item.age || '—',
      controlledBy: item.fields?.controlled_by || '—',
      qos: item.fields?.qos || '—'
    }))
  } else {
    pods.value = mockPods
  }
})

async function toggleExpand(podName) {
  if (expandedPod.value === podName) {
    expandedPod.value = null
    podDetail.value = null
  } else {
    expandedPod.value = podName
    const pod = pods.value.find(p => p.name === podName)
    if (pod) {
      await getResourceDetail('pods', pod.namespace, podName)
      if (detail.value) {
        podDetail.value = detail.value
      }
    }
  }
}

function applyAndRedeploy(p) {
  notification.value = `Applying recommended limits to ${p.name}. An agent will be watching the change for the next 10 min and if something is wrong, it will let you know.`
  setTimeout(() => {
    notification.value = null
  }, 8000)
}

const generateSparkline = (points) => {
  let path = ''
  for (let i = 0; i < points; i++) {
    const x = i * (100 / (points - 1))
    const y = 50 + Math.sin(i * 0.5) * 20 + (Math.random() * 20)
    path += `${i === 0 ? 'M' : 'L'} ${x} ${100 - y} `
  }
  return path
}
const cpuSpark = ref(generateSparkline(20))
const memSpark = ref(generateSparkline(20))
</script>

<template>
  <div class="pods-view">
    <div class="header">
      <div class="title">Pods</div>
      <div class="subtitle">The smallest deployable computing units running your containers</div>
    </div>
    
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
    </div>

    <div class="pods-list">
      <div class="pod-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-status">Status</div>
        <div class="col-restarts">Restarts</div>
        <div class="col-node">Node</div>
        <div class="col-cpu">CPU</div>
        <div class="col-mem">Memory</div>
        <div class="col-age">Age</div>
      </div>

      <div v-for="p in pods" :key="p.name" class="pod-row-container">
        <div class="pod-row" :class="p.status.toLowerCase()" @click="toggleExpand(p.name)">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #3794ff; margin-right: 8px;"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path><polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline><line x1="12" y1="22.08" x2="12" y2="12"></line></svg>
            {{ p.name }}
          </div>
          <div class="col-ns font-mono">{{ p.namespace }}</div>
          <div class="col-status">
            <span class="status-badge" :class="p.status.toLowerCase()">{{ p.status }}</span>
          </div>
          <div class="col-restarts font-mono" :class="{'high-restarts': p.restarts > 0}">{{ p.restarts }}</div>
          <div class="col-node font-mono">{{ p.node }}</div>
          <div class="col-cpu font-mono">{{ p.cpu }}</div>
          <div class="col-mem font-mono">{{ p.mem }}</div>
          <div class="col-age font-mono" style="display:flex; justify-content:space-between; align-items:center;">
            {{ p.age }}
            <svg class="chevron" :class="{ open: expandedPod === p.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded Pod Details -->
        <div class="pod-expanded" v-if="expandedPod === p.name">
          <div v-if="detailLoading" class="detail-loading">Loading pod details…</div>
          <div v-else-if="podDetail" class="expanded-grid">

            <div class="panel-col">
              <!-- Metrics Graphs -->
              <div class="expanded-card">
                <h4 class="card-title">Live Metrics</h4>
                <div class="metrics-sparklines">
                  <div class="spark-box">
                    <div class="spark-lbl">CPU Usage ({{ p.cpu }})</div>
                    <svg viewBox="0 0 100 100" preserveAspectRatio="none"><path :d="cpuSpark" fill="none" stroke="#f5a623" stroke-width="3" /></svg>
                  </div>
                  <div class="spark-box">
                    <div class="spark-lbl">Memory Usage ({{ p.mem }})</div>
                    <svg viewBox="0 0 100 100" preserveAspectRatio="none"><path :d="memSpark" fill="none" stroke="#a78bfa" stroke-width="3" /></svg>
                  </div>
                </div>
              </div>

              <!-- Properties -->
              <div class="expanded-card">
                <h4 class="card-title">Pod Properties</h4>
                <div class="props-grid">
                  <div class="prop-row" v-for="prop in podDetail.properties" :key="prop.key">
                    <span class="prop-label">{{ prop.key }}</span>
                    <span class="prop-value font-mono">{{ prop.value }}</span>
                  </div>
                </div>
              </div>

              <!-- Conditions -->
              <div class="expanded-card" v-if="podDetail.conditions && podDetail.conditions.length">
                <h4 class="card-title">Conditions</h4>
                <div class="conditions-list">
                  <div class="condition-row" v-for="c in podDetail.conditions" :key="c.type">
                    <span class="cond-type font-mono">{{ c.type }}</span>
                    <span class="cond-status" :class="c.status === 'True' ? 'ok' : 'fail'">{{ c.status }}</span>
                    <span class="cond-reason font-mono" v-if="c.reason">{{ c.reason }}</span>
                  </div>
                </div>
              </div>
            </div>

            <div class="panel-col">
              <!-- Labels & Annotations -->
              <div class="expanded-card" v-if="podDetail.labels && Object.keys(podDetail.labels).length">
                <h4 class="card-title">Labels</h4>
                <div class="labels-grid">
                  <span class="label-chip" v-for="(v, k) in podDetail.labels" :key="k">{{ k }}={{ v }}</span>
                </div>
              </div>

              <!-- Events -->
              <div class="expanded-card" v-if="podDetail.events && podDetail.events.length">
                <h4 class="card-title">Recent Events</h4>
                <div class="events-mini">
                  <div class="event-mini-row" v-for="(ev, i) in podDetail.events" :key="i" :class="ev.type?.toLowerCase()">
                    <span class="ev-type font-mono">{{ ev.type }}</span>
                    <span class="ev-reason font-mono">{{ ev.reason }}</span>
                    <span class="ev-msg">{{ ev.message }}</span>
                    <span class="ev-age font-mono">{{ ev.age }}</span>
                  </div>
                </div>
              </div>

              <!-- Annotations -->
              <div class="expanded-card" v-if="podDetail.annotations && Object.keys(podDetail.annotations).length">
                <h4 class="card-title">Annotations</h4>
                <div class="props-grid">
                  <div class="prop-row" v-for="(v, k) in podDetail.annotations" :key="k">
                    <span class="prop-label">{{ k }}</span>
                    <span class="prop-value font-mono">{{ v }}</span>
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
.pods-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.pods-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.pod-header-row {
  display: grid;
  grid-template-columns: 2.5fr 1fr 100px 80px 1.5fr 80px 80px 60px;
  gap: 16px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
  font-weight: 600;
  color: #8b8f96;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.pod-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}
.pod-row-container:last-child { border-bottom: none; }

.pod-row {
  display: grid;
  grid-template-columns: 2.5fr 1fr 100px 80px 1.5fr 80px 80px 60px;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.pod-row:hover { background: rgba(255, 255, 255, 0.03); }

.col-name { display: flex; align-items: center; font-weight: 500; color: #e8eaec; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }
.high-restarts { color: #f5a623; font-weight: 600; }

.chevron { transition: transform 0.2s ease; color: #6b7078; }
.chevron.open { transform: rotate(180deg); }

.status-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; }
.status-badge.running { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-badge.error { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.status-badge.pending { background: rgba(245, 166, 35, 0.15); color: #f5a623; }

/* Expanded View */
.pod-expanded {
  padding: 16px;
  background: #141517;
  border-top: 1px dashed rgba(255,255,255,0.08);
}
.expanded-grid {
  display: flex;
  gap: 24px;
}
.panel-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.expanded-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
  padding: 16px;
}
.card-title {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  margin: 0 0 16px 0;
}

/* Sparklines */
.metrics-sparklines {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.spark-box {
  background: #0d0d0d;
  border: 1px solid rgba(255,255,255,0.05);
  border-radius: 4px;
  padding: 8px;
  height: 70px;
  display: flex;
  flex-direction: column;
}
.spark-lbl { font-size: 11px; color: #8b8f96; text-align: center; margin-bottom: 4px; }
.spark-box svg { width: 100%; flex: 1; }

/* Detail Loading */
.detail-loading { padding: 24px; text-align: center; color: #8b8f96; font-size: 13px; }

/* Properties */
.props-grid { display: flex; flex-direction: column; gap: 6px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }

/* Conditions */
.conditions-list { display: flex; flex-direction: column; gap: 6px; }
.condition-row { display: flex; align-items: center; gap: 12px; font-size: 12px; padding: 4px 0; }
.cond-type { color: #e8eaec; min-width: 120px; }
.cond-status { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; }
.cond-status.ok { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.cond-status.fail { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.cond-reason { color: #8b8f96; }

/* Labels */
.labels-grid { display: flex; flex-wrap: wrap; gap: 6px; }
.label-chip { background: rgba(55, 148, 255, 0.1); color: #3794ff; font-size: 11px; padding: 3px 8px; border-radius: 4px; font-family: 'SF Mono', Consolas, monospace; }

/* Mini Events */
.events-mini { display: flex; flex-direction: column; gap: 4px; max-height: 200px; overflow-y: auto; }
.event-mini-row { display: grid; grid-template-columns: 60px 120px 1fr 50px; gap: 8px; font-size: 11px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); align-items: center; }
.event-mini-row.warning { color: #f5a623; }
.event-mini-row.normal { color: #b0b4ba; }
.ev-type { font-weight: 600; }
.ev-reason { color: #a78bfa; }
.ev-msg { color: #8b8f96; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ev-age { color: #6b7078; text-align: right; }

/* Agent Notification */
.agent-notification {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(167, 139, 250, 0.15);
  border: 1px solid rgba(167, 139, 250, 0.3);
  padding: 12px 16px;
  border-radius: 6px;
  margin-bottom: 16px;
  color: #e8eaec;
  font-size: 13px;
  animation: slide-down 0.3s ease-out;
}
.notif-icon { color: #a78bfa; display: flex; }
@keyframes slide-down {
  from { opacity: 0; transform: translateY(-10px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
