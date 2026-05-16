package alertproc_test

// Pipeline integration test for CR P3.18. Exercises the
// DetectAlerts → Process → Investigate → Notify chain end-to-end with
// every external dependency replaced by an in-process fake:
//   * detector — a stub that emits a fixed alert batch
//   * investigator — counts calls and returns a canned diagnosis
//   * notifier — records every NotifyAlert / NotifyFatigue invocation
//
// This is the "full pipeline" view CR P3.18 asks for — exhausts the
// per-package tests miss when they don't span the seam between alert
// detection and notification surface.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argues/argus/internal/alertproc"
	"github.com/argues/argus/internal/alerts"
	"github.com/argues/argus/internal/sqlitedb"
)

// fakeDetector stands in for k8s.Client.DetectAlerts. The test caller
// drives it by appending to Next; the pipeline consumes one batch per
// call to DetectAlerts.
type fakeDetector struct {
	mu   sync.Mutex
	next [][]alerts.Alert
}

func (f *fakeDetector) DetectAlerts(_ context.Context) ([]alerts.Alert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.next) == 0 {
		return nil, nil
	}
	batch := f.next[0]
	f.next = f.next[1:]
	return batch, nil
}

func (f *fakeDetector) enqueue(batch []alerts.Alert) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next = append(f.next, batch)
}

// recordingNotifier captures alerts and fatigue events.
type recordingNotifier struct {
	mu       sync.Mutex
	alerts   []recordedAlert
	fatigue  []recordedFatigue
}
type recordedAlert struct {
	severity, title, body string
	meta                  map[string]any
}
type recordedFatigue struct {
	sig   alertproc.Signature
	count int
	title string
}

func (n *recordingNotifier) NotifyAlert(severity, title, body string, meta map[string]any) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.alerts = append(n.alerts, recordedAlert{severity, title, body, meta})
}

func (n *recordingNotifier) NotifyFatigue(sig alertproc.Signature, count int, title string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.fatigue = append(n.fatigue, recordedFatigue{sig, count, title})
}

// recordingInvestigator returns a canned diagnosis and tracks calls.
type recordingInvestigator struct {
	calls atomic.Int64
}

func (i *recordingInvestigator) Investigate(ctx context.Context, a alerts.Alert) (*alerts.Diagnosis, error) {
	i.calls.Add(1)
	return &alerts.Diagnosis{
		AlertID:    a.ID,
		Hypothesis: "investigated by the integration test",
	}, nil
}

// failingInvestigator always errors — used to verify the notifier
// doesn't fire on investigation failure.
type failingInvestigator struct {
	calls atomic.Int64
}

func (f *failingInvestigator) Investigate(_ context.Context, _ alerts.Alert) (*alerts.Diagnosis, error) {
	f.calls.Add(1)
	return nil, errors.New("investigation failed")
}

func newPipeline(t *testing.T, inv alertproc.Investigator) (*alertproc.Processor, *recordingNotifier) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := sqlitedb.Open(t.TempDir(), logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	notifier := &recordingNotifier{}
	return alertproc.New(logger, db.DB, inv, notifier), notifier
}

// TestPipeline_FirstFiringInvestigatesAndNotifies is the happy path:
// a single new alert flows through detection → processing →
// investigation → notification. Asserts the investigator was called
// once and the notifier received the "Agent investigated" message.
func TestPipeline_FirstFiringInvestigatesAndNotifies(t *testing.T) {
	inv := &recordingInvestigator{}
	p, n := newPipeline(t, inv)

	det := &fakeDetector{}
	det.enqueue([]alerts.Alert{
		{ID: "a-1", Name: "CrashLoopBackOff", Namespace: "prod", PodName: "api-1", Severity: alerts.SeverityWarning},
	})

	// Drive one pipeline iteration manually — production code calls
	// DetectAlerts on a ticker; the test pumps it once.
	batch, err := det.DetectAlerts(context.Background())
	if err != nil {
		t.Fatalf("DetectAlerts: %v", err)
	}
	out := p.Process(context.Background(), batch)

	if len(out) != 1 {
		t.Fatalf("Process returned %d alerts, want 1", len(out))
	}

	// Wait for the background investigation goroutine to complete
	// (scheduleInvestigation runs it on a fresh goroutine).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && inv.calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if got := inv.calls.Load(); got != 1 {
		t.Errorf("investigator calls = %d, want 1", got)
	}

	// And the notifier should have received the agent-investigated
	// alert with the canned hypothesis text.
	for time.Now().Before(deadline) {
		n.mu.Lock()
		count := len(n.alerts)
		n.mu.Unlock()
		if count > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.alerts) == 0 {
		t.Fatal("notifier did not receive any alerts")
	}
	if !strings.Contains(n.alerts[0].body, "integration test") {
		t.Errorf("notifier body did not include the diagnosis hypothesis: %q", n.alerts[0].body)
	}
}

// TestPipeline_DuplicateFiringDeduped — a second occurrence of the
// same signature within the dedup window must NOT pass through
// Process and must NOT re-fire the investigator.
func TestPipeline_DuplicateFiringDeduped(t *testing.T) {
	inv := &recordingInvestigator{}
	p, _ := newPipeline(t, inv)

	alert := alerts.Alert{ID: "a-1", Name: "OOMKilled", Namespace: "prod", PodName: "api-1"}
	if out := p.Process(context.Background(), []alerts.Alert{alert}); len(out) != 1 {
		t.Fatalf("first fire: %d alerts; want 1", len(out))
	}
	// Wait for the investigation to land so the test isn't racing it.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && inv.calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}

	// Second fire of the same signature: deduped.
	if out := p.Process(context.Background(), []alerts.Alert{alert}); len(out) != 0 {
		t.Errorf("second fire: %d alerts; want 0 (deduped)", len(out))
	}

	// Give the (would-be) goroutine time to misbehave — it shouldn't.
	time.Sleep(100 * time.Millisecond)
	if got := inv.calls.Load(); got != 1 {
		t.Errorf("investigator calls = %d, want 1 (deduped fire must not re-investigate)", got)
	}
}

// TestPipeline_InvestigationFailureDoesNotNotify — when Investigate
// returns an error, the processor must NOT surface a notification
// containing a stale diagnosis. The user-facing notifier stays quiet
// for the failed run.
func TestPipeline_InvestigationFailureDoesNotNotify(t *testing.T) {
	inv := &failingInvestigator{}
	p, n := newPipeline(t, inv)

	alert := alerts.Alert{ID: "a-failure", Name: "Unhealthy", Namespace: "prod", PodName: "api-2"}
	_ = p.Process(context.Background(), []alerts.Alert{alert})

	// Let the investigation goroutine finish (it fails).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && inv.calls.Load() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // safety margin for notifier write

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.alerts) != 0 {
		t.Errorf("failed investigation must not produce a user notification; got %d", len(n.alerts))
	}
}

// TestPipeline_FatigueWarningFiresOnRepeatedSilences exercises the
// end-of-pipeline fatigue surface: after the user silences the same
// signature N times, the fatigue meta-alert fires once.
func TestPipeline_FatigueWarningFiresOnRepeatedSilences(t *testing.T) {
	inv := &recordingInvestigator{}
	p, n := newPipeline(t, inv)

	prof := alertproc.DefaultProfile()
	prof.FatigueThreshold = 3
	if err := p.SetProfile(prof); err != nil {
		t.Fatal(err)
	}

	alert := alerts.Alert{ID: "noisy-pod", Name: "Flapping", Namespace: "prod", PodName: "noisy"}

	// First fire passes through.
	_ = p.Process(context.Background(), []alerts.Alert{alert})

	// User silences three times to cross the fatigue threshold.
	for i := 0; i < 3; i++ {
		if err := p.Silence(alert.ID, false, time.Second, "stop"); err != nil {
			t.Fatal(err)
		}
	}

	// Tick the processor with no alerts to trigger the fatigue sweep.
	_ = p.Process(context.Background(), []alerts.Alert{})

	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.fatigue) == 0 {
		t.Fatal("expected fatigue event after 3 silences")
	}
}
