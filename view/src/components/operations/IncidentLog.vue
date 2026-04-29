<script setup>
import { ref, computed } from 'vue'
import { useChat } from '../../composables/useWails'

const { eventLog, fetchEventLog } = useChat()

// Fetch on mount.
fetchEventLog()

const filterType = ref('all')

const filteredEvents = computed(() => {
  let events = [...(eventLog.value || [])].reverse()
  if (filterType.value !== 'all') {
    events = events.filter(e => e.type === filterType.value)
  }
  return events
})

const counts = computed(() => {
  const c = { alert: 0, resolution: 0, investigation: 0, pattern: 0 }
  for (const e of eventLog.value || []) {
    if (c[e.type] !== undefined) c[e.type]++
  }
  return c
})

function formatTimestamp(ts) {
  if (!ts) return '—'
  const d = new Date(ts)
  const today = new Date()
  const isToday = d.toDateString() === today.toDateString()
  if (isToday) return d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  return d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short' }) + ' ' +
    d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
}

function sevColor(sev) {
  switch (sev) {
    case 'critical': return 'var(--red)'
    case 'warning': return 'var(--amber)'
    default: return 'var(--text3)'
  }
}

function typeColor(type) {
  switch (type) {
    case 'alert': return 'var(--red2)'
    case 'resolution': return 'var(--green2)'
    case 'investigation': return 'var(--accent2)'
    case 'pattern': return 'var(--purple)'
    default: return 'var(--text3)'
  }
}

function typeLabel(type) {
  switch (type) {
    case 'alert': return 'ALERT'
    case 'resolution': return 'RESOLVED'
    case 'investigation': return 'INVESTIGATED'
    case 'pattern': return 'PATTERN'
    default: return type.toUpperCase()
  }
}
</script>

<template>
  <div class="incident-log-view">
    <div class="view-header">
      <div class="view-title-row">
        <div class="view-title">Incident Log</div>
        <button class="refresh-btn" @click="fetchEventLog">Refresh</button>
      </div>
      <div class="view-sub">Argus-tracked events, investigations, and detected patterns</div>
    </div>

    <!-- Filter pills -->
    <div class="filter-row">
      <div class="filter-pill" :class="{ active: filterType === 'all' }" @click="filterType = 'all'">
        All ({{ eventLog.length }})
      </div>
      <div class="filter-pill filter-alert" :class="{ active: filterType === 'alert' }" @click="filterType = 'alert'">
        Alerts ({{ counts.alert }})
      </div>
      <div class="filter-pill filter-resolved" :class="{ active: filterType === 'resolution' }" @click="filterType = 'resolution'">
        Resolved ({{ counts.resolution }})
      </div>
      <div class="filter-pill filter-investigated" :class="{ active: filterType === 'investigation' }" @click="filterType = 'investigation'">
        Investigated ({{ counts.investigation }})
      </div>
    </div>

    <!-- Event list -->
    <div class="event-list">
      <div v-for="(event, i) in filteredEvents" :key="i" class="event-row">
        <div class="event-time">{{ formatTimestamp(event.timestamp) }}</div>
        <div class="event-type-badge" :style="{ color: typeColor(event.type) }">
          {{ typeLabel(event.type) }}
        </div>
        <div class="event-summary">{{ event.summary }}</div>
        <div v-if="event.namespace" class="event-ns">{{ event.namespace }}</div>
        <div v-if="event.severity" class="event-sev-dot" :style="{ background: sevColor(event.severity) }"></div>
      </div>

      <div v-if="filteredEvents.length === 0" class="empty-state">
        No events recorded yet — the agent will track alerts, investigations, and patterns automatically
      </div>
    </div>
  </div>
</template>

<style scoped>
.incident-log-view { display: flex; flex-direction: column; gap: 10px; }

.view-header { margin-bottom: 2px; }
.view-title-row { display: flex; align-items: center; justify-content: space-between; }
.view-title { font-size: 14px; font-weight: 500; color: var(--text); }
.view-sub { font-size: 12px; color: var(--text3); margin-top: 2px; }

.refresh-btn {
  padding: 4px 10px; border-radius: 5px; font-size: 11px; font-weight: 500;
  cursor: pointer; border: 1px solid var(--border2); background: var(--bg3);
  color: var(--text2); font-family: var(--font); transition: all 0.1s;
}
.refresh-btn:hover { background: var(--bg4); color: var(--text); }

.filter-row { display: flex; gap: 4px; }
.filter-pill {
  padding: 3px 9px; border-radius: 10px; font-size: 10.5px; font-weight: 500;
  font-family: var(--mono); cursor: pointer; border: 1px solid var(--border);
  background: var(--bg3); color: var(--text2); transition: all 0.1s;
}
.filter-pill:hover { background: var(--bg4); }
.filter-pill.active { border-color: var(--accent); color: var(--accent2); background: rgba(79,142,247,0.1); }
.filter-alert.active { border-color: var(--red); color: var(--red2); background: rgba(240,84,84,0.1); }
.filter-resolved.active { border-color: var(--green); color: var(--green2); background: rgba(62,207,142,0.06); }
.filter-investigated.active { border-color: var(--accent); color: var(--accent2); background: rgba(79,142,247,0.08); }

.event-list { display: flex; flex-direction: column; }

.event-row {
  display: flex; align-items: center; gap: 8px; padding: 8px 10px;
  border-bottom: 1px solid var(--border); transition: background 0.08s;
}
.event-row:hover { background: var(--bg3); }

.event-time { font-size: 10.5px; font-family: var(--mono); color: var(--text3); width: 70px; flex-shrink: 0; }
.event-type-badge { font-size: 9px; font-weight: 600; font-family: var(--mono); letter-spacing: 0.05em; width: 90px; flex-shrink: 0; }
.event-summary { font-size: 12px; color: var(--text2); flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.event-ns { font-size: 10px; font-family: var(--mono); color: var(--accent2); padding: 1px 5px; background: rgba(79,142,247,0.08); border-radius: 3px; flex-shrink: 0; }
.event-sev-dot { width: 5px; height: 5px; border-radius: 50%; flex-shrink: 0; }

.empty-state { text-align: center; padding: 40px; color: var(--text3); font-size: 12px; line-height: 1.6; }
</style>
