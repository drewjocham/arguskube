/**
 * Log syntax highlighting — tokenizes a log message string into
 * an array of { text, cls } segments for Vue v-for rendering.
 *
 * Exported separately so a parse error here can't take down the
 * parent component tree.
 */

const RULES = [
  { pattern: /\b(FATAL|PANIC)\b/g,          cls: 'hl-fatal' },
  { pattern: /\b(ERROR|ERR)\b/g,            cls: 'hl-error' },
  { pattern: /\b(WARN|WARNING)\b/g,         cls: 'hl-warn' },
  { pattern: /\b(INFO)\b/g,                 cls: 'hl-info' },
  { pattern: /\b(DEBUG|TRACE)\b/g,          cls: 'hl-debug' },
  { pattern: /\b(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\b/g, cls: 'hl-method' },
  { pattern: /\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?/g,  cls: 'hl-ip' },
  { pattern: /\b\d+(\.\d+)?(ns|us|ms)\b/g,  cls: 'hl-duration' },
  { pattern: /"[^"]*"/g,                     cls: 'hl-string' },
  { pattern: /[a-zA-Z_][\w.]*(?==)/g,        cls: 'hl-key' },
]

export function tokenize(msg) {
  if (!msg || typeof msg !== 'string') {
    return [{ text: String(msg || ''), cls: '' }]
  }

  var hits = []
  for (var i = 0; i < RULES.length; i++) {
    var rule = RULES[i]
    var re = rule.pattern
    re.lastIndex = 0
    var m
    while ((m = re.exec(msg)) !== null) {
      hits.push({ start: m.index, end: m.index + m[0].length, text: m[0], cls: rule.cls })
    }
  }

  if (hits.length === 0) {
    return [{ text: msg, cls: '' }]
  }

  hits.sort(function (a, b) { return a.start - b.start || b.end - a.end })

  var kept = []
  var cursor = 0
  for (var j = 0; j < hits.length; j++) {
    if (hits[j].start >= cursor) {
      kept.push(hits[j])
      cursor = hits[j].end
    }
  }

  var segs = []
  var pos = 0
  for (var k = 0; k < kept.length; k++) {
    var h = kept[k]
    if (h.start > pos) {
      segs.push({ text: msg.slice(pos, h.start), cls: '' })
    }
    segs.push({ text: h.text, cls: h.cls })
    pos = h.end
  }
  if (pos < msg.length) {
    segs.push({ text: msg.slice(pos), cls: '' })
  }

  return segs
}
