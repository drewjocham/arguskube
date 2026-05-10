import { describe, it, expect } from 'vitest'
import { CURATED_SUGGESTIONS, buildSuggestions, fixQuerySyntax } from '../logQuery'

describe('CURATED_SUGGESTIONS', () => {
  it('exposes a non-empty list of preset queries', () => {
    expect(Array.isArray(CURATED_SUGGESTIONS)).toBe(true)
    expect(CURATED_SUGGESTIONS.length).toBeGreaterThan(0)
    for (const s of CURATED_SUGGESTIONS) {
      expect(s.label).toBeTypeOf('string')
      expect(s.query).toBeTypeOf('string')
    }
  })
})

describe('buildSuggestions', () => {
  it('returns all curated entries when query is empty', () => {
    const out = buildSuggestions('', {})
    expect(out.length).toBe(CURATED_SUGGESTIONS.length)
    for (const s of out) expect(s.kind).toBe('curated')
  })

  it('filters curated entries by label match', () => {
    const out = buildSuggestions('error', {})
    expect(out.some(s => s.label.toLowerCase().includes('error'))).toBe(true)
  })

  it('produces field-value completions when the query ends in field="partial', () => {
    const fields = {
      'kubernetes.pod_namespace': ['default', 'kube-system', 'argo'],
    }
    const out = buildSuggestions('{kubernetes.pod_namespace="kube', fields)
    const fv = out.filter(s => s.kind === 'field-value')
    expect(fv.length).toBeGreaterThan(0)
    expect(fv[0].query).toBe('{kubernetes.pod_namespace="kube-system"}')
  })

  it('returns no field-value suggestions when no fields provided', () => {
    const out = buildSuggestions('{kubernetes.pod_name="', {})
    expect(out.every(s => s.kind === 'curated')).toBe(true)
  })

  it('caps field-value completions to keep the dropdown reasonable', () => {
    const values = Array.from({ length: 200 }, (_, i) => `pod-${i}`)
    const out = buildSuggestions('{kubernetes.pod_name="', {
      'kubernetes.pod_name': values,
    })
    const fv = out.filter(s => s.kind === 'field-value')
    expect(fv.length).toBeLessThanOrEqual(26)
  })
})

describe('fixQuerySyntax', () => {
  it('reports no change for an already-valid query', () => {
    const { fixed, changed, notes } = fixQuerySyntax('{level="error"}')
    expect(fixed).toBe('{level="error"}')
    expect(changed).toBe(false)
    expect(notes).toEqual([])
  })

  it('balances a missing closing brace', () => {
    const { fixed, changed, notes } = fixQuerySyntax('{level="error"')
    expect(fixed).toBe('{level="error"}')
    expect(changed).toBe(true)
    expect(notes.some(n => /closing brace/i.test(n))).toBe(true)
  })

  it('balances a missing opening brace', () => {
    const { fixed, changed } = fixQuerySyntax('level="error"}')
    expect(fixed).toBe('{level="error"}')
    expect(changed).toBe(true)
  })

  it('adds a missing closing double-quote', () => {
    const { fixed, changed, notes } = fixQuerySyntax('{level="error}')
    expect(fixed).toContain('"error"')
    expect(changed).toBe(true)
    expect(notes.some(n => /double-quote/i.test(n))).toBe(true)
  })

  it('replaces curly quotes with straight quotes', () => {
    const { fixed, notes } = fixQuerySyntax('{level=“error”}')
    expect(fixed).toBe('{level="error"}')
    expect(notes.some(n => /curly quotes/i.test(n))).toBe(true)
  })

  it('quotes bare values inside a brace block', () => {
    const { fixed, changed, notes } = fixQuerySyntax('{level=error}')
    expect(fixed).toBe('{level="error"}')
    expect(changed).toBe(true)
    expect(notes.some(n => /bare value/i.test(n))).toBe(true)
  })

  it('quotes multiple bare values across comma-separated selectors', () => {
    const { fixed } = fixQuerySyntax('{level=error,namespace=prod}')
    expect(fixed).toBe('{level="error",namespace="prod"}')
  })

  it('returns an empty-string fix for empty input without throwing', () => {
    const { fixed, changed } = fixQuerySyntax('')
    expect(fixed).toBe('')
    expect(changed).toBe(false)
  })
})
