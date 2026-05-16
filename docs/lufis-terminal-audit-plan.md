# lufis-terminal + kube/backend Terminal — Audit Plan

Status: **proposed**. PRs land one at a time in the order below; each one is independently mergeable so the queue can be paused or reordered.

Source audit (provided by the user, summarized):

> 1. PTY duplication between `kube/backend/internal/terminal/terminal.go` and `lufis-terminal/internal/pty/pty.go` (HIGH)
> 2. `terminal_handler.go` too thick — violates AGENTS.md Handler Pattern (HIGH)
> 3. lufis-terminal main.go has no DI (HIGH)
> 4. `terminal_test.go` not table-driven — violates AGENTS.md (MEDIUM)
> 5. `app_terminal_launcher.go` errors lack `%w` (MEDIUM)
> 6. `internal/screen/` → go-libvterm (HIGH library replacement)
> 7. `internal/plugin/` → hashicorp/go-plugin (HIGH library replacement)
> 8. `internal/auth/` → jwt-go (CRITICAL library replacement)
> 9. `internal/ansi/` → mature ANSI parser (MEDIUM library replacement)
> 10. `internal/config/` → viper (MEDIUM library replacement)
> 11. `internal/recorder/` → tview/bubbletea (LOW library replacement)

## How I'm triaging — explicit pushback on three items

Before the PR sequence, three audit items deserve scrutiny rather than blind execution:

### #3 "No DI in lufis main.go" — disagree, not actioning
The lufis `main.go` does manual wiring of `logger / config / pty / window / ...`. That's **the composition root**, the one place every Go program is supposed to wire dependencies. Adding a DI container here would invert idiomatic Go for no real benefit. **No PR. Pushing back.**

### #8 "internal/auth/ → jwt-go (CRITICAL)" — wrong tool, but real underlying issue
`jwt-go` parses and signs JSON Web Tokens. It does **not** store credentials. The actual concern in `internal/auth/store.go` is that API keys + tokens are written to plaintext JSON on disk — a real security gap. The fix is **OS-keychain integration** (the argus backend already does this via `secretref`), not jwt-go.

→ Reframing this as **"encrypt credential store at rest"** and landing it as PR-6 below.

### #7 "internal/plugin/ → hashicorp/go-plugin (HIGH)" — defer
`hashicorp/go-plugin` runs plugins as **separate processes** over RPC. The current lufis plugin system is in-process. Switching is days of refactor with real semantic changes (out-of-process means slower, harder to debug, but isolated from crashes). The current system works.

→ **Surveying first** as PR-9 (read-only design doc); ship code only if the survey finds concrete problems with the current setup.

## PR sequence

Each PR lands independently. Numbered in dependency order — earlier PRs unblock later ones.

| # | PR | Scope | Audit item |
| --- | --- | --- | --- |
| **1** | quick: `%w` on app_terminal_launcher errors | 5 lines, mechanical | #5 |
| **2** | table-driven conversion of `internal/terminal/terminal_test.go` | ~150 lines, mechanical | #4 |
| **3** | extract shared PTY package consumed by both backends | new `pkg/pty/`, move common parts; both call sites updated | #1 |
| **4** | refactor `terminal_handler.go` to thin handler + service | handler → service split; existing tests stay green | #2 |
| **5** | swap `internal/ansi/` for a mature ANSI parser library | evaluate `github.com/jwalton/gchalk` or upstream wezterm parser; behind a feature flag during the transition | #9 |
| **6** | encrypt the lufis credential store at rest (OS keychain) | replace plaintext JSON with `99designs/keyring` (cross-platform); **NOT jwt-go** | #8 reframed |
| **7** | evaluate replacing `internal/screen/` with `go-libvterm` | survey + ADR. Implementation only if survey approves (CGo dep is real cost) | #6 |
| **8** | replace `internal/config/` manual TOML with viper | 80-line config → viper; tests stay green | #10 |
| **9** | survey lufis plugin system vs hashicorp/go-plugin | read-only design doc; implementation deferred to a separate decision | #7 |
| **10** | `internal/recorder/` — replace with `tview` or hand-rewritten using `bubbletea`? | survey + ADR; LOW priority, queue at the back | #11 |

## Per-PR detail

### PR-1: `%w` wrapping in `app_terminal_launcher.go`
Tiny mechanical change. Audit findings:
- `fmt.Errorf("launch lufis-terminal: %w", err)` — already wrapped (good)
- `fmt.Errorf("ARGUS_LUFIS_PATH=%s does not exist", p)` — no underlying error to wrap; OK as-is
- `errors.New("lufis-terminal binary not found on PATH (set ARGUS_LUFIS_PATH)")` — sentinel error; OK as-is

After re-reading the file, the audit's MEDIUM is **debatable** — most error returns there don't have an upstream error to wrap. PR will fix any genuine wrapping omission and add a comment explaining the sentinel decisions. **Estimated time: 30 min.**

### PR-2: table-drive `internal/terminal/terminal_test.go`
14 standalone `TestXxx` functions → ~3 table-driven `TestXxxBehavior` functions covering the same surface. No behavior change, no production code touched. **Estimated time: 2 hours.**

### PR-3: extract shared PTY package
New top-level `pkg/pty/` (or `internal/pty/` shared between modules — needs Go-module-boundary check first). Move:
- The `cmd := exec.Command(shell, "-l")` PTY-start dance
- The PTY size-resize loop
- The OS-specific `setsid` / process-group attachment

Both `kube/backend/internal/terminal` and `lufis-terminal/internal/pty` become thin adapters over the shared core. **Estimated time: 1 day. Risk: medium — module boundary may force a third repo or a Go workspace.**

### PR-4: refactor terminal_handler.go to thin handler + service
Per AGENTS.md handler pattern:
- `terminal_handler.go` becomes a thin "parse args → call service → format response" handler
- New `internal/terminal/service.go` owns session lifecycle (Start, Send, Close, List)
- Handler tests use a fake service; service tests cover the real lifecycle
**Estimated time: 4 hours.**

### PR-5: ANSI parser library swap
Evaluate `github.com/jwalton/gchalk` (popular, no CGo) and `github.com/charmbracelet/x/ansi` (charmbracelet's, used by bubbletea). Pick one based on:
- API ergonomics for our use case (parsing escape sequences, not generating them)
- Footprint
- Maintenance freshness

Behind a build tag during transition so the old code stays runnable for one release. **Estimated time: 1 day.**

### PR-6: encrypt credential store at rest (formerly "jwt-go")
Replace plaintext JSON in `internal/auth/store.go` with `github.com/99designs/keyring` — a cross-platform OS-keychain wrapper (Keychain on macOS, libsecret on Linux, Credential Manager on Windows). Falls back to encrypted file on platforms without a system keyring.

Migration path: on first-run with the new code, read the old JSON, push into keyring, delete the JSON. **Estimated time: 1 day.**

### PR-7: survey `go-libvterm` for screen/ansi consolidation
go-libvterm is a CGo wrapper around the C `libvterm` library — a real terminal emulator state machine. Pros: ~700 lines of our custom emulation goes away. Cons: CGo build cost, distribution complexity (binary must ship with libvterm headers), Windows story is uncertain.

PR is **survey-only** — design doc + perf-comparison table. Implementation is a separate decision. **Estimated time: 1 day for survey, 1 week for implementation if approved.**

### PR-8: replace `internal/config/` with viper
~80 lines of manual TOML loading → viper. Justification is thin — viper is heavy for so little config — but the audit calls it out. PR will measure import size delta and revert if the cost outweighs the simplification. **Estimated time: 4 hours.**

### PR-9: survey the lufis plugin system vs hashicorp/go-plugin
**Read-only survey** — no code change. Compares:
- Current in-process plugin loading
- `hashicorp/go-plugin` (out-of-process, RPC, crash-isolated)
- Alternative: `traefik/yaegi` (in-process Go interpreter, simpler)

Output: an ADR with concrete pros/cons keyed to actual lufis plugin use cases. **Estimated time: 1 day. Implementation gated on approval.**

### PR-10: `internal/recorder/` library evaluation
Low priority. Same survey-then-decide shape as PR-9. **Estimated time: half-day.**

## Total scope
- Quick wins (PRs 1–2): half a day
- Refactors (PRs 3–4): ~1.5 days
- Library evaluations + swaps (PRs 5–8, 10): 3–4 days if all approved; ~1 day for survey work if approved selectively
- PR-9 survey: 1 day

**Realistic total: 5–8 working days if every recommendation lands. 2–3 days if we skip the heaviest library swaps.**

## Out of scope (audit items NOT actioned)

- **"No DI in lufis main.go"** — main is the composition root; idiomatic Go. No change.
- **"jwt-go for credential storage"** — wrong tool; reframed as PR-6 (keyring).

## Refs
- The audit text the user pasted
- `AGENTS.md` (handler pattern + table-driven test requirement)
- PR #91 (the chi panic fix that unblocks the build) — must merge first
