<script setup>
import { ref } from 'vue'

const deployments = ref([
  { name: 'web-app', namespace: 'default', ready: 3, desired: 3, upToDate: 3, available: 3, image: 'web-app:v1.2.4', age: '14d' },
  { name: 'worker', namespace: 'default', ready: 5, desired: 5, upToDate: 5, available: 5, image: 'worker:latest', age: '14d' },
  { name: 'payment-service', namespace: 'finance', ready: 1, desired: 2, upToDate: 2, available: 1, image: 'payment:v2.0', age: '2h' },
  { name: 'nginx-ingress', namespace: 'kube-system', ready: 2, desired: 2, upToDate: 2, available: 2, image: 'ingress-nginx:v1.9.0', age: '145d' }
])
</script>

<template>
  <div class="deployments-view">
    <div class="header">
      <div class="title">Deployments</div>
      <div class="subtitle">Declarative updates for Pods and ReplicaSets</div>
    </div>

    <div class="deployments-grid">
      <div v-for="d in deployments" :key="d.name" class="dep-card" :class="{ 'degraded': d.ready < d.desired }">
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

.dep-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
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
</style>
