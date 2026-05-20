import { describe, it, expect } from 'vitest'
import { parseCodeBlocks, isShellLanguage } from '../parseCodeBlocks'

describe('parseCodeBlocks', () => {
  it('returns an empty array for empty input', () => {
    expect(parseCodeBlocks('')).toEqual([])
    expect(parseCodeBlocks(null)).toEqual([])
    expect(parseCodeBlocks(undefined)).toEqual([])
  })

  it('returns a single text segment when there are no fences', () => {
    expect(parseCodeBlocks('hello world')).toEqual([
      { type: 'text', text: 'hello world' },
    ])
  })

  it('parses a single fenced block surrounded by text', () => {
    const segments = parseCodeBlocks(
      'before\n```bash\nkubectl get pods\n```\nafter',
    )
    expect(segments).toHaveLength(3)
    expect(segments[0]).toEqual({ type: 'text', text: 'before\n' })
    expect(segments[1]).toEqual({ type: 'code', language: 'bash', text: 'kubectl get pods' })
    expect(segments[2]).toEqual({ type: 'text', text: '\nafter' })
  })

  it('parses multiple fenced blocks in order', () => {
    const segments = parseCodeBlocks(
      'a\n```sh\nls\n```\nb\n```yaml\nkey: value\n```\nc',
    )
    expect(segments.map(s => s.type)).toEqual(['text', 'code', 'text', 'code', 'text'])
    expect(segments[1]).toMatchObject({ language: 'sh', text: 'ls' })
    expect(segments[3]).toMatchObject({ language: 'yaml', text: 'key: value' })
  })

  it('handles a fence with no language tag', () => {
    const segments = parseCodeBlocks('```\necho hi\n```')
    expect(segments).toEqual([
      { type: 'code', language: '', text: 'echo hi' },
    ])
  })

  it('treats an unclosed fence as code through end-of-string', () => {
    const segments = parseCodeBlocks('intro\n```bash\nkubectl get pods')
    expect(segments).toHaveLength(2)
    expect(segments[0]).toEqual({ type: 'text', text: 'intro\n' })
    expect(segments[1]).toEqual({ type: 'code', language: 'bash', text: 'kubectl get pods' })
  })

  it('lowercases the language tag', () => {
    expect(parseCodeBlocks('```Bash\nls\n```')[0]).toMatchObject({ language: 'bash' })
  })

  it('preserves leading whitespace inside the code body', () => {
    const seg = parseCodeBlocks('```\n  indented\n    deeper\n```')[0]
    expect(seg).toMatchObject({ text: '  indented\n    deeper' })
  })
})

describe('isShellLanguage', () => {
  it('treats unknown/empty languages as shell-runnable', () => {
    expect(isShellLanguage('')).toBe(true)
    expect(isShellLanguage('bash')).toBe(true)
    expect(isShellLanguage('sh')).toBe(true)
    expect(isShellLanguage('zsh')).toBe(true)
    expect(isShellLanguage('console')).toBe(true)
    expect(isShellLanguage('shell')).toBe(true)
  })

  it('rejects non-shell languages', () => {
    expect(isShellLanguage('yaml')).toBe(false)
    expect(isShellLanguage('json')).toBe(false)
    expect(isShellLanguage('python')).toBe(false)
    expect(isShellLanguage('go')).toBe(false)
    expect(isShellLanguage('javascript')).toBe(false)
  })
})
