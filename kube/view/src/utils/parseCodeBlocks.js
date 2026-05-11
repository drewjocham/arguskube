// parseCodeBlocks splits a markdown-ish string into an ordered list of
// segments, preserving the original text. Each segment is either:
//   { type: 'text', text: '...' }
//   { type: 'code', language: 'bash', text: '...' }
//
// Only triple-backtick fences are recognized — the goal is to surface code
// blocks as actionable UI (Copy / Send to terminal), not to render full
// markdown. Inline single-backtick code is left in the text segment.
//
// The parser tolerates unclosed fences (e.g. an LLM cuts off mid-block) by
// treating everything after the opening fence as code.
const FENCE = /```([a-zA-Z0-9_+-]*)\n?([\s\S]*?)(?:```|$)/g

const SHELL_LANGS = new Set([
  '', 'sh', 'bash', 'zsh', 'shell', 'console', 'kubectl', 'fish',
])

export function isShellLanguage(lang) {
  if (!lang) return true
  return SHELL_LANGS.has(String(lang).toLowerCase())
}

export function parseCodeBlocks(input) {
  const text = typeof input === 'string' ? input : String(input ?? '')
  if (!text) return []

  const segments = []
  let lastIndex = 0
  FENCE.lastIndex = 0

  let match
  while ((match = FENCE.exec(text)) !== null) {
    const fenceStart = match.index
    if (fenceStart > lastIndex) {
      const plain = text.slice(lastIndex, fenceStart)
      if (plain.length > 0) {
        segments.push({ type: 'text', text: plain })
      }
    }
    const language = (match[1] || '').toLowerCase()
    const body = (match[2] || '').replace(/\n+$/, '')
    segments.push({ type: 'code', language, text: body })
    lastIndex = FENCE.lastIndex
  }

  if (lastIndex < text.length) {
    const tail = text.slice(lastIndex)
    if (tail.length > 0) {
      segments.push({ type: 'text', text: tail })
    }
  }

  return segments
}
