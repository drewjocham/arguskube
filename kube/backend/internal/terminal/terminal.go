package terminal

import (
	"io"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

type Terminal struct {
	cmd    *exec.Cmd
	ptmx   *os.File
	mu     sync.Mutex
	closed bool
	logger *slog.Logger

	OnOutput func(data string)
}

func New(logger *slog.Logger) *Terminal {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Terminal{
		logger: logger,
	}
}

func (t *Terminal) Start(shell string, rows, cols uint16) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ptmx != nil {
		_ = t.closeLocked()
	}

	if shell == "" {
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

	// Read output in background. Pass ptmx explicitly so a subsequent
	// Start() reassigning t.ptmx does not race with this goroutine.
	go t.readLoop(ptmx)

	return nil
}

// StartWithEnv starts a PTY session with extra environment variables.
func (t *Terminal) StartWithEnv(shell string, rows, cols uint16, extraEnv []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ptmx != nil {
		_ = t.closeLocked()
	}

	if shell == "" {
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
	cmd.Env = append(cmd.Env, extraEnv...)

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

	go t.readLoop(ptmx)

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
		_ = t.cmd.Process.Kill()
		_ = t.cmd.Wait()
	}

	t.logger.Info("terminal closed")
	return nil
}

// readLoop reads from the given PTY file and calls OnOutput. The file is
// passed in (rather than read off t.ptmx) so a later Start() that swaps
// t.ptmx cannot race with this goroutine.
func (t *Terminal) readLoop(ptmx *os.File) {
	buf := make([]byte, 65536) // 64KB
	for {
		n, err := ptmx.Read(buf)
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

func (t *Terminal) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ptmx != nil && !t.closed
}
