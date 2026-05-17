<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { callGo, cachedCallGo } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const namespaces = ref([])
const namespace = ref('default')
const pods = ref<Array<{ value: string; label: string }>>([])
const selectedPod = ref('')
const targetHost = ref('')
const targetPort = ref(80)

const loading = ref(false)
const error = ref<string | null>(null)

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
  try {
    const nss = await callGo('ListNamespaces')
    if (nss) namespaces.value = nss
  } catch (e) {
    const err = e as Error | null
    error.value = `Failed to load namespaces: ${err?.message || String(e)}`
  }
})

async function fetchPods() {
  if (!namespace.value) return
  try {
    const result = await cachedCallGo('ListResources', ['pods', namespace.value])
    pods.value = result?.items?.map((i: Record<string, unknown>) => ({ value: i.name as string, label: `${i.name} (${i.status})` })) || []
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
    const err = e as Error | null
    error.value = err?.message || String(e)
  }

  runningCheck.value = null
}

function statusColor(ok: boolean): string {
  return ok ? '#3ecf8e' : '#f05454'
}
</script>

<template>
  <div class="pod-network-diag">
    <h2>Pod Network Diagnostics</h2>

    <div class="controls">
      <Select v-model="namespace" :options="namespaces" placeholder="Namespace" aria-label="Namespace" @change="fetchPods" />

      <select v-model="selectedPod" aria-label="Pod">
        <option value="">Select a pod</option>
        <option v-for="(pod, idx) in pods" :key="idx" :value="pod.value">
          {{ pod.label }}
        </option>
      </select>

      <input v-model="targetHost" class="input-host" placeholder="Target host (default: kube-dns)" />

      <input v-model="targetPort" type="number" placeholder="Port" min="1" max="65535" />
    </div>

    <div class="actions">
      <button :disabled="!selectedPod || runningCheck === 'network-info'" @click="runDiagnostic('network-info')">
        Network Info
      </button>
      <button :disabled="!selectedPod || runningCheck === 'dns'" @click="runDiagnostic('dns')">
        DNS Check
      </button>
      <button :disabled="!selectedPod || runningCheck === 'connectivity'" @click="runDiagnostic('connectivity')">
        Connectivity
      </button>
      <button :disabled="runningCheck === 'cni'" @click="runDiagnostic('cni')">
        CNI Status
      </button>
      <button :disabled="!selectedPod || runningCheck === 'all'" @click="runDiagnostic('all')">
        Run All Checks
      </button>
    </div>

    <div v-if="error" class="error error-banner">{{ error }}</div>

    <div v-if="runningCheck" class="loading">
      Running {{ runningCheck }}...
    </div>

    <div v-if="prodSuggestion" class="suggestion">💡 {{ prodSuggestion }}</div>

    <div v-if="networkInfo" class="result-section">
      <h3>Pod Network Info</h3>
      <table>
        <tr><td>Pod IP</td><td>{{ networkInfo.podIP }}</td></tr>
        <tr><td>Host IP</td><td>{{ networkInfo.hostIP }}</td></tr>
        <tr><td>Node</td><td>{{ networkInfo.node }}</td></tr>
        <tr><td>Host Network</td><td>{{ networkInfo.hostNetwork }}</td></tr>
        <tr><td>Containers</td><td>{{ networkInfo.containers.join(', ') }}</td></tr>
        <tr><td>CNI Annotation</td><td>{{ networkInfo.cniAnnotation }}</td></tr>
      </table>
      <div v-if="networkInfo.networkPolicies.length" class="sub-section">
        <h4>Applied Network Policies ({{ networkInfo.networkPolicies.length }})</h4>
        <div v-for="np in networkInfo.networkPolicies" :key="np.name" class="policy-row">
          <strong>{{ np.name }}</strong> — {{ np.direction }} — {{ np.podSelector }}
        </div>
      </div>
    </div>

    <div v-if="dnsResult" class="result-section">
      <h3>DNS Resolution</h3>
      <table>
        <tr><td>Hostname</td><td>{{ dnsResult.hostname }}</td></tr>
        <tr><td>Resolved</td><td><span :style="{ color: statusColor(dnsResult.resolved) }">{{ dnsResult.resolved ? 'Yes' : 'No' }}</span></td></tr>
        <tr><td>Addresses</td><td>{{ dnsResult.addresses }}</td></tr>
        <tr><td>Method</td><td>{{ dnsResult.method }}</td></tr>
        <tr v-if="dnsResult.error"><td>Error</td><td class="error">{{ dnsResult.error }}</td></tr>
      </table>
    </div>

    <div v-if="connectivityResult" class="result-section">
      <h3>Connectivity Check</h3>
      <table>
        <tr><td>Target</td><td>{{ connectivityResult.target }}:{{ connectivityResult.port }}</td></tr>
        <tr><td>Reachable</td><td><span :style="{ color: statusColor(connectivityResult.reachable) }">{{ connectivityResult.reachable ? 'Yes' : 'No' }}</span></td></tr>
        <tr><td>Duration</td><td>{{ connectivityResult.durationMs }}ms</td></tr>
        <tr v-if="connectivityResult.error"><td>Error</td><td class="error">{{ connectivityResult.error }}</td></tr>
      </table>
    </div>

    <div v-if="cniStatus" class="result-section">
      <h3>CNI Status</h3>
      <table>
        <tr><td>Plugin</td><td>{{ cniStatus.plugin }}</td></tr>
        <tr><td>Healthy</td><td><span :style="{ color: statusColor(cniStatus.healthy) }">{{ cniStatus.healthy ? 'Yes' : 'No' }}</span></td></tr>
      </table>
      <div v-if="cniStatus.daemonSets.length" class="sub-section">
        <h4>DaemonSets</h4>
        <div v-for="ds in cniStatus.daemonSets" :key="ds.name" class="ds-row">
          {{ ds.name }}: {{ ds.ready }}/{{ ds.desired }} ready
        </div>
      </div>
      <div v-if="cniStatus.error" class="error">{{ cniStatus.error }}</div>
    </div>
  </div>
</template>

<style scoped>
.pod-network-diag { padding: 1rem; }
.controls { display: flex; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 1rem; }
.actions { display: flex; gap: 0.5rem; flex-wrap: wrap; margin-bottom: 1rem; }
.error { color: #f05454; margin: 0.5rem 0; }
.loading { color: #888; margin: 0.5rem 0; }
.suggestion { background: #fff3cd; border: 1px solid #ffc107; padding: 0.5rem; border-radius: 4px; margin: 0.5rem 0; }
.result-section { background: #1a1a2e; padding: 1rem; border-radius: 6px; margin: 0.5rem 0; }
.result-section h3 { margin: 0 0 0.5rem 0; }
.sub-section { margin-top: 1rem; }
.sub-section h4 { margin: 0 0 0.25rem 0; font-size: 0.9rem; color: #aaa; }
.policy-row, .ds-row { font-size: 0.9rem; margin: 0.25rem 0; }
table { width: 100%; border-collapse: collapse; }
td { padding: 0.25rem 0.5rem; border-bottom: 1px solid #333; }
td:first-child { font-weight: bold; width: 140px; color: #aaa; }
</style>
