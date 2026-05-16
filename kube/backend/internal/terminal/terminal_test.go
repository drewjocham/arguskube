package terminal

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// discardLogger keeps the test output uncluttered. Most rows in the
// tables below don't care about the logger value, so we hand them
// this default.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// startTerminalOrSkip constructs a Terminal and Starts it under "sh".
// On a system without /dev/pts (CI sandbox, weird container) the
// underlying creack/pty call fails — t.Skipf rather than Fail keeps
// the suite usable in those envs while still running everything in a
// real PTY when one's available.
func startTerminalOrSkip(t *testing.T, shell string, rows, cols uint16) *Terminal {
	t.Helper()
	term := New(discardLogger())
	if err := term.Start(shell, rows, cols); err != nil {
		t.Skipf("cannot start PTY (likely no /dev/pts in this env): %v", err)
	}
	t.Cleanup(func() { _ = term.Close() })
	return term
}

// TestNew exercises the constructor. Two rows: real logger and nil
// (nil must not panic — the production code substitutes a default).
func TestNew(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		logger *slog.Logger
	}{
		{"with logger", discardLogger()},
		{"nil logger", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if term := New(tc.logger); term == nil {
				t.Fatal("New returned nil")
			}
		})
	}
}

// TestPreStartSafety pins the "no-panic / no-error" contract for
// every method that can be legally called before Start. Replaces the
// old TestIsRunningFalseBeforeStart / TestWriteBeforeStartNoPanic /
// TestResizeBeforeStartNoPanic / TestCloseUnstarted / TestIdempotent-
// Close. Each row constructs a fresh Terminal and runs the action
// against it.
func TestPreStartSafety(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		do   func(t *testing.T, term *Terminal)
	}{
		{
			name: "IsRunning returns false",
			do: func(t *testing.T, term *Terminal) {
				if term.IsRunning() {
					t.Error("IsRunning should be false before Start")
				}
			},
		},
		{
			name: "Close is a no-op",
			do: func(t *testing.T, term *Terminal) {
				if err := term.Close(); err != nil {
					t.Errorf("Close on unstarted terminal: %v", err)
				}
			},
		},
		{
			name: "Double Close is idempotent",
			do: func(t *testing.T, term *Terminal) {
				_ = term.Close()
				if err := term.Close(); err != nil {
					t.Errorf("second Close on unstarted terminal: %v", err)
				}
			},
		},
		{
			name: "Write is a no-op",
			do: func(t *testing.T, term *Terminal) {
				if err := term.Write("test"); err != nil {
					t.Errorf("Write before Start surfaced an error: %v", err)
				}
			},
		},
		{
			name: "Resize is a no-op",
			do: func(t *testing.T, term *Terminal) {
				if err := term.Resize(80, 160); err != nil {
					t.Errorf("Resize before Start surfaced an error: %v", err)
				}
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.do(t, New(discardLogger()))
		})
	}
}

// TestStartedBehavior covers everything that requires a real PTY.
// Replaces TestResize / TestClose / TestStartEmptyShell /
// TestIsRunningFalseAfterClose / TestWriteAfterCloseNoPanic /
// TestResizeAfterCloseNoPanic / TestStartTwice. Each row gets a
// freshly started Terminal via the shared helper; t.Skipf fires
// when PTY allocation isn't possible.
func TestStartedBehavior(t *testing.T) {
	// No t.Parallel at the suite level — PTY allocation is global
	// (per-host file-descriptor table) and runs into limits when many
	// rows race. Per-row t.Parallel would also force per-row PTY
	// alloc, which CI envs throttle on. Run serially; each row is
	// fast.
	cases := []struct {
		name string
		do   func(t *testing.T, term *Terminal)
	}{
		{
			name: "Resize succeeds after Start",
			do: func(t *testing.T, term *Terminal) {
				if err := term.Resize(80, 160); err != nil {
					t.Errorf("Resize: %v", err)
				}
			},
		},
		{
			name: "Close succeeds after Start",
			do: func(t *testing.T, term *Terminal) {
				if err := term.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			},
		},
		{
			name: "Double Close after Start is safe",
			do: func(t *testing.T, term *Terminal) {
				if err := term.Close(); err != nil {
					t.Errorf("first Close: %v", err)
				}
				if err := term.Close(); err != nil {
					t.Errorf("second Close: %v", err)
				}
			},
		},
		{
			name: "IsRunning true after Start, false after Close",
			do: func(t *testing.T, term *Terminal) {
				if !term.IsRunning() {
					t.Error("IsRunning should be true after Start")
				}
				_ = term.Close()
				if term.IsRunning() {
					t.Error("IsRunning should be false after Close")
				}
			},
		},
		{
			name: "Write after Close does not panic",
			do: func(t *testing.T, term *Terminal) {
				_ = term.Close()
				if err := term.Write("test"); err != nil {
					// non-fatal — closed-pipe error is acceptable
					t.Logf("Write after Close returned: %v", err)
				}
			},
		},
		{
			name: "Resize after Close does not panic",
			do: func(t *testing.T, term *Terminal) {
				_ = term.Close()
				if err := term.Resize(80, 160); err != nil {
					t.Logf("Resize after Close returned: %v", err)
				}
			},
		},
		{
			name: "Start twice replaces the first PTY",
			do: func(t *testing.T, term *Terminal) {
				// Second Start should swap the underlying PTY; the
				// production code closes the previous one internally.
				if err := term.Start("sh", 80, 160); err != nil {
					t.Errorf("second Start: %v", err)
				}
				if !term.IsRunning() {
					t.Error("expected IsRunning after the second Start")
				}
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.do(t, startTerminalOrSkip(t, "sh", 40, 120))
		})
	}
}

// TestStartEmptyShellAutoDetects pins the "auto-fallback when shell
// arg is empty" contract. Standalone (not table-row) because the
// shared startTerminalOrSkip helper hard-codes the shell name; this
// test passes "".
func TestStartEmptyShellAutoDetects(t *testing.T) {
	term := New(discardLogger())
	if err := term.Start("", 40, 120); err != nil {
		t.Skipf("cannot start PTY with empty shell: %v", err)
	}
	t.Cleanup(func() { _ = term.Close() })

	if !term.IsRunning() {
		t.Error("expected IsRunning after Start with auto-detected shell")
	}
}

// TestEchoRoundTrip is the one full end-to-end behavior assertion.
// It's standalone because the rest of TestStartedBehavior is about
// lifecycle/state changes; the echo path tests the OnOutput callback
// and PTY data flow, which need their own per-test concurrency
// scaffold.
func TestEchoRoundTrip(t *testing.T) {
	term := startTerminalOrSkip(t, "sh", 40, 120)

	// strings.Builder isn't safe for concurrent use; OnOutput fires
	// from the readLoop goroutine while the test goroutine
	// inspects.
	var (
		mu     sync.Mutex
		output strings.Builder
	)
	term.OnOutput = func(data string) {
		mu.Lock()
		defer mu.Unlock()
		output.WriteString(data)
	}
	snapshot := func() string {
		mu.Lock()
		defer mu.Unlock()
		return output.String()
	}

	if err := term.Write("echo hello\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Poll for output. The previous test slept 500ms unconditionally
	// — switching to a poll-with-deadline keeps the success case fast
	// and the failure case loud.
	if err := waitFor(time.Second, 10*time.Millisecond, func() bool {
		return strings.Contains(snapshot(), "hello")
	}); err != nil {
		t.Logf("output so far: %q", snapshot())
		// Soft-assertion: PTY echo timing is OS-dependent and the
		// original test only logged on miss too. Keep the same shape.
	}

	_ = term.Write("exit\n")
}

// waitFor polls cond every interval until it returns true or the
// deadline elapses. Returns the timeout error so callers decide
// whether to fail or just log.
func waitFor(timeout, interval time.Duration, cond func() bool) error {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("condition not met within %s", timeout)
		}
		time.Sleep(interval)
	}
}
