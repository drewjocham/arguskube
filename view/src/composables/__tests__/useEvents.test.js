import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { useWailsEvent, Events } from '../useEvents'

function mountComposable(fn) {
  return mount(defineComponent({ setup() { fn(); return () => null } }), { attachTo: document.body })
}

describe('useWailsEvent', () => {
  let eventsOnMock

  beforeEach(() => {
    eventsOnMock = vi.fn(() => vi.fn())
    vi.stubGlobal('runtime', { EventsOn: eventsOnMock })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('subscribes to the given event on mount', async () => {
    const cb = vi.fn()
    mountComposable(() => useWailsEvent('test:event', cb))
    expect(eventsOnMock).toHaveBeenCalledWith('test:event', cb)
  })

  it('cancels the subscription on unmount', async () => {
    const cancelFn = vi.fn()
    const eventsOn = vi.fn(() => cancelFn)
    vi.stubGlobal('runtime', { EventsOn: eventsOn })

    const wrapper = mountComposable(() => useWailsEvent('test:event', vi.fn()))
    wrapper.unmount()
    expect(cancelFn).toHaveBeenCalled()
  })

  it('gracefully handles missing window.runtime', () => {
    vi.stubGlobal('runtime', undefined)
    expect(() => {
      mountComposable(() => useWailsEvent('test:event', vi.fn()))
    }).not.toThrow()
  })

  it('gracefully handles missing EventsOn function', () => {
    vi.stubGlobal('runtime', {})
    expect(() => {
      mountComposable(() => useWailsEvent('test:event', vi.fn()))
    }).not.toThrow()
  })

  it('invokes the callback when the event fires', async () => {
    let registeredCb
    eventsOnMock = vi.fn((_name, cb) => { registeredCb = cb; return vi.fn() })
    vi.stubGlobal('runtime', { EventsOn: eventsOnMock })

    const cb = vi.fn()
    mountComposable(() => useWailsEvent('custom:event', cb))
    registeredCb({ some: 'data' })
    expect(cb).toHaveBeenCalledWith({ some: 'data' })
  })
})

describe('Events', () => {
  it('exports expected event name constants', () => {
    expect(Events.ALERT_UPDATE).toBe('alert:update')
    expect(Events.LOG_LINE).toBe('log:line')
    expect(Events.METRICS_UPDATE).toBe('metrics:update')
    expect(Events.AUTO_SUMMARY).toBe('agent:auto-summary')
    expect(Events.AGENT_EVENT).toBe('agent:event')
    expect(Events.TERMINAL_OUTPUT).toBe('terminal:output')
    expect(Events.DEEP_LINK).toBe('deep-link')
  })
})
