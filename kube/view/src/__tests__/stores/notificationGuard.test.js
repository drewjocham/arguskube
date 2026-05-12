import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useNotificationGuardStore } from '../../stores/notificationGuard'
import { useNotificationsStore } from '../../stores/notifications'
import { bus } from '../../lib/bus'

// All-in-one tests for notificationGuard — the central traffic cop in
// front of every watcher firing. Covers settings persistence, dedupe,
// rolling-window spam detection, manual silence, recovery flow, and the
// snapshotForArgus surface used by the AI chat tool.

// The jsdom localStorage in this project ships as a bare object without
// getItem/setItem methods, so we patch it with a working in-memory mock —
// the same pattern the existing notifications store tests use.
const GUARD_KEY = 'kw-notification-guard/v1'
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

describe('notificationGuard store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    for (const k of Object.keys(memory)) delete memory[k]
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  // ---- Settings ----------------------------------------------------------

  it('loads default settings when localStorage is empty', () => {
    const g = useNotificationGuardStore()
    expect(g.settings.spamThreshold).toBe(5)
    expect(g.settings.spamWindowMs).toBe(5 * 60 * 1000)
    expect(g.settings.defaultSilenceMs).toBe(60 * 60 * 1000)
    expect(g.settings.enabled).toBe(true)
  })

  it('setSettings patches reactive state and persists to localStorage', () => {
    const g = useNotificationGuardStore()
    g.setSettings({ spamThreshold: 9 })
    expect(g.settings.spamThreshold).toBe(9)
    // Verify the persistence side effect by reading the raw key.
    const raw = localStorage.getItem(GUARD_KEY)
    expect(raw).toBeTruthy()
    const parsed = JSON.parse(raw)
    expect(parsed.spamThreshold).toBe(9)
  })

  it('clamps defaultSilenceMs to a 24h hard cap', () => {
    const g = useNotificationGuardStore()
    g.setSettings({ defaultSilenceMs: 999 * 60 * 60 * 1000 }) // ~6 weeks
    expect(g.settings.defaultSilenceMs).toBe(24 * 60 * 60 * 1000)
  })

  // ---- observe() core flow ----------------------------------------------

  it('observe() with disabled guard returns false without side effects', () => {
    const g = useNotificationGuardStore()
    g.setSettings({ enabled: false })
    const ret = g.observe({ source: 'x', label: 'X', status: 'expired' })
    expect(ret).toBe(false)
    expect(Object.keys(g.sources)).toHaveLength(0)
  })

  it('observe() emits and tracks an alert on first fire', () => {
    const g = useNotificationGuardStore()
    const notif = useNotificationsStore()
    notif.clearAll()
    const emitSpy = vi.spyOn(bus, 'emit')
    const ret = g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    expect(ret).toBe(true)
    expect(g.sources.gh.lastStatus).toBe('expired')
    expect(emitSpy).toHaveBeenCalledWith('argus:save', expect.objectContaining({
      method: expect.stringMatching(/WatcherAlert:gh/),
      status: 'error',
    }))
    expect(notif.items.length).toBeGreaterThan(0)
    emitSpy.mockRestore()
  })

  it('observe() dedupes consecutive identical fires', () => {
    const g = useNotificationGuardStore()
    g.observe({ source: 'gh', label: 'GitHub', status: 'expired' }) // emits
    const ret = g.observe({ source: 'gh', label: 'GitHub', status: 'expired' }) // dedupe
    expect(ret).toBe(false)
  })

  it('observe() does not dedupe across recovery → fail transitions', () => {
    const g = useNotificationGuardStore()
    g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    g.observe({ source: 'gh', label: 'GitHub', status: 'ok' })       // recovery
    const ret = g.observe({ source: 'gh', label: 'GitHub', status: 'expired' }) // new fire
    expect(ret).toBe(true)
  })

  it('recovery flow clears any active silence and emits an info notification', () => {
    const g = useNotificationGuardStore()
    // Drive a real spam-silence path so prev.lastStatus is set to a bad
    // value. The recovery transition then has the prev state it needs to
    // recognise the flip.
    g.setSettings({ spamThreshold: 2, spamWindowMs: 60_000 })
    g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    g.observe({ source: 'gh', label: 'GitHub', status: 'invalid' })
    g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    expect(g.silences.gh).toBeTruthy()
    const notif = useNotificationsStore()
    notif.clearAll()
    g.observe({ source: 'gh', label: 'GitHub', status: 'ok' }) // recovery
    expect(g.silences.gh).toBeUndefined()
    const recovery = notif.items.find((n) => n.title.includes('recovered'))
    expect(recovery).toBeTruthy()
  })

  // ---- Spam detection ----------------------------------------------------

  it('observe() flips into spam silence after >threshold fires inside window', () => {
    const g = useNotificationGuardStore()
    g.setSettings({ spamThreshold: 3, spamWindowMs: 60_000 })
    const sources = ['gh', 'gh', 'gh', 'gh']
    // Different statuses to bypass the dedupe path so each fire is counted.
    const statuses = ['expired', 'invalid', 'expired', 'invalid']
    for (let i = 0; i < sources.length; i++) {
      g.observe({ source: sources[i], label: 'GitHub', status: statuses[i] })
    }
    // 4 transitions > threshold of 3 → spam silence on the 4th.
    expect(g.silences.gh).toBeTruthy()
    expect(g.silences.gh.reason).toBe('spam')
    expect(g.silences.gh.acknowledged).toBe(false)
  })

  it('spam silence emits a watcher:silenced bus event', () => {
    const g = useNotificationGuardStore()
    g.setSettings({ spamThreshold: 2, spamWindowMs: 60_000 })
    const emitSpy = vi.spyOn(bus, 'emit')
    g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    g.observe({ source: 'gh', label: 'GitHub', status: 'invalid' })
    g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    expect(emitSpy).toHaveBeenCalledWith('watcher:silenced', expect.objectContaining({
      source: 'gh',
      acknowledgeable: true,
    }))
    emitSpy.mockRestore()
  })

  it('respects rolling window — old fires age out and stop counting', () => {
    const g = useNotificationGuardStore()
    g.setSettings({ spamThreshold: 3, spamWindowMs: 60_000 })
    g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    g.observe({ source: 'gh', label: 'GitHub', status: 'invalid' })
    // Advance past the window.
    vi.setSystemTime(Date.now() + 70_000)
    g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    g.observe({ source: 'gh', label: 'GitHub', status: 'invalid' })
    // No spam silence — the older two fires aged out.
    expect(g.silences.gh).toBeUndefined()
  })

  it('observe() during active silence increments pendingCount and suppresses fires', () => {
    const g = useNotificationGuardStore()
    g.silence('gh', 60_000, { label: 'GitHub' })
    expect(g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })).toBe(false)
    expect(g.observe({ source: 'gh', label: 'GitHub', status: 'invalid' })).toBe(false)
    expect(g.silences.gh.pendingCount).toBe(2)
  })

  it('observe() lifts an expired silence and re-routes the alert as a recovery', () => {
    const g = useNotificationGuardStore()
    // Use the manual silence path; clampSilence floors at 60s so we silence
    // for 60s then advance time past it.
    g.silence('gh', 60_000, { label: 'GitHub' })
    vi.setSystemTime(Date.now() + 70_000)
    const notif = useNotificationsStore()
    notif.clearAll()
    const ret = g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    // After silence expiry, observe() fires the recovery + re-evaluates;
    // the silence is gone afterwards.
    expect(g.silences.gh).toBeUndefined()
    // The recovery info notification is emitted on lifting silence.
    expect(notif.items.some((n) => /Auto-resumed/.test(n.body))).toBe(true)
    expect(ret).toBe(true)
  })

  // ---- Manual silence / unsilence / acknowledge -------------------------

  it('silence() clamps duration to [60s, 24h]', () => {
    const g = useNotificationGuardStore()
    g.silence('a', 100, { label: 'A' })            // < 60s, clamps up
    g.silence('b', 999 * 24 * 3600 * 1000, { label: 'B' }) // > 24h, clamps down
    const aMs = g.silences.a.until - Date.now()
    const bMs = g.silences.b.until - Date.now()
    expect(aMs).toBeGreaterThanOrEqual(60_000)
    expect(bMs).toBeLessThanOrEqual(24 * 60 * 60 * 1000 + 1000)
  })

  it('silence() defaults reason to "manual" and pre-acknowledges', () => {
    const g = useNotificationGuardStore()
    g.silence('a', 60_000, { label: 'A' })
    expect(g.silences.a.reason).toBe('manual')
    expect(g.silences.a.acknowledged).toBe(true)
  })

  it('silence() respects custom reason for argus-driven silences', () => {
    const g = useNotificationGuardStore()
    g.silence('a', 60_000, { label: 'A', reason: 'argus' })
    expect(g.silences.a.reason).toBe('argus')
  })

  it('unsilence() removes the entry; no-op if missing', () => {
    const g = useNotificationGuardStore()
    g.silence('a', 60_000, { label: 'A' })
    g.unsilence('a')
    expect(g.silences.a).toBeUndefined()
    g.unsilence('does-not-exist')          // no throw
  })

  it('acknowledge() flips a spam silence to acknowledged', () => {
    const g = useNotificationGuardStore()
    g.setSettings({ spamThreshold: 1, spamWindowMs: 60_000 })
    g.observe({ source: 'gh', label: 'GitHub', status: 'expired' })
    g.observe({ source: 'gh', label: 'GitHub', status: 'invalid' })
    expect(g.silences.gh.acknowledged).toBe(false)
    g.acknowledge('gh')
    expect(g.silences.gh.acknowledged).toBe(true)
  })

  it('acknowledge() is a no-op for unknown sources', () => {
    const g = useNotificationGuardStore()
    g.acknowledge('nope')
    expect(g.silences.nope).toBeUndefined()
  })

  // ---- Computeds + snapshot ---------------------------------------------

  it('activeSilences sorts by until ascending', () => {
    const g = useNotificationGuardStore()
    g.silence('a', 60_000 * 30, { label: 'A' })
    g.silence('b', 60_000, { label: 'B' })
    const list = g.activeSilences
    expect(list[0].source).toBe('b')
    expect(list[1].source).toBe('a')
  })

  it('pendingAcks only includes un-acknowledged silences', () => {
    const g = useNotificationGuardStore()
    g.setSettings({ spamThreshold: 1, spamWindowMs: 60_000 })
    g.silence('manual', 60_000, { label: 'M' })  // acknowledged
    g.observe({ source: 'spammy', label: 'S', status: 'error' })
    g.observe({ source: 'spammy', label: 'S', status: 'warn' })  // spam
    expect(g.silences.spammy.acknowledged).toBe(false)
    const acks = g.pendingAcks.map((s) => s.source)
    expect(acks).toContain('spammy')
    expect(acks).not.toContain('manual')
  })

  it('snapshotForArgus exposes a stable JSON shape', () => {
    const g = useNotificationGuardStore()
    g.silence('a', 60_000, { label: 'A' })
    const snap = g.snapshotForArgus()
    expect(snap).toHaveProperty('generatedAt')
    expect(snap).toHaveProperty('settings.spamThreshold')
    expect(Array.isArray(snap.silences)).toBe(true)
    expect(snap.silences[0]).toHaveProperty('source', 'a')
    expect(snap.silences[0]).toHaveProperty('until')
    expect(snap.silences[0]).toHaveProperty('reason', 'manual')
  })

  it('observe() with no source or null event returns false', () => {
    const g = useNotificationGuardStore()
    expect(g.observe(null)).toBe(false)
    expect(g.observe({})).toBe(false)
    expect(g.observe({ source: '', status: 'error' })).toBe(false)
  })

  it('healthy-status fires update lastStatus but emit nothing', () => {
    const g = useNotificationGuardStore()
    const notif = useNotificationsStore()
    notif.clearAll()
    const ret = g.observe({ source: 'gh', label: 'GitHub', status: 'ok' })
    expect(ret).toBe(false)
    expect(g.sources.gh.lastStatus).toBe('ok')
    expect(notif.items.length).toBe(0)
  })

  it('observe() honours custom recoveryStatuses for non-ok healthy values', () => {
    const g = useNotificationGuardStore()
    g.observe({ source: 'creds', label: 'Creds', status: 'expired' })
    const ret = g.observe({
      source: 'creds', label: 'Creds', status: 'valid',
      recoveryStatuses: ['valid', 'present'],
    })
    expect(ret).toBe(true) // recovery counts as emitted
    expect(g.sources.creds.lastStatus).toBe('valid')
  })
})
