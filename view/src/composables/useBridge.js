import { ref, onMounted, onUnmounted } from 'vue'

/**
 * Core bridge — the only composable that touches Wails or REST fallback.
 * All domain composables import from here.
 */

// ---------------------------------------------------------------------------
// Environment detection
// ---------------------------------------------------------------------------

export const isWails = () => typeof window !== 'undefined' && !!window.go

export const apiBase = (() => {
  if (typeof window !== 'undefined' && window.__KUBEWATCHER_API_BASE__) {
    return window.__KUBEWATCHER_API_BASE__
  }
  return 'http://localhost:8080'
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

export async function callGo(method, ...args) {
  // Try Wails bindings via window.go (injected by Wails at build time)
  const wailsBinding = isWails() && window.go?.pkg?.App?.[method]
  if (typeof wailsBinding === 'function') {
    try {
      return await wailsBinding(...args)
    } catch (err) {
      console.error(`[wails] ${method} failed:`, err)
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
      window.dispatchEvent(new CustomEvent('argus:session-expired'))
      throw new Error('session expired — please sign in again')
    }
    if (!res.ok) {
      throw new Error(`HTTP error! status: ${res.status}`)
    }
    const data = await res.json()
    if (data.error) throw new Error(data.error)
    return data.result
  } catch (err) {
    console.warn(`[saas-api] ${method} fallback failed (is the backend running on :8080?):`, err)
    throw err
  }
}
