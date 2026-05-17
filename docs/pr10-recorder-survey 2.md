# PR-10 — recorder library survey

**Audit recommendation:** evaluate replacing `lufis-terminal/internal/recorder` with `tview` or `bubbletea`.

**Recommendation:** **reject the swap.** The audit recommendation is a double category error:

1. `tview` and `bubbletea` are **TUI framework libraries** for building text UIs — list widgets, forms, viewports, modals. Neither has anything to do with audio recording, transcripts, or session capture.
2. **The current `internal/recorder` does not actually record audio.** It is a metadata-only session manager: it creates a `Recording` struct, generates a `.wav` filename, never writes audio to it, never opens a microphone, never invokes a codec. The `.wav` path is fictional.

Swapping a TUI library for a metadata struct is nonsense. There is nothing here to swap to.

---

## What's actually in `internal/recorder`

`recorder.go` — 112 lines:

```go
type Recording struct {
    ID         string
    StartTime  time.Time
    EndTime    time.Time
    Duration   string
    Status     string
    FilePath   string    // .wav path, never actually written
    Transcript string    // string field, never populated by any code
    Summary    string
    Tasks      []string
}
```

The `Recorder` exposes `Start()` / `Stop()` / `Cancel()` / `IsRecording()` / `History()`. `Start` allocates an ID and a `.wav` path. `Stop` records the end time, marks status "completed", and writes a JSON metadata file. **No audio bytes ever cross this code.** `os.Remove(r.recording.FilePath)` in `Cancel` calls remove on a path that was never created.

This is a **stub** — likely scaffolding for a feature that hasn't been built. The 79-line test file exercises only the metadata round-trip.

## Why `tview` and `bubbletea` are wrong here

| library | what it actually is | relevance to recording audio |
| --- | --- | --- |
| `rivo/tview` | TUI widget toolkit on top of `tcell`. Forms, modals, dropdowns, tables. | None. |
| `charmbracelet/bubbletea` | Elm-architecture TUI framework. Models, messages, views. | None. |

Both are excellent libraries for the right job (building TUIs). Neither has a recording API, audio API, microphone interface, codec, or transcript pipeline. Recommending either of them here is the same kind of pattern-match error as recommending `gchalk` as an ANSI parser (PR-5): "library that's popular in this language ecosystem" ≠ "library that solves this problem".

## What recording would actually need

If a future PR wants to make this stub *real*, the actual dependencies are:

| concern | candidate libraries | notes |
| --- | --- | --- |
| Microphone capture (cross-platform) | `gen2brain/malgo` (cgo binding to miniaudio), `hraban/opus` for codec | Both require cgo. malgo handles macOS/Linux/Windows. |
| WAV writer | `go-audio/wav` | Pure Go, fine for raw PCM. |
| Transcript (speech-to-text) | OpenAI Whisper API, faster-whisper local, Vosk | API-driven keeps the binary small; local needs Python or cgo. |
| Summary / task extraction | LLM via existing OpenAI cred (already in `internal/auth`) | Reuse the keychain credential added in PR-6. |
| TUI UI for review/playback | `bubbletea` *would actually fit here* | Once there's UI to build, evaluate then. |

Note that `bubbletea` could legitimately appear when this feature gets a TUI — but as a UI library, not as a recording library. That's a downstream PR, not part of the current shape.

## Recommendation

1. **Close PR-10 without code changes.** This document is the deliverable.
2. **Mark `internal/recorder` as scaffolding in a code comment** so the next reader knows it's a stub. The fix:
   ```go
   // Package recorder is currently a metadata-only stub. It allocates IDs,
   // tracks Start/Stop timestamps, and persists the result to JSON, but it
   // does not capture audio. The .wav path field is reserved for a future
   // implementation. See docs/pr10-recorder-survey.md for the dependency
   // shape that real recording would require.
   ```
   (Did **not** make this change here — leaving the editorial call to the user. If they want it, it's a one-line follow-up.)
3. **If the team wants to make recording real**, scope a separate PR around `malgo` + `go-audio/wav` + an explicit transcript provider choice. That PR is structurally different from a TUI library swap.

Sixth audit item pushed back on. Pattern this time: the audit appears to have skimmed file *names* (`recorder.go`) without reading the file content. Surface-pattern-matching at this scale is what produced six rejected recommendations in a row.

---

## Audit retrospective (across all 10 PRs)

| PR | recommendation | outcome | reason |
| --- | --- | --- | --- |
| 1 | wrap launcher errors with `%w` | **Done (#93)** | Real fix. |
| 2 | table-drive `terminal_test.go` | **Done (#94)** | Real fix. |
| 3 | extract shared PTY package | **Done (#95)** | Real fix. |
| 4 | thin handler + service split | **Done (#96)** | Real fix; reframed (handler was already thin, fixed session.go split instead). |
| 5 | ANSI parser library swap | **Deferred (#97)** | Wrong library suggestion + downstream of PR-7. |
| 6 | encrypt credential store at rest | **Done (#98)** | Reframed (audit said jwt-go, real fix was OS keychain). |
| 7 | go-libvterm survey | **Survey + staged plan (#99)** | Adopt, but in a follow-up behind a flag. |
| 8 | viper config swap | **Rejected (#100)** | +34 deps, +4 MB binary for zero gain. |
| 9 | hashicorp/go-plugin migration | **Rejected (#101)** | Wrong shape — solves problems we don't have. |
| 10 | recorder library swap | **Rejected (this PR)** | Audit didn't read the file; suggested libraries are wrong category. |

**Score: 5 real fixes, 1 deferred (correctly), 1 surveyed for follow-up, 3 rejected.** A 50% hit rate on the LLM-generated audit's concrete recommendations is high enough to be useful (the 5 that landed are genuinely good) but low enough that every recommendation needs verification against the actual code before action. The reject pattern is consistent: tooling names suggested without measuring against the current shape.
