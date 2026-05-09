<script setup>
import { ref, onMounted } from 'vue'
import { useResources } from '../../composables/useWails'

const props = defineProps({
  type: { type: String, default: 'networkpolicies' }
})

const { result, detail, loading, error, detailLoading, listResources, getResourceDetail } = useResources()

const policies = ref([])
const endpoints = ref([])
const itemDetail = ref(null)
const expandedItem = ref(null)
const notification = ref(null)

const resourceType = props.type === 'endpoints' ? 'endpoints' : 'networkpolicies'

function mapItems() {
  if (result.value && result.value.items && result.value.items.length > 0) {
    if (resourceType === 'networkpolicies') {
      policies.value = result.value.items.map(item => ({
        name: item.name,
        namespace: item.namespace,
        podSelector: item.fields?.pod_selector || '<none>',
        ingress: item.fields?.ingress === 'true' || item.fields?.ingress === true,
        egress: item.fields?.egress === 'true' || item.fields?.egress === true,
        age: item.age || '—'
      }))
    } else {
      endpoints.value = result.value.items.map(item => ({
        name: item.name,
        namespace: item.namespace,
        endpoints: item.fields?.endpoints || '—',
        age: item.age || '—'
      }))
    }
  } else {
    policies.value = []
    endpoints.value = []
  }
}

async function refresh(force = false) {
  await listResources(resourceType, '', force)
  mapItems()
}

onMounted(() => refresh())

const items = () => resourceType === 'networkpolicies' ? policies.value : endpoints.value

async function toggleExpand(itemName) {
  if (expandedItem.value === itemName) {
    expandedItem.value = null
    itemDetail.value = null
  } else {
    expandedItem.value = itemName
    const resourceType = props.type === 'endpoints' ? 'endpoints' : 'networkpolicies'
    const item = (resourceType === 'networkpolicies' ? policies.value : endpoints.value).find(i => i.name === itemName)
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
  <div class="np-view">
    <div class="header">
      <div class="header-row">
        <div>
          <div class="title" style="text-transform: capitalize;">{{ type }}</div>
          <div class="subtitle">{{ type === 'networkpolicies' ? 'Controls traffic flow at the IP address or port level' : 'Network endpoints for Services' }}</div>
        </div>
        <button class="refresh-btn" @click="refresh(true)" :disabled="loading">{{ loading ? 'Loading…' : '↻ Refresh' }}</button>
      </div>
    </div>
    
    <div v-if="notification" class="agent-notification">
      <div class="notif-icon">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"></path></svg>
      </div>
      <div class="notif-text">{{ notification }}</div>
    </div>

    <div class="np-scroll-area">
    <div v-if="loading && !items()" class="state-box">Loading…</div>
    <div v-else-if="error" class="state-box state-error">{{ error }}</div>
    <div v-else-if="!items().length" class="state-box">No {{ type }} found in this cluster.</div>

    <div v-else class="np-list">
      <div v-if="type === 'networkpolicies'" class="np-header-row np-grid">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-sel">Pod Selector</div>
        <div class="col-dir">Ingress</div>
        <div class="col-dir">Egress</div>
        <div class="col-age">Age</div>
      </div>
      <div v-else class="np-header-row ep-grid">
        <div class="col-name">Name</div>
        <div class="col-ns">Namespace</div>
        <div class="col-eps">Endpoints</div>
        <div class="col-age">Age</div>
      </div>

      <template v-if="type === 'networkpolicies'">
        <div v-for="p in policies" :key="p.name" class="np-row-container" :class="{'ai-active-pulse': p.isApplying}">
          <div class="np-row np-grid" @click="toggleExpand(p.name)">
            <div class="col-name">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #f43f5e; margin-right: 8px;"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
              {{ p.name }}
            </div>
            <div class="col-ns font-mono">{{ p.namespace }}</div>
            <div class="col-sel font-mono tag">{{ p.podSelector }}</div>
            <div class="col-dir">{{ p.ingress ? 'Yes' : 'No' }}</div>
            <div class="col-dir">{{ p.egress ? 'Yes' : 'No' }}</div>
            <div class="col-age font-mono" style="display:flex; justify-content:space-between; align-items:center;">
              {{ p.age }}
              <svg class="chevron" :class="{ open: expandedItem === p.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="6 9 12 15 18 9"></polyline>
              </svg>
            </div>
          </div>

          <!-- Expanded View -->
          <div class="np-expanded" v-if="expandedItem === p.name">
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

              <div class="expanded-card" v-if="itemDetail.labels && Object.keys(itemDetail.labels).length">
                <h4 class="card-title">Labels</h4>
                <div class="labels-grid">
                  <span class="label-chip" v-for="(v, k) in itemDetail.labels" :key="k">{{ k }}={{ v }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>
      
      <template v-else>
        <div v-for="e in endpoints" :key="e.name" class="np-row ep-grid">
          <div class="col-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #10b981; margin-right: 8px;"><circle cx="12" cy="12" r="10"></circle><circle cx="12" cy="12" r="3"></circle></svg>
            {{ e.name }}
          </div>
          <div class="col-ns font-mono">{{ e.namespace }}</div>
          <div class="col-eps font-mono">{{ e.endpoints }}</div>
          <div class="col-age font-mono">{{ e.age }}</div>
        </div>
      </template>
    </div>
    </div>
  </div>
</template>

<style scoped>
.np-view { padding: 24px; display: flex; flex-direction: column; gap: 24px; min-height: 0; flex: 1; box-sizing: border-box; }
.np-scroll-area { flex: 1; overflow-y: auto; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }
.header-row { display: flex; justify-content: space-between; align-items: flex-start; }
.refresh-btn { background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; padding: 6px 12px; border-radius: 6px; font-size: 12px; cursor: pointer; transition: all 0.15s; }
.refresh-btn:hover { background: rgba(255,255,255,0.1); color: #fff; }
.refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.state-box { padding: 40px; text-align: center; color: #8b8f96; font-size: 13px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.state-error { color: #f05454; }

.np-list { background: #1e2023; border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 8px; overflow: hidden; }

.np-header-row {
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

.np-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  transition: all 0.3s ease;
}
.np-row-container:last-child { border-bottom: none; }

.np-row {
  display: grid;
  gap: 16px;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}

.np-grid { grid-template-columns: 2fr 1fr 2fr 80px 80px 80px; }
.ep-grid { grid-template-columns: 2fr 1fr 3fr 80px; border-bottom: 1px solid rgba(255, 255, 255, 0.04); }
.ep-grid:last-child { border-bottom: none; }

.np-row:hover { background: rgba(255, 255, 255, 0.02); }

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
.np-expanded {
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

/* Labels */
.labels-grid { display: flex; flex-wrap: wrap; gap: 6px; }
.label-chip { background: rgba(55, 148, 255, 0.1); color: #3794ff; font-size: 11px; padding: 3px 8px; border-radius: 4px; font-family: var(--mono); }

/* Agent Notification */
.agent-notification { display: flex; align-items: center; gap: 12px; background: rgba(167, 139, 250, 0.15); border: 1px solid rgba(167, 139, 250, 0.3); padding: 12px 16px; border-radius: 6px; margin-bottom: 16px; color: #e8eaec; font-size: 13px; animation: slide-down 0.3s ease-out; }
.notif-icon { color: #a78bfa; display: flex; }
@keyframes slide-down { from { opacity: 0; transform: translateY(-10px); } to { opacity: 1; transform: translateY(0); } }

.col-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: var(--mono); color: #b0b4ba; font-size: 12px; }

.tag { background: rgba(255,255,255,0.05); padding: 4px 6px; border-radius: 4px; display: inline-block; border: 1px solid rgba(255,255,255,0.05); }
</style>
