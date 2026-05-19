// Package pty is the shared PTY-lifecycle core used by both the
// kube/backend (Wails-side terminal sessions over xterm.js) and the
// lufis-terminal (native GLFW window with its own PTY surface). Both
// modules used to maintain near-identical copies of this code — see
// the lufis audit plan, PR-3.
//
// The public type is PTY rather than Terminal so callers' existing
// Terminal types (which keep their richer per-side surfaces) can
// embed *PTY without name clash.
//
// Threading model:
//   - The mutex guards every method call. PTY data flow is bursty
//     (a screen refresh writes hundreds of bytes), so we keep the
//     lock granularity coarse and accept that Write/Resize wait
//     behind a running readLoop sample.
//   - readLoop receives ptmx as an argument (rather than reading
//     p.ptmx) so a subsequent Start() that swaps p.ptmx does not
//     race with the running goroutine.
package pty

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// DefaultBufferSize is the read-buffer capacity used by the readLoop
// when StartOptions.BufferBytes is zero. 64KiB matches what
// kube/backend's old terminal package used; lufis defaulted to 8KiB
// historically and saw no perf wins below this size on macOS, so the
// merged default lands at 64KiB.
const DefaultBufferSize = 64 * 1024

// DefaultShell is what Start uses when the caller passes "" and the
// SHELL env isn't set either. Matches the legacy behavior of both
// callsites.
const DefaultShell = "zsh"

// Options configures a single Start call. Zero-value Options is
// valid: Start picks DefaultShell, no extra env, DefaultBufferSize.
type Options struct {
	// Shell is the executable to launch (e.g. "zsh", "/bin/bash").
	// "" means "use $SHELL, falling back to DefaultShell".
	Shell string

	// Rows / Cols are the initial window size of the PTY. Both must
	// be > 0; (0, 0) is accepted (PTY libraries pick a sane default)
	// but most callers should set them.
	Rows uint16
	Cols uint16

	// ExtraEnv is appended to os.Environ() before the shell starts.
	// Always overlaid AFTER TERM/COLORTERM so callers can override
	// either if they really want to.
	ExtraEnv []string

	// BufferBytes overrides the readLoop buffer size. <= 0 means
	// DefaultBufferSize.
	BufferBytes int

	// LoginShell appends "-l" to the shell args. Both legacy callsites
	// had this on unconditionally; we keep the same default (true) but
	// expose it for tests and future non-login uses.
	LoginShell bool
}

// PTY is the shared lifecycle. Embed it in your Terminal type and
// expose the methods you want; or call it directly.
type PTY struct {
	logger *slog.Logger

	mu     sync.Mutex
	cmd    *exec.Cmd
	ptmx   *os.File
	closed bool

	// done is closed by closeLocked to signal the readLoop goroutine
	// to exit. A new channel is created each Start() so each
	// incarnation gets its own cancellation signal.
	done chan struct{}

	// OnOutput is the callback the readLoop fires for each chunk of
	// data the PTY produces. Safe to set before Start; ignored when
	// nil. Implementations should not block — readLoop runs single-
	// threaded and a slow callback throttles the shell.
	OnOutput func(data string)
}

// New returns a fresh PTY. nil logger is accepted (substitutes the
// discard handler) so test code stays terse.
func New(logger *slog.Logger) *PTY {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &PTY{logger: logger}
}

// Start spawns a shell behind a fresh PTY. If a PTY is already
// running it is closed first — both legacy callsites supported
// "restart by calling Start again." Returns the wrapped pty.Start
// error on failure; nil on success.
func (p *PTY) Start(opts Options) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Allow restart: close any previous PTY first. The lufis copy
	// used to refuse Start when closed=true (treating Close as
	// permanent); the argus copy allowed restart by clearing the
	// flag. The shared core picks the more flexible argus semantics.
	if p.ptmx != nil {
		_ = p.closeLocked()
	}

	p.done = make(chan struct{})

	shell := opts.Shell
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = DefaultShell
		}
	}

	args := []string(nil)
	if opts.LoginShell {
		args = []string{"-l"}
	}
	cmd := exec.Command(shell, args...)
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)
	cmd.Env = append(cmd.Env, opts.ExtraEnv...)

	rows, cols := opts.Rows, opts.Cols
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return fmt.Errorf("pty start (%s): %w", shell, err)
	}

	p.cmd = cmd
	p.ptmx = ptmx
	p.closed = false

	p.logger.Info("pty started",
		slog.String("shell", shell),
		slog.Int("rows", int(rows)),
		slog.Int("cols", int(cols)),
	)

	bufSize := opts.BufferBytes
	if bufSize <= 0 {
		bufSize = DefaultBufferSize
	}
	go p.readLoop(ptmx, p.done, bufSize)

	return nil
}

// Write sends raw bytes to the PTY (keystrokes). Returns nil when
// the PTY isn't running — keeps callers (network handlers, UI event
// loops) from having to nil-check on every input. Wraps the underlying
// write error with a "pty write:" prefix so logs identify the source.
func (p *PTY) Write(data string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ptmx == nil || p.closed {
		return nil
	}

	if _, err := p.ptmx.WriteString(data); err != nil {
		return fmt.Errorf("pty write: %w", err)
	}
	return nil
}

// Resize updates the PTY's window dimensions. Returns nil when the
// PTY isn't running (same rationale as Write).
func (p *PTY) Resize(rows, cols uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ptmx == nil || p.closed {
		return nil
	}

	if err := pty.Setsize(p.ptmx, &pty.Winsize{Rows: rows, Cols: cols}); err != nil {
		return fmt.Errorf("pty resize: %w", err)
	}
	return nil
}

// Close releases the PTY and reaps the child process. Idempotent —
// subsequent Close calls return nil. Aggregates ptmx-close and
// process-kill errors via errors.Join so a single failure doesn't
// mask the other.
func (p *PTY) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeLocked()
}

func (p *PTY) closeLocked() error {
	if p.closed {
		return nil
	}
	p.closed = true

	// Signal the readLoop goroutine to exit.
	if p.done != nil {
		close(p.done)
	}

	var errs []error
	if p.ptmx != nil {
		if err := p.ptmx.Close(); err != nil {
			errs = append(errs, fmt.Errorf("pty close: %w", err))
		}
	}
	if p.cmd != nil && p.cmd.Process != nil {
		if err := p.cmd.Process.Kill(); err != nil {
			// "process already finished" is the common case here;
			// surface it for debugging but don't fail.
			errs = append(errs, fmt.Errorf("pty kill: %w", err))
		}
		// Wait collects the process state so the OS frees the entry.
		// The "signal: killed" exit we just sent is expected; drop it.
		_ = p.cmd.Wait()
	}

	p.logger.Info("pty closed")
	return errors.Join(errs...)
}

// IsRunning reports whether a PTY is active. Returns false before
// Start, true after a successful Start, false after Close.
func (p *PTY) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ptmx != nil && !p.closed && p.cmd != nil && p.cmd.Process != nil
}

// readLoop pulls data off the PTY until it closes or the done
// channel is signalled. ptmx is passed in rather than read off
// p.ptmx so a future Start() that swaps the field can't race the
// goroutine. done is closed by closeLocked to cancel the loop.
// bufSize is the read buffer size.
func (p *PTY) readLoop(ptmx *os.File, done <-chan struct{}, bufSize int) {
	buf := make([]byte, bufSize)
	for {
		select {
		case <-done:
			return
		default:
		}
		n, err := ptmx.Read(buf)
		if n > 0 && p.OnOutput != nil {
			p.OnOutput(string(buf[:n]))
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				p.logger.Debug("pty read error", slog.String("error", err.Error()))
			}
			return
		}
	}
}
