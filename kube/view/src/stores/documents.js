// documents — Pinia store backing Knowledge → Documents.
//
// Placeholder: holds AI-generated artifacts (policy reviews, scan
// summaries, etc.) in memory keyed by id. Consumers call `add(doc)` and
// then `list()` / `get(id)` to read them back. Replace with the real
// implementation (with persistence, dedupe, S3 sync) when that work
// resumes — at that point this stub already matches the call sites in
// NetworkPolicyList.vue and elsewhere.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

let nextId = 1
function newId() {
  return `doc-${Date.now().toString(36)}-${(nextId++).toString(36)}`
}

export const useDocumentsStore = defineStore('documents', () => {
  const docs = ref([])

  // Newest first.
  const ordered = computed(() =>
    [...docs.value].sort((a, b) => b.createdAt - a.createdAt)
  )

  function add(input) {
    const doc = {
      id: input.id || newId(),
      kind: input.kind || 'note',
      title: String(input.title || 'Untitled'),
      body: String(input.body || ''),
      sourceKind: input.sourceKind || '',
      sourcePayload: input.sourcePayload || null,
      meta: input.meta || {},
      createdAt: Date.now(),
    }
    docs.value.unshift(doc)
    return doc
  }

  function get(id) {
    return docs.value.find((d) => d.id === id) || null
  }

  function remove(id) {
    const idx = docs.value.findIndex((d) => d.id === id)
    if (idx >= 0) docs.value.splice(idx, 1)
  }

  function clear() {
    docs.value = []
  }

  function list(kind) {
    if (!kind) return ordered.value
    return ordered.value.filter((d) => d.kind === kind)
  }

  return { docs, ordered, add, get, remove, clear, list }
})
