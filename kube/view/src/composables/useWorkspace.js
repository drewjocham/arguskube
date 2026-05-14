// Workspace composable — mirrors useDBAgent.js shape. Thin wrapper over
// the Pinia store so component code doesn't import the store directly.

import { storeToRefs } from 'pinia'
import { useWorkspaceStore } from '../stores/workspace'

export function useWorkspace() {
  const store = useWorkspaceStore()
  const { services, connections, loading, error } = storeToRefs(store)
  return {
    services,
    connections,
    loading,
    error,
    list: () => store.loadConnections(),
    listServices: () => store.loadServices(),
    connect: (service) => store.startConnect(service),
    disconnect: (id) => store.disconnect(id),
  }
}
