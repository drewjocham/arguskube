<script setup>
import { ref } from 'vue'

const events = ref([
  { id: 1, type: 'Warning', reason: 'FailedScheduling', object: 'pod/web-app-58589998-f8q2b', message: '0/3 nodes are available: 3 Insufficient cpu.', age: '2m' },
  { id: 2, type: 'Normal', reason: 'Scheduled', object: 'pod/worker-6b5d9db9f8-t29zj', message: 'Successfully assigned default/worker to ip-10-0-1-143.ec2.internal', age: '5m' },
  { id: 3, type: 'Normal', reason: 'Pulling', object: 'pod/worker-6b5d9db9f8-t29zj', message: 'Pulling image "node:18-alpine"', age: '5m' },
  { id: 4, type: 'Normal', reason: 'Created', object: 'pod/worker-6b5d9db9f8-t29zj', message: 'Created container worker', age: '5m' },
  { id: 5, type: 'Warning', reason: 'BackOff', object: 'pod/old-app-deployment-6df446-2j98d', message: 'Back-off restarting failed container', age: '12m' },
])
</script>

<template>
  <div class="events-view">
    <div class="header">
      <div class="title">Cluster Events</div>
      <div class="subtitle">Real-time stream of state changes and warnings</div>
    </div>

    <div class="events-list">
      <div class="event-header-row">
        <div class="col-type">Type</div>
        <div class="col-reason">Reason</div>
        <div class="col-object">Object</div>
        <div class="col-message">Message</div>
        <div class="col-age">Age</div>
      </div>

      <div v-for="e in events" :key="e.id" class="event-row" :class="e.type.toLowerCase()">
        <div class="col-type">
          <span class="type-pill" :class="e.type.toLowerCase()">{{ e.type }}</span>
        </div>
        <div class="col-reason font-mono">{{ e.reason }}</div>
        <div class="col-object">{{ e.object }}</div>
        <div class="col-message">{{ e.message }}</div>
        <div class="col-age font-mono">{{ e.age }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.events-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  overflow-y: auto;
  height: 100%;
}
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.events-list {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  overflow: hidden;
}

.event-header-row {
  display: grid;
  grid-template-columns: 100px 150px 200px 1fr 60px;
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

.event-row {
  display: grid;
  grid-template-columns: 100px 150px 200px 1fr 60px;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.event-row:last-child { border-bottom: none; }
.event-row:hover { background: rgba(255, 255, 255, 0.02); }

.event-row.warning {
  background: rgba(245, 166, 35, 0.03);
  border-left: 3px solid #f5a623;
  padding-left: 13px;
}
.event-row.normal {
  border-left: 3px solid transparent;
  padding-left: 13px;
}

.font-mono { font-family: 'SF Mono', Consolas, monospace; }

.col-object { color: #a78bfa; font-family: 'SF Mono', Consolas, monospace; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.col-message { color: #b0b4ba; line-height: 1.4; }

.type-pill {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
}
.type-pill.normal { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.type-pill.warning { background: rgba(245, 166, 35, 0.15); color: #f5a623; }
</style>
