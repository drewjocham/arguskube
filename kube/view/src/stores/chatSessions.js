// chatSessions — Pinia store backing the multi-session AI chat panel.
//
// Placeholder implementation that satisfies the surface used by
// ArgusAIChat.vue (sortedSessions, activeId, activeSession, setActive,
// create, remove, rename, autoTitleFromFirstMessage, recordMessage). Stays
// purely in memory; persistence + cross-tab sync can be layered on later.
//
// This file was missing on the buddy/bugfix/config-section-scroll-and-switch
// branch when its consumers were checked in — landing this stub unblocks
// the build. Replace with the production implementation when that work
// resumes.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const MAX_TITLE = 60
let nextId = 1

function newId() {
  return `s-${Date.now().toString(36)}-${(nextId++).toString(36)}`
}

function makeSession(overrides = {}) {
  const now = Date.now()
  return {
    id: newId(),
    title: 'New chat',
    createdAt: now,
    updatedAt: now,
    messageCount: 0,
    ...overrides,
  }
}

export const useChatSessionsStore = defineStore('chatSessions', () => {
  const sessions = ref([makeSession({ title: 'Default session' })])
  const activeId = ref(sessions.value[0].id)

  // Newest active first, then by updatedAt desc, then createdAt desc.
  const sortedSessions = computed(() =>
    [...sessions.value].sort((a, b) => {
      if (a.id === activeId.value) return -1
      if (b.id === activeId.value) return 1
      if (a.updatedAt !== b.updatedAt) return b.updatedAt - a.updatedAt
      return b.createdAt - a.createdAt
    })
  )

  const activeSession = computed(() =>
    sessions.value.find((s) => s.id === activeId.value) || null
  )

  function setActive(id) {
    if (sessions.value.some((s) => s.id === id)) activeId.value = id
  }

  function create(title) {
    const s = makeSession(title ? { title } : {})
    sessions.value.push(s)
    activeId.value = s.id
    return s
  }

  function remove(id) {
    const idx = sessions.value.findIndex((s) => s.id === id)
    if (idx < 0) return
    sessions.value.splice(idx, 1)
    if (activeId.value === id) {
      activeId.value = sessions.value[0]?.id ?? create().id
    }
  }

  function rename(id, title) {
    const s = sessions.value.find((s) => s.id === id)
    if (!s) return
    const trimmed = String(title ?? '').trim().slice(0, MAX_TITLE)
    if (trimmed) {
      s.title = trimmed
      s.updatedAt = Date.now()
    }
  }

  // Set the title from the first user message if the session still has its
  // default name. Only fires once per session — manual renames are sticky.
  function autoTitleFromFirstMessage(id, message) {
    const s = sessions.value.find((s) => s.id === id)
    if (!s) return
    if (s.title && s.title !== 'New chat' && s.title !== 'Default session') return
    const text = String(message ?? '').trim()
    if (!text) return
    s.title = text.length > MAX_TITLE ? text.slice(0, MAX_TITLE - 1) + '…' : text
    s.updatedAt = Date.now()
  }

  function recordMessage(id) {
    const s = sessions.value.find((s) => s.id === id)
    if (!s) return
    s.messageCount += 1
    s.updatedAt = Date.now()
  }

  return {
    sessions,
    activeId,
    sortedSessions,
    activeSession,
    setActive,
    create,
    remove,
    rename,
    autoTitleFromFirstMessage,
    recordMessage,
  }
})
