// useSpotCheck — small composable that mirrors the backend
// "what I'm currently doing" event into a reactive ref. The titlebar
// reads this and renders a transient pill while a probe is running.
//
// The persistent findings flow through `argus:notification` and land
// in the notifications store; this composable only owns the
// short-lived "active right now" state.

import { ref } from 'vue'
import { useWailsEvent } from './useEvents'
import { callGo } from './useBridge'

const active = ref(null) // { checkName, description } or null

export function useSpotCheck() {
  // Subscribe at most once per app session — useWailsEvent dedupes
  // for us via the underlying runtime listener.
  useWailsEvent('argus:spotcheck:active', (data) => {
    if (!data || !data.checkName) {
      active.value = null
      return
    }
    active.value = {
      checkName: data.checkName,
      description: data.description || data.checkName,
    }
  })

  function runAll() {
    return callGo('RunSpotChecks').catch((e) => {
      console.warn('[spot-check] RunSpotChecks failed:', e)
    })
  }

  function runOne(name) {
    if (!name) return Promise.resolve()
    return callGo('RunSpotCheck', name).catch((e) => {
      console.warn('[spot-check] RunSpotCheck failed:', name, e)
    })
  }

  return { active, runAll, runOne }
}
