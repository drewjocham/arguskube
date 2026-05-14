<script setup>
import { computed, ref } from 'vue'
import Select from '../common/Select.vue'
import EndpointConfigModal from './EndpointConfigModal.vue'

// Multi-step REST scenario builder. v-model is the DistLoadScenario
// object the backend expects. We only emit visual cues; the canonical
// validation (Scenario.Validate()) runs on the backend and surfaces via
// the form's error banner.

// Mirror backend MaxScenarioEndpoints — keep both in sync if it changes.
const MAX_ENDPOINTS = 5

const props = defineProps({
  modelValue: { type: Object, required: true },
})
const emit = defineEmits(['update:modelValue'])

function patch(field, value) {
  emit('update:modelValue', { ...props.modelValue, [field]: value })
}

// ── Auth section ─────────────────────────────────────────────────────
const auth = computed(() => props.modelValue.auth || { mode: 'none' })
function setAuthMode(mode) {
  const next = { ...(props.modelValue.auth || {}), mode }
  if (mode === 'bearer' && !next.bearerMethod) next.bearerMethod = 'POST'
  if (mode === 'bearer' && !next.bearerTokenPath) next.bearerTokenPath = 'access_token'
  if (mode === 'apiKey' && !next.apiKeyHeader) next.apiKeyHeader = 'X-API-Key'
  patch('auth', next)
}
function patchAuth(field, value) {
  patch('auth', { ...(props.modelValue.auth || { mode: 'none' }), [field]: value })
}

// Bearer headers stored as object on the wire, but edited as rows here.
const bearerHeaderRows = computed(() => {
  const h = auth.value.bearerHeaders || {}
  return Object.entries(h).map(([k, v]) => ({ k, v }))
})
function setBearerHeaderRows(rows) {
  const obj = {}
  for (const r of rows) if (r.k) obj[r.k] = r.v
  patchAuth('bearerHeaders', obj)
}
function addBearerHeader() { setBearerHeaderRows([...bearerHeaderRows.value, { k: '', v: '' }]) }
function removeBearerHeader(i) {
  const next = bearerHeaderRows.value.slice(); next.splice(i, 1); setBearerHeaderRows(next)
}
function updateBearerHeader(i, field, val) {
  const next = bearerHeaderRows.value.map((r, idx) => idx === i ? { ...r, [field]: val } : r)
  setBearerHeaderRows(next)
}

const authCollapsed = ref(auth.value.mode === 'none' || !auth.value.mode)

// ── Endpoints list ────────────────────────────────────────────────────
const METHOD_OPTIONS = [
  { value: 'POST', label: 'POST' },
  { value: 'GET', label: 'GET' },
  { value: 'PUT', label: 'PUT' },
  { value: 'PATCH', label: 'PATCH' },
  { value: 'DELETE', label: 'DELETE' },
]

const endpoints = computed(() => props.modelValue.endpoints || [])

function setEndpoints(arr) { patch('endpoints', arr) }
function addEndpoint() {
  if (endpoints.value.length >= MAX_ENDPOINTS) return
  setEndpoints([...endpoints.value, blankEndpoint()])
}
function blankEndpoint() {
  return { name: '', method: 'POST', url: '', headers: {}, body: '', expect: null, chain: [] }
}
function removeEndpoint(i) {
  const next = endpoints.value.slice(); next.splice(i, 1); setEndpoints(next)
}
function updateEndpoint(i, ep) {
  const next = endpoints.value.map((e, idx) => idx === i ? ep : e); setEndpoints(next)
}

// Collapsed by default for steps after the first (per spec).
const collapsed = ref(endpoints.value.map((_, i) => i > 0))
function toggleCollapsed(i) {
  const next = collapsed.value.slice(); next[i] = !next[i]; collapsed.value = next
}

// Headers per endpoint — same object<->rows pattern as bearer headers.
function endpointHeaderRows(ep) {
  const h = ep.headers || {}
  return Object.entries(h).map(([k, v]) => ({ k, v }))
}
function setEndpointHeaders(i, rows) {
  const obj = {}
  for (const r of rows) if (r.k) obj[r.k] = r.v
  updateEndpoint(i, { ...endpoints.value[i], headers: obj })
}
function addEndpointHeader(i) {
  setEndpointHeaders(i, [...endpointHeaderRows(endpoints.value[i]), { k: '', v: '' }])
}
function removeEndpointHeader(i, idx) {
  const rows = endpointHeaderRows(endpoints.value[i]); rows.splice(idx, 1); setEndpointHeaders(i, rows)
}
function updateEndpointHeader(i, idx, field, val) {
  const rows = endpointHeaderRows(endpoints.value[i]).map((r, j) => j === idx ? { ...r, [field]: val } : r)
  setEndpointHeaders(i, rows)
}

function patchEndpoint(i, field, value) {
  updateEndpoint(i, { ...endpoints.value[i], [field]: value })
}

// Card summary text (collapsed view).
function epSummary(ep) {
  const chained = (ep.chain || []).length
  const assertions = ep.expect?.fieldChecks?.length || 0
  const matches = ep.expect?.bodyMatches ? 1 : 0
  return `${chained} chained, ${assertions + matches} assertion${assertions + matches === 1 ? '' : 's'}`
}

// ── Modal state ──────────────────────────────────────────────────────
const modalOpen = ref(false)
const modalIndex = ref(-1)
function openConfig(i) { modalIndex.value = i; modalOpen.value = true }
function onModalEndpoint(updated) {
  if (modalIndex.value < 0) return
  updateEndpoint(modalIndex.value, updated)
}

const atCap = computed(() => endpoints.value.length >= MAX_ENDPOINTS)
</script>

<template>
  <div class="scenario-builder" data-testid="scenario-builder">
    <!-- Auth section -->
    <div class="card">
      <div class="card-head">
        <button class="collapser" type="button" @click="authCollapsed = !authCollapsed">
          {{ authCollapsed ? '▶' : '▼' }} Authentication
          <span class="muted">— {{ auth.mode === 'bearer' ? 'Bearer (auth URL)' : auth.mode === 'apiKey' ? 'API key' : 'No auth' }}</span>
        </button>
      </div>
      <div v-if="!authCollapsed" class="card-body">
        <div class="seg" role="radiogroup" aria-label="Auth mode">
          <button type="button" class="seg-btn" :class="{ active: !auth.mode || auth.mode === 'none' }"
            data-testid="scenario-auth-none" @click="setAuthMode('none')">None</button>
          <button type="button" class="seg-btn" :class="{ active: auth.mode === 'bearer' }"
            data-testid="scenario-auth-bearer" @click="setAuthMode('bearer')">Bearer (auth URL)</button>
          <button type="button" class="seg-btn" :class="{ active: auth.mode === 'apiKey' }"
            data-testid="scenario-auth-apikey" @click="setAuthMode('apiKey')">API key</button>
        </div>

        <template v-if="auth.mode === 'bearer'">
          <div class="row">
            <label class="label">Auth URL</label>
            <input class="input" :value="auth.bearerAuthUrl || ''" placeholder="https://auth.example.com/oauth/token"
              data-testid="scenario-bearer-url" @input="patchAuth('bearerAuthUrl', $event.target.value)" />
          </div>
          <div class="row-2">
            <div>
              <label class="label">Method</label>
              <Select :modelValue="auth.bearerMethod || 'POST'" :options="METHOD_OPTIONS" width="100%"
                @update:modelValue="patchAuth('bearerMethod', $event)" />
            </div>
            <div>
              <label class="label">Token path (gjson)</label>
              <input class="input" :value="auth.bearerTokenPath || 'access_token'"
                @input="patchAuth('bearerTokenPath', $event.target.value)" />
            </div>
          </div>
          <div class="row">
            <label class="label">Body</label>
            <textarea class="input mono" rows="3" :value="auth.bearerBody || ''"
              @input="patchAuth('bearerBody', $event.target.value)"></textarea>
          </div>
          <div class="row">
            <label class="label">Headers</label>
            <div v-for="(h, i) in bearerHeaderRows" :key="i" class="kv-row">
              <input class="input" placeholder="Header name" :value="h.k" @input="updateBearerHeader(i, 'k', $event.target.value)" />
              <input class="input" placeholder="Value" :value="h.v" @input="updateBearerHeader(i, 'v', $event.target.value)" />
              <button type="button" class="btn-row btn-del" @click="removeBearerHeader(i)">×</button>
            </div>
            <button type="button" class="btn-row" @click="addBearerHeader">+ Add header</button>
          </div>
        </template>

        <template v-else-if="auth.mode === 'apiKey'">
          <div class="row-2">
            <div>
              <label class="label">Header name</label>
              <input class="input" :value="auth.apiKeyHeader || 'X-API-Key'" data-testid="scenario-apikey-header"
                @input="patchAuth('apiKeyHeader', $event.target.value)" />
            </div>
            <div>
              <label class="label">Header value</label>
              <input class="input" type="password" :value="auth.apiKeyValue || ''" data-testid="scenario-apikey-value"
                @input="patchAuth('apiKeyValue', $event.target.value)" />
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- Endpoints -->
    <div class="endpoints-list">
      <div v-for="(ep, i) in endpoints" :key="i" class="card endpoint-card" :class="{ invalid: !ep.url || !ep.method }">
        <div class="card-head">
          <button class="collapser" type="button" @click="toggleCollapsed(i)">
            {{ collapsed[i] ? '▶' : '▼' }}
            <span class="step-num">①{{ ['','②','③','④','⑤'][i] ? '' : '' }}</span>
            <span class="step-badge">{{ i + 1 }}</span>
            <span class="muted">{{ ep.method || 'POST' }}</span>
            <span class="ep-url">{{ ep.url || '(no url)' }}</span>
            <span class="muted">— {{ epSummary(ep) }}</span>
          </button>
          <button type="button" class="btn-row btn-del" data-testid="scenario-remove-endpoint"
            @click="removeEndpoint(i)" aria-label="Remove endpoint">×</button>
        </div>
        <div v-if="!collapsed[i]" class="card-body">
          <div class="row-2">
            <div>
              <label class="label">Name (optional)</label>
              <input class="input" :value="ep.name || ''" placeholder="login"
                @input="patchEndpoint(i, 'name', $event.target.value)" />
            </div>
            <div>
              <label class="label">Method</label>
              <Select :modelValue="ep.method || 'POST'" :options="METHOD_OPTIONS" width="100%"
                @update:modelValue="patchEndpoint(i, 'method', $event)" />
            </div>
          </div>
          <div class="row">
            <label class="label">URL</label>
            <input class="input" :class="{ invalid: !ep.url }" :value="ep.url || ''"
              placeholder="https://api.example.com/users" :data-testid="`scenario-ep-url-${i}`"
              @input="patchEndpoint(i, 'url', $event.target.value)" />
          </div>
          <div class="row">
            <label class="label">Headers</label>
            <div v-for="(h, idx) in endpointHeaderRows(ep)" :key="idx" class="kv-row">
              <input class="input" placeholder="Header" :value="h.k" @input="updateEndpointHeader(i, idx, 'k', $event.target.value)" />
              <input class="input" placeholder="Value" :value="h.v" @input="updateEndpointHeader(i, idx, 'v', $event.target.value)" />
              <button type="button" class="btn-row btn-del" @click="removeEndpointHeader(i, idx)">×</button>
            </div>
            <button type="button" class="btn-row" @click="addEndpointHeader(i)">+ Add header</button>
          </div>
          <div class="row">
            <label class="label">Body</label>
            <textarea class="input mono" rows="3" :value="ep.body || ''"
              @input="patchEndpoint(i, 'body', $event.target.value)"></textarea>
          </div>
          <div class="row">
            <button type="button" class="btn-config" :data-testid="`scenario-config-${i}`" @click="openConfig(i)">
              [Config] Chained calls &amp; expected response
              <span class="muted">— {{ epSummary(ep) }}</span>
            </button>
          </div>
        </div>
      </div>

      <button type="button" class="btn-add" :disabled="atCap" data-testid="scenario-add-endpoint"
        :title="atCap ? `Cap of ${MAX_ENDPOINTS} endpoints reached. Use chained calls for follow-ups.` : ''"
        @click="addEndpoint">
        + Add endpoint{{ atCap ? ` (max ${MAX_ENDPOINTS})` : '' }}
      </button>
    </div>

    <EndpointConfigModal
      v-if="modalIndex >= 0 && endpoints[modalIndex]"
      v-model:open="modalOpen"
      :endpoint="endpoints[modalIndex]"
      @update:endpoint="onModalEndpoint"
    />
  </div>
</template>

<style scoped>
.scenario-builder { display: flex; flex-direction: column; gap: 12px; }
.card { background: var(--bg2); border: 1px solid var(--border); border-radius: 8px; }
.card.invalid { border-color: rgba(208,88,88,0.45); }
.card-head { display: flex; align-items: center; justify-content: space-between; padding: 8px 10px; }
.collapser { background: none; border: none; color: var(--text); font: inherit; cursor: pointer; flex: 1; text-align: left; display: flex; align-items: center; gap: 6px; }
.card-body { padding: 10px 12px 12px; display: flex; flex-direction: column; gap: 10px; border-top: 1px solid var(--border); }
.row { display: flex; flex-direction: column; gap: 4px; }
.row-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.label { font-size: 11px; color: var(--text2); }
.input { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 6px 10px; font-size: 13px; color: var(--text); width: 100%; box-sizing: border-box; font-family: inherit; }
.input.mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
.input.invalid { border-color: rgba(208,88,88,0.6); }
.muted { color: var(--text3); font-size: 12px; }
.ep-url { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; color: var(--text2); }
.step-badge { display: inline-block; min-width: 18px; text-align: center; background: var(--bg3); border: 1px solid var(--border); border-radius: 4px; font-size: 11px; padding: 0 4px; color: var(--text2); }
.seg { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.seg-btn { background: var(--bg3); border: none; padding: 6px 14px; font-size: 12px; color: var(--text2); cursor: pointer; font-family: inherit; }
.seg-btn.active { background: var(--accent); color: #fff; }
.kv-row { display: grid; grid-template-columns: 1fr 1fr auto; gap: 6px; margin-bottom: 4px; }
.btn-row { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 4px 10px; font-size: 12px; cursor: pointer; color: var(--text2); }
.btn-row:hover { background: var(--bg4); color: var(--text); }
.btn-del { color: #ef4444; border-color: rgba(239,68,68,0.3); }
.btn-add { background: var(--bg3); border: 1px dashed var(--border); border-radius: 8px; padding: 8px 12px; color: var(--text2); cursor: pointer; font-size: 12px; }
.btn-add:disabled { opacity: 0.5; cursor: not-allowed; }
.btn-config { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 6px 10px; font-size: 12px; color: var(--accent2); cursor: pointer; text-align: left; }
.btn-config:hover { background: var(--bg4); }
</style>
