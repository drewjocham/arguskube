<script setup>
import { ref, onMounted } from 'vue'
import { useResources } from '../../composables/useWails'

const props = defineProps({
  type: { type: String, default: 'services' }
})

const { result, detail, loading, detailLoading, listResources, getResourceDetail } = useResources()

const resourceKind = props.type || 'services'

const mockServices = [
  { name: 'kubernetes', namespace: 'default', type: 'ClusterIP', clusterIP: '10.96.0.1', externalIP: '<none>', ports: '443/TCP', age: '145d' },
  { name: 'web-app-svc', namespace: 'default', type: 'ClusterIP', clusterIP: '10.101.45.12', externalIP: '<none>', ports: '80/TCP', age: '14d' },
  { name: 'kube-dns', namespace: 'kube-system', type: 'ClusterIP', clusterIP: '10.96.0.10', externalIP: '<none>', ports: '53/UDP, 53/TCP', age: '145d' },
  { name: 'ingress-nginx-controller', namespace: 'kube-system', type: 'LoadBalancer', clusterIP: '10.104.12.88', externalIP: '192.168.1.100', ports: '80:31245/TCP, 443:32456/TCP', age: '145d' }
]

const services = ref([])
const svcDetail = ref(null)
const expandedSvc = ref(null)
const notification = ref(null)

onMounted(async () => {
  await listResources(resourceKind, '')
  if (result.value && result.value.items && result.value.items.length > 0) {
    services.value = result.value.items.map(item => ({
      name: item.name,
      namespace: item.namespace,
      status: item.status,
      type: item.fields?.type || 'ClusterIP',
      clusterIP: item.fields?.cluster_ip || '—',
      externalIP: item.fields?.external_ip || '<none>',
      ports: item.fields?.ports || '—',
      age: item.age || '—'
    }))
  } else {
    services.value = mockServices
  }
})

async function toggleExpand(svcName) {
  if (expandedSvc.value === svcName) {
    expandedSvc.value = null
    svcDetail.value = null
  } else {
    expandedSvc.value = svcName
    const svc = services.value.find(s => s.name === svcName)
    if (svc) {
      await getResourceDetail(resourceKind, svc.namespace, svcName)
      if (detail.value) {
        svcDetail.value = detail.value
      }
    }
  }
}
</script>

<template>
  <div class="svc-view">
    <div class="header">
      <div class="title">Services</div>
      <div class="subtitle">Network abstractions exposing applications running on Pods</div>
    </div>
    
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
    </div>

    <div class="svc-list">
      <div class="svc-header-row">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-type">Type</div>
        <div class="col-ip">Cluster IP</div>
        <div class="col-ext">External IP</div>
        <div class="col-ports">Ports</div>
      </div>

      <div v-for="s in services" :key="s.name" class="svc-row-container">
        <div class="svc-row" @click="toggleExpand(s.name)">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #f5a623; margin-right: 8px;"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path></svg>
            {{ s.name }}
          </div>
          <div class="col-ns font-mono">{{ s.namespace }}</div>
          <div class="col-type">
            <span class="type-badge" :class="s.type.toLowerCase()">{{ s.type }}</span>
          </div>
          <div class="col-ip font-mono">{{ s.clusterIP }}</div>
          <div class="col-ext font-mono">{{ s.externalIP }}</div>
          <div class="col-ports font-mono" style="display:flex; justify-content:space-between; align-items:center;">
            {{ s.ports }}
            <svg class="chevron" :class="{ open: expandedSvc === s.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded Service Details -->
        <div class="svc-expanded" v-if="expandedSvc === s.name">
          <div v-if="detailLoading" style="color:#8b8f96; font-size:13px; padding:12px;">Loading…</div>
          <div v-else-if="svcDetail" class="expanded-grid">

            <!-- Properties -->
            <div class="expanded-card">
              <h4 class="card-title">Service Properties</h4>
              <div class="props-grid">
                <div class="prop-row" v-for="prop in svcDetail.properties" :key="prop.key">
                  <span class="prop-label">{{ prop.key }}</span>
                  <span class="prop-value font-mono">{{ prop.value }}</span>
                </div>
              </div>
            </div>

            <!-- Labels & Events -->
            <div class="expanded-card">
              <div v-if="svcDetail.labels && Object.keys(svcDetail.labels).length" style="margin-bottom:16px;">
                <h4 class="card-title">Labels</h4>
                <div class="labels-grid">
                  <span class="label-chip" v-for="(v, k) in svcDetail.labels" :key="k">{{ k }}={{ v }}</span>
                </div>
              </div>

              <div v-if="svcDetail.events && svcDetail.events.length">
                <h4 class="card-title">Recent Events</h4>
                <div class="events-mini">
                  <div class="event-mini-row" v-for="(ev, i) in svcDetail.events" :key="i" :class="ev.type?.toLowerCase()">
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
  </div>
</template>

<style scoped>
.svc-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; overflow-y: auto; height: 100%; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.svc-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.svc-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 1.5fr 1.5fr 1.5fr;
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

.svc-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}
.svc-row-container:last-child { border-bottom: none; }

.svc-row {
  display: grid;
  grid-template-columns: 2fr 1fr 100px 1.5fr 1.5fr 1.5fr;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.svc-row:hover { background: rgba(255, 255, 255, 0.03); }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; font-size: 12px; }

.chevron { transition: transform 0.2s ease; color: #6b7078; }
.chevron.open { transform: rotate(180deg); }

.type-badge { font-size: 11px; padding: 2px 6px; border-radius: 4px; font-weight: 600; background: rgba(255,255,255,0.05); color: #ccc; }
.type-badge.loadbalancer { background: rgba(245, 166, 35, 0.15); color: #f5a623; }

/* Expanded Area */
.svc-expanded {
  padding: 16px;
  background: #141517;
  border-top: 1px dashed rgba(255,255,255,0.08);
}
.expanded-grid {
  display: flex;
  gap: 24px;
}
.expanded-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
  padding: 16px;
  flex: 1;
}
.card-title {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  margin: 0 0 16px 0;
}

/* Properties */
.props-grid { display: flex; flex-direction: column; gap: 4px; }
.prop-row { display: flex; justify-content: space-between; font-size: 12px; padding: 4px 0; border-bottom: 1px solid rgba(255,255,255,0.03); }
.prop-label { color: #8b8f96; }
.prop-value { color: #e8eaec; max-width: 60%; text-align: right; word-break: break-all; }

/* Labels */
.labels-grid { display: flex; flex-wrap: wrap; gap: 6px; }
.label-chip { background: rgba(55, 148, 255, 0.1); color: #3794ff; font-size: 11px; padding: 3px 8px; border-radius: 4px; font-family: 'SF Mono', Consolas, monospace; }

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
.agent-notification {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(167, 139, 250, 0.15);
  border: 1px solid rgba(167, 139, 250, 0.3);
  padding: 12px 16px;
  border-radius: 6px;
  margin-bottom: 16px;
  color: #e8eaec;
  font-size: 13px;
  animation: slide-down 0.3s ease-out;
}
.notif-icon { color: #a78bfa; display: flex; }
@keyframes slide-down {
  from { opacity: 0; transform: translateY(-10px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
