# QA Report — Argus Terminal

**Reviewer:** @qa-tester
**Date:** 2026-05-16

---

## 1. Critical Bugs

### C-01: Module-level `lastSessionId` bleeds across component instances

**File:** `kube/view/src/components/terminal/TerminalView.vue:145`
**Severity:** CRITICAL

```js
let lastSessionId = null
```

This is a **module-level variable** (not `ref`, not `reactive`). If two instances of TerminalView exist, `flushPendingCommand()` on instance A mutates `lastSessionId` that instance B reads. Causes B to skip session-header comments or write stale headers.

**Fix:** Make it a `ref`:
```js
const lastSessionId = ref(null)
// ... use lastSessionId.value throughout
```

### C-02: Shell command injection via pod/namespace name

**File:** `kube/view/src/components/terminal/TerminalView.vue:208`
**Severity:** CRITICAL

```js
sendSessionInput(activeSessionId.value, `kubectl exec -it -n ${namespace} ${podName} -- sh\n`)
```

No shell escaping. A pod named `my-pod; rm -rf /` or with backticks/`$()` would execute arbitrary commands. Even without malice, spaces in pod names break the command.

**Fix:** Shell-escape with single-quote wrapping:
```js
const esc = s => "'" + s.replace(/'/g, "'\\''") + "'"
sendSessionInput(activeSessionId.value, `kubectl exec -it -n ${esc(namespace)} ${esc(podName)} -- sh\n`)
```

### C-03: Goroutine leak + FD reuse race on multiple Start()

**File:** `kube/backend/internal/terminal/terminal.go:73,119`
**Severity:** CRITICAL

`go t.readLoop(ptmx)` passes a PTY FD to the goroutine. If `Start()` is called again, `closeLocked()` closes the old PTY. If the FD number is reused (rare but possible), the old goroutine reads from the new terminal.

The `lufis-terminal` version is worse — it silently overwrites `t.ptmx` without closing the old one, leaking FDs.

**Fix:** Use context cancellation:
```go
if t.cancelCh != nil { close(t.cancelCh) }
cancelCh := make(chan struct{})
t.cancelCh = cancelCh
go t.readLoop(ptmx, cancelCh)
```

---

## 2. High-Severity Bugs

### H-01: Resize sends stale dimensions if fit() throws

**File:** `kube/view/src/components/terminal/TerminalView.vue:120`

```js
tab.fitAddon.fit()
resizeSession(tab.sessionId, tab.term.rows, tab.term.cols)
```

If `fit()` throws, `rows/cols` are left in pre-fit state. Backend never notified → dimension mismatch.

### H-02: xterm dynamic import has no error handling

**File:** `kube/view/src/components/terminal/TerminalView.vue:60-61`

```js
const { Terminal } = await import('xterm')
const { FitAddon } = await import('xterm-addon-fit')
```

No try/catch. If CDN/network fails, the terminal silently stays in "not started" state with no error message.

### H-03: Pinia store called before registration

**File:** `kube/view/src/components/terminal/TerminalView.vue:105`

```js
const captureStore = useOutputCaptureStore()
```

If the component mounts before `app.use(pinia)`, every terminal output event crashes the app.

### H-04: Copilot requests have no timeout/abort

**File:** `kube/view/src/components/terminal/TerminalView.vue:169-184`

If the AI endpoint hangs, UI freezes indefinitely showing "Thinking..." with no way to dismiss.

### H-05: UTF-8 decoding broken in ANSI parser

**File:** `lufis-terminal/internal/ansi/parser.go:488-490`

```go
func (p *Parser) decodeUTF8(b byte) {
    p.runeBuf = p.runeBuf[:0]
    p.runeBuf = append(p.runeBuf, rune(b))
}
```

Casts each byte individually to `rune`. Multi-byte sequences (e.g., `é` = 0xC3 0xA9) produce two garbage runes (Ã and ©). Non-ASCII text is completely broken.

### H-06: SGR param 22 shadowed by range case

**File:** `lufis-terminal/internal/ansi/parser.go:388-389`

The switch has `case 21 <= param && param <= 29:` (empty no-op) BEFORE `case param == 22:`. Param 22 never reaches its handler — bold/dim are never cleared.

---

## 3. Medium-Severity Bugs

| ID | File | Line | Issue |
|----|------|------|-------|
| M-01 | TerminalView.vue | 117-123 | Resize across tabs is nondeterministic (inactive tabs not resized) |
| M-02 | runbookTerminals.js | 20-39 | localStorage throws in private browsing (Safari) |
| M-03 | TerminalView.vue | 247-251 | onUnmounted leaks backend PTY processes (never calls `closeSession`) |
| M-04 | terminal.go | 99 | `extraEnv` format not validated (missing `=` crashes on some platforms) |
| M-05 | session.go | 171-178 | ANSI injection via env values in welcome banner |
| U-01 | TerminalView.vue | 117,236 | Resize handler not debounced (60fps resize floods backend) |
| U-02 | TerminalView.vue | 320 | Copilot output is raw text — no markdown rendering or syntax highlighting |
| U-05 | TerminalView.vue | 346 | Tab add menu uses `:hover` — not touch-friendly |
| U-07 | TerminalView.vue | 153-156 | `# session:` comment written to stdin corrupts user's partial command |
| G-01 | TerminalView.vue | 81,120 | `fitAddon.fit()` not in try/catch |
| G-04 | terminal.go | 129-131 | `Write` returns nil on closed terminal — silent data drop |

---

## 4. Accessibility Issues

| ID | File | Line | Issue |
|----|------|------|-------|
| A-01 | TerminalView.vue | 315 | Copilot panel is `<div>` with `@click` — no `role="dialog"`, no `aria-label` |
| A-02 | TerminalView.vue | 262 | Tab close `&times;` has no `aria-label="Close session"` |
| A-03 | TerminalView.vue | 257 | Tab buttons need `role="tab"` with `aria-selected` and `role="tablist"` container |
| A-05 | TerminalView.vue | 337,371 | Color contrast below WCAG AA threshold (`#8b8f96` on `#151719` = ~4.0:1, needs 4.5:1) |

---

## 5. Test Quality Issues

| ID | File | Line | Issue |
|----|------|------|-------|
| Q-01 | terminal_test.go | All | Tests NOT table-driven (violates AGENTS.md hard requirement) |
| Q-02 | terminal_test.go | 51,60 | `time.Sleep(500ms)` — inherently flaky in CI |
| Q-03 | terminal.go | 32-122 | 90% code duplication between `Start` and `StartWithEnv` |
