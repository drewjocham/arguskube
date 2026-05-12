import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useFeatures, useChat, useNotebooks, useRunbooks, useIncidents, useWorkflows } from '../useData'
import { invalidateCachePrefix, invalidateCache } from '../useBridge'

describe('useFeatures', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('GetFeatures')
    invalidateCachePrefix('GetTier')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns features, tier, isAllowed, refresh', () => {
    const result = useFeatures()
    expect(result.features.value.alerts).toBe(true)
    expect(result.features.value.cluster_view).toBe(true)
    expect(result.tier.value).toBe('pro')
    expect(typeof result.isAllowed).toBe('function')
    expect(typeof result.refresh).toBe('function')
  })

  it('isAllowed returns true for known features by default', () => {
    const { isAllowed } = useFeatures()
    expect(isAllowed('alerts')).toBe(true)
    expect(isAllowed('cluster_view')).toBe(true)
    expect(isAllowed('unknown_feature')).toBe(false)
  })

  it('refresh fetches features and tier from backend', async () => {
    const featureData = { alerts: true, cluster_view: false, log_stream: true }
    const tierData = 'enterprise'

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetFeatures: vi.fn().mockResolvedValue(featureData),
          GetTier: vi.fn().mockResolvedValue(tierData),
        },
      },
    })

    const { features, tier, refresh } = useFeatures()
    await refresh()

    expect(features.value).toEqual(featureData)
    expect(tier.value).toBe(tierData)
  })

  it('refresh keeps defaults when backend returns empty', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetFeatures: vi.fn().mockResolvedValue({}),
          GetTier: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { features, tier, refresh } = useFeatures()
    await refresh()

    expect(features.value.alerts).toBe(true)
    expect(tier.value).toBe('pro')
  })

  it('refresh gracefully falls back when backend unavailable', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetFeatures: vi.fn().mockRejectedValue(new Error('Backend down')),
          GetTier: vi.fn().mockRejectedValue(new Error('Backend down')),
        },
      },
    })

    const { features, tier, refresh } = useFeatures()
    await refresh()

    expect(features.value.alerts).toBe(true)
    expect(tier.value).toBe('pro')
  })

  it('refresh falls back to fetch when window.go unavailable', async () => {
    const mockFetch = vi.fn()
      .mockResolvedValueOnce({
        ok: true,
        json: vi.fn().mockResolvedValue({ result: { alerts: true, log_stream: false } }),
      })
      .mockResolvedValueOnce({
        ok: true,
        json: vi.fn().mockResolvedValue({ result: 'enterprise' }),
      })
    vi.stubGlobal('fetch', mockFetch)

    const { features, tier, refresh } = useFeatures()
    await refresh()

    expect(features.value).toEqual({ alerts: true, log_stream: false })
    expect(tier.value).toBe('enterprise')
  })
})

describe('useChat', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns all expected refs and methods', () => {
    const result = useChat()
    expect(result.history.value).toEqual([])
    expect(result.sending.value).toBe(false)
    expect(result.autoSummary.value).toBe(null)
    expect(result.eventLog.value).toEqual([])
    expect(typeof result.sendMessage).toBe('function')
    expect(typeof result.refreshHistory).toBe('function')
    expect(typeof result.fetchAutoSummary).toBe('function')
    expect(typeof result.fetchEventLog).toBe('function')
  })

  it('sendMessage calls SendChatMessage and refreshes history', async () => {
    const mockSend = vi.fn().mockResolvedValue({ content: 'ack' })
    const mockHistory = vi.fn().mockResolvedValue([{ role: 'assistant', content: 'ack' }])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          SendChatMessage: mockSend,
          GetChatHistory: mockHistory,
        },
      },
    })

    const { history, sending, sendMessage } = useChat()
    const response = await sendMessage('alert-1', 'hello')

    expect(mockSend).toHaveBeenCalledWith('alert-1', 'hello')
    expect(response).toEqual({ content: 'ack' })
    expect(history.value).toEqual([{ role: 'assistant', content: 'ack' }])
    expect(sending.value).toBe(false)
  })

  it('sendMessage sets sending correctly', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          SendChatMessage: vi.fn().mockImplementation(
            () => new Promise(r => setTimeout(() => r({}), 10))
          ),
          GetChatHistory: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { sending, sendMessage } = useChat()
    const promise = sendMessage('alert-1', 'hello')

    expect(sending.value).toBe(true)
    await promise
    expect(sending.value).toBe(false)
  })

  it('sendMessage throws on error', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          SendChatMessage: vi.fn().mockRejectedValue(new Error('Chat error')),
          GetChatHistory: vi.fn().mockResolvedValue([]),
        },
      },
    })

    const { sendMessage } = useChat()
    await expect(sendMessage('alert-1', 'hello')).rejects.toThrow('Chat error')
  })

  it('refreshHistory fetches and sets history', async () => {
    const chatHistory = [
      { role: 'user', content: 'hello' },
      { role: 'assistant', content: 'hi' },
    ]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetChatHistory: vi.fn().mockResolvedValue(chatHistory),
        },
      },
    })

    const { history, refreshHistory } = useChat()
    await refreshHistory('alert-1')

    expect(history.value).toEqual(chatHistory)
  })

  it('refreshHistory handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetChatHistory: vi.fn().mockRejectedValue(new Error('History unavailable')),
        },
      },
    })

    const { history, refreshHistory } = useChat()
    await refreshHistory('alert-1')

    expect(history.value).toEqual([])
  })

  it('fetchAutoSummary calls GetAutoSummary', async () => {
    const summaryData = { title: 'Summary', text: 'Pod restarting' }

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAutoSummary: vi.fn().mockResolvedValue(summaryData),
        },
      },
    })

    const { autoSummary, fetchAutoSummary } = useChat()
    await fetchAutoSummary('alert-1')

    expect(autoSummary.value).toEqual(summaryData)
  })

  it('fetchAutoSummary handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAutoSummary: vi.fn().mockRejectedValue(new Error('Summary unavailable')),
        },
      },
    })

    const { autoSummary, fetchAutoSummary } = useChat()
    await fetchAutoSummary('alert-1')

    expect(autoSummary.value).toBe(null)
  })

  it('fetchEventLog calls GetAgentEventLog', async () => {
    const events = [{ type: 'info', message: 'Agent started' }]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAgentEventLog: vi.fn().mockResolvedValue(events),
        },
      },
    })

    const { eventLog, fetchEventLog } = useChat()
    await fetchEventLog()

    expect(eventLog.value).toEqual(events)
  })

  it('fetchEventLog handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetAgentEventLog: vi.fn().mockRejectedValue(new Error('Log unavailable')),
        },
      },
    })

    const { eventLog, fetchEventLog } = useChat()
    await fetchEventLog()

    expect(eventLog.value).toEqual([])
  })
})

describe('useNotebooks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns all expected refs and methods', () => {
    const result = useNotebooks()
    expect(result.files.value).toEqual([])
    expect(result.loading.value).toBe(false)
    expect(result.saving.value).toBe(false)
    expect(result.synced.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(typeof result.listFiles).toBe('function')
    expect(typeof result.getFile).toBe('function')
    expect(typeof result.saveFile).toBe('function')
    expect(typeof result.deleteFile).toBe('function')
    expect(typeof result.createFolder).toBe('function')
    expect(typeof result.testConnection).toBe('function')
    expect(typeof result.addFileToTree).toBe('function')
    expect(typeof result.moveFile).toBe('function')
  })

  it('listFiles populates files from backend', async () => {
    const fileData = [
      { path: 'note.md', name: 'note.md', isDir: false },
      { path: 'dir', name: 'dir', isDir: true },
    ]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListNotebooks: vi.fn().mockResolvedValue(fileData),
        },
      },
    })

    const { files, loading, synced, error, listFiles } = useNotebooks()
    await listFiles()

    expect(files.value).toEqual(fileData)
    expect(synced.value).toBe(true)
    expect(error.value).toBe(null)
    expect(loading.value).toBe(false)
  })

  it('listFiles returns empty array on null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListNotebooks: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { files, listFiles } = useNotebooks()
    await listFiles()

    expect(files.value).toEqual([])
  })

  it('listFiles handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListNotebooks: vi.fn().mockRejectedValue(new Error('S3 error')),
        },
      },
    })

    const { files, error, loading, listFiles } = useNotebooks()
    await listFiles()

    expect(files.value).toEqual([])
    expect(error.value).toBeTruthy()
    expect(loading.value).toBe(false)
  })

  it('getFile returns file content from backend', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetNotebook: vi.fn().mockResolvedValue('# Hello'),
        },
      },
    })

    const { getFile } = useNotebooks()
    const content = await getFile('note.md')

    expect(content).toBe('# Hello')
  })

  it('getFile returns empty string on null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetNotebook: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { getFile } = useNotebooks()
    const content = await getFile('note.md')

    expect(content).toBe('')
  })

  it('getFile handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetNotebook: vi.fn().mockRejectedValue(new Error('File not found')),
        },
      },
    })

    const { getFile } = useNotebooks()
    const content = await getFile('note.md')

    expect(content).toBe('')
  })

  it('saveFile calls SaveNotebook and sets synced', async () => {
    const mockSave = vi.fn().mockResolvedValue(undefined)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveNotebook: mockSave,
        },
      },
    })

    const { saving, synced, saveFile } = useNotebooks()
    await saveFile('note.md', '# updated')

    expect(mockSave).toHaveBeenCalledWith('note.md', '# updated')
    expect(synced.value).toBe(true)
    expect(saving.value).toBe(false)
  })

  it('saveFile sets synced to false on error', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveNotebook: vi.fn().mockRejectedValue(new Error('Save failed')),
        },
      },
    })

    const { synced, saveFile } = useNotebooks()
    await saveFile('note.md', '# updated')

    expect(synced.value).toBe(false)
  })

  it('deleteFile calls DeleteNotebook and removes from local tree', async () => {
    const mockDelete = vi.fn().mockResolvedValue(undefined)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteNotebook: mockDelete,
        },
      },
    })

    const result = useNotebooks()
    result.files.value = [{ path: 'note.md' }, { path: 'other.md' }]

    await result.deleteFile('note.md')

    expect(mockDelete).toHaveBeenCalledWith('note.md')
    expect(result.files.value).toEqual([{ path: 'other.md' }])
  })

  it('deleteFile handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteNotebook: vi.fn().mockRejectedValue(new Error('Delete failed')),
        },
      },
    })

    const { files, deleteFile } = useNotebooks()
    files.value = [{ path: 'a.md' }, { path: 'b.md' }]

    await deleteFile('a.md')

    expect(files.value).toEqual([{ path: 'b.md' }])
  })

  it('createFolder calls CreateNotebookFolder then listFiles', async () => {
    const mockCreate = vi.fn().mockResolvedValue(undefined)
    const mockList = vi.fn().mockResolvedValue([{ path: 'newdir/', isDir: true }])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          CreateNotebookFolder: mockCreate,
          ListNotebooks: mockList,
        },
      },
    })

    const { files, createFolder } = useNotebooks()
    await createFolder('newdir/')

    expect(mockCreate).toHaveBeenCalledWith('newdir/')
    expect(files.value).toEqual([{ path: 'newdir/', isDir: true }])
  })

  it('testConnection returns ok on success', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          TestS3Connection: vi.fn().mockResolvedValue(undefined),
        },
      },
    })

    const { testConnection } = useNotebooks()
    const result = await testConnection()

    expect(result).toEqual({ ok: true })
  })

  it('testConnection returns error on failure', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          TestS3Connection: vi.fn().mockRejectedValue(new Error('Connection failed')),
        },
      },
    })

    const { testConnection } = useNotebooks()
    const result = await testConnection()

    expect(result.ok).toBe(false)
    expect(result.error).toBeTruthy()
  })

  it('addFileToTree calls listFiles', async () => {
    const mockList = vi.fn().mockResolvedValue([{ path: 'newfile.md' }])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListNotebooks: mockList,
        },
      },
    })

    const { addFileToTree } = useNotebooks()
    addFileToTree('dir', 'newfile.md')

    await vi.waitFor(() => expect(mockList).toHaveBeenCalled())
  })

  it('moveFile calls MoveNotebook then listFiles', async () => {
    const mockMove = vi.fn().mockResolvedValue(undefined)
    const mockList = vi.fn().mockResolvedValue([{ path: 'newdir/note.md' }])

    vi.stubGlobal('go', {
      pkg: {
        App: {
          MoveNotebook: mockMove,
          ListNotebooks: mockList,
        },
      },
    })

    const { files, moveFile } = useNotebooks()
    await moveFile('note.md', 'newdir/note.md')

    expect(mockMove).toHaveBeenCalledWith('note.md', 'newdir/note.md')
    expect(files.value).toEqual([{ path: 'newdir/note.md' }])
  })
})

describe('useRunbooks', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('ListRunbooks')
    invalidateCachePrefix('GetRunbook')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns all expected refs and methods', () => {
    const result = useRunbooks()
    expect(result.runbooks.value).toEqual([])
    expect(result.loading.value).toBe(false)
    expect(result.saving.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(typeof result.listRunbooks).toBe('function')
    expect(typeof result.getRunbook).toBe('function')
    expect(typeof result.saveRunbook).toBe('function')
    expect(typeof result.deleteRunbook).toBe('function')
    expect(typeof result.createRunbook).toBe('function')
  })

  it('listRunbooks populates runbooks from backend', async () => {
    const rbData = [{ id: 'rb-1', name: 'Pod Crash', trigger: 'PodCrash' }]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListRunbooks: vi.fn().mockResolvedValue(rbData),
        },
      },
    })

    const { runbooks, loading, error, listRunbooks } = useRunbooks()
    await listRunbooks()

    expect(runbooks.value).toEqual(rbData)
    expect(error.value).toBe(null)
    expect(loading.value).toBe(false)
  })

  it('listRunbooks handles null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListRunbooks: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { runbooks, listRunbooks } = useRunbooks()
    await listRunbooks()

    expect(runbooks.value).toEqual([])
  })

  it('listRunbooks handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListRunbooks: vi.fn().mockRejectedValue(new Error('Backend error')),
        },
      },
    })

    const { runbooks, error, listRunbooks } = useRunbooks()
    await listRunbooks()

    expect(runbooks.value).toEqual([])
    expect(error.value).toBeTruthy()
  })

  it('getRunbook returns runbook content from backend', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetRunbook: vi.fn().mockResolvedValue('steps: ...'),
        },
      },
    })

    const { getRunbook } = useRunbooks()
    const content = await getRunbook('rb-1')

    expect(content).toBe('steps: ...')
  })

  it('getRunbook returns empty string on null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetRunbook: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { getRunbook } = useRunbooks()
    const content = await getRunbook('rb-1')

    expect(content).toBe('')
  })

  it('getRunbook handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetRunbook: vi.fn().mockRejectedValue(new Error('Not found')),
        },
      },
    })

    const { getRunbook } = useRunbooks()
    const content = await getRunbook('rb-1')

    expect(content).toBe('')
  })

  it('saveRunbook calls SaveRunbook and invalidates cache', async () => {
    const mockSave = vi.fn().mockResolvedValue(undefined)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveRunbook: mockSave,
        },
      },
    })

    const { saving, saveRunbook } = useRunbooks()
    await saveRunbook('rb-1', '# my runbook')

    expect(mockSave).toHaveBeenCalledWith('rb-1', '# my runbook')
    expect(saving.value).toBe(false)
  })

  it('saveRunbook handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveRunbook: vi.fn().mockRejectedValue(new Error('Save error')),
        },
      },
    })

    const { saveRunbook } = useRunbooks()
    await expect(saveRunbook('rb-1', 'content')).resolves.toBeUndefined()
  })

  it('deleteRunbook calls DeleteRunbook and removes from list', async () => {
    const mockDelete = vi.fn().mockResolvedValue(undefined)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteRunbook: mockDelete,
        },
      },
    })

    const result = useRunbooks()
    result.runbooks.value = [{ id: 'rb-1' }, { id: 'rb-2' }]

    await result.deleteRunbook('rb-1')

    expect(mockDelete).toHaveBeenCalledWith('rb-1')
    expect(result.runbooks.value).toEqual([{ id: 'rb-2' }])
  })

  it('deleteRunbook handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteRunbook: vi.fn().mockRejectedValue(new Error('Delete error')),
        },
      },
    })

    const { runbooks, deleteRunbook } = useRunbooks()
    runbooks.value = [{ id: 'rb-1' }]

    await deleteRunbook('rb-1')

    expect(runbooks.value).toEqual([])
  })

  it('createRunbook calls CreateRunbook and adds to list', async () => {
    const newRb = { id: 'rb-3', name: 'New RB', trigger: 'Custom' }
    const mockCreate = vi.fn().mockResolvedValue(newRb)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          CreateRunbook: mockCreate,
        },
      },
    })

    const { runbooks, createRunbook } = useRunbooks()
    runbooks.value = [{ id: 'rb-1', name: 'Old' }]

    const result = await createRunbook('New RB', 'Custom')

    expect(mockCreate).toHaveBeenCalledWith('New RB', 'Custom')
    expect(result).toEqual(newRb)
    expect(runbooks.value).toContainEqual(newRb)
  })

  it('createRunbook returns null on failure', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          CreateRunbook: vi.fn().mockRejectedValue(new Error('Create error')),
        },
      },
    })

    const { createRunbook } = useRunbooks()
    const result = await createRunbook('Fails', 'Whatever')

    expect(result).toBe(null)
  })
})

describe('useIncidents', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
    invalidateCachePrefix('ListIncidents')
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns all expected refs and methods', () => {
    const result = useIncidents()
    expect(result.incidents.value).toEqual([])
    expect(result.loading.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(typeof result.listIncidents).toBe('function')
    expect(typeof result.createIncident).toBe('function')
    expect(typeof result.updateIncident).toBe('function')
    expect(typeof result.deleteIncident).toBe('function')
  })

  it('listIncidents populates incidents from backend', async () => {
    const incData = [{ id: 'inc-1', title: 'Pod crash', severity: 'high' }]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListIncidents: vi.fn().mockResolvedValue(incData),
        },
      },
    })

    const { incidents, loading, error, listIncidents } = useIncidents()
    await listIncidents()

    expect(incidents.value).toEqual(incData)
    expect(error.value).toBe(null)
    expect(loading.value).toBe(false)
  })

  it('listIncidents handles null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListIncidents: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { incidents, listIncidents } = useIncidents()
    await listIncidents()

    expect(incidents.value).toEqual([])
  })

  it('listIncidents handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListIncidents: vi.fn().mockRejectedValue(new Error('API error')),
        },
      },
    })

    const { incidents, error, listIncidents } = useIncidents()
    await listIncidents()

    expect(incidents.value).toEqual([])
    expect(error.value).toBeTruthy()
  })

  it('createIncident calls CreateIncident and prepends to list', async () => {
    const newInc = { id: 'inc-2', title: 'Memory leak', severity: 'critical' }
    const mockCreate = vi.fn().mockResolvedValue(newInc)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          CreateIncident: mockCreate,
        },
      },
    })

    const { incidents, createIncident } = useIncidents()
    incidents.value = [{ id: 'inc-1', title: 'Old' }]

    const result = await createIncident('Memory leak', 'critical', 'alert', 'desc', 'ns')

    expect(mockCreate).toHaveBeenCalledWith('Memory leak', 'critical', 'alert', 'desc', 'ns')
    expect(result).toEqual(newInc)
    expect(incidents.value[0]).toEqual(newInc)
    expect(incidents.value.length).toBe(2)
  })

  it('createIncident returns null on failure', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          CreateIncident: vi.fn().mockRejectedValue(new Error('Create error')),
        },
      },
    })

    const { createIncident } = useIncidents()
    const result = await createIncident('Fail', 'info')

    expect(result).toBe(null)
  })

  it('updateIncident calls UpdateIncident and replaces in list', async () => {
    const updated = { id: 'inc-1', title: 'Updated', status: 'resolved', severity: 'low' }
    const mockUpdate = vi.fn().mockResolvedValue(updated)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          UpdateIncident: mockUpdate,
        },
      },
    })

    const { incidents, updateIncident } = useIncidents()
    incidents.value = [{ id: 'inc-1', title: 'Old', severity: 'high' }]

    const result = await updateIncident('inc-1', 'resolved', 'Done')

    expect(mockUpdate).toHaveBeenCalledWith('inc-1', 'resolved', 'Done')
    expect(result).toEqual(updated)
    expect(incidents.value[0]).toEqual(updated)
  })

  it('updateIncident returns undefined on failure', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          UpdateIncident: vi.fn().mockRejectedValue(new Error('Update error')),
        },
      },
    })

    const { updateIncident } = useIncidents()
    const result = await updateIncident('inc-1', 'resolved', 'Done')

    expect(result).toBeUndefined()
  })

  it('deleteIncident calls DeleteIncident and removes from list', async () => {
    const mockDelete = vi.fn().mockResolvedValue(undefined)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteIncident: mockDelete,
        },
      },
    })

    const { incidents, deleteIncident } = useIncidents()
    incidents.value = [{ id: 'inc-1' }, { id: 'inc-2' }]

    await deleteIncident('inc-1')

    expect(mockDelete).toHaveBeenCalledWith('inc-1')
    expect(incidents.value).toEqual([{ id: 'inc-2' }])
  })

  it('deleteIncident handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteIncident: vi.fn().mockRejectedValue(new Error('Delete error')),
        },
      },
    })

    const { incidents, deleteIncident } = useIncidents()
    incidents.value = [{ id: 'inc-1' }]

    await deleteIncident('inc-1')

    expect(incidents.value).toEqual([])
  })
})

describe('useWorkflows', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('returns all expected refs and methods', () => {
    const result = useWorkflows()
    expect(result.workflows.value).toEqual([])
    expect(result.current.value).toBe(null)
    expect(result.loading.value).toBe(false)
    expect(result.saving.value).toBe(false)
    expect(result.error.value).toBe(null)
    expect(typeof result.listWorkflows).toBe('function')
    expect(typeof result.getWorkflow).toBe('function')
    expect(typeof result.saveWorkflow).toBe('function')
    expect(typeof result.deleteWorkflow).toBe('function')
  })

  it('listWorkflows populates workflows from backend', async () => {
    const wfData = [{ id: 'wf-1', title: 'Auto Remediation', stepCount: 3 }]

    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListWorkflows: vi.fn().mockResolvedValue(wfData),
        },
      },
    })

    const { workflows, loading, error, listWorkflows } = useWorkflows()
    await listWorkflows()

    expect(workflows.value).toEqual(wfData)
    expect(error.value).toBe(null)
    expect(loading.value).toBe(false)
  })

  it('listWorkflows handles null result', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListWorkflows: vi.fn().mockResolvedValue(null),
        },
      },
    })

    const { workflows, listWorkflows } = useWorkflows()
    await listWorkflows()

    expect(workflows.value).toEqual([])
  })

  it('listWorkflows handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          ListWorkflows: vi.fn().mockRejectedValue(new Error('Backend error')),
        },
      },
    })

    const { workflows, error, listWorkflows } = useWorkflows()
    await listWorkflows()

    expect(workflows.value).toEqual([])
    expect(error.value).toBeTruthy()
  })

  it('getWorkflow populates current workflow', async () => {
    const wfDetail = { id: 'wf-1', title: 'Full WF', steps: [{ name: 'step1' }] }

    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetWorkflow: vi.fn().mockResolvedValue(wfDetail),
        },
      },
    })

    const { current, getWorkflow } = useWorkflows()
    const result = await getWorkflow('wf-1')

    expect(result).toEqual(wfDetail)
    expect(current.value).toEqual(wfDetail)
  })

  it('getWorkflow returns null on failure', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          GetWorkflow: vi.fn().mockRejectedValue(new Error('Not found')),
        },
      },
    })

    const { current, getWorkflow } = useWorkflows()
    const result = await getWorkflow('wf-1')

    expect(result).toBe(null)
    expect(current.value).toBe(null)
  })

  it('saveWorkflow saves and updates local list', async () => {
    const savedWf = {
      id: 'wf-1', title: 'Updated WF', steps: [],
      createdAt: '2023', updatedAt: '2023',
    }

    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveWorkflow: vi.fn().mockResolvedValue(savedWf),
        },
      },
    })

    const { workflows, current, saving, saveWorkflow } = useWorkflows()
    workflows.value = [{ id: 'wf-1', title: 'Old', stepCount: 0 }]

    const result = await saveWorkflow({ id: 'wf-1', title: 'Updated WF' })

    expect(result).toEqual(savedWf)
    expect(current.value).toEqual(savedWf)
    expect(workflows.value[0].title).toBe('Updated WF')
    expect(workflows.value[0].stepCount).toBe(0)
    expect(saving.value).toBe(false)
  })

  it('saveWorkflow adds new workflow to start of list', async () => {
    const newWf = {
      id: 'wf-2', title: 'New WF', steps: [],
      createdAt: '2023', updatedAt: '2023',
    }

    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveWorkflow: vi.fn().mockResolvedValue(newWf),
        },
      },
    })

    const { workflows, saveWorkflow } = useWorkflows()
    workflows.value = [{ id: 'wf-1', title: 'Existing', stepCount: 1 }]

    await saveWorkflow({ id: 'wf-2', title: 'New WF' })

    expect(workflows.value[0]).toEqual({
      id: 'wf-2', title: 'New WF', stepCount: 0,
      createdAt: '2023', updatedAt: '2023',
    })
  })

  it('saveWorkflow returns null on failure', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          SaveWorkflow: vi.fn().mockRejectedValue(new Error('fail')),
        },
      },
    })

    const { saveWorkflow } = useWorkflows()
    const result = await saveWorkflow({ id: 'wf-2', title: 'New WF' })

    expect(result).toBe(null)
  })

  it('deleteWorkflow calls DeleteWorkflow and removes from list', async () => {
    const mockDelete = vi.fn().mockResolvedValue(undefined)

    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteWorkflow: mockDelete,
        },
      },
    })

    const result = useWorkflows()
    result.workflows.value = [{ id: 'wf-1' }, { id: 'wf-2' }]
    result.current.value = { id: 'wf-1' }

    await result.deleteWorkflow('wf-1')

    expect(mockDelete).toHaveBeenCalledWith('wf-1')
    expect(result.workflows.value).toEqual([{ id: 'wf-2' }])
    expect(result.current.value).toBe(null)
  })

  it('deleteWorkflow handles errors gracefully', async () => {
    vi.stubGlobal('go', {
      pkg: {
        App: {
          DeleteWorkflow: vi.fn().mockRejectedValue(new Error('Delete error')),
        },
      },
    })

    const { workflows, current, deleteWorkflow } = useWorkflows()
    workflows.value = [{ id: 'wf-1' }]
    current.value = { id: 'wf-1' }

    await deleteWorkflow('wf-1')

    // On error, workflows and current are NOT cleared (composable just sets error)
    expect(workflows.value).toEqual([{ id: 'wf-1' }])
    expect(current.value).toEqual({ id: 'wf-1' })
  })
})
