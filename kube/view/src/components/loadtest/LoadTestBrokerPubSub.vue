<script setup>
import { computed } from 'vue'
import Select from '../common/Select.vue'

const props = defineProps({
  modelValue: { type: Object, required: true },
})
const emit = defineEmits(['update:modelValue'])

function patch(field, value) {
  emit('update:modelValue', { ...props.modelValue, [field]: value })
}

const authModeOptions = [
  { value: 'adc', label: 'Application Default Credentials (ADC)' },
  { value: 'workload_identity', label: 'Workload Identity (GKE)' },
  { value: 'service_account_json', label: 'Service Account JSON' },
]

const showSAJson = computed(() => props.modelValue.authMode === 'service_account_json')
</script>

<template>
  <fieldset class="broker-form" aria-label="Google Pub/Sub configuration">
    <legend class="sr-only">Google Pub/Sub configuration</legend>

    <div class="form-row">
      <label for="ps-project-id" class="form-label">
        Project ID
        <span class="required" aria-label="required">*</span>
      </label>
      <input
        id="ps-project-id"
        type="text"
        class="form-input"
        placeholder="my-gcp-project"
        :value="modelValue.projectId"
        aria-required="true"
        data-testid="broker-pubsub-project-id"
        @input="patch('projectId', $event.target.value)"
      />
    </div>

    <div class="form-row">
      <label class="form-label" id="ps-auth-label">
        Auth Mode
        <span class="required" aria-label="required">*</span>
      </label>
      <Select
        :modelValue="modelValue.authMode || 'adc'"
        :options="authModeOptions"
        placeholder="Select auth mode"
        aria-labelledby="ps-auth-label"
        testid="broker-pubsub-auth-mode"
        width="100%"
        @update:modelValue="patch('authMode', $event)"
      />
    </div>

    <div v-if="showSAJson" class="form-row">
      <label for="ps-sa-json" class="form-label">
        <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
        Service Account JSON
        <span class="required" aria-label="required">*</span>
      </label>
      <textarea
        id="ps-sa-json"
        class="form-textarea sensitive"
        placeholder='{ "type": "service_account", … }'
        rows="6"
        :value="modelValue.serviceAccountJson"
        aria-required="true"
        aria-label="Service account JSON key (sensitive)"
        data-testid="broker-pubsub-sa-json"
        @input="patch('serviceAccountJson', $event.target.value)"
      />
    </div>

    <div class="form-row">
      <label for="ps-endpoint" class="form-label">Endpoint (optional)</label>
      <input
        id="ps-endpoint"
        type="text"
        class="form-input"
        placeholder="localhost:8085 (emulator)"
        :value="modelValue.endpoint"
        data-testid="broker-pubsub-endpoint"
        @input="patch('endpoint', $event.target.value)"
      />
    </div>
  </fieldset>
</template>

<style scoped>
.broker-form { border: none; padding: 0; margin: 0; }
.form-row { display: flex; flex-direction: column; gap: 4px; margin-bottom: 12px; }
.form-label { font-size: 12px; color: var(--text2); display: flex; align-items: center; gap: 4px; }
.required { color: #ef4444; font-size: 11px; }
.sensitive-icon { font-size: 11px; }
.form-input, .form-textarea {
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
.form-input:focus, .form-textarea:focus {
  outline: 2px solid var(--accent, #4f8cff);
  outline-offset: 1px;
  border-color: var(--accent, #4f8cff);
}
.form-textarea { resize: vertical; font-family: monospace; }
.form-textarea.sensitive { font-family: monospace; letter-spacing: 0.02em; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
</style>
