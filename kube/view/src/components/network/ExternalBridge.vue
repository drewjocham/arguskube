<script setup>
import { ref, onMounted } from 'vue'
import { callGo } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const bridges = ref([])
const loading = ref(false)
const error = ref(null)
const showForm = ref(false)
const form = ref({
  name: '',
  namespace: 'default',
  type: 'manual',
  externalName: '',
  externalIPs: '',
  ports: [{ name: 'default', port: 80, targetPort: 80, protocol: 'TCP' }],
})
const creating = ref(false)
const namespaces = ref([])

onMounted(async () => {
  try {
    const nss = await callGo('ListNamespaces')
    if (nss) namespaces.value = nss
  } catch {}
  await fetchBridges()
})

async function fetchBridges() {
  loading.value = true
  error.value = null
  try {
    const ns = form.value.namespace
    const result = await callGo('ListExternalBridges', ns)
    bridges.value = result || []
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

function addPort() {
  form.value.ports.push({ name: '', port: 80, targetPort: 80, protocol: 'TCP' })
}

function removePort(i) {
  if (form.value.ports.length > 1) form.value.ports.splice(i, 1)
}

async function createBridge() {
  creating.value = true
  error.value = null
  try {
    const spec = {
      name: form.value.name,
      namespace: form.value.namespace,
      type: form.value.type,
      externalName: form.value.type === 'externalname' ? form.value.externalName : '',
      externalIPs: form.value.type === 'manual' ? form.value.externalIPs.split(',').map(s => s.trim()).filter(Boolean) : [],
      ports: form.value.ports.filter(p => p.port).map(p => ({
        name: p.name || 'svc',
        port: parseInt(p.port),
        targetPort: parseInt(p.targetPort || p.port),
        protocol: p.protocol || 'TCP',
      })),
    }
    await callGo('CreateExternalBridge', spec)
    showForm.value = false
    form.value.name = ''
    form.value.externalIPs = ''
    form.value.externalName = ''
    await fetchBridges()
  } catch (e) {
    error.value = e?.message || String(e)
  }
  creating.value = false
}

async function pingBridge(bridge) {
  try {
    const ok = await callGo('PingExternalEndpoint', bridge.namespace, bridge.name)
    bridge._lastPing = ok ? 'Reachable' : 'Unreachable'
    bridge._pingTime = new Date().toLocaleTimeString()
  } catch {
    bridge._lastPing = 'Error'
  }
}
</script>

<template>
  <div class="bridge-view">
    <div class="header">
      <div class="title">External Bridges</div>
      <div class="subtitle">Route cluster traffic to external resources via selector-less Services + manual Endpoints</div>
    </div>

    <div class="toolbar">
      <button class="btn-primary" @click="showForm = !showForm">
        {{ showForm ? 'Cancel' : '+ New Bridge' }}
      </button>
      <button class="btn-secondary" @click="fetchBridges">Refresh</button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div v-if="showForm" class="form-card">
      <div class="form-section">
        <div class="section-title">Create External Bridge</div>
        <div class="form-row">
          <label class="form-label">Name</label>
          <input v-model="form.name" class="form-input" placeholder="external-db" />
        </div>
        <div class="form-row">
          <label class="form-label">Namespace</label>
          <Select v-model="form.namespace" :options="namespaces" size="sm" />
        </div>
        <div class="form-row">
          <label class="form-label">Type</label>
          <Select v-model="form.type" :options="[{value:'manual',label:'Manual Endpoints (IP-based)'},{value:'externalname',label:'ExternalName (DNS alias)'}]" size="sm" />
        </div>
        <div v-if="form.type === 'manual'" class="form-row">
          <label class="form-label">External IPs</label>
          <input v-model="form.externalIPs" class="form-input" placeholder="10.0.0.1, 10.0.0.2" />
        </div>
        <div v-if="form.type === 'externalname'" class="form-row">
          <label class="form-label">External Name</label>
          <input v-model="form.externalName" class="form-input" placeholder="db.example.com" />
        </div>

        <div class="section-subtitle">Ports</div>
        <div v-for="(p, i) in form.ports" :key="i" class="port-row">
          <input v-model="p.name" class="form-input small" placeholder="name" />
          <input v-model.number="p.port" type="number" class="form-input tiny" placeholder="Port" />
          <input v-model.number="p.targetPort" type="number" class="form-input tiny" placeholder="Target" />
          <Select v-model="p.protocol" :options="['TCP','UDP']" size="sm" />
          <button class="btn-remove" @click="removePort(i)">×</button>
        </div>
        <button class="btn-add" @click="addPort">+ Add Port</button>

        <div class="form-actions">
          <button class="btn-primary" :disabled="!form.name || creating" @click="createBridge">
            {{ creating ? 'Creating…' : 'Create Bridge' }}
          </button>
        </div>
      </div>
    </div>

    <div v-if="loading" class="state-box">Loading bridges…</div>
    <div v-else-if="bridges.length === 0" class="state-box">No external bridges found</div>

    <div v-else class="bridge-list">
      <div v-for="b in bridges" :key="b.name + b.namespace" class="bridge-card">
        <div class="bridge-header">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color:#3794ff"><path d="M18 20V10M12 20V4M6 20v-6"/></svg>
          <span class="bridge-name">{{ b.name }}</span>
          <span class="bridge-ns font-mono">{{ b.namespace }}</span>
        </div>
        <div class="bridge-meta">
          <div class="meta-chip">
            <span class="chip-label">Type</span>
            <span class="chip-value">{{ b.service?.spec?.type || '—' }}</span>
          </div>
          <div class="meta-chip">
            <span class="chip-label">External Name</span>
            <span class="chip-value">{{ b.service?.spec?.externalName || '—' }}</span>
          </div>
          <div class="meta-chip" v-if="b._lastPing">
            <span class="chip-label">Last Ping</span>
            <span class="chip-value" :class="b._lastPing === 'Reachable' ? 'ok' : 'fail'">{{ b._lastPing }}</span>
            <span class="chip-time font-mono">{{ b._pingTime }}</span>
          </div>
        </div>
        <div class="bridge-actions">
          <button class="action-btn" @click="pingBridge(b)">Ping</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bridge-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }
.toolbar { display: flex; gap: 8px; }
.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.form-card { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 20px; }
.form-section { margin-bottom: 16px; }
.section-title { font-size: 14px; font-weight: 600; color: #e8eaec; margin-bottom: 12px; }
.section-subtitle { font-size: 13px; font-weight: 500; color: #8b8f96; margin: 12px 0 8px; }
.form-row { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.form-label { font-size: 12px; color: #8b8f96; min-width: 100px; flex-shrink: 0; }
.form-input { flex: 1; padding: 7px 10px; font-size: 12.5px; background: #141517; border: 1px solid rgba(255,255,255,0.1); border-radius: 5px; color: #e8eaec; outline: none; font-family: inherit; }
.form-input:focus { border-color: #a78bfa; }
.form-input.tiny { max-width: 80px; }
.form-input.small { max-width: 120px; }
.port-row { display: flex; gap: 6px; margin-bottom: 6px; align-items: center; }
.btn-remove { background: none; border: none; color: #f05454; cursor: pointer; font-size: 16px; padding: 2px 6px; }
.btn-add { background: none; border: 1px dashed rgba(255,255,255,0.15); color: #a78bfa; padding: 5px 12px; font-size: 12px; border-radius: 4px; cursor: pointer; margin-top: 4px; }
.form-actions { margin-top: 16px; }
.btn-primary { padding: 8px 20px; font-size: 13px; background: rgba(167,139,250,0.15); border: 1px solid rgba(167,139,250,0.3); color: #a78bfa; border-radius: 6px; cursor: pointer; }
.btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }
.btn-secondary { padding: 8px 20px; font-size: 13px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 6px; cursor: pointer; }

.state-box { padding: 24px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; font-size: 13px; color: #8b8f96; text-align: center; }
.bridge-list { display: flex; flex-direction: column; gap: 8px; }
.bridge-card { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 14px 16px; display: flex; align-items: center; gap: 12px; }
.bridge-header { display: flex; align-items: center; gap: 8px; min-width: 0; }
.bridge-name { font-size: 13px; font-weight: 500; color: #e8eaec; }
.bridge-ns { font-size: 11px; color: #6b7078; }
.bridge-meta { flex: 1; display: flex; gap: 8px; flex-wrap: wrap; }
.meta-chip { display: flex; align-items: center; gap: 4px; font-size: 11px; color: #8b8f96; }
.chip-label { color: #6b7078; }
.chip-value { color: #e8eaec; }
.chip-value.ok { color: #3ecf8e; }
.chip-value.fail { color: #f05454; }
.chip-time { color: #6b7078; }
.bridge-actions { flex-shrink: 0; }
.action-btn { padding: 4px 10px; font-size: 11px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 4px; cursor: pointer; }
.action-btn:hover { background: rgba(255,255,255,0.12); }
.font-mono { font-family: var(--mono); }
</style>
