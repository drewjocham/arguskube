// Package envprobe runs lightweight environment-detection probes that
// answer the question "why doesn't this work?" before the user has to
// ask. Each probe is a small, isolated check (DNS, TLS chain, clock
// skew, …) that emits a Result. Results feed two surfaces:
//
//   - the bottom status ribbon (one StatusEvent per probe via Wails),
//   - the Settings → "Get Argus ready" checklist (one row per probe id).
//
// Probes are deliberately *additive*: a non-OK result never blocks the
// app, it just surfaces a row with a one-click remediation. The probe
// itself never mutates anything — it only observes. Remediation is a
// separate, user-confirmed action.
package envprobe

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// Severity matches the frontend setupChecklist statuses so a Result can
// be mapped straight into a checklist row without per-probe translation
// in the producer layer.
type Severity string

const (
	OK    Severity = "ok"
	Warn  Severity = "warn"
	Todo  Severity = "todo"  // user must take an action
	Error Severity = "error" // critical blocker
)

// Result is the contract every probe returns. ID is stable across runs
// (used as the checklist row id and the dedupe key in the status feed).
// Detail is human-readable, plain text — the UI renders it under the row
// title. ActionLabel + ActionID are optional; when set, the row gets a
// right-aligned button whose click is dispatched on the frontend.
type Result struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Status      Severity      `json:"status"`
	Detail      string        `json:"detail,omitempty"`
	ActionLabel string        `json:"actionLabel,omitempty"`
	ActionID    string        `json:"actionId,omitempty"`
	Ran         time.Time     `json:"ran"`
	Latency     time.Duration `json:"latencyMs"`
}

// Probe is the unit of work the Runner orchestrates. Each implementation
// is responsible for honouring the passed context (so the Runner can
// bound them with a timeout) and never panicking — defensive: the Runner
// recovers if a probe misbehaves, but well-behaved probes never reach
// that path.
type Probe interface {
	ID() string
	Run(ctx context.Context) Result
}

// Runner schedules probes, fans them out in parallel under a per-probe
// timeout, and caches the latest Result by ID. Callers (Wails bindings,
// the periodic loop) read the cached results or trigger a fresh sweep.
type Runner struct {
	probes      []Probe
	probeTimeout time.Duration
	logger      *slog.Logger

	mu   sync.RWMutex
	last map[string]Result
}

// NewRunner builds a Runner with a per-probe timeout. The timeout is a
// safety net — well-behaved probes return earlier. Zero or negative
// timeout falls back to 3s.
func NewRunner(logger *slog.Logger, probeTimeout time.Duration, probes ...Probe) *Runner {
	if probeTimeout <= 0 {
		probeTimeout = 3 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{
		probes:       probes,
		probeTimeout: probeTimeout,
		logger:       logger,
		last:         make(map[string]Result),
	}
}

// Register adds a probe at runtime. Returns the runner so callers can chain.
// Idempotent on ID — a probe with an already-registered ID replaces the
// previous instance.
func (r *Runner) Register(p Probe) *Runner {
	for i, existing := range r.probes {
		if existing.ID() == p.ID() {
			r.probes[i] = p
			return r
		}
	}
	r.probes = append(r.probes, p)
	return r
}

// RunAll fans out every registered probe in parallel, each under its own
// timeout. Returns the results sorted by ID for deterministic output. A
// probe that panics yields an Error result with the recovery message so
// one bad probe never breaks the sweep.
func (r *Runner) RunAll(ctx context.Context) []Result {
	r.mu.RLock()
	probes := make([]Probe, len(r.probes))
	copy(probes, r.probes)
	r.mu.RUnlock()

	results := make([]Result, len(probes))
	var wg sync.WaitGroup
	for i, p := range probes {
		wg.Add(1)
		go func(i int, p Probe) {
			defer wg.Done()
			results[i] = r.runOne(ctx, p)
		}(i, p)
	}
	wg.Wait()

	r.mu.Lock()
	for _, res := range results {
		r.last[res.ID] = res
	}
	r.mu.Unlock()

	sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })
	return results
}

// runOne wraps a single probe with timeout + panic recovery. It always
// returns a Result; even when the probe's goroutine panics, the caller
// gets something it can render.
func (r *Runner) runOne(ctx context.Context, p Probe) Result {
	probeCtx, cancel := context.WithTimeout(ctx, r.probeTimeout)
	defer cancel()

	start := time.Now()
	resultCh := make(chan Result, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				resultCh <- Result{
					ID:     p.ID(),
					Title:  p.ID(),
					Status: Error,
					Detail: "probe panicked",
				}
			}
		}()
		resultCh <- p.Run(probeCtx)
	}()

	select {
	case res := <-resultCh:
		if res.Ran.IsZero() {
			res.Ran = time.Now()
		}
		if res.Latency == 0 {
			res.Latency = time.Since(start)
		}
		if res.ID == "" {
			res.ID = p.ID()
		}
		return res
	case <-probeCtx.Done():
		return Result{
			ID:      p.ID(),
			Title:   p.ID(),
			Status:  Warn,
			Detail:  "probe timed out — Argus will retry shortly",
			Ran:     time.Now(),
			Latency: time.Since(start),
		}
	}
}

// Latest returns the cached Result for a probe id (post-RunAll), or zero
// value if the probe hasn't run yet.
func (r *Runner) Latest(id string) (Result, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res, ok := r.last[id]
	return res, ok
}

// All returns a snapshot of all cached results. Useful for the periodic
// dispatcher to send the full set whenever a new client connects.
func (r *Runner) All() []Result {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Result, 0, len(r.last))
	for _, res := range r.last {
		out = append(out, res)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
