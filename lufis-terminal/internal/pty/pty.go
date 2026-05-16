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

type Terminal struct {
	cmd      *exec.Cmd
	ptmx     *os.File
	mu       sync.Mutex
	closed   bool
	logger   *slog.Logger
	OnOutput func(data string)
}

func New(logger *slog.Logger) *Terminal {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Terminal{logger: logger}
}

func (t *Terminal) Start(shell string, rows, cols uint16) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	cmd := exec.Command(shell, "-l")
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: rows, Cols: cols})
	if err != nil {
		return fmt.Errorf("pty start: %w", err)
	}

	t.cmd = cmd
	t.ptmx = ptmx

	go t.readLoop(ptmx)

	return nil
}

func (t *Terminal) Write(data string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed || t.ptmx == nil {
		return nil
	}

	_, err := t.ptmx.Write([]byte(data))
	if err != nil {
		return fmt.Errorf("pty write: %w", err)
	}
	return nil
}

func (t *Terminal) Resize(rows, cols uint16) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.ptmx == nil {
		return nil
	}

	if err := pty.Setsize(t.ptmx, &pty.Winsize{Rows: rows, Cols: cols}); err != nil {
		return fmt.Errorf("pty resize: %w", err)
	}
	return nil
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

	var errs []error

	if t.ptmx != nil {
		if err := t.ptmx.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if t.cmd != nil && t.cmd.Process != nil {
		if err := t.cmd.Process.Kill(); err != nil {
			errs = append(errs, err)
		}
		if err := t.cmd.Wait(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (t *Terminal) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return !t.closed && t.cmd != nil && t.cmd.Process != nil
}

func (t *Terminal) readLoop(ptmx *os.File) {
	buf := make([]byte, 8192)
	for {
		n, err := ptmx.Read(buf)
		if err != nil {
			if err != io.EOF {
				t.logger.Debug("pty read error", "error", err)
			}
			return
		}
		if n > 0 && t.OnOutput != nil {
			t.OnOutput(string(buf[:n]))
		}
	}
}
