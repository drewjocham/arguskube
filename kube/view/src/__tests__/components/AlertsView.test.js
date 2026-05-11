import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import { nextTick } from 'vue'
import AlertsView from '../../components/alerts/AlertsView.vue'
import { useNotificationGuardStore } from '../../stores/notificationGuard'
import { useWatcherRegistryStore } from '../../stores/watcherRegistry'
import { useAppNavStore } from '../../stores/appNav'

// Mock the engine helpers — AlertsView's "Re-check all" / "Re-check"
// buttons call into the module-level helpers, which would otherwise hit
// the registry's check() functions. We stub them so we can assert
// invocation without firing real probes.
vi.mock('../../composables/useWatcherEngine', () => ({
  runDueNow: vi.fn().mockResolvedValue(undefined),
  runWatcherById: vi.fn().mockResolvedValue({ status: 'ok' }),
}))

import { runDueNow, runWatcherById } from '../../composables/useWatcherEngine'

// localStorage shim (jsdom in this repo doesn't expose functional methods)
const memory = {}
Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: (k) => (k in memory ? memory[k] : null),
    setItem: (k, v) => { memory[k] = String(v) },
    removeItem: (k) => { delete memory[k] },
    clear: () => { for (const k of Object.keys(memory)) delete memory[k] },
  },
  writable: true, configurable: true,
})

function registerSampleWatcher(reg, id = 'credential:github') {
  reg.register({
    id, label: id, kind: 'credential', intervalMs: 60_000,
    check: vi.fn().mockResolvedValue({ status: 'ok' }),
    configureAnchor: 'vault',
    enabled: true,
  })
}

describe('AlertsView.vue', () => {
  let guard, registry, appNav

  beforeEach(() => {
    setActivePinia(createPinia())
    for (const k of Object.keys(memory)) delete memory[k]
    guard = useNotificationGuardStore()
    registry = useWatcherRegistryStore()
    appNav = useAppNavStore()
    runDueNow.mockClear()
    runWatcherById.mockClear()
  })

  it('renders the three sections (silences / watchers / recent)', () => {
    const w = mount(AlertsView)
    const sections = w.findAll('.section-h')
    const titles = sections.map((s) => s.text())
    expect(titles).toContain('Active silences')
    expect(titles).toContain('Watchers')
    expect(titles).toContain('Recent alerts')
  })

  it('shows summary pills with correct counts', async () => {
    registerSampleWatcher(registry, 'a')
    registerSampleWatcher(registry, 'b')
    registry.recordResult('a', { status: 'ok' })
    registry.recordResult('b', { status: 'error' })
    guard.silence('a', 60_000, { label: 'A' })

    const w = mount(AlertsView)
    await nextTick()
    const pills = w.findAll('.summary-pill .summary-num').map((n) => n.text())
    // healthy / failing / silenced / need-ack
    expect(pills).toEqual(['1', '1', '1', '0'])
  })

  it('shows the empty-state cards when nothing is registered or silenced', () => {
    const w = mount(AlertsView)
    expect(w.text()).toMatch(/No silences/)
    expect(w.text()).toMatch(/No watchers registered yet/)
    expect(w.text()).toMatch(/No alerts yet/)
  })

  it('renders a watcher card and reflects status pill color via data attr', async () => {
    registerSampleWatcher(registry, 'credential:gh')
    registry.recordResult('credential:gh', { status: 'expired', message: 'Token died' })
    const w = mount(AlertsView)
    await nextTick()
    const card = w.find('.watcher-card')
    expect(card.exists()).toBe(true)
    expect(card.attributes('data-status')).toBe('expired')
    expect(card.text()).toContain('credential:gh')
    expect(card.text()).toContain('Token died')
  })

  it('calls runDueNow when "Re-check all watchers" is clicked', async () => {
    registerSampleWatcher(registry)
    const w = mount(AlertsView)
    await w.find('.alerts-btn.primary').trigger('click')
    await flushPromises()
    expect(runDueNow).toHaveBeenCalledWith({ force: true })
  })

  it('calls runWatcherById when the per-card Re-check is clicked', async () => {
    registerSampleWatcher(registry, 'credential:gh')
    const w = mount(AlertsView)
    await nextTick()
    const btn = w.find('.watcher-card-actions .alerts-btn')
    expect(btn.text()).toMatch(/Re-check/)
    await btn.trigger('click')
    await flushPromises()
    expect(runWatcherById).toHaveBeenCalledWith('credential:gh')
  })

  it('silence stepper +/- updates the displayed duration within bounds', async () => {
    registerSampleWatcher(registry, 'credential:gh')
    const w = mount(AlertsView)
    await nextTick()
    const stepper = w.find('.silence-stepper')
    expect(stepper.exists()).toBe(true)
    // The default value should be formatted as "1h"
    expect(stepper.find('.step-val').text()).toMatch(/h/)
    // Click + 4 times → +60min → 2h
    const plus = stepper.findAll('.step-btn').at(-1)
    await plus.trigger('click')
    await plus.trigger('click')
    await plus.trigger('click')
    await plus.trigger('click')
    expect(stepper.find('.step-val').text()).toContain('2h')
  })

  it('silence button calls guard.silence with the manual reason', async () => {
    registerSampleWatcher(registry, 'credential:gh')
    const w = mount(AlertsView)
    await nextTick()
    const buttons = w.find('.silence-stepper').findAll('.alerts-btn')
    const silenceBtn = buttons.find((b) => /Silence/i.test(b.text()))
    expect(silenceBtn).toBeTruthy()
    await silenceBtn.trigger('click')
    expect(guard.silences['credential:gh']).toBeTruthy()
    expect(guard.silences['credential:gh'].reason).toBe('manual')
  })

  it('Unsilence button removes an active silence', async () => {
    registerSampleWatcher(registry, 'credential:gh')
    guard.silence('credential:gh', 60_000, { label: 'GH' })
    const w = mount(AlertsView)
    await nextTick()
    const card = w.find('.watcher-card')
    const unBtn = card.findAll('.alerts-btn').find((b) => b.text() === 'Unsilence')
    expect(unBtn).toBeTruthy()
    await unBtn.trigger('click')
    expect(guard.silences['credential:gh']).toBeUndefined()
  })

  it('Acknowledge button on an un-acked spam silence flips acknowledged', async () => {
    // Drive a real spam silence so acknowledged starts false.
    guard.setSettings({ spamThreshold: 1, spamWindowMs: 60_000 })
    guard.observe({ source: 'gh', label: 'GH', status: 'expired' })
    guard.observe({ source: 'gh', label: 'GH', status: 'invalid' })
    expect(guard.silences.gh.acknowledged).toBe(false)
    const w = mount(AlertsView)
    await nextTick()
    const card = w.find('.silence-card.unack')
    const ackBtn = card.findAll('.alerts-btn.primary').find((b) => /Acknowledge/.test(b.text()))
    await ackBtn.trigger('click')
    expect(guard.silences.gh.acknowledged).toBe(true)
  })

  it('Configure → link jumps to Settings → watchers-notifications with returnTo Alerts', async () => {
    guard.silence('gh', 60_000, { label: 'GH' })
    const w = mount(AlertsView)
    await nextTick()
    const links = w.findAll('.alerts-btn.link')
    const cfg = links.find((b) => /Configure/.test(b.text()))
    expect(cfg).toBeTruthy()
    await cfg.trigger('click')
    expect(appNav.pending).toMatchObject({
      navId: 'settings', anchor: 'watchers-notifications',
    })
    expect(appNav.returnContext).toMatchObject({
      navId: 'alerts', anchor: 'silence:gh', label: 'Alerts',
    })
  })

  it('consumes a pending anchor on mount and scrolls/focuses the matching row', async () => {
    registerSampleWatcher(registry, 'credential:gh')
    guard.silence('credential:gh', 60_000, { label: 'GH' })
    appNav.requestNav({ navId: 'alerts', anchor: 'silence:credential:gh' })

    // Stub scrollIntoView since jsdom doesn't implement it.
    const scrollSpy = vi.fn()
    HTMLElement.prototype.scrollIntoView = scrollSpy

    const w = mount(AlertsView, { attachTo: document.body })
    // Wait for onMounted → nextTick → scrollIntoView call.
    await nextTick()
    await nextTick()
    await flushPromises()
    expect(scrollSpy).toHaveBeenCalled()

    // The silence card should be marked .focused while the anchor is hot.
    const card = w.find('.silence-card')
    expect(card.classes()).toContain('focused')
    w.unmount()
  })

  it('clears the focused outline after the 4s timeout fires', async () => {
    registerSampleWatcher(registry, 'credential:gh')
    guard.silence('credential:gh', 60_000, { label: 'GH' })
    appNav.requestNav({ navId: 'alerts', anchor: 'silence:credential:gh' })
    HTMLElement.prototype.scrollIntoView = vi.fn()

    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    const w = mount(AlertsView, { attachTo: document.body })
    await nextTick()
    await nextTick()
    await Promise.resolve() // microtask flush
    const card = w.find('.silence-card')
    expect(card.classes()).toContain('focused')
    vi.advanceTimersByTime(5000)
    await nextTick()
    expect(card.classes()).not.toContain('focused')
    vi.useRealTimers()
    w.unmount()
  })

  it('summary-pill data-tone reflects the unack state when there are pending acks', async () => {
    guard.setSettings({ spamThreshold: 1, spamWindowMs: 60_000 })
    guard.observe({ source: 'gh', label: 'GH', status: 'expired' })
    guard.observe({ source: 'gh', label: 'GH', status: 'invalid' })
    const w = mount(AlertsView)
    await nextTick()
    const pills = w.findAll('.summary-pill')
    const ackPill = pills.find((p) => p.text().includes('need ack'))
    expect(ackPill.attributes('data-tone')).toBe('unack')
  })
})
