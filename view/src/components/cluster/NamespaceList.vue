<script setup>
import { ref, onMounted } from 'vue'
import { useResources } from '../../composables/useWails'

const { result, detail, loading, error, detailLoading, listResources, getResourceDetail } = useResources()

const namespaces = ref([])
const expandedNs = ref(null)

async function fetchNamespaces(force = false) {
  await listResources('namespaces', '', force)
  if (result.value && result.value.items && result.value.items.length > 0) {
    namespaces.value = result.value.items.map(item => ({
      name: item.name,
      status: item.status,
      labels: item.fields?.labels || '—',
      age: item.age || '—',
      statusColor: item.statusColor,
      pods: '—', cpuLimits: '—', memLimits: '—',
      ingress: [], egress: [], defaultDeny: false
    }))
  } else {
    namespaces.value = []
  }
}

const nsDetail = ref(null)

onMounted(fetchNamespaces)

async function toggleExpand(nsName) {
  if (expandedNs.value === nsName) {
    expandedNs.value = null
    nsDetail.value = null
  } else {
    expandedNs.value = nsName
    await getResourceDetail('namespaces', '', nsName)
    if (detail.value) {
      nsDetail.value = detail.value
    }
  }
}
</script>

<template>
  <div class="ns-view">
    <div class="header">
      <div class="header-text">
        <div class="title">Namespaces</div>
        <div class="subtitle">Logical partitions of your cluster resources</div>
      </div>
      <button class="refresh-btn" @click="fetchNamespaces(true)" :disabled="loading">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M23 4v6h-6"></path><path d="M1 20v-6h6"></path><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
        Refresh
      </button>
    </div>

    <div v-if="loading && !namespaces.length" class="state-box">Loading namespaces…</div>
    <div v-else-if="error" class="state-box state-error">{{ error }}</div>
    <div v-else-if="!namespaces.length" class="state-box">No namespaces found.</div>

    <div v-else class="ns-list">
      <div class="ns-header-row">
        <div class="ns-col">Name</div>
        <div class="ns-col">Status</div>
        <div class="ns-col">Labels</div>
        <div class="ns-col">Age</div>
      </div>

      <div v-for="ns in namespaces" :key="ns.name" class="ns-row-container">
        <div class="ns-row" :class="ns.status.toLowerCase()" @click="toggleExpand(ns.name)">
          <div class="ns-col ns-name">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #a78bfa; margin-right: 8px;">
              <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path>
            </svg>
            {{ ns.name }}
          </div>
          <div class="ns-col">
            <span class="status-badge" :class="ns.status.toLowerCase()">{{ ns.status }}</span>
          </div>
          <div class="ns-col font-mono">{{ ns.labels }}</div>
          <div class="ns-col font-mono" style="display:flex; justify-content:space-between; align-items:center;">
            {{ ns.age }}
            <svg class="chevron" :class="{ open: expandedNs === ns.name }" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="6 9 12 15 18 9"></polyline>
            </svg>
          </div>
        </div>

        <!-- Expanded View -->
        <div class="ns-expanded" v-if="expandedNs === ns.name">
          <div class="expanded-grid">
            
            <!-- Network Policy Graph -->
            <div class="expanded-card">
              <h4 class="card-title">Network Policy Graph</h4>
              <div class="network-graph-container">
                <div class="graph-box ingress">
                  <div class="box-title">Ingress (Inbound)</div>
                  <div class="box-items" v-if="ns.ingress.length">
                    <span class="net-item" v-for="i in ns.ingress" :key="i">{{ i }}</span>
                  </div>
                  <div class="box-items empty" v-else>
                    <span v-if="ns.defaultDeny">Default Deny All</span>
                    <span v-else>Allow All</span>
                  </div>
                </div>

                <div class="graph-arrow">
                  <svg width="40" height="24" viewBox="0 0 40 24" fill="none" stroke="currentColor" stroke-width="2" class="arrow-svg"><line x1="0" y1="12" x2="36" y2="12"></line><polyline points="30 6 36 12 30 18"></polyline></svg>
                </div>

                <div class="graph-box current-ns">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="color: #a78bfa; margin-bottom: 8px;"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path></svg>
                  <div class="box-title" style="color:#fff;">{{ ns.name }}</div>
                  <div class="policy-status" :class="{ 'deny': ns.defaultDeny }">{{ ns.defaultDeny ? 'Default Deny Policy' : 'No Default Policy' }}</div>
                </div>

                <div class="graph-arrow">
                  <svg width="40" height="24" viewBox="0 0 40 24" fill="none" stroke="currentColor" stroke-width="2" class="arrow-svg"><line x1="0" y1="12" x2="36" y2="12"></line><polyline points="30 6 36 12 30 18"></polyline></svg>
                </div>

                <div class="graph-box egress">
                  <div class="box-title">Egress (Outbound)</div>
                  <div class="box-items" v-if="ns.egress.length">
                    <span class="net-item" v-for="e in ns.egress" :key="e">{{ e }}</span>
                  </div>
                  <div class="box-items empty" v-else>
                    <span v-if="ns.defaultDeny">Default Deny All</span>
                    <span v-else>Allow All</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Namespace Detail & Resource Quotas -->
            <div class="expanded-card">
              <h4 class="card-title">Namespace Details</h4>
              <div v-if="nsDetail && nsDetail.properties" class="detail-grid">
                <div class="detail-item" v-for="prop in nsDetail.properties" :key="prop.key">
                  <div class="q-label">{{ prop.key }}</div>
                  <div class="q-val font-mono">{{ prop.value }}</div>
                </div>
              </div>
              <div v-else-if="detailLoading" class="detail-grid">
                <div class="detail-item"><div class="q-label">Loading details...</div></div>
              </div>
              <div v-else class="quota-grid">
                <div class="quota-item">
                  <div class="q-label">CPU Requests Limit</div>
                  <div class="q-val font-mono">{{ ns.cpuLimits === 'none' ? 'No Limit' : ns.cpuLimits + ' Cores' }}</div>
                </div>
                <div class="quota-item">
                  <div class="q-label">Memory Requests Limit</div>
                  <div class="q-val font-mono">{{ ns.memLimits === 'none' ? 'No Limit' : ns.memLimits }}</div>
                </div>
                <div class="quota-item">
                  <div class="q-label">Max Pods Allowed</div>
                  <div class="q-val font-mono">Unlimited</div>
                </div>
                <div class="quota-item">
                  <div class="q-label">Default Request/Limit Ratio</div>
                  <div class="q-val font-mono">Not Configured (LimitRange)</div>
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
.state-box { padding: 40px; text-align: center; color: #8b8f96; font-size: 13px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; }
.state-error { color: #f05454; }

.ns-view {
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
.refresh-btn {
  display: flex; align-items: center; gap: 6px;
  background: transparent; border: 1px solid rgba(255,255,255,0.1); color: #8b8f96;
  padding: 5px 12px; border-radius: 4px; font-size: 12px; cursor: pointer; transition: all 0.2s;
}
.refresh-btn:hover { color: #fff; border-color: rgba(255,255,255,0.2); }
.refresh-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.detail-grid {
  display: grid; grid-template-columns: 1fr 1fr; gap: 12px;
}
.detail-item { display: flex; flex-direction: column; gap: 4px; }

.ns-list {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  overflow: hidden;
}

.ns-header-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 11px;
  font-weight: 600;
  color: #8b8f96;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.ns-row-container {
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}
.ns-row-container:last-child { border-bottom: none; }

.ns-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr;
  padding: 14px 16px;
  font-size: 13px;
  color: #e8eaec;
  align-items: center;
  cursor: pointer;
  transition: background 0.2s;
}
.ns-row:hover { background: rgba(255, 255, 255, 0.03); }

.ns-row.terminating { opacity: 0.6; }

.ns-name { display: flex; align-items: center; font-weight: 500; }
.font-mono { font-family: 'SF Mono', Consolas, monospace; color: #b0b4ba; }

.chevron { transition: transform 0.2s ease; color: #6b7078; }
.chevron.open { transform: rotate(180deg); }

.status-badge {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
}
.status-badge.active { background: rgba(62, 207, 142, 0.15); color: #3ecf8e; }
.status-badge.terminating { background: rgba(245, 166, 35, 0.15); color: #f5a623; }

/* Expanded Area */
.ns-expanded {
  padding: 16px;
  background: #141517;
  border-top: 1px dashed rgba(255,255,255,0.08);
}
.expanded-grid {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.expanded-card {
  background: #1e2023;
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 6px;
  padding: 16px;
}
.card-title {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  margin: 0 0 16px 0;
}

/* Network Graph */
.network-graph-container {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 12px 0;
}
.graph-box {
  background: #0d0d0d;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 6px;
  padding: 16px;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  min-height: 120px;
}
.graph-box.current-ns {
  background: rgba(167, 139, 250, 0.05);
  border-color: rgba(167, 139, 250, 0.3);
  justify-content: center;
  transform: scale(1.05);
}
.box-title { font-size: 12px; font-weight: 600; color: #8b8f96; margin-bottom: 12px; }
.box-items { display: flex; flex-direction: column; gap: 8px; width: 100%; align-items: center; }
.net-item {
  background: rgba(255,255,255,0.05);
  color: #c8c9ca;
  font-size: 11px;
  padding: 4px 12px;
  border-radius: 4px;
  width: 100%;
  text-align: center;
}
.box-items.empty {
  color: #6b7078;
  font-size: 11px;
  font-style: italic;
  flex: 1;
  display: flex;
  align-items: center;
}
.policy-status {
  font-size: 11px;
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(62, 207, 142, 0.15);
  color: #3ecf8e;
  margin-top: 8px;
}
.policy-status.deny {
  background: rgba(240, 84, 84, 0.15);
  color: #f05454;
}

.graph-arrow { color: #4b5563; }
.arrow-svg { filter: drop-shadow(0 0 2px rgba(0,0,0,0.5)); }

/* Quotas */
.quota-grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr;
  gap: 16px;
}
.quota-item { display: flex; flex-direction: column; gap: 4px; }
.q-label { font-size: 11px; color: #8b8f96; }
.q-val { font-size: 13px; color: #e8eaec; }
</style>
