<script setup>
import { ref, computed, onMounted } from 'vue'
import { useResources } from '../../composables/useWails'

const props = defineProps({
  type: { type: String, default: 'jobs' }
})

const { result, detail, loading, detailLoading, listResources, getResourceDetail } = useResources()

const mockJobs = [
  { name: 'db-backup', namespace: 'database', completions: '1/1', duration: '45s', status: 'Complete', age: '14h' },
  { name: 'data-sync-worker', namespace: 'default', completions: '0/1', duration: '2m', status: 'Running', age: '2m' },
  { name: 'failed-migration', namespace: 'public', completions: '0/1', duration: '12s', status: 'Failed', age: '1d' },
]

const mockCronjobs = [
  { name: 'nightly-backup', namespace: 'database', schedule: '0 0 * * *', suspend: false, active: 0, lastSchedule: '14h', age: '145d' },
  { name: 'log-rotation', namespace: 'kube-system', schedule: '*/15 * * * *', suspend: false, active: 1, lastSchedule: '4m', age: '42d' },
  { name: 'stale-cleanup', namespace: 'default', schedule: '0 2 * * 0', suspend: true, active: 0, lastSchedule: '4d', age: '12d' },
]

const jobs = ref([])
const cronjobs = ref([])
const itemDetail = ref(null)
const expandedItem = ref(null)
const notification = ref(null)

const list = computed(() => props.type === 'cronjobs' ? cronjobs.value : jobs.value)

async function fetchData() {
  const resourceType = props.type === 'cronjobs' ? 'cronjobs' : 'jobs'
  try {
    await listResources(resourceType, '')
    if (result.value && result.value.items && result.value.items.length > 0) {
      if (resourceType === 'jobs') {
        jobs.value = result.value.items.map(item => ({
          name: item.name,
          namespace: item.namespace,
          completions: item.fields?.completions || '0/0',
          duration: item.fields?.duration || '—',
          status: item.status || '—',
          age: item.age || '—'
        }))
      } else {
        cronjobs.value = result.value.items.map(item => ({
          name: item.name,
          namespace: item.namespace,
          schedule: item.fields?.schedule || '—',
          suspend: item.fields?.suspend === 'True' || item.fields?.suspend === true,
          active: parseInt(item.fields?.active || '0'),
          lastSchedule: item.fields?.last_schedule || '—',
          age: item.age || '—'
        }))
      }
    } else {
      jobs.value = mockJobs
      cronjobs.value = mockCronjobs
    }
  } catch (e) {
    console.error('[JobCronJobList] fetch failed:', e)
    jobs.value = mockJobs
    cronjobs.value = mockCronjobs
  }
}

onMounted(fetchData)

async function toggleExpand(itemName) {
  if (expandedItem.value === itemName) {
    expandedItem.value = null
    itemDetail.value = null
  } else {
    expandedItem.value = itemName
    const resourceType = props.type === 'cronjobs' ? 'cronjobs' : 'jobs'
    const item = list.value.find(i => i.name === itemName)
    if (item) {
      await getResourceDetail(resourceType, item.namespace, itemName)
      if (detail.value) {
        itemDetail.value = detail.value
      }
    }
  }
}
</script>

<template>
  <div class="job-view">
    <div class="header">
      <div class="title">{{ type === 'cronjobs' ? 'CronJobs' : 'Jobs' }}</div>
      <div class="subtitle">{{ type === 'cronjobs' ? 'Time-based recurring batch workloads' : 'One-off transient batch workloads' }}</div>
    </div>
    
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
    </div>

    <div class="job-list">
      <div class="job-header-row" :class="type">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        
        <template v-if="type === 'cronjobs'">
          <div class="col-schedule">Schedule</div>
          <div class="col-suspend">Suspend</div>
          <div class="col-active">Active</div>
          <div class="col-last">Last Schedule</div>
        </template>
        <template v-else>
          <div class="col-comps">Completions</div>
          <div class="col-dur">Duration</div>
          <div class="col-status">Status</div>
        </template>
        
        <div class="col-age">Age</div>
      </div>

      <div v-for="item in list" :key="item.name" class="job-row-container" :class="{'ai-active-pulse': item.isApplying}">
        <div class="job-row" :class="type" @click="toggleExpand(item.name)">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #c084fc; margin-right: 8px;"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"></rect><line x1="16" y1="2" x2="16" y2="6"></line><line x1="8" y1="2" x2="8" y2="6"></line><line x1="3" y1="10" x2="21" y2="10"></line></svg>
            {{ item.name }}
          </div>
          <div class="col-ns font-mono">{{ item.namespace }}</div>
          
          <template v-if="type === 'cronjobs'">
            <div class="col-schedule font-mono schedule-box">{{ item.schedule }}</div>
            <div class="col-suspend">{{ item.suspend ? 'True' : 'False' }}</div>
            <div class="col-active font-mono">{{ item.active }}</div>
            <div class="col-last font-mono">{{ item.lastSchedule }}</div>
          </template>
          <template v-else>
            <div class="col-comps font-mono">{{ item.completions }}</div>
            <div class="col-dur font-mono">{{ item.duration }}</div>
            <div class="col-status">
              <span class="status-badge" :class="item.status.toLowerCase()">{{ item.status }}</span>
            </div>
          </template>

          <div class="col-age font-mono" style="display:flex; justify-content:space-between; align-items:center;">
            {{ item.age }}
            <svg class="chevron" :class="{ open: expandedItem === item.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded View -->
        <div class="job-expanded" v-if="expandedItem === item.name">
          <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
          <div v-else-if="itemDetail" class="expanded-grid">
            <div class="expanded-card">
              <h4 class="card-title">Properties</h4>
              <div class="props-grid">
                <div class="prop-row" v-for="prop in itemDetail.properties" :key="prop.key">
                  <span class="prop-label">{{ prop.key }}</span>
                  <span class="prop-value font-mono">{{ prop.value }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="itemDetail.conditions && itemDetail.conditions.length">
              <h4 class="card-title">Conditions</h4>
              <div class="conditions-list">
                <div class="condition-row" v-for="c in itemDetail.conditions" :key="c.type">
                  <span class="cond-type font-mono">{{ c.type }}</span>
                  <span class="cond-status" :class="c.status === 'True' ? 'ok' : 'fail'">{{ c.status }}</span>
                  <span class="cond-reason font-mono" v-if="c.reason">{{ c.reason }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="itemDetail.events && itemDetail.events.length">
              <h4 class="card-title">Recent Events</h4>
              <div class="events-mini">
                <div class="event-mini-row" v-for="(ev, i) in itemDetail.events" :key="i" :class="ev.type?.toLowerCase()">
                  <span class="ev-type">{{ ev.type }}</span>
                  <span class="ev-reason font-mono">{{ ev.reason }}</span>
                  <span class="ev-msg">{{ ev.message }}</span>
                  <span class="ev-age font-mono">{{ ev.age }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.job-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; text-transform: capitalize; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.job-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.job-header-row {
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
.job-header-row.jobs { grid-template-columns: 2fr 1.5fr 100px 100px 100px 80px; }
.job-header-row.cronjobs { grid-template-columns: 2fr 1.5fr 120px 80px 80px 120px 80px; }

.job-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: all 0.3s ease;
}
.job-row-container:last-child { border-bottom: none; }

.job-row {
  display: grid;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.job-row.jobs { grid-template-columns: 2fr 1.5fr 100px 100px 100px 80px; }
.job-row.cronjobs { grid-template-columns: 2fr 1.5fr 120px 80px 80px 120px 80px; }
.job-row:hover { background: rgba(255, 255, 255, 0.02); }

.chevron { transition: transform 0.2s ease; color: #6b7078; }
.chevron.open { transform: rotate(180deg); }

/* Pulse Animation */
@keyframes pulse-glow {
  0% { box-shadow: inset 0 0 0px rgba(167, 139, 250, 0); background: transparent; }
  50% { box-shadow: inset 0 0 10px rgba(167, 139, 250, 0.4); background: rgba(167, 139, 250, 0.05); }
  100% { box-shadow: inset 0 0 0px rgba(167, 139, 250, 0); background: transparent; }
}
.ai-active-pulse {
  animation: pulse-glow 2s infinite;
  border-left: 3px solid #a78bfa;
}

/* Expanded Area */
.job-expanded {
  padding: 16px;
  background: #141517;
  border-top: 1px dashed rgba(255,255,255,0.08);
}
.expanded-grid {
  display: flex;
  flex-direction: column;
}
.expanded-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
  padding: 16px;
  display: flex;
  flex-direction: column;
}
.card-title { font-size: 13px; font-weight: 600; color: #fff; margin: 0 0 12px 0; }

/* Properties */
.props-grid { display: flex; flex-direction: column; gap: 4px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }

/* Conditions */
.conditions-list { display: flex; flex-direction: column; gap: 4px; }
.condition-row { display: flex; align-items: center; gap: 12px; font-size: 12px; }
.cond-type { color: #e8eaec; min-width: 120px; }
.cond-status { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; }
.cond-status.ok { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.cond-status.fail { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.cond-reason { color: #8b8f96; }

/* Mini Events */
.events-mini { display: flex; flex-direction: column; gap: 4px; max-height: 200px; overflow-y: auto; }
.event-mini-row { display: grid; grid-template-columns: 60px 120px 1fr 50px; gap: 8px; font-size: 11px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); align-items: center; }
.event-mini-row.warning { color: #f5a623; }
.event-mini-row.normal { color: #b0b4ba; }
.ev-type { font-weight: 600; }
.ev-reason { color: #a78bfa; }
.ev-msg { color: #8b8f96; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.ev-age { color: #6b7078; text-align: right; }

/* Agent Notification */
.agent-notification { display: flex; align-items: center; gap: 12px; background: rgba(167, 139, 250, 0.15); border: 1px solid rgba(167, 139, 250, 0.3); padding: 12px 16px; border-radius: 6px; margin-bottom: 16px; color: #e8eaec; font-size: 13px; animation: slide-down 0.3s ease-out; }
.notif-icon { color: #a78bfa; display: flex; }
@keyframes slide-down { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }

.schedule-box { background: rgba(0,0,0,0.2); padding: 4px 6px; border-radius: 4px; display: inline-block; color: #c084fc; border: 1px solid rgba(255,255,255,0.05); }

.status-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; background: rgba(255,255,255,0.05); color: #ccc; }
.status-badge.complete { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-badge.failed { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.status-badge.running { background: rgba(55, 148, 255, 0.15); color: #3794ff; }
</style>
