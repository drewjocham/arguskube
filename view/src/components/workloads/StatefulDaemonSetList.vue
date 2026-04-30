<script setup>
import { ref, onMounted } from 'vue'
import { useResources } from '../../composables/useWails'

const props = defineProps({
  type: { type: String, default: 'statefulsets' }
})

const { result, detail, loading, detailLoading, listResources, getResourceDetail } = useResources()

const mockWorkloads = [
  { name: 'redis-cluster', namespace: 'database', desired: 3, current: 3, ready: 3, age: '14d' },
  { name: 'elasticsearch', namespace: 'monitoring', desired: 5, current: 5, ready: 4, age: '42d' },
  { name: 'fluent-bit', namespace: 'kube-system', desired: 12, current: 12, ready: 12, age: '145d' },
]

const workloads = ref([])
const wsDetail = ref(null)
const expandedWs = ref(null)
const notification = ref(null)

onMounted(async () => {
  const resourceType = props.type || 'statefulsets'
  await listResources(resourceType, '')
  if (result.value && result.value.items && result.value.items.length > 0) {
    workloads.value = result.value.items.map(item => {
      const readyParts = (item.fields?.ready || '0/0').split('/')
      return {
        name: item.name,
        namespace: item.namespace,
        status: item.status,
        desired: parseInt(item.fields?.desired || readyParts[1] || '0'),
        current: parseInt(item.fields?.current || readyParts[0] || '0'),
        ready: parseInt(readyParts[0] || '0'),
        age: item.age || '—'
      }
    })
  } else {
    workloads.value = mockWorkloads
  }
})

async function toggleExpand(wsName) {
  if (expandedWs.value === wsName) {
    expandedWs.value = null
    wsDetail.value = null
  } else {
    expandedWs.value = wsName
    const resourceType = props.type || 'statefulsets'
    const ws = workloads.value.find(w => w.name === wsName)
    if (ws) {
      await getResourceDetail(resourceType, ws.namespace, wsName)
      if (detail.value) {
        wsDetail.value = detail.value
      }
    }
  }
}
</script>

<template>
  <div class="ws-view">
    <div class="header">
      <div class="title" style="text-transform: capitalize;">{{ type }}</div>
      <div class="subtitle">Long-running background compute workloads</div>
    </div>
    
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
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

      <div v-for="w in workloads" :key="w.name" class="ws-row-container" :class="{'ai-active-pulse': w.isApplying}">
        <div class="ws-row" @click="toggleExpand(w.name)">
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
          
          <div class="col-age font-mono" style="display:flex; justify-content:space-between; align-items:center;">
            {{ w.age }}
            <svg class="chevron" :class="{ open: expandedWs === w.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded View -->
        <div class="ws-expanded" v-if="expandedWs === w.name">
          <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
          <div v-else-if="wsDetail" class="expanded-grid">
            <div class="expanded-card">
              <h4 class="card-title">Properties</h4>
              <div class="props-grid">
                <div class="prop-row" v-for="prop in wsDetail.properties" :key="prop.key">
                  <span class="prop-label">{{ prop.key }}</span>
                  <span class="prop-value font-mono">{{ prop.value }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="wsDetail.conditions && wsDetail.conditions.length">
              <h4 class="card-title">Conditions</h4>
              <div class="conditions-list">
                <div class="condition-row" v-for="c in wsDetail.conditions" :key="c.type">
                  <span class="cond-type font-mono">{{ c.type }}</span>
                  <span class="cond-status" :class="c.status === 'True' ? 'ok' : 'fail'">{{ c.status }}</span>
                  <span class="cond-reason font-mono" v-if="c.reason">{{ c.reason }}</span>
                </div>
              </div>
            </div>

            <div class="expanded-card" v-if="wsDetail.events && wsDetail.events.length">
              <h4 class="card-title">Recent Events</h4>
              <div class="events-mini">
                <div class="event-mini-row" v-for="(ev, i) in wsDetail.events" :key="i" :class="ev.type?.toLowerCase()">
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

.ws-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: all 0.3s ease;
}
.ws-row-container:last-child { border-bottom: none; }

.ws-row {
  display: grid;
  grid-template-columns: 2fr 1fr 80px 80px 80px 1.5fr 80px;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.ws-row:hover { background: rgba(255, 255, 255, 0.02); }

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
.ws-expanded {
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

.health-bar-container {
  width: 100%; height: 6px; background: rgba(255, 255, 255, 0.06); border-radius: 3px; overflow: hidden;
}
.health-bar-fill {
  height: 100%; background: #3ecf8e; transition: width 0.3s ease;
}
.health-bar-fill.degraded { background: #f5a623; }
</style>
