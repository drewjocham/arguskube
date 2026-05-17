# Terminal Module Review

> Review against project coding guidelines (AGENTS.md)
> Date: 2026-05-17

---

## 1. Module Overview

**60+ terminal-related files** across two subsystems:

| Subsystem | Path | Purpose |
|-----------|------|---------|
| **Backend Terminal** | `kube/backend/internal/terminal/` | Embedded PTY sessions exposed via Wails/xterm.js |
| **Standalone Terminal** | `lufis-terminal/` | Full GPU-accelerated native terminal (GLFW/OpenGL, 30+ packages) |
| **API Handlers** | `kube/backend/api/pkg/app_terminal_*.go` | Launcher, copilot, and handler glue |
| **Frontend** | `kube/view/src/{components,features,stores}/terminal/` | Vue/xterm.js terminal UI |
| **Build** | `Makefile` + `lufis-terminal/Makefile` | `build-terminal-app` and `argus-terminal` binary |

---

## 2. @architect — Architecture Review

**Score: B−**

### Issues Found

| Severity | File | Issue | Guideline Violated |
|----------|------|-------|--------------------|
| **HIGH** | `kube/backend/internal/terminal/terminal.go` + `lufis-terminal/internal/pty/pty.go` | **Duplicate PTY management** — both modules independently manage PTY sessions with nearly identical lifecycle code | DRY Principle |
| **HIGH** | `kube/backend/api/pkg/terminal_handler.go` | **Handler too thick** — directly manages sessions instead of delegating to a service layer | Handler Pattern (AGENTS.md) |
| **HIGH** | `lufis-terminal/cmd/argus-terminal/main.go` | **No dependency injection** — logger/config created manually in `main` | Dependency Injection |
| **MEDIUM** | `kube/backend/internal/terminal/terminal_test.go` | **Not table-driven** — 14 standalone `TestXxx` functions | Table-Driven Tests (AGENTS.md — mandatory) |
| **MEDIUM** | `kube/backend/api/pkg/app_terminal_launcher.go` | Some error returns lack `%w` context wrapping | Error Handling (wrap with `fmt.Errorf %w`) |
| **LOW** | `kube/backend/internal/terminal/session.go` | No `context.Context` integration for session lifecycle | Concurrency (AGENTS.md mandates `context.Context`) |

### Strengths

- OS-specific launcher uses **Adapter pattern** correctly (`app_terminal_launcher_unix.go` / `_windows.go`)
- PTY abstraction in `lufis-terminal/internal/pty/` properly separates domain from infrastructure
- `sync.Mutex` governance for concurrency is solid
- Imports follow AGENTS.md grouping (stdlib → third-party → local)

### Top 3 Recommended Changes

1. **Consolidate PTY management** into a shared library to eliminate duplication
2. **Introduce a service layer** between `terminal_handler.go` and `SessionManager`
3. **Implement dependency injection** for logger/config in `lufis-terminal/cmd/argus-terminal/main.go`

---

## 3. @library-curator — "Wheel" Audit

| Module | Custom Code | Mature Alternative | Severity | Recommendation |
|--------|-------------|--------------------|----------|---------------|
| `internal/terminal/` | PTY management | Already uses `creack/pty` | MEDIUM | **Keep** (already optimal) |
| `internal/screen/` | Terminal screen buffer / emulation (~350 lines) | `go-libvterm` | **HIGH** | Consider replacement |
| `internal/plugin/` | Custom plugin architecture | `hashicorp/go-plugin` | **HIGH** | Migrate |
| `internal/auth/` | Token storage in JSON | `golang-jwt/jwt` | **CRITICAL** | Consider replacement |
| `internal/ansi/` | ANSI escape parser (~350 lines) | `github.com/aerth/go-ansi` | MEDIUM | Evaluate |
| `internal/config/` | Manual TOML config | `spf13/viper` + `pflag` | MEDIUM | Evaluate |
| `internal/crash/` | Custom crash reporter | `getsentry/sentry-go` | MEDIUM | Evaluate |
| `internal/recorder/` | Terminal session recording | `tview` / `bubbletea` | LOW | Keep or enhance |
| `internal/editor/` | Custom editor integration | `charmbracelet/bubbletea` | LOW | Keep |
| `internal/git/` | Custom git integration | `go-git/go-git` | LOW | Keep |
| `internal/i18n/` | Custom i18n | `go-i18n/go-i18n` | LOW | Evaluate |

### ⚠️ Security Note

`internal/auth/` handling token storage is labeled **CRITICAL** — custom auth storage is a prime vector for security vulnerabilities. Should leverage established auth libraries.

---

## 4. @context-archivist — Proposed `.context.md` Section

```markdown
## Terminal Module

### Purpose
Two terminal subsystems:
1. **Backend Terminal** (`kube/backend/internal/terminal/`) — PTY session lifecycle for Wails/xterm.js
2. **Standalone Terminal** (`lufis-terminal/`) — GPU-accelerated native terminal (GLFW/OpenGL)

### Key Types
| Type | Package | Responsibility |
|------|---------|---------------|
| `Terminal` | `kube/backend/internal/terminal` | PTY lifecycle (start, write, resize, close) |
| `Session` | `kube/backend/internal/terminal` | Session metadata + domain context |
| `SessionManager` | `kube/backend/internal/terminal` | CRUD for terminal sessions |
| `Domain` | `kube/backend/internal/terminal` | Enum: default, k8s, kafka, cloud |
| `PTY` | `lufis-terminal/internal/pty` | PTY lifecycle in standalone terminal |

### Key Dependencies
- `github.com/creack/pty` — PTY management (both subsystems)
- `github.com/wailsapp/wails/v2` — Backend terminal window
- `github.com/go-gl/glfw` — Standalone terminal window
- `github.com/go-gl/gl` — Standalone terminal rendering
- `github.com/golang/freetype` — Font rendering

### Build Targets
- `make build-terminal-app` — Build Wails terminal `.app` bundle
- `lufis-terminal/Makefile build` — Build standalone `argus-terminal` binary

### Test Coverage
| Package | Tests | Table-Driven? | Coverage |
|---------|-------|---------------|----------|
| `kube/backend/.../terminal` | 14 tests | ❌ (standalone `TestXxx`) | Good |
| `lufis-terminal/.../pty` | 13 tests | ✅ | Excellent |
| `lufis-terminal/.../config` | 11 tests | ✅ | Excellent |
| `lufis-terminal/.../screen` | 28 tests | ✅ | Excellent |
| `lufis-terminal/.../ansi` | 38 tests | ✅ | Excellent |
| `lufis-terminal/.../auth` | 6 tests | ✅ | Good |
| Frontend stores/composables | 5 test files | N/A | Good |

### Known Technical Debt
1. PTY code duplication between `kube/backend/internal/terminal/terminal.go` and `lufis-terminal/internal/pty/pty.go`
2. Backend `terminal_test.go` violates mandatory table-driven test requirement
3. `terminal_handler.go` bypasses service layer (thick handler anti-pattern)
4. `lufis-terminal/internal/ansi/` + `internal/screen/` = ~700 lines of custom terminal emulation that could use `go-libvterm`
5. Plugin system is fully custom — consider `hashicorp/go-plugin`
6. No `context.Context` in session lifecycle management
```

---

## 5. Summary: Top 5 Actions

| # | Action | Priority | Effort |
|---|--------|----------|--------|
| 1 | **Consolidate PTY management** into a shared library | High | Medium |
| 2 | **Refactor `terminal_handler.go`** — add service layer | High | Small |
| 3 | **Convert `terminal_test.go`** to table-driven | High | Small |
| 4 | **Replace custom `screen/` + `ansi/`** with `go-libvterm` | Medium | Large |
| 5 | **Replace custom plugin system** with `hashicorp/go-plugin` | Medium | Medium |
