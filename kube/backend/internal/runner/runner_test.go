// Package runner unit tests cover the pure-Go parts of the distributed
// load-test runner — the EventStream pub/sub, the spec-translation
// helpers, and Runner state transitions. The OpenTofu/GKE provisioning
// path lives behind r.tofuApply / r.tofuDestroy and is exercised by a
// separate integration harness that requires the tofu CLI; this file
// only tests what can be tested without a process exec.
package runner

import (
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/argues/argus/internal/saasapi"
	"github.com/argues/argus/pkg/broker"
	"github.com/argues/argus/pkg/loadtest"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ─── EventStream ──────────────────────────────────────────────────────

func TestEventStreamSubscribeAndEmit(t *testing.T) {
	t.Parallel()
	s := NewEventStream("run-1")
	t.Cleanup(s.Close)

	ch := s.Subscribe()
	s.Emit(saasapi.RunnerEvent{RunID: "run-1", Type: "x", Region: "eu"})

	select {
	case evt := <-ch:
		if evt.Region != "eu" || evt.Type != "x" {
			t.Errorf("unexpected event: %+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive the emitted event")
	}
}

func TestEventStreamFanOutToAllSubscribers(t *testing.T) {
	t.Parallel()
	s := NewEventStream("run-2")
	t.Cleanup(s.Close)

	const subscribers = 5
	chs := make([]chan saasapi.RunnerEvent, subscribers)
	for i := range chs {
		chs[i] = s.Subscribe()
	}

	s.Emit(saasapi.RunnerEvent{Type: "fan"})
	for i, ch := range chs {
		select {
		case evt := <-ch:
			if evt.Type != "fan" {
				t.Errorf("subscriber %d got %q, want %q", i, evt.Type, "fan")
			}
		case <-time.After(time.Second):
			t.Errorf("subscriber %d did not receive the event", i)
		}
	}

	if got := s.NumSubscribers(); got != subscribers {
		t.Errorf("NumSubscribers() = %d, want %d", got, subscribers)
	}
}

func TestEventStreamUnsubscribeStopsDelivery(t *testing.T) {
	t.Parallel()
	s := NewEventStream("run-3")
	t.Cleanup(s.Close)

	ch := s.Subscribe()
	s.Unsubscribe(ch)

	// Channel is closed on unsubscribe. Sending further events must not
	// panic and must not deliver to the unsubscribed channel.
	s.Emit(saasapi.RunnerEvent{Type: "after-unsubscribe"})

	// A second Unsubscribe is a no-op.
	s.Unsubscribe(ch)
}

func TestEventStreamUnsubscribeUnknownChannelIsNoop(t *testing.T) {
	t.Parallel()
	s := NewEventStream("run-3b")
	t.Cleanup(s.Close)

	// A channel that was never registered should be silently ignored.
	foreign := make(chan saasapi.RunnerEvent, 1)
	s.Unsubscribe(foreign)
}

func TestEventStreamSlowSubscriberDrops(t *testing.T) {
	t.Parallel()
	s := NewEventStream("run-4")
	t.Cleanup(s.Close)

	ch := s.Subscribe()

	// Fill the per-subscriber buffer (64) plus a bit more. The slow-
	// subscriber path must not block Emit, and the surplus events must
	// be dropped (not panic, not stuck in a queue).
	for i := 0; i < 200; i++ {
		s.Emit(saasapi.RunnerEvent{Type: "drop-me"})
	}

	// We should still be able to drain at least 1 event without timing
	// out — Emit was non-blocking.
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("non-blocking Emit appears to have blocked")
	}
}

func TestEventStreamCloseUnblocksSubscribers(t *testing.T) {
	t.Parallel()
	s := NewEventStream("run-5")
	ch := s.Subscribe()
	s.Close()

	// A closed channel is observable to range receivers and selects.
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("expected closed channel after Close()")
		}
	case <-time.After(time.Second):
		t.Fatal("Close() did not close subscriber channels within 1s")
	}

	// Second Close is a no-op.
	s.Close()

	// Subscribe-after-Close returns an immediately-closed channel.
	after := s.Subscribe()
	select {
	case _, ok := <-after:
		if ok {
			t.Error("Subscribe-after-Close returned an open channel")
		}
	case <-time.After(time.Second):
		t.Fatal("Subscribe-after-Close returned a non-closed channel")
	}
}

func TestEventStreamConcurrentEmitAndSubscribe(t *testing.T) {
	t.Parallel()
	s := NewEventStream("run-6")
	t.Cleanup(s.Close)

	const writers = 4
	const readers = 8
	const events = 500

	var wg sync.WaitGroup
	wg.Add(writers + readers)

	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < events; j++ {
				s.Emit(saasapi.RunnerEvent{Type: "race"})
			}
		}()
	}
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			ch := s.Subscribe()
			defer s.Unsubscribe(ch)
			drain, deadline := 0, time.After(2*time.Second)
			for {
				select {
				case <-ch:
					drain++
					if drain >= 5 {
						return
					}
				case <-deadline:
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestSSEEventFormat(t *testing.T) {
	t.Parallel()
	out, err := SSEEvent(saasapi.RunnerEvent{Type: "progress", RunID: "abc", Region: "eu"})
	if err != nil {
		t.Fatalf("SSEEvent: %v", err)
	}
	if !strings.HasPrefix(out, "event: progress\n") {
		t.Errorf("expected SSE event-type prefix; got %q", out)
	}
	if !strings.Contains(out, `"runId":"abc"`) {
		t.Errorf("expected runId in payload; got %q", out)
	}
	if !strings.HasSuffix(out, "\n\n") {
		t.Errorf("expected SSE double-newline terminator; got %q", out)
	}
}

// ─── Pure helpers ─────────────────────────────────────────────────────

func TestBoolToInt(t *testing.T) {
	t.Parallel()
	if got := boolToInt(true); got != 1 {
		t.Errorf("boolToInt(true) = %d, want 1", got)
	}
	if got := boolToInt(false); got != 0 {
		t.Errorf("boolToInt(false) = %d, want 0", got)
	}
}

func TestToLoadSummaryConvertsLatencies(t *testing.T) {
	t.Parallel()
	got := toLoadSummary(loadtest.Summary{
		Sent:          1000,
		Acked:         900,
		Errors:        100,
		Throughput:    50.5,
		P50AckLatency: 5 * time.Millisecond,
		P95AckLatency: 50 * time.Millisecond,
		P99AckLatency: 250 * time.Millisecond,
		Duration:      30 * time.Second,
	})
	want := &saasapi.LoadSummary{
		TotalSent:    1000,
		TotalAcked:   900,
		TotalErrors:  100,
		Throughput:   50.5,
		P50LatencyMs: 5,
		P95LatencyMs: 50,
		P99LatencyMs: 250,
		DurationSec:  30,
	}
	if *got != *want {
		t.Errorf("toLoadSummary mismatch:\n got: %+v\nwant: %+v", got, want)
	}
}

func TestToRunSpecRequiresLoadSpec(t *testing.T) {
	t.Parallel()
	r := New(saasapi.RunnerSpec{RunID: "no-loadspec"}, "", "", discardLogger())
	_, err := r.toRunSpec("http://endpoint")
	if err == nil {
		t.Fatal("expected error when LoadSpec is nil")
	}
}

func TestToRunSpecCopiesSimpleFields(t *testing.T) {
	t.Parallel()
	spec := saasapi.RunnerSpec{
		RunID: "ok-1",
		Name:  "my-test",
		LoadSpec: &saasapi.RunnerLoadSpec{
			Destination: "topic.x",
			Count:       1000,
			Workers:     16,
		},
	}
	r := New(spec, "", "", discardLogger())
	got, err := r.toRunSpec("http://broker.local")
	if err != nil {
		t.Fatalf("toRunSpec: %v", err)
	}
	if got.Name != "my-test" || got.Destination != "topic.x" || got.Count != 1000 || got.Workers != 16 {
		t.Errorf("RunSpec field copy mismatch: %+v", got)
	}
}

func TestToRunSpecDefaultsWorkersUnderRamp(t *testing.T) {
	t.Parallel()
	spec := saasapi.RunnerSpec{
		LoadSpec: &saasapi.RunnerLoadSpec{
			Destination: "t",
			Count:       1,
			Workers:     0, // no explicit count → expect ramp default 50
			Ramp:        &saasapi.DistLoadRamp{},
		},
	}
	r := New(spec, "", "", discardLogger())
	got, err := r.toRunSpec("")
	if err != nil {
		t.Fatalf("toRunSpec: %v", err)
	}
	if got.Workers != 50 {
		t.Errorf("expected default Workers=50 under Ramp; got %d", got.Workers)
	}
}

func TestToRunSpecCopiesScalePlan(t *testing.T) {
	t.Parallel()
	spec := saasapi.RunnerSpec{
		LoadSpec: &saasapi.RunnerLoadSpec{
			Destination: "t",
			Count:       1,
			Scale: &saasapi.DistLoadScale{
				PreScaleToZero: true,
				MinReplicas:    3,
			},
		},
	}
	r := New(spec, "", "", discardLogger())
	got, err := r.toRunSpec("")
	if err != nil {
		t.Fatalf("toRunSpec: %v", err)
	}
	if !got.Scale.PreScaleToZero || got.Scale.MinReplicas != 3 {
		t.Errorf("Scale plan not copied: %+v", got.Scale)
	}
}

// ─── Broker config endpoint substitution ──────────────────────────────

func TestBuildBrokerConfigSubstitutesEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name      string
		input     string
		endpoint  string
		assertFn  func(t *testing.T, cfg broker.Config)
	}{
		{
			name:     "NATS endpoint substituted",
			input:    `{"kind":"nats","nats":{"servers":"placeholder"}}`,
			endpoint: "nats://my-nats:4222",
			assertFn: func(t *testing.T, cfg broker.Config) {
				if cfg.NATS == nil || cfg.NATS.Servers != "nats://my-nats:4222" {
					t.Errorf("NATS endpoint not substituted: %+v", cfg.NATS)
				}
			},
		},
		{
			name:     "Kafka endpoint substituted",
			input:    `{"kind":"kafka","kafka":{"bootstrapServers":"placeholder","topic":"x"}}`,
			endpoint: "kafka:9092",
			assertFn: func(t *testing.T, cfg broker.Config) {
				if cfg.Kafka == nil || cfg.Kafka.BootstrapServers != "kafka:9092" {
					t.Errorf("Kafka endpoint not substituted: %+v", cfg.Kafka)
				}
			},
		},
		{
			name:     "REST endpoint substituted",
			input:    `{"kind":"rest","rest":{"baseUrl":"placeholder"}}`,
			endpoint: "http://api:8080",
			assertFn: func(t *testing.T, cfg broker.Config) {
				if cfg.REST == nil || cfg.REST.BaseURL != "http://api:8080" {
					t.Errorf("REST endpoint not substituted: %+v", cfg.REST)
				}
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := New(saasapi.RunnerSpec{Broker: json.RawMessage(tc.input)}, "", "", discardLogger())
			cfg, err := r.buildBrokerConfig(tc.endpoint)
			if err != nil {
				t.Fatalf("buildBrokerConfig: %v", err)
			}
			tc.assertFn(t, cfg)
		})
	}
}

func TestBuildBrokerConfigInvalidJSON(t *testing.T) {
	t.Parallel()
	r := New(saasapi.RunnerSpec{Broker: json.RawMessage(`{`)}, "", "", discardLogger())
	_, err := r.buildBrokerConfig("anything")
	if err == nil {
		t.Fatal("expected unmarshal error on invalid JSON")
	}
}

// ─── Runner state ────────────────────────────────────────────────────

func TestRunnerStartsInPendingState(t *testing.T) {
	t.Parallel()
	r := New(saasapi.RunnerSpec{RunID: "s1"}, "", "", discardLogger())
	if got := r.State(); got != "pending" {
		t.Errorf("State() = %q, want %q", got, "pending")
	}
	if r.Result() != nil {
		t.Error("Result() should be nil before Run completes")
	}
}

func TestRunnerCancelBeforeRunIsSafe(t *testing.T) {
	t.Parallel()
	r := New(saasapi.RunnerSpec{RunID: "s2"}, "", "", discardLogger())
	// Cancel before Run() must not panic on a nil cancel func. The
	// implementation moves the state to "canceling" unconditionally but
	// only invokes r.cancel when the runner was previously "running" —
	// that nil-guard is what's actually under test here.
	r.Cancel()
}

func TestRunnerSetRegionStateUpdatesIndex(t *testing.T) {
	t.Parallel()
	spec := saasapi.RunnerSpec{
		RunID: "s3",
		Regions: []saasapi.RegionSpec{
			{Region: "eu"},
			{Region: "us"},
		},
	}
	r := New(spec, "", "", discardLogger())

	r.setRegionState(1, "running", "")
	r.mu.RLock()
	state := r.regions[1].state
	r.mu.RUnlock()
	if state != "running" {
		t.Errorf("region 1 state = %q, want %q", state, "running")
	}
}

func TestRunnerCancelDuringRunFlipsState(t *testing.T) {
	t.Parallel()
	// Drive Cancel against a runner that has already moved into "running"
	// to confirm the running→canceling transition + Stream notification.
	r := New(saasapi.RunnerSpec{RunID: "s4"}, "", "", discardLogger())
	r.mu.Lock()
	r.state = "running"
	cancelCalled := false
	r.cancel = func() { cancelCalled = true }
	r.mu.Unlock()

	sub := r.Stream.Subscribe()
	r.Cancel()

	if got := r.State(); got != "canceling" {
		t.Errorf("State() after Cancel = %q, want %q", got, "canceling")
	}
	if !cancelCalled {
		t.Error("Cancel() did not call the stored cancel func")
	}
	select {
	case evt := <-sub:
		if evt.Type != "canceling" {
			t.Errorf("Stream event type = %q, want %q", evt.Type, "canceling")
		}
	case <-time.After(time.Second):
		t.Error("Cancel() did not emit a canceling event")
	}

	// Cleanup — drain stream.
	r.Stream.Unsubscribe(sub)
}

// ─── Smoke test for the public constructor surface ────────────────────

func TestNewRunnerEmptySpec(t *testing.T) {
	t.Parallel()
	r := New(saasapi.RunnerSpec{}, "", "", nil)
	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.Stream == nil {
		t.Fatal("New() did not create an EventStream")
	}
	if r.State() != "pending" {
		t.Errorf("default state = %q, want pending", r.State())
	}
}

