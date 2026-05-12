import { describe, it, expect } from 'vitest'
import { tokenize } from '../logHighlight'

describe('tokenize', () => {
  it('returns a single segment with empty cls for null input', () => {
    const r = tokenize(null)
    expect(r).toHaveLength(1)
    expect(r[0].text).toBe('')
    expect(r[0].cls).toBe('')
  })

  it('returns a single segment with empty cls for undefined input', () => {
    const r = tokenize(undefined)
    expect(r).toHaveLength(1)
    expect(r[0].text).toBe('')
    expect(r[0].cls).toBe('')
  })

  it('returns a single segment with empty cls for non-string input', () => {
    const r = tokenize(42)
    expect(r).toHaveLength(1)
    expect(r[0].text).toBe('42')
    expect(r[0].cls).toBe('')
  })

  it('returns a single segment with empty cls for empty string', () => {
    const r = tokenize('')
    expect(r).toHaveLength(1)
    expect(r[0].text).toBe('')
    expect(r[0].cls).toBe('')
  })

  it('highlights FATAL keyword', () => {
    const r = tokenize('FATAL: out of memory')
    expect(r.find(s => s.cls === 'hl-fatal')).toBeDefined()
  })

  it('highlights PANIC keyword', () => {
    const r = tokenize('kernel PANIC')
    expect(r.find(s => s.cls === 'hl-fatal')).toBeDefined()
  })

  it('highlights ERROR keyword', () => {
    const r = tokenize('ERROR: connection refused')
    expect(r.find(s => s.cls === 'hl-error')).toBeDefined()
  })

  it('highlights ERR keyword', () => {
    const r = tokenize('ERR: timeout')
    expect(r.find(s => s.cls === 'hl-error')).toBeDefined()
  })

  it('highlights WARN keyword', () => {
    const r = tokenize('WARN: high memory usage')
    expect(r.find(s => s.cls === 'hl-warn')).toBeDefined()
  })

  it('highlights WARNING keyword', () => {
    const r = tokenize('WARNING: disk at 90%')
    expect(r.find(s => s.cls === 'hl-warn')).toBeDefined()
  })

  it('highlights INFO keyword', () => {
    const r = tokenize('INFO: pod started')
    expect(r.find(s => s.cls === 'hl-info')).toBeDefined()
  })

  it('highlights DEBUG keyword', () => {
    const r = tokenize('DEBUG: request received')
    expect(r.find(s => s.cls === 'hl-debug')).toBeDefined()
  })

  it('highlights TRACE keyword', () => {
    const r = tokenize('TRACE: enter handler')
    expect(r.find(s => s.cls === 'hl-debug')).toBeDefined()
  })

  it('highlights HTTP methods', () => {
    const r = tokenize('GET /api/v1/pods')
    expect(r.find(s => s.cls === 'hl-method' && s.text === 'GET')).toBeDefined()
  })

  it('highlights multiple HTTP methods', () => {
    const r = tokenize('GET /api -> POST /api')
    const methods = r.filter(s => s.cls === 'hl-method')
    expect(methods).toHaveLength(2)
  })

  it('highlights IP addresses', () => {
    const r = tokenize('from 192.168.1.1')
    expect(r.find(s => s.cls === 'hl-ip')).toBeDefined()
  })

  it('highlights IP with port', () => {
    const r = tokenize('connect to 10.0.0.1:8080')
    expect(r.find(s => s.cls === 'hl-ip')).toBeDefined()
  })

  it('highlights durations', () => {
    const r = tokenize('completed in 150ms')
    expect(r.find(s => s.cls === 'hl-duration')).toBeDefined()
  })

  it('highlights quoted strings', () => {
    const r = tokenize('msg="connection reset"')
    expect(r.find(s => s.cls === 'hl-string')).toBeDefined()
  })

  it('highlights key=value patterns', () => {
    const r = tokenize('level=error pod=web-1')
    expect(r.find(s => s.cls === 'hl-key')).toBeDefined()
  })

  it('deduplicates overlapping hits (IP inside quoted string wins as string)', () => {
    const r = tokenize('"request from 10.0.0.1"')
    const stringSeg = r.find(s => s.cls === 'hl-string')
    const ipSeg = r.find(s => s.cls === 'hl-ip')
    expect(stringSeg).toBeDefined()
    expect(ipSeg).toBeUndefined()
  })

  it('returns raw text as single segment when no rules match', () => {
    const r = tokenize('just some regular text without keywords')
    expect(r).toHaveLength(1)
    expect(r[0].cls).toBe('')
    expect(r[0].text).toBe('just some regular text without keywords')
  })

  it('maintains correct segment boundaries and ordering', () => {
    const r = tokenize('INFO: connect to 10.0.0.1:8080 completed in 3ms')
    const text = r.map(s => s.text).join('')
    expect(text).toBe('INFO: connect to 10.0.0.1:8080 completed in 3ms')
  })

  it('all non-text segments have a cls', () => {
    const r = tokenize('ERROR: 192.168.1.1 GET /api in 5ms "timeout" key=val')
    for (const s of r) {
      if (s.text.trim()) {
        expect(s.cls).toBeTypeOf('string')
      }
    }
  })
})
