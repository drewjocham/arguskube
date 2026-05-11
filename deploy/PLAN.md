# Argus — Deployment Architecture Plan

## 1. System Overview

Argus is an **SRE console** for Kubernetes operations. It runs in three modes:

| Mode | Entry Point | Target |
|------|-------------|--------|
| **Desktop** | `backend/main.go` (Wails v2) | Native macOS app |
| **SaaS** | `backend/cmd/server/main.go` (HTTP) | Kubernetes / Docker |
| **Agent** | `agent/cmd/agent/main.go` (DaemonSet) | In-cluster per node |

```
# this plan is not even considering atomstack as a local launch option.
┌─────────────────────────────────────────────────────────────────┐
│                        Argus SaaS                         │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────────┐  │
│  │ Frontend │  │ Backend  │  │ Alert    │  │ MCP Server     │  │
│  │ (Vue/NGINX)│  │ (Go API) │  │ Ingress  │  │ (K8s AI Tools) │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └───────┬────────┘  │
│       │              │              │                │          │
│  ┌────┴──────────────┴──────────────┴────────────────┴──────┐   │
│  │                    Kubernetes Cluster                     │   │
│  │  ┌─────────────────────────────────────────────────────┐  │   │
│  │  │  Agent (DaemonSet — one per node)                   │  │   │
│  │  │  Informers → HTTP/SSE → Tunnel → Backend            │  │   │
│  │  └─────────────────────────────────────────────────────┘  │   │
│  └───────────────────────────────────────────────────────────┘   │
│                                                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                      │
│  │Prometheus│  │ Grafana  │  │ Ollama   │                      │
│  │(metrics) │  │(dashboards)│  │(local LLM)│                      │
│  └──────────┘  └──────────┘  └──────────┘                      │
└─────────────────────────────────────────────────────────────────┘
```

## 2. Component Responsibilities

### 2.1 Backend (`backend/`) — Go HTTP API
**Port:** 8080 | **Stateless:** Yes (SQLite per pod for dev; PG in prod)

| Responsibility | Implementation |
|---------------|---------------|
| Kubernetes resource CRUD | `internal/k8s/` — client-go wrapper for pods, deployments, nodes, etc. |
| AI diagnostics | `internal/ai/` — DeepSeek client + agent (or Ollama fallback) |
| Anomaly detection | `internal/anomaly/` — Anomstack HTTP client |
| ArgoCD integration | `internal/argocd/` — REST API client for sync/rollback |
| Cluster audit | `internal/popeye/` — Popeye runner |
| Vulnerability scanning | `internal/vulnscan/` — Trivy wrapper |
| Incident management | `internal/incidents/` — SQLite CRUD |
| Runbook management | `internal/runbooks/` — Markdown CRUD store |
| Workflow engine | `internal/workflows/` — DAG definition + execution |
| Notebook storage | `internal/notebooks/` — S3-backed markdown |
| Agent tunnel | `api/pkg/hub.go` — WebSocket for in-cluster agents |
| Feature gating | `internal/features/` — Tier-based (free/pro) |
| Telemetry | OpenTelemetry metrics + slog logging |

**Dependencies:** K8s API, SQLite (dev) / PostgreSQL (prod), S3-compatible storage, Anomstack, DeepSeek API

### 2.2 Frontend (`view/`) — Vue 3 SPA
**Port:** 3000 (nginx) | **Stateless:** Yes

| Responsibility | Implementation |
|---------------|---------------|
| UI rendering | Vue 3 Composition API, Pinia stores |
| API bridge | `callGo()` — Wails bindings or HTTP fallback |
| Real-time metrics | SSE polls every 1.5s (live mode) |
| Terminal | xterm.js via WebSocket PTY |
| Markdown editing | Tiptap editor |
| Dashboard | MetricsExplorer with dynamic charts |
| Notebooks | S3 file tree with drag-drop |

**Dependencies:** Backend API (HTTP/WS)

### 2.3 In-Cluster Agent (`agent/`) — Go DaemonSet
**Port:** 8080 | **Per-node:** Yes (DaemonSet)

| Responsibility | Implementation |
|---------------|---------------|
| Resource watching | `internal/k8s/` — SharedInformerFactory for pods, nodes, deployments, services, events |
| Metrics streaming | `/stream/metrics` — SSE endpoint, 5s intervals |
| Resource API | `/api/v1/pods`, `/api/v1/nodes`, `/api/v1/deployments`, `/api/v1/services` |
| Anomaly scores | `/api/v1/anomalies` |
| Topology graph | `/api/v1/topology` — Node-edge graph |
| Tunnel | WebSocket client → Backend hub |

**Dependencies:** K8s API (in-cluster), Backend (for SaaS tunnel)

### 2.4 Alert Ingress (`alert-ingress/`) — Go Webhook Receiver
**Port:** 8080 | **Stateless:** Yes

| Responsibility | Implementation |
|---------------|---------------|
| Webhook ingestion | `/webhooks/anomstack` — HTTP endpoint |
| Alert publishing | GCP PubSub (prod) or stdout (dev) |
| Decoupling | Buffers alerts between Anomstack/Grafana and downstream processors |

**Dependencies:** GCP PubSub (optional)

### 2.5 MCP Server (`mcp/`) — Go AI Tools
**Port:** 8080 | **Stateless:** Yes

| Responsibility | Implementation |
|---------------|---------------|
| K8s diagnostic tools | NodeStatus, PodResources, PodLogs, ClusterAnalysis |
| Alert processing | Alert store, incident history, recommendation engine |
| MCP protocol | stdio JSON-RPC or HTTP transport |
| Pod tracking | Informer-based pod state watcher |

**Dependencies:** K8s API, SQLite

### 2.6 Anomstack (`anomstack/`) — Python Anomaly Detection
**Port:** 8087 | **Stateful:** Yes (Dagster DB)

| Responsibility | Implementation |
|---------------|---------------|
| ML anomaly detection | PyOD models (Z-score, moving averages, isolation forest) |
| Metric ingestion | SQL, Python, API, Prometheus sources |
| Alert generation | Webhook → Alert Ingress |
| Pipeline orchestration | Dagster |

**Dependencies:** PostgreSQL (Dagster), Prometheus

### 2.7 Monitoring Stack
| Component | Role | Storage |
|-----------|------|---------|
| **Prometheus** | Metrics collection + alerting | Local PV (30d retention) |
| **Grafana** | Dashboards + visualization | Local PV (sqlite) |
| **Ollama** | Local LLM (fallback AI) | Local PV (model cache) |

## 3. Communication Flows

```
Frontend ──HTTP──▶ Backend ──client-go──▶ K8s API
                        │
                        ├──HTTP──▶ Anomstack
                        ├──HTTP──▶ DeepSeek API
                        ├──HTTP──▶ Ollama
                        ├──HTTP──▶ ArgoCD API
                        ├──WS────▶ Agent (tunnel)
                        └──HTTP──▶ S3 (notebooks)

Agent ──HTTP/SSE──▶ Nodes (kubelet metrics)
Agent ──WS────────▶ Backend (tunnel)

Alert Ingress ◀──Webhook── Anomstack / Grafana
Alert Ingress ──PubSub──▶ GCP (prod)
                 ──stdout─▶ Logs (dev)
```

## 4. Scaling Strategy

### 4.1 Development (Scale to 0 at Night)
| Component | Dev Replicas | Scale-to-0 | Notes |
|-----------|-------------|------------|-------|
| Backend | 1 | ✅ | No need for HA in dev |
| Frontend | 1 | ✅ | Stateless, nginx |
| Agent | DaemonSet | ❌ | Per-node; skip in dev clusters |
| Alert Ingress | 1 | ✅ | No traffic in dev |
| MCP | 1 | ✅ | No traffic in dev |
| Anomstack | 0 | ✅ | Optional; use if testing anomaly detection |
| Prometheus | 1 | ✅ | Not needed if no cluster |
| Grafana | 1 | ✅ | Not needed if no cluster |
| Ollama | 0 | ✅ | Use DeepSeek API instead |

**Dev config:** `helm install argus -f values-dev.yaml`

### 4.2 Production
| Component | Min | Max | Trigger |
|-----------|-----|-----|---------|
| Backend | 2 | 20 | CPU > 70% + request rate |
| Frontend | 2 | 10 | CPU > 70% |
| Agent | DaemonSet | — | Scales with cluster nodes |
| Alert Ingress | 2 | 10 | Request rate |
| MCP | 1 | 5 | Request rate |
| Prometheus | 1 | — | StatefulSet + persistent storage |
| Grafana | 1 | 2 | CPU > 70% |

### 4.3 Multi-Scale Ingestion Pipeline (50K+ Clusters)

For the target of 50,000+ monitored clusters, the architecture decouples ingestion from storage:

```
Edge (vmagent) ──Remote Write──▶ LB ──▶ Vector (Receiver) ──▶ Kafka
                                                                │
                                              ┌─────────────────┼─────────────────┐
                                              ▼                 ▼                 ▼
                                        Flink (Anomaly)   TSDB (Mimir)     Alertmanager
                                              │                 │                 │
                                              ▼                 ▼                 ▼
                                        Alert Topic        Grafana           PagerDuty
```

**Components at scale:**
- **vmagent** — 50MB RAM per agent, disk-buffered, stateless forwarding
- **Vector** — 10-20 replicas, stateless, receives + decompresses + publishes to Kafka
- **Kafka/Redpanda** — Partitioned by tenant_id, 72h retention for replay
- **Flink** — Stateful stream processing, sliding windows for anomaly detection
- **Grafana Mimir** — Horizontally scalable TSDB with object storage backend
- **Alertmanager** — Deduplication + grouping + routing

For the initial release, the simpler path (Prometheus → alert-ingress → anomstack) is sufficient and can evolve into the Kafka-based pipeline as cluster count grows.

## 5. Resource Requirements

### Development (Scale-to-0)
| Component | CPU | Memory | Storage |
|-----------|-----|--------|---------|
| Backend | 100m | 128Mi | 1Gi (SQLite) |
| Frontend | 50m | 64Mi | — |
| Agent | 100m | 64Mi | — |
| Alert Ingress | 50m | 32Mi | — |
| MCP | 50m | 64Mi | — |
| Prometheus | 200m | 512Mi | 8Gi |
| Grafana | 100m | 128Mi | 1Gi |

### Production
| Component | CPU | Memory | Storage |
|-----------|-----|--------|---------|
| Backend | 500m | 512Mi | 10Gi (PG) |
| Frontend | 200m | 256Mi | — |
| Agent | 200m | 128Mi | — |
| Alert Ingress | 100m | 128Mi | — |
| MCP | 200m | 256Mi | 1Gi |
| Prometheus | 1 | 2Gi | 50Gi |
| Grafana | 200m | 256Mi | 10Gi |
| Ollama | 2 | 8Gi | 20Gi |

## 6. Security

- **RBAC:** Minimal per-service ClusterRoles (read-only for backend + MCP, node-level for agent)
- **Network Policies:** Deny-all default, allow specific ingress/egress between components
- **TLS:** In-cluster service mesh (mTLS) or cert-manager for ingress
- **Secrets:** External Secrets Operator with AWS Secrets Manager / GCP Secret Manager
- **Pod Security:** `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, drop all capabilities
- **Audit:** All K8s API calls logged via audit client wrapper

## 7. Observability

- **Metrics:** OpenTelemetry + Prometheus `/metrics` endpoints on all Go services
- **Logs:** Structured JSON via `slog` — `component`, `error`, `duration` fields
- **Traces:** OpenTelemetry distributed tracing (future)
- **Health:** `/health` endpoints with dependency status (degraded mode supported)
- **Dashboards:** Grafana dashboards for each component

## 8. Directory Structure

```
deploy/
├── PLAN.md                                    # This document
├── helm/
│   ├── argus-backend/                   # Backend HTTP API
│   ├── argus-frontend/                  # Vue SPA + nginx
│   ├── argus-agent/                     # In-cluster DaemonSet
│   ├── argus-alert-ingress/             # Webhook receiver
│   ├── argus-mcp/                       # MCP AI tools server
│   └── argus-monitoring/                # Prometheus + Grafana + Ollama
└── terraform/
    ├── providers.tf                           # AWS/GCP/K8s providers
    ├── variables.tf                           # Input variables
    ├── main.tf                                # Cluster + VPC + networking
    ├── helm.tf                                # Helm release management
    ├── monitoring.tf                          # Prometheus + Grafana operator
    └── outputs.tf                             # Connection info
```
