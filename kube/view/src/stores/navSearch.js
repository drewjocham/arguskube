// Tiny store wiring the titlebar search bar to the sidebar's filter
// state. Two consumers — Titlebar reads/writes, Sidebar reads — so a
// shared Pinia store is simpler than props/events plumbing through
// App.vue.
//
// Not persisted: search input clears every session intentionally.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useNavSearchStore = defineStore('navSearch', () => {
  const query = ref('')

  const trimmed = computed(() => query.value.trim())
  const lower = computed(() => trimmed.value.toLowerCase())
  const active = computed(() => trimmed.value.length > 0)

  function setQuery(v) {
    query.value = String(v ?? '')
  }
  function clear() {
    query.value = ''
  }

  // Returns true when label loosely matches the current query. We use
  // simple case-insensitive substring matching — fast, predictable,
  // and matches the user's mental model better than fuzzy.
  function matches(label) {
    if (!active.value) return true
    return String(label || '').toLowerCase().includes(lower.value)
  }

  // Splits a label into [{text, hit}] segments so the renderer can
  // wrap matched spans in a <mark>. Single-pass, returns the original
  // label as one non-hit segment when there's no active query.
  function highlight(label) {
    const s = String(label || '')
    if (!active.value) return [{ text: s, hit: false }]
    const q = lower.value
    const lc = s.toLowerCase()
    const out = []
    let i = 0
    while (i < s.length) {
      const idx = lc.indexOf(q, i)
      if (idx < 0) {
        out.push({ text: s.slice(i), hit: false })
        break
      }
      if (idx > i) out.push({ text: s.slice(i, idx), hit: false })
      out.push({ text: s.slice(idx, idx + q.length), hit: true })
      i = idx + q.length
    }
    return out
  }

  return {
    query,
    active,
    setQuery,
    clear,
    matches,
    highlight,
  }
})
