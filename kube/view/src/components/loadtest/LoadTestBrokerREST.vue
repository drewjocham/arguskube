<script setup>
import { computed } from 'vue'
import Select from '../common/Select.vue'

// REST broker form. Mirrors backend pkg/broker/config.go RESTConfig
// keys. We hold headers as an array of {key,value} rows for editing
// ergonomics, but the canonical config object stores them the same way
// — Go's RESTConfig encodes them via a Headers map serialized as KV.

const props = defineProps({
  modelValue: { type: Object, required: true },
})
const emit = defineEmits(['update:modelValue'])

function patch(field, value) {
  emit('update:modelValue', { ...props.modelValue, [field]: value })
}

const methodOptions = [
  { value: 'POST', label: 'POST' },
  { value: 'PUT', label: 'PUT' },
  { value: 'PATCH', label: 'PATCH' },
  { value: 'GET', label: 'GET' },
  { value: 'DELETE', label: 'DELETE' },
]

const headers = computed(() => Array.isArray(props.modelValue.headers) ? props.modelValue.headers : [])
const successCodesStr = computed({
  get() {
    const arr = props.modelValue.successCodes
    if (Array.isArray(arr)) return arr.join(', ')
    return arr ?? ''
  },
  set(v) {
    // Comma-separated → array of ints. Empty strings are dropped.
    const parsed = String(v)
      .split(',')
      .map(s => s.trim())
      .filter(Boolean)
      .map(Number)
      .filter(n => Number.isFinite(n))
    patch('successCodes', parsed)
  },
})

function addHeader() {
  patch('headers', [...headers.value, { key: '', value: '' }])
}
function removeHeader(idx) {
  const next = headers.value.slice()
  next.splice(idx, 1)
  patch('headers', next)
}
function patchHeader(idx, field, value) {
  const next = headers.value.map((h, i) => i === idx ? { ...h, [field]: value } : h)
  patch('headers', next)
}
</script>

<template>
  <fieldset class="broker-form" aria-label="REST/HTTP broker configuration">
    <legend class="sr-only">REST/HTTP configuration</legend>

    <div class="form-row">
      <label for="rest-base-url" class="form-label">Base URL</label>
      <input
        id="rest-base-url"
        type="text"
        class="form-input"
        placeholder="https://api.example.com"
        :value="modelValue.baseURL"
        data-testid="broker-rest-base-url"
        @input="patch('baseURL', $event.target.value)"
      />
    </div>

    <div class="form-row">
      <label class="form-label" id="rest-method-label">Method</label>
      <Select
        :modelValue="modelValue.method || 'POST'"
        :options="methodOptions"
        aria-labelledby="rest-method-label"
        testid="broker-rest-method"
        width="100%"
        @update:modelValue="patch('method', $event)"
      />
    </div>

    <div class="form-row">
      <label for="rest-path" class="form-label">Path</label>
      <input id="rest-path" type="text" class="form-input" placeholder="/v1/events" :value="modelValue.path" data-testid="broker-rest-path" @input="patch('path', $event.target.value)" />
    </div>

    <div class="form-row">
      <label for="rest-content-type" class="form-label">Content-Type</label>
      <input id="rest-content-type" type="text" class="form-input" :value="modelValue.contentType || 'application/json'" data-testid="broker-rest-content-type" @input="patch('contentType', $event.target.value)" />
    </div>

    <div class="form-row">
      <label class="form-label">Headers</label>
      <div v-for="(h, idx) in headers" :key="idx" class="header-row">
        <input class="form-input" placeholder="Header name" :value="h.key" @input="patchHeader(idx, 'key', $event.target.value)" />
        <input class="form-input" placeholder="Value" :value="h.value" @input="patchHeader(idx, 'value', $event.target.value)" />
        <button type="button" class="btn-row btn-row--del" @click="removeHeader(idx)" aria-label="Remove header">×</button>
      </div>
      <button type="button" class="btn-row" data-testid="broker-rest-add-header" @click="addHeader">+ Add header</button>
    </div>

    <div class="form-row">
      <label for="rest-timeout" class="form-label">Timeout (seconds)</label>
      <input id="rest-timeout" type="number" min="1" class="form-input" :value="modelValue.timeoutSeconds ?? 30" data-testid="broker-rest-timeout" @input="patch('timeoutSeconds', Number($event.target.value))" />
    </div>

    <div class="form-row">
      <label for="rest-success" class="form-label">Success status codes (comma-separated)</label>
      <input id="rest-success" type="text" class="form-input" placeholder="200, 201, 202, 204" :value="successCodesStr" @input="successCodesStr = $event.target.value" />
    </div>

    <div class="form-row">
      <label for="rest-basic-user" class="form-label">Basic auth user</label>
      <input id="rest-basic-user" type="text" class="form-input" :value="modelValue.basicAuthUser" @input="patch('basicAuthUser', $event.target.value)" />
    </div>
    <div class="form-row">
      <label for="rest-basic-pw" class="form-label">
        <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
        Basic auth password
      </label>
      <input id="rest-basic-pw" type="password" class="form-input" :value="modelValue.basicAuthPassword" aria-label="Basic auth password (sensitive)" @input="patch('basicAuthPassword', $event.target.value)" />
    </div>
    <div class="form-row">
      <label for="rest-bearer" class="form-label">
        <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
        Bearer token
      </label>
      <input id="rest-bearer" type="password" class="form-input" :value="modelValue.bearerToken" aria-label="Bearer token (sensitive)" @input="patch('bearerToken', $event.target.value)" />
    </div>

    <div class="form-row form-row--inline">
      <label class="form-label toggle-label">
        <input
          type="checkbox"
          class="toggle-check"
          :checked="modelValue.insecureSkipTLS"
          aria-label="Skip TLS certificate verification"
          @change="patch('insecureSkipTLS', $event.target.checked)"
        />
        Skip TLS verification (insecure)
      </label>
    </div>
  </fieldset>
</template>

<style scoped>
.broker-form { border: none; padding: 0; margin: 0; }
.form-row { display: flex; flex-direction: column; gap: 4px; margin-bottom: 12px; }
.form-row--inline { flex-direction: row; align-items: center; }
.form-label { font-size: 12px; color: var(--text2); display: flex; align-items: center; gap: 4px; }
.sensitive-icon { font-size: 11px; }
.toggle-label { cursor: pointer; user-select: none; }
.toggle-check { accent-color: var(--accent, #4f8cff); width: 14px; height: 14px; cursor: pointer; }
.form-input {
  background: var(--bg3);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text);
  font-size: 13px;
  padding: 6px 10px;
  font-family: inherit;
  width: 100%;
  box-sizing: border-box;
}
.form-input:focus { outline: 2px solid var(--accent, #4f8cff); outline-offset: 1px; border-color: var(--accent, #4f8cff); }
.header-row { display: grid; grid-template-columns: 1fr 1fr auto; gap: 6px; margin-bottom: 4px; }
.btn-row { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 4px 10px; font-size: 12px; cursor: pointer; color: var(--text2); }
.btn-row:hover { background: var(--bg4); color: var(--text); }
.btn-row--del { color: #ef4444; border-color: rgba(239,68,68,0.3); }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
</style>
