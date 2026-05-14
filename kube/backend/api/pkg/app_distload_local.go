package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/argus/internal/saasapi"
	"github.com/argues/argus/pkg/broker"
	"github.com/argues/argus/pkg/loadtest"
	"github.com/argues/argus/pkg/loadtest/analysis"
)

// app_distload_local.go — local-runner plumbing for Distributed Load Test.
//
// Distributed Load Test is the single feature; this file owns the
// "Runner=local" branch. It hosts:
//   - the in-memory run registry shared with the SaaS branch in
//     app_distload.go (so a single runID name-space addresses either)
//   - the progress throttler that coalesces Sample callbacks into a
//     periodic argus:loadtest:progress event for the chart
//   - the DistLoadSpec → loadtest.RunSpec translation
//   - the Markdown report renderer + Notebooks export, unchanged from
//     the prior standalone Load Test feature
//
// We support exactly ONE active local run at a time. Concurrency on the
// engine side is fine (the engine has its own worker pool), but two
// concurrent runs would compete for consumer-side scaling and produce
// nonsense reports.

// loadtestRegistry holds local runs the App knows about. The active
// run (if any) is tracked separately so dispatch can reject a second
// concurrent local start with a clear message rather than silently
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

	mu         sync.RWMutex
	state      string
	finished   time.Time
	record     *loadtest.RunRecord
	lastScale  *loadtest.ScaleEvent
	reportPath string
	finalError string
	// presetID is the optional preset the user started from. Carried
	// through for audit logging only; the engine doesn't consult it.
	presetID string
	// name lets cancel/status emit a friendly title on the
	// notification.
	name string
}

// Registries and factories are keyed per *App so multiple parallel
// tests don't share state. Production has exactly one App.
var (
	loadtestRegMu sync.Mutex
	loadtestRegs  = map[*App]*loadtestRegistry{}

	loadtestFactoryMu sync.RWMutex
	loadtestFactories = map[*App]func(context.Context, broker.Config, *slog.Logger) (broker.Publisher, error){}
)

// setLoadtestPublisherFactory is the test-only hook for injecting a
// fake broker publisher into the engine before a local run starts.
// Production code does not call this — tests use it via the
// app_distload_local_test.go helpers.
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

func (r *loadtestRun) activeState() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// startLocalDistLoad runs the local engine for a DistLoadSpec. Returns
// the runID the caller hands back over Wails. Single-active-run policy
// is enforced here.
func (a *App) startLocalDistLoad(spec saasapi.DistLoadSpec) (string, error) {
	// Multi-step REST scenarios run on the SaaS workers — the desktop
	// has no scenario executor. Reject up front so the user picks Cloud
	// (or trims the scenario down to a single-endpoint REST config).
	if spec.Scenario != nil {
		return "", fmt.Errorf("scenario tests require the Cloud runner — switch Runner to Cloud regions, or remove the scenario to use a single-endpoint REST test")
	}
	runSpec, err := distLoadSpecToRunSpec(spec)
	if err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}
	if err := runSpec.Validate(); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	// Daily quota: 5/day for free tier, unlimited for Pro. We reserve a
	// slot atomically (SELECT COUNT + INSERT inside one transaction) so
	// two concurrent Start calls at the 4→5 boundary cannot both succeed.
	id := uuid.New().String()
	startedAt := time.Now()
	if _, limit, resetAt, qerr := a.reserveLocalQuotaSlot(id, startedAt); qerr != nil {
		if errors.Is(qerr, ErrLocalQuotaExceeded) {
			return "", fmt.Errorf("local load tests are limited to %d/day on the free tier (resets at %s) — upgrade to Pro for unlimited local runs",
				limit, time.Unix(resetAt, 0).Format(time.RFC3339))
		}
		// Storage hiccup: log and continue rather than block the user.
		a.logger.Warn("loadtest: quota reserve failed; allowing run",
			slog.String("error", qerr.Error()))
	}

	reg := a.loadtests()
	reg.mu.Lock()
	if reg.activeID != "" {
		existing := reg.runs[reg.activeID]
		if existing != nil && existing.activeState() == "running" {
			reg.mu.Unlock()
			// We already inserted a quota row; refund it so a "busy"
			// rejection doesn't burn the user's allowance.
			a.refundLocalQuotaSlot(id)
			return "", fmt.Errorf("a load test is already running (id=%s); cancel it first", reg.activeID)
		}
	}
	ctx, cancel := context.WithCancel(a.appCtx())
	run := &loadtestRun{
		id:       id,
		spec:     runSpec,
		cancel:   cancel,
		started:  startedAt,
		state:    "pending",
		presetID: spec.PresetID,
		name:     spec.Name,
	}
	engine := loadtest.New(runSpec, a.logger.With("component", "loadtest", "runId", id))
	if a.k8s != nil {
		engine.Scaler = loadtest.NewKubeScaler(a.k8s.GetClientset(), 0)
	}
	if f := a.loadtestPublisherFactory(); f != nil {
		engine.NewPublisher = f
	}
	// Throttle live progress emits so a 1M-message run doesn't drown
	// the WebView in events. 250 ms ≈ 4 fps, plenty for the chart.
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

	// Quota slot already reserved above (inside the atomic
	// reserveLocalQuotaSlot transaction) — no second insert.

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

	a.safeEmit("argus:loadtest:done", a.distLoadStatusOfLocal(run))
	a.safeEmitLoadTestNotification(run)
}

// localRun looks up a run in the local registry by ID. ok=false means
// the ID isn't a local one and the SaaS path should handle it.
func (a *App) localRun(runID string) (*loadtestRun, bool) {
	reg := a.loadtests()
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	r, ok := reg.runs[runID]
	return r, ok
}

// distLoadStatusOfLocal maps a local loadtestRun onto saasapi.DistLoadStatus
// so the frontend uses one status shape for both runner modes.
//
// Local fields:
//   - ProvisionProgress is nil (no VMs).
//   - Workers carries a single-element slice with Region="local" and
//     the engine's per-worker aggregate (which is the run aggregate
//     since local has one "region").
//   - Credits* are 0 (local doesn't burn SaaS credits).
func (a *App) distLoadStatusOfLocal(run *loadtestRun) *saasapi.DistLoadStatus {
	run.mu.RLock()
	defer run.mu.RUnlock()

	var summary loadtest.Summary
	if run.record != nil {
		summary = run.record.Summary
	} else if run.engine != nil {
		samples, _ := run.engine.Snapshot()
		summary = loadtest.Aggregate(samples)
	}

	worker := saasapi.WorkerStatus{
		Region:     "local",
		Sent:       summary.Sent,
		Acked:      summary.Acked,
		Errors:     summary.Errors,
		Throughput: summary.Throughput,
		P50Ms:      float64(summary.P50AckLatency) / float64(time.Millisecond),
		P95Ms:      float64(summary.P95AckLatency) / float64(time.Millisecond),
		P99Ms:      float64(summary.P99AckLatency) / float64(time.Millisecond),
		State:      run.state,
	}

	st := &saasapi.DistLoadStatus{
		RunID:      run.id,
		State:      run.state,
		Name:       run.name,
		Workers:    []saasapi.WorkerStatus{worker},
		Error:      run.finalError,
		StartedAt:  run.started,
		FinishedAt: run.finished,
	}
	if run.record != nil {
		st.Summary = &saasapi.LoadSummary{
			TotalSent:    summary.Sent,
			TotalAcked:   summary.Acked,
			TotalErrors:  summary.Errors,
			Throughput:   summary.Throughput,
			P50LatencyMs: float64(summary.P50AckLatency) / float64(time.Millisecond),
			P95LatencyMs: float64(summary.P95AckLatency) / float64(time.Millisecond),
			P99LatencyMs: float64(summary.P99AckLatency) / float64(time.Millisecond),
			DurationSec:  summary.Duration.Seconds(),
		}
	}
	return st
}

// safeEmit wraps runtime.EventsEmit so tests (and any production code
// path running before Wails Startup has installed the runtime context)
// don't see "An invalid context was passed" logs.
func (a *App) safeEmit(event string, payload any) {
	if a.ctx == nil {
		return
	}
	defer func() { _ = recover() }()
	runtime.EventsEmit(a.ctx, event, payload)
}

// safeEmitLoadTestNotification piggy-backs on the argus:notification
// channel so the user sees a toast the moment the run finishes, even
// if their attention isn't on the load-test panel.
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

// distLoadSpecToRunSpec translates the SaaS-shaped DistLoadSpec into
// the engine's RunSpec. The translation deliberately keeps the engine
// ignorant of cloud-only fields (regions, instance types) — those are
// rejected only if the caller routed a clearly-cloud spec into the
// local path.
//
// We resolve the raw broker JSON directly into broker.Config; the
// frontend posts the same shape pkg/broker uses, just nested as
// DistLoadSpec.Broker.
func distLoadSpecToRunSpec(spec saasapi.DistLoadSpec) (loadtest.RunSpec, error) {
	if spec.Destination == "" {
		return loadtest.RunSpec{}, fmt.Errorf("destination required")
	}
	if spec.Count <= 0 {
		return loadtest.RunSpec{}, fmt.Errorf("count must be > 0")
	}
	var cfg broker.Config
	if len(spec.Broker) == 0 {
		return loadtest.RunSpec{}, fmt.Errorf("broker config required")
	}
	if err := json.Unmarshal(spec.Broker, &cfg); err != nil {
		return loadtest.RunSpec{}, fmt.Errorf("broker config: %w", err)
	}

	// Payload: when the new Payload block is set, the source dictates
	// shape; otherwise fall back to the legacy PayloadSize byte-filler
	// (cloud back-compat) so older frontends still work.
	payload, err := resolvePayload(spec)
	if err != nil {
		return loadtest.RunSpec{}, err
	}

	ramp := buildRamp(spec)

	rs := loadtest.RunSpec{
		Name:        spec.Name,
		Broker:      cfg,
		Destination: spec.Destination,
		Payload:     payload,
		Count:       spec.Count,
		Workers:     spec.Workers,
		Ramp:        ramp,
	}
	return rs, nil
}

// buildRamp resolves the user-facing ramp configuration into a
// loadtest.Ramp. Precedence: the nested spec.Ramp block (rich, sent by
// the unified form) wins; the flat RampProfile/RampRate/TimeoutMins
// fields are the back-compat fallback used by older clients and the
// cloud worker payload.
func buildRamp(spec saasapi.DistLoadSpec) loadtest.Ramp {
	// Rich path: nested Ramp block carries per-profile knobs verbatim.
	if r := spec.Ramp; r != nil {
		out := loadtest.Ramp{Rate: float64(r.Rate)}
		if r.DurationSec > 0 {
			out.Duration = time.Duration(r.DurationSec) * time.Second
		}
		switch strings.ToLower(r.Profile) {
		case "linear":
			out.Kind = loadtest.RampLinear
			out.RampTo = float64(r.RampTo)
		case "step":
			out.Kind = loadtest.RampStep
			out.StepBy = float64(r.StepBy)
			if r.StepEvery > 0 {
				out.StepEvery = time.Duration(r.StepEvery) * time.Second
			}
		case "spike":
			out.Kind = loadtest.RampSpike
			out.SpikeCount = r.SpikeCount
			out.SpikeSize = r.SpikeSize
			if r.SpikeIdle > 0 {
				out.SpikeIdle = time.Duration(r.SpikeIdle) * time.Second
			}
		default:
			out.Kind = loadtest.RampConstant
			if out.Rate <= 0 {
				out.Rate = 100
			}
		}
		return out
	}

	// Back-compat path: only the flat fields are populated.
	out := loadtest.Ramp{Rate: float64(spec.RampRate)}
	switch strings.ToLower(spec.RampProfile) {
	case "linear":
		out.Kind = loadtest.RampLinear
		out.RampTo = float64(spec.RampRate)
		if spec.TimeoutMins > 0 {
			out.Duration = time.Duration(spec.TimeoutMins) * time.Minute
		}
	case "step":
		out.Kind = loadtest.RampStep
		out.StepBy = float64(spec.RampRate) / 4
		out.StepEvery = 30 * time.Second
	case "spike":
		out.Kind = loadtest.RampSpike
		out.SpikeCount = 6
		out.SpikeSize = spec.Count / 6
		out.SpikeIdle = 30 * time.Second
	default:
		out.Kind = loadtest.RampConstant
		if out.Rate <= 0 {
			out.Rate = 100
		}
		if spec.TimeoutMins > 0 {
			out.Duration = time.Duration(spec.TimeoutMins) * time.Minute
		}
	}
	return out
}

// resolvePayload builds a loadtest.Payload from a DistLoadSpec. Three
// shapes:
//
//   - spec.Payload nil → legacy filler based on spec.PayloadSize
//   - spec.Payload set, Source in upload/paste/type/ai → inline Bytes
//   - spec.Payload set, Source=="file" → read file or dir contents.
//     FileMode "template" picks the first file's bytes (engine repeats
//     it); "exact" reads every listed file into SamplePool for
//     round-robin send.
//
// File paths are revalidated here (the resolver method enforced the
// same rules, but a direct Start call shouldn't trust the frontend).
func resolvePayload(spec saasapi.DistLoadSpec) (loadtest.Payload, error) {
	if spec.Payload == nil {
		size := spec.PayloadSize
		if size <= 0 {
			size = 256
		}
		return loadtest.Payload{
			Kind:  loadtest.PayloadKindTyped,
			Bytes: bytes.Repeat([]byte{'x'}, size),
			Size:  size,
		}, nil
	}
	p := spec.Payload
	switch p.Source {
	case "upload", "paste", "type", "ai":
		b := []byte(p.Bytes)
		if len(b) == 0 {
			return loadtest.Payload{}, fmt.Errorf("payload bytes empty for source %q", p.Source)
		}
		return loadtest.Payload{
			Kind:     mapPayloadKind(p.Source),
			Bytes:    b,
			Filename: p.Filename,
			Size:     len(b),
		}, nil
	case "file":
		return resolveFilePayload(p)
	default:
		return loadtest.Payload{}, fmt.Errorf("unknown payload source %q", p.Source)
	}
}

func mapPayloadKind(src string) loadtest.PayloadKind {
	switch src {
	case "upload":
		return loadtest.PayloadKindUploaded
	case "paste":
		return loadtest.PayloadKindPasted
	case "type":
		return loadtest.PayloadKindTyped
	case "ai":
		return loadtest.PayloadKindAI
	case "file":
		return loadtest.PayloadKindFile
	default:
		return loadtest.PayloadKindTyped
	}
}

// resolveFilePayload reads from the user's filesystem. The path is
// re-validated through checkPayloadPath so a caller bypassing the
// ResolveLocalPayloadPath RPC can't escape the sandbox.
func resolveFilePayload(p *saasapi.DistLoadPayload) (loadtest.Payload, error) {
	if p.FilePath == "" {
		return loadtest.Payload{}, fmt.Errorf("filePath required for source=\"file\"")
	}
	clean, err := checkPayloadPath(p.FilePath)
	if err != nil {
		return loadtest.Payload{}, err
	}
	info, err := os.Stat(clean)
	if err != nil {
		return loadtest.Payload{}, fmt.Errorf("stat %s: %w", clean, err)
	}
	mode := p.FileMode
	if mode == "" {
		mode = "template"
	}

	if info.IsDir() {
		entries, err := os.ReadDir(clean)
		if err != nil {
			return loadtest.Payload{}, fmt.Errorf("read dir %s: %w", clean, err)
		}
		var paths []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			lower := strings.ToLower(e.Name())
			if !strings.HasSuffix(lower, ".json") && !strings.HasSuffix(lower, ".txt") {
				continue
			}
			fi, err := e.Info()
			if err != nil || fi.Size() > resolvePayloadMaxFileSize {
				continue
			}
			paths = append(paths, filepath.Join(clean, e.Name()))
			if len(paths) >= resolvePayloadMaxFiles {
				break
			}
		}
		if len(paths) == 0 {
			return loadtest.Payload{}, fmt.Errorf("no .json/.txt files in %s", clean)
		}
		sort.Strings(paths)
		switch mode {
		case "template":
			b, err := os.ReadFile(paths[0])
			if err != nil {
				return loadtest.Payload{}, fmt.Errorf("read %s: %w", paths[0], err)
			}
			return loadtest.Payload{
				Kind:     loadtest.PayloadKindFile,
				Bytes:    b,
				Filename: filepath.Base(clean),
				Size:     len(b),
			}, nil
		case "exact":
			pool := make([][]byte, 0, len(paths))
			for _, fp := range paths {
				b, err := os.ReadFile(fp)
				if err != nil {
					continue
				}
				pool = append(pool, b)
			}
			if len(pool) == 0 {
				return loadtest.Payload{}, fmt.Errorf("no readable files in %s", clean)
			}
			return loadtest.Payload{
				Kind:       loadtest.PayloadKindFile,
				SamplePool: pool,
				Bytes:      pool[0], // backup body in case the engine ignores the pool
				Filename:   filepath.Base(clean),
				Size:       len(pool[0]),
			}, nil
		default:
			return loadtest.Payload{}, fmt.Errorf("unknown fileMode %q (want \"exact\" or \"template\")", mode)
		}
	}

	// Single file: ignore mode (template and exact collapse to the
	// same shape — one body the engine repeats).
	if info.Size() > resolvePayloadMaxFileSize {
		return loadtest.Payload{}, fmt.Errorf("file %s exceeds %d bytes", clean, resolvePayloadMaxFileSize)
	}
	b, err := os.ReadFile(clean)
	if err != nil {
		return loadtest.Payload{}, fmt.Errorf("read %s: %w", clean, err)
	}
	return loadtest.Payload{
		Kind:     loadtest.PayloadKindFile,
		Bytes:    b,
		Filename: info.Name(),
		Size:     len(b),
	}, nil
}

// exportLoadTestReport renders the run record as Markdown and writes
// it to the Notebooks store under loadtest/<sanitized-name>-<ts>.md.
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

	narrative := a.tryNarrateLoadTest(rec)

	md := renderLoadTestMarkdown(rec, narrative)
	if err := a.notebooks.SaveFile(a.appCtx(), path, md); err != nil {
		return "", err
	}
	return path, nil
}

// tryNarrateLoadTest is the bridge from the App's ai.Agent to the
// loadtest analysis package. Empty return = no narrative section.
//
// 30-second cap so a slow LLM endpoint never blocks the report path.
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
// length so a free-form Spec.Name can't produce a 200-char filename or
// one with slashes that escape the loadtest folder.
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
// narrative is the optional LLM-authored prose block. When empty, the
// report omits the Narrative section entirely; the rest of the file
// stays useful with no AI client configured.
func renderLoadTestMarkdown(rec *loadtest.RunRecord, narrative string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("type: loadtest-report\n")
	fmt.Fprintf(&b, "started: %s\n", rec.Started.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "finished: %s\n", rec.Finished.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "broker: %s\n", rec.BrokerKind)
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

	// Stash the raw JSON record at the bottom so the AI narrator (and
	// any future tooling) can re-parse the run from the notebook
	// without a separate JSON sidecar.
	b.WriteString("## Raw record (JSON)\n\n")
	b.WriteString("```json\n")
	enc, _ := json.MarshalIndent(rec, "", "  ")
	b.Write(enc)
	b.WriteString("\n```\n")
	return b.String()
}

// progressThrottler coalesces Sample callbacks into a periodic
// argus:loadtest:progress event the frontend can plot without
// re-rendering on every individual ack. Aggregates the latest snapshot
// at the configured interval; if the engine completes between ticks,
// flush() forces one last emit so the chart's final point matches the
// summary.
type progressThrottler struct {
	app      *App
	runID    string
	period   time.Duration
	closeCh  chan struct{}
	tickerCh <-chan time.Time
	once     sync.Once

	mu       sync.Mutex
	samples  []loadtest.Sample
	scaleLog []loadtest.ScaleEvent
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
