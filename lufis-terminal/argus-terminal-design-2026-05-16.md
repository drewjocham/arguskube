# Terminal Design — Core Philosophy

**Product**: A GPU-accelerated terminal for software engineers, built in Go, with a first-class plugin/addon ecosystem. Think Warp's polish + VSCode's extension model + neovim's extensibility — but for the terminal.

**Not "a K8s terminal."** It's a great terminal. One of the thing it does great is K8s, because the first-party plugin (ArgusKube) is deeply integrated. But the platform is for any developer workflow.

---

## 1. The Big Idea

Most terminals are dumb pipes — they draw whatever the PTY sends. Warp proved you can build a *smart* terminal that understands structure (command blocks, output parsing, AI). But Warp is a *product*, not a *platform*.

This terminal is a **platform**. The core is small and fast. Everything else is a plugin.

### Built-in vs Plugin

| Feature | Status | Reason |
|---|---|---|
| Note taking | 🏠 **Built-in** | Attached to command blocks, survives restarts, queriable |
| ArgusKube | ⭐ First-party plugin | Deep K8s integration, best-in-class |
| SaaS sync | ⭐ First-party plugin | Cross-device vault, note, rule & credential sync |
| Obsidian | 🔗 **First-class integration** | Vault reader/writer, link notes to commands, local markdown files |
| Meeting recorder | 🏠 **Built-in** | Laptop mic + speakers. Standalone — no Zoom/Teams integration. Transcript, summary, tasks |
| SaaS licensing auth | 🏠 **Required** | SaaS login required to use the terminal. Free tier + Pro tier. 7-day offline grace period |
| OpenCode (coding agent) | 🏠 **Built-in** | ACP-compatible AI coding runtime — spawn, edit, review code in-pane |
| Editor (syntax highlighting) | 🏠 **Built-in** | Click file in sidebar → split pane with the file. Optionally open in system editor |
| Git integration | 🔗 **First-class** | Inline blame, branch switcher, PR status, diff viewer, commit UI — not a plugin |
| Every AI provider | 🏠 **Built-in, no lock-in** | BYO key — OpenAI, Anthropic, DeepSeek, Ollama, any OpenAI-compatible endpoint. SaaS tier offers bundled access as convenience, never a requirement |
| Everything else | 📦 Plugin ecosystem | Docker, DB client, themes, etc. |

### Platform vs Product

```
│ Other Terminals           │ This Terminal              │
├───────────────────────────┼────────────────────────────┤
│ Warp: "our AI, our way"  │ BYO AI provider             │
│ iTerm2: scripts only      │ Plugin API (gRPC/JSON-RPC) │
│ VSCode terminal: embedded │ First-class terminal APP   │
│ Hyper: JS plugins, slow   │ Go plugins, fast           │
```

---

## 2. Terminal Architecture (Full Stack)

```
┌─────────────────────────────────────────────────────────┐
│                   Terminal Core                         │
│  PTY / Render / Input / Scrollback / ANSI / Clipboard    │
│  (tiny, fast, ~15 files)                                 │
├─────────────────────────────────────────────────────────┤
│                   Automation Engine                     │
│  ┌─────────────────────────────────────────────────┐   │
│  │  Trigger → Condition → Action pipeline            │   │
│  │  (IFTTT for your local machine)                   │   │
│  │                                                   │   │
│  │  ┌───────────────────────────────────────────┐   │   │
│  │  │ Triggers                           │   │   │
│  │  │   Terminal Events                            │   │
│  │  │   • Command executed (pattern match)        │   │
│  │  │   • Output contains (regex/grep)            │   │
│  │  │   • Exit code non-zero                      │   │
│  │  │   • Session idle / resume                   │   │
│  │  │                                              │   │
│  │  │   Apple Ecosystem                            │   │
│  │  │   • Calendar event starts/ends               │   │
│  │  │   • Mail receives from specific sender       │   │
│  │  │   • Reminder fires                           │   │
│  │  │   • Focus mode changes                       │   │
│  │  │   • Location changes (geofence)              │   │
│  │  │                                              │   │
│  │  │   Google Ecosystem                           │   │
│  │  │   • Gmail: new email matching filter          │   │
│  │  │   • Calendar: event starts/ends              │   │
│  │  │   • Drive: file added/changed in folder      │   │
│  │  │   • Keep: note updated                       │   │
│  │  │   • Photos: new image with specific tag      │   │
│  │  │                                              │   │
│  │  │   System                                     │   │
│  │  │   • Battery level threshold                  │   │
│  │  │   • WiFi network joined/lost                 │   │
│  │  │   • Process starts/exits                     │   │
│  │  │   • File changes (inotify/fsevents)          │   │
│  │  │   • Time / cron schedule                     │   │
│  │  │   • Clipboard changes                        │   │
│  │  └───────────────────────────────────────────┘   │
│  │                                                   │
│  │  ┌───────────────────────────────────────────┐   │
│  │  │ Actions                             │   │
│  │  │   Terminal                                    │   │
│  │  │   • Execute command in pane                   │   │
│  │  │   • Open new tab/pane                         │   │
│  │  │   • Show notification                         │   │
│  │  │   • Split and run workflow                    │   │
│  │  │                                              │   │
│  │  │   Apple                                       │
│  │  │   • Send via iMessage (Shortcuts bridge)      │   │
│  │  │   • Create reminder / calendar event          │   │
│  │  │   • Send mail (AppleScript/Mail.app)          │   │
│  │  │   • Update Apple Notes                        │   │
│  │  │   • Trigger Shortcut                          │   │
│  │  │                                              │   │
│  │  │   Google                                      │   │
│  │  │   • Send Gmail                                │   │
│  │  │   • Create Calendar event                     │   │
│  │  │   • Append to Google Sheets                   │   │
│  │  │   • Create Google Doc                         │   │
│  │  │   • Update Drive file                         │   │
│  │  │                                              │   │
│  │  │   External                                    │   │
│  │  │   • Webhook (HTTP POST)                       │   │
│  │  │   • SSH to remote and run                     │   │
│  │  │   • Slack / Discord message                   │   │
│  │  │   • Push notification to phone                │   │
│  │  └───────────────────────────────────────────┘   │
│  └─────────────────────────────────────────────────┘   │
├─────────────────────────────────────────────────────────┤
│                   Plugin Host                            │
│  Plugin registry / Lifecycle / Config / IPC              │
│                                                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────────┐            │
│  │  Plugin  │ │  Plugin  │ │   Automation  │            │
│  │ Manager  │ │  Store   │ │  Triggers     │            │
│  └──────────┘ └──────────┘ │  & Actions    │            │
│                             │ (plugins can   │            │
│                             │  register new  │            │
│                             │  triggers &    │            │
│                             │  actions)      │            │
│                             └──────────────┘            │
├─────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────┐   │
│  │                 Plugins                          │   │
│  │                                                  │   │
│  │  ⭐ argus-kube    (first-party, built-in)        │   │
│  │  🔧 git-worktree  (inline git blame)             │   │
│  │  📦 docker        (container management)         │   │
│  │  🤖 ai-companion  (AI chat + command gen)        │   │
│  │  📊 resource-mon  (CPU/mem/disk in status bar)   │   │
│  │  ...  (community plugins via plugin store)       │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### Plugin Types

| Type | Runtime | Use Case |
|---|---|---|
| **Native (Go)** | In-process plugin ~ `plugin.Open()` | High-performance, tight integration (e.g., terminal protocol extensions) |
| **External Process** | gRPC / JSON-RPC over stdio | Language-agnostic, safe isolation (most plugins) |
| **WebView** | Embedded HTML/JS in sidebar | UI-heavy plugins (dashboards, charts, chat) |
| **Shell Hook** | Shell script injected via precmd/preexec | Simple integrations (git info, venv, nvm) |

**Design choice**: External process plugins (JSON-RPC) are the default path. They're safe (crash the plugin, not the terminal), language-agnostic (write in Python, JS, Rust, Go), and easy to develop.

Native Go plugins are reserved for performance-critical cases: render hooks, custom input handling, protocol extensions.

## 3. Automation Engine (IFTTT for Local + Cloud)

This is not a plugin. It's a core runtime — a trigger-condition-action pipeline that runs as a daemon inside the terminal process. It can fire on terminal events, system events, or poll Google/Apple APIs.

### Architecture

```go
package automate

// Engine runs the automation pipeline.
// Loaded on terminal start, runs in background goroutine.
type Engine struct {
    rules   []Rule
    store   *RuleStore  // SQLite-backed
    bridges []Bridge    // Google, Apple, System
}

type Rule struct {
    ID         string
    Name       string
    Enabled    bool
    Trigger    TriggerConfig
    Conditions []Condition
    Actions    []Action
}

type TriggerConfig struct {
    Kind    string // "cron", "terminal.command", "terminal.output",
                   // "terminal.exit_code", "gmail.new_email",
                   // "calendar.event_start", "calendar.event_end",
                   // "drive.file_change", "apple.reminder",
                   // "apple.focus", "system.battery",
                   // "system.wifi", "system.file_change",
                   // "webhook.incoming"
    Params  map[string]any
}

type Action struct {
    Kind     string // "terminal.exec", "terminal.notify",
                    // "terminal.open_pane",
                    // "gmail.send", "calendar.create",
                    // "sheets.append", "drive.write",
                    // "apple.shortcut", "apple.reminder.create",
                    // "slack.message", "discord.webhook",
                    // "http.post", "ssh.exec"
    Params   map[string]any
    Timeout  time.Duration
    OnError  ErrorMode // "ignore", "notify", "stop", "retry"
}
```

### Bridge System

Each ecosystem (Google, Apple, System) is a **Bridge** — a standardized interface for triggers and actions.

```go
type Bridge interface {
    Name() string                        // "google", "apple", "system"
    Triggers() []TriggerDef              // what triggers this bridge provides
    Actions() []ActionDef                // what actions this bridge provides
    Start(ctx context.Context) error     // start polling/webhook listener
    Stop() error
}
```

### Google Bridge

Uses the workspace encrypted connections pattern from arguskube (`internal/workspace/`):
- **OAuth 2.0** — stored encrypted in local SQLite (same as KubeWatcher desktop)
- **Push notifications** via PubSub or polling (5-min poll as fallback)
- **Scopes**: Gmail, Calendar, Drive, Sheets, Docs, Keep, Tasks

Triggers:
- `gmail.new_email` — filter by sender, subject, label
- `gmail.email_matches` — regex on body/subject
- `calendar.event_start` / `calendar.event_end` — 5 min before or exact
- `drive.file_changed` — by folder or file pattern
- `drive.new_file` — file created in watched folder
- `keep.note_updated` — specific note changed

Actions:
- `gmail.send` — send email (template or custom)
- `calendar.create` — create event with details
- `sheets.append` — append row to sheet
- `drive.write` — write/update file
- `docs.create` — create Google Doc from template
- `tasks.create` — create task

### Apple Bridge

Leverages:
- **AppleScript** / `osascript` (macOS only) — for Mail, Calendar, Reminders, Notes, Messages
- **Shortcuts** — trigger `.shortcut` files via `shortcuts run`
- **System Events** via `.zshenv` hooks and launchd
- **JXA** (JavaScript for Automation) as fallback
- **CloudKit** — for the iCloud plugin to register its own triggers/actions

Triggers:
- `apple.calendar.event_start` / `event_end`
- `apple.mail.new_message` — from specific sender/subject
- `apple.reminder.fires`
- `apple.focus.changed` — focus mode entered/exited
- `apple.location.arrived` / `departed` — geofence (via CoreLocation CLI)

Actions:
- `apple.imessage.send` — to contact or group (via Messages AppleScript)
- `apple.mail.send`
- `apple.reminder.create`
- `apple.calendar.create`
- `apple.note.create` / `note.append`
- `apple.shortcut.run` — run any Shortcut
- `apple.notification.send` — native macOS notification

### System Bridge

No cloud dependency. Runs locally:

Triggers:
- `system.battery` — level below/above threshold
- `system.wifi` — SSID joined/lost
- `system.process` — process started/exited by name
- `system.file_change` — inotify/FSEvents on path
- `system.clipboard` — clipboard content matches pattern
- `system.cron` — cron schedule (every syntax)
- `system.idle` — terminal idle for N minutes

Actions:
- `terminal.exec` — run command in current pane or new pane
- `terminal.notify` — in-terminal notification
- `terminal.open_pane` / `open_tab` — with specific session
- `terminal.ssh` — SSH to remote and exec
- `http.post` / `http.get` — webhook to any URL
- `slack.webhook` / `discord.webhook` — send to channel

### Example Automations

```toml
# An automation is just a TOML block in config or installed via a .rule file:

[[automation]]
name = "deploy-failed-notify"
enabled = true

[automation.trigger]
kind = "terminal.output"
params = { pane = "*", pattern = "FAILED|Error|exit code 1" }

[automation.conditions]
and = [
    { kind = "terminal.command", params = { pattern = "kubectl apply|helm upgrade" } },
    { kind = "system.battery", params = { below = 20 } },
]

[[automation.actions]]
kind = "apple.notification.send"
params = { title = "Deploy Failed", body = "{command} exited with error in {pane}" }

[[automation.actions]]
kind = "terminal.split"
params = { command = "kubectl describe pod -l app={extracted.app}", dir = "bottom" }
```

```toml
[[automation]]
name = "standup-prep"
enabled = true

[automation.trigger]
kind = "calendar.event_start"
params = { calendar = "Work", title_matches = "standup|sync|daily" }

[[automation.actions]]
kind = "terminal.open_tab"
params = {
    splits = [
        { command = "cd ~/project && git log --oneline -10" },
        { command = "kubectl get pods -A --sort-by='{.status.phase}'" },
        { command = "gh pr list --author @me" },
    ]
}

[[automation.actions]]
kind = "apple.notification.send"
params = { title = "Standup time! ☕", body = "Prep boards are ready." }
```

```toml
[[automation]]
name = "deploy-to-prod-gate"
enabled = true

[automation.trigger]
kind = "terminal.command"
params = { pattern = "kubectl apply -f .*production" }

[[automation.actions]]
kind = "terminal.notify"
params = { severity = "warning", message = "You're about to deploy to PRODUCTION. Continue? (y/N)" }

[[automation.actions]]
kind = "apple.imessage.send"
params = { contact = "team-k8s", message = "{user} is deploying to PRODUCTION: {command}" }

[[automation.actions]]
kind = "sheets.append"
params = { sheet_id = "1abc...", row = ["{timestamp}", "{user}", "{command}"] }
```

### Plugin Extensibility

Plugins can register **new triggers and actions** with the automation engine via the plugin API, just like they register sidebar panels or completions:

```go
func (p *MyPlugin) RegisterTriggers() []automate.TriggerDef {
    return []automate.TriggerDef{
        {
            Kind:        "my-plugin.custom_event",
            Description: "Fires when my custom thing happens",
            Params: []ParamDef{
                { Name: "threshold", Type: "number", Required: true },
            },
        },
    }
}

func (p *MyPlugin) RegisterActions() []automate.ActionDef {
    return []automate.ActionDef{
        {
            Kind:        "my-plugin.do_thing",
            Description: "Does my thing",
            Params: []ParamDef{
                { Name: "target", Type: "string", Required: true },
            },
        },
    }
}
```

### Storage

Everything is local-first:
- Rules stored in SQLite (`~/.argus-terminal/automations.db`)
- Credentials encrypted via AES-GCM (same pattern as `internal/workspace/`)
- OAuth tokens stored encrypted, auto-refreshed
- No cloud dependency. The rule engine runs in-process.

### SaaS Sync Layer

The **argus-cloud** first-party plugin connects the terminal to your SaaS platform. It syncs:
- **Obsidian vaults** — bidirectional sync of `.md` files across devices
- **Terminal notes** — command block notes, linked to vault notes
- **Automation rules** — triggers, conditions, actions
- **Encrypted credentials** — OAuth tokens, API keys (end-to-end encrypted)
- **Config & themes** — terminal settings across machines

Included in the service tier. No per-plugin sync subscriptions. The plugin itself is a thin client — the SaaS backend handles conflict resolution, versioning, and access control.

---

## 4. OpenCode — Built-in AI Coding Runtime

The terminal is also an ACP-compatible AI coding agent runtime. You can use it as a standard terminal, or you can spawn an AI coding session in any pane — it edits files, runs commands, reviews code, and commits changes. Same as Claude Code, Cursor Terminal, or Gemini CLI, but native.

### Architecture

```go
package opencode

// ACP (Agent Communication Protocol) runtime built into the terminal.
// Spawns in any pane — works alongside your shell sessions.

type Runtime struct {
    session  *Session
    toolset  ToolSet      // read, write, exec, search, grep, git
    model    ModelClient  // BYO model (OpenAI, Anthropic, DeepSeek, Ollama)
    workdir  string       // project directory
    history  []Turn       // conversation turns
}

type Session struct {
    ID       string
    PaneID   PaneID       // which pane it's running in
    Status   SessionStatus  // idle, thinking, executing, waiting, done
    Model    string
    Messages []Message
}

// Standard ACP tool set
type ToolSet interface {
    Read(path string) (string, error)
    Write(path string, content string) error
    Edit(path string, old string, new string) error
    Exec(command string) (ExecResult, error)
    Search(pattern string) ([]SearchMatch, error)
    Grep(pattern string) ([]GrepMatch, error)
    Git(args ...string) (string, error)
    Glob(pattern string) ([]string, error)
    Diff(path string) (string, error)
}

// LSP integration for smarter edits
type LSPClient struct {
    language string  // go, python, typescript, rust
    symbols  []Symbol
    diags    []Diagnostic
}
```

### How it works

| Feature | Description |
|---|---|
| **New coding pane** | `Ctrl+Shift+O` opens a new pane running an AI coding session |
| **Inline prompts** | Type natural language in the prompt bar, the coding agent executes |
| **Multi-pane coding** | Split panes — one with the agent, one with the shell, one with git log |
| **Review mode** | The agent can open diffs in a read-only pane for review before applying |
| **BYO model** | Connect any API key — DeepSeek, OpenAI, Anthropic, Ollama local |
| **Workspace awareness** | The agent sees your open panes, recent commands, git state |
| **Plugin hooks** | Plugins can register custom tools for the coding agent |

### OpenCode vs Terminal Mode

```
┌─────────────────────────────────────────────────────┐
│  Terminal Pane (normal shell, zsh)                   │
│  $ kubectl get pods -n prod                          │
│  $ git log --oneline -5                              │
│  $ npm run test                                       │
│  $ _                                                  │
├─────────────────────────────────────────────────────┤
│  OpenCode Pane (coding agent)                        │
│  ┌──────────────────────────────────────────────────┐│
│  │  🤖 opencode $ fix the flaky test in test/api/   ││
│  │                                                  ││
│  │  Reading test/api/auth_test.go...                ││
│  │  Found race condition in TestLoginConcurrent     ││
│  │  Adding sync.Mutex to auth handler...             ││
│  │  Running `go test ./test/api/ -run TestLogin...`  ││
│  │  ✅ PASS (1.234s)                                ││
│  │  ┌──────────────────────────────────────┐        ││
│  │  │ [Accept diff] [Reject] [Edit again]  │        ││
│  │  └──────────────────────────────────────┘        ││
│  └──────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────┘
```

### Plugin Extensibility

Plugins can register custom tools that the OpenCode runtime can call:

```go
func (p *ArgusKubePlugin) RegisterCodingTools() []opencode.ToolDef {
    return []opencode.ToolDef{
        {
            Name:        "kubectl_exec",
            Description: "Run kubectl commands against the current cluster",
            Parameters:  map[string]Param{
                "command": { Type: "string", Required: true },
                "namespace": { Type: "string" },
            },
            Execute: func(params map[string]any) (string, error) {
                // run kubectl via the MCP client
                return p.mcpClient.Exec(params)
            },
        },
        {
            Name:        "cluster_health",
            Description: "Get current cluster health summary",
            Execute: func(params map[string]any) (string, error) {
                return p.mcpClient.ClusterHealth()
            },
        },
    }
}
```

### No AI Lock-In

The terminal doesn't ship with a model. It supports every major provider with the same feature set:

| Provider | Type | Setup |
|---|---|---|
| **OpenAI** | API key | Paste key → validated → works with OpenCode, AI chat, diagnostics |
| **Anthropic (Claude)** | API key | Same flow — no special treatment, no special limitations |
| **DeepSeek** | API key | Same flow |
| **Ollama** | Local | Auto-detected — zero config, fully offline, no data leaves your machine |
| **OpenAI-compatible** | Any endpoint | Generic adapter — works with vLLM, Together, Groq, Fireworks, Perplexity, etc. |
| **SaaS tier** | Bundled | Convenience option, not a requirement. Pay for access, not for the right to use your own key |

**No provider gets preferential treatment.** The AI coding agent, diagnostics agents, chat sidebar, and every other AI feature works identically regardless of which provider you use. The only difference is latency and capability (some models are faster, some are smarter — that's a model decision, not a platform decision).

**The SaaS tier** includes bundled API access as a convenience — you pay for tokens, not for the terminal itself. But there's zero feature gating based on provider choice. Bring Ollama and work fully offline forever? Everything works.

---

## 5. Built-in Note System + Obsidian Integration

### Built-in Notes

Every command block can have notes attached — inline annotations, explanations, context. Not a plugin. Part of the terminal core.

```go
package notes

type Note struct {
    ID        string    // unique ID, linked to command block
    SessionID string    // which terminal session
    Command   string    // the command text
    ExitCode  int
    Timestamp time.Time
    Body      string    // markdown
    Tags      []string
    
    // Links to external references
    ObsidianLink *ObsidianNoteLink  // optional
}

type NoteStore interface {
    List(filter NoteFilter) ([]Note, error)
    Get(commandID string) (*Note, error)
    Save(note Note) error
    Delete(id string) error
    Search(query string) ([]Note, error)
    Export()     // export all notes as markdown files
}
```

**SaaS sync**: Notes DB syncs across devices via your SaaS platform (included in the service). Same mechanism syncs rules, credentials, and Obsidian vaults. CloudKit sync is available separately for terminal state only (notes, rules, creds).

### Obsidian Integration (First-Class)

Not a plugin. The terminal has a native understanding of Obsidian vaults:

```go
package obsidian

type Vault struct {
    Path string               // local filesystem path
    Name string
    Notes map[string]NoteFile  // .md files in vault
}

type NoteFile struct {
    Path     string
    Title    string    // first H1 or filename
    Content  string    // raw markdown
    Tags     []string  // parsed #tags
    Links    []string  // [[wikilinks]]
    Frontmatter map[string]any
    Modified time.Time
}

type Client struct {
    vaults []Vault
    watcher *fsnotify.Watcher  // live sync
}

func (c *Client) Query(ql VaultQL) ([]NoteFile, error)
func (c *Client) Create(path string, content string) error
func (c *Client) Append(noteRef string, content string) error
func (c *Client) LinkCommand(noteRef string, commandID string) *ObsidianNoteLink
```

#### What it does

- **Open vaults** — detects `.obsidian/` folders, no config needed
- **Inline command linking** — `[[cmd:abc123]]` wikilink in Obsidian renders a link back to the terminal command output
- **Export notes to vault** — every terminal note can be written as a `.md` file in the vault
- **Live preview** — Obsidian markdown renders in the terminal's note panel (same renderer, no external viewer)
- **Tag integration** — `#terminal` tags in Obsidian notes are indexed and searchable from the terminal command palette
- **VaultQL** — simple query language to find notes by tag, frontmatter, or content pattern:
  ```
  tag:#k8s AND modified:>2026-05-01 AND path:ops/
  ```
- **Command blocks from notes** — write a code block in Obsidian, hit a button in the terminal to run it
- **Daily notes auto-link** — every terminal session has a command block linking back to the daily note in the vault
- **SaaS vault sync** — with the argus-cloud plugin, vaults sync across devices via your SaaS platform. Conflict resolution, version history, access control. Included in the service.

#### Why first-class, not a plugin

Obsidian vaults are just folders of markdown files. The terminal doesn't need an API — it reads and writes `.md` files directly. The integration is:
1. Detect `.obsidian/` in common paths
2. Watch for file changes (fsnotify/FSEvents)
3. Parse frontmatter + tags + wikilinks
4. Provide a query API
5. Provide a create/append/write API

This is ~500 lines of Go, not a plugin surface. Making it a plugin would mean every plugin needs its own markdown parser and file watcher. It's a core utility, same as the note system.

---

## 6. Authentication — Two Clicks, Done

### SaaS Login Required (Terminal Licensing)

The terminal requires a SaaS account to use. It's a commercial product:

| Tier | What you get |
|---|---|
| **Free** | Local terminal + limited automations + 1 device |
| **Pro** | Everything + sync + AI credits + unlimited devices |
| **Team** | Everything + shared automations + team management |

**Offline grace period**: 7 days of cached token. After that, re-auth required.

### The Two-Click Flow

**Click 1:** Terminal launches. You see:

```
┌──────────────────────────────────────────────────────┐
│  Welcome to Argus Terminal                            │
│                                                        │
│  Sign in to get started:                              │
│                                                        │
│  ┌──────────────────────────────────────────────────┐ │
│  │  ☁️  Sign in with Google                          │ │
│  │  (creates or connects your Argus Cloud account)  │ │
│  │                                                  │ │
│  │  ✉️  Email me a magic link                        │ │
│  └──────────────────────────────────────────────────┘ │
│                                                        │
│  [▶ Sign in with Google]  [▶ Email magic link]        │
└──────────────────────────────────────────────────────┘
```

**Click 2:** OAuth popup in your browser. Authorize. Popup closes. Terminal is authenticated and ready.

Once signed in, the terminal connects to all downstream services through your SaaS account.

That's it. From there:
- SaaS is authenticated (the terminal gets a JWT + refresh token)
- The SaaS handles downstream auth: "Connect your Google account" in SaaS settings → now Gmail/Calendar/Drive triggers work
- Google OAuth from the terminal only covers what the terminal needs directly

### Ecosystem Auth (Discoverable, Not Required)

After the initial SaaS connection, the terminal discovers what else is available:

```
┌──────────────────────────────────────────────────────┐
│  Services  [☁️ Connected to Argus Cloud]             │
│                                                        │
│  These services add features. Nothing required:       │
│                                                        │
│  Google      ○ Connect → Gmail, Calendar, Drive       │
│  iCloud      ○ Connect → cross-device sync, Calendar  │
│  GitHub      ○ Connect → PRs, CI status, issues      │
│  GitLab      ○ Connect → same as GitHub               │
│  OpenAI      ○ API key → AI coding agent              │
│  Anthropic   ○ API key → AI coding agent              │
│  DeepSeek    ○ API key → AI coding agent              │
│  Ollama      ○ Detected at localhost:11434 ✓          │
│                                                        │
│  Each connection is one click. No config files.        │
│  [▶ Connect Google] [▶ Connect GitHub] [▶ Add API key] │
└──────────────────────────────────────────────────────┘
```

### How It Works

```go
package auth

type Service string

const (
    ServiceArgusCloud  Service = "argus-cloud"
    ServiceGoogle      Service = "google"
    ServiceApple       Service = "apple-icloud"
    ServiceGitHub      Service = "github"
    ServiceGitLab      Service = "gitlab"
    ServiceOpenAI      Service = "openai"
    ServiceAnthropic   Service = "anthropic"
    ServiceDeepSeek    Service = "deepseek"
)

type Credential struct {
    Service   Service
    Method    AuthMethod   // "oauth2", "api-key", "token", "detected"
    Label     string       // user-friendly name
    Status    AuthStatus   // connected, expired, disconnected, failed
    ExpiresAt time.Time
    
    // Payload (encrypted at rest)
    Token     []byte       // encrypted JWT or OAuth token
    Refresh   []byte       // encrypted refresh token
    APIKey    []byte       // encrypted API key (for key-based services)
}

type Manager struct {
    store    *CredentialStore  // SQLite, encrypted at rest (AES-GCM)
    sessions map[Service]*Session
    
    // Callbacks
    OnConnected    func(service Service)
    OnDisconnected func(service Service, reason error)
}

func (m *Manager) Connect(service Service) error {
    switch service {
    case ServiceGoogle:
        // 1. Open browser to Google OAuth URL
        // 2. Start local HTTP server on :18790
        // 3. Google redirects to localhost:18790/callback
        // 4. Exchange code for token
        // 5. Store encrypted token
        // 6. Close browser tab (if possible via JS, else tell user)
        // 7. Return success
    case ServiceArgusCloud:
        // Same pattern, but against your SaaS endpoint
        // Supports Google OAuth or email+magic-link
    case ServiceOpenAI:
        // 1. Show a single input field for API key
        // 2. Validate by calling models.list
        // 3. Store encrypted
    case ServiceOllama:
        // 1. Check localhost:11434
        // 2. If available, auto-detect. No user action needed.
    }
}

func (m *Manager) Disconnect(service Service) error
func (m *Manager) Refresh(service Service) error      // auto-refresh OAuth tokens
func (m *Manager) GetClient(service Service) (Client, error)  // returns an authenticated client
```

### Token Storage

Same pattern as the existing `internal/workspace/` in ArgusKube:
- **SQLite** database at `~/.argus-terminal/auth.db`
- **AES-256-GCM** encryption at rest
- **Key derived** from machine-specific seed + user password (optional)
- **No plaintext tokens** ever written to disk
- **Keychain integration** on macOS (`security add-generic-password`) as secondary store

### Auto-Detected Services

Some services don't need auth — the terminal detects them:

| Service | Detection |
|---|---|
| Ollama | `GET http://localhost:11434/api/tags` |
| Docker | `docker info` in PATH |
| Podman | `podman info` in PATH |
| kubectl | `kubectl version --client` + `~/.kube/config` |
| Git | `git config --global user.name` |
| AWS | `~/.aws/credentials` |
| GCP | `~/.config/gcloud/application_default_credentials.json` |

These show up as "auto-detected ✓" in the services panel. No clicks needed.

### SaaS Downstream Auth

Once connected to your SaaS, the SaaS handles its own auth flows. The terminal delegates:

```
Terminal connects to Argus Cloud (OAuth)
  → Argus Cloud asks: "Want to connect Google Drive?"
    → User clicks yes in the SaaS web UI
      → SaaS gets Google OAuth token (server-side, never touches the terminal)
        → Terminal uses SaaS API to read/write Drive files
           (terminal never needs its own Google token for Drive)
```

The terminal only holds directly-used credentials:
- Your SaaS JWT
- Local AI provider API keys (sent directly to the API, not proxied)
- Local git/kubernetes/docker config (read from filesystem, not a credential store)

Everything else is proxied through your SaaS.

### No Two-Factor Friction

- **OAuth first** — always the preferred path. One redirect, done.
- **API keys second** — single input field, validate before saving
- **Magic links** — for the SaaS initial auth (email → click link → done)
- **No TOTP setup inside the terminal** — delegate to browser OAuth which already handles 2FA

### Summary

| Step | Clicks | What happens |
|---|---|---|
| First launch | 1 | Big button: "Continue with Google" or "Use Local Mode" |
| Google OAuth | 1 | Browser popup, authorize, closes automatically |
| SaaS connected | 0 | (above step also connects SaaS if using Google) |
| Add Google Drive | 1 | Click "Connect" → browser → done |
| Add GitHub | 1 | Click "Connect" → browser → done |
| Add OpenAI key | 1 | Paste key → auto-validated → done |
| Ollama detected | 0 | Shows up automatically |
| Everything else | 0 | Detected from PATH or config files |

**Max two clicks. Usually one. Often zero.**

---

## 7. Plugin System Architecture

### Tool Dashboards (Built-in Plugin Pattern)

Each DevOps tool (Grafana, Prometheus, VictoriaMetrics, KubeVM, Terraform, etc.) gets a **tool dashboard** — a small, purpose-built UI with the common actions for that tool, rendered in a pane or sidebar panel. These are implemented as plugins using the standard plugin API (sidebar panels, toolbar items, commands), but they're first-party plugins with a consistent design language.

| Tool | Dashboard Includes |
|---|---|
| **Grafana** | Search dashboards, open in browser, export JSON, toggle alert rules, recent queries |
| **Prometheus** | Ad-hoc query bar, recent queries, targets up/down, alert rules, silences |
| **VictoriaMetrics** | PromQL query builder, recent queries, cardinality explorer, tsdb status |
| **KubeVM** | VM list, status, start/stop/restart, VMI console URL, data volume list |
| **Terraform** | State list, plan preview, apply/reject, workspace switcher, output viewer |
| **ArgoCD** | App list, sync status, rollback, diff viewer, resource tree |
| **Popeye** | Run scan, view report, filter by severity, mark as triaged |
| **K9s** | Quick resource browser, describe/logs/exec without leaving the terminal |

**Pattern**: Each dashboard is a plugin that:
1. Registers a **sidebar panel** (`ToolbarItems` + `SidebarPanels`)
2. Registers **commands** (e.g., `grafana.search`, `prometheus.query`)
3. Registers **triggers** for the automation engine (e.g., `prometheus.alert_fires`, `terraform.plan_created`)
4. Registers **actions** for the automation engine (e.g., `grafana.export_dashboard`, `kubewm.restart_vm`)
5. Provides **completions** (`ProvideCompletions`) for the command bar

#### Example: Terraform Dashboard

```
┌─────────────────────────────────────────────┐
│ Terraform  [prod] [staging] [dev]            │
│ ──────────────────────────────────────────── │
│ State: 43 resources, no drift                │
│                                              │
│ Recent commands:                             │
│  • tf plan -out=plan.tfplan  (2m ago)        │
│  • tf apply plan.tfplan      (1h ago)        │
│                                              │
│ [▶ Plan] [▶ Apply] [▶ Destroy] (dry-run)     │
│ [📋 State list] [📂 Outputs] [📊 Drift diff] │
│                                              │
│ ──────────── Automation Rules ───────────── │
│ 🔄 Auto-apply plan on PR merge   [enabled]  │
│ 🔔 Notify on drift detected       [enabled]  │
│ 🛑 Gate production applies        [enabled]  │
└─────────────────────────────────────────────┘
```

#### Example: Prometheus Dashboard

```
┌─────────────────────────────────────────────┐
│ Prometheus  [query]                           │
│ ──────────────────────────────────────────── │
│ > rate(http_requests_total[5m])               │
│ ┌──────────────────────────────────────┐     │
│ │ [chart preview rendered inline]      │     │
│ └──────────────────────────────────────┘     │
│ Recent:                                     │
│  • up{job="kubelet"}                        │
│  • node_memory_MemAvailable_bytes            │
│                                              │
│ Targets: 24 up, 2 down  [🔔 Alert rules: 12]│
│ [🔇 Silence] [📋 TSDB Status] [📊 Rules]     │
└─────────────────────────────────────────────┘
```

### Shortcuts & Scheduled Jobs via Automation

Every tool dashboard action can be:
1. **Triggered manually** via command palette or keyboard shortcut
2. **Scheduled** via the automation engine (cron, every-N, or event-driven)
3. **Chained** into workflows (e.g., run Prometheus query → if result > threshold → notify Slack → open Grafana dashboard)

Examples:

```toml
[[automation]]
name = "morning-cluster-check"
enabled = true

[automation.trigger]
kind = "system.cron"
params = { expr = "0 8 * * 1-5", tz = "Europe/Berlin" }  # weekdays at 8am

[[automation.actions]]
kind = "terminal.open_tab"
params = {
    splits = [
        { command = "prometheus query 'up == 0' --format list" },
        { command = "terraform plan -out=plan.tfplan" },
        { command = "kubectl get pods -A --sort-by='{.status.phase}'" },
    ]
}

[[automation.actions]]
kind = "grafana.open_dashboard"
params = { dashboard = "Cluster Overview", time_range = "24h" }
```

```toml
[[automation]]
name = "auto-drift-detection"
enabled = true

[automation.trigger]
kind = "system.cron"
params = { expr = "*/30 * * * *" }  # every 30 minutes

[[automation.actions]]
kind = "terraform.plan"
params = { dir = "~/infra/prod", format = "json", diff_on_change = true }

[[automation.conditions]]
if = { kind = "terraform.drift_detected" }

[[automation.actions]]
kind = "apple.notification.send"
params = { title = "Terraform drift detected", body = "Run 'terraform apply' in pane 2" }

[[automation.actions]]
kind = "sheets.append"
params = { sheet_id = "1abc...", row = ["{timestamp}", "PROD", "drift", "{diff_summary}"] }
```

### Database Diagnostics (`db-diag`)

A first-party plugin that connects to your databases and diagnoses performance issues. Not a full query browser — focused on finding problems.

#### Supported Databases

| Database | Diagnostic | Details |
|---|---|---|
| **MongoDB** | Slow queries | `currentOp` queries > 100ms, full collection scans, missing indexes |
| | Index analysis | Unused indexes, redundant indexes, covered query suggestions |
| | Hotspots | Write contention on shards, uneven chunk distribution, balancer lag |
| **PostgreSQL** | Slow queries | `pg_stat_activity` + `pg_stat_statements`, top by total_time / mean_time |
| | Index analysis | Unused indexes, duplicate indexes, missing indexes (pg_hint_plan suggestions) |
| | Bloat | Table and index bloat estimation, vacuum stats, dead tuple ratio |
| **MySQL** | Slow queries | `slow_query_log` analysis, `performance_schema` event summary |
| | Index analysis | Unused indexes, cardinality warnings, redundant indexes |
| | Locks | Long-running transactions, lock waits, deadlocks from `information_schema` |
| **Redis** | Slow commands | `SLOWLOG GET 100`, commands by latency percentiles |
| | Memory | Key expiration analysis, memory fragmentation, eviction rate |
| | Hotkeys | `redis-cli --hotkeys`, large key discovery |
| **BigTable** | Hotspots | Uneven row key distribution, node CPU skew, tablet server imbalance |
| | Storage | Compaction backlog, replication lag, storage per table per node |
| **SQLite** | Slow queries | `EXPLAIN QUERY PLAN` on recent queries, full table scans |
| | Index analysis | Missing indexes on JOIN/WHERE columns, unused indexes |

#### Dashboard

```
┌─────────────────────────────────────────────┐
│ Database Diagnostics  [🟢 postgres@prod]     │
│ ──────────────────────────────────────────── │
│ Slow Queries (last 15m)                      │
│ ┌────────────────────────────────────────┐   │
│ │ Query                          Calls  │   │
│ │ SELECT * FROM orders WHERE ...   2,341 │   │
│ │ UPDATE users SET status=...        892 │   │
│ │ DELETE FROM sessions WHERE ...     451 │   │
│ └────────────────────────────────────────┘   │
│ [🔍 Explain] [📊 Index advisor]              │
│                                              │
│ Index Issues                                  │
│ 🟡 idx_orders_user_id — unused (last used 3d)│
│ 🔴 Missing index on orders(ship_date)        │
│ 🟡 Duplicate: idx_users_email = idx_users_ea │
│                                              │
│ Storage Hotspots                              │
│ Node 3: 72% write traffic (skew ⚠️)           │
│ [▶ Run full diagnostic]                       │
└─────────────────────────────────────────────┘
```

#### Agent Spawn

When a diagnostic detects an issue, it spawns an AI agent to investigate:

```
> db-diag analyze --db postgres@prod --issue slow-queries

🤖 Agent: Analyzing slow queries on postgres@prod...
Found 3 queries with mean_time > 500ms:

1. SELECT * FROM orders JOIN users ON ... WHERE status = 'pending'
   - Missing index on orders.status (full table scan)
   - Suggested: CREATE INDEX CONCURRENTLY idx_orders_status ON orders(status)
   - Estimated improvement: 94% reduction in query time

2. UPDATE users SET last_login = NOW() WHERE id = $1
   - Already has index on users.id (covering query, OK)
   - 892 calls/15m is normal for login volume

3. DELETE FROM sessions WHERE expires_at < NOW()
   - Missing index on sessions.expires_at
   - 451 calls/15m × 6.2s = 46 minutes of DB time wasted per hour
   - Suggested: CREATE INDEX CONCURRENTLY idx_sessions_expires ON sessions(expires_at)

Report written to: ~/argus-reports/db-diag-postgres-prod-2026-05-16.md
[📋 Save to Google Docs] [☁️ Save to S3]
```

### Infrastructure Diagnostics (`infra-diag`)

A first-party plugin that diagnoses infrastructure-level issues. When an engineer needs to find where a problem is, they run a diagnostic and get a clear answer — or a narrowed-down set of suspects.

#### Diagnostic Checks

| Category | Check | What it finds |
|---|---|---|
| **DNS** | Resolution chain | Queries public + private DNS, checks latency, DNSSEC, split-horizon consistency |
| | Propagation | `dig +trace` to find stale NS records, TTL misconfigs, missing glue records |
| **TLS/SSL** | Certificate chain | Expiry, intermediate missing, revocation via OCSP/CRL, cipher strength |
| | Handshake | Round trip time, SNI mismatch, ALPN negotiation, HSTS header presence |
| **Network** | Connectivity | ICMP/TCP/HTTP(S) reachability, MTU path discovery, packet loss at each hop |
| | Latency | Per-hop latency via MTR, TCP RTT variance between regions |
| | Bandwidth | iperf-style throughput estimate (no agent required, uses HTTP range requests) |
| **Load Balancer** | Backend health | Stale backends, uneven connection distribution, slow drain, TLS termination errors |
| | Routing | Session persistence, cookie affinity mismatch, cross-zone balancing ratio |
| **Storage** | Volume latency | Read/write latency percentiles, queue depth, IOPS vs provisioned |
| | Space | Filesystem usage by inode/block, snapshots occupying space, bottleneck at PVC level |
| **Service Mesh** | mTLS | Certificate rotation status, SPIFFE identity verification, workload exclusion |
| | Traffic | Retry/ timeout config issues, circuit breaker state, Envoy proxy resource usage |

#### Dashboard

```
┌─────────────────────────────────────────────┐
│ Infra Diagnostics  [🟢 cluster-prod]         │
│ ──────────────────────────────────────────── │
│ ▶ Quick Health Check                           │
│ 🔴 DNS: api.example.com resolution 1.2s       │
│ 🟡 TLS: cert expires in 14 days               │
│ 🟢 Network: all nodes reachable               │
│ 🟢 LB: backends healthy                       │
│ 🟢 Mesh: mTLS rotating OK                     │
│ 🟡 Storage: pvc-xyz iowait elevated (62ms)    │
│                                              │
│ [▶ Full diagnostic] [▶ Network only]          │
│ [▶ DNS+TLS] [▶ Storage] [▶ Custom...]         │
│                                              │
│ History:                                       │
│ ⚪ 2h ago — Full diag: all green               │
│ 🟡 1d ago — DNS timeout on staging endpoint   │
│ 🔴 3d ago — TLS cert expired (resolved)       │
│ [📊 Trends] [💾 Save report]                  │
└─────────────────────────────────────────────┘
```

#### Agent Spawn

Same pattern as db-diag — when a check finds something, spawn an agent:

```
> infra-diag analyze --pattern dns --cluster prod

🤖 Agent: Analyzing DNS chain for cluster-prod...

Issue found: api.example.com resolves slowly (1.2s)

Diagnosis:
  • Route: client → 8.8.8.8 (12ms) → ns1.example.com (340ms) → ns2.example.com (890ms)
  • Root cause: ns2.example.com is not responding from us-east-1 (firewall?)
  • Secondary: TTL is 300 but cached records are fresh (stale cache hits 8.8.8.8)
  • Impact: all API traffic has 400-900ms DNS penalty before connection

Recommendations:
  1. Check ns2.example.com firewall rules for UDP/53 from VPC
  2. Lower TTL to 60 for active failover
  3. Add stub resolver in VPC to reduce public DNS dependency

Report written to: ~/argus-reports/infra-diag-dns-prod-2026-05-16.md
[📋 Save to Google Docs] [☁️ Save to S3]
```

### Report Delivery

Both diagnostics agents support the same output targets:

| Target | Format | Usage |
|---|---|---|
| **Local file** | Markdown | `~/.argus-terminal/reports/` — always saved |
| **Google Doc** | Markdown → Doc | Requires Google OAuth, rendered as formatted Doc with sections and code blocks |
| **S3** | Markdown | Requires S3 config, saved to `s3://bucket/reports/` |
| **Argus Cloud** | Markdown | Included in service. Stored in your SaaS account, shareable by link |
| **Clipboard** | Markdown | Copy to clipboard for pasting into Slack, PR comments, etc. |

Set via command palette or `--save-to google-docs` flag. Default is local + last-used cloud target.

### Automation Integration

Both diagnostics register triggers and actions with the automation engine:

```toml
[[automation]]
name = "hourly-db-scan"
enabled = true

[automation.trigger]
kind = "system.cron"
params = { expr = "0 * * * *" }

[[automation.actions]]
kind = "db-diag.scan"
params = { db = "postgres@prod", checks = ["slow-queries", "index-analysis"], min_severity = "medium" }

[[automation.conditions]]
if = { kind = "db-diag.issue_found", params = { severity = "high" } }

[[automation.actions]]
kind = "apple.notification.send"
params = { title = "🚨 Database issue found", body = "PostgreSQL prod has {issue.count} high-severity issues" }

[[automation.actions]]
kind = "terminal.open_pane"
params = { command = "db-diag report --db postgres@prod --severity high", dir = "right" }
```

```toml
[[automation]]
name = "weekly-infra-audit"
enabled = true

[automation.trigger]
kind = "system.cron"
params = { expr = "0 9 * * 1" }  # Monday 9am

[[automation.actions]]
kind = "infra-diag.full_check"
params = { cluster = "prod", categories = ["dns", "tls", "network", "storage"] }

[[automation.actions]]
kind = "sheets.append"
params = { sheet_id = "1abc...", row = ["{date}", "{cluster}", "{pass_count}", "{fail_count}", "{report_url}"] }
```

### Built-in Code Editor

The terminal contains a lightweight code editor with syntax highlighting. Not a plugin. Part of the core terminal, same as the note system.

#### How it works

| Action | Result |
|---|---|
| **Click a file** in the file tree sidebar | Opens in a **split pane** — editor on the top/left, shell on the bottom/right |
| **Ctrl+Click** a file | Opens in the **system editor** (`$EDITOR`, VS Code, or whatever is configured) |
| **Click a file** in a terminal output path | Same behavior — `./main.go:42` in test output is clickable |
| **Double-click** inside the editor | Selects word (standard) |
| **Right-click** in editor | Context menu: copy path, copy relative, open in system editor, copy to clipboard, share via SaaS |
| **Drag file** from tree → pane | Opens editor in that specific pane position |

#### Editor Features

```
┌────────────────────────────────────────────────────────────┐
│  main.go  ●  │  Dockerfile  │  go.mod  │  + (new tab)    │
│────────────────────────────────────────────────────────────|
│ package main                                                │
│                                                              │
│ import "fmt"                                                 │
│                                                              │
│ func main() {                                                │
│     fmt.Println("Hello, world")                              │
│ }                                                            │
│                                                              │
│ // TODO: add error handling                                  │
│ // FIXME: this is not production-ready                       │
│                                                              │
│ ────────  Ln 8  Col 3  ────  Go  ────  UTF-8  ──────────── │
└────────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────────┐
│ $ go run main.go                                            │
│ Hello, world                                                 │
│ $ _                                                          │
└────────────────────────────────────────────────────────────┘
```

**Syntax highlighting** — tree-sitter based, language-aware:
- Go, Python, TypeScript, Rust, Java, Ruby, C/C++, SQL, YAML, TOML, JSON, Markdown, Dockerfile, HCL (Terraform)
- Highlights errors inline via LSP integration
- Inline diagnostics (warnings, lint errors) shown in the gutter

**Editor actions available from the command bar:**
- `⌘+P` — file search (fuzzy find across cwd and open tabs)
- `⌘+S` — save (auto-saves on pane switch, configurable)
- `⌘+W` — close tab
- `⌘+D` — select next occurrence of word
- `⌘+/` — toggle line comment
- `⌘+Shift+F` — search in files (grep + tree-sitter query for structural search)
- `⌘+Click` on symbol — go to definition (LSP-powered)
- `⌘+Shift+Enter` — run the file (detects language, runs appropriate command)

**Theme-aware** — inherits the terminal color theme (Catppuccin, Nord, Dracula, Tokyo Night, etc.). Syntax highlighting colors are derived from the terminal ANSI palette + semantic token overrides.

#### Dual Mode: In-Terminal vs System Editor

```toml
# config.toml
[editor]
default_open = "split"    # "split" | "system" | "ask"
system_editor = "cursor"  # or "code", "vim", "nvim", "open -a TextEdit"
split_position = "right"  # "right" | "bottom" | "tab"
autosave = true
font_size = 13
line_numbers = true
minimap = false  # classic minimap in scrollbar gutter
```

Click behavior is configurable per file type:
- Regular click → split editor
- Ctrl+click → system editor
- Images, PDFs, binaries → always system default

#### File Tree Integration

The file tree sidebar (built-in) is tight with the editor:
- Click to open
- Right-click: rename, delete, move, copy path, reveal in Finder/Explorer
- Git status decorations (modified, added, deleted, conflicted)
- `.gitignore` aware — doesn't show ignored files
- Filter/search within tree (`⌘+Shift+P` → "filter files")
- Multi-root workspaces (add multiple project directories)

#### OpenCode Integration

The editor is directly connected to the OpenCode runtime:
- Highlight a function, hit `Ctrl+I` → OpenCode analyzes it for improvements
- Right-click → "Explain this" → OpenCode explains the selected code
- Right-click → "Fix with AI" → OpenCode suggests a fix inline (diff view)
- The coding agent's edits are previewed in the editor before applying

---

### First-Class Git Integration

Git is not a plugin. It's built into the terminal at every layer — the file tree, the editor, the command blocks, the status bar, and the automation engine.

#### Inline Blame

Every file opened in the editor has inline blame annotations:

```
┌────────────────────────────────────────────────────┐
│ func main() {                     // Drew Jocham   │
│     fmt.Println("Hello")          // 2 weeks ago  │
│ }                                                   │
│                                                      │
│ // TODO: add error handling    // You, yesterday   │
└────────────────────────────────────────────────────┘
```

- Shows in the editor gutter, toggled with a button or `⌘+Shift+B`
- Hover for full commit message, date, and diff
- Click to open the commit in a detail pane
- Blame is async — doesn't block the editor

#### Status Bar

```
┌─ main.go ──── Ln 8 ── Go ── ─────────────────────────┐
│  🌿 fix/auth-bug  │  +12 -3  │  🚫 1 conflict       │
└────────────────────────────────────────────────────────┘
```

- Current branch (click to switch)
- Uncommitted changes count (+/-)
- Conflict indicator (click to open conflict solver)
- Upstream status (ahead/behind)
- CI status (if connected to GitHub/GitLab)

#### Branch Switcher

`⌘+Shift+G` opens the branch switcher:

```
┌────────────────────────────────────────────┐
│ 🔍 Search branches...                       │
│────────────────────────────────────────────│
│ 🌿 fix/auth-bug                   *  ← current│
│ 🌿 main                                      │
│ 🌿 feat/new-dashboard                        │
│ ──────────────────────────────────────────── │
│ ⭐ Pull requests (3)                        │
│  #342  fix: resolve login timeout  (dependabot)│
│  #341  feat: add dashboard search            │
│  #339  chore: update deps                    │
│ ──────────────────────────────────────────── │
│ [▶ New branch] [📥 Fetch] [🔄 Rebase]        │
└────────────────────────────────────────────┘
```

- Fetches from remote asynchronously
- Shows PRs for the repo
- Stash/unstash directly from the switcher

#### Diff Viewer

When a file has uncommitted changes, a diff indicator appears next to the filename in the tab. Clicking it opens a split-pane diff:

```
┌─── main.go ──── ● ─────────────────────────────┐
│ -  fmt.Println("Hello")                         │
│ +  fmt.Println("Hello, World")                  │
│                                                  │
│ // New code                                      │
│ +  if err != nil {                               │
│ +      log.Fatal(err)                            │
│ +  }                                             │
│ ─────────────────────────────────────────────── │
│ [▶ Stage hunk] [▶ Stage all] [↩ Discard hunk]   │
│ [✏ Commit] [📋 Copy diff] [🔗 GitHub permalink] │
└──────────────────────────────────────────────────┘
```

- Hunk staging: stage individual hunks, not just files
- Hover over a line → blame annotation
- Click a line number → copy permalink to clipboard
- Stage/discard buttons inline

#### Commit UI

```
┌────────────────────────────────────────────────────────┐
│ Commit  [🌿 fix/auth-bug]                              │
│────────────────────────────────────────────────────────|
│ Summary                                                 │
│ ┌──────────────────────────────────────────────────┐   │
│ │ fix: resolve login timeout in auth handler         │   │
│ └──────────────────────────────────────────────────┘   │
│                                                          │
│ Description                                             │
│ ┌──────────────────────────────────────────────────┐   │
│ │ The OAuth callback was not handling the state     │   │
│ │ parameter correctly, causing intermittent         │   │
│ │ login failures when the session expired.          │   │
│ │                                                    │   │
│ │ Fixes: #342                                        │   │
│ └──────────────────────────────────────────────────┘   │
│                                                          │
│ Staged files: 2                                     │
│  ☑ src/auth/handler.go                               │
│  ☑ src/auth/oauth_test.go                            │
│                                                          │
│ [Commit] [Amend] [Push]                              │
└────────────────────────────────────────────────────────┘
```

- Opens automatically when you hit `⌘+Enter` in the diff viewer
- Supports commit templates from `.gitmessage`
- Conventional commit parsing (feat/fix/chore) with autocomplete
- Co-author field for pair programming
- `--no-verify` toggle for skip hooks

#### Git Log Browser

`⌘+Shift+L` opens the git log in a dedicated pane:

```
┌───────────────────────────────────────────────────────┐
│ Git Log  [🌿 fix/auth-bug]  [🔍 Search...]            │
│───────────────────────────────────────────────────────|
│ ◯ a1b2c3d  fix: resolve login timeout     10m ago     │
│ ◯ e4f5g6h  feat: add dashboard search     2h ago     │
│ ● h7i8j9k  chore: update deps             5h ago     │
│ ◯ a0b1c2d  Merge PR #338                  1d ago     │
│   ╲                                        │
│    ◯ d3e4f5g                          (branch)       │
│───────────────────────────────────────────────────────|
│  commit a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t     │
│  Author: Drew Jocham <drew@example.com>               │
│  Date:   2026-05-15 14:32:01 +0200                    │
│                                                        │
│  fix: resolve login timeout in auth handler            │
│                                                        │
│  The OAuth callback was not handling the state          │
│  parameter correctly, causing intermittent login        │
│  failures when the session expired.                    │
│                                                        │
│  Fixes: #342                                          │
│───────────────────────────────────────────────────────|
│ [Checkout] [Revert] [Cherry-pick] [Copy SHA] [Open PR]│
└───────────────────────────────────────────────────────┘
```

- Visual branch graph
- Click a commit → detail pane with full diff
- Action buttons for common operations
- Search commits by message, author, file, or SHA

#### Conflict Solver

When a merge conflict is detected, the terminal opens a three-pane conflict solver:

```
┌──────────────┬──────────────┬──────────────┐
│   Ours       │   Base       │   Theirs     │
│              │              │              │
│ func main()  │ func main()  │ func main()  │
│ {            │ {            │ {            │
│   old()      │              │   new()      │
│ }            │              │   new2()     │
│              │              │ }            │
├──────────────┴──────┬───────┴──────────────┤
│   Result              │                     │
│   func main() {       │  [Accept ours]     │
│     old()             │  [Accept theirs]    │
│     new()             │  [Accept both]      │
│     new2()            │  [Edit manually]    │
│   }                   │                     │
└───────────────────────┴──────────────────────┘
```

- Ours / Base / Theirs split view
- Click any line in a pane to add it to the result
- Keyboard navigation: `Tab` to cycle panes, `Space` to toggle lines, `Enter` to accept
- Auto-detects conflict markers in any open file

#### Automation Integration

Git events are triggers for the automation engine:

```toml
[[automation]]
name = "pr-checks"
enabled = true

[automation.trigger]
kind = "git.branch_created"
params = { pattern = "feat/*" }

[[automation.actions]]
kind = "terminal.open_pane"
params = {
    command = "gh pr create --fill --label feat",
    dir = "right"
}
```

Git triggers:
- `git.commit` — committed
- `git.branch_created` / `git.branch_deleted` / `git.branch_switched`
- `git.push` / `git.push_done`
- `git.conflict` — merge conflict detected
- `git.pr_created` / `git.pr_merged` / `git.pr_closed`
- `git.ci_status_changed` — CI passed/failed

Git actions:
- `git.commit` — commit staged changes (with message)
- `git.push` — push current branch
- `git.create_branch` — create and switch to new branch
- `git.stash` — stash working changes
- `git.revert` — revert a commit

#### Keybindings

| Binding | Action |
|---|---|
| `⌘+Shift+G` | Branch switcher |
| `⌘+Shift+L` | Git log |
| `⌘+Shift+B` | Toggle blame |
| `⌘+Shift+D` | Diff viewer for current file |
| `⌘+Shift+.` | Stage current file |
| `⌘+Enter` | Commit (when in diff view) |
| `⌘+Shift+P` then "git push" | Push |
| `⌘+Shift+P` then "git stash" | Stash |

#### Why first-class, not a plugin

Git is the one tool every developer uses in every project. Making it a plugin would mean:
- The file tree can't show git status without a plugin dependency
- The editor can't show blame without another plugin
- The status bar can't show the branch
- Automation can't trigger on git events

Git touches every layer of the terminal — the file tree, the editor, the status bar, the command palette, the diff viewer, the automation engine. It's a core utility, same as the note system.

---

### Meeting Recorder (Built-in)

Records meetings using the laptop's microphone and system audio. Standalone — no Zoom, Teams, Meet, or Slack integration. Just press record and get a transcript + summary + tasks.

#### How it Works

```
┌─ ● REC ───────────────────────────────────────────────┐
│  00:23:47  │  Meeting: Sprint Planning - 2026-05-16    │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │ Drew (00:00:12): "Let's start with the sprint    │  │
│  │ goals for this week..."                          │  │
│  │ Sarah (00:00:45): "I picked up the auth ticket   │  │
│  │ yesterday. Should have a PR by EOD."             │  │
│  │ Drew (00:01:20): "Great. Action item: review     │  │
│  │ Sarah's PR before standup tomorrow."             │  │
│  └──────────────────────────────────────────────────┘  │
│                                                        │
│  [■ Stop] [⏸ Pause] [🎤 Mic: Built-in] [🔊 Speakers] │
│                                                        │
│  Processing status when done:                          │
│  ✅ Transcribed (whisper, 12m audio → 4m process)     │
│  ✅ Summary generated                                  │
│  ✅ Tasks extracted (3 action items)                   │
│  📝 Saved to notes / daily Obsidian note               │
└────────────────────────────────────────────────────────┘
```

#### Key Design Decisions

| Decision | Why |
|---|---|
| **Local mic + system audio** | Uses the laptop's audio hardware. No app API, no browser extension, no integration contracts. Works with any meeting platform (or phone calls). |
| **Local Whisper as default** | Transcription runs locally via Ollama/Whisper. Cloud Whisper API is opt-in for speed. Privacy-sensitive meetings never leave the machine. |
| **OpenCode agent for processing** | After recording stops, the terminal spawns an OpenCode agent with the audio file. The agent transcribes, summarizes, and extracts tasks using whatever AI provider the user has configured. |
| **Output is a terminal note** | The transcript + summary + tasks become a note in the built-in note system, linked to the timestamp and the daily Obsidian note. |
| **Red dot always visible** | Recording indicator in the status bar. Cannot be hidden. If the terminal is recording, you can see it. |

#### Controls

| Action | Shortcut |
|---|---|
| Start recording | `Ctrl+Shift+R` or click mic icon in status bar |
| Stop recording | `Ctrl+Shift+R` again, or click the red dot |
| Pause/resume | `Ctrl+Shift+Escape` |
| Cancel (discard) | `Ctrl+Shift+Backspace` |
| View last recording | `Ctrl+Shift+M` opens the meeting log |

#### Architecture

```go
package recorder

type Session struct {
    ID         string
    StartedAt  time.Time
    Duration   time.Duration
    Status     RecorderStatus  // idle, recording, paused, processing, done
    MicInput   *audio.Input    // from CoreAudio / ALSA / PulseAudio
    SpeakerCapture *audio.Input  // loopback from system audio
    OutputFile  string          // path to recorded WAV/FLAC
}

type Processor struct {
    opencode *opencode.Runtime  // spawns agent with audio file
    model    ModelConfig        // local whisper, cloud whisper, or BYO
}

func (p *Processor) Process(ctx context.Context, audioPath string) (*MeetingResult, error) {
    // 1. Spawn OpenCode agent with the audio file
    // 2. Agent transcribes (whisper CLI or API)
    // 3. Agent summarizes (LLM)
    // 4. Agent extracts tasks (LLM with structured output)
    // 5. Save as terminal note
    // 6. Link to Obsidian daily note
    // 7. Return result
}

type MeetingResult struct {
    Transcript   []TranscriptSegment  // speaker, timestamp, text
    Summary      string
    ActionItems  []ActionItem
    Decisions    []string
    Attendees    []string  // speaker-diarized names
    NoteID       string    // link back to the terminal note
    ObsidianLink string    // link to daily note if vault connected
}

type ActionItem struct {
    Description string
    Assignee    string
    Priority    string  // high, medium, low
    DueDate     *time.Time
    Source      string  // "google-tasks", "apple-reminders", or "argus-cloud"
}

// Tasks are written to the user's chosen service:
// - Google Tasks: via Google OAuth (works anywhere)
// - Apple Reminders: via AppleScript/Reminders.app (macOS only)
// - Argus Cloud: via SaaS API (works anywhere, included in service)
// Configured once in settings. Default: Argus Cloud if connected, else Google Tasks.
```

---

### Plugin API

```go
// Terminal → Plugin
type PluginAPI interface {
    // Core hooks
    OnCommandExecuted(cmd Command)        // command block was executed
    OnOutputReceived(cmd Command, output Output)
    OnKeyEvent(key KeyEvent)              // plugin can intercept/handle keys
    OnSessionEvent(event SessionEvent)    // open/close/resize
    OnRender(frame *Frame)                // plugin can modify render output
    
    // UI contribution points
    ToolbarItems() []ToolbarItem          // buttons in toolbar
    StatusBarItems() []StatusBarItem      // widgets in status bar
    SidebarPanels() []SidebarPanel        // panels in sidebar
    ContextMenuItems(text string) []MenuItem
    
    // Data provision
    ProvideCompletions(ctx CompletionContext) []Completion
    ProvideHints(ctx HoverContext) []Hint
    
    // Automation (triggers & actions)
    RegisterTriggers() []automate.TriggerDef   // new triggers for the automation engine
    RegisterActions() []automate.ActionDef     // new actions for the automation engine
    
    // Capabilities
    Name() string
    Version() string
    Hooks() []PluginHookType
}

// Plugin → Terminal
type TerminalAPI interface {
    // Core
    CurrentSession() SessionInfo
    ActiveCommand() *Command
    Scrollback() []Line
    
    // Actions
    ExecuteCommand(cmd string) error
    OpenPane(dir SplitDirection)
    ClosePane(id PaneID)
    
    // Display
    ShowNotification(msg string, kind NotificationKind)
    ShowSidebar(panelID string)
    WriteToPane(paneID PaneID, text string)
    
    // State
    GetConfig(key string) (any, error)
    SetConfig(key string, value any) error
    
    // Storage
    Store() StorageBackend // key-value + structured data per plugin
}
```

---

## 8. Plugin Manifest

Every plugin ships with a `plugin.toml` manifest:

```toml
[plugin]
name = "argus-kube"
version = "1.0.0"
description = "Kubernetes cluster management with KubeWatcher"
author = "Drew Jocham"
license = "MIT"
min-terminal-version = "0.1.0"

[plugin.hooks]
on_command_executed = true
on_key_event = false
on_render = false
provide_completions = true
sidebar_panels = true
status_bar_items = true

[plugin.ui]
sidebar = { title = "K8s Cluster", icon = "☸️", default_open = false }
status_bar = [
    { id = "k8s-context", position = "left", priority = 5 },
    { id = "k8s-alerts", position = "right", priority = 8 },
]

[plugin.commands]
prefix = "k8s"
commands = [
    "k8s.context.list",
    "k8s.context.switch",
    "k8s.pods.get",
    "k8s.analyze",
    ...
]

[plugin.runtime]
type = "external"
transport = "jsonrpc"
entry_point = "argus-mcp-host"
args = ["--stdio"]
```

---

## 9. Plugin Store & Discovery

```go
package plugin

type Registry struct {
    store *Store           // plugin marketplace index
    local map[string]*Plugin  // installed plugins
}

type Store struct {
    // Central index (or self-hosted)
    IndexURL string  // e.g., "https://plugins.argus-terminal.dev/v1"
    
    // Each plugin
    Plugins []PluginIndexEntry
}

type PluginIndexEntry struct {
    Name        string
    Version     string
    Description string
    Author      string
    Downloads   int
    Rating      float64
    Tags        []string     // "k8s", "ai", "git", "docker", "database", "monitoring"
    SourceURL   string       // git repo
    BinaryURL   string       // prebuilt binary by platform
    Manifest    string       // plugin.toml inline
}
```

### Installation flow

```
> Ctrl+Shift+P → "Install Plugin"
→ Opens plugin browser
→ Search / Browse categories
→ "Install argus-kube"
→ Terminal downloads manifest, binary, verifies signature
→ Plugin registered, hooks wired, UI items added
→ No restart needed — hot-reload
```

---

## 10. Terminal Core (Minimal)

The core stays lean:

| Module | Files | Lines | Purpose |
|---|---|---|---|
| PTY manager | 1 | ~200 | Process lifecycle, resize, signals |
| ANSI parser | 2 | ~600 | Streaming parser, all DCS/OSC/CSI |
| Screen buffer | 2 | ~400 | Cell grid, scrollback ring buffer |
| Renderer | 3 | ~500 | OpenGL init, font atlas, frame draw |
| Input | 1 | ~300 | Key mapping, kitty protocol, mouse |
| Layout | 2 | ~400 | Panes, tabs, splits, drag |
| Config | 1 | ~200 | TOML loading, defaults |
| Plugin host | 3 | ~600 | Plugin lifecycle, IPC, registry, events |
| Automation engine | 4 | ~500 | Trigger engine, bridges (Google, Apple, System), rule store |
| Notes | 3 | ~400 | Inline note storage, markdown render, search |
| Obsidian | 4 | ~500 | Vault discovery, indexing, VaultQL, watcher |
| OpenCode | 5 | ~700 | ACP runtime, toolset, LSP client, session management |
| Editor | 4 | ~600 | Syntax highlighting (tree-sitter), multi-tab, split-pane, LSP diagnostics |
| File tree | 2 | ~300 | Directory browser, git status decorators, multi-root workspaces |
| **Total** | **37** | **~6,200** | |

Everything else is a plugin:
- AI assistant → plugin
- Docker management → plugin
- Resource monitoring → plugin
- **ArgusKube** → plugin (but bundled/first-party)

---

## 11. The ArgusKube Plugin — The Flagship Plugin

ArgusKube is the terminal's flagship plugin and the best demonstration of what the plugin API can do. It has deep integration because the plugin system is designed for deep integration — not special treatment in the core.

#### What it does (all via the standard plugin API):

| Feature | Plugin API Used |
|---|---|
| **Load testing** — distributed load generation against K8s clusters | Commands (`argus.loadtest`), Sidebar panels, Codex tools |
| **Cluster management** — pod list, logs, exec, deployments | Semantic completions (`kubectl get`, `logs`, `exec`), Sidebar panels |
| **AI agent** — chat with the Argus agent about cluster state | Sidebar panel with WebView render, Own MCP tools registered with the agent |
| **Alert feed** — real-time stream from the alert processor | Status bar items, Sidebar panel, Automation triggers |
| **Workflow runner** — execute pipelines from the terminal | Commands, Automation actions, Codex tools |
| **Cluster context manager** — color-coded warnings before prod ops | Status bar items, Render hooks, Key event interceptor |
| **Popeye scans** — inline results with severity filtering | Commands, Completion provider, Automation triggers |

Every bit of this integration uses the same `PluginAPI` that any community plugin would use. There is no private API, no special hooks reserved for first-party plugins. The depth you see with ArgusKube is available to anyone building on the platform.

```go
// plugins/argus-kube/main.go
package main

import (
    "github.com/argues/terminal/plugin"
    "github.com/argues/terminal/plugin/api"
)

type ArgusKubePlugin struct {
    mcpClient *MCPClient
    agent     *AgentChat
    alerts    *AlertFeed
}

func (p *ArgusKubePlugin) SidebarPanels() []api.SidebarPanel {
    return []api.SidebarPanel{
        {
            ID:    "k8s-cluster",
            Title: "Cluster",
            Icon:  "☸️",
            Render: func(ctx api.RenderContext) {
                // Render cluster state
            },
        },
        {
            ID:    "k8s-agent",
            Title: "AI Agent",
            Icon:  "🤖",
            Render: func(ctx api.RenderContext) {
                // Chat interface
            },
        },
        {
            ID:    "k8s-alerts",
            Title: "Alerts",
            Icon:  "🔔",
            Render: func(ctx api.RenderContext) {
                // Alert stream
            },
        },
    }
}

func (p *ArgusKubePlugin) ProvideCompletions(ctx api.CompletionContext) []api.Completion {
    if !strings.HasPrefix(ctx.Command, "kubectl") {
        return nil
    }
    return p.semanticCompletions(ctx)
}
```

---

## 12. Example Plugin Ecosystem

| Plugin | What it does | Runtime |
|---|---|---|
| **argus-kube** ⭐ | K8s cluster management, AI agent, alerts | External (Go) |
| **argus-cloud** ⭐ | SaaS sync layer — vaults, notes, rules, credentials across devices | External (Go) |
| **git-worktree** | Inline git blame, branch switcher, PR status | External (Go) |
| **docker-desktop** | Container logs, exec, compose management | External (Go) |
| **ai-companion** | Any LLM (OpenAI, Anthropic, local Ollama) | External (Python/JS) |
| **db-diag** | Database diagnostics — slow queries, index analysis, hotspot detection | External (Go) |
| **infra-diag** | Infrastructure diagnostics — connectivity, DNS, TLS, load balancer, storage | External (Go) |
| **ai-reporter** | Spawns an agent to analyze diagnostics and write reports to local, Google Docs, or S3 | External (Go) |
| **resource-mon** | CPU/RAM/Disk/Network graphs in status bar | Native (Go) |
| **session-sync** | Sync tabs/panes across machines | External (Go) |
| **theme-studio** | Custom theme builder | WebView |
| **ssh-manager** | SSH host manager with autocomplete | External (Go) |
| **http-inspector** | Intercept/record HTTP requests from terminal | External (Rust) |

---

## 13. Why This Wins

### vs Warp
- **Open plugin ecosystem** — not just "our AI." You want Claude? Ollama? Your own fine-tuned model? Pick one.
- **Go-based** — single binary, no Electron memory tax, cross-compile everywhere
- **Not cloud-dependent** — plugins can be fully local
- **BYO everything** — your theme, your keybinds, your plugins

### vs iTerm2 / Kitty / Alacritty
- **Actually extensible** — not just "you can run a script." A real plugin API with UI contribution points.
- **Modern UX** — command blocks, smart autocomplete, AI integration
- **Structured output** — the terminal understands what it's displaying, not just painting pixels

### vs VSCode Terminal
- **It's a terminal app**, not an IDE sidebar. Full screen, GPU accelerated, no Electron overhead.

---

## 14. Phased Delivery

### Phase 0 — "Skeleton" (1 week)
- Go module scaffold, GLFW window, OpenGL clear
- PTY spawn + display raw bytes
- Basic keyboard input → PTY

### Phase 1 — "It's a Terminal + OpenCode" (4 weeks)
- Full ANSI parser (colors, styles, cursor, scroll)
- Font atlas (JetBrains Mono, Nerd Font)
- 60fps GPU rendering
- Scrollback buffer (50k lines)
- Copy/paste, selection
- Tabs
- **Built-in note system** — command block notes, SQLite persistence
- **OpenCode runtime** — ACP-compatible AI coding agent, BYO model, toolset (read, write, edit, exec, search, git)

### Phase 2 — "Multiplayer + Obsidian" (4 weeks)
- **Obsidian integration** — vault discovery, VaultQL, file watcher, wikilink support
- Split panes (horizontal/vertical)
- Command blocks with edit/re-run
- Shell integration (preexec/precmd → timing, exit codes)
- Session persistence across restarts
- Config (TOML): font, theme, keybindings, shell
- **Automation engine foundation** — trigger engine, system bridge, cron, file watcher, terminal events

### Phase 3 — "Connected Terminal" (6 weeks)
- **Automation bridges**: Google (OAuth + Gmail/Calendar/Drive poll), Apple (AppleScript/Shortcuts)
- Plugin host: registry, lifecycle, IPC (JSON-RPC)
- Plugin API: hooks, UI contributions, storage, trigger/action registration

### Phase 4 — "Plugin System" (8 weeks)
- Plugin host: registry, lifecycle, IPC (JSON-RPC)
- Plugin API: hooks, UI contributions, storage
- Plugin store: index, install, update
- Sample plugins: git-worktree, ai-companion, resource-mon
- **ArgusKube plugin** — first-party, bundled with the terminal

### Phase 4 — "Ecosystem" (ongoing)
- Plugin SDK docs + examples
- Community plugin registry
- WebView plugin support (embedded HTML/JS)
- Collaborative sessions
- Mobile companion (view/control from phone)
- Native macOS/Linux/Windows packaging (DMG, .deb, .msi)

---

## 15. Go Scaffolding Snapshot

```
argus-terminal/
├── cmd/argus-terminal/
│   └── main.go
├── internal/
│   ├── pty/
│   ├── ansi/
│   ├── screen/
│   ├── render/
│   ├── input/
│   ├── layout/
│   ├── plugin/
│   ├── notes/               # built-in note system
│   │   ├── store.go         # SQLite-backed note persistence
│   │   ├── note.go          # Note type + NoteStore interface
│   │   └── markdown.go      # inline markdown rendering
│   ├── obsidian/            # first-class vault integration
│   │   ├── vault.go         # vault discovery + indexing
│   │   ├── query.go         # VaultQL parser
│   │   ├── watch.go         # fsnotify/FSEvents watcher
│   │   └── render.go        # Obsidian markdown → terminal renderer
│   ├── opencode/            # built-in AI coding runtime
│   │   ├── runtime.go       # ACP session + tool executor
│   │   ├── toolset.go       # read, write, edit, exec, search, grep, git
│   │   ├── lsp.go           # LSP integration for smarter edits
│   │   └── session.go       # session lifecycle + history
│   ├── editor/              # built-in code editor
│   │   ├── editor.go        # Multi-tab, split-pane editing
│   │   ├── syntax.go        # Tree-sitter highlighting
│   │   ├── diagnostics.go   # Inline LSP diagnostics in gutter
│   │   └── open.go          # File open logic (split vs system)
│   ├── filetree/            # built-in file tree
│   │   ├── tree.go          # Directory browser
│   │   ├── gitstatus.go     # Git status decorations
│   │   └── workspace.go     # Multi-root workspace management
│   ├── git/                 # built-in git integration
│   │   ├── blame.go         # Async inline blame
│   │   ├── diff.go          # Hunk-level diff viewer
│   │   ├── commit.go        # Commit UI + message editing
│   │   ├── branch.go        # Branch switcher + PR display
│   │   ├── log.go           # Git log browser + graph
│   │   ├── conflict.go      # Three-pane conflict solver
│   │   └── statusbar.go     # Branch/changes in status bar
│   └── recorder/            # built-in meeting recorder
│       ├── recorder.go      # Mic + system audio capture
│       ├── processor.go     # OpenCode agent spawn for transcript/summary/tasks
│       └── audio.go         # CoreAudio/ALSA/PulseAudio input loopback
│   └── automate/            # Automation engine
│       ├── engine.go        # Rule evaluation + event loop
│       ├── bridges.go       # Bridge interface + registry
│       ├── rule.go          # Rule, Trigger, Action, Condition types
│       ├── store.go         # SQLite-backed rule persistence
│       ├── bridge_google.go # Google OAuth + API polling
│       ├── bridge_apple.go  # AppleScript / Shortcuts bridge
│       └── bridge_system.go # File watcher, cron, battery, WiFi
├── plugins/
│   └── argus-kube/       # First-party plugin
├── plugin/
│   ├── api/
│   │   ├── hooks.go
│   │   ├── ui.go
│   │   ├── completions.go
│   │   └── storage.go
│   ├── host.go           # Plugin lifecycle
│   ├── registry.go       # Installed plugin management
│   └── store.go          # Plugin marketplace client
├── go.mod
├── go.sum
├── shaders/
└── Makefile
```

---

Want me to start writing the actual scaffolding? I can produce `main.go`, the PTY manager, ANSI parser skeleton, OpenGL init loop, and plugin host — enough to get a window on screen with a shell prompt.

```google-calendar
{
  "date": "2026-05-16",
  "refreshInterval": 60,
  "showEvents": true,
  "showTasks": true,
  "title": "📅 Calendar for 2026-05-16"
}
```