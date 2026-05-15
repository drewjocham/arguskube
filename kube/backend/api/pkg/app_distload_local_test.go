package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argues/argus/internal/saasapi"
	"github.com/argues/argus/internal/sqlitedb"
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
// otherwise.
func quietApp(t *testing.T) *App {
	t.Helper()
	return &App{
		logger: slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

// goodLocalDistSpec builds a DistLoadSpec with Runner="local" that
// passes Validate when translated to a RunSpec.
func goodLocalDistSpec(t *testing.T) saasapi.DistLoadSpec {
	t.Helper()
	brokerJSON, err := json.Marshal(broker.Config{
		Kind: broker.KindNATS,
		NATS: &broker.NATSConfig{Servers: "nats://x"},
	})
	if err != nil {
		t.Fatalf("marshal broker: %v", err)
	}
	return saasapi.DistLoadSpec{
		Runner:      "local",
		Name:        "unit-test",
		Broker:      brokerJSON,
		Destination: "test.subject",
		PayloadSize: 9,
		Count:       20,
		Workers:     4,
		RampProfile: "constant",
		RampRate:    1000,
	}
}

// startTestLocalDistLoad wires a fake publisher into the App's loadtest
// factory hook, then calls StartDistributedLoadTest with a local spec.
// The hook is set BEFORE the engine goroutine spawns, so the engine
// never dials a real broker.
func startTestLocalDistLoad(t *testing.T, a *App, spec saasapi.DistLoadSpec, pub broker.Publisher) string {
	t.Helper()
	a.setLoadtestPublisherFactory(func(_ context.Context, _ broker.Config, _ *slog.Logger) (broker.Publisher, error) {
		return pub, nil
	})
	t.Cleanup(func() { a.setLoadtestPublisherFactory(nil) })

	id, err := a.StartDistributedLoadTest(spec)
	if err != nil {
		t.Fatalf("StartDistributedLoadTest: %v", err)
	}
	return id
}

// awaitLocalState polls a local run's state up to dur; returns the
// last state seen. Reads directly from the in-process registry so
// this works without a SaaS client wired into the App.
func awaitLocalState(t *testing.T, a *App, runID string, want string, dur time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(dur)
	for time.Now().Before(deadline) {
		run, ok := a.localRun(runID)
		if !ok {
			t.Fatalf("local run %s not found", runID)
		}
		if s := run.activeState(); s == want {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	run, _ := a.localRun(runID)
	if run == nil {
		return ""
	}
	return run.activeState()
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

func TestListDistLoadPresets(t *testing.T) {
	a := quietApp(t)
	ps := a.ListDistLoadPresets()
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

func TestListDistLoadBrokerKinds(t *testing.T) {
	a := quietApp(t)
	k := a.ListDistLoadBrokerKinds()
	// Knowns now includes REST as well as the five message-broker
	// adapters. The test asserts the count so a future addition shows
	// up here as a deliberate change.
	if len(k) != 6 {
		t.Errorf("got %d broker kinds, want 6", len(k))
	}
}

func TestStartDistributedLoadTest_LocalRejectsInvalidSpec(t *testing.T) {
	a := quietApp(t)
	bad := goodLocalDistSpec(t)
	bad.Destination = ""
	_, err := a.StartDistributedLoadTest(bad)
	if err == nil {
		t.Fatal("expected error for invalid spec")
	}
}

func TestStartDistributedLoadTest_RejectsUnknownRunner(t *testing.T) {
	a := quietApp(t)
	spec := goodLocalDistSpec(t)
	spec.Runner = "moon"
	_, err := a.StartDistributedLoadTest(spec)
	if err == nil {
		t.Fatal("expected error for unknown runner")
	}
}

func TestStartDistributedLoadTest_LocalRejectsConcurrent(t *testing.T) {
	a := quietApp(t)
	spec := goodLocalDistSpec(t)
	spec.Count = 1000
	// 100/s * 1000 = 10s of work — plenty for a second StartXxx call
	// to land while the first is still running.
	spec.RampRate = 100

	firstID := startTestLocalDistLoad(t, a, spec, &fakeLoadtestPublisher{ackLatency: time.Millisecond})
	awaitLocalState(t, a, firstID, "running", time.Second)

	_, err := a.StartDistributedLoadTest(spec)
	if err == nil {
		t.Fatal("expected second start to fail (concurrent)")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("error = %q, want it to mention 'already running'", err.Error())
	}
	_ = a.CancelDistributedLoadTest(firstID)
}

func TestStartDistributedLoadTest_LocalHappyPath(t *testing.T) {
	a := quietApp(t)
	spec := goodLocalDistSpec(t)
	spec.Count = 5
	spec.RampRate = 5000 // burst

	id := startTestLocalDistLoad(t, a, spec, &fakeLoadtestPublisher{ackLatency: 100 * time.Microsecond})

	state := awaitLocalState(t, a, id, "done", 2*time.Second)
	if state != "done" {
		t.Fatalf("final state = %q, want %q", state, "done")
	}

	st, err := a.GetDistributedLoadTestStatus(id)
	if err != nil {
		t.Fatalf("GetDistributedLoadTestStatus: %v", err)
	}
	if st.Summary == nil || st.Summary.TotalSent != 5 {
		t.Errorf("Summary.TotalSent = %+v, want 5", st.Summary)
	}
	if st.Error != "" {
		t.Errorf("Error = %q (should be empty on happy path)", st.Error)
	}
	if len(st.Workers) != 1 || st.Workers[0].Region != "local" {
		t.Errorf("Workers = %+v, want single local worker", st.Workers)
	}
}

func TestCancelDistributedLoadTest_Local(t *testing.T) {
	a := quietApp(t)
	spec := goodLocalDistSpec(t)
	spec.Count = 100_000
	spec.RampRate = 100 // long run
	id := startTestLocalDistLoad(t, a, spec, &fakeLoadtestPublisher{ackLatency: time.Millisecond})

	time.Sleep(50 * time.Millisecond)
	if err := a.CancelDistributedLoadTest(id); err != nil {
		t.Fatalf("CancelDistributedLoadTest: %v", err)
	}
	state := awaitLocalState(t, a, id, "canceled", 2*time.Second)
	if state != "canceled" {
		t.Errorf("post-cancel state = %q, want canceled", state)
	}
}

func TestGetDistributedLoadTestRecord_NotFinished(t *testing.T) {
	a := quietApp(t)
	spec := goodLocalDistSpec(t)
	spec.Count = 1000
	spec.RampRate = 10 // takes a while
	id := startTestLocalDistLoad(t, a, spec, &fakeLoadtestPublisher{ackLatency: time.Millisecond})

	_, err := a.GetDistributedLoadTestRecord(id)
	if err == nil {
		t.Error("expected error for in-flight run")
	}
	_ = a.CancelDistributedLoadTest(id)
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
		Spec: loadtest.RunSpec{
			Destination: "x",
			Payload:     loadtest.Payload{Size: 1},
			Ramp:        loadtest.Ramp{Kind: loadtest.RampConstant, Rate: 1},
		},
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

// ---------------------------------------------------------------------------
// Payload + quota tests (PR adding rich payload sources + 5/day cap)
// ---------------------------------------------------------------------------

func TestGenerateLoadTestPayload_NilAgent(t *testing.T) {
	a := quietApp(t)
	a.agent = nil
	_, err := a.GenerateLoadTestPayload("anything", 0)
	if err == nil {
		t.Fatal("expected error when agent is nil")
	}
	if !strings.Contains(err.Error(), "AI agent disabled") {
		t.Errorf("err = %q, want it to mention 'AI agent disabled'", err.Error())
	}
}

func TestResolveLocalPayloadPath_RejectsRoot(t *testing.T) {
	a := quietApp(t)
	if _, err := a.ResolveLocalPayloadPath("/"); err == nil {
		t.Fatal("expected error for root path")
	}
	if _, err := a.ResolveLocalPayloadPath(""); err == nil {
		t.Fatal("expected error for empty path")
	}
	if _, err := a.ResolveLocalPayloadPath("/etc/../tmp"); err == nil {
		t.Fatal("expected error for path with '..'")
	}
}

func TestResolveLocalPayloadPath_AcceptsTempDir(t *testing.T) {
	a := quietApp(t)
	dir := t.TempDir()
	// Write a small json file so the listing has content.
	fp := dir + "/a.json"
	if err := os.WriteFile(fp, []byte(`{"x":1}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	res, err := a.ResolveLocalPayloadPath(dir)
	if err != nil {
		t.Fatalf("ResolveLocalPayloadPath: %v", err)
	}
	if res.Kind != "dir" {
		t.Errorf("kind = %q, want dir", res.Kind)
	}
	if len(res.Files) != 1 || res.Files[0].Name != "a.json" {
		t.Errorf("files = %+v", res.Files)
	}
	if !strings.Contains(res.Sample, `"x":1`) {
		t.Errorf("sample = %q", res.Sample)
	}
}

// quotaTestApp wires the App to a real on-disk SQLite so the quota
// migration runs and INSERT/COUNT exercise the real schema.
func quotaTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	sdb, err := sqlitedb.Open(dir, logger)
	if err != nil {
		t.Fatalf("sqlitedb open: %v", err)
	}
	t.Cleanup(func() { _ = sdb.Close() })
	return &App{logger: logger, db: sdb.DB}
}

func TestLocalDistLoad_QuotaCapsFreeTier(t *testing.T) {
	a := quotaTestApp(t)
	spec := goodLocalDistSpec(t)
	spec.Count = 5
	spec.RampRate = 5000

	for i := 0; i < 5; i++ {
		id := startTestLocalDistLoad(t, a, spec, &fakeLoadtestPublisher{ackLatency: 100 * time.Microsecond})
		if got := awaitLocalState(t, a, id, "done", 2*time.Second); got != "done" {
			t.Fatalf("run %d state = %q", i, got)
		}
	}

	// 6th start must fail with the quota error.
	_, err := a.StartDistributedLoadTest(spec)
	if err == nil {
		t.Fatal("expected 6th start to fail (quota)")
	}
	if !strings.Contains(err.Error(), "limited to 5/day") {
		t.Errorf("err = %q, want it to mention '5/day'", err.Error())
	}

	// GetLocalDistLoadQuota should report the cap.
	q, err := a.GetLocalDistLoadQuota()
	if err != nil {
		t.Fatalf("GetLocalDistLoadQuota: %v", err)
	}
	if q.Used != 5 || q.Limit != 5 {
		t.Errorf("quota = %+v, want used=5 limit=5", q)
	}
}

// TestLocalDistLoad_QuotaIsAtomic ensures the count+insert pair cannot
// double-spend the free-tier cap under concurrent Start calls. We bypass
// the engine path (reserveLocalQuotaSlot directly) to isolate the race.
func TestLocalDistLoad_QuotaIsAtomic(t *testing.T) {
	a := quotaTestApp(t)
	a.ctx = context.Background()
	const goroutines = 20
	var wg sync.WaitGroup
	var okCount, rejCount int32
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			runID := fmt.Sprintf("race-%d", i)
			_, _, _, err := a.reserveLocalQuotaSlot(runID, time.Now())
			if err == nil {
				atomic.AddInt32(&okCount, 1)
			} else if errors.Is(err, ErrLocalQuotaExceeded) {
				atomic.AddInt32(&rejCount, 1)
			} else {
				t.Errorf("unexpected err: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if okCount != int32(localQuotaFreeLimit) {
		t.Fatalf("expected exactly %d successful reservations, got %d ok + %d rejected",
			localQuotaFreeLimit, okCount, rejCount)
	}
	if rejCount != goroutines-int32(localQuotaFreeLimit) {
		t.Fatalf("expected %d rejections, got %d", goroutines-int32(localQuotaFreeLimit), rejCount)
	}
}

// TestProgressThrottler_FlushTerminatesGoroutine guards the fix for
// the throttler-goroutine leak: newProgressThrottler spawns a ticker
// goroutine that only exits when flush() closes its channel. If
// startLocalDistLoad bails out between throttler creation and
// ownership transfer, the guarded defer in the caller must flush so
// the goroutine doesn't leak. This test exercises the flush directly.
func TestProgressThrottler_FlushTerminatesGoroutine(t *testing.T) {
	a := quietApp(t)
	before := runtime.NumGoroutine()

	p := newProgressThrottler(a, "rid", 50*time.Millisecond)

	// Confirm a goroutine was spawned. If the implementation ever
	// changed to lazy-start, the leak guard would become unnecessary —
	// this assertion makes that pivot visible.
	if got := runtime.NumGoroutine(); got <= before {
		t.Fatalf("newProgressThrottler did not spawn a goroutine; before=%d after=%d", before, got)
	}

	p.flush()

	// Allow the ticker goroutine to observe closeCh and exit. The
	// runtime scheduler doesn't synchronously rendezvous, so poll.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("throttler goroutine did not exit after flush(); before=%d after=%d",
		before, runtime.NumGoroutine())
}

// TestProgressThrottler_FlushIsIdempotent guards the second half of
// the leak fix: the caller defers a guarded flush AND runLoadTest
// also flushes after the engine returns. Both calls hit the same
// throttler in the happy path, so flush() must be safe to call
// repeatedly (sync.Once around close(closeCh)).
func TestProgressThrottler_FlushIsIdempotent(t *testing.T) {
	a := quietApp(t)
	p := newProgressThrottler(a, "rid", 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		p.flush()
	}
}

// TestResolveRunner exercises the dispatch-layer default + reject path.
func TestResolveRunner(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "cloud", false},
		{"cloud", "cloud", false},
		{"local", "local", false},
		{"moon", "", true},
	}
	for _, c := range cases {
		got, err := resolveRunner(saasapi.DistLoadSpec{Runner: c.in})
		if (err != nil) != c.wantErr {
			t.Errorf("resolveRunner(%q): err=%v wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if got != c.want {
			t.Errorf("resolveRunner(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
