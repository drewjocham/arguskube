# KubeWatcher Feature Map & Backend Architecture

This document outlines the expected capabilities for every view within the KubeWatcher UI and the specific backend architecture required to make them functional.

## Backend Architecture Overview

The backend is split into two primary components:
1. **Wails Desktop Client (`api/pkg/app.go`)**: Acts as the bridge between the Vue UI and external services (Kubernetes API, AWS S3, OAuth).
2. **In-Cluster Agent (`agent/cmd/agent/main.go`)**: A DaemonSet running inside the Kubernetes cluster. It bypasses the control plane API bottleneck by using Informers to scrape metrics, logs, and events locally, serving them to the Desktop Client via an internal HTTP/WebSocket API.

---

## 1. Monitoring Views

### Metrics Explorer
* **Feature**: Visualize CPU, Memory, and Network usage over time for nodes, namespaces, and pods.
* **Backend Architecture**: 
  * *Desktop Client*: No direct polling.
  * *Agent*: Scrapes Kubelet `/metrics/cadvisor` and Kubernetes Metrics Server. Exposes a WebSocket endpoint `/stream/metrics` for live data.

### Alerts
* **Feature**: Aggregated list of firing alerts across the cluster.
* **Backend Architecture**:
  * *Desktop Client*: Subscribes to Agent WebSocket.
  * *Agent*: Integrates with Prometheus Alertmanager API or evaluates basic threshold rules against its local metric cache.

### Topology
* **Feature**: Visual node-edge graph of the cluster (Pods -> Services -> Ingresses).
* **Backend Architecture**:
  * *Desktop Client*: Calls standard `client-go` functions (`GetPods`, `GetServices`) to build a relationship map, or requests it directly from the Agent.
  * *Agent*: Maintains an in-memory graph via `SharedInformerFactory` to quickly serve the full topology state without hitting the API server.

### Logs (Log Explorer)
* **Feature**: Live streaming of container stdout/stderr with filtering and highlighting.
* **Backend Architecture**:
  * *Desktop Client*: Connects to the Agent's log stream via WebSocket.
  * *Agent*: Hooks directly into the Kubelet log files (`/var/log/containers/`) or uses `client-go` log streaming, buffering them and sending them to the client.

### Anomaly Detection
* **Feature**: AI-driven anomaly scores (e.g., "Sudden Memory Spike", "High Error Rate").
* **Backend Architecture**:
  * *Desktop Client*: Port-forwards to the Agent pod (`kubectl port-forward`) and fetches `/api/v1/anomalies`.
  * *Agent*: Runs a lightweight ML model (or statistical baseline algorithm) against the ingested metrics. Stores recent anomalies in memory.

### Analysis (Popeye Report)
* **Feature**: Comprehensive cluster audit showing security risks, misconfigurations, and offering "Fix with Agent" buttons.
* **Backend Architecture**:
  * *Desktop Client*: Executes the `popeye` binary under the hood or calls a dedicated Agent endpoint `/api/v1/audit`. For the "Fix with Agent" button, the client sends a POST request with the specific patch/remediation command to the Agent.
  * *Agent*: Performs deep validation of cluster manifests. Accepts authorized remediation commands via a secure RPC endpoint.

---

## 2. Cluster Views

### Nodes, Namespaces, Events
* **Feature**: High-fidelity views of core cluster infrastructure, real-time event streaming.
* **Backend Architecture**:
  * *Desktop Client*: Standard `client-go` CRUD operations. For events, it opens a watch stream using `client-go`.
  * *Agent*: Can optionally aggregate events via its `SharedInformer` to reduce control-plane load, exposing an `/api/v1/events` stream.

---

## 3. Workload Views

### Pods, Deployments, StatefulSets, DaemonSets, ReplicaSets, Jobs, CronJobs
* **Feature**: Detailed status tracking (Ready vs Desired), restart counts, duration, and age.
* **Backend Architecture**:
  * *Desktop Client*: Standard `client-go` lists and watchers. To make it highly responsive, the UI should use Wails to subscribe to Go events. Go will use `SharedInformerFactory` to watch workloads and emit Wails events (`runtime.EventsEmit`) whenever a resource changes.

---

## 4. Config Views

### ConfigMaps, Secrets, HPAs
* **Feature**: View and edit configuration data, track HPA scaling targets in real-time.
* **Backend Architecture**:
  * *Desktop Client*: `client-go` to read/write ConfigMaps and Secrets. For HPAs, it reads the `autoscaling/v2` API to get current metrics vs desired targets.

---

## 5. Network Views

### Services, Ingresses, NetworkPolicies, Endpoints
* **Feature**: Traffic flow visualization, ingress routing rules, and security policies.
* **Backend Architecture**:
  * *Desktop Client*: `client-go` API calls to `networking.k8s.io` to fetch rules.

---

## 6. Storage Views

### Volume Claims (PVCs), Volumes (PVs), StorageClasses
* **Feature**: Capacity visualization ("Liquid Glass" effect) showing requested vs available vs used storage.
* **Backend Architecture**:
  * *Desktop Client*: Gets PVC/PV sizing from `client-go`.
  * *Agent*: To get **actual usage** (e.g., 5GB used out of 10GB requested), the Agent must scrape the Kubelet volume stats API (`/stats/summary`) and serve this enriched data back to the client.

---

## 7. Operations Views

### Runbooks & Incident Log
* **Feature**: Executable markdown runbooks and tracking of cluster incidents.
* **Backend Architecture**:
  * *Desktop Client*: Stores incident metadata locally (e.g., SQLite via `gorm`) or syncs it to a remote DB.

### Workflows
* **Feature**: Visual node-based workflow builder for cluster operations.
* **Backend Architecture**:
  * *Desktop Client*: Saves workflow JSON definitions to disk. A custom Go execution engine parses the DAG and runs the specific `kubectl` or API commands sequentially.

---

## 8. Knowledge Views

### S3 Notebooks (Obsidian Style)
* **Feature**: WYSIWYG Markdown editor with syntax highlighting, synced to an S3 bucket.
* **Backend Architecture**:
  * *Desktop Client*: Uses `aws-sdk-go-v2`. When a file is saved, the Wails backend intercepts the content, writes it to a local cache directory (for offline support), and asynchronously uploads it to the configured AWS S3 bucket. Supports directory structures by mapping S3 prefixes to local folders.
