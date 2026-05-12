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

  it('enqueuePrompt stores a non-empty body for later consumption', () => {
    const s = useArgusContextStore()
    s.enqueuePrompt({ body: 'diagnose finding-1', label: 'Finding 1', sourceId: 'f-1' })
    expect(s.pendingPrompt).not.toBeNull()
    expect(s.pendingPrompt.body).toBe('diagnose finding-1')
    expect(s.pendingPrompt.label).toBe('Finding 1')
    expect(s.pendingPrompt.sourceId).toBe('f-1')
    expect(typeof s.pendingPrompt.queuedAt).toBe('string')
  })

  it('enqueuePrompt ignores empty bodies', () => {
    const s = useArgusContextStore()
    s.enqueuePrompt({ body: '   ' })
    expect(s.pendingPrompt).toBeNull()
    s.enqueuePrompt({})
    expect(s.pendingPrompt).toBeNull()
  })

  it('consumePendingPrompt drains the queued prompt', () => {
    const s = useArgusContextStore()
    s.enqueuePrompt({ body: 'go', label: 'L' })
    const taken = s.consumePendingPrompt()
    expect(taken.body).toBe('go')
    expect(s.pendingPrompt).toBeNull()
    expect(s.consumePendingPrompt()).toBeNull()
  })

  it('setInvestigating / clearInvestigating drive the busy flag', () => {
    const s = useArgusContextStore()
    expect(s.investigating).toBe(false)
    s.setInvestigating('Finding 1')
    expect(s.investigating).toBe(true)
    expect(s.investigatingLabel).toBe('Finding 1')
    s.clearInvestigating()
    expect(s.investigating).toBe(false)
    expect(s.investigatingLabel).toBe('')
  })
})
