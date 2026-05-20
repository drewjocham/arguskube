<script setup>
import { ref } from 'vue'
import { callGo } from '../../composables/useBridge'
import YamlActionBar from './YamlActionBar.vue'

const ingressYAML = ref('')
const result = ref(null)
const loading = ref(false)
const error = ref(null)

async function translate() {
  if (!ingressYAML.value.trim()) return
  loading.value = true
  error.value = null
  result.value = null
  try {
    const res = await callGo('TranslateIngressToGateway', ingressYAML.value)
    result.value = res
  } catch (e) {
    error.value = e?.message || String(e)
  }
  loading.value = false
}

async function copyToClipboard(text) {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    document.execCommand('copy')
    document.body.removeChild(ta)
  }
}
</script>

<template>
  <div class="migration-view">
    <div class="header">
      <div class="title">Ingress Migration</div>
      <div class="subtitle">Translate Kubernetes Ingress resources to Gateway API (Gateway + HTTPRoute)</div>
    </div>

    <div class="input-section">
      <div class="section-label">Paste Ingress YAML</div>
      <textarea
        v-model="ingressYAML"
        class="yaml-input"
        rows="10"
        placeholder="apiVersion: networking.k8s.io/v1&#10;kind: Ingress&#10;metadata:&#10;  name: my-ingress&#10;  namespace: default&#10;spec:&#10;  rules:&#10;    - host: example.com&#10;      http:&#10;        paths:&#10;          - path: /&#10;            pathType: Prefix&#10;            backend:&#10;              service:&#10;                name: my-service&#10;                port:&#10;                  number: 80"
      ></textarea>
      <button
        class="btn-translate"
        :disabled="!ingressYAML.trim() || loading"
        @click="translate"
      >{{ loading ? 'Translating…' : 'Translate' }}</button>
    </div>

    <div v-if="error" class="error-banner">{{ error }}</div>

    <div v-if="result" class="output-section">
      <div class="output-columns">
        <div class="output-col">
          <div class="col-header">
            <span class="col-title">Gateway</span>
            <YamlActionBar :yaml="result.gatewayYAML" suggested-name="gateway" />
          </div>
          <pre class="yaml-output">{{ result.gatewayYAML }}</pre>
        </div>
        <div class="output-col">
          <div class="col-header">
            <span class="col-title">HTTPRoute</span>
            <YamlActionBar :yaml="result.httpRouteYAML" suggested-name="httproute" />
          </div>
          <pre class="yaml-output">{{ result.httpRouteYAML }}</pre>
        </div>
      </div>
      <div v-if="result.warnings?.length" class="warnings-section">
        <div class="warnings-title">Warnings</div>
        <div v-for="(w, i) in result.warnings" :key="i" class="warning-row">{{ w }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.migration-view { padding: 24px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1; min-height: 0; }
.header .title { font-size: 20px; font-weight: 500; color: #fff; margin-bottom: 4px; }
.header .subtitle { font-size: 13px; color: #8b8f96; }

.input-section { display: flex; flex-direction: column; gap: 8px; }
.section-label { font-size: 13px; font-weight: 600; color: #e8eaec; }
.yaml-input { padding: 12px; font-size: 12.5px; font-family: var(--mono); background: #141517; border: 1px solid rgba(255,255,255,0.1); border-radius: 8px; color: #e8eaec; outline: none; resize: vertical; line-height: 1.5; tab-size: 2; }
.yaml-input:focus { border-color: #a78bfa; }
.yaml-input::placeholder { color: #6b7078; }

.btn-translate { align-self: flex-start; padding: 8px 24px; font-size: 13px; background: rgba(167,139,250,0.15); border: 1px solid rgba(167,139,250,0.3); color: #a78bfa; border-radius: 6px; cursor: pointer; }
.btn-translate:hover { background: rgba(167,139,250,0.25); }
.btn-translate:disabled { opacity: 0.4; cursor: not-allowed; }

.error-banner { padding: 10px 14px; background: rgba(240,84,84,0.12); border: 1px solid rgba(240,84,84,0.25); border-radius: 6px; color: #f05454; font-size: 12px; }

.output-section { display: flex; flex-direction: column; gap: 12px; }
.output-columns { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.output-col { display: flex; flex-direction: column; gap: 8px; }
.col-header { display: flex; align-items: center; justify-content: space-between; }
.col-title { font-size: 13px; font-weight: 600; color: #e8eaec; }
.btn-copy { padding: 4px 12px; font-size: 11px; background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.1); color: #b0b4ba; border-radius: 4px; cursor: pointer; }
.btn-copy:hover { background: rgba(255,255,255,0.12); color: #e8eaec; }

.yaml-output { padding: 12px; font-size: 12px; font-family: var(--mono); background: #141517; border: 1px solid rgba(255,255,255,0.08); border-radius: 6px; color: #b0b4ba; overflow-x: auto; white-space: pre; line-height: 1.5; max-height: 400px; overflow-y: auto; }

.warnings-section { display: flex; flex-direction: column; gap: 4px; padding: 12px; background: rgba(245,166,35,0.06); border: 1px solid rgba(245,166,35,0.2); border-radius: 6px; }
.warnings-title { font-size: 12px; font-weight: 600; color: #f5a623; }
.warning-row { font-size: 12px; color: #8b8f96; }
</style>
