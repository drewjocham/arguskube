import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import MetricsRow from '../../components/center/MetricsRow.vue'

const sampleMetrics = {
  podHealthPct: 98.5,
  podsRunning: 42,
  podsTotal: 44,
  podsPending: 2,
  podsFailed: 0,
  errorRate: 0.75,
  warningEvents: 3,
  restartCount: 2,
  restartTop: 'Pod: web-5b7d',
  totalCpuMillis: 24000,
  totalMemoryBytes: 85899345920,
}

function createWrapper(props = {}) {
  return mount(MetricsRow, {
    props: {
      metrics: null,
      ...props,
    },
  })
}

describe('MetricsRow.vue — Integration', () => {
  it('shows connecting/loading state when no metrics', () => {
    const wrapper = createWrapper()
    expect(wrapper.text()).toContain('Connecting...')
    expect(wrapper.text()).toContain('Waiting for cluster data')
  })

  it('renders 4 metric cards when metrics are provided', () => {
    const wrapper = createWrapper({ metrics: sampleMetrics })
    const cards = wrapper.findAll('.metric-card')
    expect(cards.length).toBe(4)
  })

  it('renders Pod Health card with correct value', () => {
    const wrapper = createWrapper({ metrics: sampleMetrics })
    expect(wrapper.text()).toContain('Pod Health')
    expect(wrapper.text()).toContain('98.5%')
    expect(wrapper.text()).toContain('42 / 44 running')
    expect(wrapper.text()).toContain('2 pending')
  })

  it('shows pod health with up color when >= 95%', () => {
    const wrapper = createWrapper({ metrics: sampleMetrics })
    const values = wrapper.findAll('.metric-value')
    // Pod health is first card.
    expect(values[0].classes()).toContain('metric-up')
  })

  it('shows pod health with warn color when >= 80% and < 95%', () => {
    const wrapper = createWrapper({
      metrics: { ...sampleMetrics, podHealthPct: 85.0 },
    })
    const values = wrapper.findAll('.metric-value')
    expect(values[0].classes()).toContain('metric-warn')
  })

  it('shows pod health with crit color when < 80%', () => {
    const wrapper = createWrapper({
      metrics: { ...sampleMetrics, podHealthPct: 60.0 },
    })
    const values = wrapper.findAll('.metric-value')
    expect(values[0].classes()).toContain('metric-crit')
  })

  it('renders Error Rate card with correct value', () => {
    const wrapper = createWrapper({ metrics: sampleMetrics })
    expect(wrapper.text()).toContain('Error Rate')
    expect(wrapper.text()).toContain('0.75%')
    expect(wrapper.text()).toContain('3 warning events')
  })

  it('shows error rate with up color when <= 1%', () => {
    const wrapper = createWrapper({ metrics: sampleMetrics })
    const values = wrapper.findAll('.metric-value')
    expect(values[1].classes()).toContain('metric-up')
  })

  it('shows error rate with warn color when > 1% and <= 5%', () => {
    const wrapper = createWrapper({
      metrics: { ...sampleMetrics, errorRate: 2.5 },
    })
    const values = wrapper.findAll('.metric-value')
    expect(values[1].classes()).toContain('metric-warn')
  })

  it('shows error rate with crit color when > 5%', () => {
    const wrapper = createWrapper({
      metrics: { ...sampleMetrics, errorRate: 10.0 },
    })
    const values = wrapper.findAll('.metric-value')
    expect(values[1].classes()).toContain('metric-crit')
  })

  it('renders Restart Count card with correct value', () => {
    const wrapper = createWrapper({ metrics: sampleMetrics })
    expect(wrapper.text()).toContain('Restart Count')
    expect(wrapper.text()).toContain('2')
    expect(wrapper.text()).toContain('Pod: web-5b7d')
  })

  it('shows restart count with norm color when <= 5', () => {
    const wrapper = createWrapper({ metrics: sampleMetrics })
    const values = wrapper.findAll('.metric-value')
    expect(values[2].classes()).toContain('metric-norm')
  })

  it('shows restart count with warn color when > 5 and <= 20', () => {
    const wrapper = createWrapper({
      metrics: { ...sampleMetrics, restartCount: 10 },
    })
    const values = wrapper.findAll('.metric-value')
    expect(values[2].classes()).toContain('metric-warn')
  })

  it('shows restart count with crit color when > 20', () => {
    const wrapper = createWrapper({
      metrics: { ...sampleMetrics, restartCount: 25 },
    })
    const values = wrapper.findAll('.metric-value')
    expect(values[2].classes()).toContain('metric-crit')
  })

  it('renders Cluster Resources card with formatted CPU and memory', () => {
    const wrapper = createWrapper({ metrics: sampleMetrics })
    expect(wrapper.text()).toContain('Cluster Resources')
    // 24000 millis = 24 cores
    expect(wrapper.text()).toContain('24.0 cores')
    // 80 Gi (85899345920 bytes / 1073741824 = 80.0)
    expect(wrapper.text()).toContain('80.0 Gi mem requested')
  })

  it('shows "—" for MetricsRow when no metrics object', () => {
    const wrapper = createWrapper({ metrics: null })
    expect(wrapper.text()).toContain('—')
  })
})
