<script setup>
import { ref } from 'vue'

const volumes = ref([
  { name: 'db-data-pvc', namespace: 'default', status: 'Bound', capacity: '50Gi', usedPct: 78, accessModes: 'RWO', storageClass: 'gp2', age: '145d' },
  { name: 'redis-data-pvc', namespace: 'default', status: 'Bound', capacity: '10Gi', usedPct: 34, accessModes: 'RWO', storageClass: 'gp2', age: '42d' },
  { name: 'shared-assets', namespace: 'public', status: 'Pending', capacity: '100Gi', usedPct: 0, accessModes: 'RWX', storageClass: 'efs-sc', age: '2h' },
])

function getUsageColor(pct) {
  if (pct > 85) return '#f05454' // Red if nearly full
  if (pct > 70) return '#f5a623' // Orange if getting high
  return '#3ecf8e' // Green otherwise
}
</script>

<template>
  <div class="vol-view">
    <div class="header">
      <div class="title">Persistent Volumes & Claims</div>
      <div class="subtitle">Storage resources available to your workloads</div>
    </div>

    <div class="vol-list">
      <div class="vol-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-status">Status</div>
        <div class="col-cap">Capacity</div>
        <div class="col-modes">Access Modes</div>
        <div class="col-sc">Storage Class</div>
      </div>

      <div v-for="v in volumes" :key="v.name" class="vol-row">
        <div class="col-name">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #3ecf8e; margin-right: 8px;"><ellipse cx="12" cy="5" rx="9" ry="3"></ellipse><path d="M21 12c0 1.66-4 3-9 3s-9-1.34-9-3"></path><path d="M3 5v14c0 1.66 4 3 9 3s9-1.34 9-3V5"></path></svg>
          {{ v.name }}
        </div>
        <div class="col-ns font-mono">{{ v.namespace }}</div>
        <div class="col-status">
          <span class="status-badge" :class="v.status.toLowerCase()">{{ v.status }}</span>
        </div>
        
        <div class="col-cap">
          <div class="cap-info">
            <span class="font-mono">{{ v.capacity }}</span>
            <span class="cap-used" v-if="v.status === 'Bound'">{{ v.usedPct }}% used</span>
          </div>
          <div class="cap-bar-wrapper" v-if="v.status === 'Bound'">
            <div class="cap-bar-fill" :style="{ width: v.usedPct + '%', background: getUsageColor(v.usedPct) }"></div>
          </div>
        </div>
        
        <div class="col-modes font-mono">{{ v.accessModes }}</div>
        <div class="col-sc font-mono">{{ v.storageClass }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.vol-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.vol-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.vol-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 180px 120px 1.5fr;
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

.vol-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 180px 120px 1.5fr;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.vol-row:last-child { border-bottom: none; }
.vol-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }

.status-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; background: rgba(255,255,255,0.05); color: #ccc; }
.status-badge.bound { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-badge.pending { background: rgba(245, 166, 35, 0.15); color: #f5a623; }

.col-cap { display: flex; flex-direction: column; gap: 8px; justify-content: center; }
.cap-info { display: flex; justify-content: space-between; align-items: center; }
.cap-used { font-size: 10px; color: #8b8f96; text-transform: uppercase; font-weight: 600; }

/* Glass of water effect */
.cap-bar-wrapper { 
  width: 100%; 
  height: 14px; 
  background: rgba(255, 255, 255, 0.02); 
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px; 
  overflow: hidden; 
  box-shadow: inset 0 2px 6px rgba(0,0,0,0.4);
  position: relative;
  backdrop-filter: blur(2px);
}

.cap-bar-fill { 
  height: 100%; 
  transition: width 0.8s cubic-bezier(0.4, 0, 0.2, 1); 
  box-shadow: inset 0 4px 6px rgba(255,255,255,0.2), inset 0 -2px 4px rgba(0,0,0,0.2);
  position: relative;
  border-right: 1px solid rgba(255,255,255,0.4);
}

/* Subtle liquid highlight */
.cap-bar-fill::after {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background: linear-gradient(to bottom, rgba(255,255,255,0.3) 0%, transparent 50%, rgba(0,0,0,0.1) 100%);
}
</style>
