// Package terminal is the Wails-side terminal session implementation.
// It owns per-session state (session manager, OnOutput wiring to
// xterm.js over the Wails bridge) but the PTY lifecycle itself is
// provided by the shared github.com/argues/argus-pty package. lufis-
// terminal uses the same shared core — see docs/lufis-terminal-audit-
// plan.md, PR-3, for the extraction rationale and the duplication it
// removed.
package terminal

import (
	"log/slog"

	sharedpty "github.com/argues/argus-pty"
)

// Terminal wraps the shared PTY core with the Wails-side type the
// rest of the backend already imports. The wrapper is a one-line
// indirection — every method delegates to the embedded *sharedpty.PTY
// — so existing callers continue to work without source changes.
//
// OnOutput is exposed as a Go field (rather than a setter) to
// preserve the existing assignment-style API; the field write
// flows through to the embedded PTY via setOnOutput.
type Terminal struct {
	pty *sharedpty.PTY

	// OnOutput is the per-Terminal callback. We don't expose the
	// embedded PTY directly because callers used to assign to
	// `term.OnOutput = …` against the local struct — keeping the
	// field at this layer means that code keeps compiling.
	OnOutput func(data string)
}

// New constructs a Terminal. nil logger is accepted (delegates to the
// shared PTY which substitutes the discard handler).
func New(logger *slog.Logger) *Terminal {
	t := &Terminal{pty: sharedpty.New(logger)}
	t.pty.OnOutput = func(data string) {
		if t.OnOutput != nil {
			t.OnOutput(data)
		}
	}
	return t
}

// Start spawns the shell using the legacy two-positional-args + login
// shell defaults. Delegates to the shared PTY.
func (t *Terminal) Start(shell string, rows, cols uint16) error {
	return t.pty.Start(sharedpty.Options{
		Shell:      shell,
		Rows:       rows,
		Cols:       cols,
		LoginShell: true,
	})
}

// StartWithEnv is the kube/backend-specific entrypoint that overlays
// extraEnv onto the shell's environment. The shared PTY supports this
// natively via Options.ExtraEnv; the wrapper just preserves the
// existing signature.
func (t *Terminal) StartWithEnv(shell string, rows, cols uint16, extraEnv []string) error {
	return t.pty.Start(sharedpty.Options{
		Shell:      shell,
		Rows:       rows,
		Cols:       cols,
		ExtraEnv:   extraEnv,
		LoginShell: true,
	})
}

// Write proxies the input down to the shared PTY.
func (t *Terminal) Write(data string) error { return t.pty.Write(data) }

// Resize proxies the dimension update.
func (t *Terminal) Resize(rows, cols uint16) error { return t.pty.Resize(rows, cols) }

// Close tears down the PTY and reaps the child. Idempotent.
func (t *Terminal) Close() error { return t.pty.Close() }

// IsRunning reports whether the PTY is up.
func (t *Terminal) IsRunning() bool { return t.pty.IsRunning() }
