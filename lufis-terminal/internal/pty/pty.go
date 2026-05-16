// Package pty wraps the shared github.com/argues/argus-pty PTY core
// in the Terminal type lufis-terminal's render layer already imports.
// The actual PTY lifecycle, error wrapping, restart semantics, and
// read loop live in the shared package — see docs/lufis-terminal-
// audit-plan.md PR-3.
package pty

import (
	"log/slog"

	sharedpty "github.com/argues/argus-pty"
)

// Terminal is lufis's per-window PTY surface. The underlying lifecycle
// is the shared sharedpty.PTY; this wrapper preserves the Terminal /
// Terminal.OnOutput field-style API the GLFW + ansi parser callers
// have always used.
type Terminal struct {
	pty *sharedpty.PTY

	// OnOutput is the per-Terminal callback assigned by the render
	// layer. Kept as a field (rather than a setter) so the existing
	// `term.OnOutput = func(...) { ... }` callsites compile unchanged.
	OnOutput func(data string)
}

// New constructs a Terminal. A nil logger is accepted.
func New(logger *slog.Logger) *Terminal {
	t := &Terminal{pty: sharedpty.New(logger)}
	t.pty.OnOutput = func(data string) {
		if t.OnOutput != nil {
			t.OnOutput(data)
		}
	}
	return t
}

// Start spawns the shell behind a fresh PTY. The shell argument can
// be empty — the shared core falls back to $SHELL then "zsh".
func (t *Terminal) Start(shell string, rows, cols uint16) error {
	return t.pty.Start(sharedpty.Options{
		Shell:      shell,
		Rows:       rows,
		Cols:       cols,
		LoginShell: true,
	})
}

// Write sends raw bytes to the PTY (keystrokes).
func (t *Terminal) Write(data string) error { return t.pty.Write(data) }

// Resize updates the PTY's window dimensions.
func (t *Terminal) Resize(rows, cols uint16) error { return t.pty.Resize(rows, cols) }

// Close tears down the PTY. Idempotent.
func (t *Terminal) Close() error { return t.pty.Close() }

// IsRunning reports whether the PTY is up.
func (t *Terminal) IsRunning() bool { return t.pty.IsRunning() }
