<script setup>
import { ref } from 'vue'

const pods = ref([
  { name: 'web-app-58589998-f8q2b', namespace: 'default', status: 'Running', restarts: 0, node: 'ip-10-0-1-143', cpu: '120m', mem: '256Mi', age: '2d' },
  { name: 'worker-6b5d9db9f8-t29zj', namespace: 'default', status: 'Running', restarts: 2, node: 'ip-10-0-2-45', cpu: '45m', mem: '128Mi', age: '14h' },
  { name: 'redis-master-0', namespace: 'database', status: 'Running', restarts: 0, node: 'ip-10-0-2-89', cpu: '800m', mem: '2.4Gi', age: '14d' },
  { name: 'nginx-ingress-controller-xyz', namespace: 'kube-system', status: 'Running', restarts: 1, node: 'ip-10-0-1-143', cpu: '200m', mem: '300Mi', age: '14d' },
  { name: 'failed-job-abcde', namespace: 'default', status: 'Error', restarts: 5, node: 'ip-10-0-2-45', cpu: '-', mem: '-', age: '2m' }
])
</script>

<template>
  <div class="pods-view">
    <div class="header">
      <div class="title">Pods</div>
      <div class="subtitle">The smallest deployable computing units running your containers</div>
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

      <div v-for="p in pods" :key="p.name" class="pod-row" :class="p.status.toLowerCase()">
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
        <div class="col-age font-mono">{{ p.age }}</div>
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

.pod-row {
  display: grid;
  grid-template-columns: 2.5fr 1fr 100px 80px 1.5fr 80px 80px 60px;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.pod-row:last-child { border-bottom: none; }
.pod-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; color: #e8eaec; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }
.high-restarts { color: #f5a623; font-weight: 600; }

.status-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; }
.status-badge.running { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-badge.error { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.status-badge.pending { background: rgba(245, 166, 35, 0.15); color: #f5a623; }
</style>
