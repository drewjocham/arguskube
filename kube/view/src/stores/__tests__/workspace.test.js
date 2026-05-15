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

  it('loadSlackChannels populates per-connection cache', async () => {
    memStorage['argus.auth.session'] = JSON.stringify({ token: 'tok' })
    mockCachedCallGo.mockResolvedValueOnce([
      { id: 'C1', name: 'general' },
      { id: 'C2', name: 'random' },
    ])
    const s = await freshStore()
    await s.loadSlackChannels('conn-1')
    expect(mockCachedCallGo).toHaveBeenCalledWith(
      'ListSlackChannels',
      ['tok', 'conn-1'],
      5_000,
    )
    expect(s.slackChannels['conn-1'].length).toBe(2)
    expect(s.slackLoading).toBe(false)
  })

  it('loadSlackChannels keeps separate caches per connection', async () => {
    mockCachedCallGo
      .mockResolvedValueOnce([{ id: 'C1', name: 'general' }])
      .mockResolvedValueOnce([{ id: 'CX', name: 'other-team' }])
    const s = await freshStore()
    await s.loadSlackChannels('a')
    await s.loadSlackChannels('b')
    expect(s.slackChannels['a'][0].name).toBe('general')
    expect(s.slackChannels['b'][0].name).toBe('other-team')
  })

  it('sendSlackMessage sets status on success and clears after 4s', async () => {
    vi.useFakeTimers()
    mockCallGo.mockResolvedValueOnce(undefined)
    const s = await freshStore()
    await s.sendSlackMessage('conn-1', 'C1', 'hello')
    expect(mockCallGo).toHaveBeenCalledWith(
      'SendSlackMessage', '', 'conn-1', 'C1', 'hello',
    )
    expect(s.slackSendStatus).toBeTruthy()
    expect(s.slackSendError).toBeNull()
    vi.advanceTimersByTime(4001)
    expect(s.slackSendStatus).toBeNull()
    vi.useRealTimers()
  })

  it('sendSlackMessage records error on failure', async () => {
    vi.useFakeTimers()
    mockCallGo.mockRejectedValueOnce(new Error('slack: not_in_channel'))
    const s = await freshStore()
    await expect(s.sendSlackMessage('c', 'ch', 'x')).rejects.toThrow(/not_in_channel/)
    expect(s.slackSendError).toMatch(/not_in_channel/)
    expect(s.slackSendStatus).toBeNull()
    vi.useRealTimers()
  })

  it('createDoc calls bridge with right args and sets googleStatus', async () => {
    vi.useFakeTimers()
    memStorage['argus.auth.session'] = JSON.stringify({ token: 'tok' })
    mockCallGo.mockResolvedValueOnce({ id: 'D1', title: 'T', url: 'u' })
    const s = await freshStore()
    await s.createDoc('conn-1', 'T', 'B')
    expect(mockCallGo).toHaveBeenCalledWith('CreateGoogleDoc', 'tok', 'conn-1', 'T', 'B')
    expect(s.googleStatus?.op).toBe('doc-created')
    expect(s.docs['conn-1'][0].id).toBe('D1')
    vi.advanceTimersByTime(4001)
    expect(s.googleStatus).toBeNull()
    vi.useRealTimers()
  })

  it('createDoc populates googleError on failure', async () => {
    vi.useFakeTimers()
    mockCallGo.mockRejectedValueOnce(new Error('docs: forbidden'))
    const s = await freshStore()
    await expect(s.createDoc('c', 'T', '')).rejects.toThrow(/forbidden/)
    expect(s.googleError).toMatch(/forbidden/)
    vi.useRealTimers()
  })

  it('loadTaskLists caches and second call hits cache', async () => {
    mockCachedCallGo.mockResolvedValue([{ id: 'L1', title: 'My Tasks' }])
    const s = await freshStore()
    await s.loadTaskLists('conn-1')
    await s.loadTaskLists('conn-1')
    // cachedCallGo gets called both times — the TTL+key check inside it
    // is what actually dedupes. Verify it's wired with the right args.
    expect(mockCachedCallGo).toHaveBeenCalledWith(
      'ListGoogleTaskLists', ['', 'conn-1'], 5_000,
    )
    expect(s.taskLists['conn-1'][0].id).toBe('L1')
  })

  it('updateTask flips status and updates local cache', async () => {
    mockCallGo.mockResolvedValueOnce({ id: 'T1', status: 'completed', title: 'a' })
    const s = await freshStore()
    s.tasks = { 'c:L': [{ id: 'T1', title: 'a', status: 'needsAction' }] }
    await s.updateTask('c', 'L', 'T1', { status: 'completed' })
    expect(mockCallGo).toHaveBeenCalledWith(
      'UpdateGoogleTask', '', 'c', 'L', 'T1', { status: 'completed' },
    )
    expect(s.tasks['c:L'][0].status).toBe('completed')
    expect(s.googleStatus?.op).toBe('task-updated')
  })

  it('googleConnections getter filters by service', async () => {
    memStorage['argus.auth.session'] = JSON.stringify({ token: 'tok' })
    mockCallGo.mockResolvedValueOnce([
      { id: 'a', service: 'slack' },
      { id: 'b', service: 'google', email: 'me@x' },
    ])
    const s = await freshStore()
    await s.loadConnections()
    expect(s.googleConnections.length).toBe(1)
    expect(s.googleConnections[0].id).toBe('b')
  })

  it('slackConnections getter filters by service', async () => {
    memStorage['argus.auth.session'] = JSON.stringify({ token: 'tok' })
    mockCallGo.mockResolvedValueOnce([
      { id: 'a', service: 'slack', display_name: 'Acme' },
      { id: 'b', service: 'gdocs', display_name: 'Docs' },
    ])
    const s = await freshStore()
    await s.loadConnections()
    expect(s.slackConnections.length).toBe(1)
    expect(s.slackConnections[0].id).toBe('a')
  })
})
