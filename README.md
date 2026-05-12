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

## Releasing

Argus ships to macOS via a Homebrew cask. A push of a SemVer tag triggers
`.github/workflows/release.yml`, which builds a universal (arm64 + x86_64)
`.app` bundle, ad-hoc signs it, zips it with `ditto`, and publishes the
archive together with a rendered `argus.rb` cask file as GitHub Release
assets.

```bash
# From main, with a clean tree:
git tag v0.1.0
git push origin v0.1.0
# Wait for the Release workflow to finish, then verify the assets at:
#   https://github.com/drewjocham/arguskube/releases/tag/v0.1.0
```

### Install from a Homebrew tap

If `vars.HOMEBREW_TAP_REPO` and `secrets.HOMEBREW_TAP_TOKEN` are configured
on this repo, the release job also pushes the updated cask to that tap.
End users can then:

```bash
brew tap drewjocham/tap
brew install --cask argus
```

### Install directly from a Release

Without a tap, the cask file is still uploaded to the Release. Install with:

```bash
brew install --cask https://github.com/drewjocham/arguskube/releases/download/v0.1.0/argus.rb
```

### Setting up the tap (one-time)

1. Create a public repo named `homebrew-tap` (the prefix is mandatory).
2. On this repo: set `HOMEBREW_TAP_REPO` (Variables) to `<owner>/homebrew-tap`
   and `HOMEBREW_TAP_TOKEN` (Secret) to a fine-grained PAT with
   `contents: write` on the tap repo.
3. Next tag push will populate `Casks/argus.rb` in the tap automatically.

### Signing

The release uses ad-hoc signing (no Apple Developer ID). Gatekeeper accepts
ad-hoc signed apps when launched via `brew install --cask` because the
cask's `postflight` clears the quarantine attribute. To upgrade to a notarized
build, add an Apple Developer cert + notarytool step before the `codesign`
call in the workflow.

## License

© 2026 Argus Infrastructure
