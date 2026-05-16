package alertproc

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

	"github.com/argues/argus/internal/alerts"
	"github.com/argues/argus/internal/sqlitedb"
	"github.com/argues/argus/internal/testutil"
)

// fakeNotifier captures every notification the processor emits so
// tests can assert on them without touching Wails runtime.
type fakeNotifier struct {
	mu     sync.Mutex
	alerts []notifiedAlert
	fatigue []fatigueEvent
}
type notifiedAlert struct {
	severity string
	title    string
	body     string
	meta     map[string]any
}
type fatigueEvent struct {
	sig   Signature
	count int
}

func (f *fakeNotifier) NotifyAlert(severity, title, body string, meta map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alerts = append(f.alerts, notifiedAlert{severity, title, body, meta})
}
func (f *fakeNotifier) NotifyFatigue(sig Signature, count int, _ string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fatigue = append(f.fatigue, fatigueEvent{sig, count})
}

type fakeInvestigator struct {
	// atomic because Investigate runs on a goroutine spawned by
	// Processor.Process while the test reads it from the test goroutine.
	calls atomic.Int64
}

func (f *fakeInvestigator) Investigate(ctx context.Context, a alerts.Alert) (*alerts.Diagnosis, error) {
	f.calls.Add(1)
	return &alerts.Diagnosis{
		AlertID:    a.ID,
		Hypothesis: "looked at it, looks fine",
	}, nil
}

func newTestProcessor(t *testing.T) (*Processor, *fakeNotifier, *fakeInvestigator) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := sqlitedb.Open(t.TempDir(), logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	notifier := &fakeNotifier{}
	investigator := &fakeInvestigator{}
	return New(logger, db.DB, investigator, notifier), notifier, investigator
}

func mkAlert(name, ns, pod string) alerts.Alert {
	return alerts.Alert{
		ID:        name + "-" + pod,
		Name:      name,
		Severity:  alerts.SeverityWarning,
		Namespace: ns,
		PodName:   pod,
	}
}

func TestSignatureCollapsesPodSuffix(t *testing.T) {
	a := mkAlert("CrashLoop", "prod", "payments-api-7c9d4b6f8d-abc12")
	b := mkAlert("CrashLoop", "prod", "payments-api-7c9d4b6f8d-zzz77")
	if SignatureOf(a) != SignatureOf(b) {
		t.Errorf("two pods of the same Deployment should share a signature; got %q vs %q",
			SignatureOf(a), SignatureOf(b))
	}
}

func TestSignatureKeepsStatefulSetIndex(t *testing.T) {
	// StatefulSet pods (no random suffix) should NOT collapse.
	a := mkAlert("CrashLoop", "prod", "kafka-0")
	b := mkAlert("CrashLoop", "prod", "kafka-1")
	if SignatureOf(a) == SignatureOf(b) {
		t.Errorf("StatefulSet members should NOT share a signature: %q", SignatureOf(a))
	}
}

func TestProcessDedupesRepeatedFirings(t *testing.T) {
	p, _, _ := newTestProcessor(t)
	a := mkAlert("OOMKilled", "prod", "api-deadbeef-xyz12")

	// First fire: passes through.
	out := p.Process(context.Background(), []alerts.Alert{a})
	if len(out) != 1 {
		t.Fatalf("first fire should pass through; got %d alerts", len(out))
	}
	// Second fire of same signature: deduped.
	out = p.Process(context.Background(), []alerts.Alert{a})
	if len(out) != 0 {
		t.Errorf("second fire should be deduped; got %d alerts", len(out))
	}
}

func TestStaleSignatureRefiresAfterQuietWindow(t *testing.T) {
	// Regression for the dead-code stale path: before the fix, LastSeen
	// was updated BEFORE the stale check, so now.Sub(st.LastSeen) was
	// always ~0 and a long-quiet signature could never re-fire.
	p, _, _ := newTestProcessor(t)
	a := mkAlert("OOMKilled", "prod", "api-deadbeef-xyz12")
	ctx := context.Background()

	// First fire: passes through.
	if out := p.Process(ctx, []alerts.Alert{a}); len(out) != 1 {
		t.Fatalf("first fire should pass through; got %d", len(out))
	}
	// Same signature again right away: deduped.
	if out := p.Process(ctx, []alerts.Alert{a}); len(out) != 0 {
		t.Fatalf("second fire should be deduped; got %d", len(out))
	}

	// Rewind LastSeen by 6 minutes to simulate a long quiet window
	// without sleeping the test out.
	sig := SignatureOf(a)
	p.mu.Lock()
	st := p.state[sig]
	if st == nil {
		p.mu.Unlock()
		t.Fatalf("expected state for signature %q", sig)
	}
	st.LastSeen = time.Now().Add(-6 * time.Minute)
	p.mu.Unlock()

	// Third fire: should re-fire because the signature has been quiet.
	if out := p.Process(ctx, []alerts.Alert{a}); len(out) != 1 {
		t.Errorf("stale signature should re-fire after the quiet window; got %d", len(out))
	}
}

func TestSilenceSuppressesFutureFires(t *testing.T) {
	p, _, _ := newTestProcessor(t)
	a := mkAlert("DiskPressure", "prod", "")
	_ = p.Process(context.Background(), []alerts.Alert{a})

	// User silences for 10s.
	if err := p.Silence(a.ID, false, 10*time.Second, "noisy disk"); err != nil {
		t.Fatalf("silence: %v", err)
	}
	out := p.Process(context.Background(), []alerts.Alert{a})
	if len(out) != 0 {
		t.Errorf("silenced alert should not appear; got %d", len(out))
	}
}

func TestAgentSilenceRefusedWithoutPermission(t *testing.T) {
	p, _, _ := newTestProcessor(t)
	a := mkAlert("DiskPressure", "prod", "")
	_ = p.Process(context.Background(), []alerts.Alert{a})

	// Default profile: CanSilence == false. The agent (byAgent=true)
	// must be denied.
	err := p.Silence(a.ID, true, 10*time.Second, "I think this is noise")
	if !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("expected ErrPermissionDenied; got %v", err)
	}
}

func TestAgentSilenceAllowedWithPermission(t *testing.T) {
	p, _, _ := newTestProcessor(t)
	prof := DefaultProfile()
	prof.CanSilence = true
	if err := p.SetProfile(prof); err != nil {
		t.Fatal(err)
	}
	a := mkAlert("DiskPressure", "prod", "")
	_ = p.Process(context.Background(), []alerts.Alert{a})
	if err := p.Silence(a.ID, true, 10*time.Second, "noise"); err != nil {
		t.Errorf("agent silence should be allowed; got %v", err)
	}
}

func TestFatigueWarningFires(t *testing.T) {
	p, n, _ := newTestProcessor(t)
	prof := DefaultProfile()
	prof.FatigueThreshold = 3
	if err := p.SetProfile(prof); err != nil {
		t.Fatal(err)
	}
	a := mkAlert("FlappingPod", "prod", "")

	// Fire once, silence three times to cross the threshold.
	_ = p.Process(context.Background(), []alerts.Alert{a})
	for i := 0; i < 3; i++ {
		if err := p.Silence(a.ID, false, 1*time.Second, "shut up"); err != nil {
			t.Fatal(err)
		}
	}
	// Trigger a Process tick — fatigue sweep runs at the end.
	_ = p.Process(context.Background(), []alerts.Alert{})
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.fatigue) == 0 {
		t.Error("expected a fatigue event to fire after 3 silences")
	}
	// And the user-facing notification should mention the noise.
	found := false
	for _, na := range n.alerts {
		if strings.Contains(na.body, "losing value") {
			found = true
		}
	}
	if !found {
		t.Error("expected a 'losing value' user notification for fatigue")
	}
}

func TestAutoInvestigateRespectsProfile(t *testing.T) {
	p, _, inv := newTestProcessor(t)
	a := mkAlert("CrashLoop", "prod", "api-x-y-z")

	// AutoInvestigate is on by default → investigator should run on
	// the first firing. Poll the atomic counter rather than sleeping a
	// fixed 50ms — under -race or a slow CI host the spawn-then-run
	// latency can exceed that and turn into a flaky pass.
	_ = p.Process(context.Background(), []alerts.Alert{a})
	testutil.WaitFor(t, time.Second, 5*time.Millisecond,
		func() bool { return inv.calls.Load() > 0 },
		"investigator was not invoked under AutoInvestigate=true")

	// Turn it off and confirm new signatures don't trigger another call.
	prof := DefaultProfile()
	prof.AutoInvestigate = false
	if err := p.SetProfile(prof); err != nil {
		t.Fatal(err)
	}
	before := inv.calls.Load()
	b := mkAlert("DifferentAlert", "prod", "other-x-y-z")
	_ = p.Process(context.Background(), []alerts.Alert{b})
	// A short bounded sleep is the right idiom for a negative
	// assertion ("the goroutine MUST NOT run"). We can't WaitFor a
	// non-event; instead give the spawn-then-run path a generous
	// chance to misbehave, then verify the counter is still unchanged.
	time.Sleep(50 * time.Millisecond)
	if got := inv.calls.Load(); got != before {
		t.Errorf("investigator should not be called when AutoInvestigate=false; calls = %d (was %d)", got, before)
	}
}
