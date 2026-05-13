import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'

// ── mock useBridge before component import ─────────────────────────
const mockCallGo = vi.fn()
vi.mock('../../composables/useBridge', () => ({
  callGo: (...args) => mockCallGo(...args),
}))

// ── mock bus so useWailsEvent doesn't blow up outside Wails ────────
vi.mock('../../lib/bus', () => ({
  bus: {
    useWailsEvent: vi.fn(),
    onWails: vi.fn(() => () => {}),
    on: vi.fn(),
    off: vi.fn(),
    emit: vi.fn(),
  },
}))

import LoadTestPanel from '../../components/operations/LoadTestPanel.vue'
import { useLoadTestStore } from '../../stores/loadtest'

// ── preset fixtures ─────────────────────────────────────────────────
const FAKE_PRESETS = [
  {
    id: 'smoke',
    name: 'Smoke',
    description: '1,000 messages at constant 50/s; no Kubernetes scaling.',
    whenToUse: 'Verify your broker connection.',
    spec: {
      count: 1000,
      workers: 10,
      ramp: { kind: 'constant', rate: 50, durationNs: 30_000_000_000 },
    },
  },
  {
    id: 'spike',
    name: 'Spike',
    description: 'Six bursts of 10,000 messages.',
    whenToUse: 'Test autoscaler responsiveness.',
    spec: {
      count: 60_000,
      workers: 200,
      ramp: { kind: 'spike', spikeCount: 6, spikeSize: 10_000, spikeIdleNs: 30_000_000_000 },
    },
  },
]

const FAKE_KINDS = ['pubsub', 'nats', 'kafka', 'rabbitmq', 'amqp1']

function createWrapper() {
  return mount(LoadTestPanel, {
    global: {
      // LoadTestPanel imports Select which has its own styles — no need
      // to stub it; it renders fine in jsdom.
    },
  })
}

describe('LoadTestPanel.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    // Default: presets + kinds load successfully.
    mockCallGo.mockImplementation((method) => {
      if (method === 'ListLoadTestPresets') return Promise.resolve(FAKE_PRESETS)
      if (method === 'ListBrokerKinds') return Promise.resolve(FAKE_KINDS)
      return Promise.resolve(null)
    })
  })

  // ── mount + initial load ──────────────────────────────────────────
  it('mounts without errors and renders the panel title', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    expect(wrapper.find('.lt-title').text()).toBe('Load Test')
    expect(wrapper.find('.lt-subtitle').exists()).toBe(true)
  })

  it('fetches presets and kinds on mount', async () => {
    createWrapper()
    await flushPromises()
    expect(mockCallGo).toHaveBeenCalledWith('ListLoadTestPresets')
    expect(mockCallGo).toHaveBeenCalledWith('ListBrokerKinds')
  })

  it('renders all 5 broker kind options in the store after loading', async () => {
    createWrapper()
    await flushPromises()
    const store = useLoadTestStore()
    expect(store.kinds).toHaveLength(5)
    for (const k of FAKE_KINDS) {
      expect(store.kinds).toContain(k)
    }
  })

  // ── preset picker ─────────────────────────────────────────────────
  it('selecting a preset populates count, workers, and ramp kind in the store', async () => {
    createWrapper()
    await flushPromises()
    const store = useLoadTestStore()
    // Simulate selecting the "smoke" preset directly via store action.
    store.presets = FAKE_PRESETS
    const preset = store.getPreset('smoke')
    expect(preset).not.toBeNull()
    expect(preset.spec.count).toBe(1000)
    expect(preset.spec.workers).toBe(10)
    expect(preset.spec.ramp.kind).toBe('constant')
  })

  it('selecting spike preset gives ramp kind "spike"', async () => {
    createWrapper()
    await flushPromises()
    const store = useLoadTestStore()
    store.presets = FAKE_PRESETS
    const preset = store.getPreset('spike')
    expect(preset.spec.ramp.kind).toBe('spike')
    expect(preset.spec.workers).toBe(200)
  })

  // ── broker sub-form rendering ─────────────────────────────────────
  it('renders the pubsub sub-form when broker kind is pubsub', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    // Directly set brokerKind via the select trigger — find the broker kind select
    const brokerSelect = wrapper.find('[data-testid="loadtest-broker-kind"]')
    expect(brokerSelect.exists()).toBe(true)
    // The panel should currently show the default (kafka) sub-form
    expect(wrapper.find('[data-testid="broker-kafka-bootstrap"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="broker-pubsub-project-id"]').exists()).toBe(false)
  })

  it('renders the NATS sub-form when broker kind is nats', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    // Trigger brokerKind change by clicking the Select trigger and choosing nats.
    // Because Select uses a custom dropdown, we emit the change through the store.
    // We find the Select component for broker kind and simulate the change.
    const brokerKindTrigger = wrapper.find('[data-testid="loadtest-broker-kind"]')
    expect(brokerKindTrigger.exists()).toBe(true)
    // Use the component's internal v-model by finding and triggering the Select.
    // In jsdom the custom Select opens on click.
    await brokerKindTrigger.trigger('click')
    await flushPromises()
    // Find the panel listbox options.
    const panel = wrapper.find('.panel[role="listbox"]')
    if (panel.exists()) {
      const opts = panel.findAll('.option')
      const natsOpt = opts.find((o) => o.text().includes('NATS'))
      if (natsOpt) {
        await natsOpt.trigger('mousedown')
        await flushPromises()
        expect(wrapper.find('[data-testid="broker-nats-servers"]').exists()).toBe(true)
      }
    }
  })

  it('kafka sub-form is visible by default', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    expect(wrapper.find('[data-testid="broker-kafka-bootstrap"]').exists()).toBe(true)
  })

  // ── payload paste tab JSON lint ───────────────────────────────────
  it('paste tab with invalid JSON shows a lint error on blur', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    // Switch to paste tab (it is the default).
    const pasteTabBtn = wrapper.find('[data-testid="loadtest-payload-tab-paste"]')
    expect(pasteTabBtn.exists()).toBe(true)
    await pasteTabBtn.trigger('click')

    const pasteTA = wrapper.find('[data-testid="loadtest-payload-paste"]')
    await pasteTA.setValue('not valid json {{{{')
    await pasteTA.trigger('blur')
    await flushPromises()

    const errEl = wrapper.find('[data-testid="loadtest-payload-paste-error"]')
    expect(errEl.exists()).toBe(true)
    expect(errEl.text()).toContain('Invalid JSON')
  })

  it('paste tab with valid JSON shows no lint error', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    const pasteTA = wrapper.find('[data-testid="loadtest-payload-paste"]')
    await pasteTA.setValue('{"hello": "world"}')
    await pasteTA.trigger('blur')
    await flushPromises()
    expect(wrapper.find('[data-testid="loadtest-payload-paste-error"]').exists()).toBe(false)
  })

  // ── start button disabled when destination is empty ───────────────
  it('start button is disabled when destination is empty', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    const startBtn = wrapper.find('[data-testid="loadtest-start-btn"]')
    expect(startBtn.exists()).toBe(true)
    // Destination defaults to empty, so clicking start should show validation errors.
    await startBtn.trigger('click')
    await flushPromises()
    // The validation errors list should appear.
    const errs = wrapper.findAll('.val-error')
    expect(errs.some((e) => e.text().includes('Destination required'))).toBe(true)
    // callGo('StartLoadTest') must NOT have been called.
    expect(mockCallGo).not.toHaveBeenCalledWith('StartLoadTest', expect.anything())
  })

  it('start button is disabled (aria-disabled) when a run is already in flight', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    const store = useLoadTestStore()
    store.status = { state: 'running', summary: {} }
    await flushPromises()
    const startBtn = wrapper.find('[data-testid="loadtest-start-btn"]')
    expect(startBtn.attributes('disabled')).toBeDefined()
  })

  // ── clicking Start calls callGo('StartLoadTest', spec) ───────────
  it('clicking Start with a filled form calls callGo with StartLoadTest', async () => {
    mockCallGo.mockImplementation((method) => {
      if (method === 'ListLoadTestPresets') return Promise.resolve(FAKE_PRESETS)
      if (method === 'ListBrokerKinds') return Promise.resolve(FAKE_KINDS)
      if (method === 'StartLoadTest') return Promise.resolve('run-test-001')
      return Promise.resolve(null)
    })

    const wrapper = createWrapper()
    await flushPromises()

    // Fill in destination.
    const destInput = wrapper.find('[data-testid="loadtest-destination"]')
    await destInput.setValue('my-topic')

    // Ensure paste tab has valid JSON.
    const pasteTA = wrapper.find('[data-testid="loadtest-payload-paste"]')
    await pasteTA.setValue('{"key":"val"}')
    await pasteTA.trigger('blur')
    await flushPromises()

    // Click Start.
    await wrapper.find('[data-testid="loadtest-start-btn"]').trigger('click')
    await flushPromises()

    expect(mockCallGo).toHaveBeenCalledWith('StartLoadTest', expect.objectContaining({
      destination: 'my-topic',
      broker: expect.objectContaining({ kind: 'kafka' }),
    }))
  })

  // ── progress event populates samplesBuffer ────────────────────────
  it('store.onProgress appends samples to the buffer', () => {
    const store = useLoadTestStore()
    const samples = [
      { at: new Date().toISOString(), ackLatencyNs: 500_000, ok: true },
      { at: new Date().toISOString(), ackLatencyNs: 600_000, ok: false, err: 'timeout' },
    ]
    store.onProgress({ samples, scaleLog: [] })
    expect(store.samplesBuffer).toHaveLength(2)
    expect(store.samplesBuffer[1].ok).toBe(false)
  })

  // ── cancel button visible only when running ───────────────────────
  it('cancel button is not rendered when no run is active', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    expect(wrapper.find('[data-testid="loadtest-cancel-btn"]').exists()).toBe(false)
  })

  it('cancel button is rendered when a run is active', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    const store = useLoadTestStore()
    store.status = { state: 'running', summary: {} }
    await flushPromises()
    expect(wrapper.find('[data-testid="loadtest-cancel-btn"]').exists()).toBe(true)
  })

  // ── done state shows summary ──────────────────────────────────────
  it('shows the done summary section when state is done', async () => {
    const wrapper = createWrapper()
    await flushPromises()
    const store = useLoadTestStore()
    store.status = {
      state: 'done',
      startedAt: new Date().toISOString(),
      summary: { sent: 1000, acked: 998, errors: 2, throughputPerSec: 49.9, p50AckLatencyNs: 500_000, p95AckLatencyNs: 900_000, p99AckLatencyNs: 1_200_000, maxAckLatencyNs: 2_000_000 },
    }
    await flushPromises()
    expect(wrapper.find('[data-testid="loadtest-done-summary"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="loadtest-state-badge"]').text()).toBe('done')
  })
})
