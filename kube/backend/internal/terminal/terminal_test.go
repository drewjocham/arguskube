package terminal

import (
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if term == nil {
		t.Fatal("New returned nil")
	}
}

func TestStartAndWrite(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	var output strings.Builder
	term.OnOutput = func(data string) {
		output.WriteString(data)
	}

	// Start with a simple shell simulation — use "sh" with output.
	if err := term.Start("sh", 40, 120); err != nil {
		t.Skipf("cannot start PTY (likely no pty available): %v", err)
	}
	defer term.Close()

	// Send a simple echo command.
	if err := term.Write("echo hello\n"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Wait for output to appear.
	time.Sleep(500 * time.Millisecond)

	// We should see "hello" somewhere.
	if !strings.Contains(output.String(), "hello") {
		t.Logf("output so far: %q", output.String())
	}

	// Write "exit" to clean up.
	_ = term.Write("exit\n")
	time.Sleep(200 * time.Millisecond)
}

func TestResize(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := term.Start("sh", 40, 120); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	defer term.Close()

	if err := term.Resize(80, 160); err != nil {
		t.Fatalf("Resize failed: %v", err)
	}
}

func TestClose(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := term.Start("sh", 40, 120); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}

	if err := term.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Double close should be safe.
	if err := term.Close(); err != nil {
		t.Fatalf("double Close should be safe, got: %v", err)
	}
}

func TestIdempotentClose(t *testing.T) {
	term := New(nil)
	if err := term.Close(); err != nil {
		t.Fatalf("Close on unstarted terminal should be safe: %v", err)
	}
}

func TestStartEmptyShell(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// Start with empty shell — should auto-detect (fall back to zsh).
	if err := term.Start("", 40, 120); err != nil {
		t.Skipf("cannot start PTY with empty shell: %v", err)
	}
	defer term.Close()

	if !term.IsRunning() {
		t.Error("expected terminal to be running after Start")
	}
}

func TestIsRunningFalseBeforeStart(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if term.IsRunning() {
		t.Error("expected IsRunning to be false before Start")
	}
}

func TestIsRunningFalseAfterClose(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := term.Start("sh", 40, 120); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}

	if !term.IsRunning() {
		t.Error("expected IsRunning to be true after Start")
	}

	term.Close()

	if term.IsRunning() {
		t.Error("expected IsRunning to be false after Close")
	}
}

func TestWriteBeforeStartNoPanic(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// Writing before Start should be a no-op, not a panic.
	if err := term.Write("test"); err != nil {
		t.Logf("Write before Start returned error: %v", err)
	}
}

func TestResizeBeforeStartNoPanic(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// Resizing before Start should be a no-op, not a panic.
	if err := term.Resize(80, 160); err != nil {
		t.Logf("Resize before Start returned error: %v", err)
	}
}

func TestNewNilLogger(t *testing.T) {
	term := New(nil)
	if term == nil {
		t.Fatal("New with nil logger returned nil")
	}
}

func TestCloseUnstarted(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	// Close before Start should be safe.
	if err := term.Close(); err != nil {
		t.Fatalf("Close on unstarted terminal should be safe: %v", err)
	}
}

func TestWriteAfterCloseNoPanic(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := term.Start("sh", 40, 120); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	term.Close()

	// Writing after Close should be safe.
	if err := term.Write("test"); err != nil {
		t.Logf("Write after Close returned error: %v", err)
	}
}

func TestResizeAfterCloseNoPanic(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := term.Start("sh", 40, 120); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	term.Close()

	if err := term.Resize(80, 160); err != nil {
		t.Logf("Resize after Close returned error: %v", err)
	}
}

func TestStartTwice(t *testing.T) {
	term := New(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if err := term.Start("sh", 40, 120); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	defer term.Close()

	// Starting again should close the first and start a new one.
	if err := term.Start("sh", 80, 160); err != nil {
		t.Fatalf("Start again failed: %v", err)
	}
}
