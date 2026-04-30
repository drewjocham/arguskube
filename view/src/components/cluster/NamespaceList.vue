<script setup>
import { ref } from 'vue'

const namespaces = ref([
  { name: 'default', status: 'Active', pods: 12, cpuLimits: '4.5', memLimits: '12Gi', age: '145d' },
  { name: 'kube-system', status: 'Active', pods: 24, cpuLimits: 'none', memLimits: 'none', age: '145d' },
  { name: 'monitoring', status: 'Active', pods: 8, cpuLimits: '2.0', memLimits: '8Gi', age: '42d' },
  { name: 'ingress-nginx', status: 'Active', pods: 3, cpuLimits: '1.0', memLimits: '1Gi', age: '145d' },
  { name: 'test-env', status: 'Terminating', pods: 0, cpuLimits: '0', memLimits: '0', age: '2d' }
])
</script>

<template>
  <div class="ns-view">
    <div class="header">
      <div class="title">Namespaces</div>
      <div class="subtitle">Logical partitions of your cluster resources</div>
    </div>

    <div class="ns-list">
      <div class="ns-header-row">
        <div class="ns-col">Name</div>
        <div class="ns-col">Status</div>
        <div class="ns-col">Pods</div>
        <div class="ns-col">CPU Quota</div>
        <div class="ns-col">Memory Quota</div>
        <div class="ns-col">Age</div>
      </div>

      <div v-for="ns in namespaces" :key="ns.name" class="ns-row" :class="ns.status.toLowerCase()">
        <div class="ns-col ns-name">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #a78bfa; margin-right: 8px;">
            <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path>
          </svg>
          {{ ns.name }}
        </div>
        <div class="ns-col">
          <span class="status-badge" :class="ns.status.toLowerCase()">{{ ns.status }}</span>
        </div>
        <div class="ns-col font-mono">{{ ns.pods }}</div>
        <div class="ns-col font-mono">{{ ns.cpuLimits }}</div>
        <div class="ns-col font-mono">{{ ns.memLimits }}</div>
        <div class="ns-col font-mono">{{ ns.age }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ns-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  overflow-y: auto;
  height: 100%;
}
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.ns-list {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  overflow: hidden;
}

.ns-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr 1fr 1fr;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
  font-weight: 600;
  color: #8b8f96;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.ns-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr 1fr 1fr;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.ns-row:last-child { border-bottom: none; }
.ns-row:hover { background: rgba(255, 255, 255, 0.02); }

.ns-row.terminating { opacity: 0.6; }

.ns-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; }

.status-badge {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
}
.status-badge.active { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-badge.terminating { background: rgba(245, 166, 35, 0.15); color: #f5a623; }
</style>
