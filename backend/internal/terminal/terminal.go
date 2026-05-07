package terminal

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Terminal manages a PTY session for the embedded shell.
type Terminal struct {
	cmd    *exec.Cmd
	ptmx   *os.File
	mu     sync.Mutex
	closed bool
	logger *slog.Logger

	// OnOutput is called with each chunk of terminal output.
	OnOutput func(data string)
}

// New creates a new Terminal instance.
func New(logger *slog.Logger) *Terminal {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Terminal{
		logger: logger,
	}
}

// Start opens a PTY with the given shell and dimensions.
// Common shells: "zsh", "bash", "sh".
func (t *Terminal) Start(shell string, rows, cols uint16) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ptmx != nil {
		// Already running — close existing.
		t.closeLocked()
	}

	if shell == "" {
		// Try to detect the user's shell.
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "zsh"
		}
	}

	cmd := exec.Command(shell, "-l")
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
	if err != nil {
		return err
	}

	t.cmd = cmd
	t.ptmx = ptmx
	t.closed = false

	t.logger.Info("terminal started",
		slog.String("shell", shell),
		slog.Int("rows", int(rows)),
		slog.Int("cols", int(cols)),
	)

	// Read output in background.
	go t.readLoop()

	return nil
}

// Write sends raw input data to the terminal (keystrokes from xterm.js).
func (t *Terminal) Write(data string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ptmx == nil || t.closed {
		return nil
	}

	_, err := t.ptmx.WriteString(data)
	return err
}

// Resize updates the terminal dimensions.
func (t *Terminal) Resize(rows, cols uint16) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ptmx == nil || t.closed {
		return nil
	}

	return pty.Setsize(t.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
}

// Close shuts down the terminal.
func (t *Terminal) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closeLocked()
}

func (t *Terminal) closeLocked() error {
	if t.closed {
		return nil
	}
	t.closed = true

	if t.ptmx != nil {
		t.ptmx.Close()
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait() //nolint:errcheck
	}

	t.logger.Info("terminal closed")
	return nil
}

// readLoop reads from the PTY and calls OnOutput.
func (t *Terminal) readLoop() {
	buf := make([]byte, 8192)
	for {
		n, err := t.ptmx.Read(buf)
		if n > 0 && t.OnOutput != nil {
			t.OnOutput(string(buf[:n]))
		}
		if err != nil {
			if err != io.EOF {
				t.logger.Debug("terminal read error", slog.String("error", err.Error()))
			}
			return
		}
	}
}

// IsRunning returns whether the terminal is active.
func (t *Terminal) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ptmx != nil && !t.closed
}
