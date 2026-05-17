package pty

import (
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// discardLogger keeps test output uncluttered. Production code
// substitutes the discard handler internally when New(nil) is called,
// but tests that care about log capture should pass an explicit
// handler.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// startPTYOrSkip is the shared lifecycle helper. Many CI envs lack
// /dev/pts; t.Skipf keeps the suite usable there while still running
// everything else against a real PTY when one's available.
func startPTYOrSkip(t *testing.T, opts Options) *PTY {
	t.Helper()
	p := New(discardLogger())
	if err := p.Start(opts); err != nil {
		t.Skipf("cannot start PTY (likely no /dev/pts): %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// ── New / constructor ────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		logger *slog.Logger
	}{
		{"with logger", discardLogger()},
		{"nil logger substitutes discard handler", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if p := New(tc.logger); p == nil {
				t.Fatal("New returned nil")
			}
		})
	}
}

// ── Pre-start safety: every public method is a no-op before Start ──

func TestPreStartSafety(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		do   func(t *testing.T, p *PTY)
	}{
		{"IsRunning false", func(t *testing.T, p *PTY) {
			if p.IsRunning() {
				t.Error("IsRunning should be false before Start")
			}
		}},
		{"Close is a no-op", func(t *testing.T, p *PTY) {
			if err := p.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		}},
		{"Double Close is idempotent", func(t *testing.T, p *PTY) {
			_ = p.Close()
			if err := p.Close(); err != nil {
				t.Errorf("second Close: %v", err)
			}
		}},
		{"Write is a no-op", func(t *testing.T, p *PTY) {
			if err := p.Write("x"); err != nil {
				t.Errorf("Write: %v", err)
			}
		}},
		{"Resize is a no-op", func(t *testing.T, p *PTY) {
			if err := p.Resize(80, 160); err != nil {
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

// ── Lifecycle: real PTY required, runs serially ─────────────────────

func TestStartedLifecycle(t *testing.T) {
	cases := []struct {
		name string
		do   func(t *testing.T, p *PTY)
	}{
		{
			name: "IsRunning true after Start",
			do: func(t *testing.T, p *PTY) {
				if !p.IsRunning() {
					t.Error("IsRunning should be true after Start")
				}
			},
		},
		{
			name: "IsRunning false after Close",
			do: func(t *testing.T, p *PTY) {
				_ = p.Close()
				if p.IsRunning() {
					t.Error("IsRunning should be false after Close")
				}
			},
		},
		{
			name: "Resize succeeds after Start",
			do: func(t *testing.T, p *PTY) {
				if err := p.Resize(80, 160); err != nil {
					t.Errorf("Resize: %v", err)
				}
			},
		},
		{
			name: "Close after Start succeeds",
			do: func(t *testing.T, p *PTY) {
				if err := p.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
			},
		},
		{
			name: "Write after Close does not panic",
			do: func(t *testing.T, p *PTY) {
				_ = p.Close()
				if err := p.Write("test"); err != nil {
					t.Logf("Write after Close returned: %v", err)
				}
			},
		},
		{
			name: "Resize after Close does not panic",
			do: func(t *testing.T, p *PTY) {
				_ = p.Close()
				if err := p.Resize(80, 160); err != nil {
					t.Logf("Resize after Close returned: %v", err)
				}
			},
		},
		{
			name: "Start twice replaces the previous PTY",
			do: func(t *testing.T, p *PTY) {
				if err := p.Start(Options{Shell: "sh", Rows: 80, Cols: 160, LoginShell: true}); err != nil {
					t.Errorf("second Start: %v", err)
				}
				if !p.IsRunning() {
					t.Error("IsRunning should be true after second Start")
				}
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.do(t, startPTYOrSkip(t, Options{Shell: "sh", Rows: 40, Cols: 120, LoginShell: true}))
		})
	}
}

// ── Empty shell fallback ─────────────────────────────────────────────

func TestStartFallsBackToSHELLEnv(t *testing.T) {
	// Empty Shell + SHELL set → use SHELL.
	t.Setenv("SHELL", "/bin/sh")
	p := New(discardLogger())
	if err := p.Start(Options{Rows: 40, Cols: 120, LoginShell: true}); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	if !p.IsRunning() {
		t.Error("expected IsRunning after empty-shell fallback")
	}
}

func TestStartFallsBackToDefaultShellWhenEnvUnset(t *testing.T) {
	t.Setenv("SHELL", "")
	p := New(discardLogger())
	// DefaultShell = "zsh"; on a system without zsh, Start fails and
	// we skip. The test's point is the fallback codepath runs.
	if err := p.Start(Options{Rows: 40, Cols: 120, LoginShell: true}); err != nil {
		t.Skipf("DefaultShell %q not available: %v", DefaultShell, err)
	}
	t.Cleanup(func() { _ = p.Close() })
}

// ── ExtraEnv flows through ───────────────────────────────────────────

func TestExtraEnvFlowsToTheShell(t *testing.T) {
	// Drive the env-overlay path end-to-end: spawn a shell with a
	// unique env var, ask the shell to echo it, and verify the value
	// arrives via OnOutput. This is the closest public-API proxy to
	// the old white-box "verify cmd.Env contains FOO=bar" test.
	const key = "ARGUS_PTY_TEST_TOKEN"
	const value = "tok_xK9Lm3"

	p := New(discardLogger())
	var (
		mu  sync.Mutex
		buf strings.Builder
	)
	p.OnOutput = func(s string) {
		mu.Lock()
		defer mu.Unlock()
		buf.WriteString(s)
	}

	if err := p.Start(Options{
		Shell:      "sh",
		Rows:       40,
		Cols:       120,
		LoginShell: false, // login shell can rewrite env via profile
		ExtraEnv:   []string{key + "=" + value},
	}); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	if err := p.Write("printf '%s=%s\\n' " + key + " \"$" + key + "\"\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		got := buf.String()
		mu.Unlock()
		if strings.Contains(got, key+"="+value) {
			return // success
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	got := buf.String()
	mu.Unlock()
	t.Errorf("expected ExtraEnv var to reach the shell; got %q", got)
}

// ── OnOutput callback ────────────────────────────────────────────────

func TestOnOutputFiresWhenShellWrites(t *testing.T) {
	p := New(discardLogger())
	var calls atomic.Int64
	p.OnOutput = func(string) { calls.Add(1) }

	if err := p.Start(Options{Shell: "sh", Rows: 40, Cols: 120, LoginShell: true}); err != nil {
		t.Skipf("cannot start PTY: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })

	if err := p.Write("echo hi\n"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && calls.Load() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() == 0 {
		t.Error("OnOutput never fired after shell wrote 'hi'")
	}
}

// ── Concurrent usage doesn't race ────────────────────────────────────

func TestConcurrentOperationsAreSafe(t *testing.T) {
	p := startPTYOrSkip(t, Options{Shell: "sh", Rows: 40, Cols: 120, LoginShell: true})

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = p.Write("x")
				_ = p.Resize(uint16(40+j), uint16(120+j))
				_ = p.IsRunning()
			}
		}()
	}
	wg.Wait()
}

// ── Defaults visible ─────────────────────────────────────────────────

func TestDefaultsExportedAsConstants(t *testing.T) {
	t.Parallel()
	if DefaultBufferSize <= 0 {
		t.Error("DefaultBufferSize must be positive")
	}
	if DefaultShell == "" {
		t.Error("DefaultShell must be set")
	}
}
