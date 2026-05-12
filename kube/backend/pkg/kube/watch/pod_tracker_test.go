package watch

import (
	"context"
	"log/slog"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func pod(name, namespace, uid string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(uid),
		},
	}
}

func startTracker(t *testing.T, _ *fake.Clientset, initPods ...*corev1.Pod) *PodTracker {
	t.Helper()

	objects := make([]runtime.Object, len(initPods))
	for i, p := range initPods {
		objects[i] = p
	}
	cs := fake.NewSimpleClientset(objects...)

	pt := NewPodTracker(cs, slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pt.Start(ctx); err != nil {
		t.Fatalf("PodTracker.Start() returned error: %v", err)
	}
	return pt
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPodExists(t *testing.T) {
	tests := []struct {
		name      string
		initPods  []*corev1.Pod
		checkNS   string
		checkName string
		want      bool
	}{
		{
			name:      "returns true for known pod",
			initPods:  []*corev1.Pod{pod("my-pod", "default", "uid-1")},
			checkNS:   "default",
			checkName: "my-pod",
			want:      true,
		},
		{
			name:      "returns false for unknown pod",
			initPods:  []*corev1.Pod{pod("known", "default", "uid-1")},
			checkNS:   "default",
			checkName: "unknown",
			want:      false,
		},
		{
			name:      "returns false for pod in different namespace",
			initPods:  []*corev1.Pod{pod("my-pod", "ns-a", "uid-1")},
			checkNS:   "ns-b",
			checkName: "my-pod",
			want:      false,
		},
		{
			name:      "returns false for empty tracker",
			initPods:  []*corev1.Pod{},
			checkNS:   "default",
			checkName: "anything",
			want:      false,
		},
		{
			name:      "handles pods in kube-system namespace",
			initPods:  []*corev1.Pod{pod("coredns", "kube-system", "uid-42")},
			checkNS:   "kube-system",
			checkName: "coredns",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := startTracker(t, nil, tt.initPods...)
			defer pt.Stop()

			got := pt.PodExists(tt.checkNS, tt.checkName)
			if got != tt.want {
				t.Errorf("PodExists(%q, %q) = %v, want %v", tt.checkNS, tt.checkName, got, tt.want)
			}
		})
	}
}

func TestGetPodUID(t *testing.T) {
	tests := []struct {
		name      string
		initPods  []*corev1.Pod
		checkNS   string
		checkName string
		wantUID   string
		wantOK    bool
	}{
		{
			name:      "returns UID for known pod",
			initPods:  []*corev1.Pod{pod("my-pod", "default", "uid-abc123")},
			checkNS:   "default",
			checkName: "my-pod",
			wantUID:   "uid-abc123",
			wantOK:    true,
		},
		{
			name:      "returns false for unknown pod",
			initPods:  []*corev1.Pod{pod("known", "default", "uid-1")},
			checkNS:   "default",
			checkName: "ghost",
			wantUID:   "",
			wantOK:    false,
		},
		{
			name:      "returns false for empty tracker",
			initPods:  []*corev1.Pod{},
			checkNS:   "default",
			checkName: "anything",
			wantUID:   "",
			wantOK:    false,
		},
		{
			name:      "distinguishes pods with same name in different namespaces",
			initPods: []*corev1.Pod{
				pod("shared-name", "ns-a", "uid-a"),
				pod("shared-name", "ns-b", "uid-b"),
			},
			checkNS:   "ns-b",
			checkName: "shared-name",
			wantUID:   "uid-b",
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := startTracker(t, nil, tt.initPods...)
			defer pt.Stop()

			gotUID, gotOK := pt.GetPodUID(tt.checkNS, tt.checkName)
			if gotOK != tt.wantOK {
				t.Errorf("GetPodUID(%q, %q) ok = %v, want %v", tt.checkNS, tt.checkName, gotOK, tt.wantOK)
			}
			if gotUID != tt.wantUID {
				t.Errorf("GetPodUID(%q, %q) uid = %q, want %q", tt.checkNS, tt.checkName, gotUID, tt.wantUID)
			}
		})
	}
}

func TestPodTrackerStartStopLifecycle(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *fake.Clientset
		wantErr  bool
	}{
		{
			name: "start succeeds with no initial pods",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			wantErr: false,
		},
		{
			name: "start succeeds with initial pods",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(pod("p1", "default", "u1"), pod("p2", "kube-system", "u2"))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tt.setup()
			pt := NewPodTracker(cs, slog.Default())

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := pt.Start(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Start() error = %v, wantErr = %v", err, tt.wantErr)
			}

			if err == nil {
				pt.PodExists("default", "p1")
			}

			// Stop should not panic.
			pt.Stop()

			// Double-stop should not panic either.
			pt.Stop()
		})
	}
}

func TestPodTrackerWatchUpdatesIndex(t *testing.T) {
	tests := []struct {
		name      string
		initPods  []*corev1.Pod
		action    func(t *testing.T, ctx context.Context, cs *fake.Clientset)
		checkNS   string
		checkName string
		wantExist bool
		wantUID   string
	}{
		{
			name:     "watch ADD adds pod via clientset Create",
			initPods: []*corev1.Pod{},
			action: func(t *testing.T, ctx context.Context, cs *fake.Clientset) {
				_, err := cs.CoreV1().Pods("default").Create(ctx, pod("new-pod", "default", "uid-new"), metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Create() error: %v", err)
				}
			},
			checkNS:   "default",
			checkName: "new-pod",
			wantExist: true,
			wantUID:   "uid-new",
		},
		{
			name:     "watch DELETE removes pod via clientset Delete",
			initPods: []*corev1.Pod{pod("doomed", "default", "uid-doomed")},
			action: func(t *testing.T, ctx context.Context, cs *fake.Clientset) {
				err := cs.CoreV1().Pods("default").Delete(ctx, "doomed", metav1.DeleteOptions{})
				if err != nil {
					t.Fatalf("Delete() error: %v", err)
				}
			},
			checkNS:   "default",
			checkName: "doomed",
			wantExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := make([]runtime.Object, len(tt.initPods))
			for i, p := range tt.initPods {
				objects[i] = p
			}
			cs := fake.NewSimpleClientset(objects...)

			pt := NewPodTracker(cs, slog.Default())

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := pt.Start(ctx); err != nil {
				t.Fatalf("Start() error: %v", err)
			}
			defer pt.Stop()

			// Give the watch goroutine time to start listening.
			time.Sleep(200 * time.Millisecond)

			// Perform the action (e.g., Create/Delete a pod).
			tt.action(t, ctx, cs)

			// Give the watch goroutine time to process the event.
			time.Sleep(500 * time.Millisecond)

			got := pt.PodExists(tt.checkNS, tt.checkName)
			if got != tt.wantExist {
				t.Errorf("PodExists(%q, %q) after watch event = %v, want %v", tt.checkNS, tt.checkName, got, tt.wantExist)
			}

			if tt.wantExist && tt.wantUID != "" {
				uid, ok := pt.GetPodUID(tt.checkNS, tt.checkName)
				if !ok {
					t.Errorf("GetPodUID(%q, %q) returned ok=false after watch event", tt.checkNS, tt.checkName)
				} else if uid != tt.wantUID {
					t.Errorf("after watch event, GetPodUID = %q, want %q", uid, tt.wantUID)
				}
			}
		})
	}
}

func TestPodTrackerWatchNonPodObjectIgnored(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "non-pod object in watch event is silently ignored via reactor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := fake.NewSimpleClientset()

			watcher := watch.NewFakeWithChanSize(10, false)
			cs.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(watcher, nil))

			pt := NewPodTracker(cs, slog.Default())

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := pt.Start(ctx); err != nil {
				t.Fatalf("Start() error: %v", err)
			}
			defer pt.Stop()

			time.Sleep(200 * time.Millisecond)

			// Send a non-pod object (a Node) via the fake watcher — should not panic.
			watcher.Add(&corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "fake-node", Namespace: "default"},
			})

			time.Sleep(200 * time.Millisecond)

			// The tracker should still be empty and usable.
			if pt.PodExists("default", "fake-node") {
				t.Error("node object should not be added as a pod")
			}

			// The tracker should still accept real pod watch events.
			watcher.Add(pod("real-pod", "default", "uid-real"))
			time.Sleep(200 * time.Millisecond)

			if !pt.PodExists("default", "real-pod") {
				t.Error("real pod was not added after node event was ignored")
			}
		})
	}
}
