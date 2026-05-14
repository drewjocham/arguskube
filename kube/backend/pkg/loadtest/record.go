package loadtest

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/argues/argus/pkg/broker"
)

// Sample is one publish attempt's outcome. The engine accumulates a
// stream of these; the aggregator turns them into Summary on demand
// (no premature aggregation — agents in PR-E might want to look at the
// raw stream).
type Sample struct {
	At         time.Time     `json:"at"`
	AckLatency time.Duration `json:"ackLatencyNs"`
	OK         bool          `json:"ok"`
	Err        string        `json:"err,omitempty"`
}

// ScaleEvent is one observation of the consumer Deployment's replica
// state during the run. Captured at the boundaries (pre-scale start,
// pre-scale reached zero, scale-up start, first ready replica, all
// ready) and at a coarse cadence during the drain.
type ScaleEvent struct {
	At       time.Time `json:"at"`
	Phase    string    `json:"phase"` // pre-scale | publishing | scaling-up | draining | done
	Replicas int32     `json:"replicas"`
	Ready    int32     `json:"ready"`
}

// Summary is the aggregate. Computed by Aggregate() at the end of the
// run (or live on demand for the frontend's progress chart).
type Summary struct {
	Sent          int           `json:"sent"`
	Acked         int           `json:"acked"`
	Errors        int           `json:"errors"`
	Duration      time.Duration `json:"durationNs"`
	Throughput    float64       `json:"throughputPerSec"`
	P50AckLatency time.Duration `json:"p50AckLatencyNs"`
	P95AckLatency time.Duration `json:"p95AckLatencyNs"`
	P99AckLatency time.Duration `json:"p99AckLatencyNs"`
	MaxAckLatency time.Duration `json:"maxAckLatencyNs"`
	// ErrorBreakdown counts errors by their text. Cap at top-10
	// strings — beyond that we bucket into "other". The agent in
	// PR-E uses this directly in its narrative.
	ErrorBreakdown map[string]int `json:"errorBreakdown,omitempty"`
}

// RunRecord is what the engine returns at the end. PR-C marshals this
// to JSON for the Wails event channel and PR-E feeds it to the agent.
type RunRecord struct {
	Spec       RunSpec      `json:"spec"`
	BrokerKind broker.Kind  `json:"brokerKind"`
	Started    time.Time    `json:"started"`
	Finished   time.Time    `json:"finished"`
	Samples    []Sample     `json:"samples,omitempty"`
	ScaleLog   []ScaleEvent `json:"scaleLog,omitempty"`
	Summary    Summary      `json:"summary"`
	// FinalError is set when the run aborted before completion —
	// pre-scale timeout, broker connect failure, ctx canceled. A
	// nil here means the publish phase ran to its planned end (even
	// if individual messages errored — those are in Summary.Errors).
	FinalError string `json:"finalError,omitempty"`
}

// recorder is the engine's internal append-only buffer. Concurrent
// safe; the engine's workers all hand it Sample values.
type recorder struct {
	mu       sync.Mutex
	samples  []Sample
	scaleLog []ScaleEvent
}

func newRecorder() *recorder {
	return &recorder{
		samples:  make([]Sample, 0, 1024),
		scaleLog: make([]ScaleEvent, 0, 32),
	}
}

func (r *recorder) addSample(s Sample) {
	r.mu.Lock()
	r.samples = append(r.samples, s)
	r.mu.Unlock()
}

func (r *recorder) addScale(e ScaleEvent) {
	r.mu.Lock()
	r.scaleLog = append(r.scaleLog, e)
	r.mu.Unlock()
}

func (r *recorder) snapshot() ([]Sample, []ScaleEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := make([]Sample, len(r.samples))
	copy(s, r.samples)
	e := make([]ScaleEvent, len(r.scaleLog))
	copy(e, r.scaleLog)
	return s, e
}

// Aggregate turns a Sample stream into a Summary. Exposed at package
// level so PR-C's live-progress code can call it every few seconds
// without going through Engine.
//
// The percentile method is "nearest rank" — simple, deterministic,
// and matches what gnuplot and most observability tooling do. We
// don't interpolate because at the load tester's scales (tens to
// hundreds of thousands of samples) the index gap between adjacent
// ranks is so small that linear interpolation would shave less than
// one wall-clock microsecond.
func Aggregate(samples []Sample) Summary {
	s := Summary{
		Sent:           len(samples),
		ErrorBreakdown: map[string]int{},
	}
	if len(samples) == 0 {
		return s
	}
	latencies := make([]time.Duration, 0, len(samples))
	var earliest, latest time.Time
	for i, x := range samples {
		if x.OK {
			s.Acked++
			latencies = append(latencies, x.AckLatency)
		} else {
			s.Errors++
			key := x.Err
			if key == "" {
				key = "unknown"
			}
			s.ErrorBreakdown[key]++
		}
		if i == 0 || x.At.Before(earliest) {
			earliest = x.At
		}
		if x.At.After(latest) {
			latest = x.At
		}
	}
	s.Duration = latest.Sub(earliest)
	if s.Duration > 0 {
		s.Throughput = float64(s.Sent) / s.Duration.Seconds()
	}
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		s.P50AckLatency = nearestRank(latencies, 0.50)
		s.P95AckLatency = nearestRank(latencies, 0.95)
		s.P99AckLatency = nearestRank(latencies, 0.99)
		s.MaxAckLatency = latencies[len(latencies)-1]
	}
	// Cap the breakdown at 10 keys, bucketing the rest into "other".
	if len(s.ErrorBreakdown) > 10 {
		type kv struct {
			k string
			v int
		}
		all := make([]kv, 0, len(s.ErrorBreakdown))
		for k, v := range s.ErrorBreakdown {
			all = append(all, kv{k, v})
		}
		sort.Slice(all, func(i, j int) bool { return all[i].v > all[j].v })
		capped := map[string]int{}
		for _, x := range all[:10] {
			capped[x.k] = x.v
		}
		other := 0
		for _, x := range all[10:] {
			other += x.v
		}
		capped["other"] = other
		s.ErrorBreakdown = capped
	}
	return s
}

// nearestRank returns the value at the (ceil(p*n))-th rank in a
// pre-sorted slice. For p=0.95 on n=1000 that's index 949 (0-based).
func nearestRank(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}
