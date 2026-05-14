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
  { value: 'none', label: 'No authentication' },
  { value: 'user_pass', label: 'Username / Password' },
  { value: 'token', label: 'Token' },
  { value: 'nkey', label: 'NKey seed' },
  { value: 'creds_file', label: 'Credentials file (.creds)' },
]

const authMode = computed(() => props.modelValue.authMode || 'none')
const showUserPass = computed(() => authMode.value === 'user_pass')
const showToken = computed(() => authMode.value === 'token')
const showNKey = computed(() => authMode.value === 'nkey')
const showCredsFile = computed(() => authMode.value === 'creds_file')
</script>

<template>
  <fieldset class="broker-form" aria-label="NATS / JetStream configuration">
    <legend class="sr-only">NATS / JetStream configuration</legend>

    <div class="form-row">
      <label for="nats-servers" class="form-label">
        Servers
        <span class="required" aria-label="required">*</span>
      </label>
      <input
        id="nats-servers"
        type="text"
        class="form-input"
        placeholder="nats://127.0.0.1:4222"
        :value="modelValue.servers"
        aria-required="true"
        data-testid="broker-nats-servers"
        @input="patch('servers', $event.target.value)"
      />
    </div>

    <div class="form-row form-row--inline">
      <label class="form-label toggle-label">
        <input
          type="checkbox"
          class="toggle-check"
          :checked="modelValue.useJetStream"
          aria-label="Use JetStream"
          data-testid="broker-nats-jetstream"
          @change="patch('useJetStream', $event.target.checked)"
        />
        Use JetStream
      </label>
    </div>

    <div class="form-row">
      <label class="form-label" id="nats-auth-label">Auth Mode</label>
      <Select
        :modelValue="authMode"
        :options="authModeOptions"
        placeholder="Select auth mode"
        aria-labelledby="nats-auth-label"
        testid="broker-nats-auth-mode"
        width="100%"
        @update:modelValue="patch('authMode', $event)"
      />
    </div>

    <template v-if="showUserPass">
      <div class="form-row">
        <label for="nats-username" class="form-label">Username</label>
        <input id="nats-username" type="text" class="form-input" :value="modelValue.username" data-testid="broker-nats-username" @input="patch('username', $event.target.value)" />
      </div>
      <div class="form-row">
        <label for="nats-password" class="form-label">
          <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
          Password
        </label>
        <input id="nats-password" type="password" class="form-input" :value="modelValue.password" aria-label="NATS password (sensitive)" data-testid="broker-nats-password" @input="patch('password', $event.target.value)" />
      </div>
    </template>

    <div v-if="showToken" class="form-row">
      <label for="nats-token" class="form-label">
        <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
        Token
      </label>
      <input id="nats-token" type="password" class="form-input" :value="modelValue.token" aria-label="NATS auth token (sensitive)" data-testid="broker-nats-token" @input="patch('token', $event.target.value)" />
    </div>

    <div v-if="showNKey" class="form-row">
      <label for="nats-nkey" class="form-label">
        <span class="sensitive-icon" aria-hidden="true" title="Sensitive credential">&#128274;</span>
        NKey Seed
      </label>
      <input id="nats-nkey" type="password" class="form-input" placeholder="SUAC…" :value="modelValue.nkeySeed" aria-label="NKey seed (sensitive)" data-testid="broker-nats-nkey-seed" @input="patch('nkeySeed', $event.target.value)" />
    </div>

    <div v-if="showCredsFile" class="form-row">
      <label for="nats-creds" class="form-label">Credentials file path</label>
      <input id="nats-creds" type="text" class="form-input" placeholder="/home/user/.nkeys/creds/my.creds" :value="modelValue.credsFile" data-testid="broker-nats-creds-file" @input="patch('credsFile', $event.target.value)" />
    </div>

    <div class="form-row form-row--inline">
      <label class="form-label toggle-label">
        <input
          type="checkbox"
          class="toggle-check"
          :checked="modelValue.insecureSkipVerify"
          aria-label="Skip TLS certificate verification"
          data-testid="broker-nats-insecure"
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
.form-input:focus {
  outline: 2px solid var(--accent, #4f8cff);
  outline-offset: 1px;
  border-color: var(--accent, #4f8cff);
}
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
</style>
