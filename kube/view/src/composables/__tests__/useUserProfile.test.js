import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useUserProfile } from '../useUserProfile'

function stubFetch(payload, { ok = true, throws } = {}) {
  if (throws) {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(throws))
    return
  }
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok,
    json: vi.fn().mockResolvedValue({ result: payload }),
  }))
}

describe('useUserProfile', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    if (window.go) delete window.go
  })
  afterEach(() => { vi.restoreAllMocks() })

  it('pollSuggestion returns the suggestion when the backend has one', async () => {
    stubFetch({
      suggestion: {
        kind: 'pre-stage', title: 'Open alerts', body: 'Your usual first view.',
        actionLabel: 'Open', actionId: 'userprofile.open-view:alerts',
        muteKey: 'userprofile.morning:alerts', expiresInS: 60,
      },
    })
    const p = useUserProfile()
    const sg = await p.pollSuggestion('pods')
    expect(sg.title).toBe('Open alerts')
    expect(p.suggestion.value).toEqual(sg)
    expect(p.suppressed.value).toBe(false)
  })

  it('pollSuggestion returns null and clears state when backend reports nothing', async () => {
    stubFetch({})
    const p = useUserProfile()
    const sg = await p.pollSuggestion('pods')
    expect(sg).toBeNull()
    expect(p.suggestion.value).toBeNull()
  })

  it('pollSuggestion records the suppression reason without setting a suggestion', async () => {
    stubFetch({ suppressed: true, reason: 'budget-spent' })
    const p = useUserProfile()
    const sg = await p.pollSuggestion('pods')
    expect(sg).toBeNull()
    expect(p.suppressed.value).toBe(true)
    expect(p.suppressionReason.value).toBe('budget-spent')
  })

  it('pollSuggestion silently swallows network failures', async () => {
    stubFetch(null, { throws: new Error('boom') })
    const p = useUserProfile()
    const sg = await p.pollSuggestion('pods')
    expect(sg).toBeNull()
    expect(p.suggestion.value).toBeNull()
    expect(p.suppressed.value).toBe(false)
  })

  it('recordView swallows errors and never throws', async () => {
    stubFetch(null, { throws: new Error('boom') })
    const p = useUserProfile()
    expect(() => p.recordView('alerts', 'prod', '')).not.toThrow()
  })

  it('recordView is a no-op for empty view id', () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const p = useUserProfile()
    p.recordView('')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('mute clears the optimistic suggestion ref', () => {
    stubFetch({})
    const p = useUserProfile()
    p.suggestion.value = { muteKey: 'k', title: 't' }
    p.mute('k')
    expect(p.suggestion.value).toBeNull()
  })

  it('clearActivity returns true on success and false on failure', async () => {
    stubFetch({})
    const p = useUserProfile()
    expect(await p.clearActivity()).toBe(true)
    stubFetch(null, { throws: new Error('boom') })
    expect(await p.clearActivity()).toBe(false)
  })

  it('reset clears all state', () => {
    const p = useUserProfile()
    p.suggestion.value = { muteKey: 'k' }
    p.suppressed.value = true
    p.suppressionReason.value = 'budget-spent'
    p.reset()
    expect(p.suggestion.value).toBeNull()
    expect(p.suppressed.value).toBe(false)
    expect(p.suppressionReason.value).toBe('')
  })
})
