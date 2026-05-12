package k8s

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argues/argus/internal/config"
)

func testClient(objects ...corev1.Pod) *Client {
	cs := fake.NewSimpleClientset()
	ctx := context.Background()
	for i := range objects {
		_, _ = cs.CoreV1().Pods(objects[i].Namespace).Create(ctx, &objects[i], metav1.CreateOptions{})
	}
	return &Client{
		cs:     cs,
		cfg:    &config.OnlineDataConfig{},
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

// testClientWithNodes creates a client pre-loaded with nodes.
func testClientWithNodes(nodes ...corev1.Node) *Client {
	cs := fake.NewSimpleClientset()
	ctx := context.Background()
	for i := range nodes {
		_, _ = cs.CoreV1().Nodes().Create(ctx, &nodes[i], metav1.CreateOptions{})
	}
	return &Client{
		cs:     cs,
		cfg:    &config.OnlineDataConfig{},
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

func TestGetClusterInfo(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []corev1.Node
		ctxName   string
		wantNodes int
		wantName  string
	}{
		{
			name:      "empty cluster",
			nodes:     nil,
			ctxName:   "my-cluster",
			wantNodes: 0,
			wantName:  "my-cluster",
		},
		{
			name: "two nodes",
			nodes: []corev1.Node{
				{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
			},
			ctxName:   "prod",
			wantNodes: 2,
			wantName:  "prod",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testClientWithNodes(tt.nodes...)
			c.cfg.Kubernetes.Context = tt.ctxName

			info, err := c.GetClusterInfo(context.Background())
			if err != nil {
				t.Fatalf("GetClusterInfo() error = %v", err)
			}
			if info.NodeCount != tt.wantNodes {
				t.Errorf("NodeCount = %d, want %d", info.NodeCount, tt.wantNodes)
			}
			if info.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", info.Name, tt.wantName)
			}
		})
	}
}

func TestGetMetrics(t *testing.T) {
	tests := []struct {
		name          string
		pods          []corev1.Pod
		wantTotal     int
		wantRunning   int
		wantFailed    int
		wantSLO       string
		wantHealthGt0 bool
	}{
		{
			name:        "empty cluster",
			pods:        nil,
			wantTotal:   0,
			wantRunning: 0,
			wantSLO:     "ok",
		},
		{
			name: "all healthy",
			pods: []corev1.Pod{
				makePod("web-1", "default", corev1.PodRunning, nil),
				makePod("web-2", "default", corev1.PodRunning, nil),
			},
			wantTotal:     2,
			wantRunning:   2,
			wantSLO:       "ok",
			wantHealthGt0: true,
		},
		{
			name: "slo breach — many failed",
			pods: []corev1.Pod{
				makePod("ok-1", "default", corev1.PodRunning, nil),
				makePod("fail-1", "default", corev1.PodFailed, nil),
				makePod("fail-2", "default", corev1.PodFailed, nil),
				makePod("fail-3", "default", corev1.PodFailed, nil),
				makePod("fail-4", "default", corev1.PodFailed, nil),
				makePod("fail-5", "default", corev1.PodFailed, nil),
			},
			wantTotal:   6,
			wantRunning: 1,
			wantFailed:  5,
			wantSLO:     "breach",
		},
		{
			name: "with resource requests",
			pods: []corev1.Pod{
				makePod("api-1", "default", corev1.PodRunning, &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				}),
			},
			wantTotal:     1,
			wantRunning:   1,
			wantSLO:       "ok",
			wantHealthGt0: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testClient(tt.pods...)
			m, err := c.GetMetrics(context.Background())
			if err != nil {
				t.Fatalf("GetMetrics() error = %v", err)
			}
			if m.PodsTotal != tt.wantTotal {
				t.Errorf("PodsTotal = %d, want %d", m.PodsTotal, tt.wantTotal)
			}
			if m.PodsRunning != tt.wantRunning {
				t.Errorf("PodsRunning = %d, want %d", m.PodsRunning, tt.wantRunning)
			}
			if m.PodsFailed != tt.wantFailed {
				t.Errorf("PodsFailed = %d, want %d", m.PodsFailed, tt.wantFailed)
			}
			if m.SLOStatus != tt.wantSLO {
				t.Errorf("SLOStatus = %q, want %q", m.SLOStatus, tt.wantSLO)
			}
			if tt.wantHealthGt0 && m.PodHealthPct <= 0 {
				t.Errorf("PodHealthPct = %f, want > 0", m.PodHealthPct)
			}
		})
	}
}

func TestDetectAlerts(t *testing.T) {
	tests := []struct {
		name       string
		pods       []corev1.Pod
		nodes      []corev1.Node
		wantCount  int
		wantAlerts []string // expected alert ID prefixes
	}{
		{
			name:      "healthy cluster",
			pods:      []corev1.Pod{makePod("web-1", "default", corev1.PodRunning, nil)},
			wantCount: 0,
		},
		{
			name: "OOMKilled",
			pods: []corev1.Pod{
				makeOOMPod("oom-app", "default"),
			},
			wantCount:  1,
			wantAlerts: []string{"oom-"},
		},
		{
			name: "CrashLoopBackOff",
			pods: []corev1.Pod{
				makeCrashLoopPod("crash-app", "default"),
			},
			wantCount:  1,
			wantAlerts: []string{"crash-"},
		},
		{
			name: "ImagePullBackOff",
			pods: []corev1.Pod{
				makeImagePullPod("pull-app", "default"),
			},
			wantCount:  1,
			wantAlerts: []string{"imgpull-"},
		},
		{
			name: "high restarts",
			pods: []corev1.Pod{
				makeHighRestartPod("flaky-app", "default", 10),
			},
			wantCount:  1,
			wantAlerts: []string{"restart-"},
		},
		{
			name: "node disk pressure",
			pods: nil,
			nodes: []corev1.Node{
				makeDiskPressureNode("node-1"),
			},
			wantCount:  1,
			wantAlerts: []string{"disk-"},
		},
		{
			name: "multiple alerts",
			pods: []corev1.Pod{
				makeOOMPod("oom-1", "default"),
				makeCrashLoopPod("crash-1", "prod"),
			},
			wantCount: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := fake.NewSimpleClientset()
			ctx := context.Background()
			for i := range tt.pods {
				_, _ = cs.CoreV1().Pods(tt.pods[i].Namespace).Create(ctx, &tt.pods[i], metav1.CreateOptions{})
			}
			for i := range tt.nodes {
				_, _ = cs.CoreV1().Nodes().Create(ctx, &tt.nodes[i], metav1.CreateOptions{})
			}
			c := &Client{
				cs:     cs,
				cfg:    &config.OnlineDataConfig{},
				logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
			}

			got, err := c.DetectAlerts(ctx)
			if err != nil {
				t.Fatalf("DetectAlerts() error = %v", err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("alert count = %d, want %d", len(got), tt.wantCount)
				for _, a := range got {
					t.Logf("  alert: %s (%s)", a.ID, a.Name)
				}
			}
			for i, prefix := range tt.wantAlerts {
				if i < len(got) {
					if got[i].ID[:len(prefix)] != prefix {
						t.Errorf("alert[%d].ID = %q, want prefix %q", i, got[i].ID, prefix)
					}
				}
			}
		})
	}
}

func TestGetNamespacePodCounts(t *testing.T) {
	tests := []struct {
		name string
		pods []corev1.Pod
		want map[string]int
	}{
		{
			name: "empty",
			pods: nil,
			want: map[string]int{},
		},
		{
			name: "mixed namespaces",
			pods: []corev1.Pod{
				makePod("a", "ns1", corev1.PodRunning, nil),
				makePod("b", "ns1", corev1.PodRunning, nil),
				makePod("c", "ns2", corev1.PodRunning, nil),
				makePod("d", "ns2", corev1.PodFailed, nil), // not counted
			},
			want: map[string]int{"ns1": 2, "ns2": 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testClient(tt.pods...)
			got, err := c.GetNamespacePodCounts(context.Background())
			if err != nil {
				t.Fatalf("GetNamespacePodCounts() error = %v", err)
			}
			for ns, wantCount := range tt.want {
				if got[ns] != wantCount {
					t.Errorf("namespace %q count = %d, want %d", ns, got[ns], wantCount)
				}
			}
		})
	}
}

func TestDeletePod(t *testing.T) {
	tests := []struct {
		name    string
		pod     corev1.Pod
		delNS   string
		delName string
		wantErr bool
	}{
		{
			name:    "delete existing",
			pod:     makePod("target", "default", corev1.PodRunning, nil),
			delNS:   "default",
			delName: "target",
			wantErr: false,
		},
		{
			name:    "delete non-existing",
			pod:     makePod("other", "default", corev1.PodRunning, nil),
			delNS:   "default",
			delName: "missing",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testClient(tt.pod)
			err := c.DeletePod(context.Background(), tt.delNS, tt.delName)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeletePod() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// --- test fixture builders ---

func makePod(name, ns string, phase corev1.PodPhase, resources *corev1.ResourceRequirements) corev1.Pod {
	p := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Status:     corev1.PodStatus{Phase: phase},
	}
	if resources != nil {
		p.Spec.Containers = []corev1.Container{
			{Name: "main", Resources: *resources},
		}
	}
	return p
}

func makeOOMPod(name, ns string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "app:v1"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "main",
					Image:        "app:v1",
					RestartCount: 3,
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Reason:     "OOMKilled",
							FinishedAt: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
						},
					},
				},
			},
		},
	}
}

func makeCrashLoopPod(name, ns string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "app:v1"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "main",
					Image:        "app:v1",
					RestartCount: 5,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}
}

func makeImagePullPod(name, ns string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "bad-registry/app:v99"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "main",
					Image: "bad-registry/app:v99",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"},
					},
				},
			},
		},
	}
}

func makeHighRestartPod(name, ns string, restarts int32) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "app:v1"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "main",
					Image:        "app:v1",
					RestartCount: restarts,
				},
			},
		},
	}
}

func makeDiskPressureNode(name string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue},
			},
		},
	}
}
