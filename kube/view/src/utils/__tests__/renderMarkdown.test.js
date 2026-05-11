import { describe, it, expect } from 'vitest'
import { renderMarkdown } from '../renderMarkdown'

describe('renderMarkdown', () => {
  it('returns empty for nullish / blank input', () => {
    expect(renderMarkdown(null)).toBe('')
    expect(renderMarkdown(undefined)).toBe('')
    expect(renderMarkdown('')).toBe('')
    expect(renderMarkdown('   \n\t  ')).toBe('')
  })

  it('renders a plain paragraph', () => {
    const html = renderMarkdown('hello world')
    expect(html).toContain('<p>hello world</p>')
  })

  it('renders bold and italics', () => {
    const html = renderMarkdown('this is **bold** and *italic*')
    expect(html).toMatch(/<strong>bold<\/strong>/)
    expect(html).toMatch(/<em>italic<\/em>/)
  })

  it('renders headers', () => {
    expect(renderMarkdown('# Title')).toContain('<h1>Title</h1>')
    expect(renderMarkdown('## Subtitle')).toContain('<h2>Subtitle</h2>')
    expect(renderMarkdown('### Section')).toContain('<h3>Section</h3>')
  })

  it('renders unordered and ordered lists', () => {
    const ul = renderMarkdown('- a\n- b\n- c')
    expect(ul).toContain('<ul>')
    expect(ul).toMatch(/<li>a<\/li>/)
    const ol = renderMarkdown('1. first\n2. second')
    expect(ol).toContain('<ol>')
    expect(ol).toMatch(/<li>first<\/li>/)
  })

  it('renders inline code with `<code>`', () => {
    const html = renderMarkdown('use `kubectl get pods` to list')
    expect(html).toContain('<code>kubectl get pods</code>')
  })

  it('renders blockquotes', () => {
    const html = renderMarkdown('> heads up')
    expect(html).toContain('<blockquote>')
    expect(html).toContain('heads up')
  })

  it('renders links with target=_blank and rel=noopener', () => {
    const html = renderMarkdown('see [docs](https://example.com)')
    expect(html).toMatch(/<a [^>]*target="_blank"/)
    expect(html).toMatch(/<a [^>]*rel="noopener noreferrer"/)
    expect(html).toContain('href="https://example.com"')
  })

  it('strips <script> tags (sanitizer)', () => {
    const html = renderMarkdown('hi <script>alert(1)</script> bye')
    expect(html).not.toContain('<script')
    expect(html).not.toContain('alert(1)')
  })

  it('strips inline event handlers', () => {
    const html = renderMarkdown('<a href="x" onclick="bad()">click</a>')
    expect(html).not.toContain('onclick')
  })

  it('strips javascript: URLs', () => {
    const html = renderMarkdown('[hack](javascript:alert(1))')
    expect(html).not.toMatch(/href="javascript:/i)
  })

  it('handles soft line breaks (gfm breaks: true)', () => {
    const html = renderMarkdown('line one\nline two')
    expect(html).toContain('line one')
    expect(html).toContain('line two')
    // breaks: true converts \n to <br> within paragraphs
    expect(html).toContain('<br')
  })
})
