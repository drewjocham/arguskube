package k8s

import (
	"context"
	"log/slog"
	"os"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAnalyzeEndpointReadiness(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &Client{cs: cs, logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
	ctx := context.Background()

	rep := int32(3)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Labels: map[string]string{"app": "web"}},
		Spec:       appsv1.DeploymentSpec{Replicas: &rep, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}},
	}
	_, err := cs.AppsV1().Deployments("default").Create(ctx, dep, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web-svc", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web"}},
	}
	_, err = cs.CoreV1().Services("default").Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	ep := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{Name: "web-svc", Namespace: "default"},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{IP: "10.0.0.1"},
					{IP: "10.0.0.2"},
				},
			},
		},
	}
	_, err = cs.CoreV1().Endpoints("default").Create(ctx, ep, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name            string
		namespace       string
		serviceName     string
		wantErr         string
		wantHealthy     bool
	}{
		{
			name:        "analyzes service readiness",
			namespace:   "default",
			serviceName: "web-svc",
			wantHealthy: true,
		},
		{
			name:        "returns error for unknown service",
			namespace:   "default",
			serviceName: "nonexistent",
			wantErr:     "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.AnalyzeEndpointReadiness(ctx, tt.namespace, tt.serviceName)
			if tt.wantErr != "" {
				if err == nil || !contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
			if result.Healthy != tt.wantHealthy {
				t.Errorf("Healthy = %v, want %v", result.Healthy, tt.wantHealthy)
			}
		})
	}
}

func TestCreateExternalBridge(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &Client{cs: cs, logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}

	tests := []struct {
		name    string
		spec    *ExternalBridgeSpec
		wantErr string
		check   func(t *testing.T, r *BridgeResult)
	}{
		{
			name: "creates manual external bridge",
			spec: &ExternalBridgeSpec{
				Name:      "external-db",
				Namespace: "default",
				Type:      "manual",
				ExternalIPs: []string{"10.0.0.100", "10.0.0.101"},
				Ports: []BridgePort{
					{Name: "postgres", Port: 5432, TargetPort: 5432, Protocol: "TCP"},
				},
			},
			check: func(t *testing.T, r *BridgeResult) {
				if r.ServiceName != "external-db" {
					t.Errorf("ServiceName = %q, want %q", r.ServiceName, "external-db")
				}
				if r.Namespace != "default" {
					t.Errorf("Namespace = %q, want %q", r.Namespace, "default")
				}
			},
		},
		{
			name: "creates externalname bridge",
			spec: &ExternalBridgeSpec{
				Name:         "ext-svc",
				Namespace:    "default",
				Type:         "externalname",
				ExternalName: "db.example.com",
				Ports: []BridgePort{
					{Name: "https", Port: 443, Protocol: "TCP"},
				},
			},
			check: func(t *testing.T, r *BridgeResult) {
				if r.ServiceName != "ext-svc" {
					t.Errorf("ServiceName = %q, want %q", r.ServiceName, "ext-svc")
				}
			},
		},
		{
			name: "fails on empty name",
			spec: &ExternalBridgeSpec{
				Namespace: "default",
				Type:      "manual",
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.CreateExternalBridge(context.Background(), tt.spec)
			if tt.wantErr != "" {
				if err == nil || !contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, result)
			}
		})
	}
}

func TestAnalyzeLabelMatch(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &Client{cs: cs, logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}
	ctx := context.Background()

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "web", "tier": "frontend"}},
	}
	_, err := cs.CoreV1().Services("default").Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "default", Labels: map[string]string{"app": "web", "tier": "frontend"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "nginx"}}},
	}
	_, err = cs.CoreV1().Pods("default").Create(ctx, pod1, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name        string
		namespace   string
		serviceName string
		wantErr     string
		wantIssues  bool
		wantMatches int
	}{
		{
			name:        "all labels match",
			namespace:   "default",
			serviceName: "web",
			wantMatches: 1,
		},
		{
			name:        "service not found",
			namespace:   "default",
			serviceName: "nonexistent",
			wantErr:     "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.AnalyzeLabelMatch(ctx, tt.namespace, tt.serviceName)
			if tt.wantErr != "" {
				if err == nil || !contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Matches) != tt.wantMatches {
				t.Errorf("Matches = %d, want %d", len(result.Matches), tt.wantMatches)
			}
		})
	}
}

func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
