<script setup>
import { ref, onMounted, computed } from 'vue'
import { callGo } from '../../composables/useBridge'

const role = ref('all')
const data = ref(null)
const loading = ref(false)
const error = ref(null)

const roles = [
  { id: 'all', label: 'All' },
  { id: 'operator', label: 'Operator' },
  { id: 'developer', label: 'Developer' },
]

onMounted(() => fetchStatus())

async function fetchStatus() {
  loading.value = true
  error.value = null
  data.value = null
  try {
    const result = await callGo('GetGatewayStatusByRole', role.value)
    data.value = result
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

function setRole(r) {
  role.value = r
  fetchStatus()
}

const hasIssues = computed(() => {
  if (!data.value) return false
  const items = data.value.gatewayClasses || data.value.gateways || data.value.httpRoutes || []
  return false
})

function condClass(cond) {
  if (cond.status === 'True') return 'ok'
  return 'fail'
}
</script>

<template>
  <div class="status-view">
    <div class="header">
      <div class="title">Gateway Status Dashboard</div>
      <div class="subtitle">Gateway API resource health organized by persona</div>
    </div>

    <div class="persona-bar">
      <div class="persona-label">View as:</div>
      <button
        v-for="r in roles"
        :key="r.id"
        class="persona-btn"
        :class="{ active: role === r.id }"
        @click="setRole(r.id)"
      >{{ r.label }}</button>
      <div class="persona-spacer"></div>
      <button class="btn-refresh" @click="fetchStatus">Refresh</button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>
    <div v-if="loading" class="state-box">Loading status…</div>

    <template v-if="!loading && data">
      <!-- Operator view: GatewayClasses + Gateways -->
      <template v-if="role === 'operator' || role === 'all'">
        <div v-if="data.gatewayClasses?.length > 0" class="resource-block">
          <div class="section-title">GatewayClasses</div>
          <div class="class-list">
            <div v-for="cls in data.gatewayClasses" :key="cls.name" class="class-card">
              <div class="class-header">
                <span class="class-name">{{ cls.name }}</span>
                <span class="class-params font-mono" v-if="cls.className">{{ cls.className }}</span>
              </div>
              <div v-if="cls.conditions?.length" class="cond-list">
                <div v-for="cond in cls.conditions" :key="cond.type" class="cond-row" :class="condClass(cond)">
                  <span class="cond-type">{{ cond.type }}</span>
                  <span class="cond-status-badge">{{ cond.status }}</span>
                  <span class="cond-reason font-mono">{{ cond.reason }}</span>
                  <span class="cond-msg" v-if="cond.message">{{ cond.message }}</span>
                </div>
              </div>
              <div v-else class="no-conds">No conditions reported</div>
            </div>
          </div>
        </div>

        <div v-if="data.gateways?.length > 0" class="resource-block">
          <div class="section-title">Gateways</div>
          <div class="gw-table">
            <div class="gw-table-header">
              <div class="h-name">Name</div>
              <div class="h-ns">Namespace</div>
              <div class="h-class">Class</div>
              <div class="h-addrs">Addresses</div>
              <div class="h-conds">Conditions</div>
            </div>
            <div v-for="gw in data.gateways" :key="gw.name + gw.namespace" class="gw-table-row">
              <div class="gw-name font-mono">{{ gw.name }}</div>
              <div class="gw-ns">{{ gw.namespace }}</div>
              <div class="gw-class">{{ gw.className }}</div>
              <div class="gw-addrs font-mono">{{ gw.addresses?.join(', ') || '—' }}</div>
              <div class="gw-conds">
                <span
                  v-for="cond in gw.conditions"
                  :key="cond.type"
                  class="cond-pill"
                  :class="condClass(cond)"
                >{{ cond.type }}={{ cond.status }}</span>
                <span v-if="!gw.conditions?.length" class="no-data">—</span>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- Developer view: HTTPRoutes -->
      <template v-if="role === 'developer' || role === 'all'">
        <div v-if="data.httpRoutes?.length > 0" class="resource-block">
          <div class="section-title">HTTPRoutes</div>
          <div class="route-table">
            <div class="route-table-header">
              <div class="h-name">Name</div>
              <div class="h-ns">Namespace</div>
              <div class="h-hosts">Hostnames</div>
              <div class="h-rules">Rules</div>
              <div class="h-conds">Conditions</div>
            </div>
            <div v-for="route in data.httpRoutes" :key="route.name + route.namespace" class="route-table-row">
              <div class="route-name font-mono">{{ route.name }}</div>
              <div class="route-ns">{{ route.namespace }}</div>
              <div class="route-hosts">{{ route.hostnames?.join(', ') || '—' }}</div>
              <div class="route-rules">{{ route.matches }}</div>
              <div class="route-conds">
                <span
                  v-for="cond in route.conditions"
                  :key="cond.type"
                  class="cond-pill"
                  :class="condClass(cond)"
                >{{ cond.type }}={{ cond.status }}</span>
                <span v-if="!route.conditions?.length" class="no-data">—</span>
              </div>
            </div>
          </div>
        </div>
      </template>

      <div v-if="!data.gatewayClasses?.length && !data.gateways?.length && !data.httpRoutes?.length" class="state-box">
        No Gateway API resources found
      </div>
    </template>
  </div>
</template>

<style scoped>
.status-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.persona-bar { display: flex; align-items: center; gap: 4px; padding: 8px 12px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.persona-label { font-size: 11px; color: #6b7078; text-transform: uppercase; letter-spacing: 0.05em; margin-right: 8px; }
.persona-btn { padding: 4px 12px; font-size: 12px; background: rgba(255,255,255,0.04); border: 1px solid rgba(255,255,255,0.08); color: #8b8f96; border-radius: 5px; cursor: pointer; transition: all 0.1s; }
.persona-btn:hover { background: rgba(255,255,255,0.1); color: #e8eaec; }
.persona-btn.active { background: rgba(167,139,250,0.15); border-color: rgba(167,139,250,0.3); color: #a78bfa; font-weight: 500; }
.persona-spacer { flex: 1; }
.btn-refresh { padding: 4px 12px; font-size: 12px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 5px; cursor: pointer; }
.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.resource-block { display: flex; flex-direction: column; gap: 8px; }
.section-title { font-size: 14px; font-weight: 600; color: #e8eaec; }

.class-list { display: flex; flex-direction: column; gap: 6px; }
.class-card { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 14px 16px; display: flex; flex-direction: column; gap: 8px; }
.class-header { display: flex; align-items: center; gap: 8px; }
.class-name { font-size: 13px; font-weight: 500; color: #e8eaec; }
.class-params { font-size: 11px; color: #a78bfa; }

.cond-list { display: flex; flex-direction: column; gap: 3px; }
.cond-row { display: flex; align-items: center; gap: 8px; padding: 4px 8px; border-radius: 4px; font-size: 11px; }
.cond-row.ok { background: rgba(62,207,142,0.08); }
.cond-row.fail { background: rgba(240,84,84,0.08); }
.cond-type { font-weight: 500; min-width: 100px; }
.cond-row.ok .cond-type { color: #3ecf8e; }
.cond-row.fail .cond-type { color: #f05454; }
.cond-status-badge { padding: 1px 5px; border-radius: 3px; font-size: 10px; font-weight: 600; }
.cond-row.ok .cond-status-badge { background: rgba(62,207,142,0.15); color: #3ecf8e; }
.cond-row.fail .cond-status-badge { background: rgba(240,84,84,0.15); color: #f05454; }
.cond-reason { color: #8b8f96; }
.cond-msg { color: #6b7078; flex: 1; }
.no-conds { font-size: 11px; color: #6b7078; font-style: italic; }

.gw-table, .route-table { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; overflow: hidden; }
.gw-table-header, .route-table-header { display: grid; gap: 8px; padding: 10px 14px; background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.08); color: #8b8f96; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; font-size: 10.5px; }
.gw-table-row, .route-table-row { display: grid; gap: 8px; padding: 10px 14px; border-bottom: 1px solid rgba(255,255,255,0.04); font-size: 12px; align-items: center; color: #e8eaec; }
.gw-table-row:last-child, .route-table-row:last-child { border-bottom: none; }
.gw-table-header { grid-template-columns: 1.5fr 1fr 1fr 1.5fr 2fr; }
.gw-table-row { grid-template-columns: 1.5fr 1fr 1fr 1.5fr 2fr; }
.route-table-header { grid-template-columns: 1.5fr 1fr 2fr 60px 2fr; }
.route-table-row { grid-template-columns: 1.5fr 1fr 2fr 60px 2fr; }

.gw-name { color: #a78bfa; }
.gw-ns { color: #8b8f96; }
.gw-addrs { color: #8b8f96; }
.route-name { color: #a78bfa; }
.route-ns { color: #8b8f96; }
.route-hosts { color: #b0b4ba; }
.route-rules { text-align: center; }

.gw-conds, .route-conds { display: flex; gap: 4px; flex-wrap: wrap; }
.cond-pill { padding: 2px 6px; border-radius: 3px; font-size: 10px; font-weight: 500; }
.cond-pill.ok { background: rgba(62,207,142,0.12); color: #3ecf8e; }
.cond-pill.fail { background: rgba(240,84,84,0.12); color: #f05454; }

.state-box { padding: 24px; text-align: center; color: #8b8f96; font-size: 13px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.no-data { color: #6b7078; font-style: italic; }
.font-mono { font-family: var(--mono); }
</style>
