# Argus — SRE Console

A native macOS desktop application for Kubernetes operations. Built with [Wails v2](https://wails.io) (Go + Vue 3).

## Quick Start

```bash
# Prerequisites: Go 1.26+, Node 22+, Wails CLI
# Install Wails: go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Build
cd backend && make build

# Run in dev mode (hot-reload frontend)
cd backend && wails dev
```

## Architecture

Argus runs as a native macOS binary with two communication modes:

- **Desktop mode** — Vue frontend calls Go directly via Wails bindings (`window.go.api.pkg.App.Method()`)
- **SaaS mode** — Same methods exposed as HTTP endpoints on port 8080 (`POST /api/{MethodName}`)

The frontend `callGo()` bridge auto-detects the available mode and falls back transparently.

```
┌─────────────────────────────────────────────────┐
│  macOS App (Wails v2)                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────────┐  │
│  │ Sidebar  │  │ Center   │  │ AI / Detail  │  │
│  │ Nav +    │  │ Panel    │  │ Panel        │  │
│  │ Context  │  │ (Router) │  │ (Diagnostics)│  │
│  └────┬─────┘  └────┬─────┘  └──────┬───────┘  │
│       └──────────────┴───────────────┘          │
│                    callGo()                      │
│                      │                           │
│  ┌───────────────────┴───────────────────────┐  │
│  │  Go Backend (api/pkg/app.go)              │  │
│  │  K8s · AI · Notebooks · Runbooks ·        │  │
│  │  Incidents · Workflows · Popeye · Vulns   │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
         │                          │
    kubeconfig               In-Cluster Agent
    (direct API)             (DaemonSet, optional)
```

## Features

**Monitoring** — Metrics explorer, log search, anomaly detection, vulnerability scanning, cluster audit (Popeye), topology map

**Cluster** — Nodes, namespaces, events with auto-refresh

**Workloads** — Pods (with logs, delete, filter), deployments, statefulsets, daemonsets, jobs, cronjobs

**Config & Network** — ConfigMaps, secrets, HPAs, services, ingresses, network policies, endpoints

**Storage** — PVCs, PVs, storage classes

**Operations** — Markdown runbooks (CRUD + auto-save), incident log (timeline + manual entry), workflow builder (visual step editor), ArgoCD integration

**Knowledge** — S3-backed notebooks with drag-drop file tree

**AI** — DeepSeek-powered diagnostic agent for alert analysis and remediation suggestions

## Project Structure

```
backend/
├── api/pkg/          # App struct (Wails bindings + HTTP API)
├── internal/
│   ├── ai/           # DeepSeek client + agent
│   ├── anomaly/      # Anomstack integration
│   ├── config/       # Environment-based config
│   ├── context/      # Context assembly for AI
│   ├── features/     # Feature gating by tier
│   ├── incidents/    # JSON file persistence
│   ├── k8s/          # Kubernetes client wrapper
│   ├── notebooks/    # S3 / local notebook store
│   ├── popeye/       # Cluster audit runner
│   ├── runbooks/     # Markdown CRUD store
│   ├── setup/        # One-click tool installer
│   ├── vulnscan/     # Trivy image scanner
│   └── workflows/    # Workflow CRUD + execution
├── agent/            # In-cluster agent (separate go.mod)
├── pkg/              # Shared packages (kube client, audit, logging, watch)
└── mcp/              # MCP server integration

view/
├── src/
│   ├── composables/  # useWails.js (Wails/HTTP bridge + all domain composables)
│   └── components/   # Vue 3 SFCs organized by sidebar section
└── vite.config.js
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig |
| `DEEPSEEK_API_KEY` | — | Enables AI diagnostic agent |
| `ANOMSTACK_URL` | — | Anomaly detection backend |
| `POPEYE_BINARY` | — | Popeye path (or install via Setup panel) |

## Testing

```bash
# Go backend
cd backend && go test -race ./...

# Vue frontend
cd view && npm run test:run
```

## Agent Pipeline

Argus includes an automated development pipeline at `.argus/hooks/kube-pipeline.js`. It runs specialized sub-agents before and after each session:

- **Pre-flight** — Guard-rail agent reads `.context.md`, warns about fragile code
- **Post-session** — Build check → Test suite → QA sweep → Architecture review → Test generation → Context update

See [.context.md](.context.md) for architectural decisions and known debt.

## License

© 2026 Argus Infrastructure
