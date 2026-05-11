# Frontend Testing Guide — Argus View

## Test Stack

| Layer | Tool | Command |
|-------|------|---------|
| Unit (composables, stores, utils) | Vitest + `@vue/test-utils` + jsdom | `npx vitest run` |
| Unit (watch mode) | Vitest | `npx vitest` |
| E2E | Playwright | `npx playwright test` |

Configuration:
- `vitest.config.js` — jsdom environment, `@/` path alias, Pinia auto-setup
- `vitest.setup.js` — fresh Pinia instance per test
- `e2e/playwright.config.js` — chromium only, single worker, `localhost:5173`

---

## Test Coverage Summary

| Layer | Tested | Total | % | Missing |
|-------|--------|-------|---|---------|
| Utils | 4 | 5 | 80% | `logHighlight.js` |
| Composables | 17 | 20 | 85% | `useEvents`, `useBackgroundTasks`, `useSpotCheck` |
| Stores | 7 | 10 | 70% | `agentAnalysis`, `appearance`, `auth` |
| Components | 16 | 48 | 33% | 32 components |
| E2E | 1 flow | ~14 | 7% | 13 flows (see `TEST_GAP_STATUS.md`) |

---

## Writing Tests — Patterns

### Pure functions (utils)
- Import the function directly, call it with test inputs, assert on return value.
- Table-driven tests keep things concise.
- **Reference:** `utils/__tests__/renderMarkdown.test.js`

### Composables
- Mock `callGo` / `fetch` via `vi.stubGlobal('callGo', ...)`.
- Assert on `.value` of returned refs.
- Use `nextTick()` / `flushPromises()` for async composables.
- **Reference:** `composables/__tests__/useAlerts.test.js`

### Pinia stores
- Call store methods directly (no mounting needed).
- Mock `localStorage` via `vi.stubGlobal`.
- Assert on store state after mutations.
- **Reference:** `stores/__tests__/notifications.test.js`

### Vue components
- Mount with `@vue/test-utils` `mount()` or `shallowMount()`.
- Stub child components, provide Pinia store mocks.
- Assert on rendered output (`wrapper.text()`, `wrapper.find()`).
- **Reference:** `__tests__/components/MetricsRow.test.js`

---

## Go Backend Tests

Go tests in this repository are **required to be table-driven** (`[]struct{name string, args ..., want ...}`) — see `AGENTS.md` for the full requirement and template. Every Go test function must use this pattern for consistency and comprehensive edge-case coverage.

## Priority Order

1. 🔴 **HIGH** — Core infra: `auth.js`, `appearance.js` stores; `useEvents.js`, `useBackgroundTasks.js` composables; `LogExplorer.vue`, `PodList.vue`, `MetricsExplorer.vue`, `SettingsPanel.vue` components.
2. 🟡 **MEDIUM** — Feature views: `AnomalyDetection.vue`, `S3Notebook.vue`, `WorkflowEditor.vue`, and remaining composables/stores.
3. 🟢 **LOW** — All remaining untested components.
