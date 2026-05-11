import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useArgusContextStore } from '../argusContext'

describe('argusContext store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts empty', () => {
    const s = useArgusContextStore()
    expect(s.pending).toBeNull()
    expect(s.hasContext).toBe(false)
    expect(s.label).toBe('')
  })

  it('setContext fills the pending object with sane string defaults', () => {
    const s = useArgusContextStore()
    s.setContext({ kind: 'finding', label: 'NPC-001', body: 'long body' })
    expect(s.pending).not.toBeNull()
    expect(s.pending.kind).toBe('finding')
    expect(s.pending.label).toBe('NPC-001')
    expect(s.pending.body).toBe('long body')
    expect(typeof s.pending.setAt).toBe('string')
    expect(s.hasContext).toBe(true)
    expect(s.label).toBe('NPC-001')
  })

  it('setContext with non-object input clears the pending state', () => {
    const s = useArgusContextStore()
    s.setContext({ kind: 'a', label: 'b', body: 'c' })
    expect(s.hasContext).toBe(true)
    s.setContext(null)
    expect(s.hasContext).toBe(false)
  })

  it('setting a new context replaces the previous one (only one active)', () => {
    const s = useArgusContextStore()
    s.setContext({ kind: 'a', label: 'first', body: 'one' })
    s.setContext({ kind: 'b', label: 'second', body: 'two' })
    expect(s.pending.label).toBe('second')
    expect(s.pending.body).toBe('two')
  })

  it('clearContext detaches the pending state', () => {
    const s = useArgusContextStore()
    s.setContext({ kind: 'a', label: 'x', body: 'y' })
    s.clearContext()
    expect(s.pending).toBeNull()
  })

  it('consumeForSend returns the body and clears the pending state', () => {
    const s = useArgusContextStore()
    s.setContext({ kind: 'a', label: 'x', body: 'attach me' })
    expect(s.consumeForSend()).toBe('attach me')
    expect(s.pending).toBeNull()
    // A second consume returns empty string (already drained).
    expect(s.consumeForSend()).toBe('')
  })

  it('consumeForSend without a pending context returns empty string', () => {
    const s = useArgusContextStore()
    expect(s.consumeForSend()).toBe('')
  })
})
