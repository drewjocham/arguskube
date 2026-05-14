package loadtest

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/argues/argus/pkg/broker"
)

const defaultWorkers = 50

// Engine runs a load test. Construct once per run — Engine instances
// are NOT reusable. The frontend (via PR-C) calls Run synchronously
// from a background goroutine; progress comes through the recorder
// snapshot facility, not callbacks on Engine.
type Engine struct {
	spec   RunSpec
	logger *slog.Logger
	rec    *recorder

	// Optional. Engine.Run uses these directly; they're exposed as
	// fields so tests can inject fakes without going through New().
	NewPublisher func(ctx context.Context, cfg broker.Config, logger *slog.Logger) (broker.Publisher, error)
	Scaler       Scaler

	// onSample is called for every Sample as it's recorded. Used by
	// PR-C to push live progress events to the frontend. Nil-safe.
	onSample func(Sample)

	// onScale is the same for ScaleEvents.
	onScale func(ScaleEvent)
}

// New builds an Engine. The frontend supplies the spec; the engine
// supplies sensible defaults for unset fields.
func New(spec RunSpec, logger *slog.Logger) *Engine {
	if spec.Workers <= 0 {
		spec.Workers = defaultWorkers
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		spec:         spec,
		logger:       logger,
		rec:          newRecorder(),
		NewPublisher: broker.New,
	}
}

// OnSample registers a callback fired for every recorded Sample.
func (e *Engine) OnSample(fn func(Sample)) { e.onSample = fn }

// OnScale registers a callback fired for every ScaleEvent.
func (e *Engine) OnScale(fn func(ScaleEvent)) { e.onScale = fn }

// Snapshot returns the current state of the run record. Safe to call
// from any goroutine; concurrent with Run.
func (e *Engine) Snapshot() ([]Sample, []ScaleEvent) {
	return e.rec.snapshot()
}

// Run executes the load test end to end. Returns the final RunRecord
// once publishing + optional drain observation have completed, or an
// error if the spec was invalid / pre-flight failed / context cancelled
// before publishing began.
//
// Errors during the publish phase do NOT cause Run to return early —
// they're recorded as Sample.Err and counted in Summary.Errors. The
// engine's job is to publish N messages and report what happened, not
// to give up on the first broker hiccup.
func (e *Engine) Run(ctx context.Context) (*RunRecord, error) {
	if err := e.spec.Validate(); err != nil {
		return nil, fmt.Errorf("spec invalid: %w", err)
	}
	started := time.Now()

	// 1. Pre-scale to zero, if requested. Bail out if it doesn't
	//    reach zero in the configured window — running the publish
	//    phase against a still-active consumer defeats the whole
	//    cold-start measurement.
	if err := e.preScale(ctx); err != nil {
		return e.recordError(started, err), nil
	}

	// 2. Build the broker publisher.
	pub, err := e.NewPublisher(ctx, e.spec.Broker, e.logger)
	if err != nil {
		return e.recordError(started, fmt.Errorf("broker new: %w", err)), nil
	}
	if err := pub.Connect(ctx); err != nil {
		return e.recordError(started, fmt.Errorf("broker connect: %w", err)), nil
	}
	defer pub.Close()

	// 3. Publish phase.
	if err := e.publish(ctx, pub); err != nil {
		// publish only returns ctx.Err() — per-message errors are
		// in the recorder, not returned.
		return e.recordError(started, err), nil
	}

	// 4. Post-scale up + observe drain.
	if err := e.postScale(ctx); err != nil {
		// Non-fatal: we still produce a record. Stamp the error
		// into the record but treat the run as having completed.
		e.logger.Warn("post-scale failed; report still produced",
			slog.String("error", err.Error()))
	}

	finished := time.Now()
	samples, scaleLog := e.rec.snapshot()
	return &RunRecord{
		Spec:       e.spec,
		BrokerKind: e.spec.Broker.Kind,
		Started:    started,
		Finished:   finished,
		Samples:    samples,
		ScaleLog:   scaleLog,
		Summary:    Aggregate(samples),
	}, nil
}

// publish drives the worker pool. Returns ctx.Err() if the caller
// canceled; otherwise returns nil after the schedule is exhausted.
func (e *Engine) publish(ctx context.Context, pub broker.Publisher) error {
	planner := newRampPlanner(e.spec.Ramp, e.spec.Count)
	schedule := planner.schedule()
	if len(schedule) == 0 {
		return nil
	}

	startWall := time.Now()
	// Bounded queue: schedule[i] is offered to a worker via the
	// channel, with a buffered depth equal to Workers so the
	// scheduler doesn't block the timing.
	jobs := make(chan int, e.spec.Workers)

	var wg sync.WaitGroup
	for w := 0; w < e.spec.Workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				e.publishOne(ctx, pub, i)
			}
		}()
	}

	// Scheduler goroutine — paces the queue per the schedule. We
	// drive it from this goroutine (no extra goroutine) because the
	// scheduler is purely time-driven and doesn't compete with the
	// workers for CPU.
	for i, offset := range schedule {
		target := startWall.Add(offset)
		if d := time.Until(target); d > 0 {
			select {
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				return ctx.Err()
			case <-time.After(d):
			}
		}
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return ctx.Err()
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
	return nil
}

func (e *Engine) publishOne(ctx context.Context, pub broker.Publisher, _ int) {
	msg := broker.Message{
		Destination: e.spec.Destination,
		Payload:     e.spec.Payload.Bytes,
	}
	start := time.Now()
	receipt, err := pub.Publish(ctx, msg)
	s := Sample{At: start, OK: err == nil}
	if err != nil {
		s.Err = err.Error()
	} else {
		// Trust the adapter's measurement, not our wall clock —
		// the adapter knows when the ack arrived, we just know
		// when the call returned.
		s.AckLatency = receipt.AckLatency
	}
	e.rec.addSample(s)
	if e.onSample != nil {
		e.onSample(s)
	}
}

// preScale handles the optional "scale consumer to zero before
// publishing" dance. No-op when the plan is empty or PreScaleToZero
// is false.
func (e *Engine) preScale(ctx context.Context) error {
	sp := e.spec.Scale
	if !sp.PreScaleToZero || sp.Deployment == "" {
		return nil
	}
	if e.Scaler == nil {
		return fmt.Errorf("scale plan set but no Scaler configured")
	}
	timeout := sp.PreScaleTimeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	e.observe(tctx, "pre-scale")
	if err := e.Scaler.Scale(tctx, sp.Namespace, sp.Deployment, 0); err != nil {
		return fmt.Errorf("scale to 0: %w", err)
	}
	if err := e.Scaler.WaitForReplicas(tctx, sp.Namespace, sp.Deployment, 0); err != nil {
		return fmt.Errorf("wait for 0 replicas: %w", err)
	}
	e.observe(tctx, "publishing")
	return nil
}

// postScale handles scale-up + readiness observation. Best-effort —
// failures here don't fail the run; they're logged and the record
// still ships.
func (e *Engine) postScale(ctx context.Context) error {
	sp := e.spec.Scale
	if sp.MinReplicas <= 0 || sp.Deployment == "" {
		return nil
	}
	if e.Scaler == nil {
		return fmt.Errorf("scale plan set but no Scaler configured")
	}
	timeout := sp.PostScaleTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := e.Scaler.Scale(tctx, sp.Namespace, sp.Deployment, sp.MinReplicas); err != nil {
		return fmt.Errorf("scale up: %w", err)
	}
	e.observe(tctx, "scaling-up")
	if err := e.Scaler.WaitForReplicas(tctx, sp.Namespace, sp.Deployment, sp.MinReplicas); err != nil {
		return fmt.Errorf("wait for %d replicas: %w", sp.MinReplicas, err)
	}
	e.observe(tctx, "draining")
	// Coarse drain observation: every 5 seconds for up to a minute
	// after readiness. The agent uses this curve to comment on
	// drain rate.
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	deadline := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			e.observe(ctx, "draining")
		}
	}
	e.observe(ctx, "done")
	return nil
}

// observe captures a single ScaleEvent. Silent on error — partial
// records are better than no records when the cluster connection
// blips mid-run.
func (e *Engine) observe(ctx context.Context, phase string) {
	if e.Scaler == nil {
		return
	}
	sp := e.spec.Scale
	specRep, ready, err := e.Scaler.Observe(ctx, sp.Namespace, sp.Deployment)
	if err != nil {
		return
	}
	ev := ScaleEvent{
		At:       time.Now(),
		Phase:    phase,
		Replicas: specRep,
		Ready:    ready,
	}
	e.rec.addScale(ev)
	if e.onScale != nil {
		e.onScale(ev)
	}
}

func (e *Engine) recordError(started time.Time, err error) *RunRecord {
	samples, scaleLog := e.rec.snapshot()
	return &RunRecord{
		Spec:       e.spec,
		BrokerKind: e.spec.Broker.Kind,
		Started:    started,
		Finished:   time.Now(),
		Samples:    samples,
		ScaleLog:   scaleLog,
		Summary:    Aggregate(samples),
		FinalError: err.Error(),
	}
}
