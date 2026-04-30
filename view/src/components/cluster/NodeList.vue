<script setup>
import { ref, onMounted } from 'vue'
import { useResources } from '../../composables/useWails'

const { result, detail, loading, detailLoading, listResources, getResourceDetail } = useResources()

const mockNodes = [
  {
    name: 'ip-10-0-1-143.ec2.internal', status: 'Ready', roles: 'control-plane, master',
    version: 'v1.28.2', os: 'Ubuntu 22.04', cpuCapacity: '4', memCapacity: '15.6Gi',
    internalIp: '10.0.1.143', age: '14d2h'
  },
  {
    name: 'ip-10-0-2-45.ec2.internal', status: 'Ready', roles: 'worker',
    version: 'v1.28.2', os: 'Ubuntu 22.04', cpuCapacity: '8', memCapacity: '31.4Gi',
    internalIp: '10.0.2.45', age: '14d2h'
  },
  {
    name: 'ip-10-0-2-89.ec2.internal', status: 'NotReady', roles: 'worker',
    version: 'v1.28.2', os: 'Ubuntu 22.04', cpuCapacity: '8', memCapacity: '31.4Gi',
    internalIp: '10.0.2.89', age: '14d2h'
  }
]

const nodes = ref([])
const nodeDetail = ref(null)
const expandedNode = ref(null)
const logSearch = ref('')
const isStreamingLogs = ref(true)

onMounted(async () => {
  await listResources('nodes', '')
  if (result.value && result.value.items && result.value.items.length > 0) {
    nodes.value = result.value.items.map(mapNode)
  } else {
    nodes.value = mockNodes
  }
})

function mapNode(item) {
  return {
    name: item.name,
    status: item.status,
    roles: item.fields?.roles || '—',
    version: item.fields?.version || '—',
    os: item.fields?.os_image || '—',
    cpuCapacity: item.fields?.cpu_capacity || '—',
    memCapacity: item.fields?.mem_capacity || '—',
    internalIp: item.fields?.internal_ip || '—',
    age: item.age || '—',
    statusColor: item.statusColor
  }
}

async function toggleExpand(nodeName) {
  if (expandedNode.value === nodeName) {
    expandedNode.value = null
    nodeDetail.value = null
  } else {
    expandedNode.value = nodeName
    // Fetch full detail from backend.
    await getResourceDetail('nodes', '', nodeName)
    if (detail.value) {
      nodeDetail.value = detail.value
    }
  }
}

// Generate simple sparklines
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
const diskSpark = ref(generateSparkline(20))

// Mock logs
const nodeLogs = ref([
  { time: '14:32:01', level: 'INFO', msg: 'kubelet: Starting kubelet main sync loop.' },
  { time: '14:32:05', level: 'WARN', msg: 'containerd: garbage collection delayed.' },
  { time: '14:33:12', level: 'INFO', msg: 'kube-proxy: Successfully synced proxy rules.' },
  { time: '14:34:00', level: 'INFO', msg: 'kubelet: Node status updated successfully.' },
  { time: '14:35:10', level: 'INFO', msg: 'containerd: Image garbage collection complete.' }
])
</script>

<template>
  <div class="nodes-view">
    <div class="header">
      <div class="title">Cluster Nodes</div>
      <div class="subtitle">Physical and virtual machines hosting your workloads</div>
    </div>

    <div class="nodes-grid" :class="{ 'has-expanded': expandedNode !== null }">
      <div v-for="n in nodes" :key="n.name" 
           class="node-card" 
           :class="{
             'not-ready': n.status !== 'Ready',
             'is-expanded': expandedNode === n.name,
             'is-hidden': expandedNode !== null && expandedNode !== n.name
           }"
           @click="expandedNode === null && toggleExpand(n.name)">
        
        <div class="node-header">
          <div style="display:flex; align-items:center; gap:8px; cursor:pointer;" @click.stop="toggleExpand(n.name)">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #0e639c;">
              <rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect>
              <rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect>
              <line x1="6" y1="6" x2="6.01" y2="6"></line>
              <line x1="6" y1="18" x2="6.01" y2="18"></line>
            </svg>
            <span class="node-name">{{ n.name }}</span>
            <span v-if="expandedNode === n.name" class="collapse-icon">Collapse</span>
          </div>
          <div class="node-status" :class="n.status.toLowerCase()">{{ n.status }}</div>
        </div>

        <div class="node-meta">
          <div class="meta-item"><span class="meta-label">Roles:</span> {{ n.roles }}</div>
          <div class="meta-item"><span class="meta-label">Version:</span> {{ n.version }}</div>
          <div class="meta-item"><span class="meta-label">OS:</span> {{ n.os }}</div>
          <div class="meta-item"><span class="meta-label">Age:</span> {{ n.age }}</div>
        </div>

        <div class="node-resources" v-if="n.status === 'Ready'">
          <div class="resource-bar-container">
            <div class="res-label">CPU Capacity <span>{{ n.cpuCapacity }}</span></div>
          </div>
          <div class="resource-bar-container">
            <div class="res-label">Memory Capacity <span>{{ n.memCapacity }}</span></div>
          </div>
          <div class="resource-bar-container">
            <div class="res-label">Internal IP <span>{{ n.internalIp }}</span></div>
          </div>
        </div>

        <!-- Expanded Detailed View -->
        <div class="node-expanded-content" v-if="expandedNode === n.name">
          <div class="expanded-grid">
            
            <!-- Info Panel -->
            <div class="panel-section">
              <h4 class="section-title">System Information</h4>
              <div v-if="nodeDetail" class="info-list">
                <div class="info-row" v-for="prop in nodeDetail.properties" :key="prop.key">
                  <span class="label">{{ prop.key }}:</span>
                  <span class="val">{{ prop.value }}</span>
                </div>
              </div>
              <div v-else-if="detailLoading" class="info-list">
                <div class="info-row"><span class="label">Loading details...</span></div>
              </div>
              <div v-else class="info-list">
                <div class="info-row"><span class="label">Internal IP:</span> <span class="val">{{ n.internalIp }}</span></div>
                <div class="info-row"><span class="label">Version:</span> <span class="val">{{ n.version }}</span></div>
                <div class="info-row"><span class="label">OS:</span> <span class="val">{{ n.os }}</span></div>
              </div>
            </div>

            <!-- Advanced Metrics -->
            <div class="panel-section">
              <h4 class="section-title">Historical Metrics (1h)</h4>
              <div class="metrics-sparklines">
                <div class="spark-box">
                  <div class="spark-lbl">CPU Load</div>
                  <svg viewBox="0 0 100 100" preserveAspectRatio="none"><path :d="cpuSpark" fill="none" stroke="#f5a623" stroke-width="3" /></svg>
                </div>
                <div class="spark-box">
                  <div class="spark-lbl">Memory</div>
                  <svg viewBox="0 0 100 100" preserveAspectRatio="none"><path :d="memSpark" fill="none" stroke="#a78bfa" stroke-width="3" /></svg>
                </div>
                <div class="spark-box">
                  <div class="spark-lbl">Disk I/O</div>
                  <svg viewBox="0 0 100 100" preserveAspectRatio="none"><path :d="diskSpark" fill="none" stroke="#3ecf8e" stroke-width="3" /></svg>
                </div>
              </div>
            </div>
          </div>

          <!-- Log Streaming Area -->
          <div class="node-logs-section">
            <div class="logs-header">
              <h4 class="section-title">System Service Logs (kubelet, containerd)</h4>
              <div class="logs-controls">
                <input type="text" placeholder="Filter logs..." v-model="logSearch" class="log-search" />
                <button class="log-btn" :class="{active: isStreamingLogs}" @click.stop="isStreamingLogs = !isStreamingLogs">
                  <span class="pulse-dot" v-if="isStreamingLogs"></span>
                  {{ isStreamingLogs ? 'Streaming' : 'Paused' }}
                </button>
              </div>
            </div>
            <div class="logs-viewer">
              <div v-for="(line, i) in nodeLogs" :key="i" class="log-line" v-show="line.msg.includes(logSearch)">
                <span class="time">{{ line.time }}</span>
                <span class="lvl" :class="line.level.toLowerCase()">{{ line.level }}</span>
                <span class="msg">{{ line.msg }}</span>
              </div>
            </div>
          </div>
        </div>

      </div>
    </div>
  </div>
</template>

<style scoped>
.nodes-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  overflow-y: auto;
  height: 100%;
}
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.nodes-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.node-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  transition: all 0.3s cubic-bezier(0.2, 0.8, 0.2, 1);
}
.node-card:not(.is-expanded):not(.is-hidden):hover {
  border-color: rgba(255, 255, 255, 0.2);
  transform: translateY(-2px);
  cursor: pointer;
}

.node-card.not-ready { opacity: 0.7; border-color: rgba(240, 84, 84, 0.3); }

/* Expansion State */
.nodes-grid.has-expanded {
  display: flex;
  flex-direction: column;
}
.node-card.is-hidden {
  display: none;
}
.node-card.is-expanded {
  flex: 1;
  border-color: #0e639c;
  background: #141517;
  cursor: default;
}
.collapse-icon {
  font-size: 11px;
  color: #a5d6ff;
  background: rgba(165, 214, 255, 0.1);
  padding: 2px 6px;
  border-radius: 4px;
  margin-left: 8px;
}

.node-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.node-name { font-size: 14px; font-weight: 600; color: #e8eaec; font-family: 'SF Mono', Consolas, monospace; }

.node-status { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; letter-spacing: 0.05em; text-transform: uppercase; }
.node-status.ready { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.node-status.notready { background: rgba(240, 84, 84, 0.15); color: #f05454; }

.node-meta {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  font-size: 12px;
  color: #b0b4ba;
}
.meta-label { color: #6b7078; }

.node-resources {
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: #141517;
  padding: 12px;
  border-radius: 6px;
}
.resource-bar-container { display: flex; flex-direction: column; gap: 4px; }
.res-label { display: flex; justify-content: space-between; font-size: 11px; font-weight: 500; color: #8b8f96; }
.res-label span { font-family: 'SF Mono', Consolas, monospace; }
.res-track { width: 100%; height: 6px; background: rgba(255, 255, 255, 0.06); border-radius: 3px; overflow: hidden; }
.res-fill { height: 100%; transition: width 0.3s ease; }

/* Expanded Content */
.node-expanded-content {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px dashed rgba(255,255,255,0.1);
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.expanded-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}

.panel-section {
  background: #1e2023;
  border-radius: 6px;
  padding: 16px;
  border: 1px solid rgba(255,255,255,0.05);
}
.section-title {
  font-size: 13px;
  font-weight: 600;
  color: #e8eaec;
  margin-top: 0;
  margin-bottom: 12px;
}

.info-list { display: flex; flex-direction: column; gap: 8px; font-size: 12px; }
.info-row { display: flex; justify-content: space-between; align-items: flex-start; }
.info-row .label { color: #8b8f96; }
.info-row .val { color: #e8eaec; font-family: 'SF Mono', Consolas, monospace; text-align: right; }
.val.taints { display: flex; flex-direction: column; gap: 4px; align-items: flex-end; }
.badge { background: rgba(245, 166, 35, 0.15); color: #f5a623; padding: 2px 6px; border-radius: 4px; font-size: 11px; white-space: nowrap; }

.metrics-sparklines {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 12px;
}
.spark-box {
  background: #141517;
  border-radius: 4px;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 80px;
}
.spark-lbl { font-size: 11px; color: #8b8f96; text-align: center; }
.spark-box svg { width: 100%; height: 100%; }

/* Logs View */
.node-logs-section {
  display: flex;
  flex-direction: column;
  background: #0d0d0d;
  border-radius: 6px;
  border: 1px solid rgba(255,255,255,0.05);
  overflow: hidden;
}
.logs-header {
  padding: 12px 16px;
  background: #1a1a1a;
  border-bottom: 1px solid rgba(255,255,255,0.05);
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.logs-header .section-title { margin: 0; }
.logs-controls { display: flex; gap: 12px; align-items: center; }
.log-search {
  background: #2a2a2a;
  border: 1px solid #333;
  color: #fff;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  outline: none;
}
.log-search:focus { border-color: #0e639c; }

.log-btn {
  background: #2a2a2a;
  border: 1px solid #333;
  color: #b0b4ba;
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  display: flex; align-items: center; gap: 6px;
}
.log-btn.active { color: #3ecf8e; border-color: rgba(62, 207, 142, 0.3); background: rgba(62, 207, 142, 0.1); }
.pulse-dot { width: 6px; height: 6px; border-radius: 50%; background: #3ecf8e; animation: pulse 1.5s infinite; }
@keyframes pulse { 0% { opacity: 1;} 50% {opacity: 0.3;} 100% {opacity: 1;} }

.logs-viewer {
  padding: 12px;
  font-family: 'SF Mono', Consolas, monospace;
  font-size: 12px;
  color: #d4d4d4;
  height: 250px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}
.log-line { display: flex; gap: 12px; padding: 2px 0; border-bottom: 1px solid rgba(255,255,255,0.02); }
.log-line:hover { background: rgba(255,255,255,0.03); }
.time { color: #8b8f96; flex-shrink: 0; }
.lvl { flex-shrink: 0; width: 40px; font-weight: 600; }
.lvl.info { color: #3ecf8e; }
.lvl.warn { color: #f5a623; }
.lvl.error { color: #f05454; }
.msg { color: #d4d4d4; word-break: break-all; }
</style>
