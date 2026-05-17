# api-gov — Production Readiness Assessment

**Date:** 2026-05-17
**Scope:** `api-gov/` component (Go backend, Python middleware, Python LangGraph agents, K8s deployment)

### Project-Wide Code Guidelines

All code in this repository must follow:
- **TypeScript only** — `.ts` files, never `.js`. This applies to the Vue frontend (`kube/view/`) and any other TypeScript-capable context. (api-gov is Go + Python, so this rule is informational here.)
- **DRY & Design Patterns** — Repeated logic must be abstracted using the patterns already established in the codebase: **Strategy Pattern** for interchangeable algorithms, **Adapter Pattern** for external integrations, and clean **Handler → Service → Store** layering. No code duplication, no reinventing wheels.
- **Self-documenting code** — Structure and naming should explain intent. Comments reserved for the "why" behind complex decisions.

---



## Verdict: ❌ Not Production-Ready

The codebase is well-architected with clean layering, OpenTelemetry instrumentation, and comprehensive K8s manifests — but it has **critical blockers** in security, reliability, and test coverage that must be addressed before production.

---

## 🔴 Critical Blockers (Must Fix Before Prod)

| # | Issue | Severity | File(s) | Detail |
|---|-------|----------|---------|--------|
| 1 | **No authentication on any backend endpoint** | Critical | `backend/internal/api/middleware.go`, `api.go` | Backend has zero auth middleware. CORS is `Allow-Origin: *`. The middleware sends `Authorization: Bearer {api_key}` but the backend never validates it. Any client can call any endpoint including GDPR admin routes. |
| 2 | **Fire-and-forget goroutines run with cancelled context** | Critical | `backend/internal/api/handlers.go:89,288,339,424` | All `go a.agentCli.XXX(ctx, ...)` calls pass the HTTP request context, which is cancelled as soon as the handler returns `202 Accepted`. Every goroutine immediately receives a cancelled context — traffic ingestion, drift scans, and spec analysis all fail silently. |
| 3 | **Batch traffic handler spawns 500 goroutines per request** | Critical | `backend/internal/api/handlers.go:410-426` | `handleIngestTrafficBatch` spawns one goroutine per sample (up to 500). No semaphore, worker pool, or rate limiter. Under concurrent load this causes OOM, TCP exhaustion, and thundering herd on the agent service. |
| 4 | **No retry on agent client calls** | Critical | `backend/internal/service/agent.go:80-127` | `AgentClient.post` has zero retry logic. Any transient network failure (DNS hiccup, connection reset, agent restart) permanently drops the operation. |
| 5 | **`IngestTraffic` silently swallows ALL errors** | Critical | `backend/internal/service/agent.go:143-160` | Returns `void`. Errors are logged and permanently lost. No dead-letter queue, no retry, no re-queuing. Traffic data loss is guaranteed under load. |
| 6 | **Env var mismatch: `LLM_API_KEY` vs `DEEPSEEK_API_KEY`** | Critical | `backend/internal/config/config.go:34`, `docker-compose.yml:42,55` | Go backend expects `LLM_API_KEY` but docker-compose sets `DEEPSEEK_API_KEY`. Running via `docker compose up`, the Go API starts with an empty LLM key. |
| 7 | **Service layer: 0% test coverage** | Critical | `backend/internal/service/` (316 lines, 3 files) | `SpecService`, `DriftService`, and `AgentClient` have zero tests. Business validation, GDPR deletion, and all agent communication are untested. |
| 8 | **Database CRUD: near 0% test coverage** | Critical | `backend/internal/database/specs.go`, `drift.go` (396 lines) | All `SpecStore`, `EndpointStore`, and `DriftStore` CRUD operations are untested. SQL injection vectors and parameter ordering mismatches would ship undetected. |
| 9 | **Python anomaly subsystem: 0% test coverage** | Critical | `agents/src/anomaly/` (7 files, 8 modules) | The core intelligence layer — stats, structural drift, API anomaly, batch worker, counters, metadata filter, LLM monitoring — has zero tests. |
| 10 | **Main entrypoint & app wiring: 0% test coverage** | Critical | `backend/cmd/server/main.go`, `backend/pkg/app.go` (120 lines) | Signal handling, graceful shutdown, DB connection failure, migration errors — all untested. A broken shutdown or init failure ships undetected. |

---

## 🟡 High Priority (Should Fix Before Prod)

### Reliability & Correctness

| # | Issue | File(s) | Detail |
|---|-------|---------|--------|
| 11 | **MemorySaver default for all LangGraph agents** | `agents/src/graphs/*.py` | All 6 agent graphs default to in-memory checkpointing. State is lost on agent restart. Needs `RedisSaver(redis_url)` for production. |
| 12 | **Duplicate drift reports from repeated scans** | `backend/internal/database/drift.go:39-61` | `DriftStore.CreateBatch` does plain INSERT with no `ON CONFLICT`. Re-running a drift scan creates duplicate reports. |
| 13 | **Endpoints table lacks unique constraint** | `backend/internal/database/migrations.go:20-33` | No `UNIQUE(spec_id, method, path)` — duplicate endpoint definitions can be inserted. `Upsert` relies on caller-supplied UUID for dedup. |
| 14 | **GDPR FK ordering can violate constraints** | `backend/internal/database/specs.go:110-128`, `migrations.go:38-57` | `alert_history` references `streams` without `ON DELETE CASCADE`, but streams is deleted before alert_history in GDPR cleanup. |
| 15 | **Pagination inconsistency across endpoints** | `backend/internal/api/handlers.go` | List handlers use offset-based pagination (`limit`, `offset`); drift endpoints use page-based (`page`, `page_size`, `total_pages`). Different response shapes. |
| 16 | **LLM API key not validated at agent startup** | `agents/src/config.py:10-12,31-37` | Agents start successfully even with empty `DEEPSEEK_API_KEY`. Only fail at runtime on first LLM call with an auth error. |
| 17 | **`handleAgentGenerateTests` swallows JSON decode errors** | `backend/internal/api/handlers.go:462-463` | Malformed request body silently succeeds (zero-valued struct). Every other handler properly returns 400. The error return from `json.Decode` is ignored. |
| 18 | **`DriftFilter.Offset()` mutates the filter** | `backend/internal/models/drift.go:13-21` | Calling `Offset()` mutates `Page` and `Limit` on the original struct. Calling it twice returns different values. |
| 19 | **Traffic ingestion batch worker lacks retry** | `middleware/src/middleware.py:431-467` | The middleware's `_flush_batch` has no retry. A single transient failure drops the entire batch of samples. |

### DRY & Design Patterns

| # | Issue | File(s) | Detail |
|---|-------|---------|--------|
| D1 | **GDPR deletion logic duplicated in two files** | `internal/database/cleanup.go`, `internal/database/specs.go` | `GDPRDeleteSpec` and `CountUserData` are implemented identically on `DB` (cleanup.go) and `SpecStore` (specs.go). The `cleanup.go` versions are dead code. Should be unified via a repository abstraction following the existing **SpecStore** pattern. |
| D2 | **`logKeyError` constant defined in 3 places** | `internal/api/api.go:31`, `internal/service/agent.go:16`, `internal/apperrors/response.go:15` | Three independent `const logKeyError = "error"` declarations. Should be hoisted to a shared `pkg/logging` or `internal/apperrors` package. |
| D3 | **Middleware context helpers defined but never wired** | `internal/api/context.go`, `middleware.go:21-50` | The `WithRequestContext` middleware exists alongside dead helper getters — but the chi router never registers it. Either wire it into the middleware chain or delete it (Strategy: compose middleware explicitly in `Routes()`). |
| D4 | **Investigator agent implemented but orphaned** | `agents/src/graphs/investigator.py` | Fully built (117 lines) but never imported by `orchestrator.py` or `server.py`. Should be wired as an **Adapter** called by the orchestrator when critical alerts fire, or removed. |
| D5 | **Manual UUID generation scattered across stores** | `internal/database/specs.go:21`, `drift.go:21,47` | `uuid.New().String()` is called in every store's Create method. Could be abstracted into a single `generateID()` helper or handled via DB defaults (the migration already sets `DEFAULT uuid_generate_v4()` for PK columns — the Go code contradicts PK generation by always providing a value). |

### Dead & Unused Code

| # | Issue | File(s) | Detail |
|---|-------|---------|--------|
| 20 | **`context.go` is 100% dead code** | `backend/internal/api/context.go`, `middleware.go:21-50` | 5 context key getters defined (`GetSpecID`, `GetEndpointID`, `GetDriftID`, `GetUserID`, `GetRequestID`) — zero are ever called. `WithRequestContext` middleware is never registered in the router. |
| 21 | **`cleanup.go` is 100% dead code** | `backend/internal/database/cleanup.go` | Both functions (`GetActiveSpecIDs`, `GDPRDeleteSpec`) are duplicated in `specs.go` and never called. The entire file is unreachable. |
| 22 | **`Investigator` agent is dead code** | `agents/src/graphs/investigator.py` | Fully implemented (117 lines) but never imported or wired into any graph or server endpoint. |
| 23 | **Python test imports reference nonexistent functions** | `agents/tests/test_sentinel.py:7` | Imports `buffer_traffic`, `check_threshold`, `score` — none of which exist in `sentinel.py`. Tests fail with `ImportError`. CI is silently green. |

### Security

| # | Issue | File(s) | Detail |
|---|-------|---------|--------|
| 24 | **No rate limiting on any endpoint** | `backend/internal/api/api.go:91-96` | No protection against DoS or accidental high-volume clients. |
| 25 | **Admin routes have no admin authorization** | `backend/internal/api/api.go:116-120` | `/admin/gdpr/` and `/admin/anomaly-metrics/` routes are grouped under `/admin` but have no RBAC, no auth middleware — they're just prefixed normal routes. |
| 26 | **No container vulnerability scanning** | CI/CD, Dockerfiles | No Trivy, Snyk, Grype, or SBOM generation in the pipeline. |
| 27 | **No NetworkPolicies defined** | `k8s/` | No pod-level network isolation between services. |
| 28 | **No PodDisruptionBudget** | `k8s/` | No PDB to prevent all replicas from being evicted simultaneously during node maintenance. |

### Infrastructure

| # | Issue | Detail |
|---|-------|--------|
| 29 | **Image tag `latest` in K8s manifests** | `k8s/10-api-deployment.yaml:24` — static YAML references `:latest`. Not auditable. Should use explicit SHA or Helm-style templating. |
| 30 | **No Ingress / Gateway API configuration** | No TLS termination, no host-based routing, no API gateway. |
| 31 | **No Prometheus ServiceMonitor or Grafana dashboards** | Metrics are emitted via OTLP but not wired into the observability stack. |
| 32 | **No Terraform or Pulumi infrastructure-as-code** | `k8s/` manifests are raw YAML — no IaC for the production deployment itself. |

---

## 🟢 Nice-to-Have (Improve After Launch)

| # | Issue | Detail |
|---|-------|--------|
| 33 | **No `PUT /api/v1/specs/{specID}` endpoint** | Once created, a spec cannot be updated. Users must DELETE and recreate, losing all historical drift data. |
| 34 | **No `DELETE /endpoints/{endpointID}` endpoint** | Individual endpoints cannot be removed without deleting the entire spec. |
| 35 | **No vector search API route** | `VectorSearch` is implemented in `drift.go` but has no user-facing endpoint. The `endpoints.embedding` column exists but is inaccessible. |
| 36 | **`save_drift_reports` uses N+1 INSERT loops** | `agents/src/database.py:63-75` — per-row inserts instead of `executemany` or `execute_values`. Adds latency under load. |
| 37 | **Agent Redis client accessed via private `_pool` attribute** | `batch_worker.py`, `counters.py`, `stats.py` bypass public API and access `redis_client._pool` directly. A facade leak. |
| 38 | **`Dockerfile` uses bare distroless with no non-root user** | `backend/Dockerfile:9-13` — runs as root in `alpine:3.21`. Should `USER nobody` or create a dedicated user. |
| 39 | **No version history or spec diff API** | `api_specs.version` tracks revisions but there's no endpoint to retrieve or compare old versions. |
| 40 | **`AGENT_SILICONFLOW_API_KEY` set in K8s but never read** | `k8s/11-agent-deployment.yaml:39` injects it, but no Python code reads `AGENT_SILICONFLOW_API_KEY`. |

---

## ✅ What IS Production-Ready

| Area | Strength |
|------|----------|
| **Architecture** | Clean layered separation (handlers → services → stores) with Go-Chi router. API versioned at `/api/v1/`. |
| **Observability** | OpenTelemetry instrumentation for traces + metrics. Structured JSON logging via `log/slog`. Health (`/health`) and readiness (`/ready`) endpoints. |
| **Graceful Shutdown** | Signal handling (`SIGINT`, `SIGTERM`) with errgroup, context timeout, and ordered shutdown. |
| **pgvector Embeddings** | Cosine-distance semantic search on endpoints. Vector column properly hidden from JSON responses. |
| **SQL Security** | Custom query builder generates parameterized `$N` SQL — no string concatenation, safe from injection. |
| **K8s Deployment** | Deployment, Service, HPA (3-10 replicas, CPU 70%), rolling update (maxSurge:1, maxUnavailable:0), liveness/readiness/startup probes, PgBouncer sidecar, pgBackRest S3 backups every 5min. |
| **Secrets Management** | External Secrets Operator integration with AWS Secrets Manager. 1h refresh interval. |
| **GDPR Compliance** | Right-to-erasure and data counting endpoints implemented (though untested). Transactional multi-table deletion. |
| **Middleware** | Well-designed body-capture with ASGI receive channel replay. Bounded background queue (256 samples). Batch coalescing (up to 50 samples / 500ms window). Composable lifespan. |
| **Error Handling** | Centralized `WriteHTTPResponse` with disposition-based HTTP status mapping (`Ack → 200`, `BadRequest → 400`, `Retry → 500`). Panic recovery with stack traces. |
| **Testing Pattern** | Go uses table-driven tests (required by project convention). Middleware tests are strong (~70-75% coverage, well-structured with mock transports). |
| **ADR Documentation** | 4 Architecture Decision Records documenting LangGraph rationale, Redis counter design, LLM provider costs, and real-time vs batch boundary. |

---

## Recommended Action Plan

### Phase 0 — Code Quality & DRY (Week 0)

0. Consolidate duplicated GDPR logic — remove `cleanup.go`, keep `SpecStore.GDPRDelete`
1. Hoist shared constants (`logKeyError`) into a single package
2. Wire `WithRequestContext` middleware into chi router or delete it
3. Wire `Investigator` agent into orchestrator or remove it
4. Remove manual UUID generation in stores — rely on DB `DEFAULT uuid_generate_v4()`

### Phase 1 — Security & Reliability (Week 1)

1. Add API key auth middleware to backend chi router
2. Replace fire-and-forget goroutines with a background worker pool using `context.Background()` + timeouts
3. Add semaphore/worker pool to cap concurrent outbound goroutines in `handleIngestTrafficBatch`
4. Add retry with exponential backoff to `AgentClient.post`
5. Fix `LLM_API_KEY` vs `DEEPSEEK_API_KEY` env var mismatch
6. Switch all LangGraph agents from `MemorySaver` to `RedisSaver`

### Phase 2 — Data Integrity & Dead Code (Week 2)

6. Add `UNIQUE(spec_id, method, path)` constraint to endpoints table
7. Fix `DriftStore.CreateBatch` to use `ON CONFLICT` for dedup
8. Add `ON DELETE CASCADE` to `alert_history.stream_id` FK
9. Fix pagination consistency (choose offset or page, not both)
10. Validate `DEEPSEEK_API_KEY` at agent startup
11. Remove dead code: `context.go`, `cleanup.go`, wire `Investigator` or remove it

### Phase 3 — Testing (Week 3)

12. Add service layer unit tests with mocked stores
13. Add database repository integration tests
14. Add handler tests for all 15+ endpoints
15. Add anomaly subsystem tests
16. Add app/main startup tests
17. Fix broken test imports in `test_sentinel.py`

### Phase 4 — Infrastructure (Week 4)

18. Add Ingress/Gateway API with TLS
19. Add NetworkPolicies and PodDisruptionBudget
20. Add Prometheus ServiceMonitor and Grafana dashboards
21. Add Trivy/Snyk container scanning to CI/CD
22. Template image tags (Helm or `kustomize`)

---

## Appendix: Test Coverage Summary

| Package | Source LOC | Test LOC | Coverage | Status |
|---------|-----------|----------|----------|--------|
| `internal/config` | 75 | 70 | ~100% | ✅ Strong |
| `internal/apperrors` | 135 | 164 | ~91% | ✅ Strong |
| `internal/models` | 163 | 236 | ~90% | ✅ Strong |
| `internal/api` | 767 | 304 | ~42% | ⚠️ Partial (routing + batch only) |
| `internal/database` | 661 | 147 | ~27% | ❌ Query builder only; all CRUD untested |
| `internal/service` | 316 | 0 | 0% | ❌ Untested |
| `pkg` (app.go) | 61 | 0 | 0% | ❌ Untested |
| `cmd/server` (main.go) | 59 | 0 | 0% | ❌ Untested |
| **Go total** | **~2700** | **~921** | **~28%** | ❌ |
| Python Agents | ~2300 | ~308 | ~10-15% | ❌ (only LangGraph nodes; server, DB, anomaly untested) |
| Python Middleware | ~600 | ~490 | ~70-75% | ✅ Strong |
