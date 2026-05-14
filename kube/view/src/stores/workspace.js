import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { callGo, cachedCallGo, invalidateCache, FAST_TTL } from '../composables/useBridge'

// Workspace store — owns the user's third-party integration connections
// (Slack today; Google services land in Phase 2). Tokens never travel
// through this store; the backend keeps them encrypted at rest and
// only the redacted WorkspaceConnectionView shape comes over the wire.
//
// Session-token resolution: the Wails GetSessionToken binding is the
// authoritative source (Keychain-backed). In SaaS/web mode the binding
// is absent and the bridge falls back to the bearer in localStorage —
// the backend will read it from the Authorization header instead, so
// we pass "" for the sessionToken argument and let the HTTP layer
// re-attach the bearer transparently.
//
// OAuth popup contract: startConnect() opens the system browser at the
// returned auth URL. The OAuth callback page is expected to post back
//   { type: 'workspace:complete', service, state, code }
// via window.opener.postMessage. We poll for closed-without-complete
// so the spinner doesn't hang forever if the user cancels the flow.

const OAUTH_TIMEOUT_MS = 5 * 60 * 1000
const POPUP_POLL_MS = 500

function getSessionTokenSync() {
  // The Keychain getter is async, which makes wiring it into every
  // action awkward. We rely on the HTTP fallback to fill it in for
  // SaaS mode; in Wails mode the bound method ignores the arg and
  // reads the session from the in-process state anyway (see
  // app_workspace.go workspaceUserID — it pulls from auth.store).
  try {
    if (typeof localStorage === 'undefined') return ''
    const raw = localStorage.getItem('argus.auth.session')
    if (!raw) return ''
    const parsed = JSON.parse(raw)
    return parsed?.token || ''
  } catch { return '' }
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const services = ref([])
  const connections = ref([])
  const loading = ref(false)
  const error = ref(null)

  const connectionsByService = computed(() => {
    const out = {}
    for (const c of connections.value) {
      if (!out[c.service]) out[c.service] = []
      out[c.service].push(c)
    }
    return out
  })

  async function loadServices() {
    try {
      const result = await cachedCallGo('ListWorkspaceServices', [], FAST_TTL)
      services.value = Array.isArray(result) ? result : []
    } catch (e) {
      error.value = e?.message || String(e)
      services.value = []
    }
  }

  async function loadConnections() {
    loading.value = true
    error.value = null
    try {
      const token = getSessionTokenSync()
      const result = await callGo('ListWorkspaceConnections', token)
      connections.value = Array.isArray(result) ? result : []
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  async function startConnect(service) {
    error.value = null
    const token = getSessionTokenSync()
    // The redirectURL is left blank: the backend picks the right loopback
    // listener for desktop builds (or the SaaS callback path in cloud
    // builds). Passing "" keeps the frontend out of that decision.
    const auth = await callGo('StartWorkspaceConnect', token, service, '')
    if (!auth?.url) throw new Error('workspace: backend did not return an auth URL')

    const popup = window.open(auth.url, '_blank', 'noopener,noreferrer,width=520,height=720')
    // popup may be null when noopener is honored; in that case the
    // callback page still needs to ping us via BroadcastChannel or
    // localStorage. For phase 1A we assume noopener returns a handle.
    return await new Promise((resolve, reject) => {
      const start = Date.now()
      let done = false
      const cleanup = () => {
        done = true
        window.removeEventListener('message', onMessage)
        if (poll) clearInterval(poll)
      }
      const onMessage = async (ev) => {
        const d = ev.data
        if (!d || d.type !== 'workspace:complete') return
        if (d.service !== service || d.state !== auth.state) return
        cleanup()
        try {
          const conn = await callGo('CompleteWorkspaceConnect', d.service, d.state, d.code)
          invalidateCache('ListWorkspaceConnections')
          await loadConnections()
          try { popup?.close?.() } catch { /* cross-origin close — harmless */ }
          resolve(conn)
        } catch (e) {
          error.value = e?.message || String(e)
          reject(e)
        }
      }
      window.addEventListener('message', onMessage)
      const poll = setInterval(() => {
        if (done) return
        if (Date.now() - start > OAUTH_TIMEOUT_MS) {
          cleanup()
          reject(new Error('Connection timed out — please try again'))
          return
        }
        if (popup && popup.closed) {
          cleanup()
          reject(new Error('Connection canceled before sign-in finished'))
        }
      }, POPUP_POLL_MS)
    })
  }

  async function disconnect(id) {
    error.value = null
    try {
      const token = getSessionTokenSync()
      await callGo('DeleteWorkspaceConnection', token, id)
      invalidateCache('ListWorkspaceConnections')
      await loadConnections()
    } catch (e) {
      error.value = e?.message || String(e)
      throw e
    }
  }

  return {
    services,
    connections,
    loading,
    error,
    connectionsByService,
    loadServices,
    loadConnections,
    startConnect,
    disconnect,
  }
})
