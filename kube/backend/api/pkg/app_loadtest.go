package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/argus/pkg/broker"
	"github.com/argues/argus/pkg/loadtest"
	"github.com/argues/argus/pkg/loadtest/analysis"
)

// app_loadtest.go — Wails bindings for the load-test feature.
// Bridges the engine in pkg/loadtest to the frontend.
//
// Lifecycle from the frontend's perspective:
//
//   1. ListLoadTestPresets()              → render the picker
//   2. StartLoadTest(spec)                → returns runID; engine
//                                            kicks off in a goroutine
//   3. argus:loadtest:progress events     → live throughput, P95,
//                                            scale state for the chart
//   4. argus:loadtest:done event          → final summary + path of
//                                            the markdown report
//   5. argus:notification                 → "Load test complete" toast
//   6. GetLoadTestRecord(runID)           → optional fetch for the
//                                            run-history view
//   7. CancelLoadTest(runID)              → mid-flight stop
//
// We support exactly ONE active run at a time. Concurrency on the
// runner side is fine (the engine handles its own workers), but two
// concurrent runs would compete for the consumer-side scaling dance
// and produce nonsense reports.

// LoadTestStatus is the snapshot shape the frontend polls for or
// receives via the progress event channel. It is intentionally small
// — full RunRecord (with all Samples) is only fetched on demand.
type LoadTestStatus struct {
	RunID       string             `json:"runId"`
	State       string             `json:"state"` // pending|running|done|canceled|error
	StartedAt   time.Time          `json:"startedAt"`
	FinishedAt  time.Time          `json:"finishedAt,omitempty"`
	Spec        loadtest.RunSpec   `json:"spec"`
	Summary     loadtest.Summary   `json:"summary"`
	LastScale   *loadtest.ScaleEvent `json:"lastScale,omitempty"`
	FinalError  string             `json:"finalError,omitempty"`
	ReportPath  string             `json:"reportPath,omitempty"`
}

// loadtestRegistry holds all runs the App knows about. The active
// run (if any) is tracked separately so StartLoadTest can reject a
// second concurrent start with a clear message rather than silently
// queuing.
type loadtestRegistry struct {
	mu       sync.RWMutex
	runs     map[string]*loadtestRun
	activeID string
}

func newLoadtestRegistry() *loadtestRegistry {
	return &loadtestRegistry{runs: map[string]*loadtestRun{}}
}

type loadtestRun struct {
	id      string
	spec    loadtest.RunSpec
	engine  *loadtest.Engine
	cancel  context.CancelFunc
	started time.Time

	mu          sync.RWMutex
	state       string
	finished    time.Time
	record      *loadtest.RunRecord
	lastScale   *loadtest.ScaleEvent
	reportPath  string
	finalError  string
}

// loadtest returns the registry, lazily constructed. Kept off the App
// struct's exported fields to avoid bloating it; the registry is
// stateless w.r.t. the rest of App and outlives a single Startup.
var (
	loadtestRegMu sync.Mutex
	loadtestRegs  = map[*App]*loadtestRegistry{}

	// loadtestPublisherFactories lets tests substitute the broker
	// factory used by StartLoadTest. Production leaves the per-App
	// entry nil; the engine falls back to broker.New. Keyed by
	// *App so multiple parallel tests don't interfere.
	loadtestFactoryMu sync.RWMutex
	loadtestFactories = map[*App]func(context.Context, broker.Config, *slog.Logger) (broker.Publisher, error){}
)

// setLoadtestPublisherFactory is the test-only hook for injecting a
// fake broker publisher into the engine before StartLoadTest spawns
// its goroutine. Production code does not call this — the
// app_loadtest_test.go file uses it via the unexported helper.
func (a *App) setLoadtestPublisherFactory(f func(context.Context, broker.Config, *slog.Logger) (broker.Publisher, error)) {
	loadtestFactoryMu.Lock()
	defer loadtestFactoryMu.Unlock()
	if f == nil {
		delete(loadtestFactories, a)
	} else {
		loadtestFactories[a] = f
	}
}

func (a *App) loadtestPublisherFactory() func(context.Context, broker.Config, *slog.Logger) (broker.Publisher, error) {
	loadtestFactoryMu.RLock()
	defer loadtestFactoryMu.RUnlock()
	return loadtestFactories[a]
}

func (a *App) loadtests() *loadtestRegistry {
	loadtestRegMu.Lock()
	defer loadtestRegMu.Unlock()
	r, ok := loadtestRegs[a]
	if !ok {
		r = newLoadtestRegistry()
		loadtestRegs[a] = r
	}
	return r
}

// ListLoadTestPresets is the no-arg list of the 5 audit-locked presets
// plus any future additions. Each carries its own RunSpec the user
// edits before pressing Start.
func (a *App) ListLoadTestPresets() []loadtest.Preset {
	return loadtest.PresetList()
}

// ListBrokerKinds exposes the broker kinds the load tester can target.
// Used by the frontend dropdown so adding a kind in pkg/broker
// automatically widens the UI without a frontend change.
func (a *App) ListBrokerKinds() []broker.Kind {
	return broker.Knowns
}

// StartLoadTest kicks off a run. Returns the runID the frontend uses
// to address subsequent calls (status, cancel, fetch record). The
// engine runs in a goroutine — this call returns once the spec has
// been validated and the goroutine has been launched.
func (a *App) StartLoadTest(spec loadtest.RunSpec) (string, error) {
	if err := spec.Validate(); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}
	reg := a.loadtests()
	reg.mu.Lock()
	if reg.activeID != "" {
		existing := reg.runs[reg.activeID]
		if existing != nil && existing.activeState() == "running" {
			reg.mu.Unlock()
			return "", fmt.Errorf("a load test is already running (id=%s); cancel it first", reg.activeID)
		}
	}
	id := uuid.New().String()
	ctx, cancel := context.WithCancel(a.appCtx())
	run := &loadtestRun{
		id:      id,
		spec:    spec,
		cancel:  cancel,
		started: time.Now(),
		state:   "pending",
	}
	engine := loadtest.New(spec, a.logger.With("component", "loadtest", "runId", id))
	if a.k8s != nil {
		engine.Scaler = loadtest.NewKubeScaler(a.k8s.GetClientset(), 0)
	}
	// Apply the test-injectable factory if one is registered for
	// this App. Production never registers one, so the engine uses
	// its default broker.New.
	if f := a.loadtestPublisherFactory(); f != nil {
		engine.NewPublisher = f
	}
	// Throttle live progress emits so a 1M-message run doesn't drown
	// the WebView in events. We coalesce on a 250ms ticker — the
	// frontend chart updates at ~4 fps which is plenty for human
	// perception.
	progress := newProgressThrottler(a, id, 250*time.Millisecond)
	engine.OnSample(progress.onSample)
	engine.OnScale(func(ev loadtest.ScaleEvent) {
		evCopy := ev
		run.mu.Lock()
		run.lastScale = &evCopy
		run.mu.Unlock()
		progress.onScale(ev)
	})
	run.engine = engine
	reg.runs[id] = run
	reg.activeID = id
	reg.mu.Unlock()

	go a.runLoadTest(ctx, run, progress)
	return id, nil
}

// runLoadTest is the goroutine body. Exits when the engine returns;
// publishes the final event + writes the report to Notebooks.
func (a *App) runLoadTest(ctx context.Context, run *loadtestRun, progress *progressThrottler) {
	run.mu.Lock()
	run.state = "running"
	run.mu.Unlock()

	rec, _ := run.engine.Run(ctx)
	progress.flush()

	run.mu.Lock()
	run.finished = time.Now()
	run.record = rec
	if rec != nil {
		run.finalError = rec.FinalError
	}
	switch {
	case ctx.Err() != nil:
		run.state = "canceled"
	case run.finalError != "":
		run.state = "error"
	default:
		run.state = "done"
	}
	run.mu.Unlock()

	// Write the markdown report. PR-C ships a deterministic, plain
	// summary; PR-E will replace the body with the AI agent's
	// narrative, but the on-disk path and notification stay the same.
	if rec != nil {
		path, err := a.exportLoadTestReport(rec)
		if err == nil {
			run.mu.Lock()
			run.reportPath = path
			run.mu.Unlock()
		} else {
			a.logger.Warn("loadtest: report export failed",
				slog.String("runId", run.id),
				slog.String("error", err.Error()))
		}
	}

	a.safeEmit("argus:loadtest:done", a.loadTestStatusOf(run))
	a.safeEmitLoadTestNotification(run)
}

// safeEmit wraps runtime.EventsEmit so tests (and any production code
// path running before Wails Startup has installed the runtime context)
// don't see "An invalid context was passed" logs. Production callers
// get a real emit; tests get a no-op.
func (a *App) safeEmit(event string, payload any) {
	if a.ctx == nil {
		return
	}
	defer func() { _ = recover() }()
	runtime.EventsEmit(a.ctx, event, payload)
}

// CancelLoadTest cancels an in-flight run. No-op if the run is already
// done. Returns an error if the runID is unknown.
func (a *App) CancelLoadTest(runID string) error {
	reg := a.loadtests()
	reg.mu.RLock()
	run, ok := reg.runs[runID]
	reg.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown load test runId %q", runID)
	}
	run.cancel()
	return nil
}

// GetLoadTestStatus returns the small status shape. Used by the
// frontend on view-mount to recover state after a reload.
func (a *App) GetLoadTestStatus(runID string) (*LoadTestStatus, error) {
	reg := a.loadtests()
	reg.mu.RLock()
	run, ok := reg.runs[runID]
	reg.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown load test runId %q", runID)
	}
	st := a.loadTestStatusOf(run)
	return &st, nil
}

// GetLoadTestRecord returns the full RunRecord (with all samples).
// Used by the frontend's "view raw record" link. Returns nil + error
// if the run is still in flight.
func (a *App) GetLoadTestRecord(runID string) (*loadtest.RunRecord, error) {
	reg := a.loadtests()
	reg.mu.RLock()
	run, ok := reg.runs[runID]
	reg.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown load test runId %q", runID)
	}
	run.mu.RLock()
	rec := run.record
	state := run.state
	run.mu.RUnlock()
	if rec == nil {
		return nil, fmt.Errorf("load test %s not finished (state=%s)", runID, state)
	}
	return rec, nil
}

// loadTestStatusOf builds the snapshot for an event or RPC reply.
// Locks the run; returns by value so the caller can release the lock.
func (a *App) loadTestStatusOf(run *loadtestRun) LoadTestStatus {
	run.mu.RLock()
	defer run.mu.RUnlock()
	st := LoadTestStatus{
		RunID:      run.id,
		State:      run.state,
		StartedAt:  run.started,
		FinishedAt: run.finished,
		Spec:       run.spec,
		LastScale:  run.lastScale,
		FinalError: run.finalError,
		ReportPath: run.reportPath,
	}
	if run.record != nil {
		st.Summary = run.record.Summary
	} else if run.engine != nil {
		samples, _ := run.engine.Snapshot()
		st.Summary = loadtest.Aggregate(samples)
	}
	return st
}

func (r *loadtestRun) activeState() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// safeEmitLoadTestNotification piggy-backs on the existing
// argus:notification channel so the user sees a toast the moment the
// run finishes, even if their attention isn't on the load-test panel.
func (a *App) safeEmitLoadTestNotification(run *loadtestRun) {
	run.mu.RLock()
	state := run.state
	report := run.reportPath
	final := run.finalError
	run.mu.RUnlock()

	title := "Load test complete"
	body := "Report saved to " + report
	level := "info"
	switch state {
	case "canceled":
		title = "Load test canceled"
		body = "Run was canceled before completion."
		level = "warn"
	case "error":
		title = "Load test failed"
		body = final
		level = "error"
	}
	a.safeEmit("argus:notification", map[string]any{
		"title":  title,
		"body":   body,
		"level":  level,
		"source": "loadtest",
	})
}

// exportLoadTestReport renders the run record as Markdown and writes
// it to the Notebooks store. Filename pattern:
//
//	loadtest/<sanitized-name>-<ISO-timestamp>.md
//
// The "loadtest/" prefix puts every report in its own folder so the
// notebook tree doesn't fill up with one-off files.
func (a *App) exportLoadTestReport(rec *loadtest.RunRecord) (string, error) {
	if a.notebooks == nil {
		return "", fmt.Errorf("notebooks store not configured")
	}
	name := rec.Spec.Name
	if name == "" {
		name = string(rec.BrokerKind)
		if rec.Spec.Scale.Deployment != "" {
			name = name + "-" + rec.Spec.Scale.Deployment
		}
	}
	safe := sanitizeNotebookName(name)
	ts := rec.Started.UTC().Format("20060102-150405")
	path := fmt.Sprintf("loadtest/%s-%s.md", safe, ts)

	// PR-E: try to generate an LLM-authored narrative. Best-effort —
	// failures (no client, rate limit, network blip) degrade silently
	// to the deterministic render so the operator always gets a
	// report.
	narrative := a.tryNarrateLoadTest(rec)

	md := renderLoadTestMarkdown(rec, narrative)
	if err := a.notebooks.SaveFile(a.appCtx(), path, md); err != nil {
		return "", err
	}
	return path, nil
}

// tryNarrateLoadTest is the bridge from the App's ai.Agent to the
// loadtest analysis package. Returns an empty string when no client
// is configured or the LLM call fails — the caller renders without
// a Narrative section in that case.
//
// The call uses a 30-second timeout independent of the App context.
// A slow LLM endpoint should never delay the report past half a
// minute; the operator can re-narrate later via a future "Re-analyze"
// button (out of scope for PR-E).
func (a *App) tryNarrateLoadTest(rec *loadtest.RunRecord) string {
	if a.agent == nil {
		return ""
	}
	client := a.agent.Client()
	if client == nil {
		return ""
	}
	narrator := analysis.New(client)
	ctx, cancel := context.WithTimeout(a.appCtx(), 30*time.Second)
	defer cancel()
	out, err := narrator.Narrate(ctx, rec)
	if err != nil {
		a.logger.Info("loadtest: narrative skipped",
			slog.String("reason", err.Error()))
		return ""
	}
	return out
}

// sanitizeNotebookName removes filesystem-hostile characters and caps
// the length so a free-form Spec.Name can't produce a 200-char filename
// or one with slashes that escape the loadtest folder.
func sanitizeNotebookName(s string) string {
	if s == "" {
		return "run"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ' || r == '.':
			b.WriteRune('-')
		}
	}
	out := b.String()
	if out == "" {
		out = "run"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

// renderLoadTestMarkdown builds the on-disk report.
//
// narrative is the optional LLM-authored prose block from PR-E. When
// empty, the report omits the Narrative section entirely — frontmatter
// + structured sections remain so the file is still useful even with
// no AI client configured.
func renderLoadTestMarkdown(rec *loadtest.RunRecord, narrative string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("type: loadtest-report\n")
	b.WriteString(fmt.Sprintf("started: %s\n", rec.Started.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("finished: %s\n", rec.Finished.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("broker: %s\n", rec.BrokerKind))
	if rec.Spec.Scale.Deployment != "" {
		b.WriteString(fmt.Sprintf("deployment: %s/%s\n", rec.Spec.Scale.Namespace, rec.Spec.Scale.Deployment))
	}
	b.WriteString("---\n\n")

	b.WriteString("# Load test report\n\n")
	if rec.Spec.Name != "" {
		b.WriteString("**Name:** " + rec.Spec.Name + "\n\n")
	}
	b.WriteString(fmt.Sprintf("**Destination:** `%s`\n\n", rec.Spec.Destination))
	b.WriteString(fmt.Sprintf("**Payload:** %s, %d bytes\n\n", rec.Spec.Payload.Kind, rec.Spec.Payload.Size))
	b.WriteString(fmt.Sprintf("**Ramp:** %s\n\n", rec.Spec.Ramp.Kind))

	if narrative != "" {
		b.WriteString("## Narrative\n\n")
		b.WriteString(narrative)
		if !strings.HasSuffix(narrative, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("- Sent: **%d**\n", rec.Summary.Sent))
	b.WriteString(fmt.Sprintf("- Acked: **%d**\n", rec.Summary.Acked))
	b.WriteString(fmt.Sprintf("- Errors: **%d**\n", rec.Summary.Errors))
	b.WriteString(fmt.Sprintf("- Throughput: **%.1f msg/s**\n", rec.Summary.Throughput))
	b.WriteString(fmt.Sprintf("- P50 ack latency: %s\n", rec.Summary.P50AckLatency))
	b.WriteString(fmt.Sprintf("- P95 ack latency: %s\n", rec.Summary.P95AckLatency))
	b.WriteString(fmt.Sprintf("- P99 ack latency: %s\n", rec.Summary.P99AckLatency))
	b.WriteString(fmt.Sprintf("- Max ack latency: %s\n", rec.Summary.MaxAckLatency))
	b.WriteString(fmt.Sprintf("- Wall-clock duration: %s\n\n", rec.Summary.Duration))

	if len(rec.Summary.ErrorBreakdown) > 0 {
		b.WriteString("## Errors by kind\n\n")
		for k, v := range rec.Summary.ErrorBreakdown {
			b.WriteString(fmt.Sprintf("- `%s`: %d\n", k, v))
		}
		b.WriteString("\n")
	}

	if len(rec.ScaleLog) > 0 {
		b.WriteString("## Scale timeline\n\n")
		b.WriteString("| Time | Phase | Spec | Ready |\n")
		b.WriteString("| --- | --- | --- | --- |\n")
		for _, ev := range rec.ScaleLog {
			b.WriteString(fmt.Sprintf("| %s | %s | %d | %d |\n",
				ev.At.Format(time.RFC3339), ev.Phase, ev.Replicas, ev.Ready))
		}
		b.WriteString("\n")
	}

	if rec.FinalError != "" {
		b.WriteString("## Final error\n\n```\n" + rec.FinalError + "\n```\n\n")
	}

	// Stash the raw JSON record at the bottom so the agent in PR-E
	// (and any future tooling) can re-parse the run from the
	// notebook without a separate JSON sidecar file.
	b.WriteString("## Raw record (JSON)\n\n")
	b.WriteString("```json\n")
	enc, _ := json.MarshalIndent(rec, "", "  ")
	b.Write(enc)
	b.WriteString("\n```\n")
	return b.String()
}

// progressThrottler coalesces Sample callbacks into a periodic
// "progress" event the frontend can plot without re-rendering on every
// individual ack. The throttler aggregates the latest snapshot at the
// configured interval; if the engine completes between ticks, flush()
// forces one last emit so the chart's final point matches the summary.
type progressThrottler struct {
	app      *App
	runID    string
	period   time.Duration
	closeCh  chan struct{}
	tickerCh <-chan time.Time
	once     sync.Once

	mu        sync.Mutex
	samples   []loadtest.Sample
	scaleLog  []loadtest.ScaleEvent
}

func newProgressThrottler(a *App, runID string, period time.Duration) *progressThrottler {
	t := time.NewTicker(period)
	pt := &progressThrottler{
		app:      a,
		runID:    runID,
		period:   period,
		closeCh:  make(chan struct{}),
		tickerCh: t.C,
	}
	go func() {
		defer t.Stop()
		for {
			select {
			case <-pt.closeCh:
				return
			case <-pt.tickerCh:
				pt.emit()
			}
		}
	}()
	return pt
}

func (p *progressThrottler) onSample(s loadtest.Sample) {
	p.mu.Lock()
	p.samples = append(p.samples, s)
	p.mu.Unlock()
}

func (p *progressThrottler) onScale(e loadtest.ScaleEvent) {
	p.mu.Lock()
	p.scaleLog = append(p.scaleLog, e)
	p.mu.Unlock()
}

func (p *progressThrottler) emit() {
	p.mu.Lock()
	samples := p.samples
	scales := p.scaleLog
	p.samples = nil
	p.scaleLog = nil
	p.mu.Unlock()
	if len(samples) == 0 && len(scales) == 0 {
		return
	}
	p.app.safeEmit("argus:loadtest:progress", map[string]any{
		"runId":     p.runID,
		"samples":   samples,
		"scaleLog":  scales,
		"emittedAt": time.Now(),
	})
}

// flush triggers one final emit and stops the background ticker.
// Idempotent — safe to call multiple times.
func (p *progressThrottler) flush() {
	p.once.Do(func() {
		close(p.closeCh)
		p.emit()
	})
}
