<script setup>
import { ref } from 'vue'

const hpas = ref([
  { name: 'web-app-hpa', namespace: 'default', reference: 'Deployment/web-app', targets: '24% / 80%', minPods: 3, maxPods: 10, replicas: 3, age: '14d' },
  { name: 'payment-hpa', namespace: 'finance', reference: 'Deployment/payment-service', targets: '91% / 80%', minPods: 2, maxPods: 5, replicas: 5, age: '2h' },
])

function isOverTarget(targetStr) {
  const [current, target] = targetStr.split(' / ').map(s => parseInt(s.replace('%','')))
  return current >= target
}
</script>

<template>
  <div class="hpa-view">
    <div class="header">
      <div class="title">Horizontal Pod Autoscalers</div>
      <div class="subtitle">Automatic scaling based on observed CPU utilization</div>
    </div>

    <div class="hpa-list">
      <div class="hpa-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-ref">Reference</div>
        <div class="col-tar">Targets</div>
        <div class="col-min">MinPods</div>
        <div class="col-max">MaxPods</div>
        <div class="col-rep">Replicas</div>
        <div class="col-age">Age</div>
      </div>

      <div v-for="h in hpas" :key="h.name" class="hpa-row">
        <div class="col-name">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #60a5fa; margin-right: 8px;"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline></svg>
          {{ h.name }}
        </div>
        <div class="col-ns font-mono">{{ h.namespace }}</div>
        <div class="col-ref font-mono">{{ h.reference }}</div>
        
        <div class="col-tar">
          <span class="target-badge" :class="{'alert': isOverTarget(h.targets)}">{{ h.targets }}</span>
        </div>
        
        <div class="col-min font-mono">{{ h.minPods }}</div>
        <div class="col-max font-mono">{{ h.maxPods }}</div>
        <div class="col-rep font-mono" :class="{'maxed': h.replicas === h.maxPods}">{{ h.replicas }}</div>
        <div class="col-age font-mono">{{ h.age }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.hpa-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.hpa-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.hpa-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 2fr 120px 80px 80px 80px 80px;
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

.hpa-row {
  display: grid;
  grid-template-columns: 2fr 1fr 2fr 120px 80px 80px 80px 80px;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.hpa-row:last-child { border-bottom: none; }
.hpa-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }

.target-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; background: rgba(255,255,255,0.05); color: #ccc; font-family: 'SF Mono', Consolas, monospace; border: 1px solid rgba(255,255,255,0.05); }
.target-badge.alert { background: rgba(240, 84, 84, 0.15); color: #f05454; border-color: rgba(240, 84, 84, 0.3); }

.maxed { color: #f05454; font-weight: 600; }
</style>
