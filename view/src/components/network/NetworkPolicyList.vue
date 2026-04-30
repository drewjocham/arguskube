<script setup>
import { ref } from 'vue'

const props = defineProps({
  type: { type: String, default: 'networkpolicies' }
})

const policies = ref([
  { name: 'default-deny-all', namespace: 'default', podSelector: '<none>', ingress: false, egress: false, age: '145d' },
  { name: 'allow-dns', namespace: 'default', podSelector: '<none>', ingress: false, egress: true, age: '145d' },
  { name: 'api-allow', namespace: 'backend', podSelector: 'app=api', ingress: true, egress: true, age: '42d' },
])

const endpoints = ref([
  { name: 'kubernetes', namespace: 'default', endpoints: '10.0.1.12:8443', age: '145d' },
  { name: 'web-app-svc', namespace: 'default', endpoints: '10.0.2.45:80, 10.0.1.143:80', age: '14d' },
])
</script>

<template>
  <div class="np-view">
    <div class="header">
      <div class="title" style="text-transform: capitalize;">{{ type }}</div>
      <div class="subtitle">{{ type === 'networkpolicies' ? 'Controls traffic flow at the IP address or port level' : 'Network endpoints for Services' }}</div>
    </div>

    <div class="np-list">
      <div v-if="type === 'networkpolicies'" class="np-header-row np-grid">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-sel">Pod Selector</div>
        <div class="col-dir">Ingress</div>
        <div class="col-dir">Egress</div>
        <div class="col-age">Age</div>
      </div>
      <div v-else class="np-header-row ep-grid">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-eps">Endpoints</div>
        <div class="col-age">Age</div>
      </div>

      <template v-if="type === 'networkpolicies'">
        <div v-for="p in policies" :key="p.name" class="np-row np-grid">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #f43f5e; margin-right: 8px;"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
            {{ p.name }}
          </div>
          <div class="col-ns font-mono">{{ p.namespace }}</div>
          <div class="col-sel font-mono tag">{{ p.podSelector }}</div>
          <div class="col-dir">{{ p.ingress ? 'Yes' : 'No' }}</div>
          <div class="col-dir">{{ p.egress ? 'Yes' : 'No' }}</div>
          <div class="col-age font-mono">{{ p.age }}</div>
        </div>
      </template>
      
      <template v-else>
        <div v-for="e in endpoints" :key="e.name" class="np-row ep-grid">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #10b981; margin-right: 8px;"><circle cx="12" cy="12" r="10"></circle><circle cx="12" cy="12" r="3"></circle></svg>
            {{ e.name }}
          </div>
          <div class="col-ns font-mono">{{ e.namespace }}</div>
          <div class="col-eps font-mono">{{ e.endpoints }}</div>
          <div class="col-age font-mono">{{ e.age }}</div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.np-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.np-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.np-header-row {
  display: grid;
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

.np-row {
  display: grid;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}

.np-grid { grid-template-columns: 2fr 1fr 2fr 80px 80px 80px; }
.ep-grid { grid-template-columns: 2fr 1fr 3fr 80px; }

.np-row:last-child { border-bottom: none; }
.np-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }

.tag { background: rgba(255,255,255,0.05); padding: 4px 6px; border-radius: 4px; display: inline-block; border: 1px solid rgba(255,255,255,0.05); }
</style>
