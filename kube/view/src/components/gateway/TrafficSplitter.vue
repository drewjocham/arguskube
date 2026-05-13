<script setup>
import { ref } from 'vue'
import { callGo } from '../../composables/useBridge'

const routeName = ref('')
const namespace = ref('default')
const gatewayName = ref('')
const gatewayNamespace = ref('default')
const backends = ref([{ name: '', port: 80, weight: 100 }])
const result = ref(null)
const loading = ref(false)
const error = ref(null)

function addBackend() {
  backends.value.push({ name: '', port: 80, weight: 0 })
}

function removeBackend(i) {
  if (backends.value.length > 1) backends.value.splice(i, 1)
  normalizeWeights()
}

function normalizeWeights() {
  const total = backends.value.reduce((s, b) => s + (Number(b.weight) || 0), 0)
  if (total === 0) return
  const factor = 100 / total
  backends.value.forEach(b => {
    b.weight = Math.round((Number(b.weight) || 0) * factor)
  })
  const sum = backends.value.reduce((s, b) => s + b.weight, 0)
  if (sum !== 100 && backends.value.length > 0) {
    backends.value[backends.value.length - 1].weight += (100 - sum)
  }
}

async function generate() {
  if (!routeName.value.trim() || !gatewayName.value.trim()) return
  if (!backends.value.some(b => b.name.trim())) return
  loading.value = true
  error.value = null
  result.value = null
  try {
    const payload = backends.value
      .filter(b => b.name.trim())
      .map(b => ({
        name: b.name.trim(),
        port: Number(b.port) || 80,
        weight: Number(b.weight) || 1,
      }))
    const yaml = await callGo('GenerateTrafficSplitHTTPRoute', routeName.value, namespace.value, gatewayName.value, gatewayNamespace.value, payload)
    result.value = yaml
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

async function copyYAML() {
  if (!result.value) return
  try {
    await navigator.clipboard.writeText(result.value)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = result.value
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
}
</script>

<template>
  <div class="splitter-view">
    <div class="header">
      <div class="title">Traffic Splitter</div>
      <div class="subtitle">Generate HTTPRoute manifests for weighted traffic splitting across backends</div>
    </div>

    <div class="form-card">
      <div class="form-row">
        <label class="form-label">Route Name</label>
        <input v-model="routeName" class="form-input" placeholder="canary-route" />
      </div>
      <div class="form-row">
        <label class="form-label">Namespace</label>
        <input v-model="namespace" class="form-input" placeholder="default" />
      </div>
      <div class="form-row">
        <label class="form-label">Gateway Name</label>
        <input v-model="gatewayName" class="form-input" placeholder="my-gateway" />
      </div>
      <div class="form-row">
        <label class="form-label">Gateway Namespace</label>
        <input v-model="gatewayNamespace" class="form-input" placeholder="default" />
      </div>

      <div class="backends-section">
        <div class="section-label">Backend Services</div>
        <div v-for="(b, i) in backends" :key="i" class="backend-row">
          <input v-model="b.name" class="form-input" placeholder="service-name" />
          <input v-model.number="b.port" type="number" class="form-input port-input" placeholder="Port" />
          <div class="weight-group">
            <input
              v-model.number="b.weight"
              type="range"
              min="0"
              max="100"
              class="weight-slider"
            />
            <span class="weight-value font-mono">{{ b.weight }}%</span>
          </div>
          <button class="btn-remove" @click="removeBackend(i)" :disabled="backends.length === 1">×</button>
        </div>
        <button class="btn-add" @click="addBackend">+ Add Backend</button>
      </div>

      <div class="form-actions">
        <button
          class="btn-generate"
          :disabled="!routeName.trim() || !gatewayName.trim() || !backends.some(b => b.name.trim()) || loading"
          @click="generate"
        >{{ loading ? 'Generating…' : 'Generate' }}</button>
      </div>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div v-if="result" class="result-section">
      <div class="result-header">
        <span class="result-title">Generated HTTPRoute YAML</span>
        <button class="btn-copy" @click="copyYAML">Copy</button>
      </div>
      <pre class="yaml-output">{{ result }}</pre>
    </div>
  </div>
</template>

<style scoped>
.splitter-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.form-card { background: #1e2023; border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; padding: 20px; display: flex; flex-direction: column; gap: 12px; }
.form-row { display: flex; align-items: center; gap: 10px; }
.form-label { font-size: 12px; color: #8b8f96; min-width: 130px; flex-shrink: 0; }
.form-input { flex: 1; padding: 7px 10px; font-size: 12.5px; background: #141517; border: 1px solid rgba(255,255,255,0.1); border-radius: 5px; color: #e8eaec; outline: none; font-family: inherit; }
.form-input:focus { border-color: #a78bfa; }
.form-input.port-input { max-width: 80px; }

.backends-section { display: flex; flex-direction: column; gap: 8px; padding-top: 8px; border-top: 1px solid rgba(255,255,255,0.06); }
.section-label { font-size: 13px; font-weight: 600; color: #e8eaec; }
.backend-row { display: flex; align-items: center; gap: 8px; }
.weight-group { display: flex; align-items: center; gap: 8px; flex: 1; }
.weight-slider { flex: 1; height: 4px; appearance: none; -webkit-appearance: none; background: rgba(255,255,255,0.1); border-radius: 2px; outline: none; }
.weight-slider::-webkit-slider-thumb { appearance: none; -webkit-appearance: none; width: 14px; height: 14px; border-radius: 50%; background: #a78bfa; cursor: pointer; border: none; }
.weight-slider::-moz-range-thumb { width: 14px; height: 14px; border-radius: 50%; background: #a78bfa; cursor: pointer; border: none; }
.weight-value { font-size: 12px; color: #a78bfa; min-width: 40px; text-align: right; }

.btn-remove { background: none; border: none; color: #f05454; cursor: pointer; font-size: 18px; padding: 2px 6px; line-height: 1; }
.btn-remove:disabled { opacity: 0.3; cursor: not-allowed; }
.btn-add { align-self: flex-start; background: none; border: 1px dashed rgba(255,255,255,0.15); color: #a78bfa; padding: 5px 12px; font-size: 12px; border-radius: 4px; cursor: pointer; }

.form-actions { padding-top: 4px; }
.btn-generate { padding: 8px 24px; font-size: 13px; background: rgba(167,139,250,0.15); border: 1px solid rgba(167,139,250,0.3); color: #a78bfa; border-radius: 6px; cursor: pointer; }
.btn-generate:hover { background: rgba(167,139,250,0.25); }
.btn-generate:disabled { opacity: 0.4; cursor: not-allowed; }

.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.result-section { display: flex; flex-direction: column; gap: 8px; }
.result-header { display: flex; align-items: center; justify-content: space-between; }
.result-title { font-size: 14px; font-weight: 600; color: #e8eaec; }
.btn-copy { padding: 4px 12px; font-size: 11px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 4px; cursor: pointer; }
.btn-copy:hover { background: rgba(255,255,255,0.12); color: #e8eaec; }

.yaml-output { padding: 12px; font-size: 12px; font-family: var(--mono); background: #141517; border: 1px solid rgba(255,255,255,0.08); border-radius: 6px; color: #b0b4ba; overflow-x: auto; white-space: pre; line-height: 1.5; max-height: 400px; overflow-y: auto; }
</style>
