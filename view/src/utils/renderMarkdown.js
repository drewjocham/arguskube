import { marked } from 'marked'
import DOMPurify from 'dompurify'

// Configure marked once. The CodeBlock component handles fenced code (```)
// separately so it can offer Copy / Run-in-terminal buttons; this renderer is
// only invoked on the prose segments between fences. Inline backtick code is
// still rendered as `<code>` here.
marked.setOptions({
  gfm: true,
  breaks: true,
})

// External links open in the system browser via Wails. We mark them as
// external by adding rel and a sentinel data-attr so a click handler at the
// chat root can intercept and route them appropriately if needed.
const renderer = new marked.Renderer()
const linkOpen = renderer.link?.bind(renderer)
if (linkOpen) {
  renderer.link = (...args) => {
    const html = linkOpen(...args)
    return html.replace(/^<a /, '<a target="_blank" rel="noopener noreferrer" ')
  }
}

// Render markdown to a sanitized HTML string suitable for v-html in the chat.
// Falls back to an empty string for nullish/empty input. Sanitization is
// strict: only basic structural and inline tags are allowed.
export function renderMarkdown(text) {
  if (text === null || text === undefined) return ''
  const str = String(text)
  if (!str.trim()) return ''

  let html
  try {
    html = marked.parse(str, { renderer, async: false })
  } catch (e) {
    // If marked itself throws (very rare), fall back to plain text wrapped
    // in a <p> so the message still appears.
    return `<p>${escapeHtml(str)}</p>`
  }

  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      'p', 'br', 'hr',
      'strong', 'em', 'b', 'i', 'u', 's', 'del', 'mark',
      'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
      'ul', 'ol', 'li',
      'blockquote', 'pre', 'code',
      'a', 'span',
      'table', 'thead', 'tbody', 'tr', 'th', 'td',
    ],
    ALLOWED_ATTR: ['href', 'title', 'target', 'rel', 'class'],
  })
}

function escapeHtml(s) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}
