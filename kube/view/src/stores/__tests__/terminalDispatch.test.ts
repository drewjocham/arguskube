import { describe, it, expect } from 'vitest'
import { useTerminalDispatchStore } from '../terminalDispatch'

describe('terminalDispatch store', () => {
  it('starts with no pending command and openRequestId 0', () => {
    const s = useTerminalDispatchStore()
    expect(s.pendingCommand).toBeNull()
    expect(s.openRequestId).toBe(0)
  })

  it('sendToTerminal queues a command and bumps openRequestId', () => {
    const s = useTerminalDispatchStore()
    s.sendToTerminal('kubectl get pods')
    expect(s.pendingCommand).not.toBeNull()
    expect(s.pendingCommand!.text).toBe('kubectl get pods')
    expect(typeof s.pendingCommand!.requestedAt).toBe('number')
    expect(s.openRequestId).toBe(1)
  })

  it('sendToTerminal ignores empty / non-string input', () => {
    const s = useTerminalDispatchStore()
    s.sendToTerminal('')
    s.sendToTerminal(null as unknown as string)
    s.sendToTerminal(undefined as unknown as string)
    s.sendToTerminal(42 as unknown as string)
    s.sendToTerminal({ text: 'nope' } as unknown as string)
    expect(s.pendingCommand).toBeNull()
    expect(s.openRequestId).toBe(0)
  })

  it('peekPendingCommand returns the value without clearing it', () => {
    const s = useTerminalDispatchStore()
    s.sendToTerminal('ls')
    const peeked = s.peekPendingCommand()
    expect(peeked!.text).toBe('ls')
    expect(s.pendingCommand).not.toBeNull()
  })

  it('consumePendingCommand returns the value and clears it', () => {
    const s = useTerminalDispatchStore()
    s.sendToTerminal('ls')
    const consumed = s.consumePendingCommand()
    expect(consumed!.text).toBe('ls')
    expect(s.pendingCommand).toBeNull()
    expect(s.consumePendingCommand()).toBeNull()
  })

  it('openRequestId is monotonically increasing across sends', () => {
    const s = useTerminalDispatchStore()
    s.sendToTerminal('a')
    s.sendToTerminal('b')
    s.sendToTerminal('c')
    expect(s.openRequestId).toBe(3)
  })

  it('forwards optional meta (sessionId, sectionLabel)', () => {
    const s = useTerminalDispatchStore()
    s.sendToTerminal('kubectl get pods', { sessionId: 'rb-1::verify', sectionLabel: 'Verify pods' })
    expect(s.pendingCommand!.text).toBe('kubectl get pods')
    expect(s.pendingCommand!.sessionId).toBe('rb-1::verify')
    expect(s.pendingCommand!.sectionLabel).toBe('Verify pods')
  })

  it('ignores non-object meta gracefully', () => {
    const s = useTerminalDispatchStore()
    s.sendToTerminal('cmd', 'not an object' as never)
    expect(s.pendingCommand!.text).toBe('cmd')
    expect(s.pendingCommand!.sessionId).toBeUndefined()
  })
})
