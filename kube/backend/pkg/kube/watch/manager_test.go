package watch

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/argues/argus/pkg/kube"
)

// ---------------------------------------------------------------------------
// mockClient is a hand-written mock of kube.ClientInterface used by Manager
// tests. It returns canned data for each of the three poll methods.
// ---------------------------------------------------------------------------

type mockClient struct {
	nodesFn       func(ctx context.Context) ([]kube.NodeInfo, error)
	podsFn        func(ctx context.Context) ([]kube.PodInfo, error)
	eventsFn      func(ctx context.Context) ([]kube.EventInfo, error)

	// Satisfy the rest of the interface with no-ops / panics so we catch
	// unexpected calls.
	kube.ClientInterface
}

func (m *mockClient) GetNodes(ctx context.Context) ([]kube.NodeInfo, error) {
	return m.nodesFn(ctx)
}

func (m *mockClient) GetPodsAllNamespaces(ctx context.Context) ([]kube.PodInfo, error) {
	return m.podsFn(ctx)
}

func (m *mockClient) GetEventsAllNamespaces(ctx context.Context) ([]kube.EventInfo, error) {
	return m.eventsFn(ctx)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestNewManager(t *testing.T) {
	tests := []struct {
		name     string
		client   kube.ClientInterface
		logger   *slog.Logger
		interval time.Duration
		wantNil  bool
	}{
		{
			name:     "creates manager with valid arguments",
			client:   &mockClient{},
			logger:   slog.Default(),
			interval: time.Minute,
			wantNil:  false,
		},
		{
			name:     "creates manager with zero interval",
			client:   &mockClient{},
			logger:   slog.Default(),
			interval: 0,
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.client, tt.logger, tt.interval)
			if (m == nil) != tt.wantNil {
				t.Errorf("NewManager() = %v, wantNil = %v", m, tt.wantNil)
			}
		})
	}
}

func TestManagerStartReturnsChannel(t *testing.T) {
	tests := []struct {
		name     string
		client   kube.ClientInterface
		interval time.Duration
	}{
		{
			name: "returns non-nil channel",
			client: &mockClient{
				nodesFn:  func(ctx context.Context) ([]kube.NodeInfo, error) { return nil, nil },
				podsFn:   func(ctx context.Context) ([]kube.PodInfo, error) { return nil, nil },
				eventsFn: func(ctx context.Context) ([]kube.EventInfo, error) { return nil, nil },
			},
			interval: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(tt.client, slog.Default(), tt.interval)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := m.Start(ctx)
			if ch == nil {
				t.Fatal("Start() returned nil channel")
			}

			// Give it a moment to run the initial poll, then cancel.
			time.Sleep(50 * time.Millisecond)
			cancel()

			// Channel should be closed after cancel.
			_, ok := <-ch
			if ok {
				t.Error("expected channel to be closed after context cancel")
			}
		})
	}
}

func TestManagerNodeNotReadyAlert(t *testing.T) {
	tests := []struct {
		name    string
		nodes   []kube.NodeInfo
		want    int // expected alert count
		wantKey string // expected Key() of first alert (if want > 0)
	}{
		{
			name: "all nodes ready produces no alerts",
			nodes: []kube.NodeInfo{
				{Name: "node-1", Status: "Ready"},
				{Name: "node-2", Status: "Ready"},
			},
			want: 0,
		},
		{
			name: "not-ready node produces alert",
			nodes: []kube.NodeInfo{
				{Name: "node-1", Status: "Ready"},
				{Name: "node-2", Status: "NotReady"},
			},
			want:    1,
			wantKey: "node//node-2/NodeNotReady",
		},
		{
			name: "multiple not-ready nodes produce multiple alerts",
			nodes: []kube.NodeInfo{
				{Name: "node-1", Status: "NotReady"},
				{Name: "node-2", Status: "Unknown"},
				{Name: "node-3", Status: "Ready"},
			},
			want: 2,
		},
		{
			name:    "empty node list produces no alerts",
			nodes:   []kube.NodeInfo{},
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockClient{
				nodesFn: func(ctx context.Context) ([]kube.NodeInfo, error) { return tt.nodes, nil },
				podsFn:  func(ctx context.Context) ([]kube.PodInfo, error) { return nil, nil },
				eventsFn: func(ctx context.Context) ([]kube.EventInfo, error) { return nil, nil },
			}
			m := NewManager(client, slog.Default(), time.Minute)

			ch := make(chan Alert, 64)
			m.checkNodes(context.Background(), ch)
			close(ch)

			var got []Alert
			for a := range ch {
				got = append(got, a)
			}
			if len(got) != tt.want {
				t.Errorf("checkNodes() produced %d alerts, want %d", len(got), tt.want)
			}
			if tt.want > 0 && tt.wantKey != "" && len(got) > 0 {
				if got[0].Key() != tt.wantKey {
					t.Errorf("alert.Key() = %q, want %q", got[0].Key(), tt.wantKey)
				}
			}
		})
	}
}

func TestManagerFailingPodAlerts(t *testing.T) {
	tests := []struct {
		name  string
		pods  []kube.PodInfo
		want  int  // total expected alerts
		wantRestart bool // expect a HighRestarts alert in the results
	}{
		{
			name: "healthy pods produce no alerts",
			pods: []kube.PodInfo{
				{Name: "pod-a", Namespace: "default", Status: "Running", RestartCount: 0},
				{Name: "pod-b", Namespace: "default", Status: "Running", RestartCount: 2},
			},
			want: 0,
		},
		{
			name: "CrashLoopBackOff pod produces alert",
			pods: []kube.PodInfo{
				{Name: "broken-pod", Namespace: "default", Status: "CrashLoopBackOff", RestartCount: 3},
			},
			want: 1,
		},
		{
			name: "Error status pod produces alert",
			pods: []kube.PodInfo{
				{Name: "err-pod", Namespace: "ns1", Status: "Error", RestartCount: 0},
			},
			want: 1,
		},
		{
			name: "ImagePullBackOff pod produces alert",
			pods: []kube.PodInfo{
				{Name: "img-pod", Namespace: "ns2", Status: "ImagePullBackOff", RestartCount: 1},
			},
			want: 1,
		},
		{
			name: "ErrImagePull pod produces alert",
			pods: []kube.PodInfo{
				{Name: "err-img", Namespace: "ns3", Status: "ErrImagePull", RestartCount: 0},
			},
			want: 1,
		},
		{
			name: "unknown failure status does not produce alert",
			pods: []kube.PodInfo{
				{Name: "unknown-pod", Namespace: "default", Status: "Init:Error", RestartCount: 0},
			},
			want: 0,
		},
		{
			name: "high restart count produces HighRestarts alert",
			pods: []kube.PodInfo{
				{Name: "flaky-pod", Namespace: "default", Status: "Running", RestartCount: 15},
			},
			want:        1,
			wantRestart: true,
		},
		{
			name: "high restart on failing pod produces two alerts",
			pods: []kube.PodInfo{
				{Name: "double-alert", Namespace: "default", Status: "CrashLoopBackOff", RestartCount: 20},
			},
			want: 2,
		},
		{
			name: "exactly 10 restarts does not produce HighRestarts alert",
			pods: []kube.PodInfo{
				{Name: "boundary", Namespace: "default", Status: "Running", RestartCount: 10},
			},
			want: 0,
		},
		{
			name: "empty pod list produces no alerts",
			pods: []kube.PodInfo{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockClient{
				nodesFn:  func(ctx context.Context) ([]kube.NodeInfo, error) { return nil, nil },
				podsFn:   func(ctx context.Context) ([]kube.PodInfo, error) { return tt.pods, nil },
				eventsFn: func(ctx context.Context) ([]kube.EventInfo, error) { return nil, nil },
			}
			m := NewManager(client, slog.Default(), time.Minute)

			ch := make(chan Alert, 64)
			m.checkPods(context.Background(), ch)
			close(ch)

			var got []Alert
			for a := range ch {
				got = append(got, a)
			}

			if len(got) != tt.want {
				t.Errorf("checkPods() produced %d alerts, want %d", len(got), tt.want)
			}

			if tt.wantRestart {
				found := false
				for _, a := range got {
					if a.Reason == "HighRestarts" {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected a HighRestarts alert but none found")
				}
			}
		})
	}
}

func TestManagerWarningEventAlerts(t *testing.T) {
	now := time.Now()
	recent := now.Add(-time.Minute)
	old := now.Add(-10 * time.Minute)

	tests := []struct {
		name     string
		events   []kube.EventInfo
		interval time.Duration
		want     int
	}{
		{
			name:     "recent warning event produces alert",
			events: []kube.EventInfo{
				{Type: "Warning", Reason: "OOMKilling", ObjectName: "pod-1", Message: "memory cgroup out of memory", LastTimestamp: recent},
			},
			interval: 5 * time.Minute,
			want:     1,
		},
		{
			name: "non-warning type event does not produce alert",
			events: []kube.EventInfo{
				{Type: "Normal", Reason: "Started", ObjectName: "pod-1", Message: "container started", LastTimestamp: recent},
			},
			interval: 5 * time.Minute,
			want:     0,
		},
		{
			name: "old warning event does not produce alert",
			events: []kube.EventInfo{
				{Type: "Warning", Reason: "Unhealthy", ObjectName: "pod-2", Message: "liveness probe failed", LastTimestamp: old},
			},
			interval: time.Minute,
			want:     0,
		},
		{
			name:     "empty event list produces no alerts",
			events:   []kube.EventInfo{},
			interval: time.Minute,
			want:     0,
		},
		{
			name: "multiple warning events produce multiple alerts",
			events: []kube.EventInfo{
				{Type: "Warning", Reason: "BackOff", ObjectName: "pod-1", Message: "back-off", LastTimestamp: recent},
				{Type: "Warning", Reason: "FailedMount", ObjectName: "pod-2", Message: "volume failed", LastTimestamp: recent},
				{Type: "Normal", Reason: "Pulled", ObjectName: "pod-3", Message: "pulled image", LastTimestamp: recent},
			},
			interval: 5 * time.Minute,
			want:     2,
		},
		{
			name: "event just after cutoff is included",
			events: []kube.EventInfo{
				// cutoff = now - interval*2 = now - 2min. Event at now - 1min59s is After(cutoff).
				{Type: "Warning", Reason: "NodeNotReady", ObjectName: "node-1", Message: "node not ready", LastTimestamp: now.Add(-1*time.Minute - 59*time.Second)},
			},
			interval: 1 * time.Minute,
			want:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockClient{
				nodesFn:  func(ctx context.Context) ([]kube.NodeInfo, error) { return nil, nil },
				podsFn:   func(ctx context.Context) ([]kube.PodInfo, error) { return nil, nil },
				eventsFn: func(ctx context.Context) ([]kube.EventInfo, error) { return tt.events, nil },
			}
			m := NewManager(client, slog.Default(), tt.interval)

			ch := make(chan Alert, 64)
			m.checkEvents(context.Background(), ch)
			close(ch)

			var got []Alert
			for a := range ch {
				got = append(got, a)
			}
			if len(got) != tt.want {
				t.Errorf("checkEvents() produced %d alerts, want %d", len(got), tt.want)
			}
		})
	}
}

func TestManagerHandlesClientErrorsGracefully(t *testing.T) {
	tests := []struct {
		name      string
		nodesErr  error
		podsErr   error
		eventsErr error
	}{
		{
			name:      "node check error is logged and skipped",
			nodesErr:  assertError("connection refused"),
			podsErr:   nil,
			eventsErr: nil,
		},
		{
			name:      "pod check error is logged and skipped",
			nodesErr:  nil,
			podsErr:   assertError("forbidden"),
			eventsErr: nil,
		},
		{
			name:      "event check error is logged and skipped",
			nodesErr:  nil,
			podsErr:   nil,
			eventsErr: assertError("timeout"),
		},
		{
			name:      "all checks error but no panic",
			nodesErr:  assertError("err-a"),
			podsErr:   assertError("err-b"),
			eventsErr: assertError("err-c"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockClient{
				nodesFn: func(ctx context.Context) ([]kube.NodeInfo, error) {
					return nil, tt.nodesErr
				},
				podsFn: func(ctx context.Context) ([]kube.PodInfo, error) {
					return nil, tt.podsErr
				},
				eventsFn: func(ctx context.Context) ([]kube.EventInfo, error) {
					return nil, tt.eventsErr
				},
			}
			m := NewManager(client, slog.Default(), time.Minute)

			// poll() should not panic, and should not send any alerts on error.
			ch := make(chan Alert, 64)
			m.poll(context.Background(), ch)
			close(ch)

			var got []Alert
			for a := range ch {
				got = append(got, a)
			}
			if len(got) != 0 {
				t.Errorf("poll() produced %d alerts on error, want 0", len(got))
			}
		})
	}
}

func TestManagerFullPollIntegration(t *testing.T) {
	tests := []struct {
		name   string
		nodes  []kube.NodeInfo
		pods   []kube.PodInfo
		events []kube.EventInfo
		want   int
	}{
		{
			name: "mixed cluster state produces correct alert count",
			nodes: []kube.NodeInfo{
				{Name: "master-1", Status: "Ready"},
				{Name: "worker-1", Status: "NotReady"},
			},
			pods: []kube.PodInfo{
				{Name: "good-pod", Namespace: "default", Status: "Running", RestartCount: 1},
				{Name: "bad-pod", Namespace: "kube-system", Status: "CrashLoopBackOff", RestartCount: 5},
				{Name: "flaky-pod", Namespace: "default", Status: "Running", RestartCount: 20},
			},
			events: []kube.EventInfo{
				{Type: "Warning", Reason: "OOMKilling", ObjectName: "bad-pod", Message: "oom", LastTimestamp: time.Now().Add(-30 * time.Second)},
				{Type: "Normal", Reason: "Started", ObjectName: "good-pod", Message: "started", LastTimestamp: time.Now().Add(-time.Minute)},
			},
			want: 4, // 1 node not-ready + 1 crash-loop + 1 high-restarts + 1 warning event
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockClient{
				nodesFn:  func(ctx context.Context) ([]kube.NodeInfo, error) { return tt.nodes, nil },
				podsFn:   func(ctx context.Context) ([]kube.PodInfo, error) { return tt.pods, nil },
				eventsFn: func(ctx context.Context) ([]kube.EventInfo, error) { return tt.events, nil },
			}
			m := NewManager(client, slog.Default(), time.Minute)

			ch := make(chan Alert, 64)
			m.poll(context.Background(), ch)
			close(ch)

			var got []Alert
			for a := range ch {
				got = append(got, a)
			}
			if len(got) != tt.want {
				t.Errorf("poll() produced %d alerts, want %d. Got: %+v", len(got), tt.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Alert.Key() deduplication tests
// ---------------------------------------------------------------------------

func TestAlertKeyDeduplication(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name   string
		alert1 Alert
		alert2 Alert
		wantEqual bool
	}{
		{
			name: "same kind, namespace, name, reason produces same key",
			alert1: Alert{Kind: AlertKindPod, Namespace: "default", Name: "my-pod", Reason: "CrashLoopBackOff", OccurredAt: now},
			alert2: Alert{Kind: AlertKindPod, Namespace: "default", Name: "my-pod", Reason: "CrashLoopBackOff", OccurredAt: now.Add(time.Hour)},
			wantEqual: true,
		},
		{
			name: "different kind produces different key",
			alert1: Alert{Kind: AlertKindNode, Namespace: "", Name: "node-1", Reason: "NodeNotReady"},
			alert2: Alert{Kind: AlertKindPod, Namespace: "default", Name: "node-1", Reason: "NodeNotReady"},
			wantEqual: false,
		},
		{
			name: "different namespace produces different key",
			alert1: Alert{Kind: AlertKindPod, Namespace: "ns1", Name: "my-pod", Reason: "Error"},
			alert2: Alert{Kind: AlertKindPod, Namespace: "ns2", Name: "my-pod", Reason: "Error"},
			wantEqual: false,
		},
		{
			name: "different name produces different key",
			alert1: Alert{Kind: AlertKindPod, Namespace: "default", Name: "pod-a", Reason: "OOMKilling"},
			alert2: Alert{Kind: AlertKindPod, Namespace: "default", Name: "pod-b", Reason: "OOMKilling"},
			wantEqual: false,
		},
		{
			name: "different reason produces different key",
			alert1: Alert{Kind: AlertKindEvent, Namespace: "", Name: "pod-1", Reason: "OOMKilling"},
			alert2: Alert{Kind: AlertKindEvent, Namespace: "", Name: "pod-1", Reason: "BackOff"},
			wantEqual: false,
		},
		{
			name: "different message does not affect key",
			alert1: Alert{Kind: AlertKindNode, Namespace: "", Name: "n1", Reason: "NodeNotReady", Message: "msg1"},
			alert2: Alert{Kind: AlertKindNode, Namespace: "", Name: "n1", Reason: "NodeNotReady", Message: "msg2"},
			wantEqual: true,
		},
		{
			name: "empty values produce predictable key",
			alert1: Alert{Kind: "", Namespace: "", Name: "", Reason: ""},
			alert2: Alert{Kind: "", Namespace: "", Name: "", Reason: ""},
			wantEqual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k1 := tt.alert1.Key()
			k2 := tt.alert2.Key()
			equal := k1 == k2
			if equal != tt.wantEqual {
				t.Errorf("Key() equality = %v, want %v. k1=%q, k2=%q", equal, tt.wantEqual, k1, k2)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// assertError is a simple error type so we can verify error paths.
type assertError string

func (e assertError) Error() string { return string(e) }
