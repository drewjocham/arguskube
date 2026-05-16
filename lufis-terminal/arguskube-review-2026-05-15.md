# ArgusKube / KubeWatcher — Comprehensive Code Review

**Date**: 2026-05-15  
**Repo**: `arguskube` — `d4ba839` (latest `main`)  
**Reviewers**: Architect, Test Generator, Functional Tester (parallel analysis)  
**Scope**: Production readiness, maintainability bottlenecks, test coverage, AI integration, task/job automation

---

## 1. Production Readiness by Module

### 1A. MCP Server (`kube/mcp/`) — **Prototype-grade**
The cleanest new addition structurally, but every component has "we'll fix it later" on it.

| Issue | File | Severity |
|---|---|---|
| In-memory alert store (100 cap) — data lost on restart | `server/server.go` | 🔴 High |
| Stdio transport only — no HTTP/SSE, every client spawns a subprocess | `server/server.go` | 🔴 High |
| No health endpoint, no metrics, no graceful shutdown | `server/server.go` | 🟡 Med |
| `capturePodData` goroutine unbounded — leaks under pod-flapping | `server/server.go:251-295` | 🟡 Med |
| Append-only file history store — no compaction, no retention, no index | `monitoring/history/store.go` | 🟡 Med |

**What to do**: Add SQLite-backed persistence (reuse `internal/dbconfig/` patterns from the main backend), migrate to HTTP/SSE transport for MCP, cap goroutines with a worker pool, and add a `/_health` endpoint.

### 1B. Broker Abstractions (`pkg/broker/`) — **Over-engineered for current need**
6 protocol adapters (PubSub, NATS, Kafka, RabbitMQ, AMQP1, REST) — well-crafted interfaces and plugin registry. But only the load test engine consumes it, and there's no `Subscriber`/`Consumer` interface.

**Verdict**: Premature generalization. ~1,800 lines of production Go for one feature. If you only need 1-2 brokers today, trim and defer the registry pattern.

### 1C. Load Test Engine (`pkg/loadtest/`) — **Solid core, incomplete edges**
The `Engine.Run()` method is cleanly factored (`preScale → publish → postScale`), recorder is thread-safe, ramp planner self-contained.

- **🚨 Security**: The scalar directly modifies deployments. No RBAC check, no approval workflow. On a production cluster this needs audit logging.
- **No distributed coordination** — the "distributed" in `app_distload.go` is aspirational. Local runner + an HTTP stub to a non-existent SaaS.
- **Analysis narrator** smartly falls back to deterministic Markdown when no DeepSeek key is set.

### 1D. Alert Processing (`internal/alertproc/`) — **Most production-ready module**
Signature-based dedup, fatigue detection, agent profile gating, investigation scheduling, SQLite persistence.

**Minor gaps**:
- `Investigations` and `Silence/MarkIgnored` do linear scans — O(N) per op, lock contention under alert storms
- Fatigue detection is heuristic-only: counts silences+ignores, doesn't incorporate severity, time decay, or root cause

### 1E. SpotCheck Engine (`internal/spotcheck/`) — **Production-ready**
Minimal, testable, clean `Check → Finding → Notifier` chain. Keep investing here.

### 1F. Workspace Encrypted Connections (`internal/workspace/`) — **Foundation only**
Clean AES-GCM crypto + SQLite token storage. Phase 1A only: listing, deleting, OAuth start/complete. Zero service operations — no Docs read, no Slack send, no Sheets append. The UI exists but delivers nothing.

### 1G. Anomstack ML — **Python pet project, should be deferred**
Standalone Flask/Dash app doing basic threshold-based moving averages. Problems:
- No Python in build pipeline — runs as a separate server process
- `Detect` call sends metric name + window (not data) — tight coupling: desktop app and Anomstack need same metrics DB
- `flink.go` is an empty shell
- **No Python runtime in the Wails build** — this must always be a sidecar

**Recommendation**: Pull the Go client abstraction back until the Python backend is proven in real use.

### 1H. SaaS Integration — **Mock grade**
`LoginSaaS` returns `"mock-jwt-token-from-" + provider`. The `Client` has real circuit breaker, real retry backoff, real error types — all for a non-existent API. Infrastructure hunting for a real endpoint.

### 1I. Agent Directory — **No production review possible, untested**
In-cluster Go agent (`agent/`) — 6 source files, 0 test files. Python agent modules — 21 source files, 0 test files. The agent runs *in the cluster* — untested code can silently corrupt CD operations, tunnel connections, or K8s resource state.

---

## 2. Maintainability Bottlenecks

### 2A. The `api/pkg/` Monolith (35 handler files)
Single biggest maintainability problem. `app.go` is the Wails entry point binding ~80 methods. `App` struct has **38 fields**, constructor is 50+ assignments.

**Problems**:
- **Strong coupling** — `alertproc` needs `db`, `agent`, `assembler`, `logger`. `workspace` needs `auth.store`. New features pull in more of App.
- **Init order fragility** — `startAlertProcessor`, `startSpotChecks`, `startEventLoop` depend on field init order with no explicit graph
- **Testing burden** — testing `DiagnoseAlert` requires a fully constructed App
- **The `App` struct keeps growing** — every new feature = 1 field + 1 constructor param + 1 handler file

**What to do**: Extract subsystems behind interfaces, use dependency injection, split into domain-focused controllers. The `App` should orchestrate, not accumulate.

### 2B. Frontend: 120+ Vue Components, 0 Unit Tests
The Vue frontend is large and complex — 120+ components across 20+ directories, stores, composables — with **zero component unit tests**. The single e2e smoke test (Playwright, 8 cases) only covers the static shell layout (titlebar, sidebar nav, collapse toggle) — it never interacts with data.

**High-risk untested components**:

| Component | Risk |
|---|---|
| `DeploymentList.vue`, `PodList.vue`, `JobCronJobList.vue` | Primary workload UIs |
| `TopologyMap.vue`, `NetworkPolicyList.vue` | Complex rendering |
| `VulnerabilityList.vue` | Security-critical |
| `LogExplorer.vue`, `LogStream.vue`, `MetricsExplorer.vue` | Real-time data display |
| `FinOpsView.vue`, `WasteHeatmap.vue` | Cost analysis |
| `SetupChecklist.vue`, `SetupPanel.vue` | First-run experience |
| `WorkflowEditor.vue`, `PipelinesView.vue` | Automation views |
| `ResourceDetail.vue`, `ResourceTable.vue` | Core detail/tables |

### 2C. Anomstack: 23 Test Files for ~102 Python Source Files
Untested modules present critical risk — data deletion, alert routing, LLM agent:

| Untested Module | Risk |
|---|---|
| `alerts/send.py`, `alerts/webhook.py` | Silent alert failures |
| `jobs/cleanup.py`, `retention.py`, `delete.py` | Could delete production data |
| `jobs/llmalert.py` | LLM alert generation untested |
| `ml/preprocess.py` | Core ML pipeline, no preprocessing coverage |
| `llm/agent.py` | LLM agent integration |
| `metricstore/snowflake.py` | Production DB connector |

---

## 3. Test Coverage Deep-Dive

### 3A. Go Backend — **Strong (68% file coverage)**
~128 test files across `kube/backend/` and `kube/mcp/`. Patterns:
- **Table-driven tests** with clear `{name, fields, args, want, wantErr}` structure (best practice ✅)
- Some browser/auth tests use **test suites** (x/nhooyr.io/websocket)
- **Excellent error-scenario coverage** in broker tests — tests for timeouts, connection errors, JSON decode failures

**Standouts**: broker tests are thorough, loadtest recorder tests are tight.

**Gaps**:
- **SaasAPI client** — only 1 test file, no circuit breaker tests
- **Anomaly package** (`anomstack.go`, `settings.go`, `flink.go`) — each has a test, but anomstack tests mock HTTP responses, never test the adapter lifecycle
- **Vulnerability scanner** — only an integration test, no unit tests for the scanning logic itself
- **Auth handler tests** — cover login flow but not token refresh, revocation, or concurrent sessions

### 3B. Frontend — **Near-zero**
- **3 store tests** out of ~12+ stores (`distload`, `notificationGuard`, `watcherRegistry`)
- **2 composable tests** out of ~10+ (`useArgusAlertContext`, `useWatcherEngine`)
- **0 component tests** out of 120+ components
- **0 view/router tests**
- **1 e2e smoke test** (shell layout only)

**This is the biggest risk in the project.** Frontend code changes blindly. Pinia stores are testable, vue-test-utils is in the stack. Priority targets: `DeploymentList`, `PodList`, `TopologyMap`, and the shared `ResourceTable` pattern.

### 3C. In-Cluster Agent — **No tests at all**
6 Go source files, 21 Python source files — 0 test files. The code that actually changes the cluster has zero safety net.

### 3D. API-GOV — **Weak coverage**
Go backend has tests; Python agent side is untested.

---

## 4. AI Integration Opportunities & Gaps

### Current AI Surface

| Layer | Status | Notes |
|---|---|---|
| `internal/ai/agent.go` — DeepSeek chat agent | ✅ Working | Clean context bundle, conversation management, auto-summary |
| `internal/ai/deepseek.go` — API client | ✅ Working | Retry, rate limiting, streaming support |
| MCP Server (`kube/mcp/`) | ⚠️ Prototype | Stdio-only, no persistence — foundation for more |
| `LoadTest narrator` — AI analysis reporter | ✅ Working | Falls back to deterministic Markdown |
| Anomstack LLM alerts | ⚠️ Untested | `llm/agent.py` and `jobs/llmalert.py` untested |
| Frontend AI components | ✅ UX-ready | `ArgusAIChat.vue`, `CodeBlock.vue`, `ArgusSuggestionCard.vue`, `ChatPopOut.vue` |
| FEATURES.md AI features | ✅ Documented | AI Network Profiler, AI Storage Optimizer, Security Advisor — all not connected to real AI |

**The frontend is ready for AI integration. The backend is mostly there. The missing piece is making the MCP server production-grade.**

### AI Integration Recommendations

1. **Productionize MCP Server** — Add persistence, HTTP transport, health endpoint, goroutine worker pool. This is the AI control plane.
2. **Connect frontend AI features** — `ArgusSuggestionCard`, `CodeBlock`, `AgentAnalysisNotification` all render AI output but have no backend wiring. Route through the MCP server.
3. **Unify the AI orchestration** — Currently there are 2+ AI entry points (Wails app → DeepSeek vs MCP server → tools). Pick one control path and deprecate the other.
4. **Build the consumer side of broker** — A `Subscriber` interface unlocks real-time AI-driven alert correlation via event streams.

---

## 5. Automated Task/Job Systems

### Current State

| System | Status |
|---|---|
| CronJobs / Scheduled Scans | Partially implemented in `internal/anomstack` |
| Pipeline/Workflow UI | `WorkflowEditor.vue`, `PipelinesView.vue` — UI exists, no backend |
| SpotCheck periodic checks | ✅ Production-ready engine |
| Alert processing pipeline | ✅ Near-production processor |
| Load test scheduling | Engine exists, scheduler missing |

### Gaps

- **No centralized scheduler** — cron-like scheduling is scattered across modules
- **Pipeline/Workflow backend doesn't exist** — the Vue components exist but have no Go backend
- **No DAG/retry/persistence** for task definitions — each system invents its own
- **No hooks/triggers** — alert → run workflow → verify is manual
- **Anomstack alert jobs** (`jobs/alert.py`) are runtime-loaded modules, not configurable via API

### Recommendations

1. **Audit and rationalize** — don't build a "task system" yet. Document what exists (SpotCheck, alert cycle, scheduled scans, load test runs).
2. **Add a `TaskStore` interface** — persistence for scheduled tasks. Reuse `internal/dbconfig/` SQLite patterns.
3. **Plumb WorkflowEditor backend** — it's wired as a Vue form with no API endpoint. Add `app_workflows.go` with basic CRUD.
4. **Consider Dagster** — your existing toolchain already has Python knowledge. Dagster would give you DAG scheduling, retries, and observability without inventing a new system.

---

## 6. Critical Fixes (Priority Order)

### 🔴 Do Before Shipping
1. **Secure the load scalar** — add RBAC check or approval gate before `scale deployment to 0`
2. **In-cluster agent tests** — 0 tests for code that mutates cluster state
3. **Frontend component tests** — start with `DeploymentList`, `PodList`, `ResourceTable`
4. **MCP server persistence** — in-memory alert store loses data on every restart
5. **Anomstack data lifecycle** — `cleanup.py`, `retention.py`, `delete.py` are untested

### 🟡 Do Soon
6. **App struct decomposition** — 38 fields and growing, extract domain controllers
7. **Broker trim or extend** — add `Subscriber` interface if you need it, otherwise trim to 1-2 protocols
8. **Anomstack test coverage** — alerts/send, webhook, LLM agent, preprocess, Snowflake connector
9. **SaasAPI circuit breaker tests** — infrastructure coded but not tested
10. **MCP goroutine capping** — `capturePodData` unbounded

### 🟢 Nice-to-Have
11. **Centralized task scheduler** — after SpotCheck+alert+workflows stabilize
12. **Python in Wails build** — if Anomstack stays, it needs a deterministic deployment story
13. **Workspace service operations** — the OAuth UI needs to actually *do* something
14. **SaaS endpoint** — or remove the mock and make the feature opt-in

---

## 7. Summary Grades

| Module | Production Grade |
|---|---|
| `internal/spotcheck/` | ✅ Production-ready |
| `internal/alertproc/` | ✅ Almost production (minor gaps) |
| `internal/ai/` | ✅ Functional (DeepSeek integration) |
| `internal/k8s/` | ✅ Solid (with tests) |
| `pkg/broker/` | ⚠️ Over-engineered for usage |
| `pkg/loadtest/` | ⚠️ Core is solid, edges need hardening |
| `internal/workspace/` | ⚠️ Foundation only, no service ops |
| `kube/mcp/` | 🚧 Prototype-grade |
| `internal/anomstack/` (Go) | 🚧 Stub for untested Python backend |
| `internal/saasapi/` | 🚧 Mock grade |
| `agent/` (Go + Python) | 🚧 Untested |
| `anomstack/` (Python) | 🚧 Pet project, weak coverage |
| `kube/view/src/components/` | 🚧 120 components, 0 unit tests |
| **Project Overall** | **🚧 Strong foundation, insufficient testing on edges. Ship-blocking issues: agent tests, frontend tests, load scalar security.** |

---

*Full review session data archived in workspace memory. Detailed file-level findings available on request.*
