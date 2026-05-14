<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useDistLoadStore } from '../../stores/distload'
import Select from '../common/Select.vue'
import CostEstimateCard from './CostEstimateCard.vue'
import PayloadEditor from './PayloadEditor.vue'
import RampControls from './RampControls.vue'
import LoadTestBrokerKafka from './LoadTestBrokerKafka.vue'
import LoadTestBrokerNATS from './LoadTestBrokerNATS.vue'
import LoadTestBrokerRabbitMQ from './LoadTestBrokerRabbitMQ.vue'
import LoadTestBrokerPubSub from './LoadTestBrokerPubSub.vue'
import LoadTestBrokerAMQP1 from './LoadTestBrokerAMQP1.vue'
import LoadTestBrokerREST from './LoadTestBrokerREST.vue'
import ScenarioBuilder from './ScenarioBuilder.vue'
import { isPrivateOrLoopback } from '../../lib/destinationValidation'

// Unified Load Test form. Drives both local and cloud (distributed)
// runs through a single spec shape — runner toggle decides the
// regions/credit machinery vs the local quota gate.

const store = useDistLoadStore()
const {
  regions, regionsLoading, creditBalance, error: storeError,
  loading, isRunning, presets, brokerKinds, localQuota,
} = storeToRefs(store)

// ── form state ────────────────────────────────────────────────────────
const testName = ref('')
const runner = ref('local') // 'local' | 'cloud'
// testType separates the two genuinely different shapes of test:
//   - 'eventbus' → publishing to Kafka / NATS / RabbitMQ / Pub-Sub / AMQP-1.0.
//     Needs a destination (topic / subject / queue / target).
//   - 'rest'     → directly hitting an HTTP endpoint. URL and verb live on
//     the REST sub-form; no broker destination applies.
// We remember the user's eventbus selection so flipping rest→eventbus
// doesn't lose their choice.
const testType = ref('eventbus') // 'eventbus' | 'rest'
// REST sub-mode: 'simple' (single endpoint via LoadTestBrokerREST) vs
// 'scenario' (multi-step ScenarioBuilder). Scenario requires the Cloud
// runner — backend rejects local-mode scenarios.
const restMode = ref('simple') // 'simple' | 'scenario'
const scenario = ref({
  auth: { mode: 'none' },
  endpoints: [{ method: 'POST', url: '', headers: {}, body: '', expect: null, chain: [] }],
})
const lastEventBusKind = ref('kafka')
const selectedPresetId = ref('')
const brokerKind = ref('kafka')
const brokerConfig = ref(defaultBrokerConfig('kafka'))
const destination = ref('')
const messageCount = ref(10_000)
const customCount = ref(false)
const countInput = ref(10_000)
const workers = ref(4)
const payloadSize = ref(1024)
const timeoutMins = ref(15)
const selectedRegions = ref([])

const ramp = ref({
  profile: 'constant', rate: 100, durationSec: 0, rampTo: 500,
  stepBy: 50, stepEverySec: 10, spikeCount: 6, spikeSize: 10_000, spikeIdleSec: 30,
})

const payload = ref({
  source: 'paste', bytes: '', filename: '', filePath: '',
  fileMode: 'template', aiPrompt: '',
})

const validationErrors = ref([])

// ── static option sets ───────────────────────────────────────────────
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
]
// Static fallback so the form is usable before ListDistLoadBrokerKinds
// resolves — same labels as the legacy panel.
const BROKER_LABELS = {
  kafka: 'Apache Kafka',
  nats: 'NATS / JetStream',
  rabbitmq: 'RabbitMQ',
  pubsub: 'Google Pub/Sub',
  amqp1: 'Solace / AMQP 1.0',
  rest: 'REST / HTTP',
}
// The protocol dropdown is only shown in 'eventbus' mode and hides
// REST from the list — REST is reached via the top-level test-type
// toggle so it can't be a sub-option here too.
const brokerKindOptions = computed(() => {
  const base = brokerKinds.value?.length ? brokerKinds.value : Object.keys(BROKER_LABELS)
  return base
    .filter((k) => k !== 'rest')
    .map((k) => ({ value: k, label: BROKER_LABELS[k] ?? k }))
})

const presetOptions = computed(() => [
  { value: '', label: 'No preset — configure manually' },
  ...presets.value.map((p) => ({ value: p.id, label: `${p.name} — ${p.whenToUse}` })),
])

const destinationLabel = computed(() => {
  switch (brokerKind.value) {
    case 'kafka': return 'Topic'
    case 'pubsub': return 'Topic'
    case 'nats': return 'Subject'
    case 'rabbitmq': return 'Exchange / Routing Key'
    case 'amqp1': return 'Target'
    case 'rest': return 'URL path or full URL (defaults to broker BaseURL)'
    default: return 'Destination'
  }
})

// ── default broker config shapes ──────────────────────────────────────
function defaultBrokerConfig(kind) {
  switch (kind) {
    case 'pubsub': return { projectId: '', authMode: 'adc', serviceAccountJson: '', endpoint: '' }
    case 'nats': return { servers: '', useJetStream: false, authMode: 'none', username: '', password: '', token: '', nkeySeed: '', credsFile: '', insecureSkipVerify: false }
    case 'kafka': return { bootstrapServers: '', clientId: '', authMode: 'none', username: '', password: '', oauthBearerToken: '', tlsCaCert: '', tlsClientCert: '', tlsClientKey: '', acks: 'all', insecureSkipVerify: false }
    case 'rabbitmq': return { url: '', exchange: '', exchangeType: 'topic', publisherConfirms: true, tlsCaCert: '', tlsClientCert: '', tlsClientKey: '', insecureSkipVerify: false }
    case 'amqp1': return { url: '', authMode: 'none', username: '', password: '', bearerToken: '', senderTarget: '', tlsCaCert: '', tlsClientCert: '', tlsClientKey: '', insecureSkipVerify: false }
    case 'rest': return { baseURL: '', method: 'POST', path: '', headers: [], contentType: 'application/json', timeoutSeconds: 30, insecureSkipTLS: false, basicAuthUser: '', basicAuthPassword: '', bearerToken: '', successCodes: [200, 201, 202, 204] }
    default: return {}
  }
}

watch(brokerKind, (k) => { brokerConfig.value = defaultBrokerConfig(k) })

// Top-level test-type toggle drives brokerKind. Switching to REST forces
// brokerKind='rest' and stashes the previous eventbus kind so flipping
// back restores it. Switching to Event Bus restores the remembered
// kind (or 'kafka' on first ever switch).
watch(testType, (t) => {
  if (t === 'rest') {
    if (brokerKind.value !== 'rest') lastEventBusKind.value = brokerKind.value
    brokerKind.value = 'rest'
  } else {
    brokerKind.value = lastEventBusKind.value || 'kafka'
  }
})

// ── preset apply ──────────────────────────────────────────────────────
function applyPreset(id) {
  selectedPresetId.value = id
  if (!id) return
  const p = store.getPreset(id)
  if (!p?.spec) return
  const s = p.spec
  if (s.count != null) {
    const known = COUNT_OPTIONS.find((o) => o.value === s.count)
    if (known) { customCount.value = false; messageCount.value = s.count }
    else { customCount.value = true; countInput.value = s.count }
  }
  if (s.workers) workers.value = s.workers
  if (s.ramp) {
    ramp.value = {
      ...ramp.value,
      profile: s.ramp.kind ?? s.ramp.profile ?? 'constant',
      rate: s.ramp.rate ?? ramp.value.rate,
      // backend may use durationNs for Go-side duration encoding.
      durationSec: s.ramp.durationNs ? Math.round(s.ramp.durationNs / 1e9) : (s.ramp.durationSec ?? 0),
      rampTo: s.ramp.rampTo ?? ramp.value.rampTo,
      stepBy: s.ramp.stepBy ?? ramp.value.stepBy,
      stepEverySec: s.ramp.stepEveryNs ? Math.round(s.ramp.stepEveryNs / 1e9) : (s.ramp.stepEverySec ?? 10),
      spikeCount: s.ramp.spikeCount ?? ramp.value.spikeCount,
      spikeSize: s.ramp.spikeSize ?? ramp.value.spikeSize,
      spikeIdleSec: s.ramp.spikeIdleNs ? Math.round(s.ramp.spikeIdleNs / 1e9) : (s.ramp.spikeIdleSec ?? 30),
    }
  }
}

// ── local-quota helpers ──────────────────────────────────────────────
const quotaExceeded = computed(() => {
  const q = localQuota.value
  if (!q || q.isPro) return false
  return q.used >= q.limit
})
const showQuotaBadge = computed(() => {
  const q = localQuota.value
  return q && !q.isPro
})

// ── effective count ──────────────────────────────────────────────────
const effectiveCount = computed(() => customCount.value ? countInput.value : messageCount.value)

// REST scenario mode swaps in spec.scenario and uses an empty REST broker
// config — the SaaS runner reads endpoints/auth from the scenario field
// instead of the single-endpoint REST broker shape.
const useScenario = computed(() => testType.value === 'rest' && restMode.value === 'scenario')

// ── spec ─────────────────────────────────────────────────────────────
const spec = computed(() => ({
  name: testName.value || `Load test ${new Date().toLocaleDateString()}`,
  runner: runner.value,
  presetId: selectedPresetId.value || '',
  regions: runner.value === 'cloud' ? selectedRegions.value.map(r => ({
    provider: r.provider, region: r.region,
    instanceType: r.instanceType || 't3.medium', count: r.count || 1,
  })) : [],
  broker: useScenario.value
    ? { kind: 'rest', rest: {} }
    : { kind: brokerKind.value, [brokerKind.value]: brokerConfig.value },
  scenario: useScenario.value ? scenario.value : undefined,
  destination: destination.value,
  payloadSize: payloadSize.value,
  count: effectiveCount.value,
  workers: workers.value,
  rampProfile: ramp.value.profile,
  rampRate: ramp.value.rate,
  ramp: ramp.value,
  timeoutMins: timeoutMins.value,
  payload: {
    source: payload.value.source,
    bytes: payload.value.bytes,
    filename: payload.value.filename,
    filePath: payload.value.filePath,
    fileMode: payload.value.fileMode,
    aiPrompt: payload.value.aiPrompt,
  },
}))

// ── validation ───────────────────────────────────────────────────────
function validate() {
  const errs = []
  // Event Bus needs a destination on the topic/queue/subject field;
  // REST has no analogous concept — its URL lives in the broker form.
  if (testType.value === 'eventbus') {
    if (!destination.value) errs.push('Destination is required')
  } else if (testType.value === 'rest') {
    if (useScenario.value) {
      // Scenario tests run only on the Cloud runner — the local runner
      // rejects them with a clear error from the backend. Surface that
      // upfront so the user doesn't have to read a backend stacktrace.
      if (runner.value === 'local') errs.push('Scenario tests require the Cloud runner — switch Runner to Cloud regions.')
      const eps = scenario.value?.endpoints || []
      if (!eps.length) errs.push('Scenario requires at least one endpoint')
      else if (eps.some(e => !e.url || !e.method)) errs.push('Every scenario endpoint needs a URL and method')
    } else if (!brokerConfig.value?.baseURL) {
      errs.push('Base URL is required for REST tests')
    }
  }
  if (effectiveCount.value <= 0 || effectiveCount.value > 10_000_000) errs.push('Message count must be between 1 and 10M')
  if (workers.value <= 0) errs.push('Workers must be > 0')
  if (runner.value === 'cloud') {
    if (selectedRegions.value.length === 0) errs.push('Select at least one region')
    // The cloud-reachability check applies to whichever endpoint the
    // worker will actually dial — destination for brokers, baseURL for
    // REST. Both end up as a hostname the user must expose to VMs.
    const target = testType.value === 'rest' ? brokerConfig.value?.baseURL : destination.value
    const privateReason = isPrivateOrLoopback(target)
    if (privateReason) {
      const label = testType.value === 'rest' ? 'Base URL' : 'Destination'
      errs.push(`${label} is ${privateReason} — cloud VMs can't reach it. Use a public hostname or a VPN-accessible endpoint.`)
    }
  }
  validationErrors.value = errs
}
watch(spec, validate, { deep: true })

const canStart = computed(() => {
  if (loading.value || isRunning.value) return false
  if (validationErrors.value.length > 0) return false
  if (runner.value === 'local' && quotaExceeded.value) return false
  if (runner.value === 'cloud' && selectedRegions.value.length === 0) return false
  // Endpoint check varies by test type — destination for brokers,
  // baseURL on the REST config sub-form.
  if (testType.value === 'eventbus' && !destination.value) return false
  if (testType.value === 'rest' && !useScenario.value && !brokerConfig.value?.baseURL) return false
  if (useScenario.value && runner.value === 'local') return false
  return true
})

// ── regions ──────────────────────────────────────────────────────────
const availableRegions = computed(() => regions.value ?? [])

function addRegion() {
  selectedRegions.value.push({
    provider: 'aws', region: 'us-east-1', instanceType: 't3.medium', count: 1,
    _key: Date.now() + Math.random(),
  })
}
function removeRegion(i) { selectedRegions.value.splice(i, 1) }

async function handleStart() {
  if (!canStart.value) return
  try { await store.start(spec.value) } catch { /* surfaced via store.error */ }
}

onMounted(() => {
  store.loadRegions()
  store.loadCredits()
  store.loadPresets()
  store.loadBrokerKinds()
  store.loadLocalQuota()
})
</script>

<template>
  <div class="distload-form">
    <div class="form-header">
      <h2>Load Test</h2>
      <div v-if="runner === 'cloud'" class="credit-badge" :class="{ low: creditBalance != null && creditBalance < 100 }">
        {{ creditBalance != null ? `${creditBalance} credits` : 'Loading…' }}
      </div>
      <div
        v-else-if="showQuotaBadge"
        class="quota-badge"
        :class="{ exceeded: quotaExceeded }"
        :title="quotaExceeded ? 'Free-tier daily local quota reached. Upgrade to Pro for unlimited local runs.' : ''"
        data-testid="distload-local-quota"
      >
        {{ localQuota.used }}/{{ localQuota.limit }} free local runs today
      </div>
    </div>

    <div v-if="validationErrors.length" class="error-banner">
      <div v-for="(e, i) in validationErrors" :key="i" class="error-item">{{ e }}</div>
    </div>
    <div v-if="storeError" class="error-banner">{{ storeError }}</div>

    <!-- Test name -->
    <section class="section">
      <label class="field-label">Test Name</label>
      <input v-model="testName" class="input" placeholder="Optional name for this run" />
    </section>

    <!-- Runner toggle -->
    <section class="section">
      <label class="field-label">Runner</label>
      <div class="seg" role="radiogroup" aria-label="Runner mode">
        <button type="button" class="seg-btn" :class="{ active: runner === 'local' }" role="radio" :aria-checked="runner === 'local'" data-testid="distload-runner-local" @click="runner = 'local'">Local (this machine)</button>
        <button type="button" class="seg-btn" :class="{ active: runner === 'cloud' }" role="radio" :aria-checked="runner === 'cloud'" data-testid="distload-runner-cloud" @click="runner = 'cloud'">Cloud regions</button>
      </div>
    </section>

    <!-- Test type toggle — defines what kind of test this is. The two
         shapes drive different forms below, so it's a top-level choice
         rather than a buried "rest" option in the protocol dropdown. -->
    <section class="section">
      <label class="field-label">Test type</label>
      <div class="seg" role="radiogroup" aria-label="Test type">
        <button type="button" class="seg-btn" :class="{ active: testType === 'eventbus' }" role="radio" :aria-checked="testType === 'eventbus'" data-testid="distload-type-eventbus" @click="testType = 'eventbus'">Event Bus</button>
        <button type="button" class="seg-btn" :class="{ active: testType === 'rest' }" role="radio" :aria-checked="testType === 'rest'" data-testid="distload-type-rest" @click="testType = 'rest'">REST / HTTP</button>
      </div>
      <p class="field-hint">
        {{ testType === 'eventbus'
          ? 'Publish messages to a message broker (Kafka, NATS, RabbitMQ, Pub/Sub, AMQP 1.0).'
          : 'Issue HTTP requests directly to a REST endpoint.' }}
      </p>
    </section>

    <!-- Preset -->
    <section class="section">
      <label class="field-label">Preset</label>
      <Select :modelValue="selectedPresetId" :options="presetOptions" placeholder="No preset — configure manually" testid="distload-preset" width="100%" @update:modelValue="applyPreset($event)" />
      <p v-if="selectedPresetId" class="hint">{{ presets.find(p => p.id === selectedPresetId)?.description }}</p>
    </section>

    <!-- Event Bus mode: broker dropdown + matching sub-form + destination. -->
    <template v-if="testType === 'eventbus'">
      <section class="section">
        <label class="field-label">Protocol / Broker</label>
        <Select :modelValue="brokerKind" :options="brokerKindOptions" testid="distload-broker-kind" width="100%" @update:modelValue="brokerKind = $event" />
        <div class="broker-sub">
          <LoadTestBrokerKafka    v-if="brokerKind === 'kafka'"        v-model="brokerConfig" />
          <LoadTestBrokerNATS     v-else-if="brokerKind === 'nats'"     v-model="brokerConfig" />
          <LoadTestBrokerRabbitMQ v-else-if="brokerKind === 'rabbitmq'" v-model="brokerConfig" />
          <LoadTestBrokerPubSub   v-else-if="brokerKind === 'pubsub'"   v-model="brokerConfig" />
          <LoadTestBrokerAMQP1    v-else-if="brokerKind === 'amqp1'"    v-model="brokerConfig" />
        </div>
      </section>

      <section class="section">
        <label class="field-label">{{ destinationLabel }}</label>
        <input v-model="destination" class="input" :placeholder="destinationLabel" data-testid="distload-destination" />
      </section>
    </template>

    <!-- REST mode: simple = single endpoint via LoadTestBrokerREST;
         scenario = multi-step ScenarioBuilder. Scenario requires Cloud. -->
    <template v-else>
      <section class="section">
        <label class="field-label">REST mode</label>
        <div class="seg" role="radiogroup" aria-label="REST mode">
          <button type="button" class="seg-btn" :class="{ active: restMode === 'simple' }" role="radio"
            :aria-checked="restMode === 'simple'" data-testid="distload-restmode-simple"
            @click="restMode = 'simple'">Simple (single endpoint)</button>
          <button type="button" class="seg-btn" :class="{ active: restMode === 'scenario' }" role="radio"
            :aria-checked="restMode === 'scenario'" data-testid="distload-restmode-scenario"
            @click="restMode = 'scenario'">Scenario (multi-step)</button>
        </div>
        <p class="field-hint">
          {{ restMode === 'scenario'
            ? 'Build a multi-step plan with chained calls and JSON-path assertions. Cloud runner only.'
            : 'Hit one HTTP endpoint repeatedly.' }}
        </p>
      </section>
      <section class="section">
        <label class="field-label">{{ restMode === 'scenario' ? 'Scenario' : 'REST endpoint' }}</label>
        <LoadTestBrokerREST v-if="restMode === 'simple'" v-model="brokerConfig" />
        <ScenarioBuilder v-else v-model="scenario" />
      </section>
    </template>

    <!-- Payload -->
    <section class="section">
      <label class="field-label">Payload</label>
      <PayloadEditor v-model="payload" />
    </section>

    <!-- Count / Workers / Payload size -->
    <section class="section">
      <div class="row-3col">
        <div class="form-group">
          <label class="field-label">Message Count</label>
          <Select v-if="!customCount" :modelValue="messageCount" :options="COUNT_OPTIONS" testid="distload-count-select" width="100%" @update:modelValue="messageCount = $event" />
          <input v-else v-model.number="countInput" type="number" class="input" min="1" max="10000000" data-testid="distload-count-custom" />
          <button class="link-btn" type="button" @click="customCount = !customCount; if (customCount) countInput = messageCount; else messageCount = countInput">
            {{ customCount ? 'Use presets' : 'Custom count' }}
          </button>
        </div>
        <div class="form-group">
          <label class="field-label">Workers</label>
          <input v-model.number="workers" type="number" class="input" min="1" max="500" />
        </div>
        <div class="form-group">
          <label class="field-label">Payload size hint (bytes)</label>
          <input v-model.number="payloadSize" type="number" class="input" min="1" max="1048576" />
        </div>
      </div>
    </section>

    <!-- Ramp -->
    <section class="section">
      <label class="field-label">Ramp Profile</label>
      <RampControls v-model="ramp" />
    </section>

    <!-- Timeout -->
    <section class="section">
      <label class="field-label">Timeout (minutes)</label>
      <input v-model.number="timeoutMins" type="number" class="input" min="1" max="120" />
    </section>

    <!-- Cloud regions -->
    <section v-if="runner === 'cloud'" class="section">
      <div class="row-between">
        <label class="field-label">Regions</label>
        <button class="btn-sm" :disabled="regionsLoading" @click="addRegion">+ Add Region</button>
      </div>
      <div v-if="regionsLoading" class="hint">Loading regions…</div>
      <div v-for="(reg, idx) in selectedRegions" :key="reg._key" class="region-row">
        <select v-model="reg.provider" class="input region-provider">
          <option v-for="opt in REGION_PROVIDER_OPTIONS" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
        </select>
        <select v-model="reg.region" class="input region-select">
          <option value="" disabled>Region…</option>
          <option v-for="r in availableRegions.filter(r => r.provider === reg.provider)" :key="r.region" :value="r.region">{{ r.label || r.region }}</option>
        </select>
        <input v-model.number="reg.count" type="number" class="input region-vms" min="1" max="20" placeholder="VMs" />
        <button class="btn-sm btn-remove" @click="removeRegion(idx)">×</button>
      </div>
      <div v-if="selectedRegions.length === 0" class="hint">Add at least one region to run the test from</div>
    </section>

    <!-- Footer: cost (cloud only) + start -->
    <div class="form-footer">
      <CostEstimateCard v-if="runner === 'cloud'" :spec="spec" />
      <span v-else class="footer-spacer" />
      <button
        class="btn-primary"
        :disabled="!canStart"
        :title="quotaExceeded ? 'Free-tier daily local quota reached. Upgrade to Pro for unlimited local runs.' : ''"
        data-testid="distload-start-btn"
        @click="handleStart"
      >
        {{ loading ? 'Starting…' : isRunning ? 'Test Running…' : runner === 'cloud' ? 'Start Distributed Test' : 'Start Load Test' }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.distload-form { flex: 1; overflow-y: auto; padding: 16px; display: flex; flex-direction: column; gap: 16px; max-width: 920px; }
.form-header { display: flex; justify-content: space-between; align-items: center; }
.form-header h2 { margin: 0; font-size: 16px; font-weight: 600; color: var(--text); }
.credit-badge { font-size: 12px; font-weight: 500; padding: 4px 10px; border-radius: 6px; background: rgba(79,142,247,0.12); color: var(--accent2); }
.credit-badge.low { background: rgba(208,88,88,0.12); color: #d05858; }
.quota-badge { font-size: 12px; font-weight: 500; padding: 4px 10px; border-radius: 6px; background: rgba(79,142,247,0.12); color: var(--accent2); }
.quota-badge.exceeded { background: rgba(208,88,88,0.12); color: #d05858; }
.error-banner { background: rgba(208,88,88,0.1); border: 1px solid rgba(208,88,88,0.3); border-radius: 6px; padding: 8px 12px; font-size: 12px; color: #d05858; }
.error-item + .error-item { margin-top: 4px; }
.section { display: flex; flex-direction: column; gap: 8px; padding-bottom: 14px; border-bottom: 1px solid var(--border); }
.field-label { font-size: 11px; font-weight: 600; color: var(--text2); text-transform: uppercase; letter-spacing: 0.04em; }
.input { background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 6px 10px; font-size: 13px; color: var(--text); font-family: inherit; width: 100%; box-sizing: border-box; }
.input:focus { outline: 2px solid var(--accent, #4f8cff); outline-offset: 1px; border-color: var(--accent, #4f8cff); }
.seg { display: inline-flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; align-self: flex-start; }
.seg-btn { background: var(--bg3); border: none; padding: 6px 14px; font-size: 12px; color: var(--text2); cursor: pointer; font-family: inherit; }
.seg-btn.active { background: var(--accent, #4f8cff); color: #fff; }
.hint { font-size: 12px; color: var(--text3); font-style: italic; margin: 0; }
.field-hint { font-size: 12px; color: var(--text3); margin: 4px 0 0; }
.broker-sub { margin-top: 8px; }
.row-3col { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; }
.form-group { display: flex; flex-direction: column; gap: 4px; }
.row-between { display: flex; align-items: center; justify-content: space-between; }
.region-row { display: flex; gap: 6px; align-items: center; }
.region-provider { width: 80px; flex-shrink: 0; }
.region-select { flex: 1; }
.region-vms { width: 70px; flex-shrink: 0; }
.btn-sm { padding: 4px 10px; border-radius: 6px; font-size: 11px; font-weight: 500; border: 1px solid var(--border); background: var(--bg3); color: var(--text2); cursor: pointer; }
.btn-sm:hover { background: var(--bg4); color: var(--text); }
.btn-remove { color: #d05858; border-color: rgba(208,88,88,0.3); }
.link-btn { background: none; border: none; color: var(--accent2); font-size: 11px; cursor: pointer; padding: 2px 0; text-align: left; align-self: flex-start; }
.link-btn:hover { text-decoration: underline; }
.form-footer { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding-top: 12px; }
.footer-spacer { flex: 1; }
.btn-primary { padding: 8px 20px; border-radius: 8px; font-size: 13px; font-weight: 600; border: none; background: var(--accent); color: white; cursor: pointer; }
.btn-primary:hover:not(:disabled) { opacity: 0.9; }
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
