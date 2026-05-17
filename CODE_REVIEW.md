# Code Review Report — Argus Repository

**Date:** 2026-05-17  
**Scope:** Full repository (Go backend, lufis-terminal, agent, Vue frontend, Terraform, Helm, CI)  
**Files reviewed:** ~50 key files across all components

---

## Critical (2)

### 1. lufis-terminal config test: `os.Setenv` is not goroutine-safe

**File:** `lufis-terminal/internal/config/config_test.go`

Uses `os.Setenv` directly in parallel table-driven tests without `t.Setenv`. This IS a data race when tests run with `-parallel`. `os.Setenv` in Go is explicitly documented as not safe for concurrent use.

```go
// Current (unsafe with -parallel):
os.Setenv("SHELL", tt.envShell)

// Fix:
t.Setenv("SHELL", tt.envShell)
```

The tests don't use `t.Parallel()` currently, but the table-driven structure suggests they should — and the AGENTS.md mandates table-driven tests. This is a ticking race condition.

### 2. GKE sleep script over-privileged IAM

**File:** `deploy/terraform/modules/gke-platform/sleep.sh:32-36`

Grants `roles/container.clusterAdmin` to the scheduler service account. This role includes `container.clusters.delete`, `container.clusters.updateMaster`, and `container.*.setIamPolicy`. The SA only needs to call `nodePools:setAutoscaling`. Use a custom role with just `container.clusters.update` + `container.nodePools.update` instead.

---

## Warnings (6)

### 3. lufis-terminal main.go: Ctrl-key switch is 60 lines of boilerplate

**File:** `lufis-terminal/cmd/argus-terminal/main.go:147-198`

Has an explicit `case` for each Ctrl+Letter combination. Should be a lookup table:

```go
var ctrlKeys = map[glfw.Key]byte{
    glfw.KeyA: 0x01, glfw.KeyB: 0x02, glfw.KeyC: 0x03,
    // ... etc
}
```

60 lines → 15, easier to audit for completeness.

### 4. Flink gateway mixes `http.Get` and contextual requests

**File:** `kube/flink/gateway/main.go:213`

Uses bare `http.Get(overviewURL)` without context or timeout, while `queryFlinkJobs` at line 241 correctly uses `NewRequestWithContext`. Inconsistency — the bare `http.Get` won't respect cancellation.

### 5. Agent server goroutine doesn't filter `context.Canceled`

**File:** `agent/cmd/agent/main.go:80-85`

`srv.Start` errors propagate unfiltered through errgroup, but `tunnelClient.Start` and `k8sClient.StartInformers` both filter `context.Canceled`. If `srv.Start` returns `http.ErrServerClosed` on graceful shutdown, it'll be treated as a fatal error.

```go
// Fix:
eg.Go(func() error {
    if err := srv.Start(egCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
        return err
    }
    return nil
})
```

### 6. Makefile `helm-install-dev` omits agent chart

**File:** `Makefile:289-299`

Installs `argus-backend` and `argus-frontend` but not `argus-agent`, while `helm-install` installs all three. Inconsistency that will surprise developers using the dev target.

### 7. Stale backup files in repo

Multiple files with ` 2` suffix scattered throughout: `go 2.mod`, `go 2.sum`, `App 2.vue`, `*.test 2.ts`, `terminal-review-*.md`. Also `lufis-terminal/internal/auth/store 2.go` through `store 5.go`. These are confusing and suggest incomplete cleanup. Remove or `.gitignore` them.

### 8. `.env.example` hardcodes a real-looking password

**File:** `.env.example:42`

Sets `GF_SECURITY_ADMIN_PASSWORD=argus`. Example files should use placeholder values only (`GF_SECURITY_ADMIN_PASSWORD=changeme` or `""`). While this isn't a production secret, it normalizes weak credentials in development.

---

## Suggestions (6)

### 9. lufis-terminal ANSI parser tests are not table-driven

**File:** `lufis-terminal/internal/ansi/parser_test.go`

Has ~20 individual test functions (`TestPlainText`, `TestNewline`, `TestCUU`, etc.), each testing a single case. The AGENTS.md mandates: **"ALL Go tests MUST be table-driven."** This is the most clear-cut AGENTS.md violation in the repo. Consolidate these into a few table-driven tests grouped by ANSI category (cursor movement, SGR, screen modes, etc.).

### 10. `envFloat` silently swallows parse errors

**File:** `kube/backend/internal/config/config.go:493-501`

`envFloat` returns the fallback on `ParseFloat` error with zero feedback. No log, no warning. An operator setting `ARGUS_BILLING_INPUT_PER_1M=0.15typo` gets silent zero. At minimum, add a comment acknowledging the design choice; ideally log a warning.

### 11. CI coverage floor is 25% with `continue-on-error`

**File:** `.github/workflows/ci.yml:71-96`

Sets coverage to 25% and `continue-on-error: true`. The comment says this is a baseline being established. That's fine for now, but add a tracking issue or TODO to ratchet it up. 25% is low for a production SRE product. The comment already acknowledges this — just needs a concrete plan.

### 12. Cloud Run module: public frontend deserves a comment

**File:** `deploy/terraform/modules/cloud-run/main.tf:236-241`

Frontend Cloud Run is public (`allUsers` with `run.invoker`) while backend and MCP are service-account-only. This is correct by design (SPA), but worth an explicit comment explaining why the frontend is intentionally open.

### 13. Terraform GKE module hardcodes CIDR ranges

**File:** `deploy/terraform/modules/gke-platform/main.tf:11,17,22`

Uses hardcoded `10.0.0.0/17`, `10.4.0.0/14`, `10.8.0.0/20`. These should be variables with defaults to avoid collision in multi-cluster VPCs or when deploying alongside existing infrastructure.

### 14. `kube/backend/main.go` log-prefix inconsistency

**File:** `kube/backend/main.go:53`

Uses `log.Fatalf("argus: %v", err)` (plain `log`) while the rest of the app uses structured `slog`. The `log.Fatalf` at the entry point is acceptable for bootstrap failures but worth standardizing — either document why `log` is intentionally used before the logger is available, or defer to the point where it is.

---

## Looks Good (13)

1. **Auth system is excellent.** `kube/backend/internal/auth/store.go` — bcrypt with cost parameter, dummy hash verification on non-existent email (timing attack mitigation), provider separation preventing account takeover. Production-grade auth design.

2. **Secret storage is well-architected.** macOS Keychain via `security` CLI (no CGO), in-memory fallback for Linux/tests, lufis-terminal's store separates on-disk metadata from keychain secrets. Both `secretstore` and lufis-terminal's `auth` package handle missing entries gracefully.

3. **Terraform uses GCP Secret Manager** for API keys (`cloud-run/main.tf:42-48`, `55-61`). Secrets are referenced by name, never inlined.

4. **Helm security contexts are properly restrictive** — `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, cap-drop ALL.

5. **Release pipeline uses cosign keyless signing** with GitHub OIDC — `.github/workflows/release.yml:200-214`. Container images are signed and attested.

6. **Config layering is thoughtful** — `config.go` loads env vars first, then overlays persisted desktop settings, with clear documentation about the precedence.

7. **Broker registry pattern** (`kube/backend/pkg/broker/registry.go`) with `init()` registration and factory functions — clean, extensible, testable.

8. **Chi middleware patterns** are consistent across the backend, Flink gateway, and agent — request ID, recovery, logging, typed payload binding.

9. **Signal handling** is correct and consistent across all three Go entry points — `signal.NotifyContext` or explicit channels with cleanup.

10. **Table-driven tests** in `kube/backend/internal/terminal/terminal_test.go` — good model for lufis-terminal tests to follow.

11. **The Makefile** is well-documented with help target, `doctor` for tool checks, and consistent `-count=1` and `-race` flags.

12. **No hardcoded secrets** found in source code. The `env.example` has a placeholder password (see warning) but no real credentials.

13. **lufis-terminal pty.go** handles edge cases well — locked `closed` flag, idempotent Close, buffer allocated once.

---

## Summary

| Category     | Count |
|--------------|-------|
| Critical     | 2     |
| Warnings     | 6     |
| Suggestions  | 6     |
| Looks Good   | 13    |
| **Total**    | **27** |

**Bottom line:** This is a well-architected, security-conscious codebase. Auth, secrets management, and infrastructure-as-code patterns are production-grade. The main areas needing attention:

1. Fix the `os.Setenv` race in lufis-terminal config tests
2. Scope down the over-privileged GKE IAM role in the sleep script
3. Convert the lufis-terminal ANSI parser tests to the table-driven format mandated by AGENTS.md
4. Clean up stale backup files with ` 2`, ` 3`, etc. suffixes

---

*Reviewed by Hermes Agent — 2026-05-17*
