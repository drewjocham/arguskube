package pkg

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/argues/argus/pkg/broker"
	"github.com/argues/argus/pkg/loadtest"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

type fakeLoadtestPublisher struct {
	ackLatency time.Duration
}

func (f *fakeLoadtestPublisher) Connect(_ context.Context) error { return nil }
func (f *fakeLoadtestPublisher) Publish(_ context.Context, _ broker.Message) (broker.Receipt, error) {
	return broker.Receipt{
		PublishedAt: time.Now(),
		AckLatency:  f.ackLatency,
	}, nil
}
func (f *fakeLoadtestPublisher) Close() error      { return nil }
func (f *fakeLoadtestPublisher) Kind() broker.Kind { return broker.KindNATS }

// quietApp is the minimal App that the load-test bindings need to
// function. ctx is left nil so safeEmit short-circuits — the Wails
// runtime rejects non-lifecycle contexts with a noisy stderr log
// otherwise. The engine itself uses appCtx() (which falls back to
// context.Background) so per-message ctx still works.
func quietApp(t *testing.T) *App {
	t.Helper()
	return &App{
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})),
		// ctx intentionally nil
	}
}

func goodSpec() loadtest.RunSpec {
	return loadtest.RunSpec{
		Name:        "unit-test",
		Broker:      broker.Config{Kind: broker.KindNATS, NATS: &broker.NATSConfig{Servers: "nats://x"}},
		Destination: "test.subject",
		Payload:     loadtest.Payload{Kind: loadtest.PayloadKindTyped, Bytes: []byte(`{"k":"v"}`), Size: 9},
		Count:       20,
		Ramp:        loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 1000},
		Workers:     4,
	}
}

// startTestLoadTest wires a fake publisher into the App's loadtest
// factory hook, then calls StartLoadTest. The hook is set BEFORE the
// engine goroutine spawns, so the engine never dials a real broker.
// t.Cleanup unregisters the factory so parallel tests don't leak.
func startTestLoadTest(t *testing.T, a *App, spec loadtest.RunSpec, pub broker.Publisher) string {
	t.Helper()
	a.setLoadtestPublisherFactory(func(_ context.Context, _ broker.Config, _ *slog.Logger) (broker.Publisher, error) {
		return pub, nil
	})
	t.Cleanup(func() { a.setLoadtestPublisherFactory(nil) })

	id, err := a.StartLoadTest(spec)
	if err != nil {
		t.Fatalf("StartLoadTest: %v", err)
	}
	return id
}

// awaitState polls the run's state up to dur, returns the last state.
func awaitState(t *testing.T, a *App, runID string, want string, dur time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(dur)
	for time.Now().Before(deadline) {
		st, err := a.GetLoadTestStatus(runID)
		if err != nil {
			t.Fatalf("GetLoadTestStatus: %v", err)
		}
		if st.State == want {
			return st.State
		}
		time.Sleep(10 * time.Millisecond)
	}
	st, _ := a.GetLoadTestStatus(runID)
	return st.State
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

func TestListLoadTestPresets(t *testing.T) {
	a := quietApp(t)
	ps := a.ListLoadTestPresets()
	if len(ps) != 5 {
		t.Fatalf("got %d presets, want 5", len(ps))
	}
	wantIDs := []string{"smoke", "cold-start", "soak", "linear-ramp", "spike"}
	for i, want := range wantIDs {
		if ps[i].ID != want {
			t.Errorf("preset[%d].ID = %q, want %q", i, ps[i].ID, want)
		}
	}
}

func TestListBrokerKinds(t *testing.T) {
	a := quietApp(t)
	k := a.ListBrokerKinds()
	if len(k) != 5 {
		t.Errorf("got %d kinds, want 5", len(k))
	}
}

func TestStartLoadTest_RejectsInvalidSpec(t *testing.T) {
	a := quietApp(t)
	bad := goodSpec()
	bad.Destination = "" // makes Validate fail
	_, err := a.StartLoadTest(bad)
	if err == nil {
		t.Fatal("expected error for invalid spec")
	}
}

func TestStartLoadTest_RejectsConcurrent(t *testing.T) {
	a := quietApp(t)
	spec := goodSpec()
	spec.Count = 1000
	// 100/s + 1000 msgs = 10 seconds of work — plenty of time for
	// the second StartLoadTest call to land while the first is
	// still running.
	spec.Ramp = loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 100}
	firstID := startTestLoadTest(t, a, spec, &fakeLoadtestPublisher{ackLatency: time.Millisecond})
	// Wait until the first run has transitioned past "pending"
	// into "running" — otherwise activeState() might still report
	// the initial state when we check.
	awaitState(t, a, firstID, "running", time.Second)

	// Second StartLoadTest should be rejected while the first is
	// still running.
	_, err := a.StartLoadTest(spec)
	if err == nil {
		t.Fatal("expected second StartLoadTest to fail (concurrent)")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("error = %q, want it to mention 'already running'", err.Error())
	}
	// Clean up so other tests can run.
	_ = a.CancelLoadTest(firstID)
}

func TestStartLoadTest_HappyPath_StateTransitions(t *testing.T) {
	a := quietApp(t)
	spec := goodSpec()
	spec.Count = 5
	spec.Ramp = loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 5000} // burst

	id := startTestLoadTest(t, a, spec, &fakeLoadtestPublisher{ackLatency: 100 * time.Microsecond})

	state := awaitState(t, a, id, "done", 2*time.Second)
	if state != "done" {
		t.Fatalf("final state = %q, want %q", state, "done")
	}

	st, err := a.GetLoadTestStatus(id)
	if err != nil {
		t.Fatalf("GetLoadTestStatus: %v", err)
	}
	if st.Summary.Sent != 5 {
		t.Errorf("Summary.Sent = %d, want 5", st.Summary.Sent)
	}
	if st.FinalError != "" {
		t.Errorf("FinalError = %q (should be empty on happy path)", st.FinalError)
	}
}

func TestCancelLoadTest(t *testing.T) {
	a := quietApp(t)
	spec := goodSpec()
	spec.Count = 100_000
	spec.Ramp = loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 100} // long run
	id := startTestLoadTest(t, a, spec, &fakeLoadtestPublisher{ackLatency: time.Millisecond})

	// Give the engine a moment to enter the publish phase.
	time.Sleep(50 * time.Millisecond)
	if err := a.CancelLoadTest(id); err != nil {
		t.Fatalf("CancelLoadTest: %v", err)
	}
	state := awaitState(t, a, id, "canceled", 2*time.Second)
	if state != "canceled" {
		t.Errorf("post-cancel state = %q, want canceled", state)
	}
}

func TestCancelLoadTest_UnknownRunID(t *testing.T) {
	a := quietApp(t)
	if err := a.CancelLoadTest("not-a-real-id"); err == nil {
		t.Error("expected error for unknown runID")
	}
}

func TestGetLoadTestRecord_NotFinished(t *testing.T) {
	a := quietApp(t)
	spec := goodSpec()
	spec.Count = 1000
	spec.Ramp = loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 10} // takes a while
	id := startTestLoadTest(t, a, spec, &fakeLoadtestPublisher{ackLatency: time.Millisecond})

	_, err := a.GetLoadTestRecord(id)
	if err == nil {
		t.Error("expected error for in-flight run")
	}
	_ = a.CancelLoadTest(id)
}

func TestSanitizeNotebookName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "run"},
		{"hello world", "hello-world"},
		{"foo/bar", "foobar"},
		{"foo bar.json", "foo-bar-json"},
		{"!!!", "run"},
		{strings.Repeat("a", 100), strings.Repeat("a", 60)},
	}
	for _, c := range cases {
		if got := sanitizeNotebookName(c.in); got != c.want {
			t.Errorf("sanitizeNotebookName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRenderLoadTestMarkdown_FrontmatterAndSections(t *testing.T) {
	rec := &loadtest.RunRecord{
		Spec: loadtest.RunSpec{
			Name:        "test-run",
			Destination: "test.subject",
			Payload:     loadtest.Payload{Kind: loadtest.PayloadKindTyped, Size: 9},
			Ramp:        loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 50},
		},
		BrokerKind: broker.KindNATS,
		Started:    time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC),
		Finished:   time.Date(2026, 5, 14, 0, 0, 30, 0, time.UTC),
		Summary: loadtest.Summary{
			Sent:          100,
			Acked:         98,
			Errors:        2,
			Throughput:    50.0,
			P50AckLatency: time.Millisecond,
			P95AckLatency: 5 * time.Millisecond,
			P99AckLatency: 10 * time.Millisecond,
			MaxAckLatency: 12 * time.Millisecond,
			ErrorBreakdown: map[string]int{
				"timeout": 2,
			},
		},
		ScaleLog: []loadtest.ScaleEvent{
			{At: time.Date(2026, 5, 14, 0, 0, 1, 0, time.UTC), Phase: "publishing", Replicas: 0, Ready: 0},
		},
	}
	md := renderLoadTestMarkdown(rec, "")

	// Sanity checks. We don't pin the entire output — it's a long
	// formatted string — but assert the critical fields the agent
	// in PR-E and the frontend viewer rely on.
	mustContain(t, md, "type: loadtest-report")
	mustContain(t, md, "broker: nats")
	mustContain(t, md, "# Load test report")
	mustContain(t, md, "**Name:** test-run")
	mustContain(t, md, "- Sent: **100**")
	mustContain(t, md, "- Acked: **98**")
	mustContain(t, md, "- Throughput: **50.0 msg/s**")
	mustContain(t, md, "## Scale timeline")
	mustContain(t, md, "## Errors by kind")
	mustContain(t, md, "`timeout`: 2")
	mustContain(t, md, "## Raw record (JSON)")
}

func TestRenderLoadTestMarkdown_WithFinalError(t *testing.T) {
	rec := &loadtest.RunRecord{
		Spec:       goodSpec(),
		BrokerKind: broker.KindNATS,
		Started:    time.Now(),
		Finished:   time.Now(),
		Summary:    loadtest.Summary{Sent: 0},
		FinalError: "broker connect: dial tcp: timeout",
	}
	md := renderLoadTestMarkdown(rec, "")
	mustContain(t, md, "## Final error")
	mustContain(t, md, "broker connect: dial tcp: timeout")
}

func mustContain(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("output missing expected substring %q", sub)
	}
}

// PR-E: Narrative section appears between the report header and the
// Summary section when a non-empty narrative is supplied. When empty,
// the section is omitted entirely (no header, no extra blank line)
// so a no-AI install gets a clean report.
func TestRenderLoadTestMarkdown_WithNarrative(t *testing.T) {
	rec := &loadtest.RunRecord{
		Spec: loadtest.RunSpec{
			Destination: "x",
			Payload:     loadtest.Payload{Kind: loadtest.PayloadKindTyped, Size: 1},
			Ramp:        loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 1},
		},
		BrokerKind: broker.KindNATS,
		Started:    time.Now(),
		Finished:   time.Now(),
		Summary:    loadtest.Summary{Sent: 10},
	}
	narrative := "Backlog drained in 38 seconds. P99 was within budget."
	md := renderLoadTestMarkdown(rec, narrative)
	mustContain(t, md, "## Narrative")
	mustContain(t, md, narrative)
	// Narrative must appear BEFORE Summary in the rendered file.
	narrIdx := strings.Index(md, "## Narrative")
	sumIdx := strings.Index(md, "## Summary")
	if narrIdx < 0 || sumIdx < 0 || narrIdx > sumIdx {
		t.Errorf("Narrative section should precede Summary (narrIdx=%d sumIdx=%d)", narrIdx, sumIdx)
	}
}

func TestRenderLoadTestMarkdown_NoNarrative_OmitsSection(t *testing.T) {
	rec := &loadtest.RunRecord{
		Spec:       loadtest.RunSpec{Destination: "x", Payload: loadtest.Payload{Size: 1}, Ramp: loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 1}},
		BrokerKind: broker.KindNATS,
		Started:    time.Now(),
		Finished:   time.Now(),
		Summary:    loadtest.Summary{Sent: 0},
	}
	md := renderLoadTestMarkdown(rec, "")
	if strings.Contains(md, "## Narrative") {
		t.Error("empty narrative should omit the section")
	}
}

func TestTryNarrateLoadTest_NoAgent_ReturnsEmpty(t *testing.T) {
	a := quietApp(t)
	a.agent = nil
	if got := a.tryNarrateLoadTest(&loadtest.RunRecord{}); got != "" {
		t.Errorf("got %q, want empty (no agent)", got)
	}
}
