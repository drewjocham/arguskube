import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ScenarioLiveCard from '../../components/loadtest/ScenarioLiveCard.vue'

function ep(over = {}) {
  return {
    name: '', method: 'POST', url: 'https://api/x',
    executions: 0, successes: 0, httpFails: 0, assertFails: 0,
    p50Ms: 0, p95Ms: 0, p99Ms: 0, lastFail: '', ...over,
  }
}

describe('ScenarioLiveCard.vue', () => {
  beforeEach(() => { document.body.innerHTML = ''; vi.useFakeTimers() })

  it('renders nothing when scenario has no endpoints', () => {
    const w = mount(ScenarioLiveCard, { props: { scenario: { endpoints: [] } } })
    expect(w.find('[data-testid="scenario-live-card"]').exists()).toBe(false)
  })

  it('renders a row per endpoint when status is populated', () => {
    const w = mount(ScenarioLiveCard, {
      props: { scenario: { endpoints: [ep({ url: 'https://a' }), ep({ url: 'https://b' })] } },
    })
    expect(w.find('[data-testid="scenario-live-card"]').exists()).toBe(true)
    // header row + 2 data rows
    expect(w.findAll('.row').length).toBe(3)
  })

  it('applies a pulsing class on the row when executions tick up between updates', async () => {
    const w = mount(ScenarioLiveCard, {
      props: { scenario: { endpoints: [ep({ executions: 0 })] } },
    })
    // Update to higher executions — should trigger pulsing.
    await w.setProps({ scenario: { endpoints: [ep({ executions: 5 })] } })
    const dataRow = w.findAll('.row').at(1)
    expect(dataRow.classes()).toContain('pulsing')
  })

  it('does not pulse when executions are unchanged', async () => {
    const w = mount(ScenarioLiveCard, {
      props: { scenario: { endpoints: [ep({ executions: 5 })] } },
    })
    await w.setProps({ scenario: { endpoints: [ep({ executions: 5 })] } })
    const dataRow = w.findAll('.row').at(1)
    expect(dataRow.classes()).not.toContain('pulsing')
  })
})
