import { storeToRefs } from 'pinia'
import { useTerminalDispatchStore } from '../stores/terminalDispatch'

// Compatibility shim: existing components import `useTerminalDispatch` and
// destructure refs/actions. The actual state now lives in a Pinia store
// (stores/terminalDispatch.js) so events go through the same broker as the
// rest of the app's cross-component eventing. This shim preserves the
// composable's old surface to keep diffs minimal.
export function useTerminalDispatch() {
  const store = useTerminalDispatchStore()
  const { pendingCommand, openRequestId } = storeToRefs(store)
  return {
    pendingCommand,
    requestOpen: openRequestId,
    sendToTerminal: (cmd) => store.sendToTerminal(cmd),
    consumePendingCommand: () => store.consumePendingCommand(),
    peekPendingCommand: () => store.peekPendingCommand(),
  }
}
