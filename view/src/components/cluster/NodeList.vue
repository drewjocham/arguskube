<script setup>
import { ref, onMounted } from 'vue'

const nodes = ref([
  {
    name: 'ip-10-0-1-143.ec2.internal',
    status: 'Ready',
    roles: ['control-plane', 'master'],
    version: 'v1.28.2',
    os: 'Ubuntu 22.04',
    cpuUsage: 25,
    memUsage: 68,
    diskUsage: 45,
    uptime: '14d 2h'
  },
  {
    name: 'ip-10-0-2-45.ec2.internal',
    status: 'Ready',
    roles: ['worker'],
    version: 'v1.28.2',
    os: 'Ubuntu 22.04',
    cpuUsage: 82,
    memUsage: 91,
    diskUsage: 78,
    uptime: '14d 2h'
  },
  {
    name: 'ip-10-0-2-89.ec2.internal',
    status: 'NotReady',
    roles: ['worker'],
    version: 'v1.28.2',
    os: 'Ubuntu 22.04',
    cpuUsage: 0,
    memUsage: 0,
    diskUsage: 0,
    uptime: '14d 2h'
  }
])

function getUsageColor(pct) {
  if (pct > 85) return '#f05454'
  if (pct > 70) return '#f5a623'
  return '#3ecf8e'
}
</script>

<template>
  <div class="nodes-view">
    <div class="header">
      <div class="title">Cluster Nodes</div>
      <div class="subtitle">Physical and virtual machines hosting your workloads</div>
    </div>

    <div class="nodes-grid">
      <div v-for="n in nodes" :key="n.name" class="node-card" :class="{'not-ready': n.status !== 'Ready'}">
        <div class="node-header">
          <div style="display:flex; align-items:center; gap:8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #0e639c;">
              <rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect>
              <rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect>
              <line x1="6" y1="6" x2="6.01" y2="6"></line>
              <line x1="6" y1="18" x2="6.01" y2="18"></line>
            </svg>
            <span class="node-name">{{ n.name }}</span>
          </div>
          <div class="node-status" :class="n.status.toLowerCase()">{{ n.status }}</div>
        </div>

        <div class="node-meta">
          <div class="meta-item"><span class="meta-label">Roles:</span> {{ n.roles.join(', ') }}</div>
          <div class="meta-item"><span class="meta-label">Version:</span> {{ n.version }}</div>
          <div class="meta-item"><span class="meta-label">OS:</span> {{ n.os }}</div>
          <div class="meta-item"><span class="meta-label">Uptime:</span> {{ n.uptime }}</div>
        </div>

        <div class="node-resources" v-if="n.status === 'Ready'">
          <div class="resource-bar-container">
            <div class="res-label">CPU <span>{{ n.cpuUsage }}%</span></div>
            <div class="res-track">
              <div class="res-fill" :style="{ width: n.cpuUsage + '%', background: getUsageColor(n.cpuUsage) }"></div>
            </div>
          </div>
          <div class="resource-bar-container">
            <div class="res-label">Memory <span>{{ n.memUsage }}%</span></div>
            <div class="res-track">
              <div class="res-fill" :style="{ width: n.memUsage + '%', background: getUsageColor(n.memUsage) }"></div>
            </div>
          </div>
          <div class="resource-bar-container">
            <div class="res-label">Disk <span>{{ n.diskUsage }}%</span></div>
            <div class="res-track">
              <div class="res-fill" :style="{ width: n.diskUsage + '%', background: getUsageColor(n.diskUsage) }"></div>
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
}
.node-card.not-ready { opacity: 0.7; border-color: rgba(240, 84, 84, 0.3); }

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
</style>
