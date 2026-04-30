<script setup>
import { ref } from 'vue'

const props = defineProps({
  type: { type: String, default: 'statefulsets' }
})

const workloads = ref([
  { name: 'redis-cluster', namespace: 'database', desired: 3, current: 3, ready: 3, age: '14d' },
  { name: 'elasticsearch', namespace: 'monitoring', desired: 5, current: 5, ready: 4, age: '42d' },
  { name: 'fluent-bit', namespace: 'kube-system', desired: 12, current: 12, ready: 12, age: '145d' },
])
</script>

<template>
  <div class="ws-view">
    <div class="header">
      <div class="title" style="text-transform: capitalize;">{{ type }}</div>
      <div class="subtitle">Long-running background compute workloads</div>
    </div>

    <div class="ws-list">
      <div class="ws-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-num">Desired</div>
        <div class="col-num">Current</div>
        <div class="col-num">Ready</div>
        <div class="col-health">Health</div>
        <div class="col-age">Age</div>
      </div>

      <div v-for="w in workloads" :key="w.name" class="ws-row">
        <div class="col-name">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #fb923c; margin-right: 8px;"><polygon points="12 2 2 7 12 12 22 7 12 2"></polygon><polyline points="2 17 12 22 22 17"></polyline><polyline points="2 12 12 17 22 12"></polyline></svg>
          {{ w.name }}
        </div>
        <div class="col-ns font-mono">{{ w.namespace }}</div>
        <div class="col-num font-mono">{{ w.desired }}</div>
        <div class="col-num font-mono">{{ w.current }}</div>
        <div class="col-num font-mono">{{ w.ready }}</div>
        
        <div class="col-health">
          <div class="health-bar-container">
            <div class="health-bar-fill" :style="{ width: (w.ready / w.desired * 100) + '%' }" :class="{'degraded': w.ready < w.desired}"></div>
          </div>
        </div>
        
        <div class="col-age font-mono">{{ w.age }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ws-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.ws-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.ws-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 80px 80px 80px 1.5fr 80px;
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

.ws-row {
  display: grid;
  grid-template-columns: 2fr 1fr 80px 80px 80px 1.5fr 80px;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.ws-row:last-child { border-bottom: none; }
.ws-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }

.health-bar-container {
  width: 100%; height: 6px; background: rgba(255, 255, 255, 0.06); border-radius: 3px; overflow: hidden;
}
.health-bar-fill {
  height: 100%; background: #3ecf8e; transition: width 0.3s ease;
}
.health-bar-fill.degraded { background: #f5a623; }
</style>
