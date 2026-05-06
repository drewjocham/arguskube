package pkg

import (
	"testing"

	"github.com/argues/kube-watcher/internal/argocd"
	"github.com/argues/kube-watcher/internal/k8s"
)

func TestMapK8sAppsToArgoCD(t *testing.T) {
	k8sApps := []k8s.Application{
		{
			Name:          "nginx",
			Namespace:     "default",
			SyncStatus:    "Synced",
			HealthStatus:  "Healthy",
			Replicas:      3,
			ReadyReplicas: 3,
			Image:         "nginx:1.25",
			LastSync:      "2m ago",
		},
		{
			Name:          "redis",
			Namespace:     "cache",
			SyncStatus:    "OutOfSync",
			HealthStatus:  "Degraded",
			Replicas:      2,
			ReadyReplicas: 0,
		},
	}

	apps := mapK8sAppsToArgoCD(k8sApps)

	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}

	tests := []struct {
		got  argocd.App
		want k8s.Application
	}{
		{apps[0], k8sApps[0]},
		{apps[1], k8sApps[1]},
	}

	for i, tt := range tests {
		if tt.got.Name != tt.want.Name {
			t.Errorf("apps[%d].Name = %q, want %q", i, tt.got.Name, tt.want.Name)
		}
		if tt.got.Namespace != tt.want.Namespace {
			t.Errorf("apps[%d].Namespace = %q, want %q", i, tt.got.Namespace, tt.want.Namespace)
		}
		if tt.got.SyncStatus != tt.want.SyncStatus {
			t.Errorf("apps[%d].SyncStatus = %q, want %q", i, tt.got.SyncStatus, tt.want.SyncStatus)
		}
		if tt.got.HealthStatus != tt.want.HealthStatus {
			t.Errorf("apps[%d].HealthStatus = %q, want %q", i, tt.got.HealthStatus, tt.want.HealthStatus)
		}
		if tt.got.Replicas != tt.want.Replicas {
			t.Errorf("apps[%d].Replicas = %d, want %d", i, tt.got.Replicas, tt.want.Replicas)
		}
	}

	// Check defaults for K8s fallback apps.
	if apps[0].Project != "default" {
		t.Errorf("expected Project 'default', got %q", apps[0].Project)
	}
	if apps[0].DestServer != "https://kubernetes.default.svc" {
		t.Errorf("expected DestServer 'https://kubernetes.default.svc', got %q", apps[0].DestServer)
	}
	if apps[0].DestNamespace != "default" {
		t.Errorf("expected DestNamespace 'default', got %q", apps[0].DestNamespace)
	}
}

func TestGetArgusCDStatus_NoArgoCD(t *testing.T) {
	a := &App{}
	status := a.GetArgusCDStatus()
	if status.Connected {
		t.Error("expected not connected when no argocd client")
	}
	if status.Message == "" {
		t.Error("expected non-empty message when no argocd client")
	}
}

func TestListArgusCDApps_NoCluster(t *testing.T) {
	a := &App{argoCD: nil, k8s: nil}
	_, err := a.ListArgusCDApps("")
	if err == nil {
		t.Error("expected error when no cluster and no argocd")
	}
}
