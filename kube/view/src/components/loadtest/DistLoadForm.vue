<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useDistLoadStore } from '../../stores/distload'
import Select from '../common/Select.vue'
import CostEstimateCard from './CostEstimateCard.vue'

const store = useDistLoadStore()
const { regions, creditBalance, error: storeError, loading, isRunning, isTerminal } = storeToRefs(store)

const selectedRegions = ref([])
const brokerKind = ref('kafka')
const destination = ref('')
const messageCount = ref(10000)
const payloadSize = ref(1024)
const workers = ref(4)
const rampProfile = ref('constant')
const rampRate = ref(100)
const timeoutMins = ref(15)
const testName = ref('')
const validationErrors = ref([])

const REGION_PROVIDER_OPTIONS = [
  { value: 'aws', label: 'AWS' },
  { value: 'gcp', label: 'GCP' },
]

const COUNT_OPTIONS = [
  { value: 1000, label: '1,000' },
  { value: 10000, label: '10,000' },
  { value: 100000, label: '100,000' },
  { value: 500000, label: '500,000' },
  { value: 1000000, label: '1,000,000' },
  { value: 'custom', label: 'Custom…' },
]

const RAMP_OPTIONS = [
  { value: 'constant', label: 'Constant rate' },
  { value: 'linear', label: 'Linear ramp-up' },
  { value: 'step', label: 'Step ramp' },
  { value: 'spike', label: 'Spike bursts' },
]

const customCount = ref(false)
const countInput = ref(messageCount.value)

const spec = computed(() => ({
  name: testName.value || `Load test ${new Date().toLocaleDateString()}`,
  regions: selectedRegions.value.map(r => ({
    provider: r.provider,
    region: r.region,
    instanceType: r.instanceType || 't3.medium',
    count: r.count || 1,
  })),
  broker: { kind: brokerKind.value },
  destination: destination.value,
  payloadSize: payloadSize.value,
  count: customCount.value ? countInput.value : messageCount.value,
  workers: workers.value,
  rampProfile: rampProfile.value,
  rampRate: rampRate.value,
  timeoutMins: timeoutMins.value,
}))

const brokerKinds = [
  { value: 'kafka', label: 'Apache Kafka' },
  { value: 'nats', label: 'NATS / JetStream' },
  { value: 'rabbitmq', label: 'RabbitMQ' },
  { value: 'pubsub', label: 'Google Pub/Sub' },
  { value: 'amqp1', label: 'Solace / AMQP 1.0' },
]

const availableRegions = computed(() => regions.value ?? [])

const canStart = computed(() => {
  if (loading.value || isRunning.value) return false
  if (validationErrors.value.length > 0) return false
  if (selectedRegions.value.length === 0) return false
  if (!destination.value) return false
  return true
})

function validate() {
  const errors = []
  if (selectedRegions.value.length === 0) errors.push('Select at least one region')
  if (!destination.value) errors.push('Enter a broker destination (topic / queue)')
  const count = customCount.value ? countInput.value : messageCount.value
  if (count <= 0 || count > 10_000_000) errors.push('Message count must be between 1 and 10M')
  if (payloadSize.value <= 0) errors.push('Payload size must be > 0')
  if (workers.value <= 0) errors.push('Worker count must be > 0')
  if (destination.value === 'localhost') errors.push('Broker must be reachable from cloud VMs — use a public or VPN-accessible endpoint')
  validationErrors.value = errors
}

watch(spec, validate, { deep: true })

function addRegion() {
  selectedRegions.value.push({
    provider: 'aws',
    region: 'us-east-1',
    instanceType: 't3.medium',
    count: 1,
    _key: Date.now() + Math.random(),
  })
}

function removeRegion(index) {
  selectedRegions.value.splice(index, 1)
}

async function handleStart() {
  if (!canStart.value) return
  try {
    await store.start(spec.value)
  } catch (e) {
    // error handled in store
  }
}

onMounted(() => {
  store.loadRegions()
  store.loadCredits()
})
</script>

<template>
  <div class="distload-form">
    <div class="form-header">
      <h2>Distributed Load Test</h2>
      <div class="credit-badge" :class="{ low: creditBalance != null && creditBalance < 100 }">
        {{ creditBalance != null ? `${creditBalance} credits` : 'Loading…' }}
      </div>
    </div>

    <!-- Validation errors -->
    <div v-if="validationErrors.length" class="error-banner">
      <div v-for="(err, i) in validationErrors" :key="i" class="error-item">{{ err }}</div>
    </div>

    <!-- Store errors -->
    <div v-if="storeError" class="error-banner">{{ storeError }}</div>

    <div class="form-grid">
      <!-- Left column -->
      <div class="form-col">
        <label class="field-label">Test Name</label>
        <input v-model="testName" type="text" class="input" placeholder="Optional name for this run" />

        <label class="field-label">Broker Kind</label>
        <Select v-model="brokerKind" :options="brokerKinds" />

        <label class="field-label">Destination (topic / queue)</label>
        <input v-model="destination" type="text" class="input" placeholder="e.g. my-topic" />

        <label class="field-label">Message Count</label>
        <Select v-if="!customCount" v-model="messageCount" :options="COUNT_OPTIONS" />
        <div v-else class="row">
          <input v-model.number="countInput" type="number" class="input" min="1" max="10000000" />
          <button class="btn-sm" @click="customCount = false; countInput = messageCount">Presets</button>
        </div>
        <button v-if="!customCount" class="link-btn" @click="customCount = true; countInput = messageCount">Custom count</button>
      </div>

      <!-- Middle column -->
      <div class="form-col">
        <label class="field-label">Payload Size (bytes)</label>
        <input v-model.number="payloadSize" type="number" class="input" min="1" max="1048576" />

        <label class="field-label">Workers per VM</label>
        <input v-model.number="workers" type="number" class="input" min="1" max="100" />

        <label class="field-label">Ramp Profile</label>
        <Select v-model="rampProfile" :options="RAMP_OPTIONS" />

        <label class="field-label">Publish Rate (msg/s)</label>
        <input v-model.number="rampRate" type="number" class="input" min="1" max="100000" />

        <label class="field-label">Timeout (minutes)</label>
        <input v-model.number="timeoutMins" type="number" class="input" min="1" max="120" />
      </div>

      <!-- Right column: regions -->
      <div class="form-col">
        <div class="field-label-row">
          <label class="field-label">Regions</label>
          <button class="btn-sm" @click="addRegion" :disabled="regionsLoading">+ Add Region</button>
        </div>
        <div v-if="regionsLoading" class="loading-text">Loading available regions…</div>
        <div v-for="(reg, index) in selectedRegions" :key="reg._key" class="region-row">
          <select v-model="reg.provider" class="input region-provider">
            <option v-for="opt in REGION_PROVIDER_OPTIONS" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
          <select v-model="reg.region" class="input region-select">
            <option value="" disabled>Region…</option>
            <option
              v-for="r in availableRegions.filter(r => r.provider === reg.provider)"
              :key="r.region"
              :value="r.region"
            >{{ r.label || r.region }}</option>
          </select>
          <input v-model.number="reg.count" type="number" class="input region-vms" min="1" max="20" placeholder="VMs" />
          <button class="btn-sm btn-remove" @click="removeRegion(index)">×</button>
        </div>
        <div v-if="selectedRegions.length === 0" class="hint-text">Add at least one region to run the test from</div>
      </div>
    </div>

    <!-- Cost estimate + start -->
    <div class="form-footer">
      <CostEstimateCard :spec="spec" />
      <button
        class="btn-primary"
        :disabled="!canStart"
        @click="handleStart"
      >
        {{ loading ? 'Starting…' : isRunning ? 'Test Running…' : 'Start Distributed Test' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.distload-form { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 16px; }
.form-header { display: flex; justify-content: space-between; align-items: center; }
.form-header h2 { margin: 0; font-size: 16px; font-weight: 600; color: var(--text); }
.credit-badge { font-size: 12px; font-weight: 500; padding: 4px 10px; border-radius: 6px; background: rgba(79,142,247,0.12); color: var(--accent2); }
.credit-badge.low { background: rgba(208,88,88,0.12); color: #d05858; }
.error-banner { background: rgba(208,88,88,0.1); border: 1px solid rgba(208,88,88,0.3); border-radius: 6px; padding: 8px 12px; font-size: 12px; color: #d05858; }
.error-item + .error-item { margin-top: 4px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 20px; }
.form-col { display: flex; flex-direction: column; gap: 8px; }
.field-label { font-size: 11px; font-weight: 500; color: var(--text2); text-transform: uppercase; letter-spacing: 0.04em; }
.field-label-row { display: flex; align-items: center; justify-content: space-between; }
.input { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 6px 10px; font-size: 13px; color: var(--text); font-family: inherit; }
.input:focus { outline: none; border-color: var(--accent); }
.region-row { display: flex; gap: 6px; align-items: center; }
.region-provider { width: 70px; flex-shrink: 0; }
.region-select { flex: 1; }
.region-vms { width: 60px; flex-shrink: 0; }
.btn-sm { padding: 4px 10px; border-radius: 6px; font-size: 11px; font-weight: 500; border: 1px solid var(--border); background: var(--bg3); color: var(--text2); cursor: pointer; }
.btn-sm:hover { background: var(--bg4); color: var(--text); }
.btn-remove { color: #d05858; border-color: rgba(208,88,88,0.3); }
.btn-remove:hover { background: rgba(208,88,88,0.1); }
.link-btn { background: none; border: none; color: var(--accent2); font-size: 11px; cursor: pointer; padding: 2px 0; text-align: left; }
.link-btn:hover { text-decoration: underline; }
.loading-text { font-size: 12px; color: var(--text3); }
.hint-text { font-size: 12px; color: var(--text3); font-style: italic; }
.form-footer { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding-top: 12px; border-top: 1px solid var(--border); }
.btn-primary { padding: 8px 20px; border-radius: 8px; font-size: 13px; font-weight: 600; border: none; background: var(--accent); color: white; cursor: pointer; }
.btn-primary:hover:not(:disabled) { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
.row { display: flex; gap: 6px; align-items: center; }
</style>
