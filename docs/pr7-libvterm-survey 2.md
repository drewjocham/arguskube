# PR-7 — `go-libvterm` survey for `internal/screen`

**Audit recommendation:** evaluate replacing `lufis-terminal/internal/screen` with a `libvterm` binding.

**Recommendation:** **proceed in a follow-up PR, but only behind a feature flag.** The current screen layer has real, demonstrable gaps that hurt every modern TUI (vim, k9s, lazygit, btop). `libvterm` closes them. Cost is real but bounded — cgo is already in the build via GLFW, so the new constraint is build-prereqs (`libvterm-dev`), not toolchain shape.

---

## Where the current implementation stands

`lufis-terminal/internal/screen` is 1102 lines:

| file | lines | role |
| --- | --- | --- |
| `buffer.go` | 551 | grid, cursor, scroll regions, ~50 ops (CUP/SGR/EL/ED/IL/DL/DCH/IND/RI/DECSC/DECRC…) |
| `cell.go` | 137 | `Cell`, `Attr` bitfield, `Color` (16+256+truecolor) |
| `scrollback.go` | 55 | ring buffer |
| `buffer_test.go` | 359 | grid ops, scroll regions, SGR, attribute stacking |

It is paired with `internal/ansi/parser.go` (491 lines). Together: ~1600 lines of terminal emulation that we own and maintain.

### Feature gaps in the current implementation

`grep` for the standard "modern terminal" features turns up nothing:

- **Alternate screen (DECSET 1049 / smcup / rmcup)** — vim, less, k9s, lazygit, btop, htop all rely on this. Without it, exiting these apps does *not* restore the prior screen state; the user sees their app's exit-frame frozen in their scrollback.
- **OSC 8 hyperlinks** — `gh`, `tig`, `lazygit` emit clickable links. We render the escape literally.
- **Sixel / Kitty image protocol** — `imgcat`, `chafa`, `viu`, modern `kitten` inline previews. Not supported.
- **Bracketed paste** — modern shells (zsh, fish, nushell) use this to detect pasted vs typed input. We don't advertise it.
- **OSC 52 clipboard** — what tmux + ssh use to push clipboard through a remote connection. Not supported.

These aren't theoretical — anyone who lives in k9s or lazygit *will* notice on first use.

## `go-libvterm` analysis

[`go-libvterm`](https://github.com/sgielen/go-libvterm) (or the [neovim fork](https://github.com/neovim/go-vterm)) is a cgo binding to [`libvterm`](http://www.leonerd.org.uk/code/libvterm/), the same library powering neovim's `:terminal`.

What we get for free:
- Complete VT100/xterm/DEC parser (deletes `internal/ansi`).
- Full screen buffer with alternate screen, scrollback, attributes, scroll regions.
- OSC 8, OSC 52, bracketed paste, focus events.
- Mouse protocol (SGR / 1006 / 1015 / x10).
- Sixel out of the box; Kitty image protocol via patched libvterm 0.3.

API shape:
```go
vt := vterm.New(rows, cols)
vt.SetUTF8(true)
screen := vt.ObtainScreen()
screen.Reset(true)
screen.SetDamageMerge(vterm.DamageScroll)
// then feed bytes from the PTY:
vt.Write(ptyBytes)
// and drain damage events to repaint:
for _, rect := range screen.GetDamage() { ... }
```

This is a clean event-driven model that maps well to our renderer's existing "dirty region → repaint" loop.

### Costs

- **cgo is already a hard requirement** via `go-gl/glfw`. Adding `libvterm` does not regress this — but it does add a system-library prereq.
  - macOS: `brew install libvterm` (already a transitive dep of neovim).
  - Linux: `libvterm-dev` (apt) / `libvterm` (pacman) / `libvterm-devel` (dnf).
  - Windows: requires building libvterm from source or shipping a prebuilt DLL — this is the real friction point. Neovim solves it by vendoring; we'd need to do the same.
- **CI build matrix grows.** We'd need to provision libvterm on each runner. macOS+Linux is trivial; Windows costs a one-time afternoon.
- **API translation layer.** Our renderer reads cells from `*screen.Buffer`. Need a thin shim mapping `screen.Cell` ↔ `vterm.ScreenCell`. ~150 lines.
- **Lock-in.** libvterm has its own opinions (e.g. how it merges damage). We lose the ability to inject custom dispatch behavior — most existing tweaks would need to move into pre/post hooks around libvterm.

### Code that deletes

| file | lines | fate |
| --- | --- | --- |
| `internal/ansi/parser.go` | 491 | delete |
| `internal/ansi/parser_test.go` | 350 | delete |
| `internal/screen/buffer.go` | 551 | shrink to ~150 (shim) |
| `internal/screen/cell.go` | 137 | shrink to ~50 (types only) |
| `internal/screen/scrollback.go` | 55 | delete (libvterm handles it) |
| `internal/screen/buffer_test.go` | 359 | shrink to shim tests, ~100 lines |

Net: **−1600 lines maintained code**, **+150 lines shim**, **+1 cgo system dep**.

## Recommendation

1. **Open PR-7a (separate, this survey is the artefact of PR-7).** Add `internal/screen/vterm` as a sibling package with its own constructor and shim layer. Both backends compile.
2. **Gate at runtime** with `ARGUS_SCREEN_BACKEND=libvterm` (default: existing). One week of internal dogfooding to surface API gaps the shim missed.
3. **Flip the default once** alternate-screen + OSC 8 + bracketed paste are verified against the matrix below.
4. **Delete the legacy backend** after one stable release.

### Verification matrix (must pass before flipping default)

| app | scenario | expected after fix |
| --- | --- | --- |
| `vim` | open → quit | original shell prompt restored, no vim residue in scrollback |
| `k9s` | open → ctrl+c | original shell prompt restored |
| `lazygit` | open → q | original shell prompt restored |
| `gh pr view 1` | output with OSC 8 | links rendered as actual clickable hyperlinks (xterm-style) |
| `zsh` | paste multi-line | bracketed-paste markers prevent autorun until Enter |
| `imgcat foo.png` | inline image | image renders inline (Kitty protocol) instead of escape garbage |
| `tmux ssh remote` | copy to clipboard with OSC 52 | clipboard receives the text |

### What this PR delivers

- This document.
- A 2-line note in `docs/lufis-terminal-audit-plan.md` linking to it, so future agents reading the plan find the conclusion.

The implementation work is intentionally **not** in this PR. The audit asked for an evaluation; the evaluation says "yes, but stage it." A 2000-line, multi-week change should not be bundled with a documentation deliverable.
