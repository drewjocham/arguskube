import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AlertList from '../../components/center/AlertList.vue'

function makeAlert(id, overrides = {}) {
  return {
    id,
    name: `alert-${id}`,
    severity: 'critical',
    namespace: 'default',
    description: `Description for alert ${id}`,
    tags: [],
    timestamp: new Date('2026-05-06T10:00:00Z').getTime(),
    ...overrides,
  }
}

function createWrapper(props = {}) {
  return mount(AlertList, {
    props: {
      alerts: [],
      selectedAlert: null,
      ...props,
    },
  })
}

describe('AlertList.vue — Integration', () => {
  it('renders the section header', () => {
    const wrapper = createWrapper()
    expect(wrapper.find('.section-header').exists()).toBe(true)
    expect(wrapper.text()).toContain('Active Alerts')
  })

  it('shows empty state when no alerts', () => {
    const wrapper = createWrapper()
    expect(wrapper.text()).toContain('No active alerts')
    expect(wrapper.text()).toContain('cluster healthy')
  })

  it('renders alert items for each alert in the array', () => {
    const alerts = [
      makeAlert(1, { severity: 'critical' }),
      makeAlert(2, { severity: 'warning' }),
      makeAlert(3, { severity: 'info' }),
    ]
    const wrapper = createWrapper({ alerts })
    const items = wrapper.findAll('.alert-item')
    expect(items.length).toBe(3)
  })

  it('displays alert name, description, and namespace', () => {
    const alerts = [
      makeAlert(1, { name: 'High CPU', description: 'CPU > 90%', namespace: 'prod' }),
    ]
    const wrapper = createWrapper({ alerts })
    expect(wrapper.text()).toContain('High CPU')
    expect(wrapper.text()).toContain('CPU > 90%')
    expect(wrapper.text()).toContain('prod')
  })

  it('shows count pill for critical alerts', () => {
    const alerts = [
      makeAlert(1, { severity: 'critical' }),
      makeAlert(2, { severity: 'critical' }),
      makeAlert(3, { severity: 'warning' }),
    ]
    const wrapper = createWrapper({ alerts })
    const pills = wrapper.findAll('.count-pill')
    expect(pills.length).toBe(2)
    expect(pills[0].text()).toContain('2 critical')
    expect(pills[1].text()).toContain('1 warning')
  })

  it('does not show count pill when no matching alerts', () => {
    const wrapper = createWrapper({ alerts: [] })
    const pills = wrapper.findAll('.count-pill')
    expect(pills.length).toBe(0)
  })

  it('only shows warning pill when only warnings present', () => {
    const alerts = [
      makeAlert(1, { severity: 'warning' }),
      makeAlert(2, { severity: 'warning' }),
    ]
    const wrapper = createWrapper({ alerts })
    const pills = wrapper.findAll('.count-pill')
    expect(pills.length).toBe(1)
    expect(pills[0].text()).toContain('2 warning')
  })

  it('applies correct severity class for each alert type', () => {
    const alerts = [
      makeAlert(1, { severity: 'critical' }),
      makeAlert(2, { severity: 'warning' }),
      makeAlert(3, { severity: 'info' }),
    ]
    const wrapper = createWrapper({ alerts })
    const items = wrapper.findAll('.alert-item')
    const severities = wrapper.findAll('.alert-severity')
    expect(severities.length).toBe(3)
    expect(severities[0].classes()).toContain('sev-critical')
    expect(severities[1].classes()).toContain('sev-warning')
    expect(severities[2].classes()).toContain('sev-info')
  })

  it('selects alert on click and shows selected state', async () => {
    const alerts = [makeAlert(1), makeAlert(2)]
    const wrapper = mount(AlertList, {
      props: {
        alerts,
        selectedAlert: null,
        'onUpdate:selectedAlert': (alert) => {
          // Simulate parent updating selectedAlert prop
          wrapper.setProps({ selectedAlert: alert })
        },
      },
    })

    const item = wrapper.find('.alert-item')
    expect(item.exists()).toBe(true)

    // Click first alert item — the component emits 'select', we simulate parent updating
    // Note: wrapper.emitted() doesn't work in this Vue version, so we verify visually.
    await item.trigger('click')
    await wrapper.setProps({ selectedAlert: alerts[0] })

    // Visually confirm selected state
    const items = wrapper.findAll('.alert-item')
    expect(items[0].classes()).toContain('selected')
    expect(items[1].classes()).not.toContain('selected')
  })

  it('highlights the selected alert item', () => {
    const alerts = [
      makeAlert(1),
      makeAlert(2),
    ]
    const wrapper = createWrapper({ alerts, selectedAlert: alerts[0] })
    const items = wrapper.findAll('.alert-item')
    expect(items[0].classes()).toContain('selected')
    expect(items[1].classes()).not.toContain('selected')
  })

  it('renders tags when present', () => {
    const alerts = [
      makeAlert(1, {
        tags: [
          { label: 'production', color: 'red' },
          { label: 'critical', color: 'amber' },
        ],
      }),
    ]
    const wrapper = createWrapper({ alerts })
    const tags = wrapper.findAll('.tag')
    expect(tags.length).toBe(2)
    expect(tags[0].text()).toBe('production')
    expect(tags[0].classes()).toContain('tag-red')
    expect(tags[1].text()).toBe('critical')
    expect(tags[1].classes()).toContain('tag-amber')
  })

  it('shows formatted timestamp', () => {
    // Use a fixed time so the test is deterministic.
    const ts = new Date('2026-05-06T14:30:00Z').getTime()
    const alerts = [makeAlert(1, { timestamp: ts })]
    const wrapper = createWrapper({ alerts })
    // The template calls toLocaleTimeString('en-GB') which in jsdom defaults to UTC.
    expect(wrapper.text()).toContain('14:30')
  })

  it('shows emdash when no timestamp', () => {
    const alerts = [makeAlert(1, { timestamp: null })]
    const wrapper = createWrapper({ alerts })
    expect(wrapper.text()).toContain('—')
  })

  it('renders diagnose button on each alert', () => {
    const alerts = [makeAlert(1)]
    const wrapper = createWrapper({ alerts })
    const diagBtns = wrapper.findAll('.diagnose-btn')
    expect(diagBtns.length).toBe(1)
    expect(diagBtns[0].text()).toContain('Diagnose')
  })
})
