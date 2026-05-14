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
  { value: 'none', label: 'No authentication' },
  { value: 'plain', label: 'SASL PLAIN' },
  { value: 'scram_sha256', label: 'SASL SCRAM-SHA-256' },
  { value: 'scram_sha512', label: 'SASL SCRAM-SHA-512' },
  { value: 'oauthbearer', label: 'SASL OAUTHBEARER' },
  { value: 'mtls', label: 'Mutual TLS (mTLS)' },
]

const acksOptions = [
  { value: 'all', label: 'all — full ISR durability (default)' },
  { value: 'leader', label: 'leader — leader only' },
  { value: 'none', label: 'none — fire and forget' },
]

const authMode = computed(() => props.modelValue.authMode || 'none')
const showUserPass = computed(() => ['plain', 'scram_sha256', 'scram_sha512'].includes(authMode.value))
const showOAuth = computed(() => authMode.value === 'oauthbearer')
const showMTLS = computed(() => authMode.value === 'mtls')
</script>

<template>
  <fieldset class="broker-form" aria-label="Apache Kafka configuration">
    <legend class="sr-only">Apache Kafka configuration</legend>

    <div class="form-row">
      <label for="kafka-bootstrap" class="form-label">
        Bootstrap Servers
        <span class="required" aria-label="required">*</span>
      </label>
      <input
        id="kafka-bootstrap"
        type="text"
        class="form-input"
        placeholder="broker1:9092,broker2:9092"
        :value="modelValue.bootstrapServers"
        aria-required="true"
        data-testid="broker-kafka-bootstrap"
        @input="patch('bootstrapServers', $event.target.value)"
      />
    </div>

    <div class="form-row">
      <label for="kafka-client-id" class="form-label">Client ID</label>
      <input id="kafka-client-id" type="text" class="form-input" placeholder="argus-loadtest" :value="modelValue.clientId" data-testid="broker-kafka-client-id" @input="patch('clientId', $event.target.value)" />
    </div>

    <div class="form-row">
      <label class="form-label" id="kafka-auth-label">Auth Mode</label>
      <Select
        :modelValue="authMode"
        :options="authModeOptions"
        placeholder="Select auth mode"
        aria-labelledby="kafka-auth-label"
        testid="broker-kafka-auth-mode"
        width="100%"
        @update:modelValue="patch('authMode', $event)"
      />
    </div>

    <template v-if="showUserPass">
      <div class="form-row">
        <label for="kafka-username" class="form-label">Username</label>
        <input id="kafka-username" type="text" class="form-input" :value="modelValue.username" data-testid="broker-kafka-username" @input="patch('username', $event.target.value)" />
      </div>
      <div class="form-row">
        <label for="kafka-password" class="form-label">
          <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
          Password
        </label>
        <input id="kafka-password" type="password" class="form-input" :value="modelValue.password" aria-label="Kafka SASL password (sensitive)" data-testid="broker-kafka-password" @input="patch('password', $event.target.value)" />
      </div>
    </template>

    <div v-if="showOAuth" class="form-row">
      <label for="kafka-oauth-token" class="form-label">
        <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
        OAuth Bearer Token
      </label>
      <textarea id="kafka-oauth-token" class="form-textarea sensitive" rows="4" :value="modelValue.oauthBearerToken" aria-label="OAuth bearer token (sensitive)" data-testid="broker-kafka-oauth-token" @input="patch('oauthBearerToken', $event.target.value)" />
    </div>

    <template v-if="showMTLS">
      <div class="form-row">
        <label for="kafka-ca-cert" class="form-label">
          <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
          CA Certificate (PEM)
        </label>
        <textarea id="kafka-ca-cert" class="form-textarea sensitive" rows="5" placeholder="-----BEGIN CERTIFICATE-----" :value="modelValue.tlsCaCert" aria-label="CA certificate PEM block (sensitive)" data-testid="broker-kafka-ca-cert" @input="patch('tlsCaCert', $event.target.value)" />
      </div>
      <div class="form-row">
        <label for="kafka-client-cert" class="form-label">
          <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
          Client Certificate (PEM)
        </label>
        <textarea id="kafka-client-cert" class="form-textarea sensitive" rows="5" placeholder="-----BEGIN CERTIFICATE-----" :value="modelValue.tlsClientCert" aria-label="Client certificate PEM block (sensitive)" data-testid="broker-kafka-client-cert" @input="patch('tlsClientCert', $event.target.value)" />
      </div>
      <div class="form-row">
        <label for="kafka-client-key" class="form-label">
          <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
          Client Key (PEM)
        </label>
        <textarea id="kafka-client-key" class="form-textarea sensitive" rows="5" placeholder="-----BEGIN RSA PRIVATE KEY-----" :value="modelValue.tlsClientKey" aria-label="Client private key PEM block (sensitive)" data-testid="broker-kafka-client-key" @input="patch('tlsClientKey', $event.target.value)" />
      </div>
    </template>

    <div class="form-row">
      <label class="form-label" id="kafka-acks-label">Acks</label>
      <Select
        :modelValue="modelValue.acks || 'all'"
        :options="acksOptions"
        aria-labelledby="kafka-acks-label"
        testid="broker-kafka-acks"
        width="100%"
        @update:modelValue="patch('acks', $event)"
      />
    </div>

    <div class="form-row form-row--inline">
      <label class="form-label toggle-label">
        <input
          type="checkbox"
          class="toggle-check"
          :checked="modelValue.insecureSkipVerify"
          aria-label="Skip TLS certificate verification"
          data-testid="broker-kafka-insecure"
          @change="patch('insecureSkipVerify', $event.target.checked)"
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
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
</style>
