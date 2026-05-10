# Test Results — 2026-05-10

## Frontend Unit Tests (Vitest)
- **Result:** PASS
- **Total:** 546
- **Passed:** 546
- **Failed:** 0
- **Skipped:** 0

### Notes
- 44 test files executed across components, composables, stores, and utilities.
- Several `[Vue warn]` diagnostics appear (non-blocking warnings):
  - `onMounted`/`onUnmounted` called outside of active component instance (composables tested in isolation without a Vue setup context).
  - `Property "_uid" was accessed during render but is not defined on instance` (VolumeCylinder component).
  - `Property "detailLoading" was accessed during render but is not defined on instance` (StatefulDaemonSetList component).
- All expected error-handling paths (`[saas-api] ... fallback failed`) are exercised by tests and are expected behavior.
- `--localstorage-file` warnings from Vitest config — minor, non-blocking.

## Go Backend Tests
- **Result:** PASS
- **Total packages with tests:** 24
- **Failures:** 0

### Packages tested successfully
| Package | Time |
|---|---|
| `github.com/argues/kube-watcher/api/pkg` | 3.118s |
| `github.com/argues/kube-watcher/internal/agentconn` | 0.050s |
| `github.com/argues/kube-watcher/internal/ai` | 8.666s |
| `github.com/argues/kube-watcher/internal/alertproc` | 0.196s |
| `github.com/argues/kube-watcher/internal/alerts` | 0.016s |
| `github.com/argues/kube-watcher/internal/anomaly` | 0.077s |
| `github.com/argues/kube-watcher/internal/apperrors` | 0.021s |
| `github.com/argues/kube-watcher/internal/argocd` | 0.062s |
| `github.com/argues/kube-watcher/internal/auth` | 3.184s |
| `github.com/argues/kube-watcher/internal/config` | 0.026s |
| `github.com/argues/kube-watcher/internal/context` | 0.026s |
| `github.com/argues/kube-watcher/internal/features` | 0.032s |
| `github.com/argues/kube-watcher/internal/incidents` | 0.282s |
| `github.com/argues/kube-watcher/internal/k8s` | 0.050s |
| `github.com/argues/kube-watcher/internal/logger` | 0.013s |
| `github.com/argues/kube-watcher/internal/notebooks` | 0.037s |
| `github.com/argues/kube-watcher/internal/popeye` | 1.055s |
| `github.com/argues/kube-watcher/internal/runbooks` | 0.023s |
| `github.com/argues/kube-watcher/internal/setup` | 10.632s |
| `github.com/argues/kube-watcher/internal/sqlitedb` | 0.074s |
| `github.com/argues/kube-watcher/internal/terminal` | 0.750s |
| `github.com/argues/kube-watcher/internal/tlsconfig` | 0.014s |
| `github.com/argues/kube-watcher/internal/vulnscan` | 0.025s |
| `github.com/argues/kube-watcher/internal/workflows` | 0.149s |

### Packages without test files (5)
- `github.com/argues/kube-watcher` (root)
- `github.com/argues/kube-watcher/internal/spotcheck`
- `github.com/argues/kube-watcher/pkg/audit`
- `github.com/argues/kube-watcher/pkg/kube`
- `github.com/argues/kube-watcher/pkg/kube/watch`
- `github.com/argues/kube-watcher/view` (embed placeholder)

## Go Lint
- **Result:** FAIL (golangci-lint exit code 1)
- **Violations:** 35

### Violations by category

#### errcheck (20 violations)
Unchecked error return values. Most are in test files (acceptable for test simplicity), but some are in production code:

| File | Line | Issue |
|---|---|---|
| `api/pkg/hub.go` | 159, 161 | Unchecked `SetReadDeadline` |
| `api/pkg/hub.go` | 189, 198 | Unchecked `SetWriteDeadline` |
| `api/pkg/hub.go` | 191 | Unchecked `WriteMessage` |
| `api/pkg/server.go` | 98, 194 | Unchecked `json.Encode` |
| `internal/terminal/terminal.go` | 37 | Unchecked `closeLocked` |
| `internal/terminal/terminal.go` | 120 | Unchecked `Process.Kill` |

The remaining 13 violations are in `_test.go` files (test code).

#### unused (6 violations)
| File | Line | Symbol |
|---|---|---|
| `internal/anomaly/flink.go` | 125 | `flinkJobResponse` (type) |
| `internal/config/config.go` | 248 | `homeDir` (func) |
| `pkg/kube/client.go` | 366 | `defaultKubeconfig` (func) |
| `internal/context/assembler_test.go` | 84 | `testConfig` (func) |
| `internal/context/assembler_test.go` | 89 | `testLogger` (func) |
| `internal/k8s/client.go` | 26–29 | `logKeyPod`, `logKeyNamespace`, `logKeyNode`, `logKeyError` (consts) |

#### gosimple (2 violations)
| File | Line | Issue |
|---|---|---|
| `api/pkg/app_deploy_artifacts.go` | 108 | `S1017`: replace `if HasPrefix` with `strings.TrimPrefix` |
| `internal/context/diagnose.go` | 96 | `S1039`: unnecessary `fmt.Sprintf` |

#### ineffassign (2 violations)
| File | Line | Issue |
|---|---|---|
| `internal/context/diagnose.go` | 287 | Ineffectual assignment to `relation` |
| `internal/popeye/runner.go` | 317 | Ineffectual assignment to `binaryAvailable` |

#### staticcheck (1 violation)
| File | Line | Issue |
|---|---|---|
| `internal/context/assembler_test.go` | 72 | `SA1029`: use custom type instead of built-in `string` for context key |

## Go Build
- **Result:** PASS
- **Errors:** None
- All packages compiled successfully with no errors.

## Frontend Build (Vite)
- **Result:** PASS (with warning)
- **Errors:** None

### Warning
```
(!) Some chunks are larger than 500 kB after minification. Consider:
- Using dynamic import() to code-split the application
- Use build.rollupOptions.output.manualChunks to improve chunking
- Adjust chunk size limit for this warning via build.chunkSizeLimitWarning.
```

### Build output
| Asset | Size (uncompressed) | Size (gzip) |
|---|---|---|
| `index.html` | 0.43 kB | 0.30 kB |
| `assets/index-C5_9qO80.css` | 280.61 kB | 43.18 kB |
| `assets/xterm-addon-fit-D-0KS9LU.js` | 1.71 kB | 0.77 kB |
| `assets/xterm-B8sHTpDo.js` | 282.58 kB | 69.88 kB |
| `assets/index-CPsBx1-y.js` | 1,092.11 kB | 340.95 kB |

## Summary

| Check | Result |
|---|---|
| Frontend Unit Tests | 🟢 PASS |
| Go Backend Tests | 🟢 PASS |
| Go Lint | 🔴 FAIL |
| Go Build | 🟢 PASS |
| Frontend Build | 🟡 PASS (warnings) |

**Overall status: YELLOW**

**Action items:**
1. **Address golangci-lint violations** (35 total) — prioritize production code issues:
   - `api/pkg/hub.go`: Handle errors from `SetReadDeadline`, `SetWriteDeadline`, and `WriteMessage` (4 locations).
   - `api/pkg/server.go`: Handle errors from `json.Encode` (2 locations).
   - `internal/terminal/terminal.go`: Handle errors from `closeLocked` and `Process.Kill` (2 locations).
   - `api/pkg/app_deploy_artifacts.go`: Replace `if HasPrefix` + slice with `strings.TrimPrefix` (gosimple S1017).
   - `internal/context/diagnose.go`: Remove unnecessary `fmt.Sprintf` (gosimple S1039).
   - `internal/context/diagnose.go`: Fix ineffectual assignment to `relation`.
   - `internal/popeye/runner.go`: Fix ineffectual assignment to `binaryAvailable`.
   - Remove or use dead code: `homeDir`, `defaultKubeconfig`, `flinkJobResponse`, unused k8s log key constants.
2. **Vue warnings in frontend tests** (non-blocking but noisy):
   - Composables using `onMounted`/`onUnmounted` outside of Vue setup context should be tested with `@vue/test-utils` mounting or have lifecycle hooks guarded.
   - `Property "_uid"` warning in `VolumeCylinder` — likely a missing prop definition.
   - `Property "detailLoading"` warning in `StatefulDaemonSetList` — missing prop/reactive data in template.
3. **Frontend chunk size warning** — consider code-splitting the main JS bundle (currently ~1 MB) for production.
