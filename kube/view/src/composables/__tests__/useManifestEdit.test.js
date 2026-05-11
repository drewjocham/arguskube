import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useManifestEdit } from '../useManifestEdit'

const mockCallGo = vi.fn()
vi.mock('../useBridge', () => ({
  callGo: (...args) => mockCallGo(...args),
}))

describe('useManifestEdit', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockCallGo.mockReset()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('exposes the expected reactive state and actions', () => {
    const m = useManifestEdit()
    expect(m).toMatchObject({
      manifestPopup: expect.any(Object),
      editingManifest: expect.any(Object),
      manifestContent: expect.any(Object),
      manifestKind: expect.any(Object),
      manifestName: expect.any(Object),
      manifestNamespace: expect.any(Object),
      manifestLoading: expect.any(Object),
      manifestApplying: expect.any(Object),
      manifestNotification: expect.any(Object),
      openManifest: expect.any(Function),
      closeManifest: expect.any(Function),
      toggleEditManifest: expect.any(Function),
      applyManifest: expect.any(Function),
    })
  })

  it('openManifest fetches yaml and populates state', async () => {
    mockCallGo.mockResolvedValueOnce('apiVersion: v1\nkind: Pod')
    const m = useManifestEdit()
    await m.openManifest({
      resourceType: 'pods',
      kind: 'Pod',
      namespace: 'default',
      name: 'web-1',
    })
    expect(mockCallGo).toHaveBeenCalledWith('GetResourceYaml', 'pods', 'default', 'web-1')
    expect(m.manifestPopup.value).toBe(true)
    expect(m.manifestContent.value).toBe('apiVersion: v1\nkind: Pod')
    expect(m.manifestKind.value).toBe('Pod')
    expect(m.manifestName.value).toBe('web-1')
    expect(m.manifestNamespace.value).toBe('default')
    expect(m.editingManifest.value).toBe(false)
    expect(m.manifestLoading.value).toBe(false)
  })

  it('openManifest writes an error message into manifestContent on failure', async () => {
    mockCallGo.mockRejectedValueOnce(new Error('boom'))
    const m = useManifestEdit()
    await m.openManifest({ resourceType: 'pods', kind: 'Pod', namespace: 'd', name: 'x' })
    expect(m.manifestContent.value).toContain('Error fetching manifest')
    expect(m.manifestContent.value).toContain('boom')
    expect(m.manifestLoading.value).toBe(false)
  })

  it('toggleEditManifest flips the editing flag', () => {
    const m = useManifestEdit()
    expect(m.editingManifest.value).toBe(false)
    m.toggleEditManifest()
    expect(m.editingManifest.value).toBe(true)
    m.toggleEditManifest()
    expect(m.editingManifest.value).toBe(false)
  })

  it('closeManifest resets popup state', async () => {
    mockCallGo.mockResolvedValueOnce('y: 1')
    const m = useManifestEdit()
    await m.openManifest({ resourceType: 'pods', kind: 'Pod', namespace: 'd', name: 'x' })
    m.editingManifest.value = true
    m.closeManifest()
    expect(m.manifestPopup.value).toBe(false)
    expect(m.editingManifest.value).toBe(false)
    expect(m.manifestContent.value).toBe('')
  })

  it('applyManifest sends ApplyYaml, calls onSuccess, and closes the popup on success', async () => {
    mockCallGo
      .mockResolvedValueOnce('y: 1')         // openManifest GetResourceYaml
      .mockResolvedValueOnce('Applied/Pod/x') // applyManifest ApplyYaml
    const m = useManifestEdit()
    await m.openManifest({ resourceType: 'pods', kind: 'Pod', namespace: 'd', name: 'x' })
    const onSuccess = vi.fn().mockResolvedValue()
    const result = await m.applyManifest(onSuccess)

    expect(result).toBe('Applied/Pod/x')
    expect(mockCallGo).toHaveBeenLastCalledWith('ApplyYaml', 'y: 1')
    expect(onSuccess).toHaveBeenCalled()
    expect(m.manifestPopup.value).toBe(false)
    expect(m.manifestApplying.value).toBe(false)
    expect(m.manifestNotification.value).toContain('Applied/Pod/x')
  })

  it('applyManifest does nothing when content is empty', async () => {
    const m = useManifestEdit()
    m.manifestContent.value = '   '
    const onSuccess = vi.fn()
    const result = await m.applyManifest(onSuccess)
    expect(result).toBeNull()
    expect(mockCallGo).not.toHaveBeenCalled()
    expect(onSuccess).not.toHaveBeenCalled()
  })

  it('applyManifest reports a failure notification when ApplyYaml rejects', async () => {
    mockCallGo
      .mockResolvedValueOnce('y: 1')
      .mockRejectedValueOnce(new Error('apply blew up'))
    const m = useManifestEdit()
    await m.openManifest({ resourceType: 'pods', kind: 'Pod', namespace: 'd', name: 'x' })
    const onSuccess = vi.fn()
    const result = await m.applyManifest(onSuccess)

    expect(result).toBeNull()
    expect(onSuccess).not.toHaveBeenCalled()
    expect(m.manifestNotification.value).toContain('Apply failed')
    expect(m.manifestNotification.value).toContain('apply blew up')
    expect(m.manifestPopup.value).toBe(true) // still open so the user can retry
  })

  it('manifestNotification clears after the timeout', async () => {
    mockCallGo
      .mockResolvedValueOnce('y: 1')
      .mockResolvedValueOnce('ok')
    const m = useManifestEdit()
    await m.openManifest({ resourceType: 'pods', kind: 'Pod', namespace: 'd', name: 'x' })
    await m.applyManifest()
    expect(m.manifestNotification.value).not.toBeNull()
    vi.advanceTimersByTime(5001)
    expect(m.manifestNotification.value).toBeNull()
  })
})
