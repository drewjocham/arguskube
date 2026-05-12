package kube

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/argues/argus/pkg/audit"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// ---------------------------------------------------------------------------
// mapNode tests
// ---------------------------------------------------------------------------

func TestMapNode(t *testing.T) {
	now := metav1.Now()
	hourAgo := metav1.NewTime(time.Now().Add(-1 * time.Hour))

	tests := []struct {
		name string
		node *corev1.Node
		want NodeInfo
	}{
		{
			name: "ready node with allocatable resources",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node-ready",
					CreationTimestamp: hourAgo,
					Labels:            map[string]string{"kubernetes.io/role": "worker"},
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
					},
					Allocatable: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("4"),
						corev1.ResourceMemory: resource.MustParse("16Gi"),
					},
				},
				Spec: corev1.NodeSpec{
					Taints: []corev1.Taint{{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}},
				},
			},
			want: NodeInfo{
				Name:   "node-ready",
				Status: "Ready",
				Labels: map[string]string{"kubernetes.io/role": "worker"},
				Conditions: []NodeCondition{
					{Type: "Ready", Status: "True"},
					{Type: "DiskPressure", Status: "False"},
				},
				Allocatable: map[string]string{"cpu": "4", "memory": "16Gi"},
				Taints:      []corev1.Taint{{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule}},
			},
		},
		{
			name: "not ready node with no conditions",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node-notready",
					CreationTimestamp: now,
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{},
				},
			},
			want: NodeInfo{
				Name:       "node-notready",
				Status:     "NotReady",
				Conditions: []NodeCondition{},
			},
		},
		{
			name: "node with multiple pressure conditions but not ready",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node-pressured",
					CreationTimestamp: now,
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue},
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue},
					},
				},
			},
			want: NodeInfo{
				Name:   "node-pressured",
				Status: "NotReady",
				Conditions: []NodeCondition{
					{Type: "Ready", Status: "False"},
					{Type: "DiskPressure", Status: "True"},
					{Type: "MemoryPressure", Status: "True"},
				},
			},
		},
		{
			name: "node with unknown condition status",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node-unknown",
					CreationTimestamp: now,
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionUnknown},
					},
				},
			},
			want: NodeInfo{
				Name:   "node-unknown",
				Status: "NotReady",
				Conditions: []NodeCondition{
					{Type: "Ready", Status: "Unknown"},
				},
			},
		},
		{
			name: "node with nil labels and taints",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "node-minimal",
					CreationTimestamp: now,
				},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			},
			want: NodeInfo{
				Name:       "node-minimal",
				Status:     "Ready",
				Age:        time.Since(now.Time),
				Conditions: []NodeCondition{{Type: "Ready", Status: "True"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapNode(tt.node)
			if got.Name != tt.want.Name {
				t.Errorf("mapNode().Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Status != tt.want.Status {
				t.Errorf("mapNode(%q).Status = %q, want %q", tt.node.Name, got.Status, tt.want.Status)
			}
			if len(got.Conditions) != len(tt.want.Conditions) {
				t.Errorf("mapNode(%q): got %d conditions, want %d", tt.node.Name, len(got.Conditions), len(tt.want.Conditions))
			}
			for i := range got.Conditions {
				if i < len(tt.want.Conditions) {
					if got.Conditions[i].Type != tt.want.Conditions[i].Type {
						t.Errorf("mapNode(%q).Conditions[%d].Type = %q, want %q", tt.node.Name, i, got.Conditions[i].Type, tt.want.Conditions[i].Type)
					}
					if got.Conditions[i].Status != tt.want.Conditions[i].Status {
						t.Errorf("mapNode(%q).Conditions[%d].Status = %q, want %q", tt.node.Name, i, got.Conditions[i].Status, tt.want.Conditions[i].Status)
					}
				}
			}
			if tt.want.Labels != nil {
				for k, v := range tt.want.Labels {
					if got.Labels[k] != v {
						t.Errorf("mapNode(%q).Labels[%q] = %q, want %q", tt.node.Name, k, got.Labels[k], v)
					}
				}
			}
			if tt.want.Allocatable != nil {
				for k, v := range tt.want.Allocatable {
					if got.Allocatable[k] != v {
						t.Errorf("mapNode(%q).Allocatable[%q] = %q, want %q", tt.node.Name, k, got.Allocatable[k], v)
					}
				}
			}
			if len(tt.want.Taints) != len(got.Taints) {
				t.Errorf("mapNode(%q): got %d taints, want %d", tt.node.Name, len(got.Taints), len(tt.want.Taints))
			}
			// Check age is roughly correct (within 1 second tolerance)
			if tt.want.Age != 0 {
				diff := got.Age - tt.want.Age
				if diff < -time.Second || diff > time.Second {
					t.Errorf("mapNode(%q).Age = %v, want ~%v", tt.node.Name, got.Age, tt.want.Age)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// mapPod tests
// ---------------------------------------------------------------------------

func TestMapPod(t *testing.T) {
	now := metav1.Now()
	hourAgo := metav1.NewTime(time.Now().Add(-2 * time.Hour))

	tests := []struct {
		name string
		pod  *corev1.Pod
		want PodInfo
	}{
		{
			name: "running pod with single ready container",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pod-running",
					Namespace:         "default",
					CreationTimestamp: hourAgo,
					Labels:            map[string]string{"app": "nginx"},
				},
				Spec: corev1.PodSpec{
					NodeName: "node-1",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:    "nginx",
							Image:   "nginx:1.25",
							Ready:   true,
							RestartCount: 0,
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
							},
						},
					},
				},
			},
			want: PodInfo{
				Name:         "pod-running",
				Namespace:    "default",
				Phase:        "Running",
				Status:       "Running",
				NodeName:     "node-1",
				RestartCount: 0,
				Labels:       map[string]string{"app": "nginx"},
				Containers: []ContainerInfo{
					{Name: "nginx", Image: "nginx:1.25", Ready: true, RestartCount: 0, State: "running"},
				},
			},
		},
		{
			name: "pending pod with waiting container (ContainerCreating)",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pod-pending",
					Namespace:         "staging",
					CreationTimestamp: now,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:    "app",
							Image:   "app:v1",
							Ready:   false,
							RestartCount: 0,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "ContainerCreating",
								},
							},
						},
					},
				},
			},
			want: PodInfo{
				Name:         "pod-pending",
				Namespace:    "staging",
				Phase:        "Pending",
				Status:       "ContainerCreating",
				RestartCount: 0,
				Containers: []ContainerInfo{
					{Name: "app", Image: "app:v1", Ready: false, RestartCount: 0, State: "ContainerCreating"},
				},
			},
		},
		{
			name: "pod with terminated container (Completed)",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pod-completed",
					Namespace:         "jobs",
					CreationTimestamp: hourAgo,
				},
				Spec: corev1.PodSpec{
					NodeName: "node-2",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:    "job",
							Image:   "busybox",
							Ready:   false,
							RestartCount: 1,
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "Completed",
									ExitCode: 0,
								},
							},
						},
					},
				},
			},
			want: PodInfo{
				Name:         "pod-completed",
				Namespace:    "jobs",
				Phase:        "Succeeded",
				Status:       "Succeeded",
				NodeName:     "node-2",
				RestartCount: 1,
				Containers: []ContainerInfo{
					{Name: "job", Image: "busybox", Ready: false, RestartCount: 1, State: "Completed"},
				},
			},
		},
		{
			name: "pod with error status (CrashLoopBackOff)",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pod-crashlooping",
					Namespace:         "default",
					CreationTimestamp: now,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:    "crasher",
							Image:   "crasher:latest",
							Ready:   false,
							RestartCount: 5,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
			want: PodInfo{
				Name:         "pod-crashlooping",
				Namespace:    "default",
				Phase:        "Running",
				Status:       "CrashLoopBackOff",
				RestartCount: 5,
				Containers: []ContainerInfo{
					{Name: "crasher", Image: "crasher:latest", Ready: false, RestartCount: 5, State: "CrashLoopBackOff"},
				},
			},
		},
		{
			name: "pod with multiple containers (mixed states)",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pod-multi",
					Namespace:         "multi",
					CreationTimestamp: now,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:    "sidecar",
							Image:   "sidecar:1.0",
							Ready:   true,
							RestartCount: 1,
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()},
							},
						},
						{
							Name:    "main",
							Image:   "main:2.0",
							Ready:   false,
							RestartCount: 3,
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "CrashLoopBackOff",
								},
							},
						},
					},
				},
			},
			want: PodInfo{
				Name:         "pod-multi",
				Namespace:    "multi",
				Phase:        "Running",
				Status:       "CrashLoopBackOff",
				RestartCount: 4,
				Containers: []ContainerInfo{
					{Name: "sidecar", Image: "sidecar:1.0", Ready: true, RestartCount: 1, State: "running"},
					{Name: "main", Image: "main:2.0", Ready: false, RestartCount: 3, State: "CrashLoopBackOff"},
				},
			},
		},
		{
			name: "pod with no container statuses",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pod-empty",
					Namespace:         "empty",
					CreationTimestamp: now,
				},
				Status: corev1.PodStatus{
					Phase:              corev1.PodPending,
					ContainerStatuses:  []corev1.ContainerStatus{},
				},
			},
			want: PodInfo{
				Name:         "pod-empty",
				Namespace:    "empty",
				Phase:        "Pending",
				Status:       "Pending",
				RestartCount: 0,
				Containers:   []ContainerInfo{},
			},
		},
		{
			name: "pod with unknown container state",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "pod-unknown-state",
					Namespace:         "default",
					CreationTimestamp: now,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name:    "mystery",
							Image:   "mystery:latest",
							Ready:   false,
							RestartCount: 0,
							State:   corev1.ContainerState{}, // no running, waiting, or terminated
						},
					},
				},
			},
			want: PodInfo{
				Name:         "pod-unknown-state",
				Namespace:    "default",
				Phase:        "Running",
				Status:       "Running",
				RestartCount: 0,
				Containers: []ContainerInfo{
					{Name: "mystery", Image: "mystery:latest", Ready: false, RestartCount: 0, State: "unknown"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapPod(tt.pod)
			if got.Name != tt.want.Name {
				t.Errorf("mapPod().Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Namespace != tt.want.Namespace {
				t.Errorf("mapPod(%q).Namespace = %q, want %q", tt.pod.Name, got.Namespace, tt.want.Namespace)
			}
			if got.Phase != tt.want.Phase {
				t.Errorf("mapPod(%q).Phase = %q, want %q", tt.pod.Name, got.Phase, tt.want.Phase)
			}
			if got.Status != tt.want.Status {
				t.Errorf("mapPod(%q).Status = %q, want %q", tt.pod.Name, got.Status, tt.want.Status)
			}
			if got.NodeName != tt.want.NodeName {
				t.Errorf("mapPod(%q).NodeName = %q, want %q", tt.pod.Name, got.NodeName, tt.want.NodeName)
			}
			if got.RestartCount != tt.want.RestartCount {
				t.Errorf("mapPod(%q).RestartCount = %d, want %d", tt.pod.Name, got.RestartCount, tt.want.RestartCount)
			}
			if len(got.Containers) != len(tt.want.Containers) {
				t.Errorf("mapPod(%q): got %d containers, want %d", tt.pod.Name, len(got.Containers), len(tt.want.Containers))
			} else {
				for i := range got.Containers {
					if i < len(tt.want.Containers) {
						if got.Containers[i].Name != tt.want.Containers[i].Name {
							t.Errorf("mapPod(%q).Containers[%d].Name = %q, want %q", tt.pod.Name, i, got.Containers[i].Name, tt.want.Containers[i].Name)
						}
						if got.Containers[i].Image != tt.want.Containers[i].Image {
							t.Errorf("mapPod(%q).Containers[%d].Image = %q, want %q", tt.pod.Name, i, got.Containers[i].Image, tt.want.Containers[i].Image)
						}
						if got.Containers[i].Ready != tt.want.Containers[i].Ready {
							t.Errorf("mapPod(%q).Containers[%d].Ready = %v, want %v", tt.pod.Name, i, got.Containers[i].Ready, tt.want.Containers[i].Ready)
						}
						if got.Containers[i].RestartCount != tt.want.Containers[i].RestartCount {
							t.Errorf("mapPod(%q).Containers[%d].RestartCount = %d, want %d", tt.pod.Name, i, got.Containers[i].RestartCount, tt.want.Containers[i].RestartCount)
						}
						if got.Containers[i].State != tt.want.Containers[i].State {
							t.Errorf("mapPod(%q).Containers[%d].State = %q, want %q", tt.pod.Name, i, got.Containers[i].State, tt.want.Containers[i].State)
						}
					}
				}
			}
			if tt.want.Labels != nil {
				for k, v := range tt.want.Labels {
					if got.Labels[k] != v {
						t.Errorf("mapPod(%q).Labels[%q] = %q, want %q", tt.pod.Name, k, got.Labels[k], v)
					}
				}
			}
			if tt.want.Age != 0 {
				diff := got.Age - tt.want.Age
				if diff < -time.Second || diff > time.Second {
					t.Errorf("mapPod(%q).Age = %v, want ~%v", tt.pod.Name, got.Age, tt.want.Age)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewClient tests
// ---------------------------------------------------------------------------

func TestNewClient(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		err  bool
	}{
		{
			name: "fails without valid kubeconfig",
			env:  map[string]string{"KUBECONFIG": "/nonexistent/kubeconfig"},
			err:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env and defer cleanup
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
			_, err := NewClient(logger)

			if tt.err && err == nil {
				t.Error("NewClient() expected error, got nil")
			}
			if !tt.err && err != nil {
				t.Errorf("NewClient() unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AuditOptionsFromEnv tests
// ---------------------------------------------------------------------------

func TestAuditOptionsFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		envVal  string
		wantLen int
	}{
		{
			name:    "audit enabled when ARGUS_AUDIT=true",
			envVal:  "true",
			wantLen: 1,
		},
		{
			name:    "audit disabled when ARGUS_AUDIT=false",
			envVal:  "false",
			wantLen: 0,
		},
		{
			name:    "audit disabled when ARGUS_AUDIT=1",
			envVal:  "1",
			wantLen: 0,
		},
		{
			name:    "audit disabled when env is empty",
			envVal:  "",
			wantLen: 0,
		},
		{
			name:    "audit disabled when env is not set",
			envVal:  "unset",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal == "unset" {
				os.Unsetenv("ARGUS_AUDIT")
			} else {
				t.Setenv("ARGUS_AUDIT", tt.envVal)
			}

			opts := AuditOptionsFromEnv()
			if len(opts) != tt.wantLen {
				t.Errorf("AuditOptionsFromEnv() returned %d options, want %d", len(opts), tt.wantLen)
			}

			// Verify the option actually enables audit when expected
			if tt.wantLen > 0 {
				cfg := &auditConfig{}
				for _, o := range opts {
					o(cfg)
				}
				if !cfg.enabled {
					t.Error("AuditOptionsFromEnv() option did not enable audit config")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NewAuditClient tests
// ---------------------------------------------------------------------------

func TestNewAuditClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name       string
		opts       []AuditOption
		wantWrapped bool // true if we expect an auditClient wrapper
	}{
		{
			name:       "returns base client when audit disabled",
			opts:       nil,
			wantWrapped: false,
		},
		{
			name:       "returns base client when audit not enabled in opts",
			opts:       []AuditOption{func(c *auditConfig) { c.enabled = false }},
			wantWrapped: false,
		},
		{
			name: "returns audit wrapper when audit enabled",
			opts: []AuditOption{func(c *auditConfig) { c.enabled = true }},
			wantWrapped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := &Client{cs: fake.NewSimpleClientset(), logger: logger}
			auditLogger := audit.NewSlogLogger(logger)

			got := NewAuditClient(base, auditLogger, logger, tt.opts...)

			_, isWrapped := got.(*auditClient)
			if tt.wantWrapped && !isWrapped {
				t.Error("NewAuditClient() returned a non-wrapped client, expected *auditClient")
			}
			if !tt.wantWrapped && isWrapped {
				t.Error("NewAuditClient() returned *auditClient, expected unwrapped client")
			}

			// Ensure we can still call methods on the result regardless of wrapping
			ci, err := got.GetClusterInfo(context.Background())
			if err != nil {
				t.Errorf("GetClusterInfo() on result returned error: %v", err)
			}
			if ci != nil {
				t.Logf("Cluster info returned: %+v", ci)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client HealthCheck tests with fake clientset
// ---------------------------------------------------------------------------

func TestClient_HealthCheck(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name    string
		setup   func() *fake.Clientset
		wantErr bool
	}{
		{
			name: "health check succeeds with valid discovery client",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset()
				// The fake discovery client returns a default version by default, so this should succeed
				return cs
			},
			wantErr: false,
		},
		{
			name: "health check fails when discovery returns error",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset()
				cs.Discovery().(*discoveryfake.FakeDiscovery).PrependReactor("get", "version", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("discovery unavailable")
				})
				return cs
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			err := c.HealthCheck(ctx)

			if tt.wantErr && err == nil {
				t.Error("HealthCheck() expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("HealthCheck() unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetClusterInfo tests with fake clientset
// ---------------------------------------------------------------------------

func TestClient_GetClusterInfo(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name    string
		setup   func() *fake.Clientset
		want    *ClusterInfo
		wantErr bool
	}{
		{
			name: "returns cluster info with nodes",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset(
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-3"}},
				)
				return cs
			},
			want:    &ClusterInfo{Version: "v0.0.0-master+$Format:%H$", NodeCount: 3},
			wantErr: false,
		},
		{
			name: "returns cluster info with zero nodes",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			want:    &ClusterInfo{Version: "v0.0.0-master+$Format:%H$", NodeCount: 0},
			wantErr: false,
		},
		{
			name: "errors when discovery fails",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset()
				cs.Discovery().(*discoveryfake.FakeDiscovery).PrependReactor("get", "version", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("server unavailable")
				})
				return cs
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "errors when node list fails",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset()
				cs.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("node listing denied")
				})
				return cs
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetClusterInfo(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("GetClusterInfo() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetClusterInfo() unexpected error: %v", err)
			}
			if got.Version != tt.want.Version {
				t.Errorf("GetClusterInfo().Version = %q, want %q", got.Version, tt.want.Version)
			}
			if got.NodeCount != tt.want.NodeCount {
				t.Errorf("GetClusterInfo().NodeCount = %d, want %d", got.NodeCount, tt.want.NodeCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetNodes / GetNode tests with fake clientset
// ---------------------------------------------------------------------------

func TestClient_GetNodes(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			},
		},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
			},
		},
	}

	tests := []struct {
		name    string
		setup   func() *fake.Clientset
		nodes   int
		wantErr bool
	}{
		{
			name: "returns all nodes",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(node1, node2)
			},
			nodes:   2,
			wantErr: false,
		},
		{
			name: "returns empty list when no nodes",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			nodes:   0,
			wantErr: false,
		},
		{
			name: "errors on list failure",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset()
				cs.PrependReactor("list", "nodes", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("forbidden")
				})
				return cs
			},
			nodes:   0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetNodes(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("GetNodes() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetNodes() unexpected error: %v", err)
			}
			if len(got) != tt.nodes {
				t.Errorf("GetNodes() returned %d nodes, want %d", len(got), tt.nodes)
			}
		})
	}
}

func TestClient_GetNode(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name     string
		nodeName string
		setup    func() *fake.Clientset
		wantName string
		wantErr  bool
	}{
		{
			name:     "returns existing node",
			nodeName: "node-1",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
				)
			},
			wantName: "node-1",
			wantErr:  false,
		},
		{
			name:     "returns error for non-existent node",
			nodeName: "nonexistent",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			wantName: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetNode(ctx, tt.nodeName)

			if tt.wantErr {
				if err == nil {
					t.Error("GetNode() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetNode() unexpected error: %v", err)
			}
			if got.Name != tt.wantName {
				t.Errorf("GetNode().Name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetPod / GetPods tests with fake clientset
// ---------------------------------------------------------------------------

func TestClient_GetPods(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "nginx", Image: "nginx:1.25", Ready: true,
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
		},
	}
	pendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "default"},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app", Image: "app:v1", Ready: false,
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
				},
			},
		},
	}
	otherNSPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-3", Namespace: "kube-system"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "coredns", Image: "coredns:1.11", Ready: true,
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				},
			},
		},
	}

	tests := []struct {
		name      string
		namespace string
		setup     func() *fake.Clientset
		podCount  int
		wantErr   bool
	}{
		{
			name:      "returns pods in namespace",
			namespace: "default",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(runningPod, pendingPod, otherNSPod)
			},
			podCount: 2,
			wantErr:  false,
		},
		{
			name:      "returns pods in different namespace",
			namespace: "kube-system",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(runningPod, pendingPod, otherNSPod)
			},
			podCount: 1,
			wantErr:  false,
		},
		{
			name:      "returns empty list when no pods in namespace",
			namespace: "empty-ns",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(runningPod)
			},
			podCount: 0,
			wantErr:  false,
		},
		{
			name:      "returns all pods with empty namespace",
			namespace: "",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(runningPod, pendingPod, otherNSPod)
			},
			podCount: 3,
			wantErr:  false,
		},
		{
			name:      "errors on list failure",
			namespace: "default",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset(runningPod)
				cs.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("forbidden")
				})
				return cs
			},
			podCount: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetPods(ctx, tt.namespace)

			if tt.wantErr {
				if err == nil {
					t.Error("GetPods() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetPods() unexpected error: %v", err)
			}
			if len(got) != tt.podCount {
				t.Errorf("GetPods(%q) returned %d pods, want %d", tt.namespace, len(got), tt.podCount)
			}
		})
	}
}

func TestClient_GetPod(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name      string
		namespace string
		podName   string
		setup     func() *fake.Clientset
		wantName  string
		wantErr   bool
	}{
		{
			name:      "returns existing pod",
			namespace: "default",
			podName:   "my-pod",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{Name: "my-pod", Namespace: "default"},
						Status:     corev1.PodStatus{Phase: corev1.PodRunning},
					},
				)
			},
			wantName: "my-pod",
			wantErr:  false,
		},
		{
			name:      "returns error for non-existent pod",
			namespace: "default",
			podName:   "nonexistent",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			wantName: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetPod(ctx, tt.namespace, tt.podName)

			if tt.wantErr {
				if err == nil {
					t.Error("GetPod() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetPod() unexpected error: %v", err)
			}
			if got.Name != tt.wantName {
				t.Errorf("GetPod().Name = %q, want %q", got.Name, tt.wantName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetPodsAllNamespaces tests with fake clientset
// ---------------------------------------------------------------------------

func TestClient_GetPodsAllNamespaces(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name     string
		setup    func() *fake.Clientset
		podCount int
		wantErr  bool
	}{
		{
			name: "returns pods from all namespaces",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-a", Namespace: "ns1"}},
					&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-b", Namespace: "ns2"}},
				)
			},
			podCount: 2,
			wantErr:  false,
		},
		{
			name: "returns empty when no pods exist",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			podCount: 0,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetPodsAllNamespaces(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("GetPodsAllNamespaces() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetPodsAllNamespaces() unexpected error: %v", err)
			}
			if len(got) != tt.podCount {
				t.Errorf("GetPodsAllNamespaces() returned %d pods, want %d", len(got), tt.podCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetServices tests with fake clientset
// ---------------------------------------------------------------------------

func TestClient_GetServices(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name      string
		namespace string
		setup     func() *fake.Clientset
		svcCount  int
		wantErr   bool
	}{
		{
			name:      "returns services in namespace",
			namespace: "default",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "svc-1", Namespace: "default"},
						Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, ClusterIP: "10.0.0.1"},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "svc-2", Namespace: "default"},
						Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort, ClusterIP: "10.0.0.2"},
					},
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{Name: "svc-other", Namespace: "other"},
					},
				)
			},
			svcCount: 2,
			wantErr:  false,
		},
		{
			name:      "returns empty list when no services",
			namespace: "default",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			svcCount: 0,
			wantErr:  false,
		},
		{
			name:      "errors on list failure",
			namespace: "default",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset()
				cs.PrependReactor("list", "services", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("forbidden")
				})
				return cs
			},
			svcCount: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetServices(ctx, tt.namespace)

			if tt.wantErr {
				if err == nil {
					t.Error("GetServices() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetServices() unexpected error: %v", err)
			}
			if len(got) != tt.svcCount {
				t.Errorf("GetServices(%q) returned %d services, want %d", tt.namespace, len(got), tt.svcCount)
			}
			if tt.svcCount > 0 {
				if got[0].ClusterIP != "10.0.0.1" {
					t.Errorf("GetServices()[0].ClusterIP = %q, want %q", got[0].ClusterIP, "10.0.0.1")
				}
				if got[0].Type != "ClusterIP" {
					t.Errorf("GetServices()[0].Type = %q, want %q", got[0].Type, "ClusterIP")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetEvents tests with fake clientset
// ---------------------------------------------------------------------------

func TestClient_GetEvents(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	now := metav1.Now()

	tests := []struct {
		name      string
		namespace string
		setup     func() *fake.Clientset
		evtCount  int
		wantErr   bool
	}{
		{
			name:      "returns events in namespace",
			namespace: "default",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{Name: "evt-1", Namespace: "default"},
						Type:       "Warning",
						Reason:     "BackOff",
						InvolvedObject: corev1.ObjectReference{Kind: "Pod", Name: "my-pod"},
						Message:       "Back-off restarting failed container",
						LastTimestamp: now,
						Count:         3,
					},
					&corev1.Event{
						ObjectMeta: metav1.ObjectMeta{Name: "evt-other", Namespace: "other"},
					},
				)
			},
			evtCount: 1,
			wantErr:  false,
		},
		{
			name:      "returns empty when no events",
			namespace: "default",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			evtCount: 0,
			wantErr:  false,
		},
		{
			name:      "errors on list failure",
			namespace: "default",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset()
				cs.PrependReactor("list", "events", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("forbidden")
				})
				return cs
			},
			evtCount: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetEvents(ctx, tt.namespace)

			if tt.wantErr {
				if err == nil {
					t.Error("GetEvents() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetEvents() unexpected error: %v", err)
			}
			if len(got) != tt.evtCount {
				t.Errorf("GetEvents(%q) returned %d events, want %d", tt.namespace, len(got), tt.evtCount)
			}
			if tt.evtCount > 0 {
				if got[0].Type != "Warning" {
					t.Errorf("GetEvents()[0].Type = %q, want %q", got[0].Type, "Warning")
				}
				if got[0].Reason != "BackOff" {
					t.Errorf("GetEvents()[0].Reason = %q, want %q", got[0].Reason, "BackOff")
				}
				if got[0].ObjectKind != "Pod" {
					t.Errorf("GetEvents()[0].ObjectKind = %q, want %q", got[0].ObjectKind, "Pod")
				}
				if got[0].Count != 3 {
					t.Errorf("GetEvents()[0].Count = %d, want %d", got[0].Count, 3)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetNamespaces tests with fake clientset
// ---------------------------------------------------------------------------

func TestClient_GetNamespaces(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name    string
		setup   func() *fake.Clientset
		want    []string
		wantErr bool
	}{
		{
			name: "returns namespace names",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
					&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
					&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "monitoring"}},
				)
			},
			want:    []string{"default", "kube-system", "monitoring"},
			wantErr: false,
		},
		{
			name: "returns empty when no namespaces",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			want:    []string{},
			wantErr: false,
		},
		{
			name: "errors on list failure",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset()
				cs.PrependReactor("list", "namespaces", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("forbidden")
				})
				return cs
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetNamespaces(ctx)

			if tt.wantErr {
				if err == nil {
					t.Error("GetNamespaces() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetNamespaces() unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("GetNamespaces() returned %d namespaces, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GetNamespaces()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetResourceQuotas tests with fake clientset
// ---------------------------------------------------------------------------

func TestClient_GetResourceQuotas(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name      string
		namespace string
		setup     func() *fake.Clientset
		quotaLen  int
		wantErr   bool
	}{
		{
			name:      "returns resource quotas with hard and used",
			namespace: "default",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.ResourceQuota{
						ObjectMeta: metav1.ObjectMeta{Name: "quota-1", Namespace: "default"},
						Status: corev1.ResourceQuotaStatus{
							Hard: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("10"),
								corev1.ResourceMemory: resource.MustParse("20Gi"),
							},
							Used: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("5"),
								corev1.ResourceMemory: resource.MustParse("10Gi"),
							},
						},
					},
				)
			},
			quotaLen: 1,
			wantErr:  false,
		},
		{
			name:      "returns empty when no quotas",
			namespace: "default",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset()
			},
			quotaLen: 0,
			wantErr:  false,
		},
		{
			name:      "errors on list failure",
			namespace: "default",
			setup: func() *fake.Clientset {
				cs := fake.NewSimpleClientset()
				cs.PrependReactor("list", "resourcequotas", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("forbidden")
				})
				return cs
			},
			quotaLen: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetResourceQuotas(ctx, tt.namespace)

			if tt.wantErr {
				if err == nil {
					t.Error("GetResourceQuotas() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetResourceQuotas() unexpected error: %v", err)
			}
			if len(got) != tt.quotaLen {
				t.Errorf("GetResourceQuotas(%q) returned %d quotas, want %d", tt.namespace, len(got), tt.quotaLen)
			}
			if tt.quotaLen > 0 {
				if got[0].Name != "quota-1" {
					t.Errorf("GetResourceQuotas()[0].Name = %q, want %q", got[0].Name, "quota-1")
				}
				if got[0].Hard["cpu"] != "10" {
					t.Errorf("GetResourceQuotas()[0].Hard[cpu] = %q, want %q", got[0].Hard["cpu"], "10")
				}
				if got[0].Used["memory"] != "10Gi" {
					t.Errorf("GetResourceQuotas()[0].Used[memory] = %q, want %q", got[0].Used["memory"], "10Gi")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetResource (placeholder) test
// ---------------------------------------------------------------------------

func TestClient_GetResource(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name string
		kind string
	}{
		{name: "returns nil for any kind", kind: "Pod"},
		{name: "returns nil for empty kind", kind: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: fake.NewSimpleClientset(), logger: logger}
			got, err := c.GetResource(ctx, tt.kind, "some-name")
			if err != nil {
				t.Errorf("GetResource() unexpected error: %v", err)
			}
			if got != nil {
				t.Errorf("GetResource() returned %v, want nil", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Client GetRawInterface test
// ---------------------------------------------------------------------------

func TestClient_GetRawInterface(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name string
		cs   *fake.Clientset
	}{
		{
			name: "returns the same clientset instance",
			cs:   fake.NewSimpleClientset(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.cs, logger: logger}
			got := c.GetRawInterface()
			if got != tt.cs {
				t.Error("GetRawInterface() returned a different instance")
			}
			// Verify the returned interface works by calling a method
			v, err := got.Discovery().ServerVersion()
			if err != nil {
				t.Errorf("GetRawInterface().Discovery().ServerVersion() error: %v", err)
			}
			if v == nil {
				t.Error("GetRawInterface().Discovery().ServerVersion() returned nil")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetServicesAllNamespaces / GetEventsAllNamespaces integration
// ---------------------------------------------------------------------------

func TestClient_GetServicesAllNamespaces(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name    string
		setup   func() *fake.Clientset
		count   int
		wantErr bool
	}{
		{
			name: "returns services across all namespaces",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc-a", Namespace: "ns1"}},
					&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc-b", Namespace: "ns2"}},
				)
			},
			count:   2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetServicesAllNamespaces(ctx)
			if tt.wantErr {
				if err == nil {
					t.Error("GetServicesAllNamespaces() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetServicesAllNamespaces() unexpected error: %v", err)
			}
			if len(got) != tt.count {
				t.Errorf("GetServicesAllNamespaces() returned %d, want %d", len(got), tt.count)
			}
		})
	}
}

func TestClient_GetEventsAllNamespaces(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tests := []struct {
		name    string
		setup   func() *fake.Clientset
		count   int
		wantErr bool
	}{
		{
			name: "returns events across all namespaces",
			setup: func() *fake.Clientset {
				return fake.NewSimpleClientset(
					&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "evt-a", Namespace: "ns1"}},
					&corev1.Event{ObjectMeta: metav1.ObjectMeta{Name: "evt-b", Namespace: "ns2"}},
				)
			},
			count:   2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cs: tt.setup(), logger: logger}
			got, err := c.GetEventsAllNamespaces(ctx)
			if tt.wantErr {
				if err == nil {
					t.Error("GetEventsAllNamespaces() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetEventsAllNamespaces() unexpected error: %v", err)
			}
			if len(got) != tt.count {
				t.Errorf("GetEventsAllNamespaces() returned %d, want %d", len(got), tt.count)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AuditClient delegation tests
// ---------------------------------------------------------------------------

// verificationClient is a mock ClientInterface that records method calls.
type verificationClient struct {
	ClientInterface
	calls []string
}

func (v *verificationClient) GetNodes(ctx context.Context) ([]NodeInfo, error) {
	v.calls = append(v.calls, "GetNodes")
	return nil, nil
}
func (v *verificationClient) GetNode(ctx context.Context, name string) (*NodeInfo, error) {
	v.calls = append(v.calls, "GetNode")
	return nil, nil
}
func (v *verificationClient) GetPod(ctx context.Context, ns, name string) (*PodInfo, error) {
	v.calls = append(v.calls, "GetPod")
	return nil, nil
}
func (v *verificationClient) GetPods(ctx context.Context, ns string) ([]PodInfo, error) {
	v.calls = append(v.calls, "GetPods")
	return nil, nil
}
func (v *verificationClient) GetPodsAllNamespaces(ctx context.Context) ([]PodInfo, error) {
	v.calls = append(v.calls, "GetPodsAllNamespaces")
	return nil, nil
}
func (v *verificationClient) GetPodLogs(ctx context.Context, ns, pod, container string, tail, since int64, prev bool) (string, error) {
	v.calls = append(v.calls, "GetPodLogs")
	return "", nil
}
func (v *verificationClient) GetServices(ctx context.Context, ns string) ([]ServiceInfo, error) {
	v.calls = append(v.calls, "GetServices")
	return nil, nil
}
func (v *verificationClient) GetServicesAllNamespaces(ctx context.Context) ([]ServiceInfo, error) {
	v.calls = append(v.calls, "GetServicesAllNamespaces")
	return nil, nil
}
func (v *verificationClient) GetEvents(ctx context.Context, ns string) ([]EventInfo, error) {
	v.calls = append(v.calls, "GetEvents")
	return nil, nil
}
func (v *verificationClient) GetEventsAllNamespaces(ctx context.Context) ([]EventInfo, error) {
	v.calls = append(v.calls, "GetEventsAllNamespaces")
	return nil, nil
}
func (v *verificationClient) GetNamespaces(ctx context.Context) ([]string, error) {
	v.calls = append(v.calls, "GetNamespaces")
	return nil, nil
}
func (v *verificationClient) GetResourceQuotas(ctx context.Context, ns string) ([]ResourceQuotaInfo, error) {
	v.calls = append(v.calls, "GetResourceQuotas")
	return nil, nil
}
func (v *verificationClient) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	v.calls = append(v.calls, "GetClusterInfo")
	return nil, nil
}
func (v *verificationClient) GetResource(ctx context.Context, kind, name string) ([]runtime.Object, error) {
	v.calls = append(v.calls, "GetResource")
	return nil, nil
}
func (v *verificationClient) HealthCheck(ctx context.Context) error {
	v.calls = append(v.calls, "HealthCheck")
	return nil
}
func (v *verificationClient) GetRawInterface() kubernetes.Interface {
	v.calls = append(v.calls, "GetRawInterface")
	return nil
}

// Ensure verificationClient implements ClientInterface.
var _ ClientInterface = (*verificationClient)(nil)

func TestAuditClient_DelegatesToInner(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	auditLogger := audit.NewSlogLogger(logger)

	tests := []struct {
		name       string
		callFn     func(ci ClientInterface)
		wantCall   string
	}{
		{
			name: "GetNodes delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetNodes(ctx) },
			wantCall: "GetNodes",
		},
		{
			name: "GetNode delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetNode(ctx, "n1") },
			wantCall: "GetNode",
		},
		{
			name: "GetPod delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetPod(ctx, "ns", "pod") },
			wantCall: "GetPod",
		},
		{
			name: "GetPods delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetPods(ctx, "ns") },
			wantCall: "GetPods",
		},
		{
			name: "GetPodsAllNamespaces delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetPodsAllNamespaces(ctx) },
			wantCall: "GetPodsAllNamespaces",
		},
		{
			name: "GetPodLogs delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetPodLogs(ctx, "ns", "pod", "c", 0, 0, false) },
			wantCall: "GetPodLogs",
		},
		{
			name: "GetServices delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetServices(ctx, "ns") },
			wantCall: "GetServices",
		},
		{
			name: "GetServicesAllNamespaces delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetServicesAllNamespaces(ctx) },
			wantCall: "GetServicesAllNamespaces",
		},
		{
			name: "GetEvents delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetEvents(ctx, "ns") },
			wantCall: "GetEvents",
		},
		{
			name: "GetEventsAllNamespaces delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetEventsAllNamespaces(ctx) },
			wantCall: "GetEventsAllNamespaces",
		},
		{
			name: "GetNamespaces delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetNamespaces(ctx) },
			wantCall: "GetNamespaces",
		},
		{
			name: "GetResourceQuotas delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetResourceQuotas(ctx, "ns") },
			wantCall: "GetResourceQuotas",
		},
		{
			name: "GetClusterInfo delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetClusterInfo(ctx) },
			wantCall: "GetClusterInfo",
		},
		{
			name: "GetResource delegates to inner",
			callFn: func(ci ClientInterface) { _, _ = ci.GetResource(ctx, "Pod", "name") },
			wantCall: "GetResource",
		},
		{
			name: "HealthCheck delegates to inner",
			callFn: func(ci ClientInterface) { _ = ci.HealthCheck(ctx) },
			wantCall: "HealthCheck",
		},
		{
			name: "GetRawInterface delegates to inner",
			callFn: func(ci ClientInterface) { ci.GetRawInterface() },
			wantCall: "GetRawInterface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := &verificationClient{}
			ac := &auditClient{inner: inner, audit: auditLogger, logger: logger}

			tt.callFn(ac)

			if len(inner.calls) != 1 {
				t.Fatalf("expected 1 call to inner, got %d: %v", len(inner.calls), inner.calls)
			}
			if inner.calls[0] != tt.wantCall {
				t.Errorf("expected call %q, got %q", tt.wantCall, inner.calls[0])
			}
		})
	}
}


