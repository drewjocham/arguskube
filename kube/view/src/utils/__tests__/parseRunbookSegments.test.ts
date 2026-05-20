import { describe, it, expect } from 'vitest'
import { parseRunbookSegments, uniqueSections, __test } from '../parseRunbookSegments'

describe('parseRunbookSegments', () => {
  it('returns an empty array for empty input', () => {
    expect(parseRunbookSegments('')).toEqual([])
    expect(parseRunbookSegments(null)).toEqual([])
    expect(parseRunbookSegments(undefined)).toEqual([])
  })

  it('returns a single text segment with the Preamble section when there is no heading and no fence', () => {
    const out = parseRunbookSegments('just some text')
    expect(out).toHaveLength(1)
    expect(out[0]).toMatchObject({ type: 'text', section: 'Preamble', sectionId: 'preamble' })
  })

  it('attributes the first code block to "Preamble" when no heading precedes it', () => {
    const out = parseRunbookSegments('intro\n```bash\nls\n```\n')
    const code = out.find((s): s is import('../parseRunbookSegments').CodeSegment => s.type === 'code')
    expect(code).toBeDefined()
    expect(code!.section).toBe('Preamble')
    expect(code!.sectionId).toBe('preamble')
  })

  it('groups code blocks by their preceding heading', () => {
    const md = [
      '# Verify pods',
      'Some prose.',
      '```bash',
      'kubectl get pods',
      '```',
      '## Restart deployment',
      '```bash',
      'kubectl rollout restart deployment/api',
      '```',
    ].join('\n')

    const out = parseRunbookSegments(md)
    const codes = out.filter((s): s is import('../parseRunbookSegments').CodeSegment => s.type === 'code')
    expect(codes).toHaveLength(2)
    expect(codes[0].section).toBe('Verify pods')
    expect(codes[1].section).toBe('Restart deployment')
    expect(codes[0].sectionId).not.toBe(codes[1].sectionId)
  })

  it('two code blocks under the same heading share the same sectionId', () => {
    const md = [
      '## DNS checks',
      '```bash',
      'dig kubernetes.default',
      '```',
      'next:',
      '```bash',
      'kubectl get svc -n kube-system',
      '```',
    ].join('\n')
    const codes = parseRunbookSegments(md).filter((s): s is import('../parseRunbookSegments').CodeSegment => s.type === 'code')
    expect(codes).toHaveLength(2)
    expect(codes[0].sectionId).toBe(codes[1].sectionId)
    expect(codes[0].section).toBe('DNS checks')
  })

  it('codeIndex is doc-wide and increments per code block', () => {
    const md = '## A\n```\nls\n```\n## B\n```\nps\n```\n## C\n```\nuptime\n```'
    const codes = parseRunbookSegments(md).filter((s): s is import('../parseRunbookSegments').CodeSegment => s.type === 'code')
    expect(codes.map(c => c.codeIndex)).toEqual([0, 1, 2])
  })

  it('handles multiple headings in a single text chunk by using the LAST one', () => {
    const md = [
      '# First',
      'irrelevant intro',
      '## Final',
      'now run:',
      '```bash',
      'echo hi',
      '```',
    ].join('\n')
    const code = parseRunbookSegments(md).find((s): s is import('../parseRunbookSegments').CodeSegment => s.type === 'code')
    expect(code!.section).toBe('Final')
  })

  it('lowercases the language tag', () => {
    const code = parseRunbookSegments('```Bash\nls\n```').find((s): s is import('../parseRunbookSegments').CodeSegment => s.type === 'code')
    expect(code!.language).toBe('bash')
  })

  it('handles heading levels 1–6 the same way', () => {
    const md = '###### Tiny header\n```\necho hi\n```'
    const code = parseRunbookSegments(md).find((s): s is import('../parseRunbookSegments').CodeSegment => s.type === 'code')
    expect(code!.section).toBe('Tiny header')
  })
})

describe('uniqueSections', () => {
  it('returns ordered unique sections with id + label', () => {
    // Hand-rolled minimal segments — uniqueSections only reads
    // sectionId + section so the test casts to the public type.
    const segs = [
      { sectionId: 'a', section: 'A', type: 'text', text: '' },
      { sectionId: 'a', section: 'A', type: 'code', code: '', language: '', codeIndex: 0 },
      { sectionId: 'b', section: 'B', type: 'code', code: '', language: '', codeIndex: 1 },
      { sectionId: 'a', section: 'A', type: 'code', code: '', language: '', codeIndex: 2 },
    ] as const satisfies readonly import('../parseRunbookSegments').RunbookSegment[]
    const out = uniqueSections(segs)
    expect(out).toEqual([
      { id: 'a', label: 'A' },
      { id: 'b', label: 'B' },
    ])
  })

  it('returns [] for empty input', () => {
    expect(uniqueSections([])).toEqual([])
  })
})

describe('slugify (internal)', () => {
  it('produces a safe section-id slug', () => {
    expect(__test.slugify('Verify Pods')).toBe('verify-pods')
    expect(__test.slugify('1.2.3 Foo Bar!')).toBe('1-2-3-foo-bar')
    expect(__test.slugify('   ')).toBe('section')
    expect(__test.slugify('')).toBe('section')
  })
})
