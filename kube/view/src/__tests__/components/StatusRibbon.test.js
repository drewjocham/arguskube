import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import StatusRibbon from '../../components/status/StatusRibbon.vue'
import { useStatusFeedStore } from '../../stores/statusFeed'
import { useNotificationsStore } from '../../stores/notifications'

// jsdom doesn't ship matchMedia; the ribbon reads it for prefers-reduced
// -motion. We force a default of "no preference" so the animation loop
// runs as in a real browser.
function stubMatchMedia(matches = false) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

describe('StatusRibbon.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    stubMatchMedia(false)
    // jsdom doesn't have requestAnimationFrame natively in older
    // versions; provide a no-op so the loop doesn't throw.
    window.requestAnimationFrame = vi.fn(() => 1)
    window.cancelAnimationFrame = vi.fn()
  })

  it('renders the idle state when no events have been pushed', () => {
    const wrapper = mount(StatusRibbon)
    expect(wrapper.find('[data-testid="status-ribbon"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Argus is idle')
    // No latest event → severity class is sev-idle.
    expect(wrapper.classes()).toContain('sev-idle')
  })

  it('shows the message after a status event is pushed', async () => {
    const wrapper = mount(StatusRibbon)
    const feed = useStatusFeedStore()
    feed.info('k8s', 'Refreshing 12 pods')
    await flushPromises()
    expect(wrapper.text()).toContain('Refreshing 12 pods')
  })

  it('uses sev-warn / sev-error for warn and error events', async () => {
    const wrapper = mount(StatusRibbon)
    const feed = useStatusFeedStore()
    feed.warn('envprobe', 'Corp proxy detected')
    await flushPromises()
    expect(wrapper.classes()).toContain('sev-warn')

    feed.error('agent', 'mTLS expired')
    await flushPromises()
    expect(wrapper.classes()).toContain('sev-error')
  })

  it('pauses the scroll for 3s on mouseenter and resumes on mouseleave', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(0)
    const wrapper = mount(StatusRibbon)
    const feed = useStatusFeedStore()
    const ribbon = wrapper.find('[data-testid="status-ribbon"]')

    await ribbon.trigger('mouseenter')
    expect(feed.isPaused()).toBe(true)
    // Still paused at 2.9s
    vi.setSystemTime(2900)
    expect(feed.isPaused()).toBe(true)
    // Resumed at 3.1s
    vi.setSystemTime(3100)
    expect(feed.isPaused()).toBe(false)

    // mouseleave clears any remaining pause window immediately.
    await ribbon.trigger('mouseenter')
    vi.setSystemTime(3200)
    expect(feed.isPaused()).toBe(true)
    await ribbon.trigger('mouseleave')
    expect(feed.isPaused()).toBe(false)
    vi.useRealTimers()
  })

  it('click opens the notifications panel', async () => {
    const wrapper = mount(StatusRibbon)
    const notif = useNotificationsStore()
    expect(notif.panelOpen).toBe(false)
    await wrapper.find('[data-testid="status-ribbon"]').trigger('click')
    expect(notif.panelOpen).toBe(true)
  })

  it('Enter key opens the panel; Escape closes it', async () => {
    const wrapper = mount(StatusRibbon)
    const notif = useNotificationsStore()
    const ribbon = wrapper.find('[data-testid="status-ribbon"]')
    await ribbon.trigger('keydown', { key: 'Enter' })
    expect(notif.panelOpen).toBe(true)
    await ribbon.trigger('keydown', { key: 'Escape' })
    expect(notif.panelOpen).toBe(false)
  })

  it('sets aria-live=polite and exposes the latest event via aria-label', async () => {
    const wrapper = mount(StatusRibbon)
    const ribbon = wrapper.find('[data-testid="status-ribbon"]')
    expect(ribbon.attributes('aria-live')).toBe('polite')
    expect(ribbon.attributes('role')).toBe('status')
    expect(ribbon.attributes('aria-atomic')).toBe('false')

    const feed = useStatusFeedStore()
    feed.warn('envprobe', 'Corp proxy detected')
    await flushPromises()
    expect(ribbon.attributes('aria-label')).toContain('envprobe')
    expect(ribbon.attributes('aria-label')).toContain('Corp proxy detected')
  })

  it('mirrors ribbon events into the notifications store for scroll-back', async () => {
    mount(StatusRibbon)
    const feed = useStatusFeedStore()
    const notif = useNotificationsStore()
    feed.info('k8s', 'Refreshing 12 pods')
    await flushPromises()
    expect(notif.items.length).toBe(1)
    expect(notif.items[0].kind).toBe('status')
    expect(notif.items[0].body).toBe('Refreshing 12 pods')
    expect(notif.items[0].meta?.severity).toBe('info')
  })
})
