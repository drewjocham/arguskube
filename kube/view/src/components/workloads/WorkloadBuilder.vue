<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { callGo } from '../../composables/useBridge'
import Select from '../common/Select.vue'

const tab = ref('form')

const name = ref('')
const namespace = ref('default')
const image = ref('')
const replicas = ref(1)
const generateSvc = ref(true)
const tShirtSize = ref('medium-api')

const ports = ref([{ name: 'http', port: 80, targetPort: 8080, protocol: 'TCP' }])
const envVars = ref([])

const livenessType = ref('none')
const livenessHTTPPath = ref('/healthz')
const livenessHTTPPort = ref(8080)
const livenessTCPPort = ref(8080)
const livenessCommand = ref('')
const livenessDelay = ref(10)
const livenessPeriod = ref(10)
const livenessTimeout = ref(5)
const livenessThreshold = ref(3)

const readinessType = ref('none')
const readinessHTTPPath = ref('/healthz')
const readinessHTTPPort = ref(8080)
const readinessTCPPort = ref(8080)
const readinessCommand = ref('')
const readinessDelay = ref(5)
const readinessPeriod = ref(5)
const readinessTimeout = ref(3)
const readinessThreshold = ref(2)

const startupType = ref('none')
const startupHTTPPath = ref('/healthz')
const startupHTTPPort = ref(8080)
const startupTCPPort = ref(8080)
const startupCommand = ref('')
const startupDelay = ref(0)
const startupPeriod = ref(10)
const startupTimeout = ref(1)
const startupThreshold = ref(30)

const namespaces = ref([])
const registryTags = ref([])
const tagsLoading = ref(false)
const tShirtProfiles = ref({})
const nodeCapacities = ref([])
const yamlOutput = ref(null)
const yamlLoading = ref(false)
const applyLoading = ref(false)
const result = ref(null)
const error = ref(null)
const customCPUReq = ref('')
const customMemReq = ref('')
const customCPULimit = ref('')
const customMemLimit = ref('')
const advancedResources = ref(false)

onMounted(async () => {
  try {
    const nss = await callGo('ListNamespaces')
    if (nss && nss.length) namespaces.value = nss
  } catch {}
  try {
    const sizes = await callGo('GetTShirtSizes')
    if (sizes) tShirtProfiles.value = sizes
  } catch {}
  try {
    const caps = await callGo('GetNodeCapacities')
    if (caps) nodeCapacities.value = caps
  } catch {}
})

const selectedProfile = computed(() => {
  return tShirtProfiles.value[tShirtSize.value] || {}
})

watch([tShirtSize, advancedResources], () => {
  if (!advancedResources.value && selectedProfile.value) {
    customCPUReq.value = selectedProfile.value.cpuRequest || ''
    customMemReq.value = selectedProfile.value.memoryRequest || ''
    customCPULimit.value = selectedProfile.value.cpuLimit || ''
    customMemLimit.value = selectedProfile.value.memoryLimit || ''
  }
}, { immediate: true })

let tagTimer = null
watch(image, (newVal) => {
  if (tagTimer) clearTimeout(tagTimer)
  registryTags.value = []
  if (!newVal || !newVal.includes('/')) return
  tagTimer = setTimeout(async () => {
    tagsLoading.value = true
    try {
      const tags = await callGo('ListRegistryTags', newVal.split(':')[0])
      if (tags) registryTags.value = tags
    } catch { registryTags.value = [] }
    tagsLoading.value = false
  }, 800)
})

function buildProbeSpec(type, path, httpPort, tcpPort, cmd, delay, period, timeout, threshold) {
  if (type === 'none') return null
  return {
    type,
    httpPath: path,
    httpPort: httpPort,
    tcpSocketPort: tcpPort,
    command: cmd,
    initialDelaySeconds: delay,
    periodSeconds: period,
    timeoutSeconds: timeout,
    failureThreshold: threshold,
  }
}

function addPort() {
  ports.value.push({ name: '', port: 80, targetPort: 8080, protocol: 'TCP' })
}

function removePort(i) {
  ports.value.splice(i, 1)
}

function addEnvVar() {
  envVars.value.push({ name: '', value: '' })
}

function removeEnvVar(i) {
  envVars.value.splice(i, 1)
}

async function generateYAML() {
  yamlLoading.value = true
  error.value = null
  try {
    const spec = {
      name: name.value || 'my-app',
      namespace: namespace.value || 'default',
      image: image.value || 'nginx:latest',
      replicas: parseInt(replicas.value) || 1,
      labels: { 'app.kubernetes.io/managed-by': 'argus-builder' },
      ports: ports.value.filter(p => p.port).map(p => ({
        name: p.name || `port-${p.port}`,
        port: parseInt(p.port),
        targetPort: parseInt(p.targetPort || p.port),
        protocol: p.protocol || 'TCP',
      })),
      envVars: envVars.value.filter(e => e.name).map(e => ({ name: e.name, value: e.value })),
      resources: {
        cpuRequest: customCPUReq.value || '',
        memoryRequest: customMemReq.value || '',
        cpuLimit: customCPULimit.value || '',
        memoryLimit: customMemLimit.value || '',
      },
      liveness: buildProbeSpec(livenessType.value, livenessHTTPPath.value, livenessHTTPPort.value, livenessTCPPort.value, livenessCommand.value, livenessDelay.value, livenessPeriod.value, livenessTimeout.value, livenessThreshold.value),
      readiness: buildProbeSpec(readinessType.value, readinessHTTPPath.value, readinessHTTPPort.value, readinessTCPPort.value, readinessCommand.value, readinessDelay.value, readinessPeriod.value, readinessTimeout.value, readinessThreshold.value),
      startup: buildProbeSpec(startupType.value, startupHTTPPath.value, startupHTTPPort.value, startupTCPPort.value, startupCommand.value, startupDelay.value, startupPeriod.value, startupTimeout.value, startupThreshold.value),
      generateSvc: generateSvc.value,
    }
    const result = await callGo('GenerateWorkloadYAML', spec)
    yamlOutput.value = result
  } catch (e) {
    error.value = e?.message || String(e)
  }
  yamlLoading.value = false
}

async function applyToCluster() {
  if (!yamlOutput.value) return
  applyLoading.value = true
  error.value = null
  result.value = null
  try {
    const parts = [yamlOutput.value.deployment]
    if (yamlOutput.value.service) parts.push(yamlOutput.value.service)
    const out = await callGo('ApplyYaml', parts.join('\n---\n'))
    result.value = `Applied: ${out}`
  } catch (e) {
    error.value = e?.message || String(e)
  }
  applyLoading.value = false
}

function copyYAML(text) {
  navigator.clipboard.writeText(text).catch(() => {})
}

function setProbeTemplate(type, kind) {
  const templates = {
    'http-health': { type: 'http', httpPath: '/healthz', httpPort: 8080, delay: 10, period: 10, timeout: 5, threshold: 3 },
    'http-ready': { type: 'http', httpPath: '/ready', httpPort: 8080, delay: 5, period: 5, timeout: 3, threshold: 2 },
    'tcp': { type: 'tcp', tcpPort: 8080, delay: 5, period: 10, timeout: 3, threshold: 3 },
    'command': { type: 'command', command: 'pg_isready -d postgres', delay: 5, period: 10, timeout: 5, threshold: 3 },
  }
  const t = templates[kind]
  if (!t) return
  if (type === 'liveness') {
    livenessType.value = t.type
    livenessHTTPPath.value = t.httpPath || ''
    livenessHTTPPort.value = t.httpPort || 8080
    livenessTCPPort.value = t.tcpPort || 8080
    livenessCommand.value = t.command || ''
    livenessDelay.value = t.delay
    livenessPeriod.value = t.period
    livenessTimeout.value = t.timeout
    livenessThreshold.value = t.threshold
  } else if (type === 'readiness') {
    readinessType.value = t.type
    readinessHTTPPath.value = t.httpPath || ''
    readinessHTTPPort.value = t.httpPort || 8080
    readinessTCPPort.value = t.tcpPort || 8080
    readinessCommand.value = t.command || ''
    readinessDelay.value = t.delay
    readinessPeriod.value = t.period
    readinessTimeout.value = t.timeout
    readinessThreshold.value = t.threshold
  } else if (type === 'startup') {
    startupType.value = t.type
    startupHTTPPath.value = t.httpPath || ''
    startupHTTPPort.value = t.httpPort || 8080
    startupTCPPort.value = t.tcpPort || 8080
    startupCommand.value = t.command || ''
    startupDelay.value = t.delay
    startupPeriod.value = t.period
    startupTimeout.value = t.timeout
    startupThreshold.value = t.threshold
  }
}
</script>

<template>
  <div class="builder-view">
    <div class="builder-layout">
      <div class="form-panel">
        <div class="header">
          <div class="title">Workload Builder</div>
          <div class="subtitle">Generate Kubernetes manifests without writing YAML</div>
        </div>

        <div v-if="error" class="error-banner">{{ error }}</div>
        <div v-if="result" class="success-banner">{{ result }}</div>

        <div class="form-section">
          <div class="section-title">Basic Configuration</div>
          <div class="form-row">
            <label class="form-label">Name</label>
            <input v-model="name" class="form-input" placeholder="my-service" />
          </div>
          <div class="form-row">
            <label class="form-label">Namespace</label>
            <Select v-model="namespace" :options="namespaces" size="sm" aria-label="Namespace" />
          </div>
          <div class="form-row">
            <label class="form-label">Image</label>
            <div class="image-input-wrap">
              <input v-model="image" class="form-input" placeholder="nginx:latest" />
              <span v-if="tagsLoading" class="tag-spinner"></span>
            </div>
            <div v-if="registryTags.length" class="tag-dropdown">
              <div class="tag-header">Latest tags:</div>
              <div
                v-for="t in registryTags.slice(0, 15)"
                :key="t.tag"
                class="tag-item"
                @click="image = (image.split(':')[0] || image) + ':' + t.tag"
              >{{ t.tag }}</div>
            </div>
          </div>
          <div class="form-row">
            <label class="form-label">Replicas</label>
            <input v-model.number="replicas" type="number" min="1" max="100" class="form-input short" />
          </div>
          <div class="form-row">
            <label class="form-label">
              <input type="checkbox" v-model="generateSvc" class="checkbox" />
              Generate Service
            </label>
          </div>
        </div>

        <div class="form-section">
          <div class="section-title">Ports</div>
          <div v-for="(p, i) in ports" :key="i" class="port-row">
            <input v-model="p.name" class="form-input small" placeholder="http" />
            <input v-model.number="p.port" type="number" class="form-input tiny" placeholder="80" />
            <input v-model.number="p.targetPort" type="number" class="form-input tiny" placeholder="8080" />
            <Select v-model="p.protocol" :options="['TCP','UDP']" size="sm" />
            <button class="btn-remove" @click="removePort(i)" :disabled="ports.length <= 1">×</button>
          </div>
          <button class="btn-add" @click="addPort">+ Add Port</button>
        </div>

        <div class="form-section">
          <div class="section-title">Environment Variables</div>
          <div v-for="(e, i) in envVars" :key="i" class="env-row">
            <input v-model="e.name" class="form-input" placeholder="KEY" />
            <input v-model="e.value" class="form-input" placeholder="value" />
            <button class="btn-remove" @click="removeEnvVar(i)">×</button>
          </div>
          <button class="btn-add" @click="addEnvVar">+ Add Env Var</button>
        </div>

        <div class="form-section">
          <div class="section-title">Resources (Right-Sizer)</div>
          <div class="form-row">
            <label class="form-label">T-Shirt Size</label>
            <Select v-model="tShirtSize" :options="[{value:'small-web',label:'Small Web App (100m CPU / 128Mi)'},{value:'medium-api',label:'Medium API (250m CPU / 512Mi)'},{value:'heavy-java',label:'Heavy Java App (1 CPU / 2Gi)'},{value:'data-pipeline',label:'Data Pipeline (500m CPU / 1Gi)'}]" size="sm" />
          </div>
          <div class="form-row">
            <label class="form-label">
              <input type="checkbox" v-model="advancedResources" class="checkbox" />
              Custom resources
            </label>
          </div>
          <template v-if="advancedResources">
            <div class="form-row">
              <label class="form-label">CPU Request</label>
              <input v-model="customCPUReq" class="form-input short" placeholder="250m" />
            </div>
            <div class="form-row">
              <label class="form-label">Memory Request</label>
              <input v-model="customMemReq" class="form-input short" placeholder="512Mi" />
            </div>
            <div class="form-row">
              <label class="form-label">CPU Limit</label>
              <input v-model="customCPULimit" class="form-input short" placeholder="1" />
            </div>
            <div class="form-row">
              <label class="form-label">Memory Limit</label>
              <input v-model="customMemLimit" class="form-input short" placeholder="1Gi" />
            </div>
          </template>
          <div v-if="nodeCapacities.length" class="capacity-hint">
            Node capacity: {{ nodeCapacities.map(c => c.cpu + ' CPU / ' + c.memory).join(', ') }}
          </div>
        </div>

        <div class="form-section">
          <div class="section-title">Liveness Probe</div>
          <div class="probe-templates">
            <button class="template-btn" @click="setProbeTemplate('liveness', 'http-health')">HTTP /healthz</button>
            <button class="template-btn" @click="setProbeTemplate('liveness', 'tcp')">TCP Socket</button>
            <button class="template-btn" @click="setProbeTemplate('liveness', 'command')">Command</button>
            <button class="template-btn" @click="livenessType = 'none'">None</button>
          </div>
          <template v-if="livenessType !== 'none'">
            <div class="form-row">
              <label class="form-label">Type</label>
              <Select v-model="livenessType" :options="[{value:'http',label:'HTTP GET'},{value:'tcp',label:'TCP Socket'},{value:'command',label:'Command'}]" size="sm" />
            </div>
            <div v-if="livenessType === 'http'" class="form-row">
              <label class="form-label">Path</label>
              <input v-model="livenessHTTPPath" class="form-input" placeholder="/healthz" />
            </div>
            <div v-if="livenessType === 'http'" class="form-row">
              <label class="form-label">Port</label>
              <input v-model.number="livenessHTTPPort" type="number" class="form-input short" />
            </div>
            <div v-if="livenessType === 'tcp'" class="form-row">
              <label class="form-label">Port</label>
              <input v-model.number="livenessTCPPort" type="number" class="form-input short" />
            </div>
            <div v-if="livenessType === 'command'" class="form-row">
              <label class="form-label">Command</label>
              <input v-model="livenessCommand" class="form-input" placeholder="pg_isready" />
            </div>
            <div class="probe-details">
              <div class="form-row slim">
                <label class="form-label">Delay</label>
                <input v-model.number="livenessDelay" type="number" class="form-input tiny" />
              </div>
              <div class="form-row slim">
                <label class="form-label">Period</label>
                <input v-model.number="livenessPeriod" type="number" class="form-input tiny" />
              </div>
              <div class="form-row slim">
                <label class="form-label">Timeout</label>
                <input v-model.number="livenessTimeout" type="number" class="form-input tiny" />
              </div>
              <div class="form-row slim">
                <label class="form-label">Threshold</label>
                <input v-model.number="livenessThreshold" type="number" class="form-input tiny" />
              </div>
            </div>
          </template>
        </div>

        <div class="form-section">
          <div class="section-title">Readiness Probe</div>
          <div class="probe-templates">
            <button class="template-btn" @click="setProbeTemplate('readiness', 'http-ready')">HTTP /ready</button>
            <button class="template-btn" @click="setProbeTemplate('readiness', 'tcp')">TCP Socket</button>
            <button class="template-btn" @click="setProbeTemplate('readiness', 'command')">Command</button>
            <button class="template-btn" @click="readinessType = 'none'">None</button>
          </div>
          <template v-if="readinessType !== 'none'">
            <div class="form-row">
              <label class="form-label">Type</label>
              <Select v-model="readinessType" :options="[{value:'http',label:'HTTP GET'},{value:'tcp',label:'TCP Socket'},{value:'command',label:'Command'}]" size="sm" />
            </div>
            <div v-if="readinessType === 'http'" class="form-row">
              <label class="form-label">Path</label>
              <input v-model="readinessHTTPPath" class="form-input" placeholder="/ready" />
            </div>
            <div v-if="readinessType === 'http'" class="form-row">
              <label class="form-label">Port</label>
              <input v-model.number="readinessHTTPPort" type="number" class="form-input short" />
            </div>
            <div v-if="readinessType === 'tcp'" class="form-row">
              <label class="form-label">Port</label>
              <input v-model.number="readinessTCPPort" type="number" class="form-input short" />
            </div>
            <div v-if="readinessType === 'command'" class="form-row">
              <label class="form-label">Command</label>
              <input v-model="readinessCommand" class="form-input" placeholder="pg_isready" />
            </div>
            <div class="probe-details">
              <div class="form-row slim">
                <label class="form-label">Delay</label>
                <input v-model.number="readinessDelay" type="number" class="form-input tiny" />
              </div>
              <div class="form-row slim">
                <label class="form-label">Period</label>
                <input v-model.number="readinessPeriod" type="number" class="form-input tiny" />
              </div>
              <div class="form-row slim">
                <label class="form-label">Timeout</label>
                <input v-model.number="readinessTimeout" type="number" class="form-input tiny" />
              </div>
              <div class="form-row slim">
                <label class="form-label">Threshold</label>
                <input v-model.number="readinessThreshold" type="number" class="form-input tiny" />
              </div>
            </div>
          </template>
        </div>

        <div class="form-section">
          <div class="section-title">Startup Probe</div>
          <div class="probe-templates">
            <button class="template-btn" @click="setProbeTemplate('startup', 'http-health')">HTTP /healthz</button>
            <button class="template-btn" @click="setProbeTemplate('startup', 'tcp')">TCP Socket</button>
            <button class="template-btn" @click="startupType = 'none'">None</button>
          </div>
          <template v-if="startupType !== 'none'">
            <div class="form-row">
              <label class="form-label">Type</label>
              <Select v-model="startupType" :options="[{value:'http',label:'HTTP GET'},{value:'tcp',label:'TCP Socket'},{value:'command',label:'Command'}]" size="sm" />
            </div>
            <div v-if="startupType === 'http'" class="form-row">
              <label class="form-label">Path</label>
              <input v-model="startupHTTPPath" class="form-input" placeholder="/healthz" />
            </div>
            <div v-if="startupType === 'http'" class="form-row">
              <label class="form-label">Port</label>
              <input v-model.number="startupHTTPPort" type="number" class="form-input short" />
            </div>
            <div v-if="startupType === 'tcp'" class="form-row">
              <label class="form-label">Port</label>
              <input v-model.number="startupTCPPort" type="number" class="form-input short" />
            </div>
            <div v-if="startupType === 'command'" class="form-row">
              <label class="form-label">Command</label>
              <input v-model="startupCommand" class="form-input" placeholder="pg_isready" />
            </div>
            <div class="probe-details">
              <div class="form-row slim">
                <label class="form-label">Delay</label>
                <input v-model.number="startupDelay" type="number" class="form-input tiny" />
              </div>
              <div class="form-row slim">
                <label class="form-label">Period</label>
                <input v-model.number="startupPeriod" type="number" class="form-input tiny" />
              </div>
              <div class="form-row slim">
                <label class="form-label">Timeout</label>
                <input v-model.number="startupTimeout" type="number" class="form-input tiny" />
              </div>
              <div class="form-row slim">
                <label class="form-label">Threshold</label>
                <input v-model.number="startupThreshold" type="number" class="form-input tiny" />
              </div>
            </div>
          </template>
        </div>

        <div class="form-actions">
          <button class="btn-primary" @click="generateYAML" :disabled="yamlLoading">
            {{ yamlLoading ? 'Generating…' : 'Generate YAML' }}
          </button>
          <button class="btn-secondary" @click="applyToCluster" :disabled="!yamlOutput || applyLoading">
            {{ applyLoading ? 'Applying…' : 'Apply to Cluster' }}
          </button>
        </div>
      </div>

      <div class="yaml-panel">
        <div class="yaml-header">
          <div class="yaml-title">Generated YAML</div>
          <div v-if="yamlOutput" class="yaml-actions">
            <button class="btn-copy" @click="copyYAML(yamlOutput.deployment)">Copy Deployment</button>
            <button v-if="yamlOutput.service" class="btn-copy" @click="copyYAML(yamlOutput.service)">Copy Service</button>
          </div>
        </div>
        <div class="yaml-scroll">
          <div v-if="yamlLoading" class="yaml-placeholder">Generating manifests…</div>
          <div v-else-if="!yamlOutput" class="yaml-placeholder">
            Configure your workload above and click <strong>Generate YAML</strong>
          </div>
          <pre v-else class="yaml-content font-mono">
---
# Deployment
{{ yamlOutput.deployment }}
<span v-if="yamlOutput.service">
---
# Service
{{ yamlOutput.service }}</span>
          </pre>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.builder-view { flex: 1; min-height: 0; display: flex; overflow: hidden; }
.builder-layout { flex: 1; display: flex; overflow: hidden; gap: 0; }
.form-panel { flex: 1; overflow-y: auto; padding: 24px; min-width: 0; }
.yaml-panel { width: 45%; min-width: 380px; border-left: 1px solid rgba(255,255,255,0.08); display: flex; flex-direction: column; background: #141517; }
.yaml-header { display: flex; align-items: center; justify-content: space-between; padding: 14px 16px; border-bottom: 1px solid rgba(255,255,255,0.08); flex-shrink: 0; }
.yaml-title { font-size: 13px; font-weight: 600; color: #e8eaec; }
.yaml-actions { display: flex; gap: 6px; }
.yaml-scroll { flex: 1; overflow-y: auto; padding: 16px; }
.yaml-content { font-size: 11.5px; line-height: 1.5; color: #b0b4ba; white-space: pre-wrap; margin: 0; tab-size: 2; }
.yaml-placeholder { display: flex; align-items: center; justify-content: center; height: 100%; color: #6b7078; font-size: 13px; text-align: center; padding: 40px; }

.header { margin-bottom: 20px; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.form-section { margin-bottom: 24px; padding-bottom: 20px; border-bottom: 1px solid rgba(255,255,255,0.06); }
.section-title { font-size: 14px; font-weight: 600; color: #e8eaec; margin-bottom: 12px; }
.form-row { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.form-row.slim { margin-bottom: 4px; }
.form-label { font-size: 12px; color: #8b8f96; min-width: 120px; flex-shrink: 0; }
.form-input { flex: 1; padding: 7px 10px; font-size: 12.5px; background: #1e2023; border: 1px solid rgba(255,255,255,0.1); border-radius: 5px; color: #e8eaec; outline: none; font-family: inherit; }
.form-input:focus { border-color: #a78bfa; }
.form-input.short { max-width: 160px; }
.form-input.tiny { max-width: 80px; }
.form-input.small { max-width: 120px; }
.checkbox { width: auto; margin-right: 6px; }

.image-input-wrap { flex: 1; display: flex; align-items: center; gap: 6px; }
.tag-spinner { width: 14px; height: 14px; border: 2px solid rgba(167,139,250,0.3); border-top-color: #a78bfa; border-radius: 50%; animation: spin 0.7s linear infinite; flex-shrink: 0; }
@keyframes spin { to { transform: rotate(360deg); } }

.tag-dropdown { margin-top: 6px; padding: 8px; background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 5px; max-height: 180px; overflow-y: auto; }
.tag-header { font-size: 10px; text-transform: uppercase; letter-spacing: 0.05em; color: #6b7078; margin-bottom: 4px; }
.tag-item { padding: 3px 6px; font-size: 12px; color: #a78bfa; cursor: pointer; border-radius: 3px; }
.tag-item:hover { background: rgba(167,139,250,0.12); }

.port-row, .env-row { display: flex; gap: 6px; margin-bottom: 6px; align-items: center; }
.probe-details { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 8px; }
.probe-templates { display: flex; gap: 4px; margin-bottom: 8px; flex-wrap: wrap; }
.template-btn { padding: 4px 10px; font-size: 11px; background: rgba(167,139,250,0.1); border: 1px solid rgba(167,139,250,0.2); color: #a78bfa; border-radius: 4px; cursor: pointer; }
.template-btn:hover { background: rgba(167,139,250,0.2); }

.btn-remove { background: none; border: none; color: #f05454; cursor: pointer; font-size: 16px; padding: 2px 6px; }
.btn-remove:disabled { color: #6b7078; }
.btn-add { background: none; border: 1px dashed rgba(255,255,255,0.15); color: #a78bfa; padding: 5px 12px; font-size: 12px; border-radius: 4px; cursor: pointer; margin-top: 4px; }
.btn-add:hover { background: rgba(167,139,250,0.08); }

.form-actions { display: flex; gap: 10px; margin-top: 20px; padding-bottom: 40px; }
.btn-primary { padding: 8px 20px; font-size: 13px; font-weight: 500; background: rgba(167,139,250,0.15); border: 1px solid rgba(167,139,250,0.3); color: #a78bfa; border-radius: 6px; cursor: pointer; }
.btn-primary:hover { background: rgba(167,139,250,0.25); }
.btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }
.btn-secondary { padding: 8px 20px; font-size: 13px; font-weight: 500; background: rgba(55,148,255,0.15); border: 1px solid rgba(55,148,255,0.3); color: #3794ff; border-radius: 6px; cursor: pointer; }
.btn-secondary:hover { background: rgba(55,148,255,0.25); }
.btn-secondary:disabled { opacity: 0.4; cursor: not-allowed; }
.btn-copy { padding: 4px 10px; font-size: 11px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 4px; cursor: pointer; }
.btn-copy:hover { background: rgba(255,255,255,0.12); }

.capacity-hint { font-size: 11px; color: #6b7078; margin-top: 6px; padding: 6px 10px; background: rgba(55,148,255,0.06); border-radius: 4px; border: 1px solid rgba(55,148,255,0.1); }

.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; margin-bottom: 16px; }
.success-banner { padding: 10px 14px; background: rgba(62,207,142,0.12); border: 1px solid rgba(62,207,142,0.25); border-radius: 6px; color: #3ecf8e; font-size: 12px; margin-bottom: 16px; }

.font-mono { font-family: var(--mono); }
</style>
