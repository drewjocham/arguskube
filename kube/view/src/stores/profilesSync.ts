// Thin backend bridge for the profiles store. Keeps the network
// surface in one place so the store stays focused on Vue reactivity
// and the unit tests can mock a single boundary.
//
// All functions are best-effort: a failed network call returns a
// useful default (null / {} / nothing) and lets the caller continue
// with the localStorage fallback. The store's existing semantics
// (sync mutations, localStorage as authoritative on-disk) keep
// working even when the bridge is permanently unavailable (web
// mode without auth, anonymous local dev, offline tab).

import { callGo } from '../composables/useBridge'
import type { ProfileGroup, ProfileVariant } from '../types/profiles'

// SessionTokenGetter is injected so the store can plug in its existing
// localStorage-based token reader without duplicating the parsing.
// Returning "" is fine — the backend dev-mode bypass accepts it, and
// production callers will end up with HTTP 401 which we treat as
// "backend unavailable" downstream.
export type SessionTokenGetter = () => string

// BackendGroup mirrors the Go-side profilespkg.Group on the wire.
// Snapshot arrives as a raw object (Go's json.RawMessage decoded
// into a value); the store maps it to ProfileSnapshot field-by-field.
export interface BackendGroup {
  id: string
  name: string
  description: string
  variants: BackendVariant[]
  createdAt?: string
  updatedAt?: string
}

export interface BackendVariant {
  id: string
  parentId: string
  name: string
  description: string
  version: string
  snapshot: unknown
  createdAt?: string
  updatedAt?: string
}

export interface BackendActive {
  groupId: string
  variantId: string
  updatedAt?: string
}

// loadFromBackend hydrates the store on init. Returns null on any
// failure (network down, unauthenticated, 403, parse error) — the
// caller then falls back to localStorage. Returns [] (empty array,
// not null) when the backend is reachable but the user has no
// profiles yet.
export async function loadFromBackend(
  token: SessionTokenGetter,
): Promise<BackendGroup[] | null> {
  try {
    const result = await callGo('ListProfileGroups', token())
    if (!Array.isArray(result)) return null
    return result as BackendGroup[]
  } catch {
    return null
  }
}

export async function loadActiveFromBackend(
  token: SessionTokenGetter,
): Promise<BackendActive | null> {
  try {
    const result = await callGo('GetActiveProfile', token())
    if (!result || typeof result !== 'object') return null
    const r = result as Partial<BackendActive>
    return {
      groupId: r.groupId || '',
      variantId: r.variantId || '',
      updatedAt: r.updatedAt,
    }
  } catch {
    return null
  }
}

// pushGroup upserts a group. Returns the canonical (server-stamped)
// version on success, the input unchanged on failure — the caller
// has already updated its in-memory state, so falling back to the
// input means at most a stale createdAt that the next hydration
// will correct.
export async function pushGroup(
  token: SessionTokenGetter,
  group: ProfileGroup,
): Promise<BackendGroup | ProfileGroup> {
  try {
    const result = await callGo('SaveProfileGroup', token(), {
      id: group.id,
      name: group.name,
      description: group.description,
      // We deliberately omit the variants array — variant rows are
      // managed by pushVariant calls; bundling them into the group
      // upsert would invite stale data overwriting fresh writes.
    })
    return (result as BackendGroup) ?? group
  } catch {
    return group
  }
}

export async function pushVariant(
  token: SessionTokenGetter,
  groupId: string,
  variant: ProfileVariant,
): Promise<BackendVariant | ProfileVariant> {
  try {
    const result = await callGo('SaveProfileVariant', token(), groupId, {
      id: variant.id,
      parentId: variant.parentId,
      name: variant.name,
      description: variant.description,
      version: variant.version,
      snapshot: variant.snapshot,
    })
    return (result as BackendVariant) ?? variant
  } catch {
    return variant
  }
}

// pushDelete* never raise. Backends can return 404 (we removed
// already) or 401 (unauthenticated) — neither case changes what the
// store should do, so we swallow.
export async function pushDeleteGroup(
  token: SessionTokenGetter,
  groupId: string,
): Promise<void> {
  try {
    await callGo('DeleteProfileGroup', token(), groupId)
  } catch {
    // best-effort
  }
}

export async function pushDeleteVariant(
  token: SessionTokenGetter,
  groupId: string,
  variantId: string,
): Promise<void> {
  try {
    await callGo('DeleteProfileVariant', token(), groupId, variantId)
  } catch {
    // best-effort
  }
}

export async function pushActive(
  token: SessionTokenGetter,
  groupId: string,
  variantId: string,
): Promise<void> {
  try {
    await callGo('SetActiveProfile', token(), groupId, variantId)
  } catch {
    // best-effort
  }
}
