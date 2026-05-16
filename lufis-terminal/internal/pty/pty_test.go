package pty

import (
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		logger      *slog.Logger
		wantNil     bool
		wantDiscard bool
	}{
		{
			name:    "creates terminal with logger",
			logger:  slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
			wantNil: false,
		},
		{
			name:    "creates terminal with nil logger (discards)",
			logger:  nil,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := New(tt.logger)
			require.NotNil(t, term)
			assert.NotNil(t, term.logger)
			assert.False(t, term.closed)
			assert.Nil(t, term.cmd)
			assert.Nil(t, term.ptmx)
		})
	}
}

func TestStart(t *testing.T) {
	tests := []struct {
		name       string
		shell      string
		rows       uint16
		cols       uint16
		wantErr    bool
		setupEnvFn func()
	}{
		{
			name:    "starts with echo command",
			shell:   "/bin/echo",
			rows:    24,
			cols:    80,
			wantErr: false,
		},
		{
			name:    "starts with /usr/bin/true",
			shell:   "/usr/bin/true",
			rows:    24,
			cols:    80,
			wantErr: false,
		},
		{
			name:    "starts with different dimensions",
			shell:   "/bin/echo",
			rows:    40,
			cols:    120,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := New(slog.Default())
			defer term.Close()

			err := term.Start(tt.shell, tt.rows, tt.cols)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify the terminal was started
			assert.True(t, term.IsRunning())
			assert.NotNil(t, term.cmd)
			assert.NotNil(t, term.ptmx)

			// Verify env vars were set
			require.NotNil(t, term.cmd)
			env := strings.Join(term.cmd.Env, " ")
			assert.Contains(t, env, "TERM=xterm-256color")
			assert.Contains(t, env, "COLORTERM=truecolor")
		})
	}
}

func TestStartSetsEnvVars(t *testing.T) {
	term := New(slog.Default())
	defer term.Close()

	err := term.Start("/bin/echo", 24, 80)
	require.NoError(t, err)

	require.NotNil(t, term.cmd)

	foundTerm := false
	foundColorTerm := false
	for _, env := range term.cmd.Env {
		if env == "TERM=xterm-256color" {
			foundTerm = true
		}
		if env == "COLORTERM=truecolor" {
			foundColorTerm = true
		}
	}
	assert.True(t, foundTerm, "TERM=xterm-256color should be in env")
	assert.True(t, foundColorTerm, "COLORTERM=truecolor should be in env")
}

func TestStartNonexistentShell(t *testing.T) {
	term := New(slog.Default())
	defer term.Close()

	err := term.Start("/nonexistent/shell", 24, 80)
	assert.Error(t, err)
	assert.False(t, term.IsRunning())
}

func TestWrite(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func(t *testing.T) *Terminal
		data    string
		wantErr bool
	}{
		{
			name: "writes data to PTY after start",
			setupFn: func(t *testing.T) *Terminal {
				term := New(slog.Default())
				err := term.Start("/bin/cat", 24, 80)
				require.NoError(t, err)
				return term
			},
			data:    "hello world",
			wantErr: false,
		},
		{
			name: "Write is no-op when PTY is not started",
			setupFn: func(t *testing.T) *Terminal {
				return New(slog.Default())
			},
			data:    "test data",
			wantErr: false,
		},
		{
			name: "Write is no-op after close",
			setupFn: func(t *testing.T) *Terminal {
				term := New(slog.Default())
				err := term.Start("/bin/cat", 24, 80)
				require.NoError(t, err)
				term.Close()
				return term
			},
			data:    "test after close",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := tt.setupFn(t)
			if term.closed == false && term.ptmx == nil {
				// Started case - clean up
				defer term.Close()
			}

			err := term.Write(tt.data)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestResize(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func(t *testing.T) *Terminal
		rows    uint16
		cols    uint16
		wantErr bool
	}{
		{
			name: "resize PTY to larger dimensions",
			setupFn: func(t *testing.T) *Terminal {
				term := New(slog.Default())
				err := term.Start("/bin/cat", 24, 80)
				require.NoError(t, err)
				return term
			},
			rows:    50,
			cols:    160,
			wantErr: false,
		},
		{
			name: "resize PTY to smaller dimensions",
			setupFn: func(t *testing.T) *Terminal {
				term := New(slog.Default())
				err := term.Start("/bin/cat", 24, 80)
				require.NoError(t, err)
				return term
			},
			rows:    10,
			cols:    40,
			wantErr: false,
		},
		{
			name: "Resize is no-op when PTY not started",
			setupFn: func(t *testing.T) *Terminal {
				return New(slog.Default())
			},
			rows:    24,
			cols:    80,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := tt.setupFn(t)
			if !term.closed && term.ptmx != nil {
				defer term.Close()
			}

			err := term.Resize(tt.rows, tt.cols)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestClose(t *testing.T) {
	tests := []struct {
		name    string
		setupFn func(t *testing.T) *Terminal
	}{
		{
			name: "closes running terminal marks it as closed",
			setupFn: func(t *testing.T) *Terminal {
				term := New(slog.Default())
				err := term.Start("/bin/cat", 24, 80)
				require.NoError(t, err)
				return term
			},
		},
		{
			name: "closes terminal with echo (quick exit)",
			setupFn: func(t *testing.T) *Terminal {
				term := New(slog.Default())
				err := term.Start("/bin/echo", 24, 80)
				require.NoError(t, err)
				return term
			},
		},
		{
			name: "closes unstarted terminal is safe",
			setupFn: func(t *testing.T) *Terminal {
				return New(slog.Default())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := tt.setupFn(t)
			_ = term.Close()

			// Regardless of whether Close returns an error (Wait() after Kill()
			// may return signal exit status), the terminal must be marked closed.
			assert.True(t, term.closed)
			assert.False(t, term.IsRunning())
		})
	}
}

func TestDoubleCloseIsSafe(t *testing.T) {
	term := New(slog.Default())
	err := term.Start("/bin/cat", 24, 80)
	require.NoError(t, err)

	// First close
	_ = term.Close()
	assert.True(t, term.closed)

	// Second close - should be idempotent (no panic)
	_ = term.Close()
	assert.True(t, term.closed)
}

func TestIsRunning(t *testing.T) {
	tests := []struct {
		name       string
		setupFn    func(t *testing.T) *Terminal
		wantBefore bool
		wantAfter  bool
	}{
		{
			name: "returns false before start",
			setupFn: func(t *testing.T) *Terminal {
				return New(slog.Default())
			},
			wantBefore: false,
			wantAfter:  false,
		},
		{
			name: "returns true after start",
			setupFn: func(t *testing.T) *Terminal {
				term := New(slog.Default())
				err := term.Start("/bin/cat", 24, 80)
				require.NoError(t, err)
				return term
			},
			wantBefore: true,
			wantAfter:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := tt.setupFn(t)
			assert.Equal(t, tt.wantBefore, term.IsRunning())

			if tt.wantBefore {
				term.Close()
				assert.Equal(t, tt.wantAfter, term.IsRunning())
			}
		})
	}
}

func TestStartAfterCloseIsSafe(t *testing.T) {
	term := New(slog.Default())

	// Start and close (ignore close error signal: hangup)
	err := term.Start("/bin/cat", 24, 80)
	require.NoError(t, err)
	_ = term.Close()

	// Start after close should be a no-op (returns nil)
	err = term.Start("/bin/echo", 24, 80)
	assert.NoError(t, err)
	// IsRunning should return false because closed is still true
	assert.False(t, term.IsRunning())
}

func TestOnOutputCallback(t *testing.T) {
	term := New(slog.Default())
	defer term.Close()

	var mu sync.Mutex
	var receivedData string

	term.OnOutput = func(data string) {
		mu.Lock()
		defer mu.Unlock()
		receivedData += data
	}

	err := term.Start("/bin/echo", 24, 80)
	require.NoError(t, err)

	// Give the process time to execute and read loop to process output
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	// The echo command should produce some output (the newline at minimum)
	// We can't guarantee the exact content, but we know there should be output
	t.Logf("received data from echo: %q", receivedData)
	mu.Unlock()
}

func TestOnOutputCallbackReceivesWriteData(t *testing.T) {
	term := New(slog.Default())

	var mu sync.Mutex
	var receivedData string
	done := make(chan struct{})

	term.OnOutput = func(data string) {
		mu.Lock()
		defer mu.Unlock()
		receivedData += data
		// Signal when we've received enough data
		if len(receivedData) > 0 {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	}

	err := term.Start("/bin/cat", 24, 80)
	require.NoError(t, err)
	defer term.Close()

	// Write data to the PTY
	err = term.Write("hello from pty\n")
	require.NoError(t, err)

	// Wait for the output callback to fire (cat echoes input)
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Log("timed out waiting for output callback")
	}

	mu.Lock()
	assert.Contains(t, receivedData, "hello from pty")
	mu.Unlock()
}

func TestConcurrentOperations(t *testing.T) {
	term := New(slog.Default())
	err := term.Start("/bin/cat", 24, 80)
	require.NoError(t, err)
	defer term.Close()

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			err := term.Write("test data\n")
			assert.NoError(t, err)
		}(i)
	}

	// Concurrent resize
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := term.Resize(30, 100)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
}

func TestLoggerDiscardedOnNil(t *testing.T) {
	term := New(nil)
	require.NotNil(t, term)
	require.NotNil(t, term.logger)

	// The discard handler should not panic
	term.logger.Debug("this should not panic")
	term.logger.Info("this should not panic")
}
