# Argus Terminal — Product Requirements & Build Plan

**Author**: Drew & Buddy
**Date**: 2026-05-16
**Status**: Spec complete, ready for Phase 1 build

---

## Product Vision

A GPU-accelerated terminal for software engineers, built in Go, with a first-class plugin/addon ecosystem. Not a Warp clone — a platform where the terminal core is small and fast, and everything else is a plugin. Built for the ArgusKube ecosystem first, but open to all developers.

Key differentiators:
- **Platform, not a product** — plugin API with UI hooks, automation triggers, completion providers, tool registration
- **No AI lock-in** — BYO provider (OpenAI, Anthropic, DeepSeek, Ollama, any OpenAI-compatible), no preferential treatment
- **Two-click auth** — Google OAuth or local mode. Everything is optional and discoverable
- **Built-in tools you actually need** — notes, editor, git, Obsidian, automation engine, OpenCode runtime — no plugins required

---

## Target Audience

1. **Kubernetes operators** — ArgusKube users managing clusters, load testing, incident response
2. **Platform engineers** — Terraform, Prometheus, Grafana, databases, infra diagnostics
3. **General developers** — git, editor, OpenCode AI coding agent, notes, automation
4. **Teams** — SaaS sync, shared automations, collaborative sessions (Phase 4)
5. **Anyone in meetings** — standalone recorder using laptop mic/speakers, no app integration needed

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Terminal Core (Go)                       │
│  PTY / GPU Render / Input / ANSI / Clipboard / Selection     │
│  Layout / Tabs / Splits / Config / Plugin Host               │
├─────────────────────────────────────────────────────────────┤
│             Built-in / First-Class Features                  │
│  ┌──────┬───────┬──────┬────────┬───────┬────────┬──────┐  │
│  │Notes │Editor │ Git  │Obsidian│OpenCode│Auto-   │Auth  │  │
│  │      │       │      │        │(AI     │mation  │(OAuth│  │
│  │      │       │      │        │Coding) │Engine  │+Keys)│  │
│  └──────┴───────┴──────┴────────┴───────┴────────┴──────┘  │
├─────────────────────────────────────────────────────────────┤
│                   Plugin System                              │
│  ╔═══════════════════════════════════════════════════════╗   │
│  ║ First-Party Plugins (flagship)                        ║   │
│  ║  ArgusKube │ DB Diagnostics │ Infra Diagnostics       ║   │
│  ║  Tool Dashboards (Grafana, Prometheus, Terraform...)  ║   │
│  ║  SaaS Sync (vaults, notes, rules, creds)              ║   │
│  ╠═══════════════════════════════════════════════════════╣   │
│  ║ Community Plugins (open store)                        ║   │
│  ║  Docker │ SSH Manager │ Theme Studio │ ...            ║   │
│  ╚═══════════════════════════════════════════════════════╝   │
└─────────────────────────────────────────────────────────────┘
```

---

## Feature Requirements

### 1. Terminal Core

| ID | Requirement | Priority |
|---|---|---|
| TC-1 | PTY session manager — spawn, resize, signal shell processes | P0 |
| TC-2 | Full ANSI parser — SGR colors/styles, cursor, scroll, DCS, OSC, CSI, bracketed paste, kitty keyboard, mouse tracking | P0 |
| TC-3 | GPU-accelerated rendering — OpenGL 4.1 via go-gl/glfw, font atlas, 60fps at 4K, dirty-region optimization | P0 |
| TC-4 | Scrollback buffer — 100k+ lines, O(1) ring buffer, search | P0 |
| TC-5 | Text selection — character, word, line, block (rectangle), copy | P0 |
| TC-6 | Clipboard — OSC 52, system clipboard sync | P0 |
| TC-7 | Tabs — open, close, reorder, rename, colored badges | P0 |
| TC-8 | Split panes — horizontal/vertical, drag to resize, close button | P1 |
| TC-9 | Command blocks — editable, re-runnable, with output history | P1 |
| TC-10 | Session persistence — restore tabs and panes on restart | P1 |
| TC-11 | Font atlas — JetBrains Mono, Nerd Font glyphs, powerline, ligatures | P0 |
| TC-12 | Color themes — Catppuccin, Nord, Dracula, Tokyo Night, custom TOML | P1 |
| TC-13 | Keybinding customization — TOML config, all actions remappable | P1 |
| TC-14 | Shell integration — preexec/precmd hooks, timing, exit code display | P1 |

### 2. Terminal Licensing Auth (SaaS Login Required)

The terminal requires a SaaS account to use. Not freeware — it's a commercial product with a free tier.

| ID | Requirement | Priority |
|---|---|---|
| AUTH-0 | **Terminal requires SaaS login** — no anonymous use. First launch forces auth. | P0 |
| AUTH-0a | Free tier: local terminal + limited automations + 1 device | P0 |
| AUTH-0b | Paid tier: sync, AI credits, team features, unlimited devices | P1 |
| AUTH-0c | Offline grace period — 7 days cached token, then re-auth required | P0 |
| AUTH-1 | Welcome screen on first launch — "Sign in with Google" or "Email + magic link" | P0 |
| AUTH-2 | OAuth 2.0 flow — localhost HTTP server for callback, browser popup, auto-close | P0 |
| AUTH-3 | SaaS JWT auth — connect to Argus Cloud via Google OAuth or email+magic-link | P0 |
| AUTH-4 | API key storage — single input field for OpenAI/Anthropic/DeepSeek keys, auto-validate | P0 |
| AUTH-5 | Auto-detection — Ollama, Docker, kubectl, Git, AWS, GCP from PATH/config files | P0 |
| AUTH-6 | Service panel — show all connected/discovered services, one-click connect | P1 |
| AUTH-7 | Encrypted credential storage — AES-256-GCM in SQLite, macOS Keychain secondary | P0 |
| AUTH-8 | Token refresh — auto-refresh OAuth tokens before expiry | P1 |
| AUTH-9 | Disconnect/revoke — remove credentials, revoke tokens | P1 |

### 3. OpenCode — AI Coding Runtime (Built-in)

| ID | Requirement | Priority |
|---|---|---|
| OC-1 | ACP-compatible runtime — spawn coding agent in any pane, BYO model | P0 |
| OC-2 | Toolset — read, write, edit, exec, search, grep, git, glob, diff | P0 |
| OC-3 | Multi-pane coding — agent in one pane, shell in another, git log in third | P1 |
| OC-4 | Review mode — diff preview before applying changes | P1 |
| OC-5 | Plugin tools — plugins register custom tools via `RegisterCodingTools()` | P1 |
| OC-6 | LSP integration — go-to-definition, diagnostics, symbol search for smarter edits | P2 |
| OC-7 | No AI lock-in — OpenAI, Anthropic, DeepSeek, Ollama, any OpenAI-compatible endpoint | P0 |
| OC-8 | SaaS tier — bundled API access as convenience, not requirement | P2 |

### 4. Note System (Built-in)

| ID | Requirement | Priority |
|---|---|---|
| NOTE-1 | Every command block has an associated note (inline markdown) | P1 |
| NOTE-2 | SQLite persistence — survive restarts, sessions, reboots | P1 |
| NOTE-3 | Search across all notes by content, tags, command | P1 |
| NOTE-4 | Export notes as markdown files | P2 |
| NOTE-5 | SaaS sync — notes sync across devices via Argus Cloud | P2 |
| NOTE-6 | Obsidian link — embed `[[cmd:abc123]]` wikilinks from notes | P1 |

### 5. Obsidian Integration (First-Class)

| ID | Requirement | Priority |
|---|---|---|
| OBS-1 | Vault discovery — auto-detect `.obsidian/` folders in common paths | P1 |
| OBS-2 | File watcher — fsnotify/FSEvents for live vault changes | P1 |
| OBS-3 | Frontmatter + tag + wikilink parsing | P1 |
| OBS-4 | VaultQL — query notes by tag, frontmatter, content, path | P1 |
| OBS-5 | Create/append/write `.md` files in vault | P1 |
| OBS-6 | Command blocks from notes — run code blocks from Obsidian in terminal | P2 |
| OBS-7 | Daily notes auto-link — every session links to the daily note | P2 |
| OBS-8 | SaaS vault sync — bidirectional vault sync via Argus Cloud (included in service) | P2 |

### 6. Editor (Built-in)

| ID | Requirement | Priority |
|---|---|---|
| ED-1 | Syntax highlighting — tree-sitter based (Go, Python, TS, Rust, Java, Ruby, C/C++, SQL, YAML, TOML, JSON, Markdown, Dockerfile, HCL) | P1 |
| ED-2 | Click file in tree → split pane editor | P1 |
| ED-3 | Ctrl+Click → open in system editor (`$EDITOR`) | P1 |
| ED-4 | Multi-tab editing with dirty indicators | P1 |
| ED-5 | Auto-save on pane switch | P1 |
| ED-6 | File search — `⌘+P` fuzzy find across cwd and open tabs | P1 |
| ED-7 | LSP integration — inline diagnostics in gutter, go-to-definition | P2 |
| ED-8 | Minimap — optional classic scrollbar minimap | P2 |
| ED-9 | Theme-aware — inherits terminal color theme | P1 |
| ED-10 | OpenCode integration — "Explain this", "Fix with AI", `Ctrl+I` analyze | P2 |

### 7. File Tree (Built-in)

| ID | Requirement | Priority |
|---|---|---|
| FT-1 | Directory browser — click to expand/collapse | P1 |
| FT-2 | Git status decorations — modified, added, deleted, conflicted | P1 |
| FT-3 | `.gitignore`-aware | P1 |
| FT-4 | Filter/search within tree | P2 |
| FT-5 | Multi-root workspaces | P2 |
| FT-6 | Right-click context menu — rename, delete, move, copy path, reveal in Finder | P1 |

### 8. Git Integration (First-Class)

| ID | Requirement | Priority |
|---|---|---|
| GIT-1 | Inline blame — gutter annotations, async, toggle with `⌘+Shift+B` | P1 |
| GIT-2 | Status bar — current branch, change count, conflict indicator, CI status | P1 |
| GIT-3 | Branch switcher — `⌘+Shift+G`, fetch from remote, show PRs, stash | P1 |
| GIT-4 | Diff viewer — split pane, hunk staging, discard, hover blame | P1 |
| GIT-5 | Commit UI — summary, description, co-author, conventional commits, staged files | P1 |
| GIT-6 | Git log browser — visual branch graph, search, action buttons, commit detail | P1 |
| GIT-7 | Conflict solver — three-pane (ours/base/theirs), keyboard navigation, auto-detect | P2 |
| GIT-8 | Automation triggers — git.commit, branch_created, push, conflict, PR events | P2 |
| GIT-9 | Automation actions — git.commit, push, create_branch, stash, revert | P2 |

### 9. Automation Engine (Built-in)

| ID | Requirement | Priority |
|---|---|---|
| AE-1 | Trigger-condition-action pipeline — runs as background goroutine | P1 |
| AE-2 | Trigger types: cron, terminal.command, terminal.output, terminal.exit_code | P1 |
| AE-3 | System triggers: battery, WiFi, process, file_change, clipboard | P1 |
| AE-4 | Conditions: AND/OR/NOT, chained evaluations | P2 |
| AE-5 | Action types: terminal.exec, terminal.notify, terminal.open_pane, http.post, ssh.exec | P1 |
| AE-6 | Bridge system — Google (OAuth + Gmail/Calendar/Drive), Apple (AppleScript/Shortcuts) | P2 |
| AE-7 | Bridge — System (inotify, cron, battery, WiFi) | P1 |
| AE-8 | Plugin extensibility — plugins register triggers and actions | P2 |
| AE-9 | Rule storage — SQLite, TOML config or `.rule` files | P1 |
| AE-10 | SaaS sync — rules sync across devices via Argus Cloud | P2 |

### 10. SaaS Offload Architecture

| ID | Requirement | Priority |
|---|---|---|
| SAAS-1 | SaaS handles: vault sync, note sync, rule sync, credential sync, AI inference (optional) | P1 |
| SAAS-2 | Docker containers for: db-diag, infra-diag, sandboxed code execution, pinned tool versions | P1 |
| SAAS-3 | Container lifecycle — pull on first use, auto-remove after completion, CPU/mem limits | P1 |
| SAAS-4 | Graceful degradation — no Docker? Run diagnostics from PATH. No internet? Work fully offline. | P1 |

### 11. Plugin System

| ID | Requirement | Priority |
|---|---|---|
| PLUGIN-1 | Plugin API — hooks (OnCommandExecuted, OnKeyEvent, OnRender, OnSessionEvent) | P0 |
| PLUGIN-2 | UI contribution points — toolbar, status bar, sidebar, context menu | P0 |
| PLUGIN-3 | Data provision — ProvideCompletions, ProvideHints | P0 |
| PLUGIN-4 | Automation registration — RegisterTriggers, RegisterActions | P1 |
| PLUGIN-5 | Coding tool registration — RegisterCodingTools | P1 |
| PLUGIN-6 | Plugin types — native (Go plugin), external (JSON-RPC over stdio), WebView | P0 |
| PLUGIN-7 | Plugin manifest — `plugin.toml` with hooks, UI, commands, runtime config | P0 |
| PLUGIN-8 | Plugin store — index, install, update, remove, hot-reload | P2 |
| PLUGIN-9 | Tool dashboards — Grafana, Prometheus, VictoriaMetrics, KubeVM, Terraform, ArgoCD, Popeye | P1 |

### 12. Database Diagnostics (`db-diag` Plugin)

| ID | Requirement | Priority |
|---|---|---|
| DB-1 | Supported databases: PostgreSQL, MySQL, MongoDB, Redis, BigTable, SQLite | P1 |
| DB-2 | Slow query detection — top by latency/calls, EXPLAIN analysis | P1 |
| DB-3 | Index analysis — unused, duplicate, missing indexes | P1 |
| DB-4 | Hotspot detection — BigTable row key skew, MongoDB shard imbalance, Redis hotkeys | P2 |
| DB-5 | Storage bloat — Postgres bloat, MySQL lock contention | P2 |
| DB-6 | AI agent spawn — agent analyzes findings, gives recommendations | P1 |
| DB-7 | Reports — local markdown, Google Doc, S3, clipboard | P1 |
| DB-8 | UI dashboard — compact view with issues, severity, quick actions | P1 |
| DB-9 | Automation — scheduled scans, event triggers, chained actions | P2 |

### 13. Infra Diagnostics (`infra-diag` Plugin)

| ID | Requirement | Priority |
|---|---|---|
| ID-1 | DNS diagnostic — resolution chain, propagation, split-horizon, TTL misconfig | P1 |
| ID-2 | TLS/SSL — certificate chain, expiry, OCSP/CRL, cipher strength, HSTS | P1 |
| ID-3 | Network — ICMP/TCP/HTTP reachability, MTR per-hop latency, MTU, bandwidth estimate | P1 |
| ID-4 | Load balancer — backend health, connection distribution, drain status | P2 |
| ID-5 | Storage — volume latency percentiles, IOPS vs provisioned, snapshot bloat | P2 |
| ID-6 | Service mesh — mTLS rotation, circuit breaker, Envoy resource usage | P2 |
| ID-7 | AI agent spawn — same pattern as db-diag | P1 |
| ID-8 | Reports — same output targets as db-diag | P1 |
| ID-9 | Automation — scheduled audits, event triggers, chained actions | P2 |

### 14. Auth via SaaS (Downstream)

| ID | Requirement | Priority |
|---|---|---|
| SAAS-AUTH-1 | SaaS proxy model — terminal holds SaaS JWT, SaaS handles downstream OAuth | P1 |
| SAAS-AUTH-2 | Google Drive, Gmail, Calendar — all proxied through SaaS | P1 |
| SAAS-AUTH-3 | Local API keys (AI providers) sent directly, not proxied | P0 |

### 15. Meeting Recorder (Built-in)

Records meetings using the laptop's microphone and speakers. Standalone — no Zoom, Teams, Meet, or Slack integration needed. Just press record and get a transcript + summary + task list when it's done.

| ID | Requirement | Priority |
|---|---|---|
| MR-1 | **Record from laptop mic** — capture audio from the built-in microphone or selected input device | P1 |
| MR-2 | **System audio capture** — also capture speaker output (the other side of the call) so both sides are in the transcript | P1 |
| MR-3 | **One-click start/stop** — keyboard shortcut or button in the status bar to begin recording | P1 |
| MR-4 | **Local processing option** — transcribe and summarize using a local model (Ollama/Whisper) for privacy-sensitive meetings | P1 |
| MR-5 | **Cloud processing option** — send audio to SaaS for faster/better transcription (OpenAI Whisper API or similar) | P1 |
| MR-6 | **Transcript** — full verbatim transcript with speaker diarization (who said what) | P1 |
| MR-7 | **AI summary** — auto-generated meeting summary with key discussion points | P1 |
| MR-8 | **Task extraction** — auto-detected action items, decisions, and follow-ups | P1 |
| MR-8a | **Task output** — user chooses: Google Tasks, Apple Reminders (macOS), or Argus Cloud tasks | P1 |
| MR-9 | **Output** — transcript + summary + tasks saved as a note in the terminal (linked to the meeting time/date) | P1 |
| MR-10 | **Export** — save as markdown, Google Doc, or S3 (same report targets as diagnostics) | P2 |
| MR-11 | **Recording indicator** — visible recording indicator in the status bar (red dot) so you never accidentally record | P1 |
| MR-12 | **Privacy** — recordings are local by default. Cloud processing is opt-in. Visual indicator when recording. | P1 |

**How it works**:
1. Press `Ctrl+Shift+R` to start recording (or click the mic icon in the status bar)
2. A red dot appears in the status bar. The meeting happens.
3. Press `Ctrl+Shift+R` again to stop.
4. The terminal spawns an OpenCode agent with the audio file
5. The agent transcribes (local Whisper or cloud Whisper API), summarizes, and extracts tasks
6. Results appear as a note in the terminal, linked to the current daily note in Obsidian
7. Tasks are extracted and written to the user's chosen service: **Google Tasks**, **Apple Reminders** (macOS), or **Argus Cloud tasks**. Configured once in settings.
8. The automation engine can also trigger on extracted tasks (e.g., "when a meeting task is created, add it to the project board")

---

## Build Plan — 6 Phases

### Phase 0: "Skeleton" (1 week)
*Goal: a window on screen with a shell prompt*

- [ ] Go module scaffold (`cmd/argus-terminal/main.go`)
- [ ] GLFW window + OpenGL context (go-gl/glfw v3.3)
- [ ] PTY spawn (creack/pty) + read stdout → display raw bytes
- [ ] Basic keyboard input → write to PTY
- [ ] Config loading (TOML) — font size, window size, shell path

**Deliverable**: A window that runs your shell. Ugly but functional.

### Phase 1: "It's a Terminal" (3 weeks)
*Goal: a usable terminal with notes, OpenCode, and auth*

- [ ] Full ANSI parser — colors, styles, cursor, scroll, hyperlinks, mouse
- [ ] Font atlas (JetBrains Mono + Nerd Font) via golang/freetype
- [ ] GPU render pipeline — screen buffer → vertex shader → fragment shader
- [ ] 60fps with dirty-region optimization
- [ ] Scrollback (50k lines, ring buffer, search)
- [ ] Copy/paste, selection (character/word/line/block)
- [ ] Tabs
- [ ] **Note system** — SQLite-backed, command block attachment, markdown render
- [ ] **OpenCode runtime** — ACP-compatible, read/write/edit/exec/search/git toolset
- [ ] **Auth** — welcome screen, Google OAuth, API key input, Ollama auto-detect
- [ ] Color themes — Catppuccin, Nord, Dracula, Tokyo Night

**Deliverable**: Ship a working terminal with notes, AI coding agent, and auth. ~5,000 lines.

### Phase 2: "Multiplayer + Obsidian + Git + Recorder" (5 weeks)
*Goal: split panes, diff viewer, git tools, Obsidian, automation, editor, meeting recorder*

- [ ] Split panes (horizontal/vertical, drag resize)
- [ ] Command blocks (edit, re-run, output history)
- [ ] Shell integration (preexec/precmd, timing, exit code)
- [ ] Session persistence (restore tabs on restart)
- [ ] **Git** — inline blame, diff viewer, commit UI, branch switcher, log browser
- [ ] **Obsidian** — vault discovery, file watcher, VaultQL, wikilinks
- [ ] **Editor** — tree-sitter syntax highlighting, multi-tab, file tree, LSP diagnostics
- [ ] **Automation engine** — trigger engine, system bridge (cron, file watcher, battery)
- [ ] **Meeting recorder** — mic/speaker capture, local Whisper transcript, AI summary, task extraction

**Deliverable**: A terminal you could use as your daily driver. ~8,500 lines.

### Phase 3: "Connected Terminal" (6 weeks)
*Goal: plugin system, tool dashboards, AI sidebar, automation bridges*

- [ ] **Plugin host** — registry, lifecycle, JSON-RPC IPC, hot-reload
- [ ] **Plugin API** — hooks, UI contributions, completions, triggers, actions, coding tools
- [ ] Plugin store — index, install, update
- [ ] **ArgusKube plugin** — MCP client, AI sidebar, alert feed, semantic kubectl completions
- [ ] **Tool dashboards** — Grafana, Prometheus, Terraform, ArgoCD, Popeye
- [ ] **Automation bridges** — Google (OAuth + Gmail/Calendar/Drive), Apple (AppleScript/Shortcuts)
- [ ] **Docker container orchestration** — pull, run, auto-remove, CPU/mem limits
- [ ] **db-diag plugin** — PostgreSQL, MySQL, MongoDB, Redis diagnostics
- [ ] **infra-diag plugin** — DNS, TLS, network, load balancer diagnostics

**Deliverable**: Full plugin ecosystem with ArgusKube as flagship. ~12,000 lines.

### Phase 4: "Ecosystem & Performance" (ongoing)
*Goal: community growth, mobile companion, collaboration*

- [ ] Community plugin registry (self-hostable)
- [ ] Plugin SDK + docs + examples
- [ ] WebView plugin support
- [ ] Collaborative sessions (share panes)
- [ ] Mobile companion app
- [ ] Native packaging (DMG, .deb, .msi, Homebrew)
- [ ] CI/GitHub webhook integration via SaaS
- [ ] SaaS vault sync (bidirectional, conflict resolution)
- [ ] SaaS shared automations across teams

### Phase 5: "Polish & Scale"
*Goal: production quality, performance tuning, wide adoption*

- [ ] Accessibility (screen reader, high contrast, keyboard nav)
- [ ] Internationalization (UTF-8 everywhere, RTL support, IME input)
- [ ] Performance at 4K/120fps / 8K
- [ ] GPU compute for terminal effects (transparency, blur, shader animations)
- [ ] Crash reporting + telemetry (opt-in)
- [ ] Auto-update mechanism

---

## Key Dependencies

| Package | Phase | Purpose |
|---|---|---|
| `github.com/go-gl/gl/v4.1-core/gl` | 0 | OpenGL 4.1 bindings |
| `github.com/go-gl/glfw/v3.3/glfw` | 0 | GLFW window + input |
| `github.com/golang/freetype` | 1 | Font rasterization → atlas |
| `github.com/creack/pty` | 0 | PTY allocation |
| `github.com/rivo/uniseg` | 1 | Unicode grapheme clusters |
| `github.com/mattn/go-sqlite3` | 1 | SQLite for notes, auth, config |
| `github.com/sourcegraph/jsonrpc2` | 3 | Plugin IPC |
| `github.com/fsnotify/fsnotify` | 2 | File watcher (Obsidian, system) |
| `github.com/smacker/go-tree-sitter` | 2 | Syntax highlighting |
| `github.com/BurntSushi/toml` | 0 | TOML config |

## Project Structure

```
argus-terminal/
├── cmd/argus-terminal/
│   └── main.go
├── internal/
│   ├── pty/                 # PTY manager
│   ├── ansi/                # ANSI parser
│   ├── screen/              # Screen buffer + scrollback
│   ├── render/              # GPU rendering (OpenGL)
│   ├── input/               # Key mapping + input events
│   ├── layout/              # Tabs, panes, splits
│   ├── config/              # TOML config
│   ├── auth/                # OAuth, API keys, credential store
│   ├── opencode/            # AI coding runtime (ACP)
│   ├── notes/               # Built-in note system
│   ├── obsidian/            # Obsidian vault integration
│   ├── editor/              # Code editor (tree-sitter, LSP)
│   ├── filetree/            # File tree browser
│   ├── git/                 # Git integration (blame, diff, commit, log)
│   ├── automate/            # Automation engine
│   │   ├── engine.go
│   │   ├── bridges.go
│   │   ├── rule.go
│   │   ├── store.go
│   │   ├── bridge_google.go
│   │   ├── bridge_apple.go
│   │   └── bridge_system.go
│   └── plugin/              # Plugin host
│       ├── host.go
│       └── registry.go
├── plugins/
│   └── argus-kube/          # Flagship first-party plugin
├── plugin/api/
│   ├── hooks.go
│   ├── ui.go
│   ├── completions.go
│   ├── storage.go
│   └── automate.go
├── go.mod
├── shaders/
│   ├── vert.glsl
│   └── frag.glsl
└── Makefile
```

---

## Success Criteria

| Milestone | Metric |
|---|---|
| Phase 0 done | Window with shell prompt, OpenGL clear |
| Phase 1 done | Daily-drivable terminal, notes, OpenCode, auth |
| Phase 2 done | Full git + Obsidian + editor + split panes |
| Phase 3 done | Plugin store with ArgusKube, db-diag, infra-diag |
| 10k installs | Plugin store with 50+ community plugins |
| 100k installs | SaaS tier profitable, 100+ plugins |
