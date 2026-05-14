// Package loadtest is the broker-agnostic load-test engine.
//
// What it does:
//
//   - Take a RunSpec (broker config + payload + count + ramp profile +
//     optional Kubernetes scale plan)
//   - Optionally scale a target Deployment to 0 and wait for replicas
//     to actually be zero (not just spec, ACTUAL count from
//     status.readyReplicas)
//   - Publish N messages through a worker pool, paced by a token-bucket
//     limiter driven by the ramp profile
//   - Optionally scale the Deployment back to the configured minimum
//     and wait for readiness, then keep observing drain behavior
//   - Aggregate per-message latencies into P50/P95/P99 + throughput +
//     error counts + a coarse time-series of per-second throughput
//   - Return a RunRecord that PR-C will pass to the agent + Notebooks
//
// What it does NOT do: ship its own brokers (PR-A) or render its own
// UI (PR-D). The engine takes a broker.Publisher and a Kubernetes
// scaler interface and is fully testable with fakes for both.
package loadtest

import (
	"errors"
	"fmt"
	"time"

	"github.com/argues/argus/pkg/broker"
)

// PayloadKind names how the user supplied the payload. The engine
// itself only ever sees Bytes — the frontend conversion happens before
// the spec is built — but we keep the kind in the record so the
// Notebook export can label the run faithfully.
type PayloadKind string

const (
	PayloadKindUploaded PayloadKind = "uploaded" // user uploaded a file
	PayloadKindPasted   PayloadKind = "pasted"   // user pasted JSON in the textarea
	PayloadKindTyped    PayloadKind = "typed"    // user typed it from scratch
)

// Payload is the message body the engine publishes. The frontend
// hands the engine the resolved bytes via Bytes; the optional Source
// fields are persisted into the run record so the agent's report can
// say "user uploaded payload from foo.json (1.2 KB)" rather than just
// "payload of 1247 bytes".
type Payload struct {
	Kind  PayloadKind `json:"kind"`
	Bytes []byte      `json:"-"`
	// Filename is the original upload filename when Kind=="uploaded".
	// Empty otherwise.
	Filename string `json:"filename,omitempty"`
	// Size in bytes — kept here so the JSON record can render it
	// without callers re-deriving it from a hidden Bytes field.
	Size int `json:"size"`
}

// RampKind names the shape of message rate over time. Each named
// preset (see presets.go) maps onto one of these.
type RampKind string

const (
	RampConstant RampKind = "constant" // fixed rate for the duration
	RampLinear   RampKind = "linear"   // RampFrom → RampTo over Duration
	RampStep     RampKind = "step"     // every StepEvery, rate += StepBy
	RampSpike    RampKind = "spike"    // SpikeCount bursts of SpikeSize, idle SpikeIdle between
)

// Ramp encodes how the engine emits messages over time. The total
// number of messages is RunSpec.Count; Ramp.Rate (or its evolution
// over time) determines pacing.
//
// We deliberately keep Ramp simple. Each kind only uses some fields,
// flagged in the docstrings. Validation in Validate() rejects mismatched
// shapes with a precise error so the frontend can highlight the wrong
// input.
type Ramp struct {
	Kind RampKind `json:"kind"`

	// Constant + Step + Linear all use Duration as an upper bound
	// (the run stops when EITHER Count is reached OR Duration
	// elapses, whichever first).
	Duration time.Duration `json:"durationNs,omitempty"`

	// Constant + the start of Linear/Step.
	Rate float64 `json:"rate,omitempty"` // msgs/sec

	// Linear-only: target rate at end of Duration.
	RampTo float64 `json:"rampTo,omitempty"`

	// Step-only: at every StepEvery interval, rate increases by StepBy.
	StepEvery time.Duration `json:"stepEveryNs,omitempty"`
	StepBy    float64       `json:"stepBy,omitempty"`

	// Spike-only: SpikeCount bursts, each emitting SpikeSize messages
	// instantly, with SpikeIdle between bursts. Count is unused for
	// spike — the burst plan determines the total.
	SpikeCount int           `json:"spikeCount,omitempty"`
	SpikeSize  int           `json:"spikeSize,omitempty"`
	SpikeIdle  time.Duration `json:"spikeIdleNs,omitempty"`
}

// ScalePlan describes the consumer-side autoscaling dance: scale to
// zero before the run starts so messages back up in the broker, then
// scale up to MinReplicas at TriggerAt of the way through (default:
// after all messages are published) so the engine can measure drain.
//
// Empty ScalePlan = no scaling, just publish. Used by the Smoke preset
// and for users who don't want the cold-start dance.
type ScalePlan struct {
	// Namespace + Deployment identify the target. Both required when
	// any other field is set.
	Namespace  string `json:"namespace,omitempty"`
	Deployment string `json:"deployment,omitempty"`

	// PreScaleToZero, when true, scales the deployment to 0 and
	// waits for the actual replicas to reach 0 before publishing.
	PreScaleToZero bool `json:"preScaleToZero,omitempty"`

	// MinReplicas is the scale-up target after the publish phase.
	// 0 disables post-publish scaling (you scale down but never
	// scale back — useful for a backlog-only test).
	MinReplicas int32 `json:"minReplicas,omitempty"`

	// PreScaleTimeout caps how long we wait for the 0-replica
	// observation. Default is applied in Validate when zero.
	PreScaleTimeout time.Duration `json:"preScaleTimeoutNs,omitempty"`

	// PostScaleTimeout caps the wait for MinReplicas ready pods
	// after the scale-up. Default is applied in Validate.
	PostScaleTimeout time.Duration `json:"postScaleTimeoutNs,omitempty"`
}

// RunSpec is the full description of a load test, end to end. Built
// by the frontend and posted to PR-C's StartLoadTest Wails binding.
type RunSpec struct {
	// Name is a human-readable label — appears in the Notebook
	// filename and the run-record. Optional; engine falls back to
	// "<kind>-<deployment>-<timestamp>" if empty.
	Name string `json:"name,omitempty"`

	Broker broker.Config `json:"broker"`

	// Destination overrides the per-message Destination so the
	// frontend doesn't repeat itself for every message. The engine
	// stamps this into broker.Message.Destination at publish time.
	Destination string `json:"destination"`

	Payload Payload `json:"payload"`

	// Count caps the publish phase. Reaching Count or hitting
	// Ramp.Duration (whichever first) stops the publisher.
	Count int `json:"count"`

	// Workers caps the number of concurrent in-flight publishes.
	// Default 50 when zero.
	Workers int `json:"workers,omitempty"`

	Ramp  Ramp      `json:"ramp"`
	Scale ScalePlan `json:"scale"`
}

// Validate returns the first reason the spec is unusable, or nil. The
// frontend should also do shape validation, but Validate is the
// authoritative check the engine runs before doing any work — keeps
// bad specs from causing partial side effects (scale-down without a
// publish, for example).
func (s *RunSpec) Validate() error {
	if s.Count <= 0 && s.Ramp.Kind != RampSpike {
		return errors.New("count must be > 0")
	}
	if s.Destination == "" {
		return errors.New("destination required")
	}
	if len(s.Payload.Bytes) == 0 {
		return errors.New("payload bytes required")
	}
	if _, err := s.Broker.Resolve(); err != nil {
		return fmt.Errorf("broker config: %w", err)
	}
	if err := s.Ramp.validate(); err != nil {
		return fmt.Errorf("ramp: %w", err)
	}
	if err := s.Scale.validate(); err != nil {
		return fmt.Errorf("scale: %w", err)
	}
	return nil
}

func (r *Ramp) validate() error {
	switch r.Kind {
	case RampConstant:
		if r.Rate <= 0 {
			return errors.New("constant ramp needs Rate > 0")
		}
	case RampLinear:
		if r.Rate <= 0 || r.RampTo <= 0 {
			return errors.New("linear ramp needs Rate and RampTo > 0")
		}
		if r.Duration <= 0 {
			return errors.New("linear ramp needs Duration > 0")
		}
	case RampStep:
		if r.Rate <= 0 || r.StepBy <= 0 || r.StepEvery <= 0 {
			return errors.New("step ramp needs Rate, StepBy, StepEvery > 0")
		}
	case RampSpike:
		if r.SpikeCount <= 0 || r.SpikeSize <= 0 || r.SpikeIdle <= 0 {
			return errors.New("spike ramp needs SpikeCount, SpikeSize, SpikeIdle > 0")
		}
	default:
		return fmt.Errorf("unknown ramp kind %q", r.Kind)
	}
	return nil
}

func (s *ScalePlan) validate() error {
	// Empty plan is a valid "no scaling" state.
	if s.Namespace == "" && s.Deployment == "" && !s.PreScaleToZero && s.MinReplicas == 0 {
		return nil
	}
	if s.Namespace == "" || s.Deployment == "" {
		return errors.New("namespace and deployment both required when scale plan is set")
	}
	if s.MinReplicas < 0 {
		return errors.New("minReplicas must be >= 0")
	}
	if s.PreScaleTimeout < 0 || s.PostScaleTimeout < 0 {
		return errors.New("scale timeouts must be >= 0")
	}
	return nil
}
