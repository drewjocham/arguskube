# KubeWatcher Test Plan

## Strategy: Guard Rails by Layer

Every PR/commit must pass all three layers before merging:

```
Layer 1: Unit (narrow)   ← Vitest + Go test     — per-function, per-composable, per-package
Layer 2: Integration     ← Vitest + Vue Test Utils — mount components, verify data flow
Layer 3: E2E / Smoke     ← Playwright            — login, nav tree, terminal, arguscd
```

## Priority Order

### P0 — Core Bridge + Critical Composables (already done: 37 tests)
- [x] `useBridge.js` — callGo (Wails + SaaS), isWails, useAppMode, useClusterInfo, useContexts
- [x] `useMetrics.js` — useMetrics, useCostEstimate

### P0 — Write Now (DONE ✅)
- [x] `useAlerts.js` — useAlerts, useDiagnostics
- [x] `useArgusCD.js` — listApps, sync, testConnection, error states
- [x] `useCluster.js` — useTopology
- [x] `useLogs.js` — usePodLogs, useLogStream, useLogs, useNodeLogs
- [x] `usePods.js` — usePods, useDeploymentRevisions, useVPARecommendations
- [x] `useShell.js` — useTerminal, usePodExec
- [x] `useToast.js` — addToast, removeToast

### P0 — Go Core Infrastructure (DONE ✅)
- [x] `internal/agentconn/` — agent connector with port-forward, pod discovery, fallback handling
- [x] `internal/ai/` — AI agent dispatch, DeepSeek API client, chat history, event tracking, auto-investigation
- [x] `internal/setup/` — initialization flow: tool detection, popeye install, agent deploy/undeploy

### P1 — Data/Knowledge Layer (DONE ✅)
- [x] `useData.js` — useFeatures, useChat, useNotebooks, useRunbooks, useIncidents, useWorkflows
- [x] `useSetup.js` — useSetup, useAnomaly
- [x] `useMonitoring.js` — useArgusScan, useVulnerabilities
- [x] `useNetwork.js` — useServicePods
- [x] `useResources.js` — useResources
- [x] `useMisc.js` — useCodeBlock

### P1 — Go Backend Packages
- [x] `internal/alerts/` — alert manager, severity calculations, metric collection
- [x] `internal/workflows/` — workflow CRUD (existing store_test covers CRUD + concurrent ops)
- [x] `internal/incidents/` — incident CRUD (existing store_test covers CRUD + persistence)
- [x] `internal/runbooks/` — runbook CRUD (existing test covers create/list/get/save/parse + id helpers)
- [x] `internal/notebooks/` — notebook CRUD + S3 integration (existing test covers save/get/list/delete/folder ops)
- [x] `internal/features/` — feature gate logic
- [x] `internal/config/` — config parsing and validation

### P2 — Go Operations Layer (DONE ✅)
- [x] `internal/anomaly/` — Anomstack client (detect, list jobs), settings store (CRUD rules, toggle, save/load settings)
- [x] `internal/context/` — Context assembly with cascade correlation, anomaly enrichment, diagnosis generation for 8+ alert types
- [x] `internal/terminal/` — PTY shell: start, write, resize, close, IsRunning, nil-safe operations

### P3 — Go Utilities (DONE ✅)
- [x] `internal/logger/` — Log level/format parsing, structured log keys
- [x] `internal/sqlitedb/` — SQLite open with WAL, pragmas, concurrent inserts, table creation
- [x] `internal/tlsconfig/` — TLS config loading, self-signed cert verification, min version enforcement
- [x] `internal/vulnscan/` — Vulnerability scanner constructor and scan dispatch
- [x] `internal/popeye/` — Popeye runner with option functions (config, spinach, format, sections)
- [x] `internal/apperrors/` — Error type wrapping, sentinel matching, code/message extraction

### P2 — Integration Tests (DONE ✅)
- [x] Component mount tests for all 7 components (AlertList, ArgusCDList, CenterPanel, MetricsRow, NodeList, Sidebar, TerminalView)
- [x] Sidebar navigation renders all 28 items
- [ ] CenterPanel routes activeNav correctly

### P3 — E2E Smoke Tests (Playwright)
- [ ] Page loads with correct title
- [ ] Sidebar navigation works
- [ ] Terminal can be toggled
- [ ] ArgusCD view renders

## Guard Rail Enforcement

```yaml
pre-commit:
  - vitest run           # All frontend unit tests
  - go test ./...        # All Go tests (if Go available)

pre-merge:
  - vitest run --coverage  # Coverage reports
  - go test -race ./...    # Race detection
  - playwright test        # E2E smoke tests
```

## Naming Convention

| Layer | Location | Pattern |
|-------|----------|---------|
| Unit | `__tests__/${composableName}.test.js` | `describe('useX')`, `it('does Y when Z')` |
| Go Unit | `*_test.go` in same package | `func TestXxx`, `TestSuite/` |
| Integration | `__tests__/components/${ComponentName}.test.js` | mount, assert renders, interact |
| E2E | `e2e/` | Playwright page objects |
