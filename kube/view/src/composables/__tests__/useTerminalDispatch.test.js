import { describe, it, expect, beforeEach } from 'vitest'
import { useTerminalDispatch } from '../useTerminalDispatch'

describe('useTerminalDispatch', () => {
  // Drain any state left over from previous tests (the composable holds
  // module-level shared refs by design).
  beforeEach(() => {
    const { consumePendingCommand } = useTerminalDispatch()
    consumePendingCommand()
  })

  it('shares state across callers (writes via one are visible via the other)', () => {
    const a = useTerminalDispatch()
    const b = useTerminalDispatch()
    a.sendToTerminal('echo hi')
    expect(b.pendingCommand.value).not.toBeNull()
    expect(b.pendingCommand.value.text).toBe('echo hi')
    expect(b.requestOpen.value).toBe(a.requestOpen.value)
  })

  it('sendToTerminal sets pendingCommand and increments requestOpen', () => {
    const dispatch = useTerminalDispatch()
    const before = dispatch.requestOpen.value

    dispatch.sendToTerminal('kubectl get pods')

    expect(dispatch.pendingCommand.value).not.toBeNull()
    expect(dispatch.pendingCommand.value.text).toBe('kubectl get pods')
    expect(dispatch.requestOpen.value).toBe(before + 1)
  })

  it('sendToTerminal ignores empty / non-string input', () => {
    const dispatch = useTerminalDispatch()
    const before = dispatch.requestOpen.value

    dispatch.sendToTerminal('')
    dispatch.sendToTerminal(null)
    dispatch.sendToTerminal(undefined)
    dispatch.sendToTerminal(42)

    expect(dispatch.pendingCommand.value).toBeNull()
    expect(dispatch.requestOpen.value).toBe(before)
  })

  it('consumePendingCommand returns the value and clears the ref', () => {
    const dispatch = useTerminalDispatch()
    dispatch.sendToTerminal('echo hi')
    const consumed = dispatch.consumePendingCommand()
    expect(consumed.text).toBe('echo hi')
    expect(dispatch.pendingCommand.value).toBeNull()

    // A second consume returns null (already drained).
    expect(dispatch.consumePendingCommand()).toBeNull()
  })
})
