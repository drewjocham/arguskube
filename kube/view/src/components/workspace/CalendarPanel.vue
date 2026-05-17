<script setup>
// CalendarPanel — Google Calendar view. Lists this week's events with
// create/edit/delete. Shares the unified Google connection.

import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../../stores/workspace'
import GoogleAccountHeader from './GoogleAccountHeader.vue'

const store = useWorkspaceStore()
const {
  googleConnections, calendarEvents, calendarLoading, calendarError,
  calendarStatus,
} = storeToRefs(store)

const activeID = ref(null)

// Calendar range — defaults to this week.
function thisWeekStart() {
  const d = new Date()
  d.setDate(d.getDate() - d.getDay())
  d.setHours(0, 0, 0, 0)
  return d.toISOString()
}
function thisWeekEnd() {
  const d = new Date()
  d.setDate(d.getDate() + (6 - d.getDay()))
  d.setHours(23, 59, 59, 999)
  return d.toISOString()
}

const rangeStart = ref(thisWeekStart())
const rangeEnd = ref(thisWeekEnd())

// Create form
const summary = ref('')
const startTime = ref('')
const endTime = ref('')
const location = ref('')
const description = ref('')

const createDisabled = computed(
  () =>
    calendarLoading.value ||
    !activeID.value ||
    !summary.value.trim() ||
    !startTime.value ||
    !endTime.value,
)

const events = computed(() => {
  if (!activeID.value) return []
  return calendarEvents.value[activeID.value] || []
})

function formatTime(iso) {
  if (!iso) return ''
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function toDatetimeLocal(iso) {
  if (!iso) return ''
  // Convert RFC 3339 to datetime-local format (YYYY-MM-DDTHH:MM)
  try {
    const d = new Date(iso)
    return d.toISOString().slice(0, 16)
  } catch {
    return ''
  }
}

onMounted(async () => {
  await store.loadServices()
  if (!googleConnections.value.length) await store.loadConnections()
  if (!activeID.value && googleConnections.value.length) {
    activeID.value = googleConnections.value[0].id
    await store.loadCalendarEvents(activeID.value, rangeStart.value, rangeEnd.value)
  }
})

async function refreshEvents() {
  if (!activeID.value) return
  await store.loadCalendarEvents(activeID.value, rangeStart.value, rangeEnd.value)
}

async function createEvent() {
  if (!activeID.value) return
  await store.createCalendarEvent(activeID.value, {
    summary: summary.value,
    start: new Date(startTime.value).toISOString(),
    end: new Date(endTime.value).toISOString(),
    location: location.value,
    description: description.value,
  })
  summary.value = ''
  startTime.value = ''
  endTime.value = ''
  location.value = ''
  description.value = ''
}

async function deleteEvent(eventID) {
  if (!activeID.value || !confirm('Delete this event?')) return
  await store.deleteCalendarEvent(activeID.value, eventID)
}
</script>

<template>
  <div class="panel calendar-panel">
    <GoogleAccountHeader
      :connections="googleConnections"
      v-model:active-id="activeID"
      @change="refreshEvents"
    />

    <div v-if="calendarError" class="status error">{{ calendarError }}</div>
    <div
      v-if="calendarStatus"
      class="status ok"
    >{{ calendarStatus.op }}</div>

    <!-- Create form -->
    <div class="card create-card">
      <h4>New Event</h4>
      <div class="form-row">
        <input v-model="summary" placeholder="Event title" class="input" />
        <input v-model="location" placeholder="Location (optional)" class="input" />
      </div>
      <div class="form-row">
        <input v-model="startTime" type="datetime-local" class="input" />
        <input v-model="endTime" type="datetime-local" class="input" />
      </div>
      <textarea v-model="description" placeholder="Description (optional)" class="input" rows="2" />
      <button :disabled="createDisabled" @click="createEvent" class="btn primary">
        {{ calendarLoading ? 'Creating…' : 'Create Event' }}
      </button>
    </div>

    <!-- Event list -->
    <div class="card">
      <h4>
        Events
        <span class="range">{{ formatTime(rangeStart) }} – {{ formatTime(rangeEnd) }}</span>
      </h4>
      <div v-if="calendarLoading" class="muted">Loading…</div>
      <div v-else-if="!events.length" class="muted">No events this week.</div>
      <div v-for="ev in events" :key="ev.id" class="event-row">
        <div class="event-info">
          <strong>{{ ev.summary }}</strong>
          <span class="time">{{ formatTime(ev.start) }} – {{ formatTime(ev.end) }}</span>
          <span v-if="ev.location" class="loc">{{ ev.location }}</span>
        </div>
        <button @click="deleteEvent(ev.id)" class="btn small danger">Delete</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.calendar-panel { padding: 12px; }
.card { margin-bottom: 12px; padding: 12px; border: 1px solid var(--border); border-radius: 6px; }
.card h4 { margin: 0 0 8px; }
.range { font-weight: normal; font-size: 0.85em; color: var(--muted); }
.form-row { display: flex; gap: 8px; margin-bottom: 8px; }
.form-row .input { flex: 1; }
.input { width: 100%; padding: 6px 8px; border: 1px solid var(--border); border-radius: 4px; background: var(--bg); color: var(--fg); }
textarea.input { resize: vertical; }
.create-card { background: var(--bg-secondary); }
.event-row { display: flex; align-items: center; justify-content: space-between; padding: 6px 0; border-bottom: 1px solid var(--border); }
.event-row:last-child { border-bottom: none; }
.event-info { flex: 1; }
.event-info strong { display: block; }
.time, .loc { font-size: 0.85em; color: var(--muted); }
.loc { margin-left: 8px; }
.btn { padding: 6px 12px; border: 1px solid var(--border); border-radius: 4px; cursor: pointer; background: var(--bg); color: var(--fg); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn.primary { background: var(--accent); color: white; border-color: var(--accent); }
.btn.danger { color: var(--danger); border-color: var(--danger); }
.btn.small { padding: 2px 8px; font-size: 0.85em; }
.status { padding: 6px 12px; margin-bottom: 8px; border-radius: 4px; font-size: 0.9em; }
.status.error { background: var(--danger-bg); color: var(--danger); }
.status.ok { background: var(--ok-bg); color: var(--ok); }
.muted { color: var(--muted); padding: 12px 0; }
</style>
