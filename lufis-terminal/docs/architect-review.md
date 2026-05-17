# Architecture Review — Argus Terminal

**Reviewer:** @architect
**Date:** 2026-05-16

---

## 1. Critical Issues

### C-1: Font System Has No Fallback Chain

**File:** `lufis-terminal/internal/render/font.go:41`

The `loadSystemFont()` function searches hardcoded paths (SFNSMono, Menlo, Courier New, Courier) with no proper fallback chain. Cell width is calculated as `cellW = cellH * 6 / 10` — a rough heuristic. Missing:

- JetBrains Mono (required per requirements TC-11)
- Nerd Font glyphs
- Ligature support
- Configurable `FontFamily` (the TOML config at `config.go:16-21` has no `FontFamily` field)

### C-2: Code Duplication — Start / StartWithEnv

**File:** `kube/backend/internal/terminal/terminal.go:32-122`

`Start()` (lines 32-76) and `StartWithEnv()` (lines 79-122) are ~90% identical. Every fix must be manually ported between them. Fix: `Start()` should delegate to `StartWithEnv()`:

```go
func (t *Terminal) Start(shell string, rows, cols uint16) error {
    return t.StartWithEnv(shell, rows, cols, nil)
}
```

### C-3: No Rate Limiting on WebSocket/PTY Input

**File:** `kube/backend/api/pkg/terminal_handler.go`

Shell executions lack command injection protection beyond what the PTY provides. No rate limiting on WebSocket or PTY input — a user or attacker can flood the terminal with input.

---

## 2. Architecture Patterns: What's Good vs Missing

### Good Patterns

- **Plugin system** in `lufis-terminal/plugin/` — clean separation of core terminal from plugins
- **Explicit PTY FD passing** in `readLoop(ptmx *os.File)` — prevents race on `t.ptmx` reassignment
- **Config struct pattern** in `internal/config/` — TOML-based, clean defaults
- **Screen buffer abstraction** in `internal/screen/cell.go` — clean cell/line/cursor model

### Missing Patterns

| Pattern | Missing From | Impact |
|---|---|---|
| **Adapter** | Both terminal backends | No unified interface between xterm.js web terminal and native Go terminal |
| **Strategy** | `lufis-terminal/internal/render/` | No GPU → CPU rendering fallback |
| **Repository** | `kube/backend/internal/terminal/` | Session management is procedural, not persistence-aware |
| **Factory** | `lufis-terminal/internal/pty/` | PTY creation is tightly coupled to `creack/pty` |
| **Observer** | `lufis-terminal/internal/automate/` | Events are callback-based, no pub/sub for plugin hooks |

---

## 3. Scale Concerns (100+ Simultaneous Terminals)

| Concern | Impact | Mitigation |
|---|---|---|
| Tab array linear search | `getTab()` does `tabs.find(...)` on every keystroke — O(n) per event | Use `Map<string, TabState>` |
| `captureStore` in Pinia | All terminal output goes through a single Pinia store — single writer bottleneck | Use a ring buffer per session |
| Go backend goroutine per session | Each `Start()` launches a `readLoop` goroutine — 100 sessions = 100 goroutines | OK (goroutines are cheap), but GC pressure from string allocation grows linearly |
| `lufis-terminal` plugin host | In-process Go plugins crash the terminal if a plugin panics | Use external-process plugins (JSON-RPC) as default path |

---

## 4. Specific Recommendations

### File: `lufis-terminal/internal/render/font.go`

```go
// Add FontFamily to Config
type TerminalConfig struct {
    // ...
    FontFamily string `toml:"font_family"`   // NEW
}
```

### File: `kube/backend/internal/terminal/terminal.go`

```go
// Extract interface for PTY provider
type PTYProvider interface {
    StartWithSize(cmd *exec.Cmd, size *pty.Winsize) (*os.File, error)
    Setsize(file *os.File, size *pty.Winsize) error
}
```

### File: `kube/view/src/components/terminal/TerminalView.vue`

Replace `tabs` array with a reactive Map:

```js
const tabs = reactive(new Map())
// Use: tabs.get(sessionId), tabs.set(sessionId, tab), tabs.delete(sessionId)
```

### File: `lufis-terminal/internal/ansi/parser.go`

Fix UTF-8 decoding (currently casts bytes to runes — broken for multi-byte sequences) and the SGR param 22 shadowing bug in `dispatchSGR`.
