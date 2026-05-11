package spotcheck

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/argues/argus/internal/alerts"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

// ──────────────────────────────────────────────────────────────────
// Fakes
// ──────────────────────────────────────────────────────────────────

// fakeNotifier records all calls to Active and Notify.
type fakeNotifier struct {
	mu       sync.Mutex
	active   []string          // each call: "checkName:description"
	notified []notifiedFinding // each call
}

type notifiedFinding struct {
	checkName string
	finding   Finding
}

func (f *fakeNotifier) Active(checkName, description string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active = append(f.active, checkName+":"+description)
}

func (f *fakeNotifier) Notify(checkName string, finding Finding) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notified = append(f.notified, notifiedFinding{checkName: checkName, finding: finding})
}

func (f *fakeNotifier) ActiveCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.active))
	copy(out, f.active)
	return out
}

func (f *fakeNotifier) Notified() []notifiedFinding {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]notifiedFinding, len(f.notified))
	copy(out, f.notified)
	return out
}

func (f *fakeNotifier) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.active = nil
	f.notified = nil
}

// fakeCheck implements Check with controllable Name/Description/Run.
type fakeCheck struct {
	name        string
	description string
	runFn       func(context.Context) (*Finding, error)
}

func (f *fakeCheck) Name() string        { return f.name }
func (f *fakeCheck) Description() string { return f.description }
func (f *fakeCheck) Run(ctx context.Context) (*Finding, error) {
	if f.runFn != nil {
		return f.runFn(ctx)
	}
	return nil, nil
}

// fakeMetricsProvider implements MetricsProvider with a controllable return.
type fakeMetricsProvider struct {
	metrics *alerts.ClusterMetrics
}

func (f *fakeMetricsProvider) CurrentMetrics() *alerts.ClusterMetrics {
	return f.metrics
}

// ──────────────────────────────────────────────────────────────────
// Helper: newTestEngine creates an engine with a fake notifier.
// ──────────────────────────────────────────────────────────────────

func newTestEngine(notifier *fakeNotifier, interval time.Duration) *Engine {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return New(logger, notifier, interval)
}

// ──────────────────────────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notifier := &fakeNotifier{}

	tests := []struct {
		name     string
		interval time.Duration
		want     time.Duration
	}{
		{
			name:     "uses given interval",
			interval: 5 * time.Minute,
			want:     5 * time.Minute,
		},
		{
			name:     "defaults to 30m when interval is zero",
			interval: 0,
			want:     30 * time.Minute,
		},
		{
			name:     "defaults to 30m when interval is negative",
			interval: -1 * time.Second,
			want:     30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := New(logger, notifier, tt.interval)
			if e.interval != tt.want {
				t.Errorf("got interval %v, want %v", e.interval, tt.want)
			}
			if e.notifier == nil {
				t.Error("notifier should not be nil")
			}
			if e.checks == nil {
				t.Error("checks map should not be nil")
			}
		})
	}
}

func TestAddAndNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		checks []Check
		want   []string // expected names (order-independent)
	}{
		{
			name:   "no checks",
			checks: nil,
			want:   []string{},
		},
		{
			name: "single check",
			checks: []Check{
				&fakeCheck{name: "foo", description: "foo check"},
			},
			want: []string{"foo"},
		},
		{
			name: "multiple checks",
			checks: []Check{
				&fakeCheck{name: "alpha", description: "alpha check"},
				&fakeCheck{name: "beta", description: "beta check"},
				&fakeCheck{name: "gamma", description: "gamma check"},
			},
			want: []string{"alpha", "beta", "gamma"},
		},
		{
			name: "last write wins on duplicate name",
			checks: []Check{
				&fakeCheck{name: "dup", description: "first"},
				&fakeCheck{name: "dup", description: "second"},
			},
			want: []string{"dup"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newTestEngine(&fakeNotifier{}, time.Minute)
			for _, c := range tt.checks {
				e.Add(c)
			}
			got := e.Names()

			if len(got) != len(tt.want) {
				t.Fatalf("got %d names %v, want %d names %v", len(got), got, len(tt.want), tt.want)
			}

			wantMap := make(map[string]bool, len(tt.want))
			for _, n := range tt.want {
				wantMap[n] = true
			}
			for _, n := range got {
				if !wantMap[n] {
					t.Errorf("unexpected name %q", n)
				}
				delete(wantMap, n)
			}
		})
	}
}

func TestRunAll(t *testing.T) {
	t.Parallel()

	t.Run("runs all checks and emits active then idle", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		e := newTestEngine(notifier, time.Minute)

		ran := make(map[string]bool)
		var mu sync.Mutex

		e.Add(&fakeCheck{
			name: "a", description: "check a",
			runFn: func(ctx context.Context) (*Finding, error) {
				mu.Lock()
				ran["a"] = true
				mu.Unlock()
				return nil, nil
			},
		})
		e.Add(&fakeCheck{
			name: "b", description: "check b",
			runFn: func(ctx context.Context) (*Finding, error) {
				mu.Lock()
				ran["b"] = true
				mu.Unlock()
				return nil, nil
			},
		})

		e.RunAll(context.Background())

		mu.Lock()
		if !ran["a"] || !ran["b"] {
			t.Errorf("not all checks ran: %v", ran)
		}
		mu.Unlock()

		active := notifier.ActiveCalls()
		if len(active) < 2 {
			t.Fatalf("expected at least 2 active calls, got %d: %v", len(active), active)
		}
		// Last call must be idle signal.
		last := active[len(active)-1]
		if last != ":" {
			t.Errorf("expected final idle signal ':', got %q", last)
		}
	})

	t.Run("continues after a failing check", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		e := newTestEngine(notifier, time.Minute)
		var ranB bool

		e.Add(&fakeCheck{
			name: "failing", description: "failing check",
			runFn: func(ctx context.Context) (*Finding, error) {
				return nil, assertError("boom")
			},
		})
		e.Add(&fakeCheck{
			name: "good", description: "good check",
			runFn: func(ctx context.Context) (*Finding, error) {
				ranB = true
				return nil, nil
			},
		})

		e.RunAll(context.Background())
		if !ranB {
			t.Error("expected good check to run after failing check")
		}

		notified := notifier.Notified()
		if len(notified) != 1 {
			t.Fatalf("expected 1 notification for failing check, got %d", len(notified))
		}
		if notified[0].finding.Severity != SevError {
			t.Errorf("expected SevError, got %s", notified[0].finding.Severity)
		}
	})

	t.Run("respects context cancellation mid-run", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		e := newTestEngine(notifier, time.Minute)

		blocker := make(chan struct{})
		e.Add(&fakeCheck{
			name: "blocking", description: "blocking check",
			runFn: func(ctx context.Context) (*Finding, error) {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-blocker:
					return nil, nil
				}
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		e.RunAll(ctx)
		close(blocker) // unblock if not cancelled

		active := notifier.ActiveCalls()
		if len(active) == 0 {
			t.Fatal("expected at least one active call")
		}
		last := active[len(active)-1]
		if last != ":" {
			t.Errorf("expected idle signal after cancellation, got %q", last)
		}
	})
}

func TestRunOne(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func() (*Engine, *fakeNotifier)
		runName string
		wantErr error
	}{
		{
			name: "runs known check",
			setup: func() (*Engine, *fakeNotifier) {
				n := &fakeNotifier{}
				e := newTestEngine(n, time.Minute)
				e.Add(&fakeCheck{name: "known", description: "known check"})
				return e, n
			},
			runName: "known",
			wantErr: nil,
		},
		{
			name: "returns error for unknown check",
			setup: func() (*Engine, *fakeNotifier) {
				n := &fakeNotifier{}
				e := newTestEngine(n, time.Minute)
				return e, n
			},
			runName: "unknown",
			wantErr: ErrUnknownCheck,
		},
		{
			name: "handles check error by notifying",
			setup: func() (*Engine, *fakeNotifier) {
				n := &fakeNotifier{}
				e := newTestEngine(n, time.Minute)
				e.Add(&fakeCheck{
					name: "erratic", description: "erratic check",
					runFn: func(ctx context.Context) (*Finding, error) {
						return nil, assertError("something went wrong")
					},
				})
				return e, n
			},
			runName: "erratic",
			wantErr: nil,
		},
		{
			name: "silent pass with nil finding",
			setup: func() (*Engine, *fakeNotifier) {
				n := &fakeNotifier{}
				e := newTestEngine(n, time.Minute)
				e.Add(&fakeCheck{
					name: "silent", description: "silent check",
					runFn: func(ctx context.Context) (*Finding, error) {
						return nil, nil
					},
				})
				return e, n
			},
			runName: "silent",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e, n := tt.setup()
			err := e.RunOne(context.Background(), tt.runName)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("got error %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// For known checks that return nil finding, notifier should not receive Notify.
			active := n.ActiveCalls()
			if len(active) < 1 {
				t.Error("expected at least one Active call")
			}
		})
	}
}

func TestRunOneNotifierCalledCorrectly(t *testing.T) {
	t.Parallel()

	t.Run("sends finding via notifier when check returns a finding", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		e := newTestEngine(notifier, time.Minute)

		e.Add(&fakeCheck{
			name: "finder", description: "findings check",
			runFn: func(ctx context.Context) (*Finding, error) {
				return &Finding{Severity: SevInfo, Title: "something", Body: "details"}, nil
			},
		})

		err := e.RunOne(context.Background(), "finder")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		notified := notifier.Notified()
		if len(notified) != 1 {
			t.Fatalf("expected 1 notification, got %d", len(notified))
		}
		if notified[0].checkName != "finder" {
			t.Errorf("expected check name 'finder', got %q", notified[0].checkName)
		}
		if notified[0].finding.Severity != SevInfo {
			t.Errorf("expected SevInfo, got %s", notified[0].finding.Severity)
		}
		if notified[0].finding.Title != "something" {
			t.Errorf("expected title 'something', got %q", notified[0].finding.Title)
		}
		// Meta should have check and durationMs injected.
		if notified[0].finding.Meta == nil {
			t.Fatal("Meta should not be nil")
		}
		if notified[0].finding.Meta["check"] != "finder" {
			t.Errorf("expected Meta[check]='finder', got %v", notified[0].finding.Meta["check"])
		}
		if _, ok := notified[0].finding.Meta["durationMs"]; !ok {
			t.Error("expected Meta[durationMs] to exist")
		}
		// Last active call should be idle signal.
		active := notifier.ActiveCalls()
		last := active[len(active)-1]
		if last != ":" {
			t.Errorf("expected final idle ':', got %q", last)
		}
	})

	t.Run("does not notify on nil finding", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		e := newTestEngine(notifier, time.Minute)

		e.Add(&fakeCheck{
			name: "quiet", description: "quiet check",
			runFn: func(ctx context.Context) (*Finding, error) {
				return nil, nil
			},
		})

		err := e.RunOne(context.Background(), "quiet")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(notifier.Notified()) != 0 {
			t.Error("expected no notifications for nil finding")
		}
	})

	t.Run("notifies with SevError on check error", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		e := newTestEngine(notifier, time.Minute)

		e.Add(&fakeCheck{
			name: "crashy", description: "crashy check",
			runFn: func(ctx context.Context) (*Finding, error) {
				return nil, assertError("disk full")
			},
		})

		err := e.RunOne(context.Background(), "crashy")
		if err != nil {
			t.Fatalf("RunOne should not surface check errors, got: %v", err)
		}

		notified := notifier.Notified()
		if len(notified) != 1 {
			t.Fatalf("expected 1 notification, got %d", len(notified))
		}
		if notified[0].finding.Severity != SevError {
			t.Errorf("expected SevError, got %s", notified[0].finding.Severity)
		}
		if notified[0].finding.Meta["check"] != "crashy" {
			t.Errorf("expected Meta[check]='crashy', got %v", notified[0].finding.Meta["check"])
		}
	})
}

func TestStartLoop(t *testing.T) {
	t.Parallel()

	t.Run("cancels immediately when context already cancelled", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		e := newTestEngine(notifier, time.Minute)
		e.Add(&fakeCheck{name: "fast", description: "fast check"})

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		start := time.Now()
		e.StartLoop(ctx)
		// Give the goroutine a moment to exit.
		time.Sleep(50 * time.Millisecond)

		if time.Since(start) > 2*time.Second {
			t.Error("StartLoop took too long to exit on cancelled context")
		}
	})

	t.Run("runs checks and stops when context is cancelled during delay", func(t *testing.T) {
		t.Parallel()
		notifier := &fakeNotifier{}
		e := newTestEngine(notifier, time.Hour) // long interval, not relevant
		e.Add(&fakeCheck{name: "pulse", description: "pulse check"})

		// Cancel after a short delay — should interrupt the initial 15s sleep.
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		e.StartLoop(ctx)
		time.Sleep(100 * time.Millisecond)

		// The goroutine should have exited cleanly without panicking.
		// We can't assert on ActiveCalls because the check might not fire
		// before cancellation, but we can verify no panic occurred.
	})
}

func TestRunOneErrorReturnsErrUnknownCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		checks  []Check
		runName string
		wantErr error
	}{
		{
			name:    "empty engine",
			checks:  nil,
			runName: "anything",
			wantErr: ErrUnknownCheck,
		},
		{
			name:    "not registered",
			checks:  []Check{&fakeCheck{name: "a", description: "a"}},
			runName: "b",
			wantErr: ErrUnknownCheck,
		},
		{
			name:    "finds registered check",
			checks:  []Check{&fakeCheck{name: "a", description: "a"}},
			runName: "a",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := newTestEngine(&fakeNotifier{}, time.Minute)
			for _, c := range tt.checks {
				e.Add(c)
			}
			err := e.RunOne(context.Background(), tt.runName)
			if err != tt.wantErr {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// Concrete check tests
// ──────────────────────────────────────────────────────────────────

func TestNodeReadyCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		nodes    []runtime.Object
		k8sNil   bool
		wantNil  bool   // expect nil finding (silent pass)
		wantSev  Severity // expected finding severity (if not nil)
		wantBody string   // substring expected in body
	}{
		{
			name:    "nil client returns error",
			k8sNil:  true,
			wantNil: false,
			wantSev: SevError,
		},
		{
			name: "all nodes ready no pressure",
			nodes: []runtime.Object{
				readyNode("node-1"),
				readyNode("node-2"),
			},
			wantNil: true,
		},
		{
			name: "one node not ready",
			nodes: []runtime.Object{
				readyNode("node-1"),
				notReadyNode("node-2"),
			},
			wantNil:  false,
			wantSev:  SevError,
			wantBody: "Not Ready",
		},
		{
			name: "one node with memory pressure",
			nodes: []runtime.Object{
				pressureNode("node-1", corev1.NodeMemoryPressure),
			},
			wantNil:  false,
			wantSev:  SevWarn,
			wantBody: "MemoryPressure",
		},
		{
			name: "one node with disk pressure",
			nodes: []runtime.Object{
				pressureNode("node-1", corev1.NodeDiskPressure),
			},
			wantNil:  false,
			wantSev:  SevWarn,
			wantBody: "DiskPressure",
		},
		{
			name: "one node with PID pressure",
			nodes: []runtime.Object{
				pressureNode("node-1", corev1.NodePIDPressure),
			},
			wantNil:  false,
			wantSev:  SevWarn,
			wantBody: "PIDPressure",
		},
		{
			name: "not ready + pressure combined",
			nodes: []runtime.Object{
				nodeWithConditions("node-1", []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue},
				}),
			},
			wantNil:  false,
			wantSev:  SevError,
			wantBody: "Not Ready",
		},
		{
			name: "node has unknown ready condition",
			nodes: []runtime.Object{
				nodeWithConditions("node-unknown", []corev1.NodeCondition{
					{Type: corev1.NodeReady, Status: corev1.ConditionUnknown},
				}),
			},
			wantNil:  false,
			wantSev:  SevError,
			wantBody: "Not Ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var c NodeReadyCheck
			if tt.k8sNil {
				c.K8s = nil
			} else {
				cs := fake.NewSimpleClientset(tt.nodes...)
				c.K8s = &testK8sClient{cs: cs}
			}

			finding, err := c.Run(context.Background())
			if tt.wantNil {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if finding != nil {
					t.Errorf("expected nil finding, got %+v", *finding)
				}
				return
			}

			if tt.wantSev == SevError && err == nil && finding == nil {
				t.Fatal("expected error or finding with error severity, got nil")
			}

			if finding != nil {
				if finding.Severity != tt.wantSev {
					t.Errorf("expected severity %q, got %q", tt.wantSev, finding.Severity)
				}
				if tt.wantBody != "" && !stringContains(finding.Body, tt.wantBody) {
					t.Errorf("expected body containing %q, got %q", tt.wantBody, finding.Body)
				}
				if finding.Meta == nil {
					t.Error("Meta should not be nil")
				}
				if _, ok := finding.Meta["total"]; !ok {
					t.Error("expected Meta[total] to exist")
				}
			}
		})
	}
}

func TestMetricsCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		metrics *alerts.ClusterMetrics
		minH    float64
		maxE    float64
		wantNil bool
		wantSev Severity
		wantErr bool
	}{
		{
			name:    "nil metrics returns nil finding",
			metrics: nil,
			wantNil: true,
		},
		{
			name:    "zero pods returns nil finding",
			metrics: &alerts.ClusterMetrics{PodsTotal: 0},
			wantNil: true,
		},
		{
			name: "healthy metrics returns nil finding",
			metrics: &alerts.ClusterMetrics{
				PodHealthPct:  98.0,
				PodsRunning:   10,
				PodsTotal:     10,
				ErrorRate:     1.0,
				WarningEvents: 2,
			},
			wantNil: true,
		},
		{
			name: "low health triggers warn",
			metrics: &alerts.ClusterMetrics{
				PodHealthPct:  85.0,
				PodsRunning:   8,
				PodsTotal:     10,
				ErrorRate:     2.0,
				WarningEvents: 3,
			},
			minH:    90,
			wantNil: false,
			wantSev: SevWarn,
		},
		{
			name: "very low health triggers error",
			metrics: &alerts.ClusterMetrics{
				PodHealthPct:  70.0,
				PodsRunning:   7,
				PodsTotal:     10,
				ErrorRate:     10.0,
				WarningEvents: 5,
			},
			wantNil: false,
			wantSev: SevError,
		},
		{
			name: "high error rate triggers warn",
			metrics: &alerts.ClusterMetrics{
				PodHealthPct:  98.0,
				PodsRunning:   10,
				PodsTotal:     10,
				ErrorRate:     8.0,
				WarningEvents: 2,
			},
			maxE:    5,
			wantNil: false,
			wantSev: SevWarn,
		},
		{
			name: "very high error rate triggers error",
			metrics: &alerts.ClusterMetrics{
				PodHealthPct:  95.0,
				PodsRunning:   10,
				PodsTotal:     10,
				ErrorRate:     20.0,
				WarningEvents: 2,
			},
			wantNil: false,
			wantSev: SevError,
		},
		{
			name: "many warning events triggers warn",
			metrics: &alerts.ClusterMetrics{
				PodHealthPct:  98.0,
				PodsRunning:   10,
				PodsTotal:     10,
				ErrorRate:     1.0,
				WarningEvents: 15,
			},
			wantNil: false,
			wantSev: SevWarn,
		},
		{
			name: "uses custom thresholds",
			metrics: &alerts.ClusterMetrics{
				PodHealthPct:  93.0,
				PodsRunning:   10,
				PodsTotal:     10,
				ErrorRate:     3.0,
				WarningEvents: 1,
			},
			minH:    99,
			maxE:    1,
			wantNil: false,
			wantSev: SevWarn,
		},
		{
			name: "exactly at threshold is okay",
			metrics: &alerts.ClusterMetrics{
				PodHealthPct:  95.0,
				PodsRunning:   10,
				PodsTotal:     10,
				ErrorRate:     5.0,
				WarningEvents: 5,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := MetricsCheck{
				Source:       &fakeMetricsProvider{metrics: tt.metrics},
				MinHealthPct: tt.minH,
				MaxErrorRate: tt.maxE,
			}

			finding, err := c.Run(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNil {
				if finding != nil {
					t.Errorf("expected nil finding, got %+v", *finding)
				}
				return
			}

			if finding == nil {
				t.Fatal("expected non-nil finding, got nil")
			}
			if finding.Severity != tt.wantSev {
				t.Errorf("expected severity %q, got %q", tt.wantSev, finding.Severity)
			}
			if finding.Meta == nil {
				t.Error("Meta should not be nil")
			}
			if _, ok := finding.Meta["podHealthPct"]; !ok {
				t.Error("expected Meta[podHealthPct]")
			}
		})
	}
}

func TestDecisionLogFreshnessCheck(t *testing.T) {
	t.Parallel()

	// Create a temp dir for test files.
	dir := t.TempDir()

	// Create a fresh file.
	freshPath := filepath.Join(dir, "fresh.md")
	if err := os.WriteFile(freshPath, []byte("fresh"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a stale file.
	stalePath := filepath.Join(dir, "stale.md")
	if err := os.WriteFile(stalePath, []byte("stale"), 0644); err != nil {
		t.Fatal(err)
	}
	// Set mod time far in the past.
	past := time.Now().Add(-60 * 24 * time.Hour) // 60 days ago
	if err := os.Chtimes(stalePath, past, past); err != nil {
		t.Fatal(err)
	}
	// Create a very stale file.
	veryStalePath := filepath.Join(dir, "verystale.md")
	if err := os.WriteFile(veryStalePath, []byte("very stale"), 0644); err != nil {
		t.Fatal(err)
	}
	pastVery := time.Now().Add(-365 * 24 * time.Hour) // 1 year ago
	if err := os.Chtimes(veryStalePath, pastVery, pastVery); err != nil {
		t.Fatal(err)
	}

	nonexistentPath := filepath.Join(dir, "nonexistent.md")

	tests := []struct {
		name       string
		path       string
		maxStale   time.Duration
		wantNil    bool
		wantSev    Severity
		wantTitle  string
		wantBody   string
	}{
		{
			name:     "fresh file returns nil finding",
			path:     freshPath,
			wantNil: true,
		},
		{
			name:      "stale file returns warn",
			path:      stalePath,
			maxStale:  30 * 24 * time.Hour,
			wantNil:   false,
			wantSev:   SevWarn,
			wantTitle: "Decision log stale",
		},
		{
			name:      "very stale file returns warn",
			path:      veryStalePath,
			maxStale:  30 * 24 * time.Hour,
			wantNil:   false,
			wantSev:   SevWarn,
			wantTitle: "Decision log stale",
		},
		{
			name:      "file not found returns info",
			path:      nonexistentPath,
			wantNil:   false,
			wantSev:   SevInfo,
			wantTitle: "Decision log not found",
		},
		{
			name:    "empty path uses default",
			path:    "",
			wantNil: false, // DECISION_LOG.md likely doesn't exist
			wantSev: SevInfo,
		},
		{
			name:      "custom max stale allows fresh",
			path:      stalePath,
			maxStale:  90 * 24 * time.Hour,
			wantNil:   true,
		},
		{
			name:       "stale file includes path in body",
			path:       stalePath,
			maxStale:   30 * 24 * time.Hour,
			wantNil:    false,
			wantSev:    SevWarn,
			wantBody:   stalePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := DecisionLogFreshnessCheck{
				Path:             tt.path,
				MaxStaleDuration: tt.maxStale,
			}

			finding, err := c.Run(context.Background())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantNil {
				if finding != nil {
					t.Errorf("expected nil finding, got %+v", *finding)
				}
				return
			}

			if finding == nil {
				t.Fatal("expected non-nil finding, got nil")
			}
			if finding.Severity != tt.wantSev {
				t.Errorf("expected severity %q, got %q", tt.wantSev, finding.Severity)
			}
			if tt.wantTitle != "" && !stringContains(finding.Title, tt.wantTitle) {
				t.Errorf("expected title containing %q, got %q", tt.wantTitle, finding.Title)
			}
			if tt.wantBody != "" && !stringContains(finding.Body, tt.wantBody) {
				t.Errorf("expected body containing %q, got %q", tt.wantBody, finding.Body)
			}
			if finding.Meta == nil {
				t.Error("Meta should not be nil")
			}
			if _, ok := finding.Meta["path"]; !ok {
				t.Error("expected Meta[path]")
			}
		})
	}
}

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

// testK8sClient wraps a fake.Clientset to satisfy the K8sClient interface
// that NodeReadyCheck requires.
type testK8sClient struct {
	cs kubernetes.Interface
}

func (c *testK8sClient) GetClientset() kubernetes.Interface {
	return c.cs
}

// assertError is a simple error type for test assertions.
type assertError string

func (e assertError) Error() string { return string(e) }

// stringContains reports whether substr is within s.
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ── Node helpers ──────────────────────────────────────────────────

func readyNode(name string) *corev1.Node {
	return nodeWithConditions(name, []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
	})
}

func notReadyNode(name string) *corev1.Node {
	return nodeWithConditions(name, []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
	})
}

func pressureNode(name string, pressureType corev1.NodeConditionType) *corev1.Node {
	return nodeWithConditions(name, []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
		{Type: pressureType, Status: corev1.ConditionTrue},
	})
}

func nodeWithConditions(name string, conditions []corev1.NodeCondition) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Conditions: conditions,
		},
	}
}
