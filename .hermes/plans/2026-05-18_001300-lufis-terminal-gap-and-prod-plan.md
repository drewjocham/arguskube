# lufis-terminal: Gap Analysis & Production-Readiness Plan

**Date:** 2026-05-18
**Status:** Planning — no execution

---

## Goal

Audit `lufis-terminal/` against the full product requirements spec
(`argus-terminal-requirements-2026-05-16.md`) and produce a prioritized,
phase-by-phase plan to bring every feature from stub/skeleton to
production-ready.

---

## Current State Summary

The codebase has **55 Go source files** with working prototypes across most
architectural packages. A GLFW window opens, spawns a PTY, parses basic ANSI,
and renders text via OpenGL. But **every feature beyond raw terminal
emulation is a skeleton or stub**. The spec calls this **mid Phase 0** with
partial Phase 1 scaffolding.

Reality check:
- **Phase 0 skeleton**: ✓ GLFW window, PTY, basic rendering, TOML config
- **Phase 1**: ⚠ Started but every module needs significant work
- **Phases 2–5**: ✗ Not started beyond package stubs and type definitions

---

## Complete Feature Inventory — Implementation Status

### 1. Terminal Core (TC-1 through TC-14)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| TC-1 | PTY session manager | ⚠ STUB | Start works. Missing: multi-session, signal forwarding (SIGWINCH wired but fragile), no graceful shutdown/context cancellation. C-03: goroutine leak on multiple Start() |
| TC-2 | Full ANSI parser | ⚠ BROKEN | 491-line custom parser. **H-05**: UTF-8 decoding completely broken (casts single bytes to runes). **H-06**: SGR param 22 shadowed, bold/dim never cleared. Missing: DCS sequences, full OSC (only title), kitty keyboard protocol, mouse tracking (SGR/URXVT/SGR1006), bracketed paste, focus reporting |
| TC-3 | GPU-accelerated rendering | ⚠ BASIC | OpenGL renders screen buffer to RGBA image each frame. Missing: shader files (GLSL vert/frag mentioned in spec but not in repo), dirty-region optimization beyond single `Dirty()` flag, 60fps target not enforced (no vsync/frame timer) |
| TC-4 | Scrollback buffer | ⚠ STUB | `scrollback.go` exists. Needs verification of ring buffer O(1) semantics, search integration, 100k+ line capacity testing |
| TC-5 | Text selection | ✗ NOT IMPLEMENTED | No selection code exists — character, word, line, block (rectangle) all missing |
| TC-6 | Clipboard (OSC 52) | ✗ NOT IMPLEMENTED | No clipboard code, no system clipboard sync, no OSC 52 handling |
| TC-7 | Tabs | ✗ NOT IMPLEMENTED | `layout.go` has Pane structs but no tab bar rendering, no open/close/reorder/rename, no colored badges |
| TC-8 | Split panes | ✗ NOT IMPLEMENTED | `layout.go` defines split directions but no drag-to-resize, no close button, no UI rendering |
| TC-9 | Command blocks | ⚠ STUB | `blocks.go` has data model (JSON file store). Not rendered in terminal, no edit/re-run/output history UI |
| TC-10 | Session persistence | ⚠ STUB | `session.go` serializes tabs/panes to JSON. Not wired: no restore on restart, no auto-save on close |
| TC-11 | Font atlas | ⚠ PARTIAL | Loads system SFNSMono/Menlo. Missing: JetBrains Mono (spec TC-11), Nerd Font glyphs, Powerline symbols, ligature support, `FontFamily` config option |
| TC-12 | Color themes | ✗ NOT IMPLEMENTED | Hardcoded `defFg`/`defBg` (gray/black). No Catppuccin, Nord, Dracula, Tokyo Night, no custom TOML theme support |
| TC-13 | Keybinding customization | ✗ NOT IMPLEMENTED | 60 lines of hardcoded Ctrl-key switch in main.go. No TOML keybinding config, no remapping |
| TC-14 | Shell integration | ⚠ STUB | 53-line stub. No preexec/precmd hooks, no command timing, no exit code display in prompt, no shell-side integration script |

### 2. Terminal Licensing Auth (AUTH-0 through AUTH-9)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| AUTH-0 | SaaS login required | ✗ NOT IMPLEMENTED | Terminal starts without auth. No login gate on first launch |
| AUTH-0a | Free tier | ✗ NOT IMPLEMENTED | No tier concept at all |
| AUTH-0c | Offline grace period | ✗ NOT IMPLEMENTED | No token caching, no 7-day grace |
| AUTH-1 | Welcome screen | ✗ NOT IMPLEMENTED | No Google OAuth button, no email+magic-link UI |
| AUTH-2 | OAuth 2.0 flow | ✗ NOT IMPLEMENTED | No localhost HTTP callback server, no browser popup |
| AUTH-3 | SaaS JWT auth | ✗ NOT IMPLEMENTED | `auth/store.go` stores API keys but no SaaS JWT flow |
| AUTH-4 | API key storage | ⚠ STUB | Keys stored in plaintext JSON on disk (security gap per audit PR-6). No input field, no auto-validate |
| AUTH-5 | Auto-detection | ✗ NOT IMPLEMENTED | No Ollama/Docker/kubectl/git/AWS/GCP detection from PATH |
| AUTH-6 | Service panel | ✗ NOT IMPLEMENTED | No UI to show connected services |
| AUTH-7 | Encrypted credential storage | ⚠ PARTIAL | `security.go` has AES-256-GCM vault. `auth/store.go` uses plaintext JSON (NOT encrypted). Audit PR-6 reframes this as "encrypt credential store at rest" |
| AUTH-8 | Token refresh | ✗ NOT IMPLEMENTED | No auto-refresh |
| AUTH-9 | Disconnect/revoke | ✗ NOT IMPLEMENTED | No credential removal, no token revocation |

### 3. OpenCode — AI Coding Runtime (OC-1 through OC-8)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| OC-1 | ACP-compatible runtime | ⚠ STUB | `opencode/agent.go` (201 lines) spawns external CLI process. Not a native ACP implementation — shells out to `opencode` binary |
| OC-2 | Toolset (read/write/edit/exec/search/grep/git/glob/diff) | ⚠ STUB | `client.go` defines tool dispatch but 8 of 9 tools are placeholder stubs returning "not implemented" |
| OC-3 | Multi-pane coding | ✗ NOT IMPLEMENTED | No pane coordination |
| OC-4 | Review mode | ✗ NOT IMPLEMENTED | No diff preview |
| OC-5 | Plugin coding tools | ✗ NOT IMPLEMENTED | No `RegisterCodingTools()` in plugin API |
| OC-6 | LSP integration | ✗ NOT IMPLEMENTED | No LSP client |
| OC-7 | No AI lock-in | ⚠ PARTIAL | Hardcoded to Ollama/OpenAI. Missing: Anthropic, DeepSeek, generic OpenAI-compatible endpoint |
| OC-8 | SaaS tier API access | ✗ NOT IMPLEMENTED | No bundled API access |

### 4. Note System (NOTE-1 through NOTE-6)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| NOTE-1 | Command block → note attachment | ⚠ STUB | `notes/store.go` has CRUD. Not wired to command blocks in rendering |
| NOTE-2 | SQLite persistence | ✗ WRONG | Uses JSON file, not SQLite as spec requires |
| NOTE-3 | Search across notes | ✗ NOT IMPLEMENTED | No search by content/tags/command |
| NOTE-4 | Export as markdown | ✗ NOT IMPLEMENTED | |
| NOTE-5 | SaaS sync | ✗ NOT IMPLEMENTED | |
| NOTE-6 | Obsidian wikilinks | ✗ NOT IMPLEMENTED | |

### 5. Obsidian Integration (OBS-1 through OBS-8)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| OBS-1 | Vault discovery | ⚠ STUB | `findVaults()` scans common paths. No `.obsidian/` folder check |
| OBS-2 | File watcher | ✗ NOT IMPLEMENTED | No fsnotify/FSEvents live watching |
| OBS-3 | Frontmatter + tag + wikilink parsing | ⚠ BASIC | Basic regex parsing, likely buggy on edge cases |
| OBS-4 | VaultQL | ⚠ STUB | Linear file scan, not indexed query |
| OBS-5 | Create/append/write .md | ⚠ BASIC | Basic file write |
| OBS-6 | Command blocks from notes | ✗ NOT IMPLEMENTED | |
| OBS-7 | Daily notes auto-link | ✗ NOT IMPLEMENTED | |
| OBS-8 | SaaS vault sync | ✗ NOT IMPLEMENTED | |

### 6. Editor (ED-1 through ED-10)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| ED-1 | Tree-sitter syntax highlighting | ✗ NOT IMPLEMENTED | `editor.go` reads/writes files as plain text. No tree-sitter, no syntax highlighting at all |
| ED-2 | Click file → split pane editor | ✗ NOT IMPLEMENTED | No UI integration |
| ED-3 | Ctrl+Click → system $EDITOR | ✗ NOT IMPLEMENTED | |
| ED-4 | Multi-tab editing | ✗ NOT IMPLEMENTED | |
| ED-5 | Auto-save on pane switch | ✗ NOT IMPLEMENTED | |
| ED-6 | File search (⌘+P) | ✗ NOT IMPLEMENTED | |
| ED-7 | LSP integration | ✗ NOT IMPLEMENTED | |
| ED-8 | Minimap | ✗ NOT IMPLEMENTED | |
| ED-9 | Theme-aware | ✗ NOT IMPLEMENTED | |
| ED-10 | OpenCode integration | ✗ NOT IMPLEMENTED | |

### 7. File Tree (FT-1 through FT-6)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| FT-1 | Directory browser | ⚠ STUB | `FileNode` struct in editor.go but no tree rendering |
| FT-2 | Git status decorations | ✗ NOT IMPLEMENTED | `GitState` field exists but not populated |
| FT-3 | .gitignore-aware | ✗ NOT IMPLEMENTED | |
| FT-4 | Filter/search | ✗ NOT IMPLEMENTED | |
| FT-5 | Multi-root workspaces | ✗ NOT IMPLEMENTED | |
| FT-6 | Right-click context menu | ✗ NOT IMPLEMENTED | |

### 8. Git Integration (GIT-1 through GIT-9)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| GIT-1 | Inline blame | ⚠ STUB | `git.go` has `Blame()` using CLI. No gutter rendering, no async, no toggle |
| GIT-2 | Status bar | ✗ NOT IMPLEMENTED | No status bar rendered at all |
| GIT-3 | Branch switcher | ✗ NOT IMPLEMENTED | |
| GIT-4 | Diff viewer | ✗ NOT IMPLEMENTED | |
| GIT-5 | Commit UI | ✗ NOT IMPLEMENTED | |
| GIT-6 | Git log browser | ✗ NOT IMPLEMENTED | |
| GIT-7 | Conflict solver | ✗ NOT IMPLEMENTED | |
| GIT-8 | Automation triggers | ✗ NOT IMPLEMENTED | |
| GIT-9 | Automation actions | ✗ NOT IMPLEMENTED | |

### 9. Automation Engine (AE-1 through AE-10)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| AE-1 | Trigger-condition-action pipeline | ⚠ BASIC | Engine runs in background goroutine. 202-line engine.go with rule evaluation loop |
| AE-2 | Terminal triggers | ⚠ PARTIAL | `TriggerCommand`, `TriggerOutput`, `TriggerExitCode`, `TriggerCron` defined. Only cron wired |
| AE-3 | System triggers | ✗ NOT IMPLEMENTED | Types defined (`TriggerBattery`, `TriggerFileChange`, `TriggerClipboard`, `TriggerIdle`). No bridge implementation |
| AE-4 | Conditions (AND/OR/NOT) | ✗ NOT IMPLEMENTED | No condition evaluation |
| AE-5 | Action types | ⚠ PARTIAL | `ActionExec`, `ActionNotify`, `ActionOpenPane`, `ActionHTTP`, `ActionSSH` defined. Only `ActionExec` wired |
| AE-6 | Google/Apple bridges | ✗ NOT IMPLEMENTED | No `bridge_google.go`, no `bridge_apple.go` |
| AE-7 | System bridge | ✗ NOT IMPLEMENTED | No `bridge_system.go` |
| AE-8 | Plugin extensibility | ✗ NOT IMPLEMENTED | No `RegisterTriggers`/`RegisterActions` in plugin API |
| AE-9 | Rule storage | ⚠ STUB | Rules in-memory only. No SQLite/TOML/.rule persistence |
| AE-10 | SaaS sync | ✗ NOT IMPLEMENTED | |

### 10. Plugin System (PLUGIN-1 through PLUGIN-9)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| PLUGIN-1 | Plugin API hooks | ⚠ PARTIAL | 6 hooks defined. Only `command_executed` has any wiring |
| PLUGIN-2 | UI contribution points | ⚠ STUB | `RegisterCommand`, `RegisterSidebarPanel` exist. No toolbar/statusbar/context-menu contributions |
| PLUGIN-3 | Completions/hints | ✗ NOT IMPLEMENTED | No `ProvideCompletions`, no `ProvideHints` |
| PLUGIN-4 | Automation registration | ✗ NOT IMPLEMENTED | |
| PLUGIN-5 | Coding tool registration | ✗ NOT IMPLEMENTED | |
| PLUGIN-6 | Plugin types | ⚠ STUB | In-process Go only. No JSON-RPC external process, no WebView |
| PLUGIN-7 | Plugin manifest | ✗ NOT IMPLEMENTED | No `plugin.toml` support |
| PLUGIN-8 | Plugin store | ✗ NOT IMPLEMENTED | No install/update/remove/hot-reload |
| PLUGIN-9 | Tool dashboards | ✗ NOT IMPLEMENTED | No Grafana/Prometheus/Terraform/ArgoCD/Popeye dashboards |

### 11. Database Diagnostics Plugin (DB-1 through DB-9)

| ID | Requirement | Status |
|----|------------|--------|
| DB-1 through DB-9 | All | ✗ STUB — `plugins/db-diag/plugin.go` has `Name()`/`Version()`/`Init()`/`Shutdown()` returning nil. No real implementation |

### 12. Infra Diagnostics Plugin (ID-1 through ID-9)

| ID | Requirement | Status |
|----|------------|--------|
| ID-1 through ID-9 | All | ✗ STUB — `plugins/infra-diag/plugin.go` has `Name()`/`Version()`/`Init()`/`Shutdown()` returning nil. No real implementation |

### 13. Meeting Recorder (MR-1 through MR-12)

| ID | Requirement | Status | What's Missing |
|----|------------|--------|----------------|
| MR-1 | Laptop mic capture | ✗ STUB | `recorder.go` (112 lines) stores recording metadata. No actual audio capture — no portaudio, no CoreAudio, no mic input |
| MR-2 | System audio capture | ✗ NOT IMPLEMENTED | No speaker/loopback capture |
| MR-3 | One-click start/stop | ✗ NOT IMPLEMENTED | No UI integration, no Ctrl+Shift+R shortcut |
| MR-4 | Local processing (Whisper) | ✗ NOT IMPLEMENTED | No Whisper integration |
| MR-5 | Cloud processing | ✗ NOT IMPLEMENTED | |
| MR-6 | Speaker diarization | ✗ NOT IMPLEMENTED | |
| MR-7 | AI summary | ✗ NOT IMPLEMENTED | |
| MR-8 | Task extraction | ✗ NOT IMPLEMENTED | |
| MR-8a | Task output (Google Tasks/Apple Reminders/Argus Cloud) | ✗ NOT IMPLEMENTED | |
| MR-9 | Note output | ✗ NOT IMPLEMENTED | |
| MR-10 | Export | ✗ NOT IMPLEMENTED | |
| MR-11 | Recording indicator | ✗ NOT IMPLEMENTED | |
| MR-12 | Privacy | ✗ NOT IMPLEMENTED | No local-by-default enforcement, no opt-in cloud flag |

### 14. SaaS Offload (SAAS-1 through SAAS-4)

| ID | Requirement | Status |
|----|------------|--------|
| SAAS-1 | SaaS sync services | ✗ NOT IMPLEMENTED |
| SAAS-2 | Docker containers | ✗ NOT IMPLEMENTED |
| SAAS-3 | Container lifecycle | ✗ NOT IMPLEMENTED |
| SAAS-4 | Graceful degradation | ✗ NOT IMPLEMENTED |

### 15. SaaS Downstream Auth (SAAS-AUTH-1 through SAAS-AUTH-3)

| ID | Requirement | Status |
|----|------------|--------|
| SAAS-AUTH-1 | SaaS proxy model | ✗ NOT IMPLEMENTED |
| SAAS-AUTH-2 | Google Drive/Gmail/Calendar | ✗ NOT IMPLEMENTED |
| SAAS-AUTH-3 | Local API keys direct | ⚠ PARTIAL (store exists but no UI) |

### 16. Cross-Cutting / Phase 4–5 Features

| Feature | Status |
|---------|--------|
| `internal/crash/reporter.go` | ⚠ STUB — writes crash reports to JSON files. Not wired to main.go, no telemetry upload |
| `internal/i18n/i18n.go` | ⚠ STUB — 154-line bundle with EN/DE/FR/ES/JA/ZH. Only default English messages exist |
| `internal/palette/` (k8s, solace, rabbitmq, pubsub) | ⚠ STUB — shell-type registrations in the palette system. No rendering integration |
| Accessibility | ✗ Not started |
| Auto-update | ✗ Not started |
| Native packaging (DMG/.deb/.msi/Homebrew) | ✗ Not started |
| Community plugin registry | ✗ Not started |
| Collaborative sessions | ✗ Not started |
| Mobile companion app | ✗ Not started |

---

## Known Critical Bugs (from QA Report)

1. **C-03 / H-05**: UTF-8 decoding broken — multi-byte sequences produce garbage
2. **H-06**: SGR param 22 shadowed — bold/dim never cleared
3. **C-03**: PTY goroutine leak on multiple Start() calls
4. **Audit #5**: `os.Setenv` not goroutine-safe in config tests
5. **Audit #4**: Tests not table-driven (violates AGENTS.md)
6. **Audit #6**: Credential store uses plaintext JSON (security gap)

---

## Production-Readiness Plan — 7 Phases

### Phase 0: Fix What's Broken (2 weeks)

Fix all known critical bugs before adding any features. A terminal with broken
UTF-8 and leaked file descriptors is not a foundation to build on.

| Task | Effort | Depends On |
|------|--------|------------|
| Fix H-05: rewrite UTF-8 decoder with proper multi-byte sequence handling | 0.5 day | — |
| Fix H-06: reorder SGR switch cases so param 22 hits before range 21-29 | 0.5 day | — |
| Fix C-03: add context cancellation to PTY readLoop, close old PTY before new Start() | 1 day | — |
| Convert ANSI parser tests to table-driven format (per AGENTS.md) | 1 day | H-05, H-06 fixes |
| Convert terminal_test.go to table-driven | 2 hours | — |
| Fix `os.Setenv` race in config tests (use `t.Setenv`) | 0.5 day | — |
| Encrypt credential store (audit PR-6: replace plaintext JSON with AES-GCM via existing security.Vault OR 99designs/keyring) | 1 day | — |
| Add missing ANSI sequences: mouse tracking (SGR1006), bracketed paste, kitty keyboard progressive enhancement | 2 days | H-05 fix |
| Wire crash reporter into main.go (defer+recover) | 0.5 day | — |

**Phase 0 deliverable**: a terminal that correctly renders UTF-8, handles all
standard escape sequences, doesn't leak resources, and stores credentials
encrypted. Tests are table-driven and pass reliably.

### Phase 1: Complete the Terminal Core (3 weeks)

Make it a real terminal, not a tech demo.

| Task | Effort | Depends On |
|------|--------|------------|
| **Text selection** — character/word/line/block modes, visual highlight, copy to clipboard | 3 days | — |
| **Clipboard** — OSC 52 read/write, system clipboard sync (macOS pbcopy/pbpaste, Linux xclip, Windows clipboard) | 1 day | Selection |
| **GPU rendering pipeline hardening** — add GLSL shader files (vert/frag), vsync frame timer, dirty-region optimization (track dirty rects per-cell, not one global flag) | 3 days | — |
| **Font system upgrade** — bundle JetBrains Mono + Nerd Font glyphs, add `FontFamily` config, ligature support via HarfBuzz or manual glyph substitution, Powerline symbols | 3 days | — |
| **Scrollback verification** — test 100k+ line capacity, add search (Ctrl+Shift+F with regex), verify O(1) ring buffer | 2 days | — |
| **Color themes** — implement Catppuccin, Nord, Dracula, Tokyo Night as embedded TOML, add theme switching, custom TOML theme loading | 2 days | — |
| **Tabs** — tab bar rendering (OpenGL), open/close/reorder/rename, colored badges (exit code, git status), keyboard nav (Ctrl+Tab, Ctrl+Shift+Tab) | 3 days | GPU pipeline |
| **Keybinding customization** — extract hardcoded switch from main.go into keybinding registry, TOML config for remapping, action dispatch system | 2 days | — |
| **Shell integration** — preexec/precmd hook injection, command timing display, exit code in prompt, ship zsh/bash integration scripts | 2 days | — |

**Phase 1 deliverable**: daily-drivable terminal with tabs, themes, selection,
clipboard, custom keybindings, and shell integration. Complete TC-1 through TC-14.

### Phase 2: Auth & Licensing (2 weeks)

Without auth, there is no commercial product.

| Task | Effort | Depends On |
|------|--------|------------|
| **First-launch welcome screen** — OpenGL-rendered auth UI: "Sign in with Google" button, email+magic-link option, skip/local-only mode | 3 days | Phase 0 credential encryption |
| **OAuth 2.0 flow** — localhost HTTP callback server (random port), browser popup, token exchange, auto-close server | 2 days | Welcome screen |
| **SaaS JWT auth** — connect to Argus Cloud, validate JWT, extract tier/entitlements | 2 days | OAuth flow |
| **API key input + validate** — styled input field, auto-validate on paste (test API call), masked display | 1 day | Welcome screen |
| **Auto-detection** — scan PATH for Ollama, Docker, kubectl, git, AWS CLI, GCloud. Show discovered services in welcome flow | 1 day | — |
| **Offline grace period** — cache JWT with 7-day expiry, encrypted local storage, re-auth prompt on expiry | 1 day | SaaS JWT |
| **Service panel** — UI showing all connected/discovered services with status indicators and one-click connect | 2 days | Auto-detection |
| **Token refresh + revoke** — background refresh goroutine, disconnect button clears credentials + revokes server-side | 1 day | SaaS JWT |

**Phase 2 deliverable**: terminal requires login, supports Google OAuth, stores
API keys, auto-detects tools. Offline mode works for 7 days.

### Phase 3: Built-in Features (4 weeks)

Notes, command blocks, session persistence, Obsidian, OpenCode hardening.

| Task | Effort | Depends On |
|------|--------|------------|
| **Notes → SQLite migration** — replace JSON store with `mattn/go-sqlite3`, add search, tags, export-to-markdown | 2 days | — |
| **Command blocks UI** — render blocks in terminal with edit/re-run/output-history, attach notes to blocks, fold/collapse | 3 days | Notes SQLite |
| **Session persistence** — auto-save on close, restore tabs/panes/working-dirs on restart, serialize PTY state | 2 days | Tabs (Phase 1) |
| **Split panes** — horizontal/vertical split, drag-to-resize divider, close button per pane, keyboard shortcuts. One PTY per pane | 3 days | Tabs (Phase 1) |
| **Git integration** — inline blame (rendered in left gutter), status bar (branch, changes, CI), branch switcher (⌘+Shift+G with fuzzy find) | 4 days | Split panes |
| **Git — diff viewer + commit UI** — split-pane diff, hunk staging, commit message editor, conventional commit helpers | 3 days | Git blame/status |
| **Git — log browser** — visual branch graph (OpenGL line primitives), search, action buttons, commit detail popup | 2 days | Git diff |
| **Obsidian hardening** — file watcher (fsnotify on macOS/fsnotify elsewhere), indexed VaultQL, frontmatter validation, daily-note auto-link | 3 days | — |
| **OpenCode hardening** — implement all 9 toolset stubs (read/write/edit/exec/search/grep/git/glob/diff), add Anthropic + DeepSeek + generic OpenAI-compatible providers | 3 days | — |
| **Editor** — tree-sitter Go/Python/TS/Rust/YAML/JSON/Markdown grammars, syntax highlighting via OpenGL vertex colors, multi-tab, auto-save, ⌘+P fuzzy file search, Ctrl+Click → open in $EDITOR | 5 days | Split panes |
| **File tree** — directory browser with expand/collapse, git status decorations, .gitignore filtering, right-click context menu | 3 days | Editor, Git |
| **Command palette** — ⌘+Shift+P with fuzzy search across all commands, actions, open files, git branches | 2 days | Keybinding system (Phase 1) |

**Phase 3 deliverable**: full editor, git, Obsidian, notes, command blocks,
split panes, and session persistence. This is a terminal you can use as a daily
driver.

### Phase 4: Automation Engine & Bridges (3 weeks)

| Task | Effort | Depends On |
|------|--------|------------|
| **System bridge** — battery polling (IOKit on macOS, upower on Linux), WiFi SSID monitor, file watcher bridge, clipboard monitor, cron scheduler, idle detection | 4 days | — |
| **Condition evaluation** — AND/OR/NOT engine, chained conditions, type-safe param validation | 2 days | — |
| **Complete action dispatch** — wire ActionNotify, ActionOpenPane, ActionHTTP, ActionSSH with error modes (ignore/notify/stop/retry) | 2 days | — |
| **Rule persistence** — SQLite rule store, TOML config loading, `.rule` file import, enable/disable toggle | 2 days | — |
| **Google bridge** — OAuth 2.0 (encrypted storage), Gmail push/poll, Calendar watch, Drive file monitor, Sheets/Docs/Tasks API | 5 days | Phase 2 auth |
| **Apple bridge** — AppleScript/osascript for Mail/Calendar/Reminders/Notes/Messages, Shortcuts integration, CoreLocation geofence CLI, focus mode detection | 4 days | — |
| **Plugin extensibility** — `RegisterTriggers()`, `RegisterActions()` in plugin API, validation, sandboxed execution | 2 days | — |
| **Automation UI** — rule editor (TOML or visual), test-run button, execution log, enable/disable per-rule | 2 days | — |

**Phase 4 deliverable**: full IFTTT engine with Google + Apple bridges, rule
persistence, and plugin-extensible triggers/actions.

### Phase 5: Plugin System Hardening (2 weeks)

| Task | Effort | Depends On |
|------|--------|------------|
| **External process plugins** — JSON-RPC over stdio (sourcegraph/jsonrpc2), plugin lifecycle (start/stop/health-check), crash isolation, stdout/stderr capture | 4 days | — |
| **Plugin manifest** — `plugin.toml` spec: name, version, hooks, UI contributions, commands, runtime config, dependency declarations | 2 days | — |
| **Completions + hints** — `ProvideCompletions()` API, `ProvideHints()` API, hook into command palette | 2 days | Plugin manifest |
| **Coding tool registration** — `RegisterCodingTools()` API, tool dispatch in OpenCode agent, sandboxing | 2 days | OpenCode (Phase 3) |
| **Plugin store** — local registry index, install from Git/HTTP, version resolution, update check, hot-reload on file change | 3 days | Plugin manifest |
| **WebView plugin support** — embedded WebView for UI-heavy plugins (dashboards, charts), message passing bridge | 0 day (defer) | — |

**Phase 5 deliverable**: fully operational plugin ecosystem with external
process isolation, completions, coding tool registration, and a local plugin
store.

### Phase 6: Meeting Recorder (2 weeks)

| Task | Effort | Depends On |
|------|--------|------------|
| **Audio capture** — macOS CoreAudio (portaudio CGo binding or exec `sox`/`ffmpeg`), mic input device selection, system audio loopback (BlackHole/Soundflower dependency or CoreAudio aggregate device) | 4 days | — |
| **Recording UI** — Ctrl+Shift+R toggle, red dot status bar indicator, elapsed time, pause/resume | 2 days | Phase 1 status bar |
| **Local transcription** — bundle whisper.cpp or call local Ollama Whisper model, stream-to-transcript, speaker diarization (simple silence-based or pyannote) | 4 days | Audio capture |
| **AI summary + tasks** — call OpenCode agent with transcript, generate summary + action items + decisions | 2 days | Local transcription, OpenCode |
| **Task dispatch** — Google Tasks API, Apple Reminders (osascript), Argus Cloud tasks, user preference config | 2 days | Google bridge (Phase 4), Apple bridge (Phase 4) |
| **Export** — markdown, Google Doc, S3 (same report targets as diagnostics) | 1 day | — |
| **Privacy** — local-by-default toggle, opt-in cloud, recording indicator always-on, auto-delete after N days config | 1 day | — |

**Phase 6 deliverable**: one-click meeting recording with local Whisper
transcription, AI summary, and task extraction to Google Tasks / Apple
Reminders.

### Phase 7: Production Polish (3 weeks)

| Task | Effort | Depends On |
|------|--------|------------|
| **Performance** — profile GPU rendering at 4K, optimize draw call batching, benchmark scrollback at 100k lines, reduce frame time variance | 3 days | Phase 1 GPU |
| **Crash reporting + telemetry** — wire reporter.go into main, optional upload to SaaS, anonymized perf metrics | 2 days | Phase 2 auth |
| **Auto-update** — Sparkle framework on macOS, apt repository on Linux, MSI auto-update on Windows. Version check on startup | 3 days | — |
| **Native packaging** — DMG with code-sign + notarization (macOS), .deb + .rpm (Linux), .msi (Windows), Homebrew cask | 4 days | — |
| **Accessibility** — VoiceOver support, high-contrast theme, full keyboard navigation, screen reader announcements for terminal output changes | 3 days | — |
| **Internationalization** — complete DE/FR/ES/JA/ZH message bundles, RTL rendering support, IME input composition window | 3 days | Phase 1 tabs |
| **Error handling audit** — review every `_ =` error suppression, add proper logging/recovery, ensure no silent data loss (audit G-04) | 2 days | — |
| **Integration tests** — end-to-end test: launch terminal → spawn PTY → run commands → verify output → close cleanly. CI pipeline with headless rendering (Xvfb/offscreen) | 3 days | — |

**Phase 7 deliverable**: production-quality, packaged, signed, auto-updating,
accessible, internationalized terminal application.

---

## Dependency Graph

```
Phase 0 (Fix bugs)
  └─→ Phase 1 (Terminal core)
        ├─→ Phase 2 (Auth)
        │     └─→ Phase 4 (Automation bridges — needs auth for Google OAuth)
        ├─→ Phase 3 (Built-in features)
        │     └─→ Phase 5 (Plugin hardening — needs OpenCode, editor, git)
        │           └─→ Phase 6 (Meeting recorder — needs OpenCode, bridges)
        └─→ Phase 7 (Polish — can start in parallel but needs all prior phases to finish)
```

---

## Total Estimated Effort

| Phase | Weeks | Cumulative |
|-------|-------|------------|
| 0: Fix bugs | 2 | 2 |
| 1: Terminal core | 3 | 5 |
| 2: Auth | 2 | 7 |
| 3: Built-in features | 4 | 11 |
| 4: Automation engine | 3 | 14 |
| 5: Plugin hardening | 2 | 16 |
| 6: Meeting recorder | 2 | 18 |
| 7: Production polish | 3 | 21 |

**~21 weeks (~5 months) to full production readiness for a single developer.**
A team of 3 could parallelize heavily (Phases 1+2+3 overlap significantly) and
deliver in ~3 months.

---

## Risks & Open Questions

1. **GPU rendering at scale** — go-gl/glfw has limited community. If OpenGL 4.1
   proves unstable on diverse hardware, consider Vulkan via go-vk or fall back
   to a software renderer using a retained-mode UI framework. This is the
   single biggest technical risk.

2. **CGo dependency proliferation** — go-sqlite3, portaudio, whisper.cpp,
   tree-sitter all require CGo. Cross-compilation becomes harder. Consider
   pure-Go alternatives where possible (modernc.org/sqlite, Go-based audio
   capture, pure-Go Whisper inference).

3. **JetBrains Mono bundling** — the font is SIL OFL licensed and can be
   bundled. But Nerd Font patched versions are large (~5MB per variant).
   Consider on-demand glyph generation rather than bundling full patched fonts.

4. **Meeting recorder audio loopback** — capturing system audio on macOS
   requires either BlackHole (3rd-party kext, user must install) or a
   CoreAudio aggregate device (complex, fragile). This feature may need to ship
   as mic-only initially.

5. **Plugin store scope creep** — a full plugin registry with publishing,
   versioning, and community moderation is a product in itself. Phase 5 ships a
   local-only store. The community registry is deferred to Phase 4 of the spec
   (post-1.0).

6. **SaaS backend dependency** — auth, sync, and SaaS offload features all
   require a running Argus Cloud backend. The terminal must work fully offline
   (graceful degradation) when the backend is unreachable. This is designed
   into the spec (SAAS-4) but must be tested rigorously.
