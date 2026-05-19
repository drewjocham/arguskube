# Context Update — Argus Terminal

**Reviewer:** @context-keeper
**Date:** 2026-05-16

---

## Current State Summary

The Argus project has **two terminal implementations** at different maturity levels:

### 1. Kube Web Terminal (Vue.js/xterm.js) — Production

**Location:** `kube/view/src/components/terminal/TerminalView.vue`
**Status:** Production, actively used in the ArgusKube dashboard

- 401-line single-file component with tabs, copilot, pod exec, context switching
- Uses xterm.js with xterm-addon-fit for terminal emulation
- Pinia stores: `terminalDispatch` (command dispatch), `runbookTerminals` (session pinning/overrides)
- Go backend PTY handler at `kube/backend/internal/terminal/terminal.go` (198 lines)

**Font issue:** Uses `'Cascadia Mono'` as primary font — not available on macOS. Should prioritize `SF Mono` or use `ui-monospace` CSS keyword.

### 2. Native Go Terminal (lufis-terminal) — In Development

**Location:** `lufis-terminal/`
**Status:** Phase 0-1 build (GLFW window visible, basic rendering, PTY)

- OpenGL 4.1 rendering via go-gl/glfw
- Plugin system with ArgusKube, infra-diag, db-diag plugins
- Automation engine with trigger-condition-action pipeline
- Note system (SQLite-backed), auth store, encryption

**Font issue:** `font.go` loads from system paths (SFNSMono, Menlo) but has no `FontFamily` config option and no JetBrains Mono support (required per spec TC-11).

---

## Key Findings from Reviews

### @architect
- Code duplication: `Start()` / `StartWithEnv()` — 90% identical
- No adapter/repository/strategy patterns across terminal backends
- Font system lacks fallback chain, JetBrains Mono, Nerd Fonts

### @explore
- **Smoking gun:** `TerminalView.vue:64` uses Cascadia Mono (Windows font) as first priority
- Inconsistent font stacks between embedded and pop-out terminals
- No `@font-face` declarations — all fonts must be pre-installed

### @qa-tester
- **C-01:** Module-level `lastSessionId` bleeds across component instances
- **C-02:** Shell command injection in `execIntoPod`
- **C-03:** Goroutine leak in Go backend
- **H-05:** UTF-8 decoding completely broken in ANSI parser
- **H-06:** SGR param 22 shadowed by range case (bold/dim never cleared)

### @test-generator
- Go backend tests NOT table-driven (violates AGENTS.md)
- `time.Sleep(500ms)` makes tests flaky
- `StartWithEnv`, `readLoop` completely untested (P0 gaps)
- TerminalView.test.js missing initError, tab lifecycle, copilot tests

---

## Open Questions

1. Should the two terminal implementations share code, or remain independent?
2. Should we bundle JetBrains Mono (per spec) or rely on system fonts?
3. Is the `lufis-terminal` project actively being developed, or is the Kube web terminal the primary focus?
