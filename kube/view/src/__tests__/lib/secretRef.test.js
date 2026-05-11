import { describe, it, expect } from 'vitest'
import {
  parseSecretRef,
  formatSecretRef,
  isResolvable,
  describeSecretRef,
  secretRefIsValid,
  SECRET_REF_KINDS,
  SECRET_REF_META,
} from '../../lib/secretRef'

describe('secretRef.parseSecretRef', () => {
  it('returns an empty inline ref for null / undefined / empty input', () => {
    for (const v of [null, undefined, '']) {
      expect(parseSecretRef(v)).toEqual({ kind: 'inline', value: '', key: '', raw: '' })
    }
  })

  it('treats a value without a colon as inline', () => {
    expect(parseSecretRef('hunter2')).toEqual({
      kind: 'inline', value: 'hunter2', key: '', raw: 'hunter2',
    })
  })

  it('parses each known kind correctly', () => {
    const cases = [
      ['env:DATABASE_URL',         { kind: 'env',         value: 'DATABASE_URL', key: '' }],
      ['file:/etc/foo/secret',     { kind: 'file',        value: '/etc/foo/secret', key: '' }],
      ['volume:my-vol/secret-key', { kind: 'volume',      value: 'my-vol/secret-key', key: '' }],
      ['aws-secret:prod/db',       { kind: 'aws-secret',  value: 'prod/db', key: '' }],
      ['vault:github-pat',         { kind: 'vault',       value: 'github-pat', key: '' }],
    ]
    for (const [input, want] of cases) {
      const got = parseSecretRef(input)
      expect(got.kind).toBe(want.kind)
      expect(got.value).toBe(want.value)
      expect(got.key).toBe(want.key)
    }
  })

  it('parses the optional #key suffix for kinds that support it', () => {
    expect(parseSecretRef('aws-secret:prod/db#username')).toEqual({
      kind: 'aws-secret', value: 'prod/db', key: 'username', raw: 'aws-secret:prod/db#username',
    })
    expect(parseSecretRef('vault:gh-pat#token')).toEqual({
      kind: 'vault', value: 'gh-pat', key: 'token', raw: 'vault:gh-pat#token',
    })
  })

  it('uses the LAST # so values containing # work for the leading segment', () => {
    // a contrived case but the parser must be deterministic
    const got = parseSecretRef('aws-secret:weird#name#real-key')
    expect(got.value).toBe('weird#name')
    expect(got.key).toBe('real-key')
  })

  it('falls back to inline for unknown prefixes, preserving the colon', () => {
    const got = parseSecretRef('mysql://user:pass@host')
    expect(got.kind).toBe('inline')
    expect(got.value).toBe('mysql://user:pass@host')
  })

  it('strips a single space after the kind separator for natural typing', () => {
    expect(parseSecretRef('vault: gh-pat').value).toBe('gh-pat')
    // but NOT additional spaces
    expect(parseSecretRef('vault:  gh-pat').value).toBe(' gh-pat')
  })

  it('strips an explicit "inline:" prefix when present', () => {
    const got = parseSecretRef('inline:literal value')
    expect(got.kind).toBe('inline')
    expect(got.value).toBe('literal value')
  })

  it('lowercases the kind so case-insensitive prefixes work', () => {
    expect(parseSecretRef('ENV:HOST').kind).toBe('env')
    expect(parseSecretRef('AWS-SECRET:foo').kind).toBe('aws-secret')
  })

  it('coerces non-string input to string', () => {
    expect(parseSecretRef(42)).toEqual({ kind: 'inline', value: '42', key: '', raw: '42' })
  })
})

describe('secretRef.formatSecretRef', () => {
  it('returns empty string for falsy input', () => {
    expect(formatSecretRef(null)).toBe('')
    expect(formatSecretRef(undefined)).toBe('')
  })

  it('returns the bare value for inline (no prefix)', () => {
    expect(formatSecretRef({ kind: 'inline', value: 'foo' })).toBe('foo')
  })

  it('writes "kind:value" without a hash when key is empty', () => {
    expect(formatSecretRef({ kind: 'env', value: 'HOST', key: '' })).toBe('env:HOST')
  })

  it('appends "#key" for kinds with a structured key', () => {
    expect(formatSecretRef({ kind: 'aws-secret', value: 'prod/db', key: 'user' }))
      .toBe('aws-secret:prod/db#user')
  })

  it('round-trips with parseSecretRef', () => {
    const inputs = [
      'plain',
      'env:HOST',
      'file:/etc/secret',
      'volume:my-vol/secret-key',
      'aws-secret:prod/db#user',
      'vault:gh',
    ]
    for (const i of inputs) {
      expect(formatSecretRef(parseSecretRef(i))).toBe(i)
    }
  })

  it('treats unknown kind as inline (no prefix)', () => {
    expect(formatSecretRef({ kind: 'bogus', value: 'foo' })).toBe('foo')
  })
})

describe('secretRef.isResolvable', () => {
  it('returns false for inline and falsy refs', () => {
    expect(isResolvable(null)).toBe(false)
    expect(isResolvable({ kind: 'inline', value: 'x' })).toBe(false)
    expect(isResolvable({ kind: '', value: 'x' })).toBe(false)
  })

  it('returns true for any non-inline known kind', () => {
    for (const k of SECRET_REF_KINDS) {
      if (k === 'inline') continue
      expect(isResolvable({ kind: k, value: 'x' })).toBe(true)
    }
  })
})

describe('secretRef.describeSecretRef', () => {
  it('formats inline values', () => {
    expect(describeSecretRef({ kind: 'inline', value: 'x' })).toBe('Inline value')
    expect(describeSecretRef({ kind: 'inline', value: '' })).toBe('Empty inline value')
  })

  it('formats sourced refs with their meta label', () => {
    expect(describeSecretRef({ kind: 'aws-secret', value: 'prod/db', key: 'user' }))
      .toBe('AWS Secrets Mgr · prod/db (user)')
    expect(describeSecretRef({ kind: 'env', value: 'HOST' }))
      .toBe('Env var · HOST')
  })

  it('falls back to the kind label when value is empty', () => {
    expect(describeSecretRef({ kind: 'vault', value: '' })).toBe('Argus Vault')
  })
})

describe('secretRef.secretRefIsValid', () => {
  it('allows empty inline (nullable env vars)', () => {
    expect(secretRefIsValid({ kind: 'inline', value: '' })).toBe(true)
  })

  it('requires uppercase identifier-style env names', () => {
    expect(secretRefIsValid({ kind: 'env', value: 'HOST' })).toBe(true)
    expect(secretRefIsValid({ kind: 'env', value: 'A_b1' })).toBe(true)
    expect(secretRefIsValid({ kind: 'env', value: '' })).toBe(false)
    expect(secretRefIsValid({ kind: 'env', value: '1HOST' })).toBe(false)
    expect(secretRefIsValid({ kind: 'env', value: 'HOST WITH SPACE' })).toBe(false)
  })

  it('requires absolute path for file', () => {
    expect(secretRefIsValid({ kind: 'file', value: '/etc/x' })).toBe(true)
    expect(secretRefIsValid({ kind: 'file', value: 'relative' })).toBe(false)
  })

  it('requires <name>/<path> for volume', () => {
    expect(secretRefIsValid({ kind: 'volume', value: 'my-vol/secret' })).toBe(true)
    expect(secretRefIsValid({ kind: 'volume', value: 'just-a-name' })).toBe(false)
  })

  it('requires non-empty value for cloud vault refs', () => {
    for (const k of ['aws-secret', 'gcp-secret', 'azure-vault', 'vault']) {
      expect(secretRefIsValid({ kind: k, value: 'foo' })).toBe(true)
      expect(secretRefIsValid({ kind: k, value: '' })).toBe(false)
    }
  })

  it('rejects unknown kinds', () => {
    expect(secretRefIsValid({ kind: 'mystery', value: 'foo' })).toBe(false)
  })
})

describe('secretRef.SECRET_REF_KINDS / META', () => {
  it('exposes a frozen list of kinds', () => {
    expect(Array.isArray(SECRET_REF_KINDS)).toBe(true)
    expect(SECRET_REF_KINDS).toContain('inline')
    expect(SECRET_REF_KINDS).toContain('aws-secret')
    expect(Object.isFrozen(SECRET_REF_KINDS)).toBe(true)
  })

  it('has meta entries for every kind', () => {
    for (const k of SECRET_REF_KINDS) {
      expect(SECRET_REF_META[k]).toBeTruthy()
      expect(typeof SECRET_REF_META[k].label).toBe('string')
      expect(typeof SECRET_REF_META[k].hint).toBe('string')
    }
  })
})
