<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { callGo, cachedCallGo } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const namespaces = ref([])
const namespace = ref('default')
const pods = ref([])
const selectedPod = ref('')
const targetHost = ref('')
const targetPort = ref(80)

const loading = ref(false)
const error = ref(null)

interface NetworkPolicy {
  name: string
  direction: string
  podSelector: string
}

interface PodNetworkInfo {
  name: string
  namespace: string
  podIP: string
  hostIP: string
  node: string
  hostNetwork: boolean
  containers: string[]
  cniAnnotation: string
  networkPolicies: NetworkPolicy[]
}

interface DNSResult {
  hostname: string
  resolved: boolean
  addresses: string
  method: string
  error: string
}

interface ConnectivityResult {
  target: string
  port: number
  reachable: boolean
  durationMs: number
  error: string
}

interface CNIDaemonSet {
  name: string
  namespace: string
  desired: number
  ready: number
}

interface CNIStatus {
  plugin: string
  healthy: boolean
  daemonSets: CNIDaemonSet[]
  error: string
}

interface DiagSummary {
  networkInfo: PodNetworkInfo
  dnsResult: DNSResult
  connectivity: ConnectivityResult
  cniStatus: CNIStatus
}

const networkInfo = ref<PodNetworkInfo | null>(null)
const dnsResult = ref<DNSResult | null>(null)
const connectivityResult = ref<ConnectivityResult | null>(null)
const cniStatus = ref<CNIStatus | null>(null)
const runningCheck = ref<string | null>(null)

const prodSuggestion = computed(() => {
  if (!networkInfo.value) return null
  const p = networkInfo.value
  const suggestions: string[] = []
  if (p.hostNetwork) suggestions.push('Pod uses hostNetwork — bypasses CNI entirely')
  if (p.networkPolicies.length === 0) suggestions.push('No NetworkPolicies apply — traffic is wide open')
  if (p.podIP === '') suggestions.push('Pod has no IP assigned — CNI may be unhealthy')
  return suggestions.length ? suggestions : null
})

onMounted(async () => {
  // Surfacing the namespace fetch error instead of swallowing it —
  // the old `catch {}` left an empty namespace list with no hint to
  // the user, which the QA review flagged as the reason the "Pod
  // Network Diag" tool looked broken on first open when ListNamespaces
  // failed (most often: no kubeconfig bound, or RBAC missing).
  try {
    const nss = await callGo('ListNamespaces')
    if (nss) namespaces.value = nss
  } catch (e) {
    error.value = `Failed to load namespaces: ${e?.message || e}`
  }
})

async function fetchPods() {
  if (!namespace.value) return
  try {
    const result = await cachedCallGo('ListResources', ['pods', namespace.value])
    pods.value = result?.items?.map(i => ({ value: i.name, label: `${i.name} (${i.status})` })) || []
  } catch {
    pods.value = []
  }
}

async function runDiagnostic(check: string) {
  if (!selectedPod.value) return
  runningCheck.value = check
  error.value = null

  try {
    switch (check) {
      case 'network-info': {
        const result = await callGo('GetPodNetworkInfo', namespace.value, selectedPod.value)
        networkInfo.value = result
        break
      }
      case 'dns': {
        const host = targetHost.value || 'kubernetes.default.svc.cluster.local'
        const result = await callGo('RunPodDNSCheck', namespace.value, selectedPod.value, host)
        dnsResult.value = result
        break
      }
      case 'connectivity': {
        if (!targetHost.value) {
          error.value = 'Enter a target hostname or IP for the connectivity check'
          break
        }
        const result = await callGo('RunPodConnectivityCheck', namespace.value, selectedPod.value, targetHost.value, targetPort.value)
        connectivityResult.value = result
        break
      }
      case 'cni': {
        const result = await callGo('GetCNIStatus')
        cniStatus.value = result
        break
      }
      case 'all': {
        const host = targetHost.value || 'kubernetes.default.svc.cluster.local'
        const result: DiagSummary = await callGo('RunPodNetworkDiagnostics', namespace.value, selectedPod.value, host, targetPort.value)
        if (result) {
          networkInfo.value = result.networkInfo
          dnsResult.value = result.dnsResult
          connectivityResult.value = result.connectivity
          cniStatus.value = result.cniStatus
        }
        break
      }
    }
  } catch (e) {
    error.value = e?.message || String(e)
  }

  runningCheck.value = null
}

function statusColor(ok: boolean): string {
  return ok ? '#3ecf8e' : '#f05454'
}

function statusIcon(ok: boolean): string {
  return ok ? '✓' : '✗'
}
</script>

<template>
  <div class="pod-diag-view">
    <div class="header">
      <div class="title">Pod Network Diagnostics</div>
      <div class="subtitle">Run network diagnostics from any pod — DNS, connectivity, CNI health, and policy analysis</div>
    </div>

    <div class="controls-row">
      <div class="control-group">
        <label class="ctrl-label">Namespace</label>
        <Select v-model="namespace" :options="namespaces" @change="fetchPods" size="sm" aria-label="Namespace" />
      </div>
      <div class="control-group">
        <label class="ctrl-label">Pod</label>
        <Select v-model="selectedPod" :options="pods" placeholder="Select a pod…" size="sm" aria-label="Pod" />
        <button class="btn-sm" @click="fetchPods" title="Refresh pod list">↻</button>
      </div>
      <div class="control-group">
        <label class="ctrl-label">Target (for DNS/connectivity)</label>
        <input v-model="targetHost" type="text" class="input-host" placeholder="hostname or IP" />
        <span class="port-label">:</span>
        <input v-model.number="targetPort" type="number" class="input-port" placeholder="port" min="1" max="65535" />
      </div>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div class="action-bar">
      <button class="btn-diag" :disabled="!selectedPod || runningCheck !== null" @click="runDiagnostic('all')">
        {{ runningCheck === 'all' ? 'Running…' : 'Run All Checks' }}
      </button>
      <button class="btn-diag btn-secondary" :disabled="!selectedPod || runningCheck !== null" @click="runDiagnostic('network-info')">
        {{ runningCheck === 'network-info' ? 'Running…' : 'Network Info' }}
      </button>
      <button class="btn-diag btn-secondary" :disabled="!selectedPod || runningCheck !== null" @click="runDiagnostic('dns')">
        {{ runningCheck === 'dns' ? 'Running…' : 'DNS Check' }}
      </button>
      <button class="btn-diag btn-secondary" :disabled="!selectedPod || runningCheck !== null" @click="runDiagnostic('connectivity')">
        {{ runningCheck === 'connectivity' ? 'Running…' : 'Connectivity' }}
      </button>
      <button class="btn-diag btn-secondary" :disabled="runningCheck !== null" @click="runDiagnostic('cni')">
        {{ runningCheck === 'cni' ? 'Running…' : 'CNI Status' }}
      </button>
    </div>

    <!-- Prod Suggestion -->
    <div v-if="prodSuggestion" class="suggestion-card">
      <div class="suggestion-title">Observations</div>
      <div v-for="(s, i) in prodSuggestion" :key="i" class="suggestion-item">▸ {{ s }}</div>
    </div>

    <!-- Network Info -->
    <div v-if="networkInfo" class="result-section">
      <div class="section-title">Pod Network Info</div>
      <div class="info-grid">
        <div class="info-item"><span class="info-key">Pod IP</span><span class="info-val font-mono">{{ networkInfo.podIP || '—' }}</span></div>
        <div class="info-item"><span class="info-key">Host IP</span><span class="info-val font-mono">{{ networkInfo.hostIP || '—' }}</span></div>
        <div class="info-item"><span class="info-key">Node</span><span class="info-val font-mono">{{ networkInfo.node || '—' }}</span></div>
        <div class="info-item"><span class="info-key">Host Network</span><span class="info-val">{{ networkInfo.hostNetwork ? 'Yes' : 'No' }}</span></div>
        <div class="info-item"><span class="info-key">Containers</span><span class="info-val">{{ networkInfo.containers?.join(', ') || '—' }}</span></div>
        <div class="info-item" v-if="networkInfo.cniAnnotation"><span class="info-key">CNI Annotation</span><span class="info-val font-mono">{{ networkInfo.cniAnnotation }}</span></div>
      </div>
      <div v-if="networkInfo.networkPolicies?.length" class="policy-section">
        <div class="inline-title">Applied NetworkPolicies</div>
        <div class="policy-table">
          <div class="policy-header">
            <span class="pol-name">Name</span>
            <span class="pol-dir">Direction</span>
            <span class="pol-sel">Pod Selector</span>
          </div>
          <div v-for="p in networkInfo.networkPolicies" :key="p.name + p.direction" class="policy-row">
            <span class="pol-name font-mono">{{ p.name }}</span>
            <span class="pol-dir"><span class="dir-badge" :class="p.direction">{{ p.direction }}</span></span>
            <span class="pol-sel">{{ p.podSelector }}</span>
          </div>
        </div>
      </div>
      <div v-else-if="networkInfo.networkPolicies" class="no-policies">
        No NetworkPolicies match this pod — all traffic is allowed
      </div>
    </div>

    <!-- DNS Result -->
    <div v-if="dnsResult" class="result-section">
      <div class="section-title">
        DNS Resolution
        <span class="status-indicator" :style="{ color: statusColor(dnsResult.resolved) }">{{ statusIcon(dnsResult.resolved) }}</span>
      </div>
      <div class="info-grid">
        <div class="info-item"><span class="info-key">Hostname</span><span class="info-val font-mono">{{ dnsResult.hostname }}</span></div>
        <div class="info-item"><span class="info-key">Status</span><span class="info-val" :style="{ color: statusColor(dnsResult.resolved) }">{{ dnsResult.resolved ? 'Resolved' : 'Failed' }}</span></div>
        <div class="info-item"><span class="info-key">Method</span><span class="info-val">{{ dnsResult.method }}</span></div>
        <div class="info-item" v-if="dnsResult.addresses"><span class="info-key">Addresses</span><span class="info-val font-mono" style="white-space:pre-line">{{ dnsResult.addresses }}</span></div>
      </div>
      <div v-if="dnsResult.error" class="diag-error">Error: {{ dnsResult.error }}</div>
    </div>

    <!-- Connectivity Result -->
    <div v-if="connectivityResult" class="result-section">
      <div class="section-title">
        Connectivity Check
        <span class="status-indicator" :style="{ color: statusColor(connectivityResult.reachable) }">{{ statusIcon(connectivityResult.reachable) }}</span>
      </div>
      <div class="info-grid">
        <div class="info-item"><span class="info-key">Target</span><span class="info-val font-mono">{{ connectivityResult.target }}:{{ connectivityResult.port }}</span></div>
        <div class="info-item"><span class="info-key">Status</span><span class="info-val" :style="{ color: statusColor(connectivityResult.reachable) }">{{ connectivityResult.reachable ? 'Reachable' : 'Unreachable' }}</span></div>
        <div class="info-item" v-if="connectivityResult.durationMs"><span class="info-key">Duration</span><span class="info-val">{{ connectivityResult.durationMs }}ms</span></div>
      </div>
      <div v-if="connectivityResult.error" class="diag-error">Error: {{ connectivityResult.error }}</div>
    </div>

    <!-- CNI Status -->
    <div v-if="cniStatus" class="result-section">
      <div class="section-title">
        CNI Status
        <span class="status-indicator" :style="{ color: statusColor(cniStatus.healthy) }">{{ statusIcon(cniStatus.healthy) }}</span>
      </div>
      <div class="info-grid">
        <div class="info-item"><span class="info-key">Plugin</span><span class="info-val">{{ cniStatus.plugin }}</span></div>
        <div class="info-item"><span class="info-key">Health</span><span class="info-val" :style="{ color: statusColor(cniStatus.healthy) }">{{ cniStatus.healthy ? 'Healthy' : 'Degraded' }}</span></div>
      </div>
      <div v-if="cniStatus.daemonSets?.length" class="ds-table">
        <div class="ds-header">
          <span class="ds-name">DaemonSet</span>
          <span class="ds-ns">Namespace</span>
          <span class="ds-ready">Ready</span>
          <span class="ds-desired">Desired</span>
        </div>
        <div v-for="ds in cniStatus.daemonSets" :key="ds.name + ds.namespace" class="ds-row">
          <span class="ds-name font-mono">{{ ds.name }}</span>
          <span class="ds-ns">{{ ds.namespace }}</span>
          <span class="ds-ready" :style="{ color: ds.ready === ds.desired && ds.desired > 0 ? '#3ecf8e' : '#f05454' }">{{ ds.ready }}</span>
          <span class="ds-desired">{{ ds.desired }}</span>
        </div>
      </div>
      <div v-if="cniStatus.error" class="diag-error">Error: {{ cniStatus.error }}</div>
    </div>
  </div>
</template>

<style scoped>
.pod-diag-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.controls-row { display: flex; gap: 16px; flex-wrap: wrap; align-items: flex-end; }
.control-group { display: flex; align-items: center; gap: 6px; }
.ctrl-label { font-size: 11px; color: #8b8f96; text-transform: uppercase; letter-spacing: 0.04em; white-space: nowrap; }
.btn-sm { padding: 4px 8px; font-size: 14px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 5px; cursor: pointer; line-height: 1; }
.btn-sm:hover { background: rgba(255,255,255,0.1); }

.input-host { padding: 4px 8px; font-size: 12px; background: #1e2023; border: 1px solid rgba(255,255,255,0.1); color: #e8eaec; border-radius: 5px; width: 160px; font-family: var(--mono); }
.input-port { padding: 4px 8px; font-size: 12px; background: #1e2023; border: 1px solid rgba(255,255,255,0.1); color: #e8eaec; border-radius: 5px; width: 64px; }
.port-label { color: #8b8f96; font-size: 13px; }

.action-bar { display: flex; gap: 8px; flex-wrap: wrap; }
.btn-diag { padding: 8px 16px; font-size: 13px; font-weight: 500; background: #3794ff; color: #fff; border: none; border-radius: 6px; cursor: pointer; transition: background 0.15s; }
.btn-diag:hover:not(:disabled) { background: #4fa3ff; }
.btn-diag:disabled { opacity: 0.45; cursor: not-allowed; }
.btn-secondary { background: rgba(255,255,255,0.07); color: #b0b4ba; border: 1px solid rgba(255,255,255,0.1); }
.btn-secondary:hover:not(:disabled) { background: rgba(255,255,255,0.12); }

.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.suggestion-card { padding: 14px; background: rgba(245,166,35,0.08); border: 1px solid rgba(245,166,35,0.2); border-radius: 8px; }
.suggestion-title { font-size: 13px; font-weight: 600; color: #f5a623; margin-bottom: 6px; }
.suggestion-item { font-size: 12px; color: #b0b4ba; line-height: 1.6; }

.result-section { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 20px; display: flex; flex-direction: column; gap: 12px; }
.section-title { font-size: 14px; font-weight: 600; color: #e8eaec; display: flex; align-items: center; gap: 8px; }
.status-indicator { font-size: 16px; font-weight: 700; }

.info-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(240px, 1fr)); gap: 8px; }
.info-item { display: flex; flex-direction: column; gap: 2px; padding: 8px 10px; background: rgba(255,255,255,0.03); border-radius: 5px; }
.info-key { font-size: 10.5px; color: #8b8f96; text-transform: uppercase; letter-spacing: 0.04em; }
.info-val { font-size: 13px; color: #e8eaec; word-break: break-all; }

.policy-section { display: flex; flex-direction: column; gap: 8px; }
.inline-title { font-size: 12px; font-weight: 600; color: #8b8f96; text-transform: uppercase; letter-spacing: 0.04em; }
.policy-table { background: rgba(255,255,255,0.02); border-radius: 5px; overflow: hidden; }
.policy-header, .policy-row { display: grid; grid-template-columns: 1.5fr 100px 1fr; gap: 8px; padding: 8px 10px; font-size: 12px; align-items: center; }
.policy-header { background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.06); color: #8b8f96; font-weight: 600; font-size: 10.5px; text-transform: uppercase; }
.policy-row { border-bottom: 1px solid rgba(255,255,255,0.03); color: #b0b4ba; }
.policy-row:last-child { border-bottom: none; }
.dir-badge { padding: 2px 6px; border-radius: 3px; font-size: 10px; font-weight: 600; text-transform: uppercase; }
.dir-badge.ingress { background: rgba(55,148,255,0.15); color: #3794ff; }
.dir-badge.egress { background: rgba(245,166,35,0.15); color: #f5a623; }
.dir-badge.ingress+egress, .dir-badge.both { background: rgba(160,96,255,0.15); color: #a060ff; }

.no-policies { padding: 12px; background: rgba(245,166,35,0.06); border: 1px solid rgba(245,166,35,0.15); border-radius: 5px; font-size: 12px; color: #f5a623; }

.diag-error { padding: 8px 10px; background: rgba(240,84,84,0.08); border: 1px solid rgba(240,84,84,0.15); border-radius: 5px; font-size: 12px; color: #f05454; }

.ds-table { background: rgba(255,255,255,0.02); border-radius: 5px; overflow: hidden; }
.ds-header, .ds-row { display: grid; grid-template-columns: 1.5fr 1fr 80px 80px; gap: 8px; padding: 8px 10px; font-size: 12px; align-items: center; }
.ds-header { background: rgba(255,255,255,0.03); border-bottom: 1px solid rgba(255,255,255,0.06); color: #8b8f96; font-weight: 600; font-size: 10.5px; text-transform: uppercase; }
.ds-row { border-bottom: 1px solid rgba(255,255,255,0.03); color: #b0b4ba; }
.ds-row:last-child { border-bottom: none; }

.font-mono { font-family: var(--mono); }
</style>
