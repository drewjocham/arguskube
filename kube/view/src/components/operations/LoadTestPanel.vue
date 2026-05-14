<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { bus } from '../../lib/bus'
import { useLoadTestStore } from '../../stores/loadtest'
import Select from '../common/Select.vue'
import LoadTestBrokerPubSub from './loadtest/LoadTestBrokerPubSub.vue'
import LoadTestBrokerNATS from './loadtest/LoadTestBrokerNATS.vue'
import LoadTestBrokerKafka from './loadtest/LoadTestBrokerKafka.vue'
import LoadTestBrokerRabbitMQ from './loadtest/LoadTestBrokerRabbitMQ.vue'
import LoadTestBrokerAMQP1 from './loadtest/LoadTestBrokerAMQP1.vue'

// ── constants ────────────────────────────────────────────────────────
const KIND_LABELS = {
  pubsub:   'Google Pub/Sub',
  nats:     'NATS / JetStream',
  kafka:    'Apache Kafka',
  rabbitmq: 'RabbitMQ (AMQP 0.9)',
  amqp1:    'Solace / AMQP 1.0',
}

const COUNT_OPTIONS = [
  { value: 1_000,     label: '1,000' },
  { value: 10_000,    label: '10,000' },
  { value: 100_000,   label: '100,000' },
  { value: 500_000,   label: '500,000' },
  { value: 1_000_000, label: '1,000,000' },
  { value: 'custom',  label: 'Custom…' },
]

const RAMP_KIND_OPTIONS = [
  { value: 'constant', label: 'Constant rate' },
  { value: 'linear',   label: 'Linear ramp-up' },
  { value: 'step',     label: 'Step ramp' },
  { value: 'spike',    label: 'Spike bursts' },
]

const PAYLOAD_TABS = ['upload', 'paste', 'type']

// ── store ────────────────────────────────────────────────────────────
const store = useLoadTestStore()
const { presets, kinds, activeRunId, status, samplesBuffer, scaleLogBuffer, isRunning, isDone, throughputSeries, loading, error: storeError } = storeToRefs(store)

// ── form state ───────────────────────────────────────────────────────
const selectedPresetId = ref('')
const brokerKind = ref('kafka')
const brokerConfig = ref(defaultBrokerConfig('kafka'))

const destination = ref('')
const payloadTab = ref('paste')
const uploadedFilename = ref('')
const uploadedSize = ref(0)
const uploadedBytes = ref('')
const pasteJson = ref('{}')
const pasteError = ref('')
const typedJson = ref('')
const typedError = ref('')

const countChoice = ref(10_000)
const countCustom = ref(10_000)
const workers = ref(50)

const rampKind = ref('constant')
const rampRate = ref(100)
const rampDuration = ref(0)
const rampTo = ref(500)
const rampStepBy = ref(50)
const rampStepEvery = ref(10)
const rampSpikeCount = ref(6)
const rampSpikeSize = ref(10_000)
const rampSpikeIdle = ref(30)

const scalePlanOpen = ref(false)
const scaleNamespace = ref('')
const scaleDeployment = ref('')
const scalePreScaleToZero = ref(false)
const scaleMinReplicas = ref(0)
const scalePreTimeout = ref(120)
const scalePostTimeout = ref(600)

const validationErrors = ref([])

// ── destination label per broker kind ────────────────────────────────
const destinationLabel = computed(() => {
  switch (brokerKind.value) {
    case 'kafka':  return 'Topic'
    case 'pubsub': return 'Topic'
    case 'nats':   return 'Subject'
    case 'rabbitmq': return 'Exchange / Routing Key'
    case 'amqp1':  return 'Target'
    default:       return 'Topic / subject / queue name'
  }
})

// ── broker kinds as Select options ───────────────────────────────────
const brokerKindOptions = computed(() =>
  (kinds.value.length
    ? kinds.value
    : Object.keys(KIND_LABELS)
  ).map((k) => ({ value: k, label: KIND_LABELS[k] ?? k }))
)

// ── preset picker options ─────────────────────────────────────────────
const presetOptions = computed(() => [
  { value: '', label: 'No preset — configure manually' },
  ...presets.value.map((p) => ({ value: p.id, label: `${p.name} — ${p.whenToUse}` })),
])

// ── count ─────────────────────────────────────────────────────────────
const effectiveCount = computed(() =>
  countChoice.value === 'custom' ? Number(countCustom.value) : Number(countChoice.value)
)

// ── payload bytes ─────────────────────────────────────────────────────
const payloadKind = computed(() => {
  if (payloadTab.value === 'upload') return 'uploaded'
  if (payloadTab.value === 'paste') return 'pasted'
  return 'typed'
})

const payloadText = computed(() => {
  if (payloadTab.value === 'upload') return uploadedBytes.value
  if (payloadTab.value === 'paste') return pasteJson.value
  return typedJson.value
})

// ── default broker config shapes ──────────────────────────────────────
function defaultBrokerConfig(kind) {
  switch (kind) {
    case 'pubsub':   return { projectId: '', authMode: 'adc', serviceAccountJson: '', endpoint: '' }
    case 'nats':     return { servers: '', useJetStream: false, authMode: 'none', username: '', password: '', token: '', nkeySeed: '', credsFile: '', insecureSkipVerify: false }
    case 'kafka':    return { bootstrapServers: '', clientId: '', authMode: 'none', username: '', password: '', oauthBearerToken: '', tlsCaCert: '', tlsClientCert: '', tlsClientKey: '', acks: 'all', insecureSkipVerify: false }
    case 'rabbitmq': return { url: '', exchange: '', exchangeType: 'topic', publisherConfirms: true, tlsCaCert: '', tlsClientCert: '', tlsClientKey: '', insecureSkipVerify: false }
    case 'amqp1':    return { url: '', authMode: 'none', username: '', password: '', bearerToken: '', senderTarget: '', tlsCaCert: '', tlsClientCert: '', tlsClientKey: '', insecureSkipVerify: false }
    default:         return {}
  }
}

// ── broker kind change ────────────────────────────────────────────────
watch(brokerKind, (newKind) => {
  brokerConfig.value = defaultBrokerConfig(newKind)
})

// ── preset apply ──────────────────────────────────────────────────────
function applyPreset(id) {
  selectedPresetId.value = id
  if (!id) return
  const preset = store.getPreset(id)
  if (!preset) return
  const spec = preset.spec

  if (spec.count) {
    const known = COUNT_OPTIONS.find((o) => o.value === spec.count)
    if (known) { countChoice.value = spec.count } else { countChoice.value = 'custom'; countCustom.value = spec.count }
  }
  if (spec.workers) workers.value = spec.workers
  if (spec.ramp) {
    rampKind.value = spec.ramp.kind ?? 'constant'
    // Go durations are stored as nanoseconds in JSON (durationNs field).
    rampRate.value = spec.ramp.rate ?? 100
    rampDuration.value = spec.ramp.durationNs ? Math.round(spec.ramp.durationNs / 1e9) : 0
    rampTo.value = spec.ramp.rampTo ?? 500
    rampStepBy.value = spec.ramp.stepBy ?? 50
    rampStepEvery.value = spec.ramp.stepEveryNs ? Math.round(spec.ramp.stepEveryNs / 1e9) : 10
    rampSpikeCount.value = spec.ramp.spikeCount ?? 6
    rampSpikeSize.value = spec.ramp.spikeSize ?? 10_000
    rampSpikeIdle.value = spec.ramp.spikeIdleNs ? Math.round(spec.ramp.spikeIdleNs / 1e9) : 30
  }
  if (spec.scale && (spec.scale.namespace || spec.scale.deployment || spec.scale.preScaleToZero || spec.scale.minReplicas)) {
    scalePlanOpen.value = true
    scaleNamespace.value = spec.scale.namespace ?? ''
    scaleDeployment.value = spec.scale.deployment ?? ''
    scalePreScaleToZero.value = spec.scale.preScaleToZero ?? false
    scaleMinReplicas.value = spec.scale.minReplicas ?? 0
    scalePreTimeout.value = spec.scale.preScaleTimeoutNs ? Math.round(spec.scale.preScaleTimeoutNs / 1e9) : 120
    scalePostTimeout.value = spec.scale.postScaleTimeoutNs ? Math.round(spec.scale.postScaleTimeoutNs / 1e9) : 600
  } else {
    scalePlanOpen.value = false
  }
}

// ── JSON lint ─────────────────────────────────────────────────────────
function lintPaste() {
  if (!pasteJson.value.trim()) { pasteError.value = 'Payload cannot be empty'; return }
  try { JSON.parse(pasteJson.value); pasteError.value = '' } catch (e) { pasteError.value = `Invalid JSON: ${e.message}` }
}

function lintTyped() {
  if (!typedJson.value.trim()) { typedError.value = ''; return }
  try { JSON.parse(typedJson.value); typedError.value = '' } catch (e) { typedError.value = `Invalid JSON: ${e.message}` }
}

// ── file upload ───────────────────────────────────────────────────────
function onFileChange(e) {
  const file = e.target.files?.[0]
  if (!file) return
  uploadedFilename.value = file.name
  uploadedSize.value = file.size
  const reader = new FileReader()
  reader.onload = (ev) => { uploadedBytes.value = ev.target.result }
  reader.readAsText(file)
}

function onDrop(e) {
  e.preventDefault()
  const file = e.dataTransfer?.files?.[0]
  if (!file) return
  uploadedFilename.value = file.name
  uploadedSize.value = file.size
  const reader = new FileReader()
  reader.onload = (ev) => { uploadedBytes.value = ev.target.result }
  reader.readAsText(file)
}

// ── validation ────────────────────────────────────────────────────────
function validate() {
  const errs = []
  if (!destination.value.trim()) errs.push('Destination required')
  if (!payloadText.value.trim()) errs.push('Payload cannot be empty')
  if (payloadTab.value === 'paste' && pasteError.value) errs.push('Fix JSON lint errors in Paste tab')
  if (payloadTab.value === 'type' && typedError.value) errs.push('Fix JSON lint errors in Type tab')
  if (effectiveCount.value <= 0 && rampKind.value !== 'spike') errs.push('Count must be > 0')
  if (workers.value < 1 || workers.value > 500) errs.push('Workers must be 1–500')
  validationErrors.value = errs
  return errs.length === 0
}

watch([destination, payloadTab, pasteJson, typedJson, countChoice, countCustom, workers, rampKind], () => {
  if (validationErrors.value.length) validate()
})

const canStart = computed(() => !isRunning.value && !loading.value)

// ── build RunSpec ─────────────────────────────────────────────────────
function buildSpec() {
  const enc = new TextEncoder()
  const bytes = Array.from(enc.encode(payloadText.value))

  const ramp = { kind: rampKind.value }
  switch (rampKind.value) {
    case 'constant':
      ramp.rate = Number(rampRate.value)
      if (rampDuration.value > 0) ramp.durationNs = Number(rampDuration.value) * 1e9
      break
    case 'linear':
      ramp.rate = Number(rampRate.value)
      ramp.rampTo = Number(rampTo.value)
      ramp.durationNs = Number(rampDuration.value) * 1e9
      break
    case 'step':
      ramp.rate = Number(rampRate.value)
      ramp.stepBy = Number(rampStepBy.value)
      ramp.stepEveryNs = Number(rampStepEvery.value) * 1e9
      if (rampDuration.value > 0) ramp.durationNs = Number(rampDuration.value) * 1e9
      break
    case 'spike':
      ramp.spikeCount = Number(rampSpikeCount.value)
      ramp.spikeSize = Number(rampSpikeSize.value)
      ramp.spikeIdleNs = Number(rampSpikeIdle.value) * 1e9
      break
  }

  const brokerConfigKey = {
    pubsub: 'pubsub', nats: 'nats', kafka: 'kafka', rabbitmq: 'rabbitmq', amqp1: 'amqp1',
  }[brokerKind.value]

  const spec = {
    broker: {
      kind: brokerKind.value,
      [brokerConfigKey]: { ...brokerConfig.value },
    },
    destination: destination.value.trim(),
    payload: {
      kind: payloadKind.value,
      bytes,
      filename: uploadedFilename.value || undefined,
      size: bytes.length,
    },
    count: rampKind.value === 'spike' ? 0 : effectiveCount.value,
    workers: Number(workers.value),
    ramp,
  }

  const hasScale = scaleNamespace.value || scaleDeployment.value || scalePreScaleToZero.value || scaleMinReplicas.value > 0
  if (hasScale) {
    spec.scale = {
      namespace: scaleNamespace.value,
      deployment: scaleDeployment.value,
      preScaleToZero: scalePreScaleToZero.value,
      minReplicas: Number(scaleMinReplicas.value),
      preScaleTimeoutNs: Number(scalePreTimeout.value) * 1e9,
      postScaleTimeoutNs: Number(scalePostTimeout.value) * 1e9,
    }
  }

  return spec
}

// ── start / cancel ────────────────────────────────────────────────────
async function onStart() {
  if (!validate()) return
  const spec = buildSpec()
  try {
    await store.start(spec)
  } catch {
    // error surfaced from store
  }
}

async function onCancel() {
  await store.cancel()
}

// ── event bus subscriptions ───────────────────────────────────────────
bus.useWailsEvent('argus:loadtest:progress', (payload) => { store.onProgress(payload) })
bus.useWailsEvent('argus:loadtest:done', (payload) => { store.onDone(payload) })

// ── mount ─────────────────────────────────────────────────────────────
onMounted(async () => {
  await Promise.all([store.loadPresets(), store.loadKinds()])
})

// ── state badge helper ────────────────────────────────────────────────
function stateColor(state) {
  switch (state) {
    case 'running':  return '#3b82f6'
    case 'done':     return '#10b981'
    case 'canceled': return '#f5a623'
    case 'error':    return '#ef4444'
    default:         return '#8b8f96'
  }
}

// ── SVG throughput chart ──────────────────────────────────────────────
const CHART_H = 200
const CHART_W = 520 // viewBox width; SVG scales with CSS width:100%
const CHART_PAD = { top: 10, right: 10, bottom: 30, left: 48 }

const chartPoints = computed(() => {
  const series = throughputSeries.value
  if (series.length < 2) return ''
  const xs = series.map((p) => p.elapsedSec)
  const ys = series.map((p) => p.msgsPerSec)
  const xMin = xs[0]; const xMax = xs[xs.length - 1]
  const yMax = Math.max(...ys, 1)
  const innerW = CHART_W - CHART_PAD.left - CHART_PAD.right
  const innerH = CHART_H - CHART_PAD.top - CHART_PAD.bottom
  const toX = (x) => CHART_PAD.left + ((x - xMin) / (xMax - xMin || 1)) * innerW
  const toY = (y) => CHART_PAD.top + innerH - (y / yMax) * innerH
  return series.map((p, i) => `${i === 0 ? 'M' : 'L'}${toX(p.elapsedSec).toFixed(1)},${toY(p.msgsPerSec).toFixed(1)}`).join(' ')
})

const chartYMax = computed(() => {
  const series = throughputSeries.value
  if (!series.length) return 1
  return Math.max(...series.map((p) => p.msgsPerSec), 1)
})

const chartXMax = computed(() => {
  const series = throughputSeries.value
  if (!series.length) return 0
  return series[series.length - 1].elapsedSec
})

// Y-axis labels
const yTicks = computed(() => {
  const max = chartYMax.value
  const step = Math.ceil(max / 4)
  const ticks = []
  for (let v = 0; v <= max; v += step) ticks.push(v)
  return ticks
})

function fmtNs(ns) {
  if (!ns) return '—'
  const ms = ns / 1e6
  return ms < 1 ? `${(ns / 1000).toFixed(0)} µs` : `${ms.toFixed(1)} ms`
}

function fmtBytes(n) {
  if (n < 1024) return `${n} B`
  return `${(n / 1024).toFixed(1)} KB`
}
</script>

<template>
  <div class="lt-panel" role="main" aria-label="Load Test panel">

    <!-- ── 1. Header ─────────────────────────────────────────────── -->
    <div class="lt-header">
      <h2 class="lt-title">Load Test</h2>
      <p class="lt-subtitle">Generate broker traffic to measure consumer scale + drain behavior.</p>
    </div>

    <!-- ── 2. Preset picker ──────────────────────────────────────── -->
    <section class="lt-section" aria-label="Preset">
      <label class="section-label" id="preset-label">Preset</label>
      <Select
        :modelValue="selectedPresetId"
        :options="presetOptions"
        placeholder="No preset — configure manually"
        aria-labelledby="preset-label"
        testid="loadtest-preset-select"
        width="100%"
        @update:modelValue="applyPreset($event)"
      />
      <p v-if="selectedPresetId" class="preset-hint">
        {{ presets.find(p => p.id === selectedPresetId)?.description }}
      </p>
    </section>

    <!-- ── 3. Broker ─────────────────────────────────────────────── -->
    <section class="lt-section" aria-label="Broker">
      <label class="section-label" id="broker-kind-label">Broker</label>
      <Select
        :modelValue="brokerKind"
        :options="brokerKindOptions"
        placeholder="Select broker"
        aria-labelledby="broker-kind-label"
        testid="loadtest-broker-kind"
        width="100%"
        @update:modelValue="brokerKind = $event"
      />
      <div class="broker-sub-form">
        <LoadTestBrokerPubSub   v-if="brokerKind === 'pubsub'"   v-model="brokerConfig" />
        <LoadTestBrokerNATS     v-else-if="brokerKind === 'nats'"     v-model="brokerConfig" />
        <LoadTestBrokerKafka    v-else-if="brokerKind === 'kafka'"    v-model="brokerConfig" />
        <LoadTestBrokerRabbitMQ v-else-if="brokerKind === 'rabbitmq'" v-model="brokerConfig" />
        <LoadTestBrokerAMQP1    v-else-if="brokerKind === 'amqp1'"    v-model="brokerConfig" />
      </div>
    </section>

    <!-- ── 4. Destination ────────────────────────────────────────── -->
    <section class="lt-section" aria-label="Destination">
      <label for="lt-destination" class="section-label">
        {{ destinationLabel }}
        <span class="required" aria-label="required">*</span>
      </label>
      <input
        id="lt-destination"
        type="text"
        class="form-input"
        :placeholder="`Enter ${destinationLabel.toLowerCase()}`"
        v-model="destination"
        aria-required="true"
        data-testid="loadtest-destination"
      />
    </section>

    <!-- ── 5. Payload ────────────────────────────────────────────── -->
    <section class="lt-section" aria-label="Payload">
      <div class="section-label">Payload <span class="required" aria-label="required">*</span></div>
      <div class="payload-tabs" role="tablist" aria-label="Payload input method">
        <button
          v-for="tab in PAYLOAD_TABS"
          :key="tab"
          type="button"
          role="tab"
          :aria-selected="payloadTab === tab"
          :aria-controls="`payload-panel-${tab}`"
          class="payload-tab"
          :class="{ active: payloadTab === tab }"
          :data-testid="`loadtest-payload-tab-${tab}`"
          @click="payloadTab = tab"
        >
          {{ tab === 'upload' ? 'Upload file' : tab === 'paste' ? 'Paste JSON' : 'Type' }}
        </button>
      </div>

      <!-- Upload -->
      <div
        v-show="payloadTab === 'upload'"
        :id="`payload-panel-upload`"
        role="tabpanel"
        aria-labelledby="payload-tab-upload"
        class="payload-panel"
      >
        <div
          class="dropzone"
          data-testid="loadtest-payload-dropzone"
          @dragover.prevent
          @drop="onDrop"
        >
          <span v-if="uploadedFilename" class="drop-filename">
            {{ uploadedFilename }} ({{ fmtBytes(uploadedSize) }})
          </span>
          <span v-else class="drop-hint">Drop a .json file here, or</span>
          <label class="file-pick-btn">
            Browse
            <input
              type="file"
              accept=".json,application/json"
              class="sr-only"
              data-testid="loadtest-payload-file-input"
              @change="onFileChange"
            />
          </label>
        </div>
      </div>

      <!-- Paste JSON -->
      <div
        v-show="payloadTab === 'paste'"
        :id="`payload-panel-paste`"
        role="tabpanel"
        aria-labelledby="payload-tab-paste"
        class="payload-panel"
      >
        <textarea
          class="form-textarea"
          :class="{ 'input-error': pasteError }"
          rows="6"
          aria-label="Paste JSON payload"
          data-testid="loadtest-payload-paste"
          v-model="pasteJson"
          @blur="lintPaste"
        />
        <p v-if="pasteError" class="error-msg" role="alert" data-testid="loadtest-payload-paste-error">{{ pasteError }}</p>
      </div>

      <!-- Type -->
      <div
        v-show="payloadTab === 'type'"
        :id="`payload-panel-type`"
        role="tabpanel"
        aria-labelledby="payload-tab-type"
        class="payload-panel"
      >
        <textarea
          class="form-textarea"
          :class="{ 'input-error': typedError }"
          rows="6"
          aria-label="Type JSON payload"
          data-testid="loadtest-payload-type"
          v-model="typedJson"
          @blur="lintTyped"
        />
        <p v-if="typedError" class="error-msg" role="alert">{{ typedError }}</p>
      </div>
    </section>

    <!-- ── 6. Count + Workers ─────────────────────────────────────── -->
    <section class="lt-section" aria-label="Count and workers">
      <div class="row-2col">
        <div class="form-group">
          <label class="section-label" id="lt-count-label">Message Count</label>
          <Select
            :modelValue="countChoice"
            :options="COUNT_OPTIONS"
            aria-labelledby="lt-count-label"
            testid="loadtest-count-select"
            width="100%"
            @update:modelValue="countChoice = $event"
          />
          <input
            v-if="countChoice === 'custom'"
            type="number"
            class="form-input mt4"
            min="1"
            :value="countCustom"
            aria-label="Custom message count"
            data-testid="loadtest-count-custom"
            @input="countCustom = Number($event.target.value)"
          />
        </div>
        <div class="form-group">
          <label for="lt-workers" class="section-label">Workers</label>
          <input
            id="lt-workers"
            type="number"
            class="form-input"
            min="1"
            max="500"
            :value="workers"
            aria-label="Number of concurrent publisher workers (1–500)"
            aria-required="true"
            data-testid="loadtest-workers"
            @input="workers = Number($event.target.value)"
          />
        </div>
      </div>
    </section>

    <!-- ── 7. Ramp ────────────────────────────────────────────────── -->
    <section class="lt-section" aria-label="Ramp profile">
      <div class="section-label">Ramp Profile</div>
      <Select
        :modelValue="rampKind"
        :options="RAMP_KIND_OPTIONS"
        aria-label="Ramp kind"
        testid="loadtest-ramp-kind"
        width="100%"
        @update:modelValue="rampKind = $event"
      />

      <div class="ramp-fields">
        <!-- constant -->
        <template v-if="rampKind === 'constant'">
          <div class="row-2col">
            <div class="form-group">
              <label for="ramp-rate" class="form-label">Rate (msgs/sec)</label>
              <input id="ramp-rate" type="number" class="form-input" min="1" :value="rampRate" data-testid="loadtest-ramp-rate" @input="rampRate = Number($event.target.value)" />
            </div>
            <div class="form-group">
              <label for="ramp-duration" class="form-label">Max Duration (sec, 0 = unlimited)</label>
              <input id="ramp-duration" type="number" class="form-input" min="0" :value="rampDuration" data-testid="loadtest-ramp-duration" @input="rampDuration = Number($event.target.value)" />
            </div>
          </div>
        </template>

        <!-- linear -->
        <template v-else-if="rampKind === 'linear'">
          <div class="row-3col">
            <div class="form-group">
              <label for="ramp-rate-from" class="form-label">Start Rate (msgs/sec)</label>
              <input id="ramp-rate-from" type="number" class="form-input" min="1" :value="rampRate" data-testid="loadtest-ramp-rate" @input="rampRate = Number($event.target.value)" />
            </div>
            <div class="form-group">
              <label for="ramp-rate-to" class="form-label">End Rate (msgs/sec)</label>
              <input id="ramp-rate-to" type="number" class="form-input" min="1" :value="rampTo" data-testid="loadtest-ramp-to" @input="rampTo = Number($event.target.value)" />
            </div>
            <div class="form-group">
              <label for="ramp-linear-dur" class="form-label">Duration (sec)</label>
              <input id="ramp-linear-dur" type="number" class="form-input" min="1" :value="rampDuration" data-testid="loadtest-ramp-duration" @input="rampDuration = Number($event.target.value)" />
            </div>
          </div>
        </template>

        <!-- step -->
        <template v-else-if="rampKind === 'step'">
          <div class="row-2col">
            <div class="form-group">
              <label for="ramp-step-rate" class="form-label">Start Rate (msgs/sec)</label>
              <input id="ramp-step-rate" type="number" class="form-input" min="1" :value="rampRate" data-testid="loadtest-ramp-rate" @input="rampRate = Number($event.target.value)" />
            </div>
            <div class="form-group">
              <label for="ramp-step-by" class="form-label">Step By (msgs/sec)</label>
              <input id="ramp-step-by" type="number" class="form-input" min="1" :value="rampStepBy" data-testid="loadtest-ramp-step-by" @input="rampStepBy = Number($event.target.value)" />
            </div>
            <div class="form-group">
              <label for="ramp-step-every" class="form-label">Step Every (sec)</label>
              <input id="ramp-step-every" type="number" class="form-input" min="1" :value="rampStepEvery" data-testid="loadtest-ramp-step-every" @input="rampStepEvery = Number($event.target.value)" />
            </div>
            <div class="form-group">
              <label for="ramp-step-dur" class="form-label">Max Duration (sec, 0 = unlimited)</label>
              <input id="ramp-step-dur" type="number" class="form-input" min="0" :value="rampDuration" data-testid="loadtest-ramp-duration" @input="rampDuration = Number($event.target.value)" />
            </div>
          </div>
        </template>

        <!-- spike -->
        <template v-else-if="rampKind === 'spike'">
          <div class="row-3col">
            <div class="form-group">
              <label for="ramp-spike-count" class="form-label">Spike Count (bursts)</label>
              <input id="ramp-spike-count" type="number" class="form-input" min="1" :value="rampSpikeCount" data-testid="loadtest-ramp-spike-count" @input="rampSpikeCount = Number($event.target.value)" />
            </div>
            <div class="form-group">
              <label for="ramp-spike-size" class="form-label">Spike Size (msgs/burst)</label>
              <input id="ramp-spike-size" type="number" class="form-input" min="1" :value="rampSpikeSize" data-testid="loadtest-ramp-spike-size" @input="rampSpikeSize = Number($event.target.value)" />
            </div>
            <div class="form-group">
              <label for="ramp-spike-idle" class="form-label">Idle Between (sec)</label>
              <input id="ramp-spike-idle" type="number" class="form-input" min="1" :value="rampSpikeIdle" data-testid="loadtest-ramp-spike-idle" @input="rampSpikeIdle = Number($event.target.value)" />
            </div>
          </div>
        </template>
      </div>
    </section>

    <!-- ── 8. Scale plan (collapsible) ───────────────────────────── -->
    <section class="lt-section" aria-label="Scale plan">
      <button
        type="button"
        class="collapsible-header"
        :aria-expanded="scalePlanOpen"
        data-testid="loadtest-scale-toggle"
        @click="scalePlanOpen = !scalePlanOpen"
      >
        <span class="section-label" style="margin:0">Scale Plan</span>
        <span class="chev" aria-hidden="true">{{ scalePlanOpen ? '▾' : '▸' }}</span>
      </button>
      <div v-if="scalePlanOpen" class="scale-body">
        <div class="row-2col">
          <div class="form-group">
            <label for="scale-ns" class="form-label">Namespace</label>
            <input id="scale-ns" type="text" class="form-input" v-model="scaleNamespace" data-testid="loadtest-scale-namespace" />
          </div>
          <div class="form-group">
            <label for="scale-dep" class="form-label">Deployment</label>
            <input id="scale-dep" type="text" class="form-input" v-model="scaleDeployment" data-testid="loadtest-scale-deployment" />
          </div>
        </div>
        <div class="form-row--inline mb8">
          <label class="form-label toggle-label">
            <input type="checkbox" class="toggle-check" v-model="scalePreScaleToZero" aria-label="Pre-scale consumer to zero before publishing" data-testid="loadtest-scale-pre-zero" />
            Pre-scale to zero before publishing
          </label>
        </div>
        <div class="row-3col">
          <div class="form-group">
            <label for="scale-min-rep" class="form-label">Min Replicas (0 = no scale-up)</label>
            <input id="scale-min-rep" type="number" class="form-input" min="0" v-model.number="scaleMinReplicas" data-testid="loadtest-scale-min-replicas" />
          </div>
          <div class="form-group">
            <label for="scale-pre-timeout" class="form-label">Pre-scale Timeout (sec)</label>
            <input id="scale-pre-timeout" type="number" class="form-input" min="0" v-model.number="scalePreTimeout" data-testid="loadtest-scale-pre-timeout" />
          </div>
          <div class="form-group">
            <label for="scale-post-timeout" class="form-label">Post-scale Timeout (sec)</label>
            <input id="scale-post-timeout" type="number" class="form-input" min="0" v-model.number="scalePostTimeout" data-testid="loadtest-scale-post-timeout" />
          </div>
        </div>
      </div>
    </section>

    <!-- ── 9. Start / Cancel ─────────────────────────────────────── -->
    <section class="lt-section lt-actions" aria-label="Run controls">
      <ul v-if="validationErrors.length" class="validation-errors" role="list" aria-label="Validation errors">
        <li v-for="err in validationErrors" :key="err" class="val-error" role="listitem">{{ err }}</li>
      </ul>
      <p v-if="storeError" class="error-msg" role="alert">{{ storeError }}</p>
      <div class="action-row">
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!canStart || isRunning"
          :aria-disabled="!canStart || isRunning"
          data-testid="loadtest-start-btn"
          @click="onStart"
        >
          {{ loading ? 'Starting…' : 'Start load test' }}
        </button>
        <button
          v-if="isRunning"
          type="button"
          class="btn btn-danger"
          data-testid="loadtest-cancel-btn"
          @click="onCancel"
        >
          Cancel
        </button>
      </div>
    </section>

    <!-- ── 10. Live progress ─────────────────────────────────────── -->
    <section v-if="status" class="lt-section lt-progress" aria-label="Live progress" aria-live="polite">

      <!-- State badge -->
      <div class="progress-header">
        <span
          class="state-badge"
          :style="{ background: stateColor(status.state) }"
          data-testid="loadtest-state-badge"
        >{{ status.state }}</span>
        <span v-if="status.startedAt" class="state-time">
          Started {{ new Date(status.startedAt).toLocaleTimeString() }}
        </span>
      </div>

      <!-- Counters -->
      <div class="counters" aria-label="Run counters">
        <div class="counter-item">
          <span class="counter-val" data-testid="loadtest-counter-sent">{{ status.summary?.sent ?? '—' }}</span>
          <span class="counter-lbl">Sent</span>
        </div>
        <div class="counter-item">
          <span class="counter-val" data-testid="loadtest-counter-acked">{{ status.summary?.acked ?? '—' }}</span>
          <span class="counter-lbl">Acked</span>
        </div>
        <div class="counter-item">
          <span class="counter-val" data-testid="loadtest-counter-errors">{{ status.summary?.errors ?? '—' }}</span>
          <span class="counter-lbl">Errors</span>
        </div>
        <div class="counter-item">
          <span class="counter-val" data-testid="loadtest-counter-throughput">{{ status.summary?.throughputPerSec != null ? status.summary.throughputPerSec.toFixed(1) : '—' }}</span>
          <span class="counter-lbl">msg/s</span>
        </div>
        <div class="counter-item">
          <span class="counter-val">{{ fmtNs(status.summary?.p50AckLatencyNs) }}</span>
          <span class="counter-lbl">P50 ack</span>
        </div>
        <div class="counter-item">
          <span class="counter-val">{{ fmtNs(status.summary?.p95AckLatencyNs) }}</span>
          <span class="counter-lbl">P95 ack</span>
        </div>
        <div class="counter-item">
          <span class="counter-val">{{ fmtNs(status.summary?.p99AckLatencyNs) }}</span>
          <span class="counter-lbl">P99 ack</span>
        </div>
      </div>

      <!-- Throughput chart (inline SVG) -->
      <div class="chart-wrap" aria-label="Throughput over time chart" role="img">
        <svg
          v-if="throughputSeries.length >= 2"
          :viewBox="`0 0 ${CHART_W} ${CHART_H}`"
          class="chart-svg"
          aria-hidden="true"
        >
          <!-- grid lines -->
          <g class="grid-lines">
            <line
              v-for="tick in yTicks"
              :key="tick"
              :x1="CHART_PAD.left"
              :x2="CHART_W - CHART_PAD.right"
              :y1="CHART_PAD.top + (CHART_H - CHART_PAD.top - CHART_PAD.bottom) - (tick / chartYMax) * (CHART_H - CHART_PAD.top - CHART_PAD.bottom)"
              :y2="CHART_PAD.top + (CHART_H - CHART_PAD.top - CHART_PAD.bottom) - (tick / chartYMax) * (CHART_H - CHART_PAD.top - CHART_PAD.bottom)"
              stroke="#333" stroke-width="1"
            />
          </g>
          <!-- y-axis labels -->
          <g class="y-labels">
            <text
              v-for="tick in yTicks"
              :key="`yl-${tick}`"
              :x="CHART_PAD.left - 4"
              :y="CHART_PAD.top + (CHART_H - CHART_PAD.top - CHART_PAD.bottom) - (tick / chartYMax) * (CHART_H - CHART_PAD.top - CHART_PAD.bottom) + 4"
              text-anchor="end"
              font-size="10"
              fill="#8b8f96"
            >{{ tick }}</text>
          </g>
          <!-- x-axis label -->
          <text
            :x="CHART_W / 2"
            :y="CHART_H - 4"
            text-anchor="middle"
            font-size="10"
            fill="#8b8f96"
          >elapsed seconds ({{ chartXMax }}s)</text>
          <!-- line -->
          <path
            v-if="chartPoints"
            :d="chartPoints"
            fill="none"
            stroke="#4f8cff"
            stroke-width="2"
            stroke-linejoin="round"
            stroke-linecap="round"
          />
        </svg>
        <div v-else class="chart-empty">Throughput chart will appear once data arrives…</div>
      </div>

      <!-- Scale events log -->
      <div v-if="scaleLogBuffer.length" class="scale-log" aria-label="Scale events log">
        <div class="scale-log-title">Scale events</div>
        <div class="scale-log-scroll" role="log" aria-live="polite" data-testid="loadtest-scale-log">
          <div v-for="(ev, i) in scaleLogBuffer" :key="i" class="scale-log-row">
            <span class="scale-time">{{ new Date(ev.at).toLocaleTimeString() }}</span>
            <span class="scale-phase">{{ ev.phase }}</span>
            <span class="scale-replicas">spec: {{ ev.replicas }}</span>
            <span class="scale-ready">ready: {{ ev.ready }}</span>
          </div>
        </div>
      </div>

      <!-- Done: summary table + report link -->
      <div v-if="isDone && status.summary" class="done-summary" data-testid="loadtest-done-summary">
        <div class="summary-title">Summary</div>
        <table class="summary-table" aria-label="Run summary">
          <tbody>
            <tr><th>Sent</th><td>{{ status.summary.sent }}</td></tr>
            <tr><th>Acked</th><td>{{ status.summary.acked }}</td></tr>
            <tr><th>Errors</th><td>{{ status.summary.errors }}</td></tr>
            <tr><th>Throughput</th><td>{{ status.summary.throughputPerSec?.toFixed(1) }} msg/s</td></tr>
            <tr><th>P50 ack latency</th><td>{{ fmtNs(status.summary.p50AckLatencyNs) }}</td></tr>
            <tr><th>P95 ack latency</th><td>{{ fmtNs(status.summary.p95AckLatencyNs) }}</td></tr>
            <tr><th>P99 ack latency</th><td>{{ fmtNs(status.summary.p99AckLatencyNs) }}</td></tr>
            <tr><th>Max ack latency</th><td>{{ fmtNs(status.summary.maxAckLatencyNs) }}</td></tr>
          </tbody>
        </table>
        <p v-if="status.finalError" class="error-msg" role="alert">Error: {{ status.finalError }}</p>
        <p v-if="status.reportPath" class="report-path">
          Report saved:
          <code class="report-code" data-testid="loadtest-report-path">{{ status.reportPath }}</code>
        </p>
      </div>
    </section>
  </div>
</template>

<style scoped>
.lt-panel {
  display: flex;
  flex-direction: column;
  gap: 0;
  overflow-y: auto;
  padding: 16px 20px 40px;
  max-width: 780px;
  font-size: 13px;
  color: var(--text);
}

.lt-header { margin-bottom: 16px; }
.lt-title { font-size: 18px; font-weight: 600; margin: 0 0 4px; color: var(--text); }
.lt-subtitle { font-size: 12px; color: var(--text2); margin: 0; }

.lt-section {
  border-top: 1px solid var(--border);
  padding: 14px 0;
}
.section-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text2);
  margin-bottom: 8px;
  display: block;
}
.required { color: #ef4444; font-size: 11px; margin-left: 2px; }
.preset-hint { font-size: 12px; color: var(--text2); margin: 6px 0 0; font-style: italic; }
.broker-sub-form { margin-top: 12px; }

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
.input-error { border-color: #ef4444 !important; }
.form-textarea { resize: vertical; font-family: monospace; }
.mt4 { margin-top: 4px; }

.row-2col { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.row-3col { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px; }
.form-group { display: flex; flex-direction: column; gap: 4px; }
.form-label { font-size: 12px; color: var(--text2); }

/* Payload tabs */
.payload-tabs { display: flex; gap: 0; margin-bottom: 8px; border-bottom: 1px solid var(--border); }
.payload-tab {
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  padding: 6px 14px;
  font-size: 12px;
  color: var(--text2);
  cursor: pointer;
  font-family: inherit;
  transition: color 0.15s, border-color 0.15s;
}
.payload-tab:hover { color: var(--text); }
.payload-tab.active { color: var(--accent, #4f8cff); border-bottom-color: var(--accent, #4f8cff); }
.payload-tab:focus-visible { outline: 2px solid var(--accent, #4f8cff); outline-offset: 1px; }
.payload-panel { padding-top: 4px; }

/* Dropzone */
.dropzone {
  border: 2px dashed var(--border);
  border-radius: 8px;
  padding: 24px;
  text-align: center;
  color: var(--text2);
  font-size: 12px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  transition: border-color 0.15s;
}
.dropzone:hover { border-color: var(--accent, #4f8cff); }
.drop-hint { color: var(--text2); }
.drop-filename { color: var(--text); font-weight: 500; }
.file-pick-btn {
  background: var(--bg4);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 5px 14px;
  font-size: 12px;
  cursor: pointer;
  color: var(--text);
  transition: background 0.15s;
}
.file-pick-btn:hover { background: var(--bg5, var(--bg4)); }

/* Ramp fields */
.ramp-fields { margin-top: 12px; }

/* Scale plan */
.collapsible-header {
  background: none;
  border: none;
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 0;
  width: 100%;
  color: inherit;
  font-family: inherit;
}
.collapsible-header:focus-visible { outline: 2px solid var(--accent, #4f8cff); outline-offset: 2px; border-radius: 4px; }
.chev { font-size: 12px; color: var(--text2); }
.scale-body { padding-top: 12px; }
.form-row--inline { display: flex; align-items: center; gap: 6px; }
.mb8 { margin-bottom: 8px; }
.toggle-label { cursor: pointer; user-select: none; font-size: 12px; color: var(--text2); display: flex; align-items: center; gap: 6px; }
.toggle-check { accent-color: var(--accent, #4f8cff); width: 14px; height: 14px; }

/* Actions */
.lt-actions { display: flex; flex-direction: column; gap: 8px; }
.action-row { display: flex; gap: 10px; align-items: center; }
.validation-errors { padding: 0; margin: 0 0 8px; list-style: none; display: flex; flex-direction: column; gap: 4px; }
.val-error { font-size: 12px; color: #ef4444; }
.error-msg { font-size: 12px; color: #ef4444; margin: 0; }

.btn {
  padding: 8px 20px;
  border-radius: 6px;
  border: none;
  font-size: 13px;
  font-family: inherit;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.15s, opacity 0.15s;
}
.btn:focus-visible { outline: 2px solid var(--accent, #4f8cff); outline-offset: 2px; }
.btn:disabled { opacity: 0.45; cursor: not-allowed; }
.btn-primary { background: var(--accent, #4f8cff); color: #fff; }
.btn-primary:hover:not(:disabled) { filter: brightness(1.1); }
.btn-danger { background: #ef4444; color: #fff; }
.btn-danger:hover { filter: brightness(1.1); }

/* Progress section */
.lt-progress { display: flex; flex-direction: column; gap: 14px; }
.progress-header { display: flex; align-items: center; gap: 10px; }
.state-badge {
  display: inline-block;
  padding: 3px 10px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 600;
  color: #fff;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.state-time { font-size: 11px; color: var(--text2); }

.counters { display: flex; flex-wrap: wrap; gap: 12px; }
.counter-item { display: flex; flex-direction: column; align-items: center; background: var(--bg3); border: 1px solid var(--border); border-radius: 8px; padding: 8px 16px; min-width: 70px; }
.counter-val { font-size: 16px; font-weight: 600; color: var(--text); }
.counter-lbl { font-size: 10px; color: var(--text2); text-transform: uppercase; letter-spacing: 0.04em; margin-top: 2px; }

.chart-wrap { background: var(--bg3); border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.chart-svg { width: 100%; display: block; }
.chart-empty { padding: 40px 20px; text-align: center; color: var(--text2); font-size: 12px; }

.scale-log { display: flex; flex-direction: column; gap: 4px; }
.scale-log-title { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text2); }
.scale-log-scroll { max-height: 160px; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; background: var(--bg3); border: 1px solid var(--border); border-radius: 6px; padding: 6px 8px; }
.scale-log-row { display: flex; gap: 12px; font-size: 11px; font-family: monospace; color: var(--text2); }
.scale-time { color: var(--text2); }
.scale-phase { color: #4f8cff; font-weight: 500; }
.scale-replicas, .scale-ready { color: var(--text); }

.done-summary { display: flex; flex-direction: column; gap: 8px; }
.summary-title { font-size: 12px; font-weight: 600; color: var(--text); }
.summary-table { border-collapse: collapse; font-size: 12px; }
.summary-table th { text-align: left; color: var(--text2); font-weight: 400; padding: 3px 16px 3px 0; min-width: 140px; }
.summary-table td { color: var(--text); font-weight: 500; padding: 3px 0; }
.report-path { font-size: 12px; color: var(--text2); margin: 0; }
.report-code { font-family: monospace; font-size: 11px; color: var(--text); background: var(--bg3); padding: 2px 6px; border-radius: 4px; user-select: all; }

.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0,0,0,0); white-space: nowrap; border: 0; }
</style>
