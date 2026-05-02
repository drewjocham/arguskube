package k8s

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argues/kube-watcher/internal/alerts"
)

func TestFmtAge(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"zero", time.Time{}, "—"},
		{"30s ago", now.Add(-30 * time.Second), "30s"},
		{"5m ago", now.Add(-5 * time.Minute), "5m"},
		{"2h ago", now.Add(-2 * time.Hour), "2h"},
		{"2h30m ago", now.Add(-2*time.Hour - 30*time.Minute), "2h30m"},
		{"3d ago", now.Add(-3 * 24 * time.Hour), "3d"},
		{"3d6h ago", now.Add(-3*24*time.Hour - 6*time.Hour), "3d6h"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmtAge(tt.t)
			if got != tt.want {
				t.Errorf("fmtAge() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOrDash(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", "—"},
		{"non-empty", "hello", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orDash(tt.input); got != tt.want {
				t.Errorf("orDash(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFmtMapSlice(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want string
	}{
		{"nil", nil, ""},
		{"empty", map[string]string{}, ""},
		{"single", map[string]string{"app": "web"}, "app=web"},
		{"sorted", map[string]string{"b": "2", "a": "1"}, "a=1, b=2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fmtMapSlice(tt.m); got != tt.want {
				t.Errorf("fmtMapSlice() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFmtServicePorts(t *testing.T) {
	tests := []struct {
		name  string
		ports []corev1.ServicePort
		want  string
	}{
		{"nil", nil, ""},
		{"single", []corev1.ServicePort{
			{Port: 80, Protocol: corev1.ProtocolTCP},
		}, "80/TCP"},
		{"named", []corev1.ServicePort{
			{Name: "http", Port: 80, Protocol: corev1.ProtocolTCP},
			{Name: "https", Port: 443, Protocol: corev1.ProtocolTCP},
		}, "http:80/TCP, https:443/TCP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fmtServicePorts(tt.ports); got != tt.want {
				t.Errorf("fmtServicePorts() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFmtAccessModes(t *testing.T) {
	tests := []struct {
		name  string
		modes []corev1.PersistentVolumeAccessMode
		want  string
	}{
		{"empty", nil, ""},
		{"RWO", []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, "RWO"},
		{"mixed", []corev1.PersistentVolumeAccessMode{
			corev1.ReadWriteOnce, corev1.ReadOnlyMany,
		}, "RWO, ROX"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fmtAccessModes(tt.modes); got != tt.want {
				t.Errorf("fmtAccessModes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name string
		b    int64
		want string
	}{
		{"bytes", 512, "512B"},
		{"megabytes", 256 * 1024 * 1024, "256Mi"},
		{"gigabytes", int64(2.5 * 1024 * 1024 * 1024), "2.5Gi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatBytes(tt.b); got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.b, got, tt.want)
			}
		})
	}
}

func TestPodStatus(t *testing.T) {
	tests := []struct {
		name      string
		pod       *corev1.Pod
		wantState string
		wantColor string
	}{
		{
			name: "running",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
			wantState: "Running",
			wantColor: "green",
		},
		{
			name: "crash loop",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
						}},
					},
				},
			},
			wantState: "CrashLoopBackOff",
			wantColor: "red",
		},
		{
			name: "OOMKilled",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled"},
						}},
					},
				},
			},
			wantState: "OOMKilled",
			wantColor: "red",
		},
		{
			name: "pending",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{Phase: corev1.PodPending},
			},
			wantState: "Pending",
			wantColor: "amber",
		},
		{
			name: "image pull backoff",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"},
						}},
					},
				},
			},
			wantState: "ImagePullBackOff",
			wantColor: "red",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, color := podStatus(tt.pod)
			if state != tt.wantState {
				t.Errorf("podStatus() state = %q, want %q", state, tt.wantState)
			}
			if color != tt.wantColor {
				t.Errorf("podStatus() color = %q, want %q", color, tt.wantColor)
			}
		})
	}
}

func int32Ptr(i int32) *int32 { return &i }

func TestDeploymentStatus(t *testing.T) {
	tests := []struct {
		name string
		dep  *appsv1.Deployment
		want string
	}{
		{
			name: "all ready",
			dep: &appsv1.Deployment{
				Spec:   appsv1.DeploymentSpec{Replicas: int32Ptr(3)},
				Status: appsv1.DeploymentStatus{ReadyReplicas: 3},
			},
			want: "Running",
		},
		{
			name: "updating",
			dep: &appsv1.Deployment{
				Spec:   appsv1.DeploymentSpec{Replicas: int32Ptr(3)},
				Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
			},
			want: "Updating",
		},
		{
			name: "progressing failed",
			dep: &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse},
					},
				},
			},
			want: "Progressing",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deploymentStatus(tt.dep); got != tt.want {
				t.Errorf("deploymentStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeStatus(t *testing.T) {
	tests := []struct {
		name      string
		node      *corev1.Node
		wantState string
		wantColor string
	}{
		{
			name: "ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			},
			wantState: "Ready",
			wantColor: "green",
		},
		{
			name: "not ready",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					},
				},
			},
			wantState: "NotReady",
			wantColor: "red",
		},
		{
			name:      "no conditions",
			node:      &corev1.Node{},
			wantState: "Unknown",
			wantColor: "gray",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, color := nodeStatus(tt.node)
			if state != tt.wantState {
				t.Errorf("nodeStatus() state = %q, want %q", state, tt.wantState)
			}
			if color != tt.wantColor {
				t.Errorf("nodeStatus() color = %q, want %q", color, tt.wantColor)
			}
		})
	}
}

func TestNodeRoles(t *testing.T) {
	tests := []struct {
		name string
		node *corev1.Node
		want string
	}{
		{
			name: "control-plane",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			},
			want: "control-plane",
		},
		{
			name: "no roles",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeRoles(tt.node); got != tt.want {
				t.Errorf("nodeRoles() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractImages(t *testing.T) {
	tests := []struct {
		name       string
		containers []corev1.Container
		want       string
	}{
		{"empty", nil, ""},
		{
			"single",
			[]corev1.Container{{Image: "docker.io/library/nginx:1.25"}},
			"nginx:1.25",
		},
		{
			"multiple",
			[]corev1.Container{
				{Image: "ghcr.io/org/app:v2"},
				{Image: "redis:7"},
			},
			"app:v2, redis:7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractImages(tt.containers); got != tt.want {
				t.Errorf("extractImages() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSevOrder(t *testing.T) {
	tests := []struct {
		name string
		sev  alerts.Severity
		want int
	}{
		{"critical", alerts.SeverityCritical, 0},
		{"warning", alerts.SeverityWarning, 1},
		{"info", alerts.Severity("info"), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sevOrder(tt.sev); got != tt.want {
				t.Errorf("sevOrder(%q) = %d, want %d", tt.sev, got, tt.want)
			}
		})
	}
}
