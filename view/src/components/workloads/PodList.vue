<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useResources, usePodLogs, useTimeSeriesMetrics, callGo } from '../../composables/useWails'

const { result, detail, loading, detailLoading, error: resourceError, listResources, getResourceDetail, listNamespaces, namespaces } = useResources()
const { logs: podLogs, loading: logsLoading, fetchLogs } = usePodLogs()
const { queryMetrics } = useTimeSeriesMetrics()

// Demo data shown when the backend is unreachable or returns nothing.
const mockPods = [
  { name: 'api-gateway-5f8b9c7d4-x2k9p', namespace: 'default', status: 'Running', statusColor: 'green', restarts: '0', node: 'node-1', cpu: '50m', mem: '128Mi', age: '3d', controlledBy: 'ReplicaSet', qos: 'Burstable' },
  { name: 'frontend-7c6d8f9b2-m4n7q', namespace: 'default', status: 'Running', statusColor: 'green', restarts: '2', node: 'node-2', cpu: '100m', mem: '256Mi', age: '1d', controlledBy: 'ReplicaSet', qos: 'Burstable' },
  { name: 'postgres-0', namespace: 'database', status: 'Running', statusColor: 'green', restarts: '0', node: 'node-1', cpu: '250m', mem: '512Mi', age: '7d', controlledBy: 'StatefulSet', qos: 'Guaranteed' },
  { name: 'redis-cache-6b4c8d2f1-p9r3s', namespace: 'cache', status: 'Running', statusColor: 'green', restarts: '0', node: 'node-2', cpu: '25m', mem: '64Mi', age: '5d', controlledBy: 'ReplicaSet', qos: 'Burstable' },
  { name: 'worker-processor-8e7f6a5b3-k1l2m', namespace: 'default', status: 'CrashLoopBackOff', statusColor: 'red', restarts: '14', node: 'node-1', cpu: '—', mem: '—', age: '2h', controlledBy: 'ReplicaSet', qos: 'BestEffort' },
]

const pods = ref([])
const podDetail = ref(null)
const expandedPod = ref(null)
const notification = ref(null)
const searchQuery = ref('')
const selectedNamespace = ref('')
const activeTab = ref('details')
const logTailLines = ref(100)
const logContent = ref([])
const logSearchFilter = ref('')
const confirmDelete = ref(null)
const connectionError = ref(null)
let autoRefreshTimer = null

async function fetchPods() {
  connectionError.value = null
  try {
    await listResources('pods', selectedNamespace.value)

    // Check if the composable caught an error.
    if (resourceError.value) {
      console.error('[PodList] backend error:', resourceError.value)
      connectionError.value = resourceError.value
      pods.value = mockPods
      return
    }

    // Check what we got back.
    console.log('[PodList] raw result:', JSON.stringify(result.value)?.substring(0, 500))

    if (result.value && result.value.items && result.value.items.length > 0) {
      pods.value = result.value.items.map(mapPod)
      console.log(`[PodList] loaded ${pods.value.length} real pods`)
    } else if (result.value && result.value.items && result.value.items.length === 0) {
      connectionError.value = 'Backend returned 0 pods. Check namespace filter or cluster connection.'
      pods.value = mockPods
    } else {
      connectionError.value = 'Could not reach backend — showing demo data.'
      pods.value = mockPods
    }
  } catch (e) {
    console.error('[PodList] unexpected error in fetchPods:', e)
    connectionError.value = `Error: ${e?.message || e}`
    pods.value = mockPods
  }
}

function mapPod(item) {
  return {
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
  }
}

const filteredPods = computed(() => {
  if (!searchQuery.value) return pods.value
  const q = searchQuery.value.toLowerCase()
  return pods.value.filter(p =>
    p.name.toLowerCase().includes(q) ||
    p.namespace.toLowerCase().includes(q) ||
    p.status.toLowerCase().includes(q) ||
    p.node.toLowerCase().includes(q)
  )
})

const statusCounts = computed(() => {
  const counts = { running: 0, pending: 0, error: 0, other: 0 }
  for (const p of pods.value) {
    const s = p.status.toLowerCase()
    if (s === 'running') counts.running++
    else if (s === 'pending' || s === 'containercreating') counts.pending++
    else if (s === 'error' || s === 'crashloopbackoff' || s === 'imagepullbackoff') counts.error++
    else counts.other++
  }
  return counts
})

// Container status parsing from detail.extra.containers
const containerStatuses = computed(() => {
  if (!podDetail.value?.extra?.containers) return []
  return podDetail.value.extra.containers.map(c => {
    const parts = c.value.split('|')
    return {
      name: c.key,
      state: parts[0] || 'Unknown',
      image: parts[1] || '—',
      ready: parts[2] === 'true',
      restarts: parseInt(parts[3]) || 0,
      detail: parts[4] || ''
    }
  })
})

const generateSparkline = (points) => {
  let path = ''
  for (let i = 0; i < points; i++) {
    const x = i * (100 / (points - 1))
    const y = 50 + Math.sin(i * 0.5) * 20 + (Math.random() * 20)
    path += `${i === 0 ? 'M' : 'L'} ${x} ${100 - y} `
  }
  return path
}

function buildSparklinePath(data) {
  if (!data || !data.length) return generateSparkline(20)
  let path = ''
  const points = data.length
  for (let i = 0; i < points; i++) {
    const x = i * (100 / (points - 1))
    const y = data[i]
    path += `${i === 0 ? 'M' : 'L'} ${x} ${100 - y} `
  }
  return path
}

const cpuSpark = ref(generateSparkline(20))
const memSpark = ref(generateSparkline(20))

async function toggleExpand(podName) {
  if (expandedPod.value === podName) {
    expandedPod.value = null
    podDetail.value = null
    activeTab.value = 'details'
    logContent.value = []
  } else {
    expandedPod.value = podName
    activeTab.value = 'details'
    cpuSpark.value = generateSparkline(20)
    memSpark.value = generateSparkline(20)
    
    const pod = pods.value.find(p => p.name === podName)
    if (pod) {
      await getResourceDetail('pods', pod.namespace, podName)
      if (detail.value) {
        podDetail.value = detail.value
      }
      
      try {
        const [cpuData, memData] = await Promise.all([
          queryMetrics(`cpu_pod_${podName}`, '1h'),
          queryMetrics(`mem_pod_${podName}`, '1h')
        ])
        
        if (cpuData && cpuData.length) cpuSpark.value = buildSparklinePath(cpuData)
        if (memData && memData.length) memSpark.value = buildSparklinePath(memData)
      } catch (e) {
        console.error('Failed to load real metrics:', e)
      }
    }
  }
}

async function switchTab(tab) {
  activeTab.value = tab
  if (tab === 'logs') {
    await loadLogs()
  }
}

async function loadLogs() {
  const pod = pods.value.find(p => p.name === expandedPod.value)
  if (!pod) return
  await fetchLogs(pod.namespace, pod.name, logTailLines.value)
  logContent.value = podLogs.value || []
}

const filteredLogs = computed(() => {
  if (!logSearchFilter.value) return logContent.value
  const q = logSearchFilter.value.toLowerCase()
  return logContent.value.filter(l => l.message?.toLowerCase().includes(q))
})

async function deletePod(pod) {
  confirmDelete.value = null
  notification.value = `Deleting pod ${pod.name}...`
  try {
    await callGo('DeletePod', pod.namespace, pod.name)
    notification.value = `Pod ${pod.name} deleted. It may be recreated by its controller.`
    // Remove from local list immediately.
    pods.value = pods.value.filter(p => p.name !== pod.name)
    if (expandedPod.value === pod.name) {
      expandedPod.value = null
      podDetail.value = null
    }
  } catch (e) {
    notification.value = `Failed to delete pod: ${e?.message || e}`
  }
  setTimeout(() => notification.value = null, 5000)
}

function applyAndRedeploy(p) {
  notification.value = `Applying recommended limits to ${p.name}. An agent will be watching the change for the next 10 min and if something is wrong, it will let you know.`
  setTimeout(() => notification.value = null, 8000)
}

async function onNamespaceChange() {
  expandedPod.value = null
  podDetail.value = null
  await fetchPods()
}

onMounted(async () => {
  await listNamespaces()
  await fetchPods()
})

onUnmounted(() => {
  if (autoRefreshTimer) clearInterval(autoRefreshTimer)
})
</script>

<template>
  <div class="pods-view">
    <!-- Header -->
    <div class="header">
      <div class="header-text">
        <div class="title">Pods</div>
        <div class="subtitle">The smallest deployable computing units running your containers</div>
      </div>
      <div class="header-controls">
        <div class="status-summary">
          <span class="status-chip running">{{ statusCounts.running }} Running</span>
          <span class="status-chip pending" v-if="statusCounts.pending">{{ statusCounts.pending }} Pending</span>
          <span class="status-chip error" v-if="statusCounts.error">{{ statusCounts.error }} Error</span>
        </div>
        <button class="refresh-btn" @click="fetchPods" :disabled="loading">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6"></path><path d="M1 20v-6h6"></path><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
          Refresh
        </button>
      </div>
    </div>

    <!-- Connection error banner -->
    <div v-if="connectionError" class="error-banner">
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>
      <span>{{ connectionError }}</span>
      <button class="error-dismiss" @click="connectionError = null">&times;</button>
    </div>

    <!-- Notification -->
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
      <button class="notif-close" @click="notification = null">&times;</button>
    </div>

    <!-- Delete confirmation -->
    <div v-if="confirmDelete" class="confirm-overlay" @click.self="confirmDelete = null">
      <div class="confirm-dialog">
        <div class="confirm-title">Delete Pod</div>
        <div class="confirm-msg">Are you sure you want to delete <strong>{{ confirmDelete.name }}</strong> in namespace <strong>{{ confirmDelete.namespace }}</strong>? If managed by a controller, it will be recreated.</div>
        <div class="confirm-actions">
          <button class="confirm-cancel" @click="confirmDelete = null">Cancel</button>
          <button class="confirm-delete" @click="deletePod(confirmDelete)">Delete</button>
        </div>
      </div>
    </div>

    <!-- Filter bar -->
    <div class="filter-bar">
      <div class="search-box">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
        <input type="text" v-model="searchQuery" placeholder="Filter pods by name, namespace, status, node..." class="search-input" />
      </div>
      <select v-model="selectedNamespace" @change="onNamespaceChange" class="ns-select">
        <option value="">All Namespaces</option>
        <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
      </select>
      <div class="pod-count">{{ filteredPods.length }} pod{{ filteredPods.length !== 1 ? 's' : '' }}</div>
    </div>

    <!-- Pod table -->
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

      <div v-if="loading && !pods.length" class="loading-row">Loading pods...</div>

      <div v-for="p in filteredPods" :key="p.name + p.namespace" class="pod-row-container">
        <div class="pod-row" :class="p.status.toLowerCase()" @click="toggleExpand(p.name)">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #3794ff; margin-right: 8px; flex-shrink:0;"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path><polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline><line x1="12" y1="22.08" x2="12" y2="12"></line></svg>
            <span class="pod-name-text">{{ p.name }}</span>
          </div>
          <div class="col-ns font-mono">{{ p.namespace }}</div>
          <div class="col-status">
            <span class="status-badge" :class="p.status.toLowerCase()">{{ p.status }}</span>
          </div>
          <div class="col-restarts font-mono" :class="{'high-restarts': parseInt(p.restarts) > 0}">{{ p.restarts }}</div>
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
          <!-- Tabs -->
          <div class="tab-bar">
            <button class="tab-btn" :class="{ active: activeTab === 'details' }" @click="switchTab('details')">Details</button>
            <button class="tab-btn" :class="{ active: activeTab === 'containers' }" @click="switchTab('containers')">Containers</button>
            <button class="tab-btn" :class="{ active: activeTab === 'logs' }" @click="switchTab('logs')">Logs</button>
            <button class="tab-btn" :class="{ active: activeTab === 'events' }" @click="switchTab('events')">Events</button>

            <div class="tab-actions">
              <button class="action-btn delete-btn" @click.stop="confirmDelete = p">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                Delete Pod
              </button>
            </div>
          </div>

          <div v-if="detailLoading" class="detail-loading">Loading pod details...</div>

          <!-- Details Tab -->
          <div v-else-if="activeTab === 'details' && podDetail" class="expanded-grid">
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
              <!-- Labels -->
              <div class="expanded-card" v-if="podDetail.labels && Object.keys(podDetail.labels).length">
                <h4 class="card-title">Labels</h4>
                <div class="labels-grid">
                  <span class="label-chip" v-for="(v, k) in podDetail.labels" :key="k">{{ k }}={{ v }}</span>
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

          <!-- Containers Tab -->
          <div v-else-if="activeTab === 'containers'" class="containers-section">
            <div v-if="containerStatuses.length" class="container-cards">
              <div v-for="c in containerStatuses" :key="c.name" class="container-card" :class="c.state.toLowerCase()">
                <div class="container-header">
                  <div class="container-name">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="2" width="20" height="20" rx="2" ry="2"></rect><line x1="7" y1="2" x2="7" y2="22"></line><line x1="17" y1="2" x2="17" y2="22"></line><line x1="2" y1="12" x2="22" y2="12"></line></svg>
                    {{ c.name }}
                  </div>
                  <span class="container-state" :class="c.state.toLowerCase()">{{ c.state }}</span>
                </div>
                <div class="container-meta">
                  <div class="cmeta-row">
                    <span class="cmeta-label">Image:</span>
                    <span class="cmeta-value font-mono">{{ c.image }}</span>
                  </div>
                  <div class="cmeta-row">
                    <span class="cmeta-label">Ready:</span>
                    <span class="cmeta-value" :class="c.ready ? 'ok' : 'fail'">{{ c.ready ? 'Yes' : 'No' }}</span>
                  </div>
                  <div class="cmeta-row">
                    <span class="cmeta-label">Restarts:</span>
                    <span class="cmeta-value" :class="{'high-restarts': c.restarts > 0}">{{ c.restarts }}</span>
                  </div>
                  <div class="cmeta-row" v-if="c.detail">
                    <span class="cmeta-label">Detail:</span>
                    <span class="cmeta-value">{{ c.detail }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="empty-state">
              No container status information available. The pod may not have started yet.
            </div>
          </div>

          <!-- Logs Tab -->
          <div v-else-if="activeTab === 'logs'" class="logs-section">
            <div class="logs-toolbar">
              <div class="logs-search-box">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
                <input type="text" v-model="logSearchFilter" placeholder="Filter log lines..." class="log-filter-input" />
              </div>
              <select v-model="logTailLines" @change="loadLogs" class="log-lines-select">
                <option :value="50">Last 50 lines</option>
                <option :value="100">Last 100 lines</option>
                <option :value="500">Last 500 lines</option>
                <option :value="1000">Last 1000 lines</option>
              </select>
              <button class="refresh-btn small" @click="loadLogs" :disabled="logsLoading">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6"></path><path d="M1 20v-6h6"></path><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
                Reload
              </button>
            </div>
            <div class="logs-viewer" v-if="filteredLogs.length">
              <div v-for="(line, i) in filteredLogs" :key="i" class="log-line">
                <span class="log-ts font-mono" v-if="line.timestamp">{{ new Date(line.timestamp).toLocaleTimeString() }}</span>
                <span class="log-src font-mono" v-if="line.source">{{ line.source }}</span>
                <span class="log-msg">{{ line.message }}</span>
              </div>
            </div>
            <div v-else-if="logsLoading" class="empty-state">Loading logs...</div>
            <div v-else class="empty-state">No logs available for this pod.</div>
          </div>

          <!-- Events Tab -->
          <div v-else-if="activeTab === 'events' && podDetail" class="events-section">
            <div v-if="podDetail.events && podDetail.events.length" class="events-mini">
              <div class="event-mini-header">
                <span>Type</span><span>Reason</span><span>Message</span><span>Age</span>
              </div>
              <div class="event-mini-row" v-for="(ev, i) in podDetail.events" :key="i" :class="ev.type?.toLowerCase()">
                <span class="ev-type">
                  <span class="type-pill" :class="ev.type?.toLowerCase()">{{ ev.type }}</span>
                </span>
                <span class="ev-reason font-mono">{{ ev.reason }}</span>
                <span class="ev-msg">{{ ev.message }}</span>
                <span class="ev-age font-mono">{{ ev.age }}</span>
              </div>
            </div>
            <div v-else class="empty-state">No recent events for this pod.</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pods-view { padding: 24px; display: flex; flex-direction: column; gap: 20px; overflow-y: auto; height: 100%; }

/* Header */
.header { display: flex; justify-content: space-between; align-items: flex-start; }
.header-text .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header-text .subtitle { font-size: 13px; color: #8b8f96; }
.header-controls { display: flex; align-items: center; gap: 12px; }

.status-summary { display: flex; gap: 8px; }
.status-chip {
  font-size: 11px; padding: 3px 8px; border-radius: 4px; font-weight: 600;
}
.status-chip.running { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-chip.pending { background: rgba(245, 166, 35, 0.15); color: #f5a623; }
.status-chip.error { background: rgba(240, 84, 84, 0.15); color: #f05454; }

.refresh-btn {
  display: flex; align-items: center; gap: 6px;
  background: transparent; border: 1px solid rgba(255,255,255,0.1); color: #8b8f96;
  padding: 5px 12px; border-radius: 4px; font-size: 12px; cursor: pointer; transition: all 0.2s;
}
.refresh-btn:hover { color: #fff; border-color: rgba(255,255,255,0.2); }
.refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.refresh-btn.small { padding: 4px 8px; font-size: 11px; }

/* Filter Bar */
.filter-bar { display: flex; align-items: center; gap: 12px; }
.search-box {
  flex: 1; display: flex; align-items: center; gap: 8px;
  background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 6px; padding: 8px 12px;
}
.search-box svg { color: #6b7078; flex-shrink: 0; }
.search-input {
  flex: 1; background: transparent; border: none; color: #e8eaec; font-size: 13px; outline: none;
}
.search-input::placeholder { color: #4b5058; }
.ns-select {
  background: #1e2023; border: 1px solid rgba(255,255,255,0.08); color: #e8eaec;
  padding: 8px 12px; border-radius: 6px; font-size: 13px; outline: none; cursor: pointer; min-width: 160px;
}
.pod-count { font-size: 12px; color: #6b7078; white-space: nowrap; }

/* Pod Table */
.pods-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.pod-header-row {
  display: grid;
  grid-template-columns: 2.5fr 1fr 100px 80px 1.5fr 80px 80px 60px;
  gap: 16px;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px; font-weight: 600; color: #8b8f96; text-transform: uppercase; letter-spacing: 0.05em;
}

.loading-row { padding: 24px; text-align: center; color: #8b8f96; font-size: 13px; }

.pod-row-container { border-bottom: 1px solid rgba(255, 255, 255, 0.04); }
.pod-row-container:last-child { border-bottom: none; }

.pod-row {
  display: grid;
  grid-template-columns: 2.5fr 1fr 100px 80px 1.5fr 80px 80px 60px;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px; color: #e8eaec; align-items: center; cursor: pointer; transition: background 0.2s;
}
.pod-row:hover { background: rgba(255, 255, 255, 0.03); }

.col-name { display: flex; align-items: center; font-weight: 500; color: #e8eaec; overflow: hidden; }
.pod-name-text { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }
.high-restarts { color: #f5a623 !important; font-weight: 600; }

.chevron { transition: transform 0.2s ease; color: #6b7078; flex-shrink: 0; }
.chevron.open { transform: rotate(180deg); }

.status-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; }
.status-badge.running { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-badge.error, .status-badge.crashloopbackoff, .status-badge.imagepullbackoff { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.status-badge.pending, .status-badge.containercreating { background: rgba(245, 166, 35, 0.15); color: #f5a623; }
.status-badge.completed, .status-badge.succeeded { background: rgba(62, 207, 142, 0.1); color: #6bc9a0; }
.status-badge.terminating { background: rgba(167, 139, 250, 0.15); color: #a78bfa; }

/* Expanded View */
.pod-expanded {
  background: #141517;
  border-top: 1px dashed rgba(255,255,255,0.08);
}

/* Tab Bar */
.tab-bar {
  display: flex; align-items: center; gap: 4px;
  padding: 8px 16px;
  border-bottom: 1px solid rgba(255,255,255,0.06);
  background: rgba(255,255,255,0.02);
}
.tab-btn {
  background: transparent; border: none; color: #6b7078; padding: 6px 14px;
  font-size: 12px; font-weight: 500; cursor: pointer; border-radius: 4px; transition: all 0.2s;
}
.tab-btn:hover { color: #b0b4ba; background: rgba(255,255,255,0.04); }
.tab-btn.active { color: #fff; background: rgba(255,255,255,0.08); }
.tab-actions { margin-left: auto; display: flex; gap: 8px; }
.action-btn {
  display: flex; align-items: center; gap: 5px;
  background: transparent; border: 1px solid rgba(255,255,255,0.08); color: #8b8f96;
  padding: 4px 10px; border-radius: 4px; font-size: 11px; cursor: pointer; transition: all 0.2s;
}
.delete-btn:hover { color: #f05454; border-color: rgba(240, 84, 84, 0.3); background: rgba(240, 84, 84, 0.08); }

/* Panels */
.expanded-grid { display: flex; gap: 24px; padding: 16px; }
.panel-col { flex: 1; display: flex; flex-direction: column; gap: 16px; }
.expanded-card {
  background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.05); border-radius: 6px; padding: 16px;
}
.card-title { font-size: 13px; font-weight: 600; color: #fff; margin: 0 0 16px 0; }

/* Sparklines */
.metrics-sparklines { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.spark-box {
  background: #0d0d0d; border: 1px solid rgba(255,255,255,0.05);
  border-radius: 4px; padding: 8px; height: 70px; display: flex; flex-direction: column;
}
.spark-lbl { font-size: 11px; color: #8b8f96; text-align: center; margin-bottom: 4px; }
.spark-box svg { width: 100%; flex: 1; }

/* Detail Loading */
.detail-loading { padding: 24px; text-align: center; color: #8b8f96; font-size: 13px; }
.empty-state { padding: 32px; text-align: center; color: #6b7078; font-size: 13px; }

/* Properties */
.props-grid { display: flex; flex-direction: column; gap: 6px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; flex-shrink: 0; }
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

/* Container Cards */
.containers-section { padding: 16px; }
.container-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 16px; }
.container-card {
  background: #1e2023; border: 1px solid rgba(255,255,255,0.06); border-radius: 8px; padding: 16px;
  display: flex; flex-direction: column; gap: 12px; transition: border-color 0.2s;
}
.container-card.running { border-left: 3px solid #3ecf8e; }
.container-card.waiting { border-left: 3px solid #f5a623; }
.container-card.terminated, .container-card.completed { border-left: 3px solid #6b7078; }

.container-header { display: flex; justify-content: space-between; align-items: center; }
.container-name { display: flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 600; color: #e8eaec; }
.container-name svg { color: #3794ff; }
.container-state {
  font-size: 11px; padding: 2px 8px; border-radius: 4px; font-weight: 600;
}
.container-state.running { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.container-state.waiting { background: rgba(245, 166, 35, 0.15); color: #f5a623; }
.container-state.terminated { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.container-state.completed { background: rgba(62, 207, 142, 0.1); color: #6bc9a0; }
.container-state.unknown { background: rgba(255,255,255,0.05); color: #8b8f96; }

.container-meta { display: flex; flex-direction: column; gap: 6px; }
.cmeta-row { display: flex; justify-content: space-between; font-size: 12px; }
.cmeta-label { color: #6b7078; }
.cmeta-value { color: #b0b4ba; max-width: 70%; text-align: right; word-break: break-all; }
.cmeta-value.ok { color: #3ecf8e; }
.cmeta-value.fail { color: #f05454; }

/* Logs */
.logs-section { display: flex; flex-direction: column; }
.logs-toolbar {
  display: flex; align-items: center; gap: 12px; padding: 10px 16px;
  border-bottom: 1px solid rgba(255,255,255,0.05); background: rgba(255,255,255,0.02);
}
.logs-search-box {
  flex: 1; display: flex; align-items: center; gap: 6px;
  background: #0d0d0d; border: 1px solid rgba(255,255,255,0.06); border-radius: 4px; padding: 5px 8px;
}
.logs-search-box svg { color: #6b7078; }
.log-filter-input {
  flex: 1; background: transparent; border: none; color: #e8eaec; font-size: 12px; outline: none;
  font-family: 'SF Mono', Consolas, monospace;
}
.log-filter-input::placeholder { color: #4b5058; }
.log-lines-select {
  background: #0d0d0d; border: 1px solid rgba(255,255,255,0.06); color: #b0b4ba;
  padding: 5px 8px; border-radius: 4px; font-size: 11px; outline: none; cursor: pointer;
}

.logs-viewer {
  padding: 12px 16px;
  font-family: 'SF Mono', Consolas, monospace; font-size: 12px;
  color: #d4d4d4; max-height: 400px; overflow-y: auto;
  background: #0a0a0a;
}
.log-line {
  display: flex; gap: 12px; padding: 2px 0; border-bottom: 1px solid rgba(255,255,255,0.02);
  line-height: 1.5;
}
.log-line:hover { background: rgba(255,255,255,0.03); }
.log-ts { color: #6b7078; flex-shrink: 0; min-width: 80px; }
.log-src { color: #3794ff; flex-shrink: 0; }
.log-msg { color: #d4d4d4; word-break: break-all; white-space: pre-wrap; }

/* Events */
.events-section { padding: 16px; }
.events-mini { display: flex; flex-direction: column; gap: 0; max-height: 350px; overflow-y: auto; }
.event-mini-header {
  display: grid; grid-template-columns: 80px 140px 1fr 60px; gap: 12px;
  font-size: 11px; font-weight: 600; color: #6b7078; text-transform: uppercase; letter-spacing: 0.05em;
  padding: 8px 0; border-bottom: 1px solid rgba(255,255,255,0.06);
}
.event-mini-row {
  display: grid; grid-template-columns: 80px 140px 1fr 60px; gap: 12px;
  font-size: 12px; padding: 8px 0; border-bottom: 1px solid rgba(255,255,255,0.03); align-items: center;
}
.type-pill { font-size: 10px; padding: 2px 6px; border-radius: 3px; font-weight: 600; }
.type-pill.normal { background: rgba(62, 207, 142, 0.12); color: #3ecf8e; }
.type-pill.warning { background: rgba(245, 166, 35, 0.12); color: #f5a623; }
.ev-type { font-weight: 600; }
.ev-reason { color: #a78bfa; font-family: 'SF Mono', Consolas, monospace; font-size: 11px; }
.ev-msg { color: #8b8f96; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ev-age { color: #6b7078; text-align: right; font-family: 'SF Mono', Consolas, monospace; font-size: 11px; }

/* Error banner */
.error-banner {
  display: flex; align-items: center; gap: 10px;
  background: rgba(240, 84, 84, 0.1); border: 1px solid rgba(240, 84, 84, 0.25);
  padding: 10px 16px; border-radius: 6px; color: #f5a623; font-size: 13px;
}
.error-banner svg { flex-shrink: 0; color: #f5a623; }
.error-banner span { flex: 1; }
.error-dismiss {
  background: transparent; border: none; color: #6b7078; font-size: 18px; cursor: pointer; padding: 0 4px; line-height: 1;
}
.error-dismiss:hover { color: #e8eaec; }

/* Notification */
.agent-notification {
  display: flex; align-items: center; gap: 12px;
  background: rgba(55, 148, 255, 0.1); border: 1px solid rgba(55, 148, 255, 0.25);
  padding: 10px 16px; border-radius: 6px; color: #e8eaec; font-size: 13px;
  animation: slide-down 0.3s ease-out;
}
.notif-icon { color: #3794ff; display: flex; flex-shrink: 0; }
.notif-text { flex: 1; }
.notif-close {
  background: transparent; border: none; color: #6b7078; font-size: 18px; cursor: pointer;
  padding: 0 4px; line-height: 1;
}
.notif-close:hover { color: #e8eaec; }
@keyframes slide-down {
  from { opacity: 0; transform: translateY(-10px); }
  to { opacity: 1; transform: translateY(0); }
}

/* Confirm dialog */
.confirm-overlay {
  position: fixed; inset: 0; z-index: 1000;
  background: rgba(0,0,0,0.6); display: flex; align-items: center; justify-content: center;
}
.confirm-dialog {
  background: #1e2023; border: 1px solid rgba(255,255,255,0.12); border-radius: 10px;
  padding: 24px; max-width: 420px; width: 90%;
}
.confirm-title { font-size: 16px; font-weight: 600; color: #fff; margin-bottom: 12px; }
.confirm-msg { font-size: 13px; color: #b0b4ba; line-height: 1.5; margin-bottom: 20px; }
.confirm-msg strong { color: #e8eaec; }
.confirm-actions { display: flex; justify-content: flex-end; gap: 10px; }
.confirm-cancel {
  background: transparent; border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba;
  padding: 6px 16px; border-radius: 6px; font-size: 13px; cursor: pointer;
}
.confirm-cancel:hover { color: #fff; border-color: rgba(255,255,255,0.2); }
.confirm-delete {
  background: rgba(240, 84, 84, 0.15); border: 1px solid rgba(240, 84, 84, 0.3); color: #f05454;
  padding: 6px 16px; border-radius: 6px; font-size: 13px; font-weight: 600; cursor: pointer;
}
.confirm-delete:hover { background: rgba(240, 84, 84, 0.25); }
</style>
