# PR-5 — ANSI parser library survey

**Audit recommendation:** swap `lufis-terminal/internal/ansi` for `jwalton/gchalk` or `charmbracelet/x/ansi`.

**Recommendation:** **defer until PR-7 lands.** The audit's two suggested libraries were not a like-for-like match, and any parser swap is downstream of the screen-emulator decision in PR-7. Pushing this through now would burn ~500 lines of regression risk for no end-user benefit.

---

## What lufis-terminal does today

- `internal/ansi/parser.go` — 491 lines. Hand-rolled Paul Williams DEC ANSI state machine (ground / escape / CSI{Entry,Param,Intermediate,Ignore} / OSC / DCS).
- `internal/ansi/parser_test.go` — 350 lines, exercises CUP / SGR / OSC 0/2 / scroll regions / DECSC/DECRC / private modes / UTF-8 multi-byte / clamps.
- Output target: every dispatch calls `p.buffer.<Op>` against `internal/screen.Buffer`. The parser is not a standalone library — it is the front half of the lufis VT emulator.

Coupling matters. The parser does not produce a stream of events; it directly mutates a screen buffer. Any replacement has to either:
1. preserve the exact `screen.Buffer` call surface (and we still own the translation glue), or
2. replace `screen.Buffer` at the same time (which is PR-7's job).

## The two audit suggestions, examined

### `jwalton/gchalk`
Not a parser. `gchalk` is a Go port of Node's `chalk` — a library for **emitting** styled output (`gchalk.Red("hi")`). It has no CSI state machine, no OSC handling, no cursor-movement dispatch. Recommending it here was a category error in the audit.

### `charmbracelet/x/ansi`
The real candidate. It is the parser layer extracted from Bubble Tea's terminal stack:
- Implements the same Paul Williams state machine we wrote by hand.
- Event-style API (`Parser.Parse(data) → Sequence`) — emits typed sequences the consumer must dispatch.
- Actively maintained by Charm, used by Bubble Tea, Glow, Soft Serve.
- Adds a transitive dep on `charmbracelet/x/*` (cell width, term, conpty).

Effort to adopt:
- Rewrite ~250 lines of dispatch glue to translate `x/ansi.Sequence` → `screen.Buffer.<Op>` calls.
- Drop the state-machine half (~250 lines) and most of the test file (state-machine tests become library tests we don't need to maintain).
- New dep tree: `charmbracelet/x/ansi` + `mattn/go-runewidth` (we already have an equivalent in screen).
- Risk: subtle divergences in mode handling (DECSCUSR, DECSET 1049 alternate screen) where our hand-roll has been tuned against real shells.

## Why this is downstream of PR-7

PR-7 surveys `go-libvterm` (cgo binding to libvterm) as a replacement for `internal/screen`. `libvterm` is a **complete** terminal emulator: it ships its own VT100/xterm parser and its own screen buffer. If PR-7 says "adopt libvterm", `internal/ansi` deletes outright — no `charmbracelet/x/ansi` swap needed.

The decision tree:

| PR-7 outcome | What happens to `internal/ansi` |
| --- | --- |
| Adopt `go-libvterm` | Delete `internal/ansi` entirely. Parser swap = wasted work. |
| Keep `internal/screen` | Re-evaluate ANSI parser swap *on top of the same screen contract.* |
| Adopt a different screen lib | Re-evaluate against that library's parser story. |

In all three branches, doing the parser swap *first* either becomes redundant or has to be redone against the new screen contract.

## What we'd gain from a swap (if PR-7 keeps `internal/screen`)

- −250 lines of state-machine code we maintain.
- Bug fixes for free (Charm's tests are broader than ours, especially around OSC 8 hyperlinks, OSC 52 clipboard, Kitty image protocol).
- Hyperlink + image protocol support without us writing it.

## What we'd lose

- Tight coupling means we currently know exactly which CSI sequence touches which buffer op. A library boundary obscures that, making bugs harder to bisect.
- Our current parser is ~6 KB of memory + zero allocations on the hot path; `x/ansi` allocates per-sequence.
- One more dep + transitive deps to vet.

## Recommendation

1. **Do not open a PR for an ANSI parser swap now.**
2. **Block on PR-7's recommendation.** If PR-7 says "keep `internal/screen`", reopen this survey with a concrete proof-of-concept branch (5 hours: bridge `x/ansi.Sequence` → `screen.Buffer`).
3. **Reject `jwalton/gchalk`** outright — wrong category of library.

Estimated total effort if we proceed after PR-7: **8 hours** (4 hours glue + 4 hours regression testing against real shells: zsh, bash, fish, nushell with k9s, lazygit, btop).
