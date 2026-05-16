# Argus Terminal — Design Considerations

## Performance & Speed

The terminal must feel instant. Every millisecond of latency is noticeable.

1. **GPU rendering pipeline cannot block on PTY I/O** — render loop and PTY reader are separate goroutines with a lock-free cell buffer (atomic swap or double-buffering). If the PTY is spewing data, the render loop still draws at 60fps.

2. **Lazy everything** — don't start automation engine, Obsidian watcher, git blame cache, or plugin host until the shell prompt appears. Terminal goes from launch → visible window → shell prompt in under 500ms.

3. **Plugin cold start is invisible** — plugins start in the background, register hooks atomically. User sees a notification, not a spinner.

4. **No blocking file I/O on the render thread** — git blame, file tree indexing, Obsidian vault scan all on worker goroutines. File tree shows loading state while scanning.

5. **Terminal rendering is the only P0 thread** — everything else (notes, editor, git, Obsidian, automations, plugins) can be deferred or drop frames under load.

## Maintainability

1. **The core is ~15 files, not 200.** Everything else is `internal/` or a plugin. Core = PTY, ANSI, screen buffer, GPU render, input, layout, plugin host, config, auth.

2. **Single responsibility for packages** — `git/blame.go` does blame and only blame. If a file exceeds 400 lines, it's doing too much.

3. **The plugin API is the contract** — design once, test thoroughly, avoid breaking changes. Use stable JSON-RPC schema.

4. **Feature flags for everything new** — every component starts behind a flag. Buggy? Disable without a code change.

5. **Opt-in telemetry** — crash reports + anonymous perf metrics. Know if GPU drops to 30fps on M1 vs Intel.

## Don't Annoy the User

1. **Command blocks are opt-in, not default.** Default to traditional scrollback. Subtle toggle in settings.

2. **No auto-AI.** Never inject AI output unprompted. AI lives in sidebar/pane the user opens explicitly.

3. **Plugins are silent until used.** No notification, no toolbar icon, no appearance change until user interacts.

4. **Status bar is for terminal info, not plugin ads.** Branch, exit codes, cluster context, recording indicator. No promotional nudges.

5. **Zero onboarding.** No tutorial overlay, no welcome tour. Usable immediately. Discoverability via command palette (`Ctrl+Shift+P`).

6. **Config file or GUI, never both required.** Everything configurable in TOML. Everything clickable in settings.

## Still a Terminal

1. **Default view is pure terminal.** No sidebar, no bottom panel. 100% terminal screen. Everything else is opt-in.

2. **Everything is a pane.** Editor, git log, AI chat, meeting recorder — all panes. Same keybindings, same drag, same close.

3. **Command palette is the safety valve.** `Ctrl+Shift+P` → type what you want. No learning a new UI.

4. **Keyboard-first, always.** Every feature has a shortcut. Mouse never required.

5. **Additions invisible when not in use.** If you never use the meeting recorder, it doesn't exist. No icon, no menu item.

6. **PTY output is the hero.** Every non-terminal feature is visually subordinate. Collapsible sidebars, smaller default splits.

```google-calendar
{
  "date": "2026-05-16",
  "refreshInterval": 60,
  "showEvents": true,
  "showTasks": true,
  "title": "📅 Calendar for 2026-05-16"
}
```