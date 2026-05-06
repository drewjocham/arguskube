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

### P1 — Data/Knowledge Layer (DONE ✅)
- [x] `useData.js` — useFeatures, useChat, useNotebooks, useRunbooks, useIncidents, useWorkflows
- [x] `useSetup.js` — useSetup, useAnomaly
- [x] `useMonitoring.js` — useArgusScan, useVulnerabilities
- [x] `useNetwork.js` — useServicePods
- [x] `useResources.js` — useResources
- [x] `useMisc.js` — useCodeBlock

### P1 — Go Backend Packages
- [ ] `internal/alerts/` — alert manager, severity calculations, metric collection
- [ ] `internal/workflows/` — workflow CRUD (extend existing store_test)
- [ ] `internal/incidents/` — incident CRUD (extend existing store_test)
- [ ] `internal/runbooks/` — runbook CRUD
- [ ] `internal/notebooks/` — notebook CRUD + S3 integration
- [ ] `internal/features/` — feature gate logic
- [ ] `internal/config/` — config parsing and validation

### P2 — Integration Tests
- [ ] Component mount tests for AlertList, MetricsRow, TerminalView, ArgusCDList
- [ ] Sidebar navigation renders all 28 items
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
