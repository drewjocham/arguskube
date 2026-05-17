// Lufis-side pty wrapper tests. The shared PTY core
// (github.com/argues/argus-pty) carries the comprehensive lifecycle /
// env / concurrency coverage; the tests here only exercise the
// adapter contract — that Terminal correctly forwards its public API
// to the embedded *sharedpty.PTY and that the OnOutput field-style
// callback is wired through correctly.
//
// White-box assertions on private fields (term.cmd, term.ptmx,
// term.closed, term.logger) that lived here before the extraction
// moved up into the shared package as black-box equivalents — see
// pkg/pty/pty_test.go.
package pty

import (
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// startTerminalOrSkip mirrors the shared module's helper. PTY-less
// CI envs skip cleanly.
func startTerminalOrSkip(t *testing.T, shell string, rows, cols uint16) *Terminal {
	t.Helper()
	term := New(discardLogger())
	if err := term.Start(shell, rows, cols); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	t.Cleanup(func() { _ = term.Close() })
	return term
}

func TestNew(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		logger *slog.Logger
	}{
		{"with logger", discardLogger()},
		{"nil logger accepted", nil},
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

func TestPreStartPublicAPIIsNoOp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		do   func(t *testing.T, term *Terminal)
	}{
		{"IsRunning false", func(t *testing.T, term *Terminal) {
			if term.IsRunning() {
				t.Error("expected IsRunning false before Start")
			}
		}},
		{"Close is no-op", func(t *testing.T, term *Terminal) {
			if err := term.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		}},
		{"Write is no-op", func(t *testing.T, term *Terminal) {
			if err := term.Write("x"); err != nil {
				t.Errorf("Write: %v", err)
			}
		}},
		{"Resize is no-op", func(t *testing.T, term *Terminal) {
			if err := term.Resize(80, 160); err != nil {
				t.Errorf("Resize: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.do(t, New(discardLogger()))
		})
	}
}

func TestStartedAPIDelegatesToSharedCore(t *testing.T) {
	cases := []struct {
		name string
		do   func(t *testing.T, term *Terminal)
	}{
		{"IsRunning true after Start", func(t *testing.T, term *Terminal) {
			if !term.IsRunning() {
				t.Error("expected IsRunning after Start")
			}
		}},
		{"Resize succeeds", func(t *testing.T, term *Terminal) {
			if err := term.Resize(80, 160); err != nil {
				t.Errorf("Resize: %v", err)
			}
		}},
		{"Close + IsRunning flip", func(t *testing.T, term *Terminal) {
			_ = term.Close()
			if term.IsRunning() {
				t.Error("expected IsRunning false after Close")
			}
		}},
		{"Double Close is safe", func(t *testing.T, term *Terminal) {
			_ = term.Close()
			if err := term.Close(); err != nil {
				t.Errorf("second Close: %v", err)
			}
		}},
		{"Start twice replaces previous PTY", func(t *testing.T, term *Terminal) {
			if err := term.Start("sh", 80, 160); err != nil {
				t.Errorf("second Start: %v", err)
			}
			if !term.IsRunning() {
				t.Error("expected IsRunning after second Start")
			}
		}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.do(t, startTerminalOrSkip(t, "sh", 40, 120))
		})
	}
}

// TestOnOutputFieldFiresWhenShellWrites is the one assertion that
// genuinely belongs at this layer: the adapter assigns its own
// OnOutput field, and a closure inside New routes the shared PTY's
// callback to it. Verifies the wiring.
func TestOnOutputFieldFiresWhenShellWrites(t *testing.T) {
	term := New(discardLogger())
	var calls atomic.Int64
	term.OnOutput = func(string) { calls.Add(1) }

	if err := term.Start("sh", 40, 120); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	t.Cleanup(func() { _ = term.Close() })

	if err := term.Write("echo hi\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && calls.Load() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() == 0 {
		t.Error("OnOutput field never fired — adapter wiring is broken")
	}
}

// TestOnOutputUnassignedDoesNotPanic ensures the nil-callback
// branch on the embedded PTY is still reachable through the
// adapter: leaving OnOutput unset must not crash when output
// arrives.
func TestOnOutputUnassignedDoesNotPanic(t *testing.T) {
	term := New(discardLogger())
	// Explicitly do NOT assign OnOutput.
	if err := term.Start("sh", 40, 120); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	t.Cleanup(func() { _ = term.Close() })

	if err := term.Write("echo hi\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Wait a beat for output to arrive without a callback wired.
	time.Sleep(200 * time.Millisecond)
	// No panic = success.
}

// TestConcurrentPublicCallsAreSafe re-checks the lock semantics at
// the adapter layer. The shared module covers this too; the
// duplicate here is cheap insurance that the adapter doesn't
// introduce an unsynchronized path of its own.
func TestConcurrentPublicCallsAreSafe(t *testing.T) {
	term := startTerminalOrSkip(t, "sh", 40, 120)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = term.Write("x")
				_ = term.Resize(uint16(40+j), uint16(120+j))
				_ = term.IsRunning()
			}
		}()
	}
	wg.Wait()
}
