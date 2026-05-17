import { storeToRefs } from 'pinia'
import { useTerminalDispatchStore } from '../stores/terminalDispatch'
import type { PendingCommand, CommandMeta } from '../features/terminal/types'

export function useTerminalDispatch() {
  const store = useTerminalDispatchStore()
  const { pendingCommand, openRequestId } = storeToRefs(store)
  return {
    pendingCommand,
    requestOpen: openRequestId,
    sendToTerminal: (cmd: string, meta?: CommandMeta) => store.sendToTerminal(cmd, meta as Record<string, unknown> | undefined),
    consumePendingCommand: (): PendingCommand | null => store.consumePendingCommand(),
    peekPendingCommand: (): PendingCommand | null => store.peekPendingCommand(),
  }
}
