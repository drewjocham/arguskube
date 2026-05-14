package loadtest

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argues/argus/pkg/broker"
)

// fakePublisher: a minimal broker.Publisher for testing the engine in
// isolation from real adapters. Records every Publish and returns a
// configurable AckLatency / err. Concurrent-safe.
type fakePublisher struct {
	mu          sync.Mutex
	connectErr  error
	publishErr  error
	ackLatency  time.Duration
	received    int32
	connectHits int32
	closed      bool
}

func (f *fakePublisher) Connect(_ context.Context) error {
	atomic.AddInt32(&f.connectHits, 1)
	return f.connectErr
}
func (f *fakePublisher) Publish(_ context.Context, _ broker.Message) (broker.Receipt, error) {
	atomic.AddInt32(&f.received, 1)
	if f.publishErr != nil {
		return broker.Receipt{}, f.publishErr
	}
	return broker.Receipt{
		PublishedAt: time.Now(),
		AckLatency:  f.ackLatency,
	}, nil
}
func (f *fakePublisher) Close() error      { f.mu.Lock(); f.closed = true; f.mu.Unlock(); return nil }
func (f *fakePublisher) Kind() broker.Kind { return broker.KindNATS }

// fakeScaler tracks Scale calls + delivers a controlled view of
// "ready replicas" so engine tests can verify WaitForReplicas logic
// without sleeping.
type fakeScaler struct {
	mu           sync.Mutex
	scaledTo     []int32
	ready        int32 // what Observe reports
	specReplicas int32
	observeHits  int32
	scaleErr     error
	waitErr      error
}

func (s *fakeScaler) Scale(_ context.Context, _, _ string, r int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scaledTo = append(s.scaledTo, r)
	s.specReplicas = r
	if s.scaleErr != nil {
		return s.scaleErr
	}
	return nil
}
func (s *fakeScaler) WaitForReplicas(_ context.Context, _, _ string, target int32) error {
	if s.waitErr != nil {
		return s.waitErr
	}
	s.mu.Lock()
	s.ready = target
	s.mu.Unlock()
	return nil
}
func (s *fakeScaler) Observe(_ context.Context, _, _ string) (int32, int32, error) {
	atomic.AddInt32(&s.observeHits, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.specReplicas, s.ready, nil
}

func minimalSpec() RunSpec {
	return RunSpec{
		Name:        "test",
		Broker:      broker.Config{Kind: broker.KindNATS, NATS: &broker.NATSConfig{Servers: "nats://x"}},
		Destination: "test.subject",
		Payload:     Payload{Kind: PayloadKindTyped, Bytes: []byte(`{"k":"v"}`), Size: 9},
		Count:       100,
		Ramp:        Ramp{Kind: RampConstant, Rate: 1000},
		Workers:     5,
	}
}

// --- Validate ---------------------------------------------------------------

func TestRunSpec_Validate(t *testing.T) {
	good := minimalSpec()
	if err := good.Validate(); err != nil {
		t.Fatalf("baseline spec should validate, got: %v", err)
	}

	cases := []struct {
		name    string
		mutate  func(*RunSpec)
		errSub  string
	}{
		{"empty destination", func(s *RunSpec) { s.Destination = "" }, "destination"},
		{"empty payload", func(s *RunSpec) { s.Payload.Bytes = nil }, "payload"},
		{"zero count for non-spike", func(s *RunSpec) { s.Count = 0 }, "count"},
		{"bad ramp", func(s *RunSpec) { s.Ramp = Ramp{Kind: "weird"} }, "ramp"},
		{"bad broker kind", func(s *RunSpec) { s.Broker.Kind = "unknown" }, "broker"},
		{"scale plan missing namespace", func(s *RunSpec) {
			s.Scale = ScalePlan{Deployment: "x", PreScaleToZero: true}
		}, "namespace and deployment"},
		{"negative minReplicas", func(s *RunSpec) {
			s.Scale = ScalePlan{Namespace: "n", Deployment: "d", MinReplicas: -1}
		}, "minReplicas"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := minimalSpec()
			c.mutate(&s)
			err := s.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.errSub)
			}
		})
	}
}

func TestRunSpec_Validate_SpikeAllowsZeroCount(t *testing.T) {
	s := minimalSpec()
	s.Count = 0
	s.Ramp = Ramp{Kind: RampSpike, SpikeCount: 2, SpikeSize: 5, SpikeIdle: time.Second}
	if err := s.Validate(); err != nil {
		t.Fatalf("spike spec should validate with Count=0: %v", err)
	}
}

// --- Ramp planner -----------------------------------------------------------

func TestRamp_Constant(t *testing.T) {
	r := Ramp{Kind: RampConstant, Rate: 100}
	sch := newRampPlanner(r, 10).schedule()
	if len(sch) != 10 {
		t.Fatalf("len=%d, want 10", len(sch))
	}
	// 100 msgs/sec → 10ms gap.
	wantGap := 10 * time.Millisecond
	for i := 1; i < len(sch); i++ {
		gap := sch[i] - sch[i-1]
		if absDur(gap-wantGap) > time.Microsecond {
			t.Errorf("gap[%d]=%v, want %v", i, gap, wantGap)
		}
	}
}

func TestRamp_Constant_DurationCap(t *testing.T) {
	// 100 msgs/sec, Duration=50ms means at most 5 messages.
	r := Ramp{Kind: RampConstant, Rate: 100, Duration: 50 * time.Millisecond}
	sch := newRampPlanner(r, 1000).schedule()
	if len(sch) != 5 {
		t.Fatalf("len=%d, want 5 (Duration cap)", len(sch))
	}
}

func TestRamp_Linear_RateClimbs(t *testing.T) {
	r := Ramp{Kind: RampLinear, Rate: 1, RampTo: 100, Duration: 10 * time.Second}
	sch := newRampPlanner(r, 500).schedule()
	if len(sch) < 100 {
		t.Fatalf("schedule too short: %d", len(sch))
	}
	// First gap should be much larger than the last gap because
	// the rate climbed from 1/s to 100/s.
	firstGap := sch[1] - sch[0]
	lastGap := sch[len(sch)-1] - sch[len(sch)-2]
	if firstGap <= lastGap*5 {
		t.Errorf("expected first gap (%v) to be >> last gap (%v)", firstGap, lastGap)
	}
}

func TestRamp_Step(t *testing.T) {
	// 10/s for the first 100ms, then 20/s, then 30/s. Over 300ms ~6 messages.
	r := Ramp{Kind: RampStep, Rate: 10, StepBy: 10, StepEvery: 100 * time.Millisecond, Duration: 300 * time.Millisecond}
	sch := newRampPlanner(r, 1000).schedule()
	if len(sch) < 5 || len(sch) > 8 {
		t.Errorf("len=%d, want ~6 (step ramp 10→20→30/s over 300ms)", len(sch))
	}
}

func TestRamp_Spike(t *testing.T) {
	r := Ramp{Kind: RampSpike, SpikeCount: 3, SpikeSize: 100, SpikeIdle: time.Second}
	sch := newRampPlanner(r, 0).schedule()
	if len(sch) != 300 {
		t.Fatalf("len=%d, want 300", len(sch))
	}
	// First 100 should all share offset=0, next 100 share offset=1s, etc.
	if sch[0] != 0 || sch[99] != 0 {
		t.Error("first burst should all be at t=0")
	}
	if sch[100] != time.Second {
		t.Errorf("second burst starts at %v, want 1s", sch[100])
	}
}

// --- Aggregator -------------------------------------------------------------

func TestAggregate_Empty(t *testing.T) {
	s := Aggregate(nil)
	if s.Sent != 0 || s.Acked != 0 || s.Errors != 0 {
		t.Errorf("empty aggregate non-zero: %+v", s)
	}
}

func TestAggregate_AllOK(t *testing.T) {
	now := time.Now()
	samples := []Sample{
		{At: now, AckLatency: 1 * time.Millisecond, OK: true},
		{At: now, AckLatency: 2 * time.Millisecond, OK: true},
		{At: now, AckLatency: 3 * time.Millisecond, OK: true},
		{At: now, AckLatency: 4 * time.Millisecond, OK: true},
		{At: now, AckLatency: 100 * time.Millisecond, OK: true},
	}
	s := Aggregate(samples)
	if s.Sent != 5 || s.Acked != 5 || s.Errors != 0 {
		t.Fatalf("counts wrong: %+v", s)
	}
	// P50 of [1,2,3,4,100]ms at rank ceil(0.5*5)=3 → 3ms.
	if s.P50AckLatency != 3*time.Millisecond {
		t.Errorf("P50 = %v, want 3ms", s.P50AckLatency)
	}
	// P95 of 5 samples at rank ceil(0.95*5)=5 → 100ms.
	if s.P95AckLatency != 100*time.Millisecond {
		t.Errorf("P95 = %v, want 100ms", s.P95AckLatency)
	}
	if s.MaxAckLatency != 100*time.Millisecond {
		t.Errorf("Max = %v, want 100ms", s.MaxAckLatency)
	}
}

func TestAggregate_MixedErrors(t *testing.T) {
	now := time.Now()
	samples := []Sample{
		{At: now, AckLatency: time.Millisecond, OK: true},
		{At: now, OK: false, Err: "timeout"},
		{At: now, OK: false, Err: "timeout"},
		{At: now, OK: false, Err: "auth"},
	}
	s := Aggregate(samples)
	if s.Acked != 1 || s.Errors != 3 {
		t.Fatalf("counts wrong: %+v", s)
	}
	if s.ErrorBreakdown["timeout"] != 2 || s.ErrorBreakdown["auth"] != 1 {
		t.Errorf("breakdown wrong: %v", s.ErrorBreakdown)
	}
}

// --- Engine end-to-end ------------------------------------------------------

func TestEngine_HappyPath_NoScale(t *testing.T) {
	spec := minimalSpec()
	spec.Count = 50
	spec.Ramp = Ramp{Kind: RampConstant, Rate: 5000} // 5k/s → 50 msgs in 10ms

	fp := &fakePublisher{ackLatency: 200 * time.Microsecond}
	e := New(spec, slog.Default())
	e.NewPublisher = func(_ context.Context, _ broker.Config, _ *slog.Logger) (broker.Publisher, error) {
		return fp, nil
	}

	rec, err := e.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rec.FinalError != "" {
		t.Errorf("FinalError = %q", rec.FinalError)
	}
	if rec.Summary.Sent != 50 || rec.Summary.Acked != 50 {
		t.Errorf("counts: %+v", rec.Summary)
	}
	if got := atomic.LoadInt32(&fp.received); got != 50 {
		t.Errorf("publisher received %d, want 50", got)
	}
	if !fp.closed {
		t.Error("publisher should be Close()d")
	}
}

func TestEngine_BrokerConnectFailure_RecordsError(t *testing.T) {
	spec := minimalSpec()
	e := New(spec, slog.Default())
	e.NewPublisher = func(_ context.Context, _ broker.Config, _ *slog.Logger) (broker.Publisher, error) {
		return &fakePublisher{connectErr: errors.New("auth denied")}, nil
	}
	rec, err := e.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned Go error (expected nil — failures via FinalError): %v", err)
	}
	if rec.FinalError == "" {
		t.Error("expected FinalError to be set")
	}
}

func TestEngine_PublishErrorsCountAsErrors(t *testing.T) {
	spec := minimalSpec()
	spec.Count = 10
	spec.Ramp = Ramp{Kind: RampConstant, Rate: 1000}
	e := New(spec, slog.Default())
	e.NewPublisher = func(_ context.Context, _ broker.Config, _ *slog.Logger) (broker.Publisher, error) {
		return &fakePublisher{publishErr: errors.New("dest not writable")}, nil
	}
	rec, err := e.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rec.Summary.Errors != 10 {
		t.Errorf("errors = %d, want 10", rec.Summary.Errors)
	}
}

func TestEngine_PreScale_RunsScaler(t *testing.T) {
	spec := minimalSpec()
	spec.Count = 5
	spec.Ramp = Ramp{Kind: RampConstant, Rate: 1000}
	spec.Scale = ScalePlan{
		Namespace: "ns", Deployment: "dep",
		PreScaleToZero: true, MinReplicas: 2,
		PreScaleTimeout: time.Second, PostScaleTimeout: time.Second,
	}
	fp := &fakePublisher{}
	fs := &fakeScaler{}
	e := New(spec, slog.Default())
	e.NewPublisher = func(_ context.Context, _ broker.Config, _ *slog.Logger) (broker.Publisher, error) {
		return fp, nil
	}
	e.Scaler = fs

	// Drain observation phase loops on 5s ticks for up to a minute;
	// use a short ctx so we don't actually wait that long.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	rec, _ := e.Run(ctx)
	if rec == nil {
		t.Fatal("nil record")
	}
	// Scaler should have been called with 0 then with 2.
	if len(fs.scaledTo) < 2 {
		t.Fatalf("scaledTo = %v, want >=2 entries", fs.scaledTo)
	}
	if fs.scaledTo[0] != 0 {
		t.Errorf("first scale target = %d, want 0", fs.scaledTo[0])
	}
	if fs.scaledTo[1] != 2 {
		t.Errorf("second scale target = %d, want 2", fs.scaledTo[1])
	}
}

func TestEngine_PreScaleFailure_AbortsBeforePublish(t *testing.T) {
	spec := minimalSpec()
	spec.Scale = ScalePlan{
		Namespace: "ns", Deployment: "dep",
		PreScaleToZero: true, PreScaleTimeout: 50 * time.Millisecond,
	}
	fp := &fakePublisher{}
	fs := &fakeScaler{scaleErr: errors.New("RBAC denied")}
	e := New(spec, slog.Default())
	e.NewPublisher = func(_ context.Context, _ broker.Config, _ *slog.Logger) (broker.Publisher, error) {
		return fp, nil
	}
	e.Scaler = fs

	rec, _ := e.Run(context.Background())
	if rec.FinalError == "" {
		t.Error("expected FinalError after pre-scale failure")
	}
	if atomic.LoadInt32(&fp.received) != 0 {
		t.Error("publisher should not have been called after pre-scale failure")
	}
}

func TestEngine_ContextCancel_ReturnsRecord(t *testing.T) {
	spec := minimalSpec()
	spec.Count = 10_000
	spec.Ramp = Ramp{Kind: RampConstant, Rate: 100} // would take 100s
	fp := &fakePublisher{}
	e := New(spec, slog.Default())
	e.NewPublisher = func(_ context.Context, _ broker.Config, _ *slog.Logger) (broker.Publisher, error) {
		return fp, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	rec, _ := e.Run(ctx)
	if rec == nil {
		t.Fatal("nil record on cancel")
	}
	if rec.FinalError == "" {
		t.Error("expected FinalError after cancel")
	}
}

// --- SamplePool round-robin -------------------------------------------------

// recordingPublisher captures the bytes of every Publish so the test
// can assert round-robin distribution across SamplePool entries.
type recordingPublisher struct {
	mu       sync.Mutex
	received [][]byte
}

func (p *recordingPublisher) Connect(_ context.Context) error { return nil }
func (p *recordingPublisher) Publish(_ context.Context, m broker.Message) (broker.Receipt, error) {
	p.mu.Lock()
	cpy := make([]byte, len(m.Payload))
	copy(cpy, m.Payload)
	p.received = append(p.received, cpy)
	p.mu.Unlock()
	return broker.Receipt{PublishedAt: time.Now()}, nil
}
func (p *recordingPublisher) Close() error      { return nil }
func (p *recordingPublisher) Kind() broker.Kind { return broker.KindNATS }

func TestEngine_SamplePool_RoundRobin(t *testing.T) {
	pool := [][]byte{[]byte(`{"k":0}`), []byte(`{"k":1}`), []byte(`{"k":2}`)}
	spec := minimalSpec()
	spec.Count = 9
	// Single worker keeps the round-robin order deterministic so the
	// test can assert exactly which payload landed where.
	spec.Workers = 1
	spec.Ramp = Ramp{Kind: RampConstant, Rate: 5000}
	spec.Payload = Payload{
		Kind:       PayloadKindFile,
		SamplePool: pool,
		Size:       len(pool[0]),
	}

	rp := &recordingPublisher{}
	e := New(spec, slog.Default())
	e.NewPublisher = func(_ context.Context, _ broker.Config, _ *slog.Logger) (broker.Publisher, error) {
		return rp, nil
	}
	rec, err := e.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rec.FinalError != "" {
		t.Fatalf("FinalError = %q", rec.FinalError)
	}

	rp.mu.Lock()
	defer rp.mu.Unlock()
	if len(rp.received) != 9 {
		t.Fatalf("received %d, want 9", len(rp.received))
	}
	counts := map[string]int{}
	for _, b := range rp.received {
		counts[string(b)]++
	}
	for i, want := range pool {
		if counts[string(want)] != 3 {
			t.Errorf("pool[%d] sent %d times, want 3 (counts=%v)", i, counts[string(want)], counts)
		}
	}
}

// --- Presets ----------------------------------------------------------------

func TestPresetList_FiveLocked(t *testing.T) {
	ps := PresetList()
	if len(ps) != 5 {
		t.Fatalf("got %d presets, want 5", len(ps))
	}
	wantIDs := []string{"smoke", "cold-start", "soak", "linear-ramp", "spike"}
	for i, want := range wantIDs {
		if ps[i].ID != want {
			t.Errorf("preset %d id = %q, want %q", i, ps[i].ID, want)
		}
	}
}

func TestPresetList_AllValidShapes(t *testing.T) {
	ps := PresetList()
	for _, p := range ps {
		// Presets are starting points — they don't have a Broker
		// or Destination filled in (the user picks those). Patch
		// the missing bits and assert Validate passes.
		s := p.Spec
		s.Broker = broker.Config{Kind: broker.KindNATS, NATS: &broker.NATSConfig{Servers: "nats://x"}}
		s.Destination = "test"
		s.Payload = Payload{Kind: PayloadKindTyped, Bytes: []byte("x"), Size: 1}
		// Presets with PreScaleToZero or MinReplicas need a target
		// the user picks at runtime — fill those in too so the
		// Validate is exercising preset shape end to end.
		if s.Scale.PreScaleToZero || s.Scale.MinReplicas > 0 {
			s.Scale.Namespace = "ns"
			s.Scale.Deployment = "dep"
		}
		if err := s.Validate(); err != nil {
			t.Errorf("preset %q does not validate after patching: %v", p.ID, err)
		}
	}
}

// --- Helpers ----------------------------------------------------------------

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
