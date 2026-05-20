// Helpers for the LogExplorer query input — small, dependency-free utilities
// for shaping suggestions and repairing common syntax issues so they're
// testable in isolation. Anything that needs to call into the AI backend
// belongs alongside but in a separate module.

export interface CuratedSuggestion {
  label: string
  query: string
  description: string
}

export interface Suggestion extends CuratedSuggestion {
  kind: 'curated' | 'field-value'
}

export type FieldMap = Record<string, readonly (string | number)[]>

// Curated list of common queries that show up in the suggestions dropdown.
// Each entry is { label, query, description } so the dropdown can render a
// human-readable hint alongside the actual selector.
export const CURATED_SUGGESTIONS: readonly CuratedSuggestion[] = [
  {
    label: 'All errors',
    query: '{level="error"}',
    description: 'Lines logged at error level across all streams.',
  },
  {
    label: 'Pod crashes',
    query: '{message=~".*(panic|fatal|CrashLoopBackOff).*"}',
    description: 'Likely crash signals — panics, fatals, crashloop messages.',
  },
  {
    label: 'OOM kills',
    query: '{message=~".*OOMKilled.*"}',
    description: 'Out-of-memory kill events.',
  },
  {
    label: 'kube-system noise',
    query: '{kubernetes.pod_namespace="kube-system"}',
    description: 'Logs from the kube-system namespace.',
  },
  {
    label: 'Last hour',
    query: '{message=~".+"}',
    description: 'All logs (use the limit + time controls to scope).',
  },
  {
    label: 'CoreDNS',
    query: '{kubernetes.pod_name=~"coredns-.*"}',
    description: 'DNS resolver pods only.',
  },
]

// Build the dropdown content for a given query string. Returns an ordered
// list of suggestions: matches against curated entries first, then label-
// based completions, then live field-completion suggestions derived from the
// `fields` argument (a map of fieldName → known values).
export function buildSuggestions(query: string | undefined | null, fields: FieldMap = {}): Suggestion[] {
  const q = (query || '').trim().toLowerCase()
  const out: Suggestion[] = []

  for (const s of CURATED_SUGGESTIONS) {
    if (!q || s.label.toLowerCase().includes(q) || s.query.toLowerCase().includes(q)) {
      out.push({ ...s, kind: 'curated' })
    }
  }

  // Field-value completions: when the user typed `something="parti…`, suggest
  // matching values from `fields[field]`.
  const m = String(query || '').match(/([\w.-]+)\s*(=~?|=)\s*"?([^"}]*)$/)
  if (m) {
    const [, field, , partial] = m
    const values = fields[field] || []
    const partialLower = (partial || '').toLowerCase()
    for (const v of values) {
      if (!partialLower || String(v).toLowerCase().includes(partialLower)) {
        out.push({
          label: `${field}="${v}"`,
          query: `{${field}="${v}"}`,
          description: `Filter by ${field}.`,
          kind: 'field-value',
        })
        if (out.length > 25) break
      }
    }
  }

  return out
}

// Cheap client-side query repair. Fixes the obvious LogQL/Loki-style mistakes
// without round-tripping to a model:
//
//   - balances unmatched { } and " "
//   - replaces smart quotes with regular ones (common when copied from docs)
//   - converts bare `field=value` to `field="value"`
//   - normalizes whitespace
//
// Returns { fixed, changed, notes[] } so the UI can show what changed.
export interface FixQueryResult {
  fixed: string
  changed: boolean
  notes: string[]
}

export function fixQuerySyntax(input: unknown): FixQueryResult {
  const original = String(input || '')
  const notes: string[] = []
  let q = original

  // 1. Curly / unicode quotes → straight ASCII. Run first so all later
  //    quote-counting logic sees consistent characters.
  const smartQuoteRE = /[“”‘’]/g
  if (smartQuoteRE.test(q)) {
    q = q.replace(smartQuoteRE, '"')
    notes.push('Replaced curly quotes with straight quotes.')
  }

  // 2. Balance braces — fix this BEFORE the per-block passes below so that
  //    those passes see well-formed { … } regions.
  {
    const opens = (q.match(/\{/g) || []).length
    const closes = (q.match(/\}/g) || []).length
    if (opens > closes) {
      q = q + '}'.repeat(opens - closes)
      notes.push(`Added ${opens - closes} missing closing brace(s).`)
    } else if (closes > opens) {
      q = '{'.repeat(closes - opens) + q
      notes.push(`Added ${closes - opens} missing opening brace(s).`)
    }
  }

  // 3. Per-brace-block fixes:
  //      a) repair missing closing quote BEFORE the next `,` or `}`
  //      b) wrap bare values in double-quotes
  let quoteFixed = false
  let bareFixed = false
  q = q.replace(/\{([^{}]*)\}/g, (_full, innerOriginal) => {
    let inner = innerOriginal

    // a) Missing closing quote: if the block has an odd number of quotes,
    //    insert one at the next selector boundary (`,` or end of block).
    const dq = (inner.match(/"/g) || []).length
    if (dq % 2 === 1) {
      const lastQuote = inner.lastIndexOf('"')
      const tail = inner.slice(lastQuote + 1)
      const stop = tail.search(/[,]/) // brace handled by the surround
      if (stop === -1) {
        inner = inner + '"'
      } else {
        inner = inner.slice(0, lastQuote + 1) + tail.slice(0, stop) + '"' + tail.slice(stop)
      }
      quoteFixed = true
    }

    // b) Bare values: `field=value` (or `field=~value`) → `field="value"`.
    inner = inner.replace(/([\w.-]+)\s*(=~?|=)\s*([^"\s,}=][^,}]*)/g, (_match: string, field: string, op: string, value: string) => {
      bareFixed = true
      return `${field}${op}"${value.trim()}"`
    })

    return `{${inner}}`
  })
  if (quoteFixed) notes.push('Added a missing closing double-quote.')
  if (bareFixed) notes.push('Quoted bare values inside { … } blocks.')

  // 4. Last-resort: if there's *still* an odd number of quotes outside any
  //    brace block (rare), append a closing one so the query at least parses.
  {
    const dq = (q.match(/"/g) || []).length
    if (dq % 2 === 1) {
      q = q + '"'
      notes.push('Appended a final closing double-quote.')
    }
  }

  // 5. Normalize whitespace (outside of quoted regions). The curly-quote
  //    pass already ran, so quotes here are ASCII.
  q = q.replace(/[\t ]+/g, ' ').trim()

  return { fixed: q, changed: q !== original, notes }
}
