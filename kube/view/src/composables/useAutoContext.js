import { ref } from 'vue'
import { callGo } from './useBridge'

// useAutoContext drives the §2 "first launch reads the context set" flow.
// On the very first call per session it asks the backend to:
//   1. probe every kubeconfig context in parallel,
//   2. choose the best one (active-reachable > fallback-reachable > active-anyway),
//   3. switch the live k8s client to that context.
//
// The backend emits "argus:status" events along the way so the bottom
// <StatusRibbon> shows the journey live — this composable does NOT push
// into the statusFeed itself, to avoid double-reporting.
//
// State is module-level on purpose: the resolution is one-per-session
// data, and multiple consumers (App.vue triggers it, the checklist
// producer watches it, the sidebar reads probe rows) MUST observe the
// same refs or the watchers won't fire across boundaries. A fresh
// useAutoContext() call returns handles into this shared state.

const resolution = ref(null)   // k8s.ContextResolution from the backend
const error = ref(null)
const loading = ref(false)
let attempted = false

async function _run() {
  loading.value = true
  error.value = null
  try {
    const r = await callGo('AutoResolveContext')
    resolution.value = r || null
    return r
  } catch (e) {
    error.value = e?.message || String(e)
    resolution.value = null
    return null
  } finally {
    loading.value = false
  }
}

export function useAutoContext() {
  // resolve() is the entry point App.vue calls once after auth. The second
  // call is a no-op so re-mounts don't re-probe.
  async function resolve() {
    if (attempted) return resolution.value
    attempted = true
    return _run()
  }
  // reprobe() lets the user (or the settings checklist) force a fresh scan
  // — e.g. after editing kubeconfig or toggling VPN.
  async function reprobe() {
    attempted = true
    return _run()
  }

  return { resolution, error, loading, resolve, reprobe }
}

// Test-only helper: reset the module-level state between tests without
// exporting it as part of the public surface.
export const __test = {
  reset() {
    attempted = false
    resolution.value = null
    error.value = null
    loading.value = false
  },
}
