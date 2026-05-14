<script setup>
import Select from '../../common/Select.vue'

const props = defineProps({
  modelValue: { type: Object, required: true },
})
const emit = defineEmits(['update:modelValue'])

function patch(field, value) {
  emit('update:modelValue', { ...props.modelValue, [field]: value })
}

const exchangeTypeOptions = [
  { value: 'direct', label: 'direct' },
  { value: 'topic', label: 'topic (default)' },
  { value: 'fanout', label: 'fanout' },
  { value: 'headers', label: 'headers' },
]
</script>

<template>
  <fieldset class="broker-form" aria-label="RabbitMQ AMQP 0.9 configuration">
    <legend class="sr-only">RabbitMQ AMQP 0.9 configuration</legend>

    <div class="form-row">
      <label for="rmq-url" class="form-label">
        AMQP URL
        <span class="required" aria-label="required">*</span>
      </label>
      <input
        id="rmq-url"
        type="text"
        class="form-input"
        placeholder="amqp://user:pass@host:5672/vhost"
        :value="modelValue.url"
        aria-required="true"
        data-testid="broker-rabbitmq-url"
        @input="patch('url', $event.target.value)"
      />
    </div>

    <div class="form-row">
      <label for="rmq-exchange" class="form-label">
        Exchange
        <span class="required" aria-label="required">*</span>
      </label>
      <input
        id="rmq-exchange"
        type="text"
        class="form-input"
        placeholder="my-exchange"
        :value="modelValue.exchange"
        aria-required="true"
        data-testid="broker-rabbitmq-exchange"
        @input="patch('exchange', $event.target.value)"
      />
    </div>

    <div class="form-row">
      <label class="form-label" id="rmq-exchange-type-label">Exchange Type</label>
      <Select
        :modelValue="modelValue.exchangeType || 'topic'"
        :options="exchangeTypeOptions"
        aria-labelledby="rmq-exchange-type-label"
        testid="broker-rabbitmq-exchange-type"
        width="100%"
        @update:modelValue="patch('exchangeType', $event)"
      />
    </div>

    <div class="form-row form-row--inline">
      <label class="form-label toggle-label">
        <input
          type="checkbox"
          class="toggle-check"
          :checked="modelValue.publisherConfirms !== false"
          aria-label="Enable publisher confirms (required for accurate ack latency)"
          data-testid="broker-rabbitmq-confirms"
          @change="patch('publisherConfirms', $event.target.checked)"
        />
        Publisher confirms (recommended)
      </label>
    </div>

    <details class="tls-section">
      <summary class="tls-summary">Optional mTLS certificates</summary>
      <div class="tls-body">
        <div class="form-row">
          <label for="rmq-ca-cert" class="form-label">
            <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
            CA Certificate (PEM)
          </label>
          <textarea id="rmq-ca-cert" class="form-textarea sensitive" rows="5" placeholder="-----BEGIN CERTIFICATE-----" :value="modelValue.tlsCaCert" aria-label="CA certificate PEM block" data-testid="broker-rabbitmq-ca-cert" @input="patch('tlsCaCert', $event.target.value)" />
        </div>
        <div class="form-row">
          <label for="rmq-client-cert" class="form-label">
            <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
            Client Certificate (PEM)
          </label>
          <textarea id="rmq-client-cert" class="form-textarea sensitive" rows="5" placeholder="-----BEGIN CERTIFICATE-----" :value="modelValue.tlsClientCert" aria-label="Client certificate PEM block" data-testid="broker-rabbitmq-client-cert" @input="patch('tlsClientCert', $event.target.value)" />
        </div>
        <div class="form-row">
          <label for="rmq-client-key" class="form-label">
            <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
            Client Key (PEM)
          </label>
          <textarea id="rmq-client-key" class="form-textarea sensitive" rows="5" placeholder="-----BEGIN RSA PRIVATE KEY-----" :value="modelValue.tlsClientKey" aria-label="Client private key PEM block" data-testid="broker-rabbitmq-client-key" @input="patch('tlsClientKey', $event.target.value)" />
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
