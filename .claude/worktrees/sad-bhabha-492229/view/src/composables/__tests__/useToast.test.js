import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useToast } from '../useToast'

describe('useToast', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('returns toasts, addToast, removeToast', () => {
    const { toasts, addToast, removeToast } = useToast()
    expect(toasts.value).toEqual([])
    expect(typeof addToast).toBe('function')
    expect(typeof removeToast).toBe('function')
  })

  it('addToast adds a toast with the message', () => {
    const { toasts, addToast } = useToast()

    addToast('Operation succeeded')

    expect(toasts.value).toHaveLength(1)
    expect(toasts.value[0].message).toBe('Operation succeeded')
    expect(toasts.value[0].id).toBeDefined()
  })

  it('addToast adds multiple toasts', () => {
    const { toasts, addToast } = useToast()

    addToast('First toast')
    addToast('Second toast')

    expect(toasts.value).toHaveLength(2)
    expect(toasts.value[0].message).toBe('First toast')
    expect(toasts.value[1].message).toBe('Second toast')
  })

  it('addToast with duration auto-removes after the specified time', () => {
    const { toasts, addToast } = useToast()

    addToast('Auto-remove toast', 3000)

    expect(toasts.value).toHaveLength(1)

    vi.advanceTimersByTime(3000)

    expect(toasts.value).toHaveLength(0)
  })

  it('addToast with default duration auto-removes after 4000ms', () => {
    const { toasts, addToast } = useToast()

    addToast('Default duration toast')

    expect(toasts.value).toHaveLength(1)

    vi.advanceTimersByTime(4000)

    expect(toasts.value).toHaveLength(0)
  })

  it('addToast does not remove before duration expires', () => {
    const { toasts, addToast } = useToast()

    addToast('Persist toast', 5000)

    vi.advanceTimersByTime(3000)

    expect(toasts.value).toHaveLength(1)
  })

  it('removeToast removes the toast by id', () => {
    const { toasts, addToast, removeToast } = useToast()

    addToast('Remove me')
    const id = toasts.value[0].id

    removeToast(id)

    expect(toasts.value).toHaveLength(0)
  })

  it('removeToast only removes the specified toast', () => {
    const { toasts, addToast, removeToast } = useToast()

    addToast('Keep me')
    addToast('Remove me')
    const keepId = toasts.value[0].id
    const removeId = toasts.value[1].id

    removeToast(removeId)

    expect(toasts.value).toHaveLength(1)
    expect(toasts.value[0].id).toBe(keepId)
  })

  it('removeToast is safe for non-existent id', () => {
    const { toasts, addToast, removeToast } = useToast()

    addToast('Only toast')

    removeToast(99999)

    expect(toasts.value).toHaveLength(1)
  })
})
