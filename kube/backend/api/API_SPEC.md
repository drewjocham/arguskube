# Argus API Specification

> Version 0.1.0 — Generated 2026-05-02

Argus exposes two API surfaces: **Wails bindings** for the native desktop app and an **HTTP/JSON API** for the SaaS web client. Both surfaces call the same Go methods on the `App` struct. All methods return `(result, error)` unless noted otherwise.

---

## Transport

### Wails Desktop Bindings

Auto-generated TypeScript stubs live in `view/src/wailsjs/go/pkg/App.js`. The frontend calls them via:

```js
import { ListResources } from '../wailsjs/go/pkg/App'
const result = await ListResources('pods', '_all')
```

### HTTP/JSON API (SaaS Mode)

A single dynamic RPC endpoint dispatches to the same methods:

```
POST /api/{MethodName}
Content-Type: application/json

{ "args": [arg1, arg2, ...] }
```

Response:

```json
{ "result": <return_value>, "error": "<error_message or empty>" }
```

CORS is enabled for all origins. The server listens on `:8080` by default (`argus_PORT`).

### WebSocket

```
GET /tunnel
```

Bidirectional channel for in-cluster Argus agents. Handled by `Hub.HandleTunnel()`.

### Webhooks

```
POST /webhooks/anomstack
```

Ingests anomaly alerts from external detectors (Anomstack, custom). See [WebhookPayload](#webhookpayload) for the request schema. Returns `{"alert_id": "<uuid>", "status": "ok"}`.

---

## Methods

### Cluster Connection

#### `GetClusterInfo() → ClusterInfo`

Returns cluster metadata. Safe to call when disconnected (returns placeholder values).

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Kubeconfig context name |
| `nodeCount` | int | Number of nodes |
| `k8sVersion` | string | Server version string |

#### `ListContexts() → ContextInfo[]`

Returns all available kubeconfig contexts. Reads the kubeconfig file directly — works even when no cluster is connected.

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Context name |
| `cluster` | string | Cluster name |
| `active` | bool | Whether this is the current context |

#### `SwitchContext(name string) → error`

Changes the active kubeconfig context at runtime. If no k8s client exists yet (e.g. initial connection failed), bootstraps one targeting this context. Rebuilds the agent connector with the new client.

---

### Metrics & Alerts

#### `GetMetrics() → ClusterMetrics`

Returns cluster health metrics. Caches the last successful result.

| Field | Type | Description |
|-------|------|-------------|
| `podHealthPct` | float64 | Percentage of pods in Running/Succeeded state |
| `podsRunning` | int | Running pod count |
| `podsTotal` | int | Total pod count |
| `podsPending` | int | Pending pod count |
| `podsFailed` | int | Failed pod count |
| `errorRate` | float64 | Current error rate |
| `errorRatePrev` | float64 | Previous-period error rate |
| `restartCount` | int32 | Total container restarts |
| `restartTop` | string | Top restarter, e.g. `"payments-api: 32"` |
| `warningEvents` | int | Warning event count |
| `totalCPUMillis` | int64 | Cluster-wide CPU request in millicores |
| `totalMemoryBytes` | int64 | Cluster-wide memory request in bytes |
| `p99Latency` | string | P99 latency (from Prometheus if configured) |
| `sloStatus` | string | `"ok"` or `"breach"` |

#### `GetAlerts() → Alert[]`

Returns enriched alerts from the live cluster merged with webhook-received anomaly alerts. Sorted by severity and timestamp.

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique alert ID |
| `name` | string | Alert title |
| `severity` | string | `"critical"`, `"warning"`, `"info"` |
| `namespace` | string | Kubernetes namespace |
| `timestamp` | time | Alert timestamp |
| `podName` | string | Affected pod name |
| `podPhase` | string | Pod phase (Running, Pending, Failed, etc.) |
| `restartCount` | int32 | Container restart count |
| `containerID` | string | Container ID |
| `memoryLimit` | string | Container memory limit |
| `memoryRequest` | string | Container memory request |
| `cpuLimit` | string | Container CPU limit |
| `cpuRequest` | string | Container CPU request |
| `cpuThrottle` | float64 | CPU throttle percentage |
| `nodeName` | string | Node hosting the pod |
| `diskUsage` | float64 | Node disk usage percentage |
| `diskCapacity` | string | Node disk capacity |
| `evictedPods` | string[] | Pods evicted from this node |
| `imageTag` | string | Current container image |
| `previousImage` | string | Previous container image (rollback detection) |
| `deployTime` | time | Deployment timestamp |
| `description` | string | Human-readable alert description |
| `tags` | Tag[] | Categorization tags with colors |
| `relatedAlerts` | string[] | IDs of cascade-correlated alerts |

**Tag:**

| Field | Type | Values |
|-------|------|--------|
| `label` | string | Tag text |
| `color` | string | `"red"`, `"blue"`, `"amber"`, `"purple"`, `"teal"` |

---

### Resources

#### `ListResources(kind string, namespace string) → ResourceListResult`

Lists Kubernetes resources of the given kind. Pass `"_all"` as namespace to list across all namespaces.

**Supported kinds:** `pods`, `deployments`, `statefulsets`, `daemonsets`, `replicasets`, `jobs`, `cronjobs`, `services`, `endpoints`, `ingresses`, `networkpolicies`, `configmaps`, `secrets`, `hpas`, `pvcs`, `pvs`, `storageclasses`, `nodes`, `namespaces`, `events`

| Field | Type | Description |
|-------|------|-------------|
| `schema` | ResourceSchema | Column definitions for the resource kind |
| `items` | ResourceItem[] | List of resources |
| `total` | int | Total count |

**ResourceSchema:**

| Field | Type | Description |
|-------|------|-------------|
| `kind` | string | Resource kind |
| `columns` | ResourceColumn[] | Column definitions |

**ResourceColumn:**

| Field | Type | Description |
|-------|------|-------------|
| `key` | string | Column key (used in `Fields` map) |
| `label` | string | Display label |
| `width` | string | Suggested column width |

**ResourceItem:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Resource name |
| `namespace` | string | Namespace |
| `status` | string | Health status label |
| `statusColor` | string | `"green"`, `"red"`, `"amber"`, `"blue"`, `"gray"` |
| `age` | string | Human-readable age (e.g. `"3d"`, `"12h"`) |
| `fields` | map[string]string | Column values keyed by ResourceColumn.key |

#### `GetResourceDetail(kind string, namespace string, name string) → ResourceDetailResult`

Returns full details for a specific resource including labels, annotations, conditions, events, and resource-specific data.

| Field | Type | Description |
|-------|------|-------------|
| `kind` | string | Resource kind |
| `name` | string | Resource name |
| `namespace` | string | Namespace |
| `created` | string | Creation timestamp |
| `labels` | map[string]string | Kubernetes labels |
| `annotations` | map[string]string | Kubernetes annotations |
| `properties` | KeyValue[] | Typed key-value pairs |
| `data` | map[string]string | ConfigMap/Secret data entries |
| `conditions` | ResourceCondition[] | Status conditions |
| `events` | ResourceEvent[] | Recent events for this resource |
| `extra` | map[string]any | Resource-specific extra data |

**ResourceCondition:**

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Condition type (e.g. `"Ready"`, `"Available"`) |
| `status` | string | `"True"`, `"False"`, `"Unknown"` |
| `reason` | string | Machine-readable reason |
| `message` | string | Human-readable message |
| `age` | string | Time since last transition |

**ResourceEvent:**

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `"Normal"`, `"Warning"` |
| `reason` | string | Event reason |
| `message` | string | Event message |
| `count` | int32 | Occurrence count |
| `age` | string | Time since event |

#### `ListAllNamespaces() → string[]`

Returns all namespace names for the namespace picker.

#### `DeletePod(namespace string, podName string) → error`

Deletes a pod by namespace and name.

---

### Topology & Applications

#### `GetTopology(namespace string) → TopologyResult`

Builds a topology graph from live cluster state showing the relationship between nodes, deployments, and pods.

**TopologyResult:**

| Field | Type | Description |
|-------|------|-------------|
| `nodes` | TopologyNode[] | Graph nodes |
| `edges` | TopologyEdge[] | Graph edges |

**TopologyNode:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique ID, e.g. `"node/worker-1"`, `"deploy/default/nginx"` |
| `kind` | string | `"node"`, `"deployment"`, `"pod"`, `"service"` |
| `name` | string | Resource name |
| `namespace` | string | Namespace |
| `status` | string | `"ok"`, `"warn"`, `"crit"` |

**TopologyEdge:**

| Field | Type | Description |
|-------|------|-------------|
| `source` | string | Source node ID |
| `target` | string | Target node ID |
| `label` | string | Edge label |

#### `ListApplications(namespace string) → Application[]`

Returns deployment-based "applications" with rollout status.

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Deployment name |
| `namespace` | string | Namespace |
| `syncStatus` | string | `"Synced"`, `"OutOfSync"` |
| `healthStatus` | string | `"Healthy"`, `"Degraded"`, `"Progressing"` |
| `replicas` | int32 | Desired replicas |
| `readyReplicas` | int32 | Ready replicas |
| `image` | string | Container image |
| `lastSync` | string | Last sync timestamp |

#### `SyncApplication(namespace string, name string) → error`

Triggers a rollout restart on the specified deployment.

---

### Logs

#### `GetPodLogs(namespace string, podName string, tailLines int64) → LogLine[]`

Returns recent log lines for a pod.

**LogLine:**

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | time | Log timestamp |
| `source` | string | Source identifier (pod/container) |
| `level` | string | `"error"`, `"warn"`, `"info"`, `"ok"` |
| `message` | string | Log message |

#### `StreamPodLogsFollow(namespace string, podName string, container string, tailLines int64) → string[]`

Streams pod logs with `follow=true`. Collects into a buffer (max 500 lines) with a 10-second timeout. For real-time streaming, use WebSocket events.

#### `QueryLogs(query string, namespace string, limit int) → LogQueryResult`

Searches pod logs across the namespace with a text filter.

| Field | Type | Description |
|-------|------|-------------|
| `entries` | LogEntry[] | Matching log lines |
| `total` | int | Total matches |
| `fields` | string[] | Available fields |
| `histogram` | int[] | 50-bucket hit distribution over time |

**LogEntry:**

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string | Log timestamp |
| `message` | string | Log message |
| `pod` | string | Pod name |
| `namespace` | string | Namespace |
| `container` | string | Container name |
| `node` | string | Node name |

#### `GetNodeLogs(nodeName string, tailLines int) → NodeLogEntry[]`

Fetches kubelet/containerd/kube-proxy system logs from a cluster node via the kubelet proxy API.

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string | Log timestamp, e.g. `"May 01 14:32:01"` |
| `level` | string | `"INFO"`, `"WARN"`, `"ERROR"` |
| `service` | string | `"kubelet"`, `"containerd"`, `"kube-proxy"` |
| `message` | string | Log message |

---

### Deployments & Scaling

#### `GetDeploymentRevisions(namespace string, deployment string, limit int) → DeploymentRevision[]`

Returns the rollout history for a deployment. Default limit is 25.

| Field | Type | Description |
|-------|------|-------------|
| `revision` | string | Revision number |
| `replicaSet` | string | ReplicaSet name |
| `image` | string | Container image |
| `replicas` | int | Desired replicas |
| `readyReplicas` | int | Ready replicas |
| `active` | bool | Whether this is the current revision |
| `changeCause` | string | Annotation value from `kubernetes.io/change-cause` |
| `createdAt` | time | ReplicaSet creation time |

#### `GetVPARecommendations(namespace string) → VPARecommendation[]`

Returns VerticalPodAutoscaler recommendations from `autoscaling.k8s.io/v1`.

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | VPA name |
| `namespace` | string | Namespace |
| `targetRef` | string | Target reference, e.g. `"Deployment/web-app"` |
| `updateMode` | string | Update mode (`"Auto"`, `"Off"`, etc.) |
| `containers` | VPAContainerRecommend[] | Per-container recommendations |
| `createdAt` | time | VPA creation time |

**VPAContainerRecommend:**

| Field | Type | Description |
|-------|------|-------------|
| `containerName` | string | Container name |
| `lowerCPU` | string | Lower bound CPU |
| `lowerMemory` | string | Lower bound memory |
| `targetCPU` | string | Target CPU |
| `targetMemory` | string | Target memory |
| `upperCPU` | string | Upper bound CPU |
| `upperMemory` | string | Upper bound memory |

#### `ScaleDeployment(namespace string, name string, replicas int32) → error`

Scales a deployment to the specified replica count.

#### `RestartDeployment(namespace string, name string) → error`

Triggers a rolling restart on a deployment.

---

### AI & Diagnostics

> Requires **Pro** tier. Returns `features.ErrProRequired` on Free tier.

#### `DiagnoseAlert(alertID string) → ctxassembly.Bundle`

Assembles a full diagnostic context bundle for the given alert, including related metrics, events, logs, cascade correlations, and decision log entries.

#### `SendChatMessage(alertID string, message string) → string`

Sends a user message to the DeepSeek AI agent. Builds enriched diagnostic context with metrics, events, restarters, and cascade alerts. Returns the AI response text.

#### `GetChatHistory(alertID string) → ChatEntry[]`

Returns the conversation history for an alert's diagnostic session.

| Field | Type | Description |
|-------|------|-------------|
| `role` | string | `"user"` or `"assistant"` |
| `content` | string | Message text |

#### `GetAutoSummary(alertID string) → AutoSummary`

Returns the auto-investigation summary for an alert (generated by background agent).

#### `GetAgentEventLog() → AgentEvent[]`

Returns the agent's tracked events and detected patterns.

#### `RunArgusScan() → popeye.Report`

Executes a Popeye cluster scan and returns sanitization findings.

---

### Metrics & Time Series

#### `QueryTimeSeriesMetrics(query string, timeRange string) → float64[]`

Returns time-series data points for a metric query. Used by the Metrics Explorer.

---

### Anomaly Detection

#### `GetAnomalyJobs() → anomaly.Job[]`

Returns configured Anomstack anomaly detection jobs. Requires Pro tier.

---

### Vulnerability Scanning

#### `ListVulnerabilities() → ScannedImage[]`

Returns cached Trivy vulnerability scan results.

**ScannedImage:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Scan ID |
| `name` | string | Image name |
| `namespace` | string | Namespace where image is running |
| `lastScan` | string | Last scan timestamp |
| `critical` | int | Critical CVE count |
| `high` | int | High CVE count |
| `medium` | int | Medium CVE count |
| `low` | int | Low CVE count |
| `status` | string | Scan status |
| `cves` | Vulnerability[] | Individual vulnerabilities |

**Vulnerability:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | CVE ID (e.g. `"CVE-2024-1234"`) |
| `severity` | string | `"CRITICAL"`, `"HIGH"`, `"MEDIUM"`, `"LOW"` |
| `package` | string | Affected package name |
| `title` | string | CVE title |
| `description` | string | CVE description |

#### `ScanImage(image string, engine string) → string`

Triggers a Trivy vulnerability scan for a single image. Returns scan output.

#### `ScanAllImages(namespace string) → ScannedImage[]`

Scans all container images in the given namespace.

---

### FinOps / Cost Estimation

#### `EstimateCosts() → ClusterCostReport`

Estimates cluster costs based on resource requests using configurable per-unit pricing (defaults to AWS on-demand rates).

**ClusterCostReport:**

| Field | Type | Description |
|-------|------|-------------|
| `namespaces` | CostBreakdown[] | Cost per namespace |
| `topDeployments` | CostBreakdown[] | Top 20 deployments by cost |
| `totalCostHr` | float64 | Total cluster cost per hour |
| `totalCostDay` | float64 | Total cost per day |
| `totalCostMo` | float64 | Total cost per month (730 hours) |
| `totalCPU` | float64 | Total CPU cores requested |
| `totalMemGB` | float64 | Total memory in GB requested |
| `podCount` | int | Total pod count |

**CostBreakdown:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Resource name |
| `namespace` | string | Namespace |
| `kind` | string | `"Namespace"`, `"Deployment"`, `"Pod"` |
| `cpuCores` | float64 | CPU cores requested |
| `memoryGB` | float64 | Memory in GB |
| `cpuCostHr` | float64 | CPU cost per hour |
| `memCostHr` | float64 | Memory cost per hour |
| `totalCostHr` | float64 | Total cost per hour |
| `totalCostDay` | float64 | Total cost per day |
| `totalCostMo` | float64 | Total cost per month |
| `podCount` | int | Pod count |

---

### Operations

#### Runbooks

| Method | Signature | Description |
|--------|-----------|-------------|
| `ListRunbooks` | `() → Runbook[]` | List all runbooks |
| `GetRunbook` | `(id string) → string` | Get runbook content by ID |
| `SaveRunbook` | `(id string, content string) → error` | Update runbook content |
| `DeleteRunbook` | `(id string) → error` | Delete a runbook |
| `CreateRunbook` | `(name string, trigger string) → Runbook` | Create a new runbook |

#### Incidents

| Method | Signature | Description |
|--------|-----------|-------------|
| `ListIncidents` | `() → Incident[]` | List all incidents |
| `CreateIncident` | `(title, severity, incType, description, namespace string) → Incident` | Create an incident |
| `UpdateIncident` | `(id, status, description string) → Incident` | Update incident status |
| `DeleteIncident` | `(id string) → error` | Delete an incident |

#### Notebooks (S3-backed)

| Method | Signature | Description |
|--------|-----------|-------------|
| `ListNotebooks` | `() → FileEntry[]` | List notebook files |
| `GetNotebook` | `(path string) → string` | Get notebook content |
| `SaveNotebook` | `(path string, content string) → error` | Save notebook content |
| `DeleteNotebook` | `(path string) → error` | Delete a notebook |
| `CreateNotebookFolder` | `(path string) → error` | Create a folder |
| `MoveNotebook` | `(oldPath string, newPath string) → error` | Move/rename a notebook |
| `TestS3Connection` | `() → error` | Test S3 bucket connectivity |

---

### Settings

#### `GetSettings() → SettingsPayload`

Returns the current runtime configuration. The API key is masked for display.

**SettingsPayload:**

| Field | Type | Description |
|-------|------|-------------|
| `kubeconfigPath` | string | Path to kubeconfig (supports colon-separated multi-file) |
| `currentContext` | string | Active kubeconfig context |
| `namespace` | string | Default namespace filter |
| `deepseekApiKey` | string | Masked API key (e.g. `"sk-…1234"`) |
| `anomstackUrl` | string | Anomstack endpoint |
| `prometheusUrl` | string | Prometheus endpoint |
| `tier` | string | Current tier (`"free"` or `"pro"`) |
| `logLevel` | string | Log level (`"info"`, `"debug"`, etc.) |

#### `UpdateSettings(s SettingsPayload) → error`

Applies runtime setting overrides. Only non-empty fields are applied. If kubeconfig or context changes, triggers a full k8s client reconnect and restarts the event loop.

Masked API key values (containing `•`, `…`, or `*`) are ignored to prevent overwriting the real key with the display value.

---

### Setup & Tools

| Method | Signature | Description |
|--------|-----------|-------------|
| `CheckToolStatus` | `() → ToolStatus[]` | Check availability of CLI tools (kubectl, popeye, trivy) |
| `InstallArgusScan` | `() → SetupResult` | Install the Argus scan CLI |
| `DeployAgent` | `(namespace string) → SetupResult` | Deploy the in-cluster agent |
| `UndeployAgent` | `(namespace string) → SetupResult` | Remove the in-cluster agent |

---

### Terminal

| Method | Signature | Description |
|--------|-----------|-------------|
| `StartTerminal` | `(rows int, cols int) → error` | Open a PTY shell session |
| `SendTerminalInput` | `(data string) → error` | Write raw input to the terminal |
| `ResizeTerminal` | `(rows int, cols int) → error` | Update terminal dimensions |

Terminal output is pushed via Wails event `terminal:output`.

---

### Feature Gating

#### `GetFeatures() → map[Feature]bool`

Returns all features and their availability for the current tier.

#### `GetTier() → Tier`

Returns the current subscription tier (`"free"` or `"pro"`).

**Free tier features:** alerts, cluster_view, log_stream, topology, cascade_correlation, anomstack_anomaly, decision_log_context

**Pro tier features:** ai_diagnostics, runbook_automation, multi_cluster, extended_history, custom_runbooks, arguscd

---

## Push Events (Wails EventsEmit)

The backend pushes real-time updates via Wails events. Subscribe with `runtime.EventsOn(eventName, callback)`.

| Event | Payload | Description |
|-------|---------|-------------|
| `alert:update` | `Alert[]` | Alerts refreshed (polled every 10s) |
| `metrics:update` | `ClusterMetrics` | Metrics refreshed (polled every 15s) |
| `log:line` | `LogLine` | New log line from watched pods |
| `agent:auto-summary` | `AutoSummary` | Background agent completed investigation |
| `agent:event` | `AgentEvent` | Agent detected a pattern or event |
| `terminal:output` | `string` | Terminal PTY output |
| `deep-link` | `string` | Custom URL scheme received (`argus://...`) |

---

## Webhook Schema

### WebhookPayload

POST to `/webhooks/anomstack`:

```json
{
  "title": "CPU anomaly on web-api",
  "metricName": "cpu_usage",
  "threshold": 0.85,
  "score": 0.97,
  "severity": "critical",
  "namespace": "production",
  "podName": "web-api-7f8d9c6b5-x2k4p",
  "nodeName": "worker-3",
  "labels": {
    "app": "web-api",
    "team": "platform"
  }
}
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `argus_KUBECONFIG` | `$KUBECONFIG` | Kubeconfig path (colon-separated for multi-file) |
| `argus_CONTEXT` | _(current)_ | Override kubeconfig context |
| `argus_NAMESPACE` | _(all)_ | Default namespace filter |
| `argus_IN_CLUSTER` | `false` | Use in-cluster config |
| `DEEPSEEK_API_KEY` | — | DeepSeek API key for AI diagnostics |
| `ANOMSTACK_URL` | `http://localhost:8087` | Anomstack endpoint |
| `ANOMSTACK_API_KEY` | — | Anomstack API key |
| `PROMETHEUS_URL` | _(auto-detect)_ | Prometheus endpoint for metrics enrichment |
| `argus_TIER` | `pro` | Feature tier (`free` / `pro`) |
| `argus_LICENSE` | — | License key |
| `argus_PORT` | `8080` | HTTP API port |
| `argus_METRICS_PORT` | `9090` | Prometheus metrics port |
| `argus_LOG_LEVEL` | `info` | Log level |
| `argus_LOG_FORMAT` | `text` | Log format (`text` / `json`) |
| `argus_POPEYE_BIN` | `popeye` | Path to popeye binary |
| `argus_S3_BUCKET` | — | S3 bucket for notebooks |
| `argus_S3_REGION` | `us-east-1` | S3 region |
| `argus_S3_ENDPOINT` | — | S3 endpoint (for MinIO etc.) |
| `argus_S3_ACCESS_KEY` | — | S3 access key |
| `argus_S3_SECRET_KEY` | — | S3 secret key |
| `argus_DECISION_LOG` | `DECISION_LOG.md` | Path to decision log |

---

## Error Handling

All methods return `(result, error)`. Common error conditions:

| Error | Cause |
|-------|-------|
| `no cluster connected — check kubeconfig` | `a.k8s == nil` — no cluster connection at startup |
| `features.ErrProRequired` | Method requires Pro tier |
| `reconnect failed: <detail>` | Settings update triggered reconnect that failed |
| `k8s config: <detail>` | Kubeconfig parsing or loading error |
| `k8s client: <detail>` | Kubernetes client creation error |
