<script setup>
import { ref } from 'vue'

const services = ref([
  { name: 'kubernetes', namespace: 'default', type: 'ClusterIP', clusterIP: '10.96.0.1', externalIP: '<none>', ports: '443/TCP', age: '145d' },
  { name: 'web-app-svc', namespace: 'default', type: 'ClusterIP', clusterIP: '10.101.45.12', externalIP: '<none>', ports: '80/TCP', age: '14d' },
  { name: 'kube-dns', namespace: 'kube-system', type: 'ClusterIP', clusterIP: '10.96.0.10', externalIP: '<none>', ports: '53/UDP, 53/TCP', age: '145d' },
  { name: 'ingress-nginx-controller', namespace: 'kube-system', type: 'LoadBalancer', clusterIP: '10.104.12.88', externalIP: '192.168.1.100', ports: '80:31245/TCP, 443:32456/TCP', age: '145d' }
])
</script>

<template>
  <div class="svc-view">
    <div class="header">
      <div class="title">Services</div>
      <div class="subtitle">Network abstractions exposing applications running on Pods</div>
    </div>

    <div class="svc-list">
      <div class="svc-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-type">Type</div>
        <div class="col-ip">Cluster IP</div>
        <div class="col-ext">External IP</div>
        <div class="col-ports">Ports</div>
      </div>

      <div v-for="s in services" :key="s.name" class="svc-row">
        <div class="col-name">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #f5a623; margin-right: 8px;"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path></svg>
          {{ s.name }}
        </div>
        <div class="col-ns font-mono">{{ s.namespace }}</div>
        <div class="col-type">
          <span class="type-badge" :class="s.type.toLowerCase()">{{ s.type }}</span>
        </div>
        <div class="col-ip font-mono">{{ s.clusterIP }}</div>
        <div class="col-ext font-mono">{{ s.externalIP }}</div>
        <div class="col-ports font-mono">{{ s.ports }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.svc-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.svc-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.svc-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 1.5fr 1.5fr 1.5fr;
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

.svc-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 1.5fr 1.5fr 1.5fr;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.svc-row:last-child { border-bottom: none; }
.svc-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }

.type-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; background: rgba(255,255,255,0.05); color: #ccc; }
.type-badge.loadbalancer { background: rgba(245, 166, 35, 0.15); color: #f5a623; }
</style>
