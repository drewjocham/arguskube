// Package spotcheck runs lightweight cluster health probes on a
// schedule and emits the results as Argus notifications.
//
// The engine is deliberately minimal: each Check is a small, focused
// function ("are nodes Ready?", "is DECISION_LOG.md still being
// updated?"). It does NOT replace the AI scan / vulnerability scan /
// Argus scan — those are deeper and slower. Spot-checks are the
// equivalent of an SRE walking past the dashboard and going "yep,
// that still looks fine."
//
// Scheduling: a top-level goroutine ticks every Interval, runs every
// registered check sequentially, and emits one Wails event per check
// for "currently running" plus one notification per finding.
package spotcheck

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Severity mirrors the kinds the frontend already understands.
type Severity string

const (
	SevOK    Severity = "ok"
	SevInfo  Severity = "info"
	SevWarn  Severity = "warn"
	SevError Severity = "error"
)

// Finding is the result of a single check. A check can return
// (nil, nil) to signal "nothing notable to report" — only Findings
// with non-OK severities trigger notifications by default.
type Finding struct {
	Severity Severity
	Title    string
	Body     string
	// Meta is JSON-encodable arbitrary state surfaced to the UI for
	// "details" expansion. Keep it small.
	Meta map[string]any
}

// Check is the contract every probe satisfies.
type Check interface {
	// Name is the stable identifier used for rerun routing. Must be
	// unique within an Engine.
	Name() string
	// Description is shown in the "currently doing X" pill.
	Description() string
	// Run executes the check. ctx may be cancelled by the engine if
	// shutdown is requested mid-check.
	Run(ctx context.Context) (*Finding, error)
}

// Notifier is what the engine calls to surface activity + findings.
// Implemented by the Wails-bound App so the runtime stays optional in
// tests (tests pass a fake notifier).
type Notifier interface {
	// Active is called immediately before a check starts. Pass the
	// human-readable description; pass empty string when nothing is
	// running so the frontend can hide the pill.
	Active(checkName, description string)
	// Notify persists a finding via the user-visible notifications
	// channel.
	Notify(checkName string, f Finding)
}

// Engine owns the registered checks + the periodic loop.
type Engine struct {
	logger   *slog.Logger
	notifier Notifier
	interval time.Duration

	mu     sync.RWMutex
	checks map[string]Check
}

// New builds an engine with the given Notifier and tick interval.
// Callers register checks via Add() before calling StartLoop.
func New(logger *slog.Logger, notifier Notifier, interval time.Duration) *Engine {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &Engine{
		logger:   logger.With("component", "spotcheck"),
		notifier: notifier,
		interval: interval,
		checks:   make(map[string]Check),
	}
}

// Add registers a check. Last write wins on name collisions; that's
// fine because the engine isn't a plugin host — the caller controls
// the registration order.
func (e *Engine) Add(c Check) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.checks[c.Name()] = c
}

// Names returns the registered check names in arbitrary order. Used
// by tests + admin endpoints.
func (e *Engine) Names() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]string, 0, len(e.checks))
	for n := range e.checks {
		out = append(out, n)
	}
	return out
}

// RunAll runs every registered check sequentially, in lock order
// (sorted-ish by registration). Emits Active/idle around each run.
// Errors from individual checks are logged and surfaced as error
// findings; one bad check doesn't abort the rest.
func (e *Engine) RunAll(ctx context.Context) {
	e.mu.RLock()
	checks := make([]Check, 0, len(e.checks))
	for _, c := range e.checks {
		checks = append(checks, c)
	}
	e.mu.RUnlock()

	for _, c := range checks {
		select {
		case <-ctx.Done():
			e.notifier.Active("", "")
			return
		default:
		}
		e.runOne(ctx, c)
	}
	e.notifier.Active("", "")
}

// RunOne runs a single check by name. Returns ErrUnknownCheck when
// no such check is registered. Used by the frontend "rerun" button.
func (e *Engine) RunOne(ctx context.Context, name string) error {
	e.mu.RLock()
	c, ok := e.checks[name]
	e.mu.RUnlock()
	if !ok {
		return ErrUnknownCheck
	}
	e.runOne(ctx, c)
	e.notifier.Active("", "")
	return nil
}

// ErrUnknownCheck signals the caller asked for a check that wasn't registered.
var ErrUnknownCheck = errors.New("spotcheck: unknown check")

func (e *Engine) runOne(ctx context.Context, c Check) {
	e.notifier.Active(c.Name(), c.Description())
	start := time.Now()
	finding, err := c.Run(ctx)
	dur := time.Since(start)

	if err != nil {
		e.logger.Warn("check failed",
			slog.String("check", c.Name()),
			slog.Duration("duration", dur),
			slog.String("error", err.Error()),
		)
		e.notifier.Notify(c.Name(), Finding{
			Severity: SevError,
			Title:    fmt.Sprintf("Spot-check error: %s", c.Description()),
			Body:     err.Error(),
			Meta:     map[string]any{"check": c.Name(), "durationMs": dur.Milliseconds()},
		})
		return
	}
	if finding == nil {
		// Silent pass — the check has nothing to report. We still log
		// it for debugging but don't spam the notification stream.
		e.logger.Debug("check passed",
			slog.String("check", c.Name()),
			slog.Duration("duration", dur),
		)
		return
	}
	if finding.Meta == nil {
		finding.Meta = map[string]any{}
	}
	finding.Meta["check"] = c.Name()
	finding.Meta["durationMs"] = dur.Milliseconds()
	e.notifier.Notify(c.Name(), *finding)
}

// StartLoop runs the engine on a ticker until ctx is cancelled.
// Fires once immediately so the user sees activity on first launch
// instead of waiting a full interval.
func (e *Engine) StartLoop(ctx context.Context) {
	go func() {
		// Small initial delay so we don't fight the rest of startup.
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
		e.RunAll(ctx)
		t := time.NewTicker(e.interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				e.RunAll(ctx)
			}
		}
	}()
}
