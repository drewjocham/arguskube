// parseRunbookSegments takes runbook markdown and returns an ordered list of
// segments, each tagged with the heading section it belongs to. The runbook
// preview uses these to render text segments as HTML and code-fence segments
// as interactive <RunbookCodeBlock> components (Copy + Run-in-terminal).
//
// Section semantics (driven by the user's spec):
//   - The most recent heading at any level (#, ##, ###, …) is the "section".
//   - Code blocks that share the same section share the same default
//     terminal session.
//   - A heading change introduces a new logical session.
//   - Top-of-document blocks (before any heading) get section "preamble".
//
// Output shape:
//   [
//     { type: 'text', text: '…', section: 'Preamble', sectionId: 'preamble' },
//     { type: 'code', code: '…', language: 'bash',
//        section: 'Verify pods', sectionId: '1-verify-pods', codeIndex: 0 },
//     …
//   ]
//
// codeIndex is a doc-wide running counter so two code blocks with identical
// content under the same heading still get distinct ids.

const FENCE = /```([a-zA-Z0-9_+-]*)\n?([\s\S]*?)(?:```|$)/g

export interface TextSegment {
  type: 'text'
  text: string
  section: string
  sectionId: string
}
export interface CodeSegment {
  type: 'code'
  code: string
  language: string
  section: string
  sectionId: string
  codeIndex: number
}
export type RunbookSegment = TextSegment | CodeSegment

export interface SectionRef {
  id: string
  label: string
}

function slugify(s: unknown): string {
  return String(s || '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64) || 'section'
}

// Walk the input linearly. Between fence matches we may cross headings; the
// "current section" tracks whichever heading we last saw.
export function parseRunbookSegments(input: unknown): RunbookSegment[] {
  const text = typeof input === 'string' ? input : String(input ?? '')
  if (!text) return []

  const segments: RunbookSegment[] = []
  let codeIndex = 0
  let currentSection = 'Preamble'
  let currentSectionId = 'preamble'
  let sectionCounter = 0

  // updateSection scans `chunk` for the LAST heading line and, if found,
  // updates current section. We use the last heading because if a chunk has
  // multiple headings, all subsequent code blocks belong to the most recent.
  function updateSectionFromChunk(chunk: string): void {
    const matches = [...chunk.matchAll(/^(#{1,6})\s+(.+?)\s*$/gm)]
    if (matches.length > 0) {
      const last = matches[matches.length - 1]
      sectionCounter++
      currentSection = last[2].trim()
      currentSectionId = `${sectionCounter}-${slugify(currentSection)}`
    }
  }

  let lastIndex = 0
  FENCE.lastIndex = 0

  let match: RegExpExecArray | null
  while ((match = FENCE.exec(text)) !== null) {
    const fenceStart = match.index
    if (fenceStart > lastIndex) {
      const plain = text.slice(lastIndex, fenceStart)
      if (plain.length > 0) {
        // Headings live in text segments, so update section from the chunk
        // BEFORE we emit the segment — code blocks then get attributed to
        // the heading that immediately preceded them.
        updateSectionFromChunk(plain)
        segments.push({
          type: 'text',
          text: plain,
          section: currentSection,
          sectionId: currentSectionId,
        })
      }
    }
    const language = (match[1] || '').toLowerCase()
    const body = (match[2] || '').replace(/\n+$/, '')
    segments.push({
      type: 'code',
      code: body,
      language,
      section: currentSection,
      sectionId: currentSectionId,
      codeIndex: codeIndex++,
    })
    lastIndex = FENCE.lastIndex
  }

  if (lastIndex < text.length) {
    const tail = text.slice(lastIndex)
    if (tail.length > 0) {
      updateSectionFromChunk(tail)
      segments.push({
        type: 'text',
        text: tail,
        section: currentSection,
        sectionId: currentSectionId,
      })
    }
  }

  return segments
}

// uniqueSections returns the ordered list of (sectionId, section) pairs that
// appear in the segments. Used to drive the "select a terminal session"
// UI — each section maps to one logical session.
export function uniqueSections(segments: readonly RunbookSegment[]): SectionRef[] {
  const seen = new Set<string>()
  const out: SectionRef[] = []
  for (const s of segments) {
    if (!seen.has(s.sectionId)) {
      seen.add(s.sectionId)
      out.push({ id: s.sectionId, label: s.section })
    }
  }
  return out
}

export const __test = { slugify }
