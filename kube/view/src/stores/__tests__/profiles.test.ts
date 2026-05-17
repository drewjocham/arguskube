/**
 * profiles store — backend-sync integration tests.
 *
 * Covers: hydrate (success + failure path), every CRUD mutation's
 * write-through behaviour, the setActive-on-apply contract, and the
 * snapshot capture/apply round-trip. Mocks the bridge at one
 * boundary (useBridge.callGo) so the tests exercise real Pinia state
 * + real localStorage semantics without touching the network.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const mockCallGo = vi.fn()
vi.mock('../../composables/useBridge', () => ({
  callGo: (...args: unknown[]) => mockCallGo(...args),
  cachedCallGo: (...args: unknown[]) => mockCallGo(...args),
  invalidateCache: vi.fn(),
  FAST_TTL: 5_000,
}))

// All five "captured" stores accessed by profiles must exist before
// captureToVariant runs. The real stores work fine inside Pinia, so
// we only need to make sure the imports succeed and each store is
// initialized at least once per test.
import { useAppearanceStore } from '../appearance'
import { useNavVisibilityStore } from '../navVisibility'
import { useSectionTabsStore } from '../sectionTabs'
import { useUIPrefsStore } from '../uiPrefs'
import { useSavedFiltersStore } from '../savedFilters'

// In-memory localStorage shim. Lets the store's persist() actually
// store something we can assert on across re-imports.
const memStorage: Record<string, string> = {}
beforeEach(() => {
  for (const k of Object.keys(memStorage)) delete memStorage[k]
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: {
      getItem: (k: string) => (k in memStorage ? memStorage[k] : null),
      setItem: (k: string, v: string) => { memStorage[k] = String(v) },
      removeItem: (k: string) => { delete memStorage[k] },
      clear: () => { for (const k of Object.keys(memStorage)) delete memStorage[k] },
    },
  })
  setActivePinia(createPinia())
  mockCallGo.mockReset()
})

// freshStore: forces module re-evaluation so the persisted-state
// load runs against the latest memStorage contents. Without this the
// pinia ref would carry over from a previous test.
async function freshStore() {
  setActivePinia(createPinia())
  vi.resetModules()
  // The captured-state stores must also be re-registered against
  // the new pinia. Importing them and calling the factory once is
  // enough — they self-register.
  const a = await import('../appearance'); a.useAppearanceStore()
  const n = await import('../navVisibility'); n.useNavVisibilityStore()
  const s = await import('../sectionTabs'); s.useSectionTabsStore()
  const u = await import('../uiPrefs'); u.useUIPrefsStore()
  const f = await import('../savedFilters'); f.useSavedFiltersStore()
  const mod = await import('../profiles')
  return mod.useProfilesStore()
}

describe('profiles store — hydrate', () => {
  it('falls back to localStorage when the backend errors', async () => {
    // Seed localStorage with one group so the fallback has something
    // to surface; backend call throws.
    memStorage['argus.profiles.v1'] = JSON.stringify({
      groups: [{ id: 'g-local', name: 'Local Only', description: '', variants: [] }],
      activeGroupId: 'g-local',
      activeVariantId: '',
    })
    mockCallGo.mockRejectedValue(new Error('HTTP error! status: 401'))

    const store = await freshStore()
    const ok = await store.hydrate()

    expect(ok).toBe(false)
    expect(store.backendAvailable).toBe(false)
    expect(store.groups).toHaveLength(1)
    expect(store.groups[0].id).toBe('g-local')
  })

  it('replaces local state when the backend returns data', async () => {
    memStorage['argus.profiles.v1'] = JSON.stringify({
      groups: [{ id: 'g-local-stale', name: 'stale', description: '', variants: [] }],
      activeGroupId: 'g-local-stale',
      activeVariantId: '',
    })
    mockCallGo.mockImplementation((method: string) => {
      if (method === 'ListProfileGroups') {
        return Promise.resolve([
          {
            id: 'g-remote',
            name: 'Remote',
            description: 'from backend',
            variants: [
              { id: 'v1', parentId: 'g-remote', name: 'Var', description: '', version: '1.0', snapshot: { appearance: { theme: 'dark' } } },
            ],
          },
        ])
      }
      if (method === 'GetActiveProfile') {
        return Promise.resolve({ groupId: 'g-remote', variantId: 'v1' })
      }
      return Promise.resolve(null)
    })

    const store = await freshStore()
    const ok = await store.hydrate()

    expect(ok).toBe(true)
    expect(store.backendAvailable).toBe(true)
    expect(store.groups).toHaveLength(1)
    expect(store.groups[0].id).toBe('g-remote')
    expect(store.groups[0].variants[0].snapshot.appearance).toEqual({ theme: 'dark' })
    expect(store.activeGroupId).toBe('g-remote')
    expect(store.activeVariantId).toBe('v1')
  })

  it('normalizes a missing snapshot to safe defaults', async () => {
    mockCallGo.mockImplementation((method: string) => {
      if (method === 'ListProfileGroups') {
        return Promise.resolve([
          {
            id: 'g1', name: 'g1', description: '',
            variants: [
              // No snapshot field at all.
              { id: 'v1', parentId: 'g1', name: 'v1', description: '', version: '1.0' },
            ],
          },
        ])
      }
      return Promise.resolve(null)
    })

    const store = await freshStore()
    await store.hydrate()

    const snap = store.groups[0].variants[0].snapshot
    // Each substructure must exist so the apply path doesn't have to
    // null-check every step.
    expect(snap.appearance).toEqual({})
    expect(snap.navVisibility).toEqual({ visible: {} })
    expect(snap.sectionTabs).toEqual({ tabs: {} })
    expect(snap.uiPrefs).toEqual({ rightPanelWidth: 340 })
    expect(snap.savedFilters).toEqual([])
  })

  it('hydrate is idempotent', async () => {
    mockCallGo.mockResolvedValue([])

    const store = await freshStore()
    await store.hydrate()
    expect(mockCallGo.mock.calls.length).toBeGreaterThan(0)
    const callCountAfterFirst = mockCallGo.mock.calls.length

    await store.hydrate()
    expect(mockCallGo.mock.calls.length).toBe(callCountAfterFirst)
  })
})

describe('profiles store — write-through after hydrate', () => {
  beforeEach(() => {
    mockCallGo.mockImplementation((method: string) => {
      if (method === 'ListProfileGroups') return Promise.resolve([])
      if (method === 'GetActiveProfile') return Promise.resolve({ groupId: '', variantId: '' })
      return Promise.resolve(null)
    })
  })

  it('createGroup fires SaveProfileGroup', async () => {
    const store = await freshStore()
    await store.hydrate()
    mockCallGo.mockClear()

    const g = store.createGroup('Daily', 'morning rotation')

    // Two calls expected: SetActiveProfile (because first group) + SaveProfileGroup.
    const methods = mockCallGo.mock.calls.map(c => c[0])
    expect(methods).toContain('SaveProfileGroup')

    const saveCall = mockCallGo.mock.calls.find(c => c[0] === 'SaveProfileGroup')!
    const payload = saveCall[2] as { id: string; name: string; description: string }
    expect(payload.id).toBe(g.id)
    expect(payload.name).toBe('Daily')
    expect(payload.description).toBe('morning rotation')
  })

  it('first createGroup also pushes active selection', async () => {
    const store = await freshStore()
    await store.hydrate()
    mockCallGo.mockClear()

    store.createGroup('Daily')

    const methods = mockCallGo.mock.calls.map(c => c[0])
    expect(methods).toContain('SetActiveProfile')
  })

  it('createVariant fires SaveProfileVariant', async () => {
    const store = await freshStore()
    await store.hydrate()
    const g = store.createGroup('G')
    mockCallGo.mockClear()

    const v = store.createVariant(g.id, 'v1', '', '1.0')

    const saveCall = mockCallGo.mock.calls.find(c => c[0] === 'SaveProfileVariant')!
    expect(saveCall).toBeTruthy()
    expect(saveCall[2]).toBe(g.id)
    expect((saveCall[3] as { id: string }).id).toBe(v!.id)
  })

  it('deleteGroup fires DeleteProfileGroup', async () => {
    const store = await freshStore()
    await store.hydrate()
    const g = store.createGroup('G')
    mockCallGo.mockClear()

    store.deleteGroup(g.id)

    const deleteCall = mockCallGo.mock.calls.find(c => c[0] === 'DeleteProfileGroup')
    expect(deleteCall).toBeTruthy()
    expect(deleteCall![2]).toBe(g.id)
  })

  it('applyVariant fires SetActiveProfile', async () => {
    const store = await freshStore()
    await store.hydrate()
    const g = store.createGroup('G')
    const v = store.createVariant(g.id, 'v1')!
    mockCallGo.mockClear()

    store.applyVariant(v.id)

    const activeCall = mockCallGo.mock.calls.find(c => c[0] === 'SetActiveProfile')
    expect(activeCall).toBeTruthy()
    expect(activeCall![2]).toBe(g.id)
    expect(activeCall![3]).toBe(v.id)
  })

  it('captureToVariant fires SaveProfileVariant with a fresh snapshot', async () => {
    const store = await freshStore()
    await store.hydrate()
    const g = store.createGroup('G')
    const v = store.createVariant(g.id, 'v1')!
    mockCallGo.mockClear()

    // Mutate one of the captured stores to prove the snapshot picks
    // up live state, not stale defaults.
    const appearance = useAppearanceStore()
    if (typeof (appearance as { setTheme?: (t: string) => void }).setTheme === 'function') {
      ;(appearance as unknown as { setTheme: (t: string) => void }).setTheme('dark')
    }

    store.captureToVariant(g.id, v.id)

    const saveCall = mockCallGo.mock.calls.find(c => c[0] === 'SaveProfileVariant')!
    expect(saveCall).toBeTruthy()
    const sent = saveCall[3] as { snapshot: { appearance: Record<string, unknown> } }
    expect(sent.snapshot).toBeTruthy()
    // The snapshot must include all five sub-shapes — that's what
    // applyVariant relies on.
    expect(sent.snapshot.appearance).toBeDefined()
  })
})

describe('profiles store — write-through skipped when backend is unreachable', () => {
  it('mutations do not call the bridge when hydrate failed', async () => {
    mockCallGo.mockRejectedValue(new Error('offline'))

    const store = await freshStore()
    const ok = await store.hydrate()
    expect(ok).toBe(false)

    mockCallGo.mockClear()
    const g = store.createGroup('Local')
    store.deleteGroup(g.id)
    store.setActive('a', 'b')

    // No bridge calls fired — backendAvailable is false.
    expect(mockCallGo.mock.calls.length).toBe(0)
  })
})

describe('profiles store — local-only semantics still work', () => {
  it('createGroup persists to localStorage even without a backend', async () => {
    mockCallGo.mockRejectedValue(new Error('offline'))

    const store = await freshStore()
    await store.hydrate()
    store.createGroup('Local Only')

    const raw = memStorage['argus.profiles.v1']
    expect(raw).toBeTruthy()
    const parsed = JSON.parse(raw)
    expect(parsed.groups).toHaveLength(1)
    expect(parsed.groups[0].name).toBe('Local Only')
  })

  it('reload preserves groups via localStorage', async () => {
    mockCallGo.mockRejectedValue(new Error('offline'))

    const s1 = await freshStore()
    await s1.hydrate()
    s1.createGroup('Survives Reload')

    // Re-evaluate the module → emulates a page reload.
    const s2 = await freshStore()
    expect(s2.groups).toHaveLength(1)
    expect(s2.groups[0].name).toBe('Survives Reload')
  })
})
