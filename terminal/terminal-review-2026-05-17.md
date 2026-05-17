# Unified Review — Argus Terminal Module

**Generated:** 2026-05-17
**Reviewers:** @architect, @explore, @library-curator, @general

---

## 1. Bug Status Summary

| Bug | Description | File:Line | Status | Fixed In |
|-----|------------|-----------|--------|----------|
| C-01 | `let lastSessionId` module-level — cross-instance bleed | `TerminalView.vue:149` | **❌ Still exists** | — |
| C-02 | Shell command injection in `execIntoPod()` | `TerminalView.vue:212` | **❌ Still exists** | — |
| C-03 | `readLoop` goroutine data race on PTY FD | `terminal.go:73,119` | **✅ Fixed** | `7c4cdf3` |
| — | Table-driven test compliance (AGENTS.md mandate) | `terminal_test.go` | **✅ Fixed** | `79097d3` |
| — | `time.Sleep(500ms)` flaky polling in tests | `terminal_test.go:51,60` | **✅ Fixed** | `79097d3` |
| H-01 | `fit()` not in try/catch — stale resize dims | `TerminalView.vue:120` | **❌ Still exists** | — |
| H-02 | xterm dynamic import has no error handling | `TerminalView.vue:60-61` | **❌ Still exists** | — |
| H-03 | Pinia store called at module level before registration | `TerminalView.vue:105` | **❌ Still exists** | — |
| H-04 | Copilot requests have no timeout — permanent "Thinking..." | `TerminalView.vue:173-181` | **❌ Still exists** | — |
| M-03 | `onUnmounted` never calls `closeSession()` — PTY leak | `TerminalView.vue:251-255` | **❌ Still exists** | — |
| — | `Start()`/`StartWithEnv()` 90% code duplication | `terminal.go:32-122` | **❌ Still exists** | — |
| — | Resize handler not debounced — 60fps backend flood | `TerminalView.vue:117-123` | **❌ Still exists** | — |
| — | `Write()` silently returns `nil` on closed terminal | `terminal.go:129-131` | **❌ Still exists** | — |
| — | `extraEnv` format not validated (missing `=` crashes) | `terminal.go:99` | **❌ Still exists** | — |
| — | `readLoop` has no context cancellation mechanism | `terminal.go:178-192` | **❌ Still exists** | — |
| — | `tabs` array O(n) `.find()` lookup per keystroke | `TerminalView.vue:21` | **❌ Still exists** | — |
| — | No `StartWithEnv` test coverage | `terminal_test.go` | **❌ Still exists** | — |
| — | No concurrent access / `-race` tests | `terminal_test.go` | **❌ Still exists** | — |
| — | No error injection tests (PTY failure) | `terminal_test.go` | **❌ Still exists** | — |
| — | Missing frontend tests (initError, tab lifecycle, copilot) | `TerminalView.test.js` | **❌ Still exists** | — |

---

## 2. Phone App UX Assessment

**The terminal module is not suitable as a phone app in its current form.** No mobile/phone app code exists in the repository — it's only an aspirational Phase 4 goal (`lufis-terminal/argus-terminal-requirements-2026-05-16.md:352`).

### Critical UX Gaps

| Gap | Details |
|-----|---------|
| No situational awareness | Users see a raw shell prompt — no dashboard, summary panel, or "here's what's happening" |
| No contact/ownership info | Zero on-call info, team rosters, escalation paths |
| Flat information hierarchy | No distinction between alerts, context, and actions — everything is in-band terminal output |
| Touch-hostile UI | Tab add menus use `:hover` only (line 346), buttons are `4px 8px` at `11px` font (needs 44x44pt) |
| No responsive design | No viewport meta, no media queries, no breakpoints |
| `# session:` comment injection | Line 153-156: writes `# session:` to stdin — corrupts user's partial command |

### What a Phone App Needs

1. **Notification/status-first layout** — replace terminal-first with alerts, resource health, on-call contact
2. **Terminal as secondary detail view** — expandable, not the primary surface
3. **44x44pt minimum touch targets** on all interactive elements
4. **Responsive breakpoints** with media queries
5. **On-call contact display** — who to contact, escalation paths

---

## 3. AGENTS.md Coding Guidelines Compliance

| Rule | Status | Details |
|------|--------|---------|
| Table-driven tests (hard req) | **✅ Fixed** (`79097d3`) | Now 3 table-driven tests + 2 standalone |
| DRY principle | **❌ Violated** | `Start()`/`StartWithEnv()` 90% identical (198 lines) |
| Error wrapping with `fmt.Errorf` | **❌ Missing** | PTY errors returned unwrapped |
| `context.Context` for goroutines | **❌ Missing** | `readLoop` has no cancellation mechanism |
| `log/slog` (not `log`/`fmt.Print`) | **✅ Compliant** | All files use `slog` |
| Import ordering | **✅ Compliant** | stdlib → third-party → local |
| Naming conventions | **✅ Compliant** | CamelCase exports, snake_case unexported |
| `logKey*` constants | **⚠️ Inconsistent** | Some handlers use inline key-value pairs |
| Mock injection for tests | **❌ Missing** | Hard dependency on `creack/pty` — no `PTYProvider` interface |
| 2-click feature rule | **❌ Violated** | Phone UX currently requires many steps to get status info |

---

## 4. Architecture Issues (by Severity)

### P0 — Security / Data Loss

| Issue | File:Line | Recommendation |
|-------|-----------|----------------|
| Shell injection | `TerminalView.vue:212` | Shell-escape with single-quote wrapping |
| Module-level state bleed | `TerminalView.vue:149` | Change `let lastSessionId` to `const lastSessionId = ref(null)` |
| Silent data drop | `terminal.go:129-131` | Return error instead of nil when terminal closed |

### P1 — Reliability / Maintainability

| Issue | File:Line | Recommendation |
|-------|-----------|----------------|
| Code duplication | `terminal.go:32-76` | Delegate `Start()` → `StartWithEnv(nil)` |
| No resize debounce | `TerminalView.vue:117` | Debounce to max 100ms interval |
| PTY leak on unmount | `TerminalView.vue:251-255` | Call `closeSession()` for each tab in `onUnmounted` |
| No error handling | `TerminalView.vue:60-61` | Wrap xterm import in try/catch |
| Copilot no timeout | `TerminalView.vue:173-181` | Add `AbortController` with 30s timeout |
| `extraEnv` validation | `terminal.go:99` | Skip entries without `=` separator |
| 90% duplicated test surface | `terminal_test.go` | Add `TestStartWithEnv`, concurrent tests, error injection |

### P2 — Performance / UX

| Issue | File:Line | Recommendation |
|-------|-----------|----------------|
| O(n) tab lookup | `TerminalView.vue:21` | Replace `tabs` array with `Map<string, TabState>` |
| Stale `termRefs` on close | `TerminalView.vue:41-50` | `delete termRefs.value[sessionId]` in `closeTab` |
| No touch-friendly menus | `TerminalView.vue:346` | Add click-to-toggle for tab-add menu |
| Stale resize dims | `TerminalView.vue:120` | Wrap `fit()` in try/catch |

---

## 5. Library / Dependency Recommendations

| Custom Code | Replace With | File/Area |
|-------------|-------------|-----------|
| PTY management | Keep `creack/pty`, add context wrapper | `terminal.go` |
| Shell escaping | `shell-quote` (JS) or manual `esc()` | `TerminalView.vue:212` |
| Rate limiting | `golang.org/x/time/rate` | `terminal_handler.go` |
| ANSI parser (buggy UTF-8) | `github.com/mgutz/ansi` or similar | `lufis-terminal/internal/ansi/` |
| GPU rendering | `fyne.io/fyne` or `ebiten` | `lufis-terminal/internal/render/` |
| Terminal testing | `goexpect` | `terminal_test.go` |
| Accessibility audit | `axe-core` | Frontend CI |

---

## 6. Files Referenced

| File | Lines | Purpose |
|------|-------|---------|
| `kube/view/src/components/terminal/TerminalView.vue` | 421 | Vue.js xterm.js frontend |
| `kube/backend/internal/terminal/terminal.go` | 198 | Go PTY handler |
| `kube/backend/internal/terminal/terminal_test.go` | 290 | Go tests (recently rewritten) |
| `kube/backend/api/pkg/terminal_handler.go` | 28 | HTTP/WebSocket handler |
| `kube/backend/api/pkg/app_terminal_copilot.go` | 115 | AI copilot |
| `kube/backend/api/pkg/app_terminal_launcher.go` | 228 | Pop-out terminal launcher |
| `kube/view/src/stores/terminalDispatch.js` | 47 | Pinia dispatch store |
| `lufis-terminal/` | ~7500 | Native Go terminal (in dev) |
| `AGENTS.md` | 196 | Project coding guidelines |
