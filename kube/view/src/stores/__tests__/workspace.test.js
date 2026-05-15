import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

const mockCallGo = vi.fn()
const mockCachedCallGo = vi.fn()
const mockInvalidate = vi.fn()
vi.mock('../../composables/useBridge', () => ({
  callGo: (...a) => mockCallGo(...a),
  cachedCallGo: (...a) => mockCachedCallGo(...a),
  invalidateCache: (...a) => mockInvalidate(...a),
  FAST_TTL: 5_000,
}))

const memStorage = {}
beforeEach(() => {
  for (const k of Object.keys(memStorage)) delete memStorage[k]
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: {
      getItem: (k) => (k in memStorage ? memStorage[k] : null),
      setItem: (k, v) => { memStorage[k] = String(v) },
      removeItem: (k) => { delete memStorage[k] },
      clear: () => { for (const k of Object.keys(memStorage)) delete memStorage[k] },
    },
  })
  setActivePinia(createPinia())
  mockCallGo.mockReset()
  mockCachedCallGo.mockReset()
  mockInvalidate.mockReset()
})

async function freshStore() {
  setActivePinia(createPinia())
  vi.resetModules()
  const mod = await import('../workspace')
  return mod.useWorkspaceStore()
}

describe('workspace store', () => {
  it('loadServices populates services from the bridge', async () => {
    mockCachedCallGo.mockResolvedValueOnce(['gdocs'])
    const s = await freshStore()
    await s.loadServices()
    expect(mockCachedCallGo).toHaveBeenCalledWith('ListWorkspaceServices', [], 5_000)
    expect(s.services).toEqual(['gdocs'])
  })

  it('loadConnections stores rows and groups by service', async () => {
    memStorage['argus.auth.session'] = JSON.stringify({ token: 'tok' })
    mockCallGo.mockResolvedValueOnce([
      { id: 'a', service: 'gdocs', display_name: 'Acme' },
      { id: 'b', service: 'gdocs', display_name: 'Beta' },
    ])
    const s = await freshStore()
    await s.loadConnections()
    expect(mockCallGo).toHaveBeenCalledWith('ListWorkspaceConnections', 'tok')
    expect(s.connections.length).toBe(2)
    expect(s.connectionsByService.gdocs.length).toBe(2)
  })

  it('loadConnections records error message on failure', async () => {
    mockCallGo.mockRejectedValueOnce(new Error('boom'))
    const s = await freshStore()
    await s.loadConnections()
    expect(s.error).toMatch(/boom/)
    expect(s.connections).toEqual([])
  })

  it('disconnect calls backend and invalidates cache', async () => {
    mockCallGo
      .mockResolvedValueOnce(undefined)   // DeleteWorkspaceConnection
      .mockResolvedValueOnce([])          // ListWorkspaceConnections refresh
    const s = await freshStore()
    await s.disconnect('conn-id')
    expect(mockCallGo).toHaveBeenCalledWith('DeleteWorkspaceConnection', '', 'conn-id')
    expect(mockInvalidate).toHaveBeenCalledWith('ListWorkspaceConnections')
  })
})
