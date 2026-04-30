# KubeWatcher Features & Component Requirements

This document tracks the requirements and features implemented across the various Vue components in the KubeWatcher UI.

## `MetricsExplorer.vue` (Center / Dashboards)
- **Live Stream Mode**: Ability to toggle real-time metrics streaming.
- **Background Polling**: When Live Stream is active, polling occurs every 1.5s in the background without triggering disruptive loading spinners.
- **Dynamic Charting**: Graphs physically shift to the left seamlessly to visualize new data points organically.

## `NodeList.vue` (Cluster / Nodes)
- **Master-Detail Expansion**: Clicking on a node card expands it, hiding other cards and taking over the view.
- **System Information Panel**: Displays internal/external IPs, container runtime, kubelet version, and active taints.
- **Tailored Metrics Sparklines**: Inline SVG charts showing historical CPU Load, Memory, and Disk I/O.
- **Live Log Streaming**: A built-in system-service terminal tailing `kubelet` and `containerd` logs. Features a text filter for searching and a play/pause stream toggle.

## `NamespaceList.vue` (Cluster / Namespaces)
- **Network Policy Graph**: Visual flow-diagram detailing the inbound (ingress) and outbound (egress) access points for the namespace.
- **Policy Evaluation**: Highlights default deny vs default allow networking setups.
- **Resource Quotas**: Readouts detailing CPU Limits, Memory Limits, and Max Pod count guardrails.

## `PodList.vue` (Workloads / Pods)
- **Live Metrics**: Real-time CPU and Memory usage sparklines.
- **Resource Allocation & VPA**: Side-by-side comparison of current Requests/Limits vs Popeye/VPA Recommendations.
- **Apply & Redeploy Workflow**: A button to apply AI recommendations, triggering a notification that the KubeWatcher agent will actively monitor the new deployment state for 10 minutes.
- **Configuration Context**: Quick links/badges highlighting attached ConfigMaps.
- **Interactive YAML Editor**: Integrated text area to view and edit the raw Deployment/StatefulSet YAML.

## `DeploymentList.vue` (Workloads / Deployments)
- **Manifest Revision Tracker**: A label attached to each deployment summarizing the last 25 revisions (e.g., "25/25 No Changes" or "X/25 Changes").
- **Timeline History**: Clicking the tracker opens a vertical timeline visualizing the deployment history.
- **Change Details**: The timeline explicitly highlights what changed (e.g., Image Updates, Scaling events) versus simple background manifest syncs.

## `CodeBlockComponent.vue` (S3 Notebooks / Markdown Editor)
- **Sandbox Execution**: A 'Play' icon inside code blocks that simulates running the script in a sandbox, outputting the result to a built-in terminal display.
- **Antigravity AI Insights**: A 'Magic Wand' icon that simulates an AI agent analyzing the code block, returning architectural or security recommendations.
- **Clipboard Utility**: Standard copy-to-clipboard functionality.

## `ServiceList.vue` (Network / Services)
- **Live Endpoint Routing (Topology)**: Visual flow diagram mapping ClusterIP and selector to active backend Pod endpoints.
- **AI Network Profiler**: Assesses configurations (sessionAffinity, externalTrafficPolicy) to suggest performance and security optimizations.

## `VolumeList.vue` (Storage / PVCs & PVs)
- **Live I/O Metrics**: Sparklines tracing real-time Disk IOPS (Reads/Writes).
- **Storage Topology**: Identifies which Pod is mounting the volume and at what exact path.
- **AI Storage Optimizer**: Monitors for IOPS throttling or over-provisioning and suggests runtime storage class migrations (e.g. gp2 to gp3).

## `VulnerabilityList.vue` (Monitoring / Vulnerabilities)
- **Top-Level Metrics Dash**: Highlights Total Scanned Images and counts for Critical, High, and Medium vulnerabilities.
- **Engine Selector**: Ability to trigger Global or localized scans using external engines like Snyk, Trivy, or Falco.
- **Antigravity Security Advisor**: AI provides architectural mitigation strategies for flagged images (e.g., pinning to SHA, utilizing distroless images).
- **CVE Breakdown**: Explicit listing of CVEs with their severity, vulnerable package, description, and required fix.

## Backend Architecture Overview

The backend is split into two primary components:
- **Wails Desktop Client (`api/pkg/app.go`)**: Acts as the bridge between the Vue UI and external services (Kubernetes API, AWS S3, OAuth).
- **In-Cluster Agent (`agent/cmd/agent/main.go`)**: A DaemonSet running inside the Kubernetes cluster. It bypasses the control plane API bottleneck by using Informers to scrape metrics, logs, and events locally, serving them to the Desktop Client via an internal HTTP/WebSocket API.

### 1. Monitoring Views
- **Metrics Explorer**:
  - **Feature**: Visualize CPU, Memory, and Network usage over time for nodes, namespaces, and pods.
  - **Backend**: Desktop Client does no direct polling. Agent scrapes Kubelet `/metrics/cadvisor` and Kubernetes Metrics Server. Exposes a WebSocket endpoint `/stream/metrics` for live data.
- **Alerts**:
  - **Feature**: Aggregated list of firing alerts across the cluster.
  - **Backend**: Desktop Client subscribes to Agent WebSocket. Agent integrates with Prometheus Alertmanager API or evaluates basic threshold rules against its local metric cache.
- **Topology**:
  - **Feature**: Visual node-edge graph of the cluster (Pods -> Services -> Ingresses).
  - **Backend**: Desktop Client calls standard `client-go` functions (`GetPods`, `GetServices`) or requests directly from the Agent. Agent maintains an in-memory graph via `SharedInformerFactory` to quickly serve the full topology state.
- **Logs (Log Explorer)**:
  - **Feature**: Live streaming of container stdout/stderr with filtering and highlighting.
  - **Backend**: Desktop Client connects to Agent's log stream via WebSocket. Agent hooks directly into the Kubelet log files (`/var/log/containers/`) or uses `client-go` log streaming, buffering them and sending them to the client.
- **Anomaly Detection**:
  - **Feature**: AI-driven anomaly scores (e.g., "Sudden Memory Spike", "High Error Rate").
  - **Backend**: Desktop Client port-forwards to the Agent pod (`kubectl port-forward`) and fetches `/api/v1/anomalies`. Agent runs a lightweight ML model (or statistical baseline algorithm) against ingested metrics.
- **Analysis (Popeye Report)**:
  - **Feature**: Comprehensive cluster audit showing security risks, misconfigurations, and offering "Fix with Agent" buttons.
  - **Backend**: Desktop Client executes the popeye binary under the hood or calls a dedicated Agent endpoint `/api/v1/audit`. For the "Fix with Agent" button, the client sends a POST request with the specific patch/remediation command to the Agent. Agent performs deep validation of cluster manifests and accepts authorized remediation commands via a secure RPC endpoint.

### 2. Cluster Views
- **Nodes, Namespaces, Events**:
  - **Feature**: High-fidelity views of core cluster infrastructure, real-time event streaming.
  - **Backend**: Desktop Client uses standard `client-go` CRUD operations. For events, it opens a watch stream using `client-go`. Agent can optionally aggregate events via its `SharedInformer` to reduce control-plane load, exposing an `/api/v1/events` stream.

### 3. Workload Views
- **Pods, Deployments, StatefulSets, DaemonSets, ReplicaSets, Jobs, CronJobs**:
  - **Feature**: Detailed status tracking (Ready vs Desired), restart counts, duration, and age.
  - **Backend**: Desktop Client uses standard `client-go` lists and watchers. To make it highly responsive, the UI should use Wails to subscribe to Go events. Go will use `SharedInformerFactory` to watch workloads and emit Wails events (`runtime.EventsEmit`) whenever a resource changes.

### 4. Config Views
- **ConfigMaps, Secrets, HPAs**:
  - **Feature**: View and edit configuration data, track HPA scaling targets in real-time.
  - **Backend**: Desktop Client uses `client-go` to read/write ConfigMaps and Secrets. For HPAs, it reads the `autoscaling/v2` API to get current metrics vs desired targets.

### 5. Network Views
- **Services, Ingresses, NetworkPolicies, Endpoints**:
  - **Feature**: Traffic flow visualization, ingress routing rules, and security policies.
  - **Backend**: Desktop Client uses `client-go` API calls to `networking.k8s.io` to fetch rules.

### 6. Storage Views
- **Volume Claims (PVCs), Volumes (PVs), StorageClasses**:
  - **Feature**: Capacity visualization ("Liquid Glass" effect) showing requested vs available vs used storage.
  - **Backend**: Desktop Client gets PVC/PV sizing from `client-go`. Agent scrapes the Kubelet volume stats API (`/stats/summary`) and serves this enriched data back to the client to get actual usage (e.g., 5GB used out of 10GB requested).

### 7. Operations Views
- **Runbooks & Incident Log**:
  - **Feature**: Executable markdown runbooks and tracking of cluster incidents.
  - **Backend**: Desktop Client stores incident metadata locally (e.g., SQLite via gorm) or syncs it to a remote DB.
- **Workflows**:
  - **Feature**: Visual node-based workflow builder for cluster operations.
  - **Backend**: Desktop Client saves workflow JSON definitions to disk. A custom Go execution engine parses the DAG and runs the specific `kubectl` or API commands sequentially.

### 8. Knowledge Views
- **S3 Notebooks (Obsidian Style)**:
  - **Feature**: WYSIWYG Markdown editor with syntax highlighting, synced to an S3 bucket.
  - **Backend**: Desktop Client uses `aws-sdk-go-v2`. When a file is saved, the Wails backend intercepts the content, writes it to a local cache directory (for offline support), and asynchronously uploads it to the configured AWS S3 bucket. Supports directory structures by mapping S3 prefixes to local folders.

### Desktop Features (Pro)
- **Isolated Program Environment**: Runs as a "program inside a program," providing a terminal-like experience that is fully isolated and agent-friendly.
- **Standalone Desktop App**: The terminal can be "popped out" from the main interface and run as a separate application item on your laptop.
- **Agent-Centric Workflows**: Specifically designed to support agentic workflows, allowing the AI to execute commands and manage files in its own dedicated space.
- **Workflows Support**: Integrated workflows that function seamlessly within the popped-out application window.
- **Pro Version Exclusive**: Both the "pop out" desktop app functionality and advanced workflow support are only available in the Pro version.
