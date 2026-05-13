import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useStatusFeedStore, __test } from '../statusFeed'

describe('statusFeed store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts empty and reports no errors', () => {
    const s = useStatusFeedStore()
    expect(s.items).toEqual([])
    expect(s.latest).toBeNull()
    expect(s.hasErrors).toBe(false)
    expect(s.isPaused()).toBe(false)
  })

  it('push() normalizes severity and source to valid enums', () => {
    const s = useStatusFeedStore()
    s.push({ source: 'not-a-source', severity: 'banana', message: 'hi' })
    expect(s.items).toHaveLength(1)
    expect(s.items[0].source).toBe('system')
    expect(s.items[0].severity).toBe('info')
    expect(s.items[0].id).toBeTruthy()
    expect(s.items[0].ts).toBeTruthy()
  })

  it('push() drops empty messages', () => {
    const s = useStatusFeedStore()
    expect(s.push({ source: 'k8s', message: '' })).toBeNull()
    expect(s.push({ source: 'k8s', message: '   ' })).toBeNull()
    expect(s.items).toEqual([])
  })

  it('collapses identical (source, message) within the window', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-13T09:00:00Z'))
    const s = useStatusFeedStore()
    s.info('k8s', 'Refreshing 12 pods')
    s.info('k8s', 'Refreshing 12 pods')
    expect(s.items).toHaveLength(1)
    vi.setSystemTime(new Date('2026-05-13T09:00:02Z'))
    s.info('k8s', 'Refreshing 12 pods')
    expect(s.items).toHaveLength(2)
    vi.useRealTimers()
  })

  it('lets different sources through even with the same message', () => {
    const s = useStatusFeedStore()
    s.info('k8s', 'Connected')
    s.info('argocd', 'Connected')
    expect(s.items).toHaveLength(2)
  })

  it('enforces the ring buffer cap', () => {
    const s = useStatusFeedStore()
    for (let i = 0; i < __test.RING_CAPACITY + 25; i++) {
      // Unique messages to bypass the collapse window.
      s.info('k8s', `event ${i}`)
    }
    expect(s.items.length).toBe(__test.RING_CAPACITY)
    expect(s.items[0].message).toBe(`event ${25}`)
    expect(s.items.at(-1).message).toBe(`event ${__test.RING_CAPACITY + 24}`)
  })

  it('warn() and error() set severity', () => {
    const s = useStatusFeedStore()
    s.warn('envprobe', 'Corp proxy detected')
    s.error('agent', 'mTLS expired')
    expect(s.items[0].severity).toBe('warn')
    expect(s.items[1].severity).toBe('error')
    expect(s.hasErrors).toBe(true)
  })

  it('pauseFor() extends but does not shorten the deadline', () => {
    vi.useFakeTimers()
    vi.setSystemTime(0)
    const s = useStatusFeedStore()
    s.pauseFor(3000)
    expect(s.isPaused()).toBe(true)
    s.pauseFor(500) // shorter: must not reduce
    vi.setSystemTime(1000)
    expect(s.isPaused()).toBe(true)
    vi.setSystemTime(4000)
    expect(s.isPaused()).toBe(false)
    vi.useRealTimers()
  })

  it('resume() clears the pause', () => {
    const s = useStatusFeedStore()
    s.pauseFor(5000)
    expect(s.isPaused()).toBe(true)
    s.resume()
    expect(s.isPaused()).toBe(false)
  })

  it('scrollItems is newest-first', () => {
    const s = useStatusFeedStore()
    s.info('k8s', 'first')
    s.info('k8s', 'second')
    s.info('k8s', 'third')
    expect(s.scrollItems.map(e => e.message)).toEqual(['third', 'second', 'first'])
  })
})
