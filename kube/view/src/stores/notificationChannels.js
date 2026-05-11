// notificationChannels — list of delivery destinations for in-app alerts.
//
// Persisted to localStorage. Each channel: { id, kind, label, target, enabled }.
// kind ∈ {'desktop','email','slack','google-chat','webhook'}. target is the
// kind-specific detail (email address, Slack/Google Chat webhook URL, etc.).
// 'desktop' has no target — it's just an OS notification toggle.

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const STORAGE_KEY = 'kw-notification-channels/v1'

function loadInitial() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) return parsed
    }
  } catch {
    // fallthrough
  }
  // First-run default: a disabled desktop channel so the section isn't
  // empty when the user opens it.
  return [
    { id: 'desktop-default', kind: 'desktop', label: 'Desktop notifications', target: '', enabled: false },
  ]
}

function persist(list) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(list))
  } catch {
    // ignore
  }
}

let nextId = 1
function newId(kind) {
  return `${kind}-${Date.now().toString(36)}-${(nextId++).toString(36)}`
}

export const useNotificationChannelsStore = defineStore('notificationChannels', () => {
  const channels = ref(loadInitial())

  const enabled = computed(() => channels.value.filter((c) => c.enabled))
  // hasAny is what callers usually want to ask: "is there *any* channel that
  // would actually deliver a notification right now?" — i.e. enabled AND
  // (kind=='desktop' OR target is non-empty).
  const hasAny = computed(() =>
    enabled.value.some((c) => c.kind === 'desktop' || (c.target || '').trim().length > 0)
  )

  function add(kind, label = '') {
    const c = {
      id: newId(kind),
      kind,
      label: label || defaultLabel(kind),
      target: '',
      enabled: false,
    }
    channels.value = [...channels.value, c]
    persist(channels.value)
    return c
  }

  function update(id, patch) {
    channels.value = channels.value.map((c) =>
      c.id === id ? { ...c, ...patch } : c
    )
    persist(channels.value)
  }

  function remove(id) {
    channels.value = channels.value.filter((c) => c.id !== id)
    persist(channels.value)
  }

  return { channels, enabled, hasAny, add, update, remove }
})

function defaultLabel(kind) {
  switch (kind) {
    case 'email':   return 'Email'
    case 'slack':   return 'Slack webhook'
    case 'google-chat': return 'Google Chat webhook'
    case 'webhook': return 'Generic webhook'
    case 'desktop': return 'Desktop notifications'
    default:        return 'Channel'
  }
}
