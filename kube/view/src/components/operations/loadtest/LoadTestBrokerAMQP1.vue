<script setup>
import { computed } from 'vue'
import Select from '../../common/Select.vue'

const props = defineProps({
  modelValue: { type: Object, required: true },
})
const emit = defineEmits(['update:modelValue'])

function patch(field, value) {
  emit('update:modelValue', { ...props.modelValue, [field]: value })
}

const authModeOptions = [
  { value: 'none', label: 'SASL ANONYMOUS / none' },
  { value: 'plain', label: 'SASL PLAIN (username + password)' },
  { value: 'external', label: 'SASL EXTERNAL (mTLS identity)' },
  { value: 'bearer', label: 'Bearer token' },
]

const authMode = computed(() => props.modelValue.authMode || 'none')
const showUserPass = computed(() => authMode.value === 'plain')
const showBearer = computed(() => authMode.value === 'bearer')
</script>

<template>
  <fieldset class="broker-form" aria-label="AMQP 1.0 (Solace / Azure Service Bus / Artemis) configuration">
    <legend class="sr-only">AMQP 1.0 configuration</legend>

    <div class="form-row">
      <label for="amqp1-url" class="form-label">
        AMQP URL
        <span class="required" aria-label="required">*</span>
      </label>
      <input
        id="amqp1-url"
        type="text"
        class="form-input"
        placeholder="amqps://host:5671"
        :value="modelValue.url"
        aria-required="true"
        data-testid="broker-amqp1-url"
        @input="patch('url', $event.target.value)"
      />
    </div>

    <div class="form-row">
      <label for="amqp1-target" class="form-label">
        Sender Target
        <span class="required" aria-label="required">*</span>
      </label>
      <input
        id="amqp1-target"
        type="text"
        class="form-input"
        placeholder="queue/my-queue  or  topic/orders.created"
        :value="modelValue.senderTarget"
        aria-required="true"
        data-testid="broker-amqp1-sender-target"
        @input="patch('senderTarget', $event.target.value)"
      />
    </div>

    <div class="form-row">
      <label class="form-label" id="amqp1-auth-label">Auth Mode</label>
      <Select
        :modelValue="authMode"
        :options="authModeOptions"
        placeholder="Select auth mode"
        aria-labelledby="amqp1-auth-label"
        testid="broker-amqp1-auth-mode"
        width="100%"
        @update:modelValue="patch('authMode', $event)"
      />
    </div>

    <template v-if="showUserPass">
      <div class="form-row">
        <label for="amqp1-username" class="form-label">Username</label>
        <input id="amqp1-username" type="text" class="form-input" :value="modelValue.username" data-testid="broker-amqp1-username" @input="patch('username', $event.target.value)" />
      </div>
      <div class="form-row">
        <label for="amqp1-password" class="form-label">
          <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
          Password
        </label>
        <input id="amqp1-password" type="password" class="form-input" :value="modelValue.password" aria-label="AMQP 1.0 password (sensitive)" data-testid="broker-amqp1-password" @input="patch('password', $event.target.value)" />
      </div>
    </template>

    <div v-if="showBearer" class="form-row">
      <label for="amqp1-bearer" class="form-label">
        <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
        Bearer Token
      </label>
      <textarea id="amqp1-bearer" class="form-textarea sensitive" rows="4" :value="modelValue.bearerToken" aria-label="AMQP 1.0 bearer token (sensitive)" data-testid="broker-amqp1-bearer-token" @input="patch('bearerToken', $event.target.value)" />
    </div>

    <details class="tls-section">
      <summary class="tls-summary">Optional mTLS certificates</summary>
      <div class="tls-body">
        <div class="form-row">
          <label for="amqp1-ca-cert" class="form-label">
            <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
            CA Certificate (PEM)
          </label>
          <textarea id="amqp1-ca-cert" class="form-textarea sensitive" rows="5" placeholder="-----BEGIN CERTIFICATE-----" :value="modelValue.tlsCaCert" aria-label="CA certificate PEM block" data-testid="broker-amqp1-ca-cert" @input="patch('tlsCaCert', $event.target.value)" />
        </div>
        <div class="form-row">
          <label for="amqp1-client-cert" class="form-label">
            <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
            Client Certificate (PEM)
          </label>
          <textarea id="amqp1-client-cert" class="form-textarea sensitive" rows="5" placeholder="-----BEGIN CERTIFICATE-----" :value="modelValue.tlsClientCert" aria-label="Client certificate PEM block" data-testid="broker-amqp1-client-cert" @input="patch('tlsClientCert', $event.target.value)" />
        </div>
        <div class="form-row">
          <label for="amqp1-client-key" class="form-label">
            <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
            Client Key (PEM)
          </label>
          <textarea id="amqp1-client-key" class="form-textarea sensitive" rows="5" placeholder="-----BEGIN RSA PRIVATE KEY-----" :value="modelValue.tlsClientKey" aria-label="Client private key PEM block" data-testid="broker-amqp1-client-key" @input="patch('tlsClientKey', $event.target.value)" />
        </div>
        <div class="form-row form-row--inline">
          <label class="form-label toggle-label">
            <input
              type="checkbox"
              class="toggle-check"
              :checked="modelValue.insecureSkipVerify"
              aria-label="Skip TLS certificate verification"
              data-testid="broker-amqp1-insecure"
              @change="patch('insecureSkipVerify', $event.target.checked)"
            />
            Skip TLS verification (insecure)
          </label>
        </div>
      </div>
    </details>
  </fieldset>
</template>

<style scoped>
.broker-form { border: none; padding: 0; margin: 0; }
.form-row { display: flex; flex-direction: column; gap: 4px; margin-bottom: 12px; }
.form-row--inline { flex-direction: row; align-items: center; }
.form-label { font-size: 12px; color: var(--text2); display: flex; align-items: center; gap: 4px; }
.required { color: #ef4444; font-size: 11px; }
.sensitive-icon { font-size: 11px; }
.toggle-label { cursor: pointer; user-select: none; }
.toggle-check { accent-color: var(--accent, #4f8cff); width: 14px; height: 14px; cursor: pointer; }
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
.form-textarea.sensitive { font-family: monospace; }
.tls-section { margin-bottom: 12px; }
.tls-summary { font-size: 12px; color: var(--text2); cursor: pointer; user-select: none; padding: 4px 0; }
.tls-body { padding-top: 8px; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
</style>
