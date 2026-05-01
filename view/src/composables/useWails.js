import { ref, onMounted, onUnmounted } from 'vue'

/**
 * Bridge to Wails Go bindings.
 * In dev mode (no window.go), falls back to mock data.
 */
export const isWails = () => typeof window !== 'undefined' && window.go

/**
 * Calls a Go method via Wails bindings or falls back to an HTTP API for SaaS mode.
 * @param {string} method - Method name on the App struct (e.g., 'GetAlerts')
 * @param  {...any} args - Arguments to pass
 */
export async function callGo(method, ...args) {
  if (isWails()) {
    try {
      return await window.go.api.pkg.App[method](...args)
    } catch (err) {
      console.error(`[wails] ${method} failed:`, err)
      throw err
    }
  }

  // Fallback to REST API for SaaS mode
  try {
    const res = await fetch(`http://localhost:8080/api/${method}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ args })
    })
    
    if (!res.ok) {
      throw new Error(`HTTP error! status: ${res.status}`)
    }
    
    const data = await res.json()
    if (data.error) {
      throw new Error(data.error)
    }
    return data.result
  } catch (err) {
    console.warn(`[saas-api] ${method} fallback failed (is the backend running on :8080?):`, err)
    // We let the components fall back to their local mock variables on failure
    throw err
  }
}

/**
 * Composable for App Mode (e.g., 'dashboard' or 'terminal')
 */
export function useAppMode() {
  const mode = ref('dashboard')
  
  async function fetch() {
    try {
      const res = await callGo('GetAppMode')
      if (res) mode.value = res
    } catch (e) {
      console.warn('[app-mode] failed to fetch mode, using dashboard', e)
    }
  }

  onMounted(fetch)
  return { mode }
}

/**
 * Composable for cluster info.
 */
export function useClusterInfo() {
  const info = ref(null)
  const loading = ref(true)
  const error = ref(null)

  async function fetch() {
    loading.value = true
    try {
      info.value = await callGo('GetClusterInfo')
    } catch (e) {
      error.value = e
    } finally {
      loading.value = false
    }
  }

  onMounted(fetch)
  return { info, loading, error, refresh: fetch }
}

/**
 * Composable for kubeconfig context listing and switching.
 */
export function useContexts() {
  const contexts = ref([])
  const loading = ref(false)
  const switching = ref(false)
  const error = ref(null)

  const mockContexts = [
    { name: 'k3s-local', cluster: 'k3s-local', active: true },
    { name: 'staging-eks', cluster: 'staging-eks', active: false },
    { name: 'production-gke', cluster: 'production-gke', active: false },
  ]

  async function listContexts() {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ListContexts')
      contexts.value = result && result.length > 0 ? result : mockContexts
    } catch (e) {
      error.value = e?.message || String(e)
      contexts.value = mockContexts
    } finally {
      loading.value = false
    }
  }

  async function switchContext(name) {
    switching.value = true
    error.value = null
    try {
      await callGo('SwitchContext', name)
      // Mark the new active context locally.
      contexts.value = contexts.value.map(c => ({ ...c, active: c.name === name }))
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      switching.value = false
    }
  }

  return { contexts, loading, switching, error, listContexts, switchContext }
}

/**
 * Composable for cluster metrics with auto-refresh.
 */
export function useMetrics(intervalMs = 5000) {
  const metrics = ref(null)
  const loading = ref(true)
  let timer = null

  async function fetch() {
    try {
      metrics.value = await callGo('GetMetrics')
    } catch (e) {
      console.error('[metrics]', e)
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    fetch()
    timer = setInterval(fetch, intervalMs)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  async function queryMetrics(query, timeRange) {
    try {
      return await callGo('QueryTimeSeriesMetrics', query, timeRange)
    } catch (e) {
      console.error('[metrics] queryTimeSeries:', e)
      return null
    }
  }

  return { metrics, loading, refresh: fetch, queryMetrics }
}

/**
 * Composable for time-series metric queries (no polling, on-demand only).
 */
export function useTimeSeriesMetrics() {
  async function queryMetrics(query, timeRange) {
    try {
      return await callGo('QueryTimeSeriesMetrics', query, timeRange)
    } catch (e) {
      console.error('[metrics] queryTimeSeries:', e)
      return null
    }
  }
  return { queryMetrics }
}

/**
 * Composable for alerts with auto-refresh.
 */
export function useAlerts(intervalMs = 5000) {
  const alerts = ref([])
  const loading = ref(true)
  let timer = null

  async function fetch() {
    try {
      const result = await callGo('GetAlerts')
      if (result) alerts.value = result
    } catch (e) {
      console.error('[alerts]', e)
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    fetch()
    timer = setInterval(fetch, intervalMs)
  })

  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })

  return { alerts, loading, refresh: fetch }
}

/**
 * Composable for AI diagnostics.
 */
export function useDiagnostics() {
  const bundle = ref(null)
  const loading = ref(false)
  const error = ref(null)

  async function diagnose(alertId) {
    loading.value = true
    error.value = null
    try {
      bundle.value = await callGo('DiagnoseAlert', alertId)
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  return { bundle, loading, error, diagnose }
}

/**
 * Composable for feature gate checks.
 */
export function useFeatures() {
  const features = ref({})
  const tier = ref('free')

  async function fetch() {
    try {
      features.value = (await callGo('GetFeatures')) || {}
      tier.value = (await callGo('GetTier')) || 'free'
    } catch (e) {
      console.error('[features]', e)
    }
  }

  onMounted(fetch)

  function isAllowed(feature) {
    return features.value[feature] === true
  }

  return { features, tier, isAllowed, refresh: fetch }
}

/**
 * Composable for pod logs.
 */
export function usePodLogs() {
  const logs = ref([])
  const loading = ref(false)

  async function fetch(namespace, podName, tailLines = 50) {
    loading.value = true
    try {
      const result = await callGo('GetPodLogs', namespace, podName, tailLines)
      if (result) logs.value = result
    } catch (e) {
      console.error('[logs]', e)
    } finally {
      loading.value = false
    }
  }

  return { logs, loading, fetch }
}

/**
 * Composable for AI agent chat.
 */
export function useChat() {
  const history = ref([])
  const sending = ref(false)
  const autoSummary = ref(null)
  const eventLog = ref([])

  async function sendMessage(alertId, message) {
    sending.value = true
    try {
      const response = await callGo('SendChatMessage', alertId, message)
      // Refresh history after send.
      await refreshHistory(alertId)
      return response
    } catch (e) {
      console.error('[chat]', e)
      throw e
    } finally {
      sending.value = false
    }
  }

  async function refreshHistory(alertId) {
    try {
      const result = await callGo('GetChatHistory', alertId)
      if (result) history.value = result
    } catch (e) {
      console.error('[chat history]', e)
    }
  }

  async function fetchAutoSummary(alertId) {
    try {
      autoSummary.value = await callGo('GetAutoSummary', alertId)
    } catch (e) {
      console.error('[auto-summary]', e)
    }
  }

  async function fetchEventLog() {
    try {
      const result = await callGo('GetAgentEventLog')
      if (result) eventLog.value = result
    } catch (e) {
      console.error('[event-log]', e)
    }
  }

  return { history, sending, autoSummary, eventLog, sendMessage, refreshHistory, fetchAutoSummary, fetchEventLog }
}

/**
 * Composable for Popeye cluster scan.
 */
export function usePopeye() {
  const report = ref(null)
  const loading = ref(false)
  const error = ref(null)

  const mockReport = {
    timestamp: new Date().toISOString(),
    score: 72, grade: 'C', clusterName: 'demo-cluster', scanTimeMs: 1240,
    totalOk: 48, totalInfo: 12, totalWarn: 9, totalError: 5,
    findings: [
      { id: 'pop-1', resource: 'pod', name: 'web-app-6d8f9b-xp2kl', namespace: 'default', severity: 'error', sevLevel: 3, message: '[POP-106] No resources requests/limits defined', explanation: 'This container has no CPU or memory resource requests/limits set. Without resource definitions, the scheduler cannot make informed placement decisions.', fix: 'Add resource requests and limits. Start with requests based on observed usage and set limits at 1.5-2x requests.', command: 'kubectl edit pod web-app-6d8f9b-xp2kl -n default' },
      { id: 'pop-2', resource: 'pod', name: 'worker-7c4b2f-m9z3q', namespace: 'default', severity: 'error', sevLevel: 3, message: '[POP-301] No probes defined', explanation: 'This container has no liveness or readiness probes. Kubernetes cannot detect if the application is healthy or ready.', fix: 'Add liveness and readiness probes. Use httpGet for HTTP services or tcpSocket for TCP.', command: 'kubectl edit pod worker-7c4b2f-m9z3q -n default' },
      { id: 'pop-3', resource: 'deploy', name: 'api-gateway', namespace: 'ingress', severity: 'error', sevLevel: 3, message: '[POP-107] Container uses image tag \':latest\'', explanation: 'Using :latest makes deployments non-reproducible. Different nodes may pull different versions.', fix: 'Pin the image to a specific version tag or SHA digest.', command: 'kubectl get deploy api-gateway -n ingress -o jsonpath=\'{.spec.template.spec.containers[*].image}\'' },
      { id: 'pop-4', resource: 'pod', name: 'cache-redis-0', namespace: 'data', severity: 'warning', sevLevel: 2, message: '[POP-108] CPU limit not set', explanation: 'Without CPU limits, a single pod can monopolize node CPU, causing throttling for colocated workloads.', fix: 'Set appropriate CPU limits to ensure fair scheduling.', command: 'kubectl edit pod cache-redis-0 -n data' },
      { id: 'pop-5', resource: 'deploy', name: 'payment-service', namespace: 'finance', severity: 'warning', sevLevel: 2, message: '[POP-500] Single replica detected', explanation: 'Running with only one replica creates a single point of failure.', fix: 'Increase replica count to at least 2 for production workloads. Configure a PodDisruptionBudget.', command: 'kubectl scale deploy payment-service -n finance --replicas=2' },
      { id: 'pop-6', resource: 'pod', name: 'debug-shell-9x2f1', namespace: 'default', severity: 'error', sevLevel: 3, message: '[POP-306] Container runs as root', explanation: 'Running as root inside a container increases the blast radius of a container escape.', fix: 'Set runAsNonRoot: true, readOnlyRootFilesystem: true, and allowPrivilegeEscalation: false.', command: 'kubectl get pod debug-shell-9x2f1 -n default -o jsonpath=\'{.spec.template.spec.securityContext}\'' },
      { id: 'pop-7', resource: 'sa', name: 'default', namespace: 'default', severity: 'warning', sevLevel: 2, message: '[POP-303] Pod uses default service account', explanation: 'Using the default service account may have broader permissions than needed.', fix: 'Create a dedicated service account with minimal RBAC permissions.', command: 'kubectl describe sa default -n default' },
      { id: 'pop-8', resource: 'secret', name: 'old-tls-cert', namespace: 'kube-system', severity: 'warning', sevLevel: 2, message: '[POP-800] Secret appears unused', explanation: 'This resource exists but is not referenced by any workload. Unused resources add clutter and may hold exploitable secrets.', fix: 'Clean up resources that are no longer needed.', command: 'kubectl delete secret old-tls-cert -n kube-system' },
      { id: 'pop-9', resource: 'svc', name: 'api-gateway', namespace: 'ingress', severity: 'info', sevLevel: 1, message: '[POP-700] Found no matching endpoints', explanation: 'This service resource passed the check or has a minor informational note.', fix: 'Review the Popeye documentation for detailed remediation guidance.', command: 'kubectl describe svc api-gateway -n ingress' },
    ]
  }

  async function runScan() {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('RunPopeye')
      report.value = result || mockReport
    } catch (e) {
      console.warn('[popeye] backend unavailable, using demo report:', e)
      report.value = mockReport
    } finally {
      loading.value = false
    }
  }

  return { report, loading, error, runScan }
}

/**
 * Composable for the resource browser.
 */
export function useResources() {
  const result = ref(null)
  const detail = ref(null)
  const namespaces = ref([])
  const loading = ref(false)
  const detailLoading = ref(false)
  const error = ref(null)

  async function listResources(kind, namespace) {
    loading.value = true
    error.value = null
    try {
      // Default to '_all' (all namespaces) when no namespace is specified.
      result.value = await callGo('ListResources', kind, namespace || '_all')
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  async function getResourceDetail(kind, namespace, name) {
    detailLoading.value = true
    try {
      detail.value = await callGo('GetResourceDetail', kind, namespace || '', name)
    } catch (e) {
      console.error('[resource-detail]', e)
    } finally {
      detailLoading.value = false
    }
  }

  async function listNamespaces() {
    try {
      const result = await callGo('ListAllNamespaces')
      if (result) namespaces.value = result
    } catch (e) {
      console.error('[namespaces]', e)
    }
  }

  return { result, detail, namespaces, loading, detailLoading, error, listResources, getResourceDetail, listNamespaces }
}

/**
 * Composable for the embedded terminal.
 */
export function useTerminal() {
  async function startTerminal(rows, cols) {
    try {
      await callGo('StartTerminal', rows, cols)
    } catch (e) {
      console.error('[terminal]', e)
    }
  }

  async function sendInput(data) {
    try {
      await callGo('SendTerminalInput', data)
    } catch (e) {
      console.error('[terminal-input]', e)
    }
  }

  async function resizeTerminal(rows, cols) {
    try {
      await callGo('ResizeTerminal', rows, cols)
    } catch (e) {
      console.error('[terminal-resize]', e)
    }
  }

  return { startTerminal, sendInput, resizeTerminal }
}

/**
 * Mock notebook data for dev mode (no Go backend).
 */
const mockNotebookFiles = [
  { id: 'incidents', name: 'Incidents', path: 'incidents', type: 'folder', children: [
    { id: 'incidents/2024-03-01_db_outage', name: '2024-03-01_db_outage.md', path: 'incidents/2024-03-01_db_outage.md', type: 'file', modified: '2024-03-01' },
    { id: 'incidents/post_mortems', name: 'post_mortems.md', path: 'incidents/post_mortems.md', type: 'file', modified: '2024-02-15' },
  ]},
  { id: 'runbook-docs', name: 'Runbooks', path: 'runbook-docs', type: 'folder', children: [
    { id: 'runbook-docs/redis_oom', name: 'redis_oom.md', path: 'runbook-docs/redis_oom.md', type: 'file', modified: '2024-01-20' },
  ]},
  { id: 'getting_started', name: 'getting_started.md', path: 'getting_started.md', type: 'file', modified: '2024-03-10' },
  { id: 'architecture', name: 'architecture.md', path: 'architecture.md', type: 'file', modified: '2024-03-08' },
]

const mockNotebookContents = {
  'getting_started.md': `# KubeWatcher Knowledge Base\n\nWelcome to your connected notebook.\n\nThis works like **Obsidian** or **Notion**. All files you create here are backed by the configured S3 bucket or stored locally. You can use this space to store incident post-mortems, custom runbook documentation, and team notes.\n\n## Quick Start\n1. Click the **+** icon to create a new file.\n2. Create folders to organize your knowledge.\n3. Everything auto-saves!\n\n## Features\n- **Rich Markdown editing** with syntax highlighting\n- **S3 sync** for team-wide access\n- **Local cache** for offline use`,
  'architecture.md': `# Architecture\n\nOur SaaS runs in a hybrid model.\n\n## Components\n- **Local Desktop:** Stores config locally, connects to cluster\n- **Agent Pod:** In-cluster DaemonSet for edge ML inference\n- **S3 Notebooks:** Persistent markdown storage\n\n## Data Flow\n1. Desktop client connects to cluster via kubeconfig\n2. Agent scrapes metrics from Kubelet and Metrics Server\n3. Desktop port-forwards to agent pod for live data`,
  'incidents/2024-03-01_db_outage.md': `# DB Outage — March 1, 2024\n\n## Summary\nThe primary PostgreSQL replica went OOM at 03:42 UTC causing a 12-minute read outage.\n\n## Timeline\n- **03:42** — Alert fires: \`db-primary-oom\`\n- **03:44** — On-call acknowledges\n- **03:48** — Root cause identified: batch job consumed 8GB\n- **03:54** — Pod restarted, reads restored\n\n## Root Cause\nA nightly analytics batch job ran without memory limits.`,
  'incidents/post_mortems.md': `# Post-Mortem Template\n\n## Incident Summary\n_What happened?_\n\n## Impact\n_Who was affected and for how long?_\n\n## Timeline\n_Chronological events_\n\n## Root Cause\n_Why did this happen?_\n\n## Action Items\n1. _Fix to prevent recurrence_\n2. _Monitoring improvements_`,
  'runbook-docs/redis_oom.md': `# Redis OOM Runbook\n\n## Trigger\nAlert: \`redis-oom-killed\`\n\n## Steps\n1. **Check memory usage:** \`kubectl top pod -l app=redis\`\n2. **Review eviction policy:** \`redis-cli CONFIG GET maxmemory-policy\`\n3. **Flush stale keys:** \`redis-cli --scan --pattern 'cache:expired:*' | xargs redis-cli DEL\`\n4. **Increase limits if needed:** Update the StatefulSet resource limits\n5. **Verify:** Confirm memory usage stabilized`,
}

/**
 * Composable for S3-backed notebooks (with dev-mode mock fallback).
 */
export function useNotebooks() {
  const files = ref([])
  const loading = ref(false)
  const saving = ref(false)
  const synced = ref(false)
  const error = ref(null)

  // In-memory content store for dev mode.
  const localContents = ref({ ...mockNotebookContents })

  async function listFiles() {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ListNotebooks')
      if (result && result.length > 0) {
        files.value = result
      } else {
        // Backend returned empty or null — use mock data.
        files.value = mockNotebookFiles
      }
      synced.value = true
    } catch (e) {
      error.value = e?.message || String(e)
      files.value = mockNotebookFiles
    } finally {
      loading.value = false
    }
  }

  async function getFile(path) {
    try {
      const result = await callGo('GetNotebook', path)
      if (result != null) return result
    } catch (e) {
      // Fall through to local.
    }
    // Local / mock fallback.
    return localContents.value[path] || ''
  }

  async function saveFile(path, content) {
    saving.value = true
    localContents.value[path] = content
    try {
      await callGo('SaveNotebook', path, content)
      synced.value = true
    } catch (e) {
      // Still saved locally in-memory.
      synced.value = false
    } finally {
      saving.value = false
    }
  }

  async function deleteFile(path) {
    delete localContents.value[path]
    try {
      await callGo('DeleteNotebook', path)
    } catch (e) {
      console.error('[notebooks] deleteFile failed:', e)
    }
    // Rebuild file list: remove the deleted entry.
    files.value = removeFromTree(files.value, path)
  }

  async function createFolder(path) {
    try {
      await callGo('CreateNotebookFolder', path)
    } catch (e) {
      // Dev mode — add folder to local tree.
    }
    // Add folder to tree if not already present.
    const exists = files.value.some(f => f.path === path)
    if (!exists) {
      files.value = [...files.value, { id: path, name: path, path, type: 'folder', children: [] }]
    }
  }

  async function testConnection() {
    try {
      await callGo('TestS3Connection')
      return { ok: true }
    } catch (e) {
      return { ok: false, error: e?.message || String(e) }
    }
  }

  /**
   * Add a new file entry to the tree (for create-file in dev mode).
   */
  function addFileToTree(path, name) {
    const parts = path.split('/')
    if (parts.length === 1) {
      // Root-level file.
      const exists = files.value.some(f => f.path === path)
      if (!exists) {
        files.value = [...files.value, { id: path.replace('.md', ''), name, path, type: 'file', modified: new Date().toISOString() }]
      }
    } else {
      // Nested — just refresh the whole list.
      listFiles()
    }
  }

  async function moveFile(oldPath, newPath) {
    // Update local content store.
    if (localContents.value[oldPath]) {
      localContents.value[newPath] = localContents.value[oldPath]
      delete localContents.value[oldPath]
    }
    try {
      await callGo('MoveNotebook', oldPath, newPath)
    } catch (e) {
      // Dev mode — just update the tree locally.
    }
    // Rebuild tree: remove from old location, add to new.
    files.value = removeFromTree(files.value, oldPath)
    const fileName = newPath.split('/').pop()
    addFileToTree(newPath, fileName)
  }

  return { files, loading, saving, synced, error, listFiles, getFile, saveFile, deleteFile, createFolder, testConnection, addFileToTree, moveFile }
}

/**
 * Remove a file from a nested tree by path.
 */
function removeFromTree(tree, path) {
  return tree
    .filter(item => item.path !== path)
    .map(item => {
      if (item.children) {
        return { ...item, children: removeFromTree(item.children, path) }
      }
      return item
    })
}

/**
 * Mock runbook data for dev mode.
 */
const mockRunbooks = [
  { id: 'oomkilled-response', name: 'OOMKilled Response', trigger: 'OOMKilled alert', status: 'ready', steps: 4, lastRun: '2h ago', path: 'oomkilled-response.md' },
  { id: 'crashloop-triage', name: 'CrashLoop Triage', trigger: 'CrashLoopBackOff', status: 'ready', steps: 5, lastRun: '6h ago', path: 'crashloop-triage.md' },
  { id: 'node-pressure-remediation', name: 'Node Pressure Remediation', trigger: 'DiskPressure / MemoryPressure', status: 'ready', steps: 6, lastRun: '1d ago', path: 'node-pressure-remediation.md' },
  { id: 'deploy-rollback', name: 'Deploy Rollback', trigger: 'Manual / Error rate spike', status: 'draft', steps: 3, lastRun: 'Never', path: 'deploy-rollback.md' },
  { id: 'certificate-renewal', name: 'Certificate Renewal', trigger: 'cert-manager warning', status: 'ready', steps: 4, lastRun: '14d ago', path: 'certificate-renewal.md' },
]

const mockRunbookContents = {
  'oomkilled-response': `---\nname: OOMKilled Response\ntrigger: OOMKilled alert\nstatus: ready\n---\n\n# OOMKilled Response\n\n## Trigger\nOOMKilled alert fires on any pod.\n\n## Steps\n1. **Identify the pod:** \`kubectl get pods --field-selector=status.phase=Failed\`\n2. **Check memory usage:** \`kubectl top pod <name>\`\n3. **Review limits:** \`kubectl describe pod <name> | grep -A5 Limits\`\n4. **Increase limits or fix the leak:** Update the Deployment resource spec\n\n## Notes\nCheck if the OOM is a one-time spike or a memory leak pattern.`,
  'crashloop-triage': `---\nname: CrashLoop Triage\ntrigger: CrashLoopBackOff\nstatus: ready\n---\n\n# CrashLoop Triage\n\n## Trigger\nPod enters CrashLoopBackOff state.\n\n## Steps\n1. **Get pod status:** \`kubectl describe pod <name>\`\n2. **Check logs:** \`kubectl logs <name> --previous\`\n3. **Check events:** \`kubectl get events --sort-by=.lastTimestamp\`\n4. **Check image:** Verify the image tag is correct and pullable\n5. **Restart or rollback:** \`kubectl rollout undo deployment/<name>\``,
  'node-pressure-remediation': `---\nname: Node Pressure Remediation\ntrigger: DiskPressure / MemoryPressure\nstatus: ready\n---\n\n# Node Pressure Remediation\n\n## Trigger\nNode condition shows DiskPressure or MemoryPressure.\n\n## Steps\n1. **Identify node:** \`kubectl get nodes\` — look for NotReady or pressure conditions\n2. **Check disk usage:** \`kubectl describe node <name> | grep -A10 Conditions\`\n3. **Evict non-critical pods:** \`kubectl drain <node> --ignore-daemonsets\`\n4. **Clean up images:** \`docker system prune\` on the node\n5. **Clean up logs:** Rotate and compress old container logs\n6. **Uncordon:** \`kubectl uncordon <node>\``,
  'deploy-rollback': `---\nname: Deploy Rollback\ntrigger: Manual / Error rate spike\nstatus: draft\n---\n\n# Deploy Rollback\n\n## Trigger\nManual trigger or automated error rate spike detection.\n\n## Steps\n1. **Check rollout history:** \`kubectl rollout history deployment/<name>\`\n2. **Rollback:** \`kubectl rollout undo deployment/<name>\`\n3. **Verify:** \`kubectl rollout status deployment/<name>\``,
  'certificate-renewal': `---\nname: Certificate Renewal\ntrigger: cert-manager warning\nstatus: ready\n---\n\n# Certificate Renewal\n\n## Trigger\ncert-manager warning about expiring certificate.\n\n## Steps\n1. **Check certificate status:** \`kubectl get certificates\`\n2. **Check cert-manager logs:** \`kubectl logs -n cert-manager deploy/cert-manager\`\n3. **Force renewal:** \`kubectl delete secret <tls-secret>\`\n4. **Verify new cert:** \`kubectl describe certificate <name>\``,
}

/**
 * Composable for runbook CRUD.
 */
export function useRunbooks() {
  const runbooks = ref([])
  const loading = ref(false)
  const saving = ref(false)
  const error = ref(null)
  const localContents = ref({ ...mockRunbookContents })

  async function listRunbooks() {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ListRunbooks')
      if (result && result.length > 0) {
        runbooks.value = result
      } else {
        runbooks.value = mockRunbooks
      }
    } catch (e) {
      error.value = e?.message || String(e)
      runbooks.value = mockRunbooks
    } finally {
      loading.value = false
    }
  }

  async function getRunbook(id) {
    try {
      const result = await callGo('GetRunbook', id)
      if (result != null) return result
    } catch (e) {
      // Fall through to local.
    }
    return localContents.value[id] || `# ${id}\n\n_No content yet._`
  }

  async function saveRunbook(id, content) {
    saving.value = true
    localContents.value[id] = content
    try {
      await callGo('SaveRunbook', id, content)
    } catch (e) {
      console.error('[runbooks] saveRunbook failed:', e)
    } finally {
      saving.value = false
    }
  }

  async function deleteRunbook(id) {
    delete localContents.value[id]
    try {
      await callGo('DeleteRunbook', id)
    } catch (e) {
      // Dev mode — just remove from local list.
    }
    runbooks.value = runbooks.value.filter(rb => rb.id !== id)
  }

  async function createRunbook(name, trigger) {
    let rb = null
    try {
      rb = await callGo('CreateRunbook', name, trigger)
    } catch (e) {
      // Dev mode fallback.
    }

    if (!rb) {
      // Build a local mock runbook.
      const id = name.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
      rb = { id, name, trigger: trigger || '', status: 'draft', steps: 4, lastRun: 'Never', path: id + '.md' }
      const content = `---\nname: ${name}\ntrigger: ${trigger || ''}\nstatus: draft\n---\n\n# ${name}\n\n## Trigger\n${trigger || '_Define trigger_'}\n\n## Steps\n1. **Assess** — Verify the alert is legitimate.\n2. **Investigate** — Check logs and metrics.\n3. **Remediate** — Apply the fix.\n4. **Verify** — Confirm resolution.\n\n## Notes\n_Add notes here._`
      localContents.value[id] = content
    }

    // Add to the list.
    runbooks.value = [...runbooks.value, rb]
    return rb
  }

  return { runbooks, loading, saving, error, listRunbooks, getRunbook, saveRunbook, deleteRunbook, createRunbook }
}

/**
 * Composable for one-click tool setup.
 */
export function useSetup() {
  const tools = ref([])
  const loading = ref(false)
  const actionLoading = ref(null) // which tool is being installed

  const mockTools = [
    { name: 'kubectl', installed: true, version: 'v1.30.2', via: 'binary', message: 'kubectl available' },
    { name: 'docker', installed: true, version: '27.0.3', via: 'binary', message: 'Docker available' },
    { name: 'helm', installed: false, message: 'Helm not found' },
    { name: 'popeye', installed: false, message: 'Popeye not found. Install via binary or Docker.' },
    { name: 'kubewatcher-agent', installed: false, message: 'Agent not deployed to cluster' },
  ]

  async function checkTools() {
    loading.value = true
    try {
      const result = await callGo('CheckToolStatus')
      tools.value = result && result.length > 0 ? result : mockTools
    } catch (e) {
      console.error('[setup]', e)
      tools.value = mockTools
    } finally {
      loading.value = false
    }
  }

  async function installPopeye() {
    actionLoading.value = 'popeye'
    try {
      const result = await callGo('InstallPopeye')
      await checkTools()
      return result
    } catch (e) {
      console.error('[setup] installPopeye:', e)
      return { success: false, message: e?.message || String(e) }
    } finally {
      actionLoading.value = null
    }
  }

  async function deployAgent(namespace) {
    actionLoading.value = 'kubewatcher-agent'
    try {
      const result = await callGo('DeployAgent', namespace || 'kubewatcher')
      await checkTools()
      return result
    } catch (e) {
      console.error('[setup] deployAgent:', e)
      return { success: false, message: e?.message || String(e) }
    } finally {
      actionLoading.value = null
    }
  }

  async function undeployAgent(namespace) {
    actionLoading.value = 'kubewatcher-agent'
    try {
      const result = await callGo('UndeployAgent', namespace || 'kubewatcher')
      await checkTools()
      return result
    } catch (e) {
      console.error('[setup] undeployAgent:', e)
      return { success: false, message: e?.message || String(e) }
    } finally {
      actionLoading.value = null
    }
  }

  return { tools, loading, actionLoading, checkTools, installPopeye, deployAgent, undeployAgent }
}

/**
 * Composable for Anomaly Agent connectivity.
 */
export function useAnomaly() {
  const anomalies = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function connectAgent(namespace = 'all') {
    loading.value = true
    error.value = null
    try {
      anomalies.value = await callGo('ConnectToAgent', namespace)
    } catch (e) {
      error.value = e.message || 'Failed to connect to agent'
    } finally {
      loading.value = false
    }
  }

  return { anomalies, loading, error, connectAgent }
}

/**
 * Composable for log querying.
 */
export function useLogs() {
  const entries = ref([])
  const histogram = ref([])
  const fields = ref([])
  const total = ref(0)
  const loading = ref(false)
  const queryTime = ref(0)
  const error = ref(null)

  async function queryLogs(query = '*', namespace = '', limit = 100) {
    loading.value = true
    error.value = null
    const start = performance.now()
    try {
      const result = await callGo('QueryLogs', query, namespace, limit)
      queryTime.value = Math.round(performance.now() - start)
      if (result) {
        entries.value = result.entries || []
        histogram.value = result.histogram || []
        fields.value = result.fields || []
        total.value = result.total || 0
      }
    } catch (e) {
      error.value = e?.message || String(e)
      queryTime.value = Math.round(performance.now() - start)
    } finally {
      loading.value = false
    }
  }

  return { entries, histogram, fields, total, loading, queryTime, error, queryLogs }
}

/**
 * Composable for incident CRUD.
 */
export function useIncidents() {
  const incidents = ref([])
  const loading = ref(false)
  const error = ref(null)

  const mockIncidents = [
    { id: 'inc-1', title: 'OOMKilled: payment-api', severity: 'critical', status: 'resolved', type: 'alert', description: 'Payment API pod OOMKilled at 03:42 UTC', namespace: 'finance', createdAt: new Date(Date.now() - 3600000).toISOString(), updatedAt: new Date().toISOString() },
    { id: 'inc-2', title: 'High memory usage on node-3', severity: 'warning', status: 'investigating', type: 'investigation', description: 'Node showing MemoryPressure condition', namespace: 'infra', createdAt: new Date(Date.now() - 7200000).toISOString(), updatedAt: new Date().toISOString() },
    { id: 'inc-3', title: 'CrashLoopBackOff: worker', severity: 'critical', status: 'open', type: 'alert', description: 'Worker pod crashing after latest deploy', namespace: 'default', createdAt: new Date(Date.now() - 1800000).toISOString(), updatedAt: new Date().toISOString() },
  ]

  async function listIncidents() {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ListIncidents')
      incidents.value = result && result.length > 0 ? result : mockIncidents
    } catch (e) {
      error.value = e?.message || String(e)
      incidents.value = mockIncidents
    } finally {
      loading.value = false
    }
  }

  async function createIncident(title, severity, type, description, namespace) {
    try {
      const inc = await callGo('CreateIncident', title, severity || 'info', type || 'alert', description || '', namespace || '')
      if (inc) {
        incidents.value = [inc, ...incidents.value]
        return inc
      }
    } catch (e) {
      console.error('[incidents] create:', e)
    }
    // Dev mode fallback.
    const mock = { id: 'inc-' + Date.now(), title, severity: severity || 'info', status: 'open', type: type || 'alert', description: description || '', namespace: namespace || '', createdAt: new Date().toISOString(), updatedAt: new Date().toISOString() }
    incidents.value = [mock, ...incidents.value]
    return mock
  }

  async function updateIncident(id, status, description) {
    try {
      const updated = await callGo('UpdateIncident', id, status || '', description || '')
      if (updated) {
        incidents.value = incidents.value.map(i => i.id === id ? updated : i)
        return updated
      }
    } catch (e) {
      console.error('[incidents] update:', e)
    }
    // Local fallback.
    incidents.value = incidents.value.map(i => {
      if (i.id === id) {
        return { ...i, status: status || i.status, description: description || i.description, updatedAt: new Date().toISOString() }
      }
      return i
    })
  }

  async function deleteIncident(id) {
    try {
      await callGo('DeleteIncident', id)
    } catch (e) {
      console.error('[incidents] delete:', e)
    }
    incidents.value = incidents.value.filter(i => i.id !== id)
  }

  return { incidents, loading, error, listIncidents, createIncident, updateIncident, deleteIncident }
}

/**
 * Composable for topology graph.
 */
export function useTopology() {
  const topology = ref(null)
  const loading = ref(false)
  const error = ref(null)

  async function fetchTopology(namespace = '') {
    loading.value = true
    error.value = null
    try {
      topology.value = await callGo('GetTopology', namespace)
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  return { topology, loading, error, fetchTopology }
}

/**
 * Composable for ArgoCD-like applications (backed by deployments).
 */
export function useApplications() {
  const applications = ref([])
  const loading = ref(false)
  const error = ref(null)

  async function listApplications(namespace = '') {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ListApplications', namespace)
      if (result && result.length > 0) {
        applications.value = result
      }
    } catch (e) {
      error.value = e?.message || String(e)
    } finally {
      loading.value = false
    }
  }

  async function syncApplication(namespace, name) {
    try {
      await callGo('SyncApplication', namespace, name)
      // Refresh after sync.
      await listApplications(namespace)
    } catch (e) {
      console.error('[applications] sync:', e)
      throw e
    }
  }

  return { applications, loading, error, listApplications, syncApplication }
}

/**
 * Composable for vulnerabilities.
 */
export function useVulnerabilities() {
  const images = ref([])
  const loading = ref(false)
  const error = ref(null)

  const mockVulnerabilities = [
    { 
      id: 'img-1', name: 'web-app:v1.2.4', namespace: 'default', lastScan: '10m ago', 
      critical: 2, high: 5, medium: 12, low: 24, status: 'Vulnerable',
      cves: [
        { id: 'CVE-2023-38545', pkg: 'curl', severity: 'Critical', desc: 'Heap based buffer overflow in SOCKS5 proxy handshake.', fix: 'Upgrade to curl 8.4.0' },
        { id: 'CVE-2023-4911', pkg: 'glibc', severity: 'Critical', desc: 'Buffer overflow in ld.so (Looney Tunables).', fix: 'Update glibc to 2.38-r1' }
      ],
      aiOpt: { issue: 'Base image is using debian:bullseye which has numerous unpatched CVEs.', fix: 'Rebuild image using distroless/cc-debian12 to reduce attack surface and drop 85% of these vulnerabilities.' }
    },
    { 
      id: 'img-2', name: 'worker:latest', namespace: 'default', lastScan: '1h ago', 
      critical: 0, high: 1, medium: 4, low: 8, status: 'Warning',
      cves: [
        { id: 'CVE-2023-5363', pkg: 'openssl', severity: 'High', desc: 'Incorrect cipher key & IV length processing.', fix: 'Upgrade to openssl 3.0.12' }
      ],
      aiOpt: { issue: 'Using "latest" tag is an anti-pattern and masks underlying OS updates.', fix: 'Pin to a specific SHA digest or immutable tag.' }
    },
    { 
      id: 'img-3', name: 'payment:v2.0', namespace: 'finance', lastScan: '2m ago', 
      critical: 0, high: 0, medium: 0, low: 0, status: 'Clean',
      cves: [],
      aiOpt: { issue: 'None', fix: 'Image is optimal and following least-privilege principles.' }
    },
    { 
      id: 'img-4', name: 'ingress-nginx:v1.9.0', namespace: 'kube-system', lastScan: '1d ago', 
      critical: 1, high: 3, medium: 15, low: 40, status: 'Vulnerable',
      cves: [
        { id: 'CVE-2023-44487', pkg: 'nginx', severity: 'Critical', desc: 'HTTP/2 Rapid Reset Attack.', fix: 'Upgrade to ingress-nginx v1.9.3+' }
      ],
      aiOpt: { issue: 'Nginx ingress controller is exposed to HTTP/2 Rapid Reset DOS.', fix: 'Patch controller deployment and enable global rate limiting.' }
    }
  ]

  async function listVulnerabilities() {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ListVulnerabilities')
      images.value = result && result.length > 0 ? result : mockVulnerabilities
    } catch (e) {
      error.value = e?.message || String(e)
      images.value = mockVulnerabilities
    } finally {
      loading.value = false
    }
  }

  async function scanImage(image, engine) {
    try {
      const result = await callGo('ScanImage', image, engine)
      // Refresh the list after scanning a single image.
      await listVulnerabilities()
      return result
    } catch (e) {
      console.error('[vulnerabilities] scan:', e)
      return 'Scan failed'
    }
  }

  async function scanAllImages(namespace = '') {
    loading.value = true
    error.value = null
    try {
      const result = await callGo('ScanAllImages', namespace)
      images.value = result && result.length > 0 ? result : mockVulnerabilities
      return result
    } catch (e) {
      console.warn('[vulnerabilities] scanAll failed, keeping cached data:', e)
      error.value = e?.message || String(e)
      return null
    } finally {
      loading.value = false
    }
  }

  return { images, loading, error, listVulnerabilities, scanImage, scanAllImages }
}

/**
 * Composable for code blocks and sandboxing.
 */
export function useCodeBlock() {
  const isRunning = ref(false)
  const output = ref('')
  const isAnalyzing = ref(false)
  const suggestion = ref('')

  async function runCode(code, language) {
    isRunning.value = true
    output.value = ''
    try {
      const result = await callGo('RunCodeSandbox', code, language)
      output.value = result || 'No output'
    } catch (e) {
      output.value = e?.message || String(e)
    } finally {
      isRunning.value = false
    }
  }

  async function getAiSuggestion(code, language) {
    isAnalyzing.value = true
    suggestion.value = ''
    try {
      const result = await callGo('GetCodeSuggestion', code, language)
      suggestion.value = result || 'No suggestion available'
    } catch (e) {
      suggestion.value = e?.message || String(e)
    } finally {
      isAnalyzing.value = false
    }
  }

  return { isRunning, output, isAnalyzing, suggestion, runCode, getAiSuggestion }
}
