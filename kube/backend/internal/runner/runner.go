// Package runner implements the distributed load-test runner that
// lives on GCP (Cloud Run). The desktop sends a RunnerSpec; the runner
// fans it out across regions, provisioning ephemeral spot GKE clusters
// + brokers via OpenTofu and Helm, running the load-test engine against
// each, then tearing everything down.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/argues/argus/internal/saasapi"
	"github.com/argues/argus/pkg/broker"
	"github.com/argues/argus/pkg/loadtest"
)

// Runner orchestrates a distributed load test across one or more
// regions. One Runner per run — not reusable.
type Runner struct {
	spec       saasapi.RunnerSpec
	logger     *slog.Logger
	workspace  string // directory for per-run tofu state
	modulePath string // path to runner-region tofu module

	mu      sync.RWMutex
	state   string // pending | running | canceling | done | error
	regions []regionState
	started time.Time
	result  *saasapi.RunnerResult

	// Stream is the event broadcaster. The HTTP handler attaches
	// SSE listeners to this.
	Stream *EventStream

	// cancel cancels all in-flight region work. Created in Run() and
	// called by Cancel() — propagates to every regionCtx in executeRegion.
	cancel context.CancelFunc
}

type regionState struct {
	name    string
	state   string // pending | provisioning | running | tearing_down | done | error
	err     string
	summary *saasapi.LoadSummary
}

// New creates a runner. modulePath is the filesystem path to the
// runner-region OpenTofu module. workspace is where per-run tofu
// state dirs are created.
func New(spec saasapi.RunnerSpec, modulePath, workspace string, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	regions := make([]regionState, len(spec.Regions))
	for i, r := range spec.Regions {
		regions[i] = regionState{name: r.Region, state: "pending"}
	}
	return &Runner{
		spec:       spec,
		logger:     logger.With("component", "runner", "runId", spec.RunID),
		workspace:  workspace,
		modulePath: modulePath,
		state:      "pending",
		regions:    regions,
		Stream:     NewEventStream(spec.RunID),
	}
}

// Run executes the full orchestration: provision → publish → teardown
// for each region, then aggregate the results. Blocks until all regions
// complete or the context is canceled.
func (r *Runner) Run(ctx context.Context) (*saasapi.RunnerResult, error) {
	// Derive a cancellable context so Cancel() can abort in-flight
	// regions. The caller's ctx still triggers cancellation too.
	ctx, r.cancel = context.WithCancel(ctx)

	r.mu.Lock()
	r.state = "running"
	r.started = time.Now()
	r.mu.Unlock()

	r.Stream.Emit(saasapi.RunnerEvent{
		RunID: r.spec.RunID, Type: saasapi.EventProvisioning,
		Message: "Starting distributed run", Timestamp: time.Now(),
	})

	// Fan-out: execute each region concurrently.
	type regionOut struct {
		index int
		rec   *loadtest.RunRecord
		err   error
	}
	results := make(chan regionOut, len(r.spec.Regions))
	var wg sync.WaitGroup

	for i, reg := range r.spec.Regions {
		wg.Add(1)
		go func(i int, reg saasapi.RegionSpec) {
			defer wg.Done()
			rec, err := r.executeRegion(ctx, i, reg)
			results <- regionOut{i, rec, err}
		}(i, reg)
	}
	wg.Wait()
	close(results)

	// Collect results.
	aggRegionResults := make([]saasapi.RunnerRegionResult, len(r.spec.Regions))
	var aggSent, aggAcked, aggErrors int
	for res := range results {
		rr := saasapi.RunnerRegionResult{Region: r.spec.Regions[res.index].Region}
		if res.err != nil {
			rr.Success = false
			rr.Error = res.err.Error()
			r.setRegionState(res.index, "error", res.err.Error())
		} else if res.rec != nil {
			rr.Success = true
			rr.Summary = toLoadSummary(res.rec.Summary)
			aggSent += res.rec.Summary.Sent
			aggAcked += res.rec.Summary.Acked
			aggErrors += res.rec.Summary.Errors
		}
		aggRegionResults[res.index] = rr
	}

	r.mu.Lock()
	r.state = "done"
	r.result = &saasapi.RunnerResult{
		RunID:   r.spec.RunID,
		State:   "done",
		Regions: aggRegionResults,
		Summary: &saasapi.LoadSummary{
			TotalSent:   aggSent,
			TotalAcked:  aggAcked,
			TotalErrors: aggErrors,
		},
		CreditsHeld: r.spec.CreditsHeld,
		StartedAt:   r.started,
		FinishedAt:  time.Now(),
	}
	// Check for cancellation.
	if ctx.Err() != nil {
		r.state = "canceled"
		r.result.State = "canceled"
	}
	result := r.result
	r.mu.Unlock()

	r.Stream.Emit(saasapi.RunnerEvent{
		RunID: r.spec.RunID, Type: saasapi.EventComplete,
		Summary: result.Summary, Timestamp: time.Now(),
	})
	r.Stream.Close()

	return result, nil
}

// State returns the current runner state.
func (r *Runner) State() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

// Result returns the final result. nil if not yet complete.
func (r *Runner) Result() *saasapi.RunnerResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.result
}

// Cancel signals all in-flight regions to stop and begin teardown.
// Cancels the derived context so every executeRegion goroutine sees
// ctx.Done() and begins its deferred cleanup.
func (r *Runner) Cancel() {
	r.mu.Lock()
	wasRunning := r.state == "running"
	r.state = "canceling"
	r.mu.Unlock()

	if wasRunning && r.cancel != nil {
		r.cancel()
		r.Stream.Emit(saasapi.RunnerEvent{
			RunID: r.spec.RunID, Type: "canceling",
			Message: "User requested cancellation", Timestamp: time.Now(),
		})
	}
}

func (r *Runner) setRegionState(index int, state, err string) {
	r.mu.Lock()
	r.regions[index].state = state
	r.regions[index].err = err
	r.mu.Unlock()
}

// executeRegion runs the full lifecycle for one region.
func (r *Runner) executeRegion(ctx context.Context, index int, reg saasapi.RegionSpec) (*loadtest.RunRecord, error) {
	regionCtx, regionCancel := context.WithCancel(ctx)
	defer regionCancel()

	regionLogger := r.logger.With("region", reg.Region)

	r.setRegionState(index, "provisioning", "")
	r.Stream.Emit(saasapi.RunnerEvent{
		RunID: r.spec.RunID, Type: saasapi.EventProvisioning,
		Region: reg.Region, Message: "Provisioning spot GKE cluster...",
		Timestamp: time.Now(),
	})

	cleanup := &cleanupState{runID: r.spec.RunID, region: reg.Region, workspace: r.workspace}
	defer func() {
		cleanup.run(regionLogger)
		if cleanup.workDir != "" {
			// Use a background context with timeout so deferred cleanup
			// still runs even when the run context is already cancelled,
			// but doesn't hang forever if tofu destroy stalls.
			destroyCtx, destroyCancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer destroyCancel()
			if err := r.tofuDestroy(destroyCtx, cleanup.workDir); err != nil {
				regionLogger.Warn("tofu destroy failed", "error", err)
			}
		}
	}()

	// 1. Provision via OpenTofu.
	endpoint, err := r.tofuApply(regionCtx, reg, cleanup)
	if err != nil {
		return nil, fmt.Errorf("provision %s: %w", reg.Region, err)
	}
	cleanup.provisioned = true

	r.Stream.Emit(saasapi.RunnerEvent{
		RunID: r.spec.RunID, Type: saasapi.EventProvisioned,
		Region: reg.Region, Message: "Cluster ready, connecting to broker...",
		Timestamp: time.Now(),
	})

	// 2. Build broker publisher from spec + provisioned endpoint.
	cfg, err := r.buildBrokerConfig(endpoint)
	if err != nil {
		return nil, fmt.Errorf("broker config: %w", err)
	}

	pub, err := broker.New(regionCtx, cfg, regionLogger)
	if err != nil {
		return nil, fmt.Errorf("broker new: %w", err)
	}
	if err := pub.Connect(regionCtx); err != nil {
		return nil, fmt.Errorf("broker connect: %w", err)
	}
	defer pub.Close()

	// 3. Convert runner spec to engine RunSpec.
	runSpec, err := r.toRunSpec(endpoint)
	if err != nil {
		return nil, fmt.Errorf("build runspec: %w", err)
	}

	// 4. Run load test.
	r.setRegionState(index, "running", "")
	engine := loadtest.New(runSpec, regionLogger)
	engine.OnSample(func(s loadtest.Sample) {
		r.Stream.Emit(saasapi.RunnerEvent{
			RunID: r.spec.RunID, Type: saasapi.EventProgress,
			Region: reg.Region,
			Progress: &saasapi.WorkerStatus{
				Region: reg.Region, Sent: 1, Acked: boolToInt(s.OK),
			},
			Timestamp: time.Now(),
		})
	})
	engine.OnScale(func(e loadtest.ScaleEvent) {
		r.Stream.Emit(saasapi.RunnerEvent{
			RunID: r.spec.RunID, Type: saasapi.EventScale,
			Region: reg.Region,
			Scale: &saasapi.RunnerScaleEvent{
				At: e.At, Phase: e.Phase,
				Replicas: e.Replicas, Ready: e.Ready,
			},
			Timestamp: time.Now(),
		})
	})

	rec, err := engine.Run(regionCtx)
	if err != nil {
		r.setRegionState(index, "error", err.Error())
		r.Stream.Emit(saasapi.RunnerEvent{
			RunID: r.spec.RunID, Type: saasapi.EventError,
			Region: reg.Region, Error: err.Error(),
			Timestamp: time.Now(),
		})
		return rec, err
	}

	r.setRegionState(index, "done", "")
	r.Stream.Emit(saasapi.RunnerEvent{
		RunID: r.spec.RunID, Type: saasapi.EventRegionDone,
		Region: reg.Region, Message: "Region complete",
		Summary: toLoadSummary(rec.Summary), Timestamp: time.Now(),
	})

	return rec, nil
}

// toRunSpec converts the runner spec into a loadtest.RunSpec for the
// engine. The broker config is resolved separately with the correct
// endpoint substituted from the provisioned infra.
func (r *Runner) toRunSpec(endpoint string) (loadtest.RunSpec, error) {
	ls := r.spec.LoadSpec
	if ls == nil {
		return loadtest.RunSpec{}, fmt.Errorf("loadSpec is required")
	}

	spec := loadtest.RunSpec{
		Name:        r.spec.Name,
		Destination: ls.Destination,
		Count:       ls.Count,
		Workers:     ls.Workers,
	}

	if ls.Ramp != nil {
		spec.Ramp = loadtest.Ramp{
			Rate: float64(ls.Ramp.Rate),
		}
		switch ls.Ramp.Profile {
		case "linear":
			spec.Ramp.Kind = loadtest.RampLinear
			spec.Ramp.RampTo = float64(ls.Ramp.RampTo)
			if ls.Ramp.DurationSec > 0 {
				spec.Ramp.Duration = time.Duration(ls.Ramp.DurationSec) * time.Second
			}
		case "step":
			spec.Ramp.Kind = loadtest.RampStep
			spec.Ramp.StepBy = float64(ls.Ramp.StepBy)
			if ls.Ramp.StepEvery > 0 {
				spec.Ramp.StepEvery = time.Duration(ls.Ramp.StepEvery) * time.Second
			}
		case "spike":
			spec.Ramp.Kind = loadtest.RampSpike
			spec.Ramp.SpikeCount = ls.Ramp.SpikeCount
			spec.Ramp.SpikeSize = ls.Ramp.SpikeSize
			if ls.Ramp.SpikeIdle > 0 {
				spec.Ramp.SpikeIdle = time.Duration(ls.Ramp.SpikeIdle) * time.Second
			}
		default:
			spec.Ramp.Kind = loadtest.RampConstant
		}
		if spec.Workers <= 0 {
			spec.Workers = 50
		}
	}

	if ls.Scale != nil {
		spec.Scale = loadtest.ScalePlan{
			PreScaleToZero: ls.Scale.PreScaleToZero,
			MinReplicas:    ls.Scale.MinReplicas,
		}
	}

	return spec, nil
}

// buildBrokerConfig takes the original spec.Broker JSON and substitutes
// the endpoint from the provisioned infra (in-cluster service address).
func (r *Runner) buildBrokerConfig(endpoint string) (broker.Config, error) {
	var cfg broker.Config
	if err := json.Unmarshal(r.spec.Broker, &cfg); err != nil {
		return broker.Config{}, err
	}

	// Substitute the in-cluster endpoint for connectable brokers.
	// PubSub uses the GCP project ID directly (no in-cluster address).
	if endpoint != "" && cfg.Kind != broker.KindPubSub {
		switch cfg.Kind {
		case broker.KindNATS:
			if cfg.NATS != nil {
				cfg.NATS.Servers = endpoint
			}
		case broker.KindKafka:
			if cfg.Kafka != nil {
				cfg.Kafka.BootstrapServers = endpoint
			}
		case broker.KindRabbitMQ:
			if cfg.RabbitMQ != nil {
				cfg.RabbitMQ.URL = endpoint
			}
		case broker.KindAMQP1:
			if cfg.AMQP1 != nil {
				cfg.AMQP1.URL = endpoint
			}
		case broker.KindREST:
			if cfg.REST != nil {
				cfg.REST.BaseURL = endpoint
			}
		}
	}

	return cfg, nil
}

func toLoadSummary(s loadtest.Summary) *saasapi.LoadSummary {
	return &saasapi.LoadSummary{
		TotalSent:    s.Sent,
		TotalAcked:   s.Acked,
		TotalErrors:  s.Errors,
		Throughput:   s.Throughput,
		P50LatencyMs: float64(s.P50AckLatency) / float64(time.Millisecond),
		P95LatencyMs: float64(s.P95AckLatency) / float64(time.Millisecond),
		P99LatencyMs: float64(s.P99AckLatency) / float64(time.Millisecond),
		DurationSec:  s.Duration.Seconds(),
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// cleanupState tracks whether tofu has been applied so the deferred
// cleanup knows whether to run tofu destroy or just clean up files.
type cleanupState struct {
	runID       string
	region      string
	workspace   string
	workDir     string
	provisioned bool
}

func (c *cleanupState) run(logger *slog.Logger) {
	if !c.provisioned || c.workDir == "" {
		return
	}
	logger.Info("tearing down region infrastructure",
		"region", c.region, "workDir", c.workDir)
	// Actual tofu destroy happens via the Runner's tofuDestroy method,
	// called from the deferred cleanup in executeRegion.
}

// UUID is re-exported so tests can generate run IDs without importing google/uuid.
var NewUUID = uuid.NewString
