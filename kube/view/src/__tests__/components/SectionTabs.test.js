import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import SectionTabs from '../../components/shared/SectionTabs.vue'

const TABS = [
  { id: 'pods', label: 'Pods' },
  { id: 'deployments', label: 'Deployments' },
  { id: 'cronjobs', label: 'Cron Jobs' },
  { id: 'arguscd', label: 'ArgusCD', pro: true },
]

function mountTabs(props = {}) {
  return mount(SectionTabs, {
    props: {
      tabs: TABS,
      activeTab: 'pods',
      ...props,
    },
  })
}

describe('SectionTabs.vue', () => {
  it('renders one button per tab', () => {
    const wrapper = mountTabs()
    const buttons = wrapper.findAll('[data-testid^="section-tab-"]')
    expect(buttons).toHaveLength(TABS.length)
    expect(wrapper.text()).toContain('Pods')
    expect(wrapper.text()).toContain('Deployments')
  })

  it('marks the active tab with .active and aria-selected', () => {
    const wrapper = mountTabs({ activeTab: 'deployments' })
    const active = wrapper.find('[data-testid="section-tab-deployments"]')
    expect(active.classes()).toContain('active')
    expect(active.attributes('aria-selected')).toBe('true')
    const inactive = wrapper.find('[data-testid="section-tab-pods"]')
    expect(inactive.classes()).not.toContain('active')
    expect(inactive.attributes('aria-selected')).toBe('false')
  })

  it('emits update:active-tab when a different tab is clicked', async () => {
    const wrapper = mountTabs()
    await wrapper.find('[data-testid="section-tab-deployments"]').trigger('click')
    const emitted = wrapper.emitted('update:active-tab')
    expect(emitted).toHaveLength(1)
    expect(emitted[0]).toEqual(['deployments'])
  })

  it('does not re-emit when the active tab is re-clicked', async () => {
    const wrapper = mountTabs()
    await wrapper.find('[data-testid="section-tab-pods"]').trigger('click')
    expect(wrapper.emitted('update:active-tab')).toBeUndefined()
  })

  it('Enter / Space key navigation fires the same event', async () => {
    const wrapper = mountTabs()
    await wrapper.find('[data-testid="section-tab-cronjobs"]').trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('update:active-tab')[0]).toEqual(['cronjobs'])
    await wrapper.find('[data-testid="section-tab-deployments"]').trigger('keydown', { key: ' ' })
    expect(wrapper.emitted('update:active-tab')[1]).toEqual(['deployments'])
  })

  it('renders the PRO badge for pro tabs', () => {
    const wrapper = mountTabs()
    const arguscd = wrapper.find('[data-testid="section-tab-arguscd"]')
    expect(arguscd.text()).toContain('PRO')
  })

  it('renders badge counts when provided', () => {
    const wrapper = mountTabs({ badgeCounts: { pods: 3, deployments: 0 } })
    const pods = wrapper.find('[data-testid="section-tab-pods"]')
    expect(pods.text()).toContain('3')
    // Zero counts are omitted (no noise).
    const deps = wrapper.find('[data-testid="section-tab-deployments"]')
    expect(deps.text()).not.toContain('0')
  })

  it('uses red dot for error severity', () => {
    const wrapper = mountTabs({ badgeCounts: { pods: 2 }, badgeSeverity: 'error' })
    const dot = wrapper.find('[data-testid="section-tab-pods"] .tab-dot')
    expect(dot.attributes('style')).toContain('var(--red)')
  })

  it('renders the actions slot after the spacer', () => {
    const wrapper = mount(SectionTabs, {
      props: { tabs: TABS, activeTab: 'pods' },
      slots: { actions: '<button class="toolbar-btn">Diagnose</button>' },
    })
    expect(wrapper.find('.toolbar-btn').exists()).toBe(true)
    expect(wrapper.find('.toolbar-btn').text()).toBe('Diagnose')
  })

  it('tolerates an empty tabs array without crashing', () => {
    const wrapper = mount(SectionTabs, {
      props: { tabs: [], activeTab: '' },
    })
    expect(wrapper.findAll('[data-testid^="section-tab-"]')).toHaveLength(0)
  })

  it('role="tablist" on the container and role="tab" on each button', () => {
    const wrapper = mountTabs()
    expect(wrapper.find('[data-testid="section-tabs"]').attributes('role')).toBe('tablist')
    expect(wrapper.find('[data-testid="section-tab-pods"]').attributes('role')).toBe('tab')
  })
})
