import { callGo } from './useBridge'
import { useNavVisibilityStore } from '../stores/navVisibility'

// useNavVisibilityProbes — bridges the §C1 "smart first-launch
// defaults" requirement to the navVisibility store. On startup we
// probe the backend for each optional section's prerequisite; if the
// probe resolves truthy we reveal the matching section.
//
// Two contracts the rest of the app depends on:
//
//   1. Probes are silent. A failure leaves the section in its current
//      (hidden) state — the user can still reveal it in Settings.
//      Never auto-HIDE; the user's explicit "show" always wins.
//
//   2. Probes never block the UI. They run in parallel inside
//      navVisibility.initialize() which itself runs as a fire-and-
//      forget Promise off App.vue's onMounted. The initial render
//      shows just the 5 core sections; optional ones flip in
//      asynchronously.

function safeBool(v) {
  // Coerce backend responses to a boolean. We treat numbers (count
  // queries), non-empty strings (path/url responses), arrays (list
  // responses with len > 0), and the literal `true` as "yes, this
  // subsystem is present".
  if (typeof v === 'number') return v > 0
  if (typeof v === 'string') return v.trim().length > 0
  if (Array.isArray(v)) return v.length > 0
  return !!v
}

function withSafe(promise) {
  return Promise.resolve(promise).catch(() => null)
}

/**
 * Build the probe map navVisibility.initialize() consumes. Each
 * function returns a Promise<boolean>. Failures resolve to false
 * (the surrounding initialize() already swallows rejections, but we
 * collapse here too so a noisy console.warn doesn't surface mid-boot).
 */
export function buildDefaultProbes() {
  return {
    // STORAGE — reveal when the cluster has any PVCs. We list pvcs
    // rather than pvs because pvcs are namespace-scoped and reflect
    // workload state more directly. A cluster with PVCs almost
    // certainly has a Storage tab user.
    storage: async () => {
      const res = await withSafe(callGo('ListResources', 'pvcs', ''))
      if (!res) return false
      if (Array.isArray(res)) return res.length > 0
      // Backend may wrap the list under a key — accept either shape.
      if (Array.isArray(res?.items)) return res.items.length > 0
      return false
    },

    // KNOWLEDGE — reveal when the user has configured an S3 bucket
    // for notebooks. We read settings instead of probing the bucket
    // directly so the probe is instant + works while offline.
    knowledge: async () => {
      const settings = await withSafe(callGo('GetSettings'))
      if (!settings || typeof settings !== 'object') return false
      // Accept either the camelCase Wails shape or the snake_case
      // HTTP shape so a SaaS deploy works the same as the desktop
      // build.
      const bucket = settings.s3Bucket ?? settings.S3Bucket ?? settings.s3_bucket
      return safeBool(bucket)
    },

    // CONFIG and ADMIN are deliberately opt-in. Config rarely
    // changes day to day; Admin (Setup + Settings) is reachable via
    // the right-click "Open Navigation Settings" path on any section
    // header and via Cmd+K. Hiding them by default keeps the sidebar
    // calm for the SRE's home view.
  }
}

/**
 * Composable that runs the probes and feeds them into the store.
 * Call once from App.vue after sign-in.
 */
export function useNavVisibilityProbes() {
  const store = useNavVisibilityStore()

  async function run() {
    if (store.initialized) return
    try {
      await store.initialize(buildDefaultProbes())
    } catch {
      // Already swallowed inside initialize, but defense-in-depth:
      // never bubble probe errors past this composable.
    }
  }

  return { run }
}

// Test-only export so unit tests can verify the probe map shape
// without mocking the bridge.
export const __test = { buildDefaultProbes, safeBool }
