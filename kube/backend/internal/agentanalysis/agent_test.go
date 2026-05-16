// Package agentanalysis tests cover the lightweight loop / constructor
// behavior. RunAnalysis itself sleeps 3 seconds and emits Wails events,
// neither of which is useful to exercise in a unit test — that work
// belongs in an integration harness with a fake Wails runtime.
package agentanalysis

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/argues/argus/internal/config"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewAgentStoresDependencies(t *testing.T) {
	t.Parallel()
	cfg := &config.OnlineDataConfig{}
	ctx := context.Background()
	a := NewAgent(discardLogger(), cfg, ctx)

	if a == nil {
		t.Fatal("NewAgent returned nil")
	}
	if a.cfg != cfg {
		t.Error("NewAgent did not store cfg")
	}
	if a.appCtx != ctx {
		t.Error("NewAgent did not store appCtx")
	}
	if a.logger == nil {
		t.Error("NewAgent did not initialize logger")
	}
}

func TestNewAgentNilAppCtxAccepted(t *testing.T) {
	t.Parallel()
	// nil appCtx is the "no Wails runtime attached" path; RunAnalysis
	// branches on this. The constructor must accept it.
	a := NewAgent(discardLogger(), &config.OnlineDataConfig{}, nil)
	if a.appCtx != nil {
		t.Error("NewAgent should preserve nil appCtx without substitution")
	}
}

func TestStartLoopReturnsOnContextCancel(t *testing.T) {
	t.Parallel()
	// StartLoop uses a 1-hour ticker, so the only way out within the
	// test budget is the ctx.Done() branch — that's the contract this
	// test pins down.
	a := NewAgent(discardLogger(), &config.OnlineDataConfig{}, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		a.StartLoop(ctx)
		close(done)
	}()

	// Give the goroutine a moment to enter the select.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartLoop did not return within 2s of ctx cancel — leak risk in production")
	}
}

func TestStartLoopReturnsImmediatelyWithCancelledCtx(t *testing.T) {
	t.Parallel()
	a := NewAgent(discardLogger(), &config.OnlineDataConfig{}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before StartLoop even sees the channel.

	done := make(chan struct{})
	go func() {
		a.StartLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("StartLoop did not return for a pre-cancelled context")
	}
}

func TestTriggerAnalysisDoesNotBlock(t *testing.T) {
	t.Parallel()
	// TriggerAnalysis fires RunAnalysis in a goroutine. Even though
	// RunAnalysis sleeps 3 seconds internally, TriggerAnalysis itself
	// must return immediately — that's the contract for the UI button.
	a := NewAgent(discardLogger(), &config.OnlineDataConfig{}, nil)

	start := time.Now()
	a.TriggerAnalysis()
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("TriggerAnalysis blocked for %s; expected non-blocking", elapsed)
	}
	// No further wait — the spawned goroutine sleeps and emits Wails
	// events but we deliberately don't depend on its completion.
}
