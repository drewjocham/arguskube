<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  type: { type: String, default: 'jobs' }
})

const jobs = ref([
  { name: 'db-backup', namespace: 'database', completions: '1/1', duration: '45s', status: 'Complete', age: '14h' },
  { name: 'data-sync-worker', namespace: 'default', completions: '0/1', duration: '2m', status: 'Running', age: '2m' },
  { name: 'failed-migration', namespace: 'public', completions: '0/1', duration: '12s', status: 'Failed', age: '1d' },
])

const cronjobs = ref([
  { name: 'nightly-backup', namespace: 'database', schedule: '0 0 * * *', suspend: false, active: 0, lastSchedule: '14h', age: '145d' },
  { name: 'log-rotation', namespace: 'kube-system', schedule: '*/15 * * * *', suspend: false, active: 1, lastSchedule: '4m', age: '42d' },
  { name: 'stale-cleanup', namespace: 'default', schedule: '0 2 * * 0', suspend: true, active: 0, lastSchedule: '4d', age: '12d' },
])

const list = computed(() => props.type === 'cronjobs' ? cronjobs.value : jobs.value)
</script>

<template>
  <div class="job-view">
    <div class="header">
      <div class="title">{{ type === 'cronjobs' ? 'CronJobs' : 'Jobs' }}</div>
      <div class="subtitle">{{ type === 'cronjobs' ? 'Time-based recurring batch workloads' : 'One-off transient batch workloads' }}</div>
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

      <div v-for="item in list" :key="item.name" class="job-row" :class="type">
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

        <div class="col-age font-mono">{{ item.age }}</div>
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

.job-row {
  display: grid;
  gap: 16px;
  padding: 14px 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
}
.job-row.jobs { grid-template-columns: 2fr 1.5fr 100px 100px 100px 80px; }
.job-row.cronjobs { grid-template-columns: 2fr 1.5fr 120px 80px 80px 120px 80px; }
.job-row:last-child { border-bottom: none; }
.job-row:hover { background: rgba(255, 255, 255, 0.02); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }

.schedule-box { background: rgba(0,0,0,0.2); padding: 4px 6px; border-radius: 4px; display: inline-block; color: #c084fc; border: 1px solid rgba(255,255,255,0.05); }

.status-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; background: rgba(255,255,255,0.05); color: #ccc; }
.status-badge.complete { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-badge.failed { background: rgba(240, 84, 84, 0.15); color: #f05454; }
.status-badge.running { background: rgba(55, 148, 255, 0.15); color: #3794ff; }
</style>
