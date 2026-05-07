<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useResources } from '../../composables/useWails'

const { result, loading, listResources } = useResources()


const events = ref([])
const autoRefresh = ref(true)
const filterType = ref('all')
let refreshTimer = null

async function fetchEvents() {
  await listResources('events', '')
  if (result.value && result.value.items && result.value.items.length > 0) {
    events.value = result.value.items.map((item, i) => ({
      id: i,
      type: item.fields?.type || item.status || 'Normal',
      reason: item.fields?.reason || '—',
      object: item.fields?.object || '—',
      message: item.fields?.message || '—',
      age: item.age || '—',
      count: item.fields?.count || '1'
    }))
  } else {
    events.value = []
  }
}

function startAutoRefresh() {
  stopAutoRefresh()
  refreshTimer = setInterval(fetchEvents, 10000)
}
function stopAutoRefresh() {
  if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
}
function toggleAutoRefresh() {
  autoRefresh.value = !autoRefresh.value
  if (autoRefresh.value) startAutoRefresh()
  else stopAutoRefresh()
}

const displayEvents = computed(() => {
  if (filterType.value === 'all') return events.value
  return events.value.filter(e => e.type.toLowerCase() === filterType.value)
})

onMounted(async () => {
  await fetchEvents()
  if (autoRefresh.value) startAutoRefresh()
})
onUnmounted(() => stopAutoRefresh())
</script>

<template>
  <div class="events-view">
    <div class="header">
      <div class="header-text">
        <div class="title">Cluster Events</div>
        <div class="subtitle">Real-time stream of state changes and warnings</div>
      </div>
      <div class="header-controls">
        <div class="filter-group">
          <button class="filter-btn" :class="{ active: filterType === 'all' }" @click="filterType = 'all'">All</button>
          <button class="filter-btn" :class="{ active: filterType === 'warning' }" @click="filterType = 'warning'">Warnings</button>
          <button class="filter-btn" :class="{ active: filterType === 'normal' }" @click="filterType = 'normal'">Normal</button>
        </div>
        <button class="refresh-btn" @click="fetchEvents" :disabled="loading">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6"></path><path d="M1 20v-6h6"></path><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
        </button>
        <button class="auto-btn" :class="{ active: autoRefresh }" @click="toggleAutoRefresh">
          {{ autoRefresh ? 'Live' : 'Paused' }}
        </button>
      </div>
    </div>

    <div class="event-count">{{ displayEvents.length }} events</div>

    <div class="events-list">
      <div class="event-header-row">
        <div class="col-type">Type</div>
        <div class="col-reason">Reason</div>
        <div class="col-object">Object</div>
        <div class="col-message">Message</div>
        <div class="col-age">Age</div>
      </div>

      <div v-for="e in displayEvents" :key="e.id" class="event-row" :class="e.type.toLowerCase()">
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
.header { display: flex; justify-content: space-between; align-items: flex-start; }
.header-text .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header-text .subtitle { font-size: 13px; color: #8b8f96; }
.header-controls { display: flex; align-items: center; gap: 8px; }
.filter-group { display: flex; gap: 4px; }
.filter-btn {
  background: transparent; border: 1px solid rgba(255,255,255,0.1); color: #8b8f96;
  padding: 4px 10px; border-radius: 4px; font-size: 12px; cursor: pointer; transition: all 0.2s;
}
.filter-btn.active { background: rgba(255,255,255,0.08); color: #fff; border-color: rgba(255,255,255,0.2); }
.refresh-btn {
  background: transparent; border: 1px solid rgba(255,255,255,0.1); color: #8b8f96;
  padding: 5px 8px; border-radius: 4px; cursor: pointer; display: flex; align-items: center; transition: all 0.2s;
}
.refresh-btn:hover { color: #fff; border-color: rgba(255,255,255,0.2); }
.auto-btn {
  background: transparent; border: 1px solid rgba(255,255,255,0.1); color: #8b8f96;
  padding: 4px 10px; border-radius: 4px; font-size: 12px; cursor: pointer; transition: all 0.2s;
}
.auto-btn.active { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; border-color: rgba(62, 207, 142, 0.3); }
.event-count { font-size: 12px; color: #8b8f96; }

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

.font-mono { font-family: var(--mono); }

.col-object { color: #a78bfa; font-family: var(--mono); font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
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
