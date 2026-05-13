package envprobe

import (
	"context"
	"io"
	"log/slog"
	"sort"
	"testing"
	"time"
)

// stubProbe is a configurable Probe used to exercise the Runner in
// isolation. It records whether it ran and can simulate hangs and panics.
type stubProbe struct {
	id        string
	result    Result
	delay     time.Duration
	panicWith string
	calls     int
}

func (s *stubProbe) ID() string { return s.id }
func (s *stubProbe) Run(ctx context.Context) Result {
	s.calls++
	if s.panicWith != "" {
		panic(s.panicWith)
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return Result{ID: s.id, Status: Warn, Detail: "ctx done"}
		}
	}
	if s.result.ID == "" {
		s.result.ID = s.id
	}
	return s.result
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRunner_RunAll_FansOutAndCaches(t *testing.T) {
	a := &stubProbe{id: "a", result: Result{Status: OK, Title: "A"}}
	b := &stubProbe{id: "b", result: Result{Status: Warn, Title: "B"}}
	r := NewRunner(discardLogger(), time.Second, a, b)
	results := r.RunAll(context.Background())

	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	// Sorted by ID.
	if results[0].ID != "a" || results[1].ID != "b" {
		t.Errorf("results not sorted by id: %+v", results)
	}
	if got, ok := r.Latest("a"); !ok || got.Status != OK {
		t.Errorf("Latest(a) want OK, got %+v ok=%v", got, ok)
	}
	if r.All()[0].ID != "a" {
		t.Errorf("All() not sorted")
	}
}

func TestRunner_RunAll_PanicIsContained(t *testing.T) {
	a := &stubProbe{id: "a", result: Result{Status: OK}}
	bad := &stubProbe{id: "boom", panicWith: "oops"}
	r := NewRunner(discardLogger(), time.Second, a, bad)
	results := r.RunAll(context.Background())

	if len(results) != 2 {
		t.Fatalf("panic should not have skipped a probe, got %d", len(results))
	}
	for _, res := range results {
		if res.ID == "boom" && res.Status != Error {
			t.Errorf("panic probe should yield Error, got %+v", res)
		}
		if res.ID == "a" && res.Status != OK {
			t.Errorf("sibling probe must complete, got %+v", res)
		}
	}
}

func TestRunner_RunAll_TimeoutYieldsWarn(t *testing.T) {
	slow := &stubProbe{id: "slow", delay: 200 * time.Millisecond}
	r := NewRunner(discardLogger(), 20*time.Millisecond, slow)
	start := time.Now()
	results := r.RunAll(context.Background())
	elapsed := time.Since(start)

	if elapsed > 150*time.Millisecond {
		t.Errorf("RunAll should bail at probe timeout, took %v", elapsed)
	}
	if results[0].Status != Warn {
		t.Errorf("timeout should yield Warn, got %s", results[0].Status)
	}
}

func TestRunner_Register_ReplacesById(t *testing.T) {
	v1 := &stubProbe{id: "x", result: Result{Status: OK}}
	v2 := &stubProbe{id: "x", result: Result{Status: Todo}}
	r := NewRunner(discardLogger(), time.Second, v1)
	r.Register(v2)
	results := r.RunAll(context.Background())
	if len(results) != 1 {
		t.Fatalf("Register should replace by id, got %d entries", len(results))
	}
	if results[0].Status != Todo {
		t.Errorf("want v2 (Todo), got %s", results[0].Status)
	}
}

func TestRunner_NewRunner_DefaultsTimeoutToThreeSeconds(t *testing.T) {
	r := NewRunner(nil, 0)
	if r.probeTimeout != 3*time.Second {
		t.Errorf("default timeout should be 3s, got %v", r.probeTimeout)
	}
}

func TestRunner_RunAll_PreservesResultMetadata(t *testing.T) {
	now := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	p := &stubProbe{id: "meta", result: Result{
		ID: "meta", Status: OK, Title: "Pre-set", Ran: now, Latency: 42 * time.Millisecond,
	}}
	r := NewRunner(discardLogger(), time.Second, p)
	results := r.RunAll(context.Background())
	if !results[0].Ran.Equal(now) {
		t.Errorf("Ran should be preserved when set, got %v", results[0].Ran)
	}
	if results[0].Latency != 42*time.Millisecond {
		t.Errorf("Latency should be preserved when set, got %v", results[0].Latency)
	}
}

func TestRunner_All_Deterministic(t *testing.T) {
	a := &stubProbe{id: "alpha", result: Result{Status: OK}}
	b := &stubProbe{id: "beta", result: Result{Status: OK}}
	c := &stubProbe{id: "gamma", result: Result{Status: OK}}
	r := NewRunner(discardLogger(), time.Second, c, a, b)
	r.RunAll(context.Background())

	ids := make([]string, 0)
	for _, res := range r.All() {
		ids = append(ids, res.ID)
	}
	if !sort.StringsAreSorted(ids) {
		t.Errorf("All() not sorted: %v", ids)
	}
}
