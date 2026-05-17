# Test Review & Plan — Argus Terminal

**Reviewer:** @test-generator
**Date:** 2026-05-16

---

## 1. Go Backend Tests — `kube/backend/internal/terminal/terminal_test.go`

### 1.1 Critical Issues

| Issue | Severity | Detail |
|---|---|---|
| **Not table-driven** | P0 | All 15 test functions are standalone — violates AGENTS.md hard requirement |
| **`time.Sleep` for async** | P0 | Uses `time.Sleep(500ms)`, `time.Sleep(200ms)` — flaky in CI |
| **PTY-dependent** | P1 | Tests `t.Skipf` when no PTY available — no mock abstraction |
| **No `StartWithEnv` test** | P1 | `StartWithEnv()` is completely untested |
| **No concurrent access tests** | P1 | No race-condition testing with `-race` |
| **No error injection tests** | P1 | No test for `pty.StartWithSize` failure, PTY read failure etc. |

### 1.2 What's Missing (Critical Gaps)

| Missing Test | Impact | Priority |
|---|---|---|
| `TestStartWithEnv` | Zero coverage on `StartWithEnv()` | P0 |
| `TestReadLoop` / `TestOnOutput` | Core goroutine — untested | P0 |
| `TestConcurrentAccess` | No `-race` safety verified | P1 |
| `TestWriteError` | PTM write failure path | P1 |
| `TestStartError` | `pty.StartWithSize` returns error | P1 |
| `TestCloseDuringRead` | Close races with readLoop goroutine | P1 |

### 1.3 Proposed Table-Driven Rewrite

Replace all 15 standalone tests with a clean table-driven structure + channel-based coordination to eliminate `time.Sleep`:

```go
package terminal

import (
    "log/slog"
    "testing"
)

func TestNew(t *testing.T) {
    tests := []struct {
        name   string
        logger *slog.Logger
    }{
        {name: "with nil logger does not panic", logger: nil},
        {name: "with real logger returns non-nil", logger: slog.New(slog.DiscardHandler)},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            term := New(tt.logger)
            if term == nil { t.Fatal("New returned nil") }
        })
    }
}

func TestIsRunning(t *testing.T) {
    term := New(slog.New(slog.DiscardHandler))
    tests := []struct {
        name  string
        setup func()
        want  bool
    }{
        {name: "false before Start", setup: func() {}, want: false},
        {name: "false after Close", setup: func() { term.Start("sh", 40, 120); term.Close() }, want: false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.setup()
            if got := term.IsRunning(); got != tt.want {
                t.Errorf("IsRunning() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

For PTY-backed tests, use **channel-based coordination** instead of `time.Sleep`:

```go
func TestStartAndWrite_readsOutput(t *testing.T) {
    term := New(slog.New(slog.DiscardHandler))
    outputCh := make(chan string, 10)
    term.OnOutput = func(data string) { outputCh <- data }

    if err := term.Start("sh", 40, 120); err != nil {
        t.Skipf("PTY not available: %v", err)
    }
    t.Cleanup(func() { term.Close() })

    if err := term.Write("echo HELLO_PTY_TEST\n"); err != nil {
        t.Fatal(err)
    }

    select {
    case data := <-outputCh:
        if !strings.Contains(data, "HELLO_PTY_TEST") {
            t.Errorf("output %q does not contain expected marker", data)
        }
    case <-time.After(3 * time.Second):
        t.Fatal("timed out waiting for PTY output")
    }
}
```

---

## 2. Frontend Tests — `TerminalView.test.js`

### 2.1 Current Coverage (8 tests)

| Test | Status |
|------|--------|
| Hidden when `visible=false` | ✅ |
| Visible when `visible=true` | ✅ |
| Renders `.terminal-element` | ✅ |
| Calls `createSession` on init | ✅ |
| Writes on `terminal:output` event | ✅ |
| Handles bare string backward compat | ✅ |
| Calls `fitAddon.fit` on resize | ✅ |
| Disposes on unmount | ✅ |
| Flushes queued command after init | ✅ |
| Renders tab bar | ✅ |

### 2.2 Critical Gaps (P0-P1)

| Missing Test | Priority |
|---|---|
| **initError state** — no test for `createSession` rejection → `.terminal-error` rendered | P0 |
| **retryInit** — no test for the Retry button | P1 |
| **addTab / closeTab** — tab lifecycle | P1 |
| **podPickerOpen + execIntoPod** — K8s Pod Exec overlay | P1 |
| **ctxSwitcherOpen + switchCtx** — Context switcher overlay | P1 |
| **Copilot interaction** (handleCopilotSubmit, handleExplain, closeCopilot) | P1 |
| **Multiple sessions** — switching `activeSessionId` | P2 |

---

## 3. Store Tests — Excellent Coverage

### `terminalDispatch.test.js` (7 tests)

✅ Initial state, sendToTerminal, empty/non-string input, peek, consume, monotonic ID, meta forwarding, non-object meta.

Only gap: no test for `meta` with only `sectionLabel` but no `sessionId` (P3).

### `runbookTerminals.test.js` (10 tests)

✅ Initial state, resolveTarget fallback, pinDocument, per-document scope, unpin, override beats pin, clear override, clearDocument, persistence, pinnedSessionFor, buildSessionId.

Only gap: corrupt localStorage JSON parsing (P3).

### `useTerminalDispatch.test.js` (4 tests)

✅ Shared state, send, empty input, consume. Thin wrapper — store tests cover the real logic.

---

## 4. Priority Action Plan

### Sprint 1 (P0 — Must Fix)

| # | File | Action | Estimate |
|---|---|---|---|
| 1 | `terminal_test.go` | Full rewrite to table-driven + channel-based async | 1-2 days |
| 2 | `terminal_test.go` | Add `TestStartWithEnv` | < 1 day |
| 3 | `session_test.go` | **NEW** — SessionManager tests (NewSession, CloseSession, ListSessions) | 1 day |
| 4 | `TerminalView.test.js` | Add initError + retryInit tests | < 1 day |
| 5 | `useShell.test.js` | Add `useTerminalSession` tests (7 test cases) | < 1 day |

### Sprint 2 (P1 — Important)

| # | File | Action | Estimate |
|---|---|---|---|
| 6 | `terminal_test.go` | Add concurrent access tests with `-race` | 1 day |
| 7 | `terminal_test.go` | Add error injection tests (PTY failure) | 1 day |
| 8 | `TerminalView.test.js` | Add tab lifecycle (addTab, closeTab) | 1 day |
| 9 | `TerminalView.test.js` | Add pod exec + context switcher | 1 day |
| 10 | `TerminalView.test.js` | Add copilot interaction | < 1 day |

---

## 5. Long-Term Recommendation: Interface Extraction

The root cause of flaky Go tests is the hard dependency on `creack/pty`. Extract an interface:

```go
type PTYProvider interface {
    StartWithSize(cmd *exec.Cmd, size *pty.Winsize) (*os.File, error)
    Setsize(file *os.File, size *pty.Winsize) error
}
```

This enables fast, deterministic unit tests with mock injection. Estimate: 1-2 days for extraction + mock generation + test rewrite. Priority: P1.
