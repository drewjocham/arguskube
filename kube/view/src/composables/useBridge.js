import { ref, onMounted, onUnmounted } from 'vue'
import { bus } from '../lib/bus'

/**
 * Core bridge — the only composable that touches Wails or REST fallback.
 * All domain composables import from here.
 */

// ---------------------------------------------------------------------------
// Environment detection
// ---------------------------------------------------------------------------

export const isWails = () => typeof window !== 'undefined' && !!window.go

export const apiBase = (() => {
  if (typeof window !== 'undefined' && window.__argus_API_BASE__) {
    return window.__argus_API_BASE__
  }
  // 127.0.0.1 — not "localhost" — because on macOS `localhost` resolves
  // to ::1 first. Argus binds 127.0.0.1:8080 (IPv4) by default, so when
  // anything else (Docker, a stray ipv6 listener) is on the IPv6 :8080
  // socket the webview silently hits *that* instead and /auth/providers
  // never reaches us. ARGUS_AUTH_DISABLED then looks broken because
  // loadProviders falls back to authDisabled=false.
  return 'http://127.0.0.1:8080'
})()

// ---------------------------------------------------------------------------
// Response cache — prevents redundant backend calls
// ---------------------------------------------------------------------------

const _cache = new Map()
export const DEFAULT_TTL = 30_000
export const FAST_TTL = 5_000

function _cacheKey(method, args) {
  return args.length ? `${method}:${args.join(':')}` : method
}

/**
 * Cached version of callGo for read-only fetches.
 */
export async function cachedCallGo(method, args = [], ttl = DEFAULT_TTL) {
  const key = _cacheKey(method, args)
  const cached = _cache.get(key)
  if (cached && (Date.now() - cached.ts) < ttl) {
    return cached.data
  }
  const result = await callGo(method, ...args)
  _cache.set(key, { data: result, ts: Date.now() })
  return result
}

export function invalidateCache(method, ...args) {
  const key = _cacheKey(method, args)
  _cache.delete(key)
}

export function invalidateCachePrefix(prefix) {
  for (const key of _cache.keys()) {
    if (key.startsWith(prefix)) _cache.delete(key)
  }
}

// ---------------------------------------------------------------------------
// Raw Go method call
// ---------------------------------------------------------------------------

// Method-name prefixes that signal a *mutating* call. Anything matching
// triggers a global `argus:save` event so the SaveToastStack can render a
// transparent top-right notification — and the persistent bell-panel feed
// can keep a record. Read-only fetches (Get*, List*, Query*) are skipped.
const MUTATING_PREFIXES = [
  'Save', 'Update', 'Create', 'Delete', 'Apply', 'Move',
]
// Specific methods that don't fit the prefix rule but the user still thinks
// of as "this saved/changed something."
const MUTATING_EXTRAS = new Set([
  'ToggleAnomalyRule',
  'SwitchContext',
  'SyncArgusCDApp',
  'RefreshArgusCDApp',
  'RollbackArgusCDApp',
  'AckAlert',
  'SilenceAlert',
  'MarkAlertIgnored',
  'SetAgentProfile',
])

function isMutating(method) {
  if (!method) return false
  if (MUTATING_EXTRAS.has(method)) return true
  for (const p of MUTATING_PREFIXES) {
    if (method.startsWith(p) && method.length > p.length) {
      // 'SetPaused' is a UI hint, not a save the user pressed Save for.
      if (method === 'SetPaused') return false
      return true
    }
  }
  return false
}

function emitSaveEvent(detail) {
  try {
    bus.emit('argus:save', detail)
  } catch {
    // silently no-op in test environments without EventBus
  }
}

export async function callGo(method, ...args) {
  const startedAt = (typeof performance !== 'undefined' ? performance.now() : Date.now())
  const announce = isMutating(method)
  let finalized = false
  const finalize = (status, error) => {
    if (!announce || finalized) return
    finalized = true
    const now = (typeof performance !== 'undefined' ? performance.now() : Date.now())
    emitSaveEvent({
      method,
      status,
      durationMs: Math.round(now - startedAt),
      error: error ? (error?.message || String(error)) : '',
    })
  }

  // Try Wails bindings via window.go (injected by Wails at build time)
  const wailsBinding = isWails() && window.go?.pkg?.App?.[method]
  if (typeof wailsBinding === 'function') {
    try {
      const out = await wailsBinding(...args)
      finalize('ok')
      return out
    } catch (err) {
      console.error(`[wails] ${method} failed:`, err)
      finalize('error', err)
      throw err
    }
  }

  // Fallback to REST API for SaaS mode. Pull the session token from
  // localStorage rather than importing the auth store — the store
  // depends on this bridge, and a circular import would break Pinia
  // initialization order.
  const headers = { 'Content-Type': 'application/json' }
  try {
    const raw = typeof localStorage !== 'undefined' && localStorage.getItem('argus.auth.session')
    if (raw) {
      const parsed = JSON.parse(raw)
      if (parsed?.token) headers.Authorization = `Bearer ${parsed.token}`
    }
  } catch { /* private mode / locked storage — request goes out unauthenticated and gets a 401 */ }
  try {
    const res = await fetch(`${apiBase}/api/${method}`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ args })
    })
    if (res.status === 401) {
      // Session expired or missing. Clear local state so the next
      // tick of the Vue app routes the user to LoginView, then bubble
      // the error so the caller can stop retrying.
      try { localStorage.removeItem('argus.auth.session') } catch {}
      bus.emit('argus:session-expired', {})
      const e = new Error('session expired — please sign in again')
      finalize('error', e)
      throw e
    }
    if (!res.ok) {
      const e = new Error(`HTTP error! status: ${res.status}`)
      finalize('error', e)
      throw e
    }
    const data = await res.json()
    if (data.error) {
      const e = new Error(data.error)
      finalize('error', e)
      throw e
    }
    finalize('ok')
    return data.result
  } catch (err) {
    console.warn(`[saas-api] ${method} fallback failed (is the backend running on :8080?):`, err)
    finalize('error', err)
    throw err
  }
}
