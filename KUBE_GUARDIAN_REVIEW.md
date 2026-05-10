# Kube Guardian Review — KubeWatcher Go Backend

**Role:** Kubernetes & Infrastructure SRE  
**Date:** May 10, 2026  
**Scope:** Backend K8s integration, alert pipeline, security, dual-mode architecture, agent tunnel, test gaps, GitOps readiness  

---

## 1. Kubernetes Client Layer

### 1a. `backend/internal/k8s/client.go` — Kubeconfig Loading & Security

**Kubeconfig resolution** supports three modes: in-cluster (`rest.InClusterConfig`), single-file explicit path, and colon-separated multi-file (KUBECONFIG-style). The `kubeconfigLoadingRules` function correctly uses client-go's `Precedence` field for multi-file merge. **Good.**

**Security concerns:**
- No kubeconfig impersonation support (`ConfigOverrides` exposes `Impersonate`, but code never sets it)
- No client-side rate limiting or throttling configured. `rest.Config` created via `clientcmd.NewNonInteractiveDeferredLoadingClientConfig` returns defaults — `QPS` and `Burst` remain at client-go defaults (5/10), which is acceptable for a desktop tool but **insufficient for the SaaS deployment** behind the Hub.
- The `SwitchContext` method (line 606-625) reinitializes the entire client with a new `*rest.Config`. This is a heavyweight operation and the old clientset is dropped without waiting for in-flight requests to drain — potential goroutine leak.
- `GetRestConfig()` and `GetClientset()` expose raw internals to callers like `agentconn.Connector`. This makes the interface leaky.

### 1b. `backend/internal/k8s/exec.go` — SPDY Streaming

**Strengths:**
- Clean `PodExecSession` abstraction with proper `io.Pipe` usage for stdin/stdout bridging
- Shell detection via `command -v bash || exec sh` is pragmatic
- `TerminalSizeQueue` implementation correct
- `Close()` uses mutex to prevent double-close

**Issues:**
- **No context deadline propagation to the SPDY stream.** `exec.StreamWithContext` uses the parent context, but there is no explicit timeout set for exec sessions. Long-running idle sessions persist until the client disconnects or the pod terminates.
- `readLoop` (line 164) uses a raw `io.Reader.Read` which blocks indefinitely — no select on `ctx.Done()`. If the pipe never closes, the goroutine leaks.
- Buffer size of 8192 is small for terminals running `cat largefile` or `helm list`. Not a security issue but a UX problem.
- `done` channel is closed by `exec.StreamWithContext` returning, but `readLoop` is launched before `StreamWithContext` starts — if the pipe closes before `readLoop` reads, it exits fine. No leak, but fragile ordering.

### 1c. `backend/internal/k8s/resources_*.go` — Resource Listing Patterns

**Major finding: no pagination on ANY List call.** Every `List(ctx, metav1.ListOptions{})` call in the codebase uses default options with no `Limit` and no `Continue` token. This means:
- In clusters with 1000+ pods, every 5-second poll fetches **the entire pod list**
- `GetMetrics` (client.go:139) calls `Pods(ns).List` with no limit
- `DetectAlerts` (client.go:253) fetches ALL pods and ALL nodes every 5 seconds
- `pollLogs` (events.go:187) fetches ALL pods every 3 seconds
- Resource listers in `resources_workloads.go` do the same

**Impact:** ~3 List ALL calls per poll cycle × 3 pollers = 9 unbounded API calls per cycle. In a 500-pod cluster that's ~5000+ pod objects serialized and deserialized every 5 seconds.

**The `GetWarningEvents` method (line 370) is especially ironic** — it passes `Limit` to the server but then ignores pagination by not checking `Continue`. This limits the response to `limit` events but won't handle multi-page results.

**Resource lister pattern** uses `metav1.ListOptions{}` empty struct consistently — no field selectors or label selectors passed from the frontend for server-side filtering. All filtering appears to happen client-side after fetching everything.

### 1d. `backend/pkg/kube/client.go` — The MCP's Separate Client

**Why two implementations?** The file header states: "intentionally separate from internal/k8s [...] so the MCP binary can be built independently." This is a **build-time dependency isolation** choice, not a functional distinction.

**Problems:**
- Massive code duplication: both implement `List`-style methods with almost identical `for i := range list.Items { out = append(out, mapItem(&list.Items[i])) }` patterns
- The MCP client has a proper `ClientInterface` abstraction; internal/k8s has no equivalent interface, making it impossible to mock the desktop client for testing
- The MCP client has an `auditClient` decorator for access logging — internal/k8s has no equivalent
- The MCP client's `AuditOptionsFromEnv` uses environment variable `KUBEWATCHER_AUDIT` — this should be a config value, not an env var
- `GetResource` (line 289) is a stub returning `nil, nil` — dead code

---

## 2. Alert Processing Pipeline

### 2a. `backend/internal/alertproc/processor.go` — Dedup & Fatigue

**Strengths:**
- SHA-256 signature-based dedup via `SignatureOf` is correct and well-conceived
- Fatigue detection logic (counting silences + ignores against threshold) is sound
- `Process()` method correctly uses `sync.RWMutex` and copies the profile to avoid lock contention in the investigation goroutine
- `scheduleInvestigation` uses `context.WithTimeout` (60s) — good
- First-fire-only dedup with 5-minute staleness window is reasonable for alert fatigue prevention

**Issues:**
- **In-memory state only** (line 29 comment acknowledges this). On restart, all signature state is lost. For a desktop app this is "annoying, not unsafe" — for the SaaS backend it means every restart triggers re-investigation for every alert.
- Linear scan for alert ID → signature mapping in `Silence()` (line 317). "N is bounded" is true, but with long-running clusters accumulating signatures, this could stall the mutex.
- `PersistProfile` uses `ON CONFLICT(id) DO UPDATE` — correct SQLite upsert pattern
- `alert_events` table has no TTL/retention policy — unbounded growth
- No rate limiting on `Investigate`: if 50 new alerts fire simultaneously, 50 goroutines launch and 50 DeepSeek API calls happen in parallel

### 2b. `backend/api/pkg/events.go` — Polling vs Informers

**Design choice:** The desktop app uses three separate polling loops (alerts at 5s, metrics at 10s, logs at 3s). The in-cluster agent (agent/) uses informers (list-watch with cache). This is the single biggest architectural issue:

| Aspect | Desktop (backend/api/pkg/events.go) | Agent (agent/internal/k8s/client.go) |
|--------|------|-------|
| Pattern | Polling | Informers (List+Watch) |
| Cache | None (fetches everything every cycle) | SharedInformerFactory (10min resync) |
| API pressure | 3 parallel pollers × unbounded List calls | Watch events only + periodic resync |
| Real-time | 3-5s latency | Near-instant |
| Scalability | ❌ Poor | ✅ Excellent |

**Why this matters:** In the SaaS deployment model, the backend runs as a pod in the cluster. If it uses polling (like the desktop client does), every 5 seconds it issues 3+ full List calls against the API server. With 100+ tenants this becomes untenable.

**Missing circuit breaker:** If the cluster API is unreachable, all three pollers log warnings (`a.logger.WarnContext(ctx, "alert poll failed", ...)`) and `continue` — but no backoff, no circuit breaker, no degraded-mode signal. The pollers hammer the unreachable API at full cadence forever.

**Specific bugs in pollLogs:**
- `SinceTime` uses `now.Add(-4 * time.Second)` — this assumes the ticker fires exactly on schedule. In practice, slow API responses can cause the tick to drift. Should use the previous tick timestamp.
- `maxLines` (200) cap is good for preventing memory blow-up, but it means only pods early in the list get log lines fetched. Pod order is undefined — log coverage is effectively random.

---

## 3. Security Review

### 3a. `backend/api/pkg/server_security.go` — CORS, Token Auth, Origin

**Excellent security posture across the board:**
- **Deny-by-default CORS:** only origins in `KUBEWATCHER_API_ALLOWED_ORIGINS` or localhost are echoed back
- **Loopback-only bind** by default (`127.0.0.1`)
- **Constant-time token comparison** via `subtle.ConstantTimeCompare`
- `remoteIsLocal()` correctly uses `net.IP.IsLoopback()` — foolproof
- `isLocalOrigin()` handles Wails (`wails://`) and Tauri (`tauri://`) scheme origins
- `itob` (itoa-style int formatting) avoids importing `strconv` — tiny optimization, interesting choice

**Minor issues:**
- `authenticateService` compares token from `os.Getenv` every request — environment variable lookups are cheap but the token should be cached at startup
- CORS `Access-Control-Max-Age: 300` is only 5 minutes — browsers will re-preflight more often than necessary. 24h (86400) is typical for APIs using only POST/GET/OPTIONS
- No rate limiting on the token auth endpoint itself — brute-force of the service token is possible over LAN

### 3b. `backend/internal/auth/oidc.go` — OAuth PKCE

**Correct RFC 8252 implementation:**
- PKCE with S256 challenge method (`sha256.Sum256` + `base64.RawURLEncoding`)
- `randomToken(32)` for verifier = 256 bits of entropy — exceeds OWF recommendation
- `randomToken(24)` for state
- Replay protection: `completed_at` column checked before exchange (line 147)
- Time-bound pending flows: 15-minute expiry for abandoned OAuth (line 242)
- Error path: `MarkPendingError` stamps the error into `oauth_pending` so the frontend poll can surface it
- Token verification: `oidc.Verifier.Verify()` called with `ClientID` matching
- Email verification enforced for Google provider (line 191)

**Issues:**
- `oauthConfig` includes `ClientSecret` in the exchange even for public clients (Google is treated as confidential). This is technically correct (the secret is ignored in the authorization code grant for native apps) but a comment would help
- `PollPending` checks `tok == ""` for "still pending" — but if the session token is somehow empty (shouldn't happen), it stays pending until 15-min timeout. Consider `completed_at == 0 AND error == "" AND session_token == ""` for a more robust check
- No `httptest` for the OAuth callback handler — critical path with no unit test coverage

### 3c. `backend/internal/tlsconfig/certs.go` — mTLS for Agent Tunnel

**Correct and secure:**
- ECDSA P-256 key generation
- CA cert with `KeyUsageCertSign | KeyUsageCRLSign`, `IsCA: true`, `MaxPathLen: 0`
- Agent certs with `ExtKeyUsageClientAuth` only — cannot be used as server certs
- `ServerTLSConfig` uses `RequireAndVerifyClientCert` with TLS 1.3 minimum
- `AgentTLSConfig` uses `RootCAs` (not `ClientCAs`) for server verification
- PEM files written with `0600` permissions — good file permission hygiene
- Clock skew tolerance of 5 minutes (`NotBefore: time.Now().Add(-5*time.Minute)`)

**Issues:**
- No CRL or OCSP revocation support. Once a CA cert is deployed, it cannot be revoked without redeploying all agents
- No certificate expiration monitoring — expired agent certs fail silently with confusing mTLS errors
- `WritePEM` overwrites existing files atomically? No — it uses `os.WriteFile` which is not atomic. CRITICAL for CA key files — a crash during write corrupts the CA

### 3d. `backend/internal/sqlitedb/db.go` — SQL Injection, WAL Mode

**SQL injection:** All queries use `?` placeholders via `db.Exec` and `db.QueryRow`. **No injection risk.** ✅

**WAL mode:** Enabled via DSN parameter `_pragma=journal_mode(WAL)`. Busy timeout set to 5000ms. **Good.**

**`SetMaxOpenConns(1)`:** Single writer enforced — necessary for SQLite's concurrency model. **Correct.**

**Migration system:** Versioned migrations with tracking table. Only appends new migrations. **Good.** However, no `DOWN` migrations — rollback requires manual SQL.

### 3e. API Key Masking in Settings

`backend/api/pkg/app_desktop.go` `GetSettings()` (line 300): DeepSeek API key, ArgoCD token, and Snyk token are all properly masked:
- Keys > 8 chars: show first 4 + "…" + last 4
- Keys <= 8 chars: show "••••"
- `UpdateSettings` (line 352) checks `containsMask()` to prevent masked values from overwriting real keys

**Correct implementation.** ✅

---

## 4. Dual-Mode Architecture (Desktop vs SaaS)

### 4a. Shared App Struct

`App` struct (`app.go`) is the single instance shared between Wails (desktop) and HTTP (SaaS). All bindings are thin wrappers calling same `App` methods.

**Concurrency analysis:**
- `execSession` has `sync.RWMutex` protection ✅
- `paused` is `atomic.Bool` ✅
- `webhookAlerts` has `sync.RWMutex` ✅
- `cachedMetrics` has no mutex — written by `pollMetrics` goroutine and read by `GetMetrics` and agent. **Race condition.** A concurrent read during write could see a partially constructed `*alerts.ClusterMetrics` (pointer assignment is atomic on x86_64, but field readers could see stale values).
- `hub` is initialized in `NewApp` and then `WithTLS` is called on it from the SaaS setup path — no mutex, but happens-before is established via constructor ordering (not provably safe through code review alone)

### 4b. Feature Gating

`backend/internal/features/gate.go`: Gate is created from `config.Tier` at startup. `Allowed()` checks a static `proOnly` map.

**Enforcement is backend-side:** Every feature access in the app layer calls `a.gate.Allowed(features.FeatureAI)` before proceeding. This is not just frontend hiding — it's server-enforced. **Good.**

**Weakness:** The tier comes from `config.OnlineDataConfig` which is loaded from a file. For the SaaS deployment, this means tier changes require a restart. No runtime tier re-evaluation.

### 4c. Race Conditions in `api/pkg/app.go`

The `Startup` method (line 161):
1. Sets `a.ctx`
2. Calls `GetMetrics` synchronously
3. Calls `startAlertProcessor`
4. Calls `a.StartEventLoop(ctx)` — launches 3 goroutines
5. Calls `startSpotChecks` — launches another goroutine
6. Creates and starts `periodicAgent` — launches yet another goroutine

**Problem:** Steps 2-6 are sequential but the goroutines from step 4 start immediately and can read `a.cachedMetrics` before step 2 writes it. However, step 2 runs before step 4, so the first write happens before the pollers start. **Safe by happenstance, not by contract.**

The `SwitchContext` path (app_desktop.go:46) replaces `a.agentConn` without synchronizing with the `agentconn.Connector` callers. A topology fetch could use a stale connector after context switch.

---

## 5. Agent & Tunnel

### 5a. `backend/api/pkg/hub.go` — WebSocket Reliability

**Strengths:**
- `CheckOrigin` defers to `allowedOrigins()` — same allowlist as REST API
- Send buffer `256` entries — reasonable for command-and-control
- `SendCommand` uses non-blocking send with `default` case — won't block the caller if agent is slow
- Read limit of 512KB per message
- Ping/pong at 54s interval with 60s read deadline — proper keepalive

**Issues:**
- **No pong response in writePump:** The server sends pings but never waits for pongs. `SetPongHandler` only resets the read deadline. If the agent's TCP connection drops, the server waits 60s (read deadline) before detecting the disconnection.
- **Write deadline on pings:** `SetWriteDeadline(10s)` on ping writes — if the agent's send buffer is full, the ping blocks for 10s then drops the connection. Correct behavior but 10s is generous.
- **readPump discards all messages:** Line 175 logs incoming telemetry but never processes it — the routing is a TODO comment. In production, agent telemetry (anomaly scores, topology) goes nowhere.
- **No reconnection backoff for the Hub itself** — clients reconnect, but the Hub has no mechanism to detect a split-brain scenario.

### 5b. `backend/internal/agentconn/connector.go` — Port-Forward Reliability

**Strengths:**
- Dynamic port allocation via `net.Listen("tcp", "127.0.0.1:0")` — avoids port conflicts
- Proper SPDY round-tripper setup
- 10s timeout on port-forward readiness

**Issues:**
- **Single-shot: no retry logic.** If the port-forward fails, the entire operation fails. In a cluster with pod churn, this means topology/anomaly fetches fail randomly.
- **No context propagation to `ForwardPorts()`:** The `stopCh` is closed in the timeout case and deferred `close(stopCh)`, but there's no select on `ctx.Done()` in the goroutine. A cancelled context doesn't stop the port-forward.
- **All three methods (GetAnomalies, GetTopology, GetEvents) create a new port-forward each time.** This is expensive — each port-forward requires a SPDY round-trip and a new local listener. A shared long-lived port-forward would be more efficient.
- **Label selector `agentLabelSelector` is hardcoded** (`app.kubernetes.io/name=kubewatcher-agent`). Multi-tenancy would require this to be configurable.
- `FindAgentPod` returns only the first matching pod — if there are multiple agent replicas, behavior is undefined.

### 5c. `agent/` Directory — Read-Only or Mutating?

**Mixed:**
- **Read-only routes:** `GET /api/v1/pods`, `/nodes`, `/anomalies`, `/events`, `/deployments`, `/services`, `/topology`, `/health` — all read from informer cache ✅
- **Mutating:** `agent/internal/cd/applier.go` — `ApplyManifest` performs Server-Side Apply via `Patch(ctx, name, types.ApplyPatchType, ...)`. The agent can CREATE/UPDATE resources in the cluster. This is part of the `arguscd` deployment feature.

**Security implication:** The agent runs with a ClusterRole that allows Server-Side Apply. The CD applier is callable from the agent's HTTP API. If the tunnel WebSocket is compromised, an attacker can deploy arbitrary manifests to the cluster. The `fieldManager: "arguscd"` provides audit trail but no access control.

**Agent informer pattern (agent/internal/k8s/client.go):**
- Uses `SharedInformerFactory` with `10*time.Minute` resync
- Correctly calls `informerFactory.Start(ctx.Done())` and `cache.WaitForCacheSync`
- Event handlers log all pod/node/svc/dep/event activity at DEBUG level

---

## 6. Test Infrastructure Gaps

Confirmed gaps from the audit document and source inspection:

| Package | Test Files | Lines of Code | Severity |
|---------|-----------|---------------|----------|
| `internal/spotcheck/` | **0** | 218 | 🔴 **High** — Engine + Check interface + RunAll/RunOne logic goes untested |
| `internal/agentanalysis/` | **0** | 67 | 🟡 Medium — stub with 3s sleep, but analysis triggering path is untested |
| `internal/auth/password.go` | **0** | 27 | 🔴 **High** — bcrypt hash/verify, cost config, edge cases (empty, short) |
| `internal/auth/oidc.go` | **0** | 335 | 🔴 **High** — PKCE, token exchange, provider discovery — critical auth path |
| `internal/auth/store.go` | **0** | 262 | 🔴 **High** — Session creation/validation/revocation, user CRUD, OAuth state |
| `internal/k8s/exec.go` | **0** | 175 | 🟡 Medium — SPDY streaming, pipe management, resize lifecycle |
| `internal/k8s/logs.go` | **0** | (unread) | 🟡 Medium — Log query engine |
| `internal/k8s/metrics.go` | **0** | (unread) | 🟡 Medium — Metrics queries |
| `internal/k8s/resources_*.go` | **0** | (est. 600+) | 🔴 **High** — All workload/network/config/storage listers untested |
| `api/pkg/events.go` | **0** | 258 | 🔴 **High** — Background polling loops, dedup, log streaming |
| `api/pkg/hub.go` | **0** | 232 | 🔴 **High** — WebSocket tunnels, agent routing, ping/pong |

**agentanalysis specifically:** The `agent.go` file is a stub. The `RunAnalysis` method:
- Sleeps 3 seconds (`time.Sleep(3 * time.Second)`)
- Emits a hardcoded result text
- Makes no actual K8s queries
- Runs every hour with `time.NewTicker(1 * time.Hour)`

This should either be removed (with dead-code elimination) or implemented with real analysis.

---

## 7. GitOps-Readiness Score

**Rating: 5.5 / 10**

### Assessment by Criteria:

| Criterion | Score | Rationale |
|-----------|-------|-----------|
| **Helm charts** | 7/10 | 6 charts (backend, frontend, agent, mcp, alert-ingress, monitoring). Proper Chart.yaml, values.yaml separation, dev override files. Missing: chart testing (chart-testing), schema validation, app-version bump automation |
| **Declarative config** | 6/10 | ConfigMap-driven but secrets via `existingSecret` pattern is good. Missing: Kyverno policies for admission control, ArgoCD ApplicationSets for multi-tenant |
| **Secrets management** | 5/10 | Supports external secrets (via `existingSecret`), but default assumes a single managed Secret. No External Secrets Operator or SealedSecrets integration |
| **IaC (Terraform)** | 7/10 | Terraform configs exist for EKS + VPC. Good module usage (terraform-aws-eks, terraform-aws-vpc). Missing: state locking config, remote backend setup |
| **CI/CD** | 4/10 | No `.github/workflows/` deployment pipelines visible. No ArgoCD Application manifests in the repo. No image tag pinning (uses `latest`-style `""` default) |
| **Image tagging** | 3/10 | Default `tag: ""` means Helm uses `Chart.AppVersion`. No semantic versioning or commit SHA tagging strategy |
| **GitOps bootstrap** | 4/10 | No bootstrap manifests (crossplane, argo-cd itself). User must run `helm install` manually |
| **Multi-tenancy** | 3/10 | Namespace isolation assumed but not enforced. No NetworkPolicies, no ResourceQuota defaults in Helm values |

### Specific Findings:

- **No PodDisruptionBudget** in any chart — backend will be disrupted during node drains
- **No NetworkPolicy** resources in any chart — zero network isolation between components
- **Backend HPA** values exist but `autoscaling.enabled: false` by default — production deployments must remember to enable it
- **No priorityClassName** set — critical control-plane pods compete with batch workloads
- **Persistence** uses PVC but no backup/restore strategy documented or configured
- **Helm values** expose secret booleans (`env.secret.deepseekAPIKey: false`) that control secret creation — no actual secret management integration

---

## 8. Reliability Recommendations

### R1: Replace Polling with Informers in the Desktop Client

**Problem:** `events.go` uses 3 parallel pollers at 5s/10s/3s intervals, each doing unbounded List calls. The in-cluster agent (`agent/internal/k8s/client.go`) already has the correct informer pattern. The desktop client is a fork ahead.

**Action:**
- Create a shared `InformerClient` adapter in `internal/k8s/` that wraps `SharedInformerFactory`
- Replace `pollAlerts`, `pollMetrics`, `pollLogs` with event-handler callbacks on Pod/Node/Event informers
- Keep the poller as a fallback for clusters where watch is unavailable (very rare)
- Add `context.Context` cancellation propagation to all informer lifecycle

**Testability gain:** Informer callbacks are pure functions — easy to unit test with fake clientsets.

### R2: Add Pagination and Rate Limiting to All K8s List Calls

**Problem:** Every `List(ctx, metav1.ListOptions{})` call uses no `Limit` or `Continue`. In clusters >500 pods, this causes O(n) serialization/deserialization every few seconds.

**Action:**
- Create a paginated list helper: `func (c *Client) listWithPagination(ctx, gvr, ns string, opts metav1.ListOptions) func() ([]unstructured.Unstructured, bool, error)` that handles `Continue` tokens
- Set `rest.Config.RateLimiter` to a custom rate limiter with per-endpoint budgets
- Add `metav1.ListOptions{Limit: 500}` as minimum to all List calls
- Monitor `X-RateLimit-*` response headers for API server pressure

### R3: Add Circuit Breaker for Cluster Unreachability

**Problem:** When the K8s API server is unreachable, all 3 pollers log and `continue` — hammering the unavailable endpoint at full cadence forever.

**Action:**
- Wrap the K8s clientset in a circuit-breaker decorator (e.g., `github.com/sony/gobreaker` or a simple state machine)
- States: `Closed` (normal), `HalfOpen` (probe), `Open` (fail fast)
- Trip conditions: 5 consecutive timeouts or connection errors within 30s
- On `Open`: return cached metrics/alerts (with stale flag), reduce log spam
- Auto-recovery: probe every 30s in HalfOpen, close on success
- Surface circuit state to frontend so users see "cluster unreachable" vs "all clear"

### R4: Implement Agent Connection Pool for Port-Forwarding

**Problem:** `agentconn.Connector` creates a brand-new SPDY port-forward for every `GetAnomalies()`, `GetTopology()`, or `GetEvents()` call. This is expensive and unreliable.

**Action:**
- Create a long-lived port-forward connection pool (single pod, persistent tunnel)
- Health-check the connection every 30s with a lightweight HTTP GET `/health`
- On failure: exponential backoff with jitter (500ms, 1s, 2s, 4s, max 30s), then find a new agent pod
- Add `context.Context` propagation so cancelled requests don't leave orphaned port-forwards
- Use a connection-scoped HTTP client (not `http.DefaultClient`) with per-connection timeouts

### R5: Add Tests for the Three Zero-Coverage Auth Packages

**Problem:** `password.go`, `oidc.go`, and `store.go` have zero test files. These are security-critical paths handling bcrypt hashing, OAuth token exchange, and session management.

**Hard requirement:** All Go tests in this repo MUST be table-driven (`[]struct{name string, args ..., want ...}`). See `AGENTS.md` for the template and rationale.

**Action (minimum bar):**
- `password.go`: Table-driven tests for `hashPassword` (valid password, too short, boundary at 12 chars), `verifyPassword` (correct, incorrect, empty hash, empty plain, wrong cost)
- `store.go`: In-memory SQLite (per test), test `CreateLocalUser` (success, duplicate email, invalid email), `AuthenticateLocal` (success, wrong password, nonexistent user), `CreateSession`/`ValidateSession` (valid token, expired, tampered), `RevokeSession` (idempotent), `UpsertOAuthUser` (new user, existing, race condition)
- `oidc.go`: Mock HTTP server for OIDC discovery, test `StartLogin` (returns URL+state), `CompleteLogin` (success, replay, expired state, wrong verifier), `PollPending` (pending, completed, error, expired, missing state)

These tests do not require a live K8s cluster — they test pure Go logic with in-memory SQLite and HTTP test servers. No excuse for zero coverage.

---

## Summary of Critical Findings

1. **No pagination on List calls** — will break in large clusters
2. **Polling instead of informers** — 3x parallel unbounded polls, API-server-unfriendly
3. **No circuit breaker** — unreachable clusters get hammered forever
4. **In-memory alert state** — lost on restart, SaaS will re-investigate every alert
5. **5 auth packages with zero test coverage** — `password.go`, `oidc.go`, `store.go` are security-critical and untested
6. **Agent tunnel is effectively a telemetry black hole** — `readPump` receives messages but routes them nowhere
7. **Port-forward per request** — SPDY tunnel created/destroyed for every data fetch
8. **`agentanalysis` is a 3-second-sleep stub** — wastes goroutine and ticker
9. **No PodDisruptionBudget, no NetworkPolicy** in any Helm chart — production-unaware defaults
10. **Race condition on `cachedMetrics`** — concurrent readers and writer with no synchronization

---

*Review generated by kube-guardian agent — SRE perspective focused on scalability, security, and reliability of the K8s integration layer.*
