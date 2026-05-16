/**
 * Barrel file — re-exports everything from domain-specific composables
 * for backward compatibility.
 *
 * ✅ All imports will resolve identically to before the refactor.
 * ✅ Tree-shaking compatible (bundlers only include what's used).
 * ✅ New code should import from domain files directly.
 *
 * Before (still works):  import { useMetrics, useAlerts } from './useWails'
 * After (preferred):     import { useMetrics } from './useMetrics'
 *                        import { useAlerts }  from './useAlerts'
 */

// Bridge primitives
export { callGo, cachedCallGo, isWails, DEFAULT_TTL, FAST_TTL, invalidateCache, invalidateCachePrefix, apiBase } from './useBridge'

// Cluster / app mode / topology
export { useAppMode, useClusterInfo, useContexts, useTopology } from './useCluster'

// Metrics and cost estimation
export { useMetrics, useTimeSeriesMetrics, useCostEstimate } from './useMetrics'

// Alerts and AI diagnostics
export { useAlerts, useDiagnostics } from './useAlerts'

// Resource browser
export { useResources } from './useResources'

// Logs (pod, stream, query, node)
export { usePodLogs, useLogStream, useLogs, useNodeLogs } from './useLogs'

// Terminal and pod exec
export { useTerminal, useTerminalSession, useTerminalCopilot, usePodExec } from './useShell'

// ArgusCD
export { useArgusCD, useApplications } from './useArgusCD'

// Data / CRUD composables
export { useFeatures, useChat, useNotebooks, useRunbooks, useIncidents, useWorkflows } from './useData'

// Monitoring / scanning
export { useArgusScan, useVulnerabilities } from './useMonitoring'

// Setup and anomaly
export { useSetup, useAnomaly } from './useSetup'

// Pods, deployment revisions, and VPA recommendations
export { usePods, useDeploymentRevisions, useVPARecommendations } from './usePods'

// Network
export { useServicePods } from './useNetwork'

// Misc
export { useCodeBlock } from './useMisc'
