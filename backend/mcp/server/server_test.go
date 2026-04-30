package server

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kube-watcher/mcp/monitoring/history"
	"kube-watcher/pkg/kube"
	kwatch "kube-watcher/pkg/kube/watch"

	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
)

// --- mock kube client ---

type stubK8sClient struct {
	healthErr error
}

func (s *stubK8sClient) GetNodes(_ context.Context) ([]kube.NodeInfo, error)         { return nil, nil }
func (s *stubK8sClient) GetNode(_ context.Context, _ string) (*kube.NodeInfo, error) { return nil, nil }
func (s *stubK8sClient) GetPods(_ context.Context, _ string) ([]kube.PodInfo, error) { return nil, nil }
func (s *stubK8sClient) GetPodsAllNamespaces(_ context.Context) ([]kube.PodInfo, error) {
	return nil, nil
}
func (s *stubK8sClient) GetPod(_ context.Context, _, _ string) (*kube.PodInfo, error) {
	return nil, nil
}
func (s *stubK8sClient) GetPodLogs(_ context.Context, _, _, _ string, _, _ int64, _ bool) (string, error) {
	return "", nil
}
func (s *stubK8sClient) GetServices(_ context.Context, _ string) ([]kube.ServiceInfo, error) {
	return nil, nil
}
func (s *stubK8sClient) GetServicesAllNamespaces(_ context.Context) ([]kube.ServiceInfo, error) {
	return nil, nil
}
func (s *stubK8sClient) GetEvents(_ context.Context, _ string) ([]kube.EventInfo, error) {
	return nil, nil
}
func (s *stubK8sClient) GetEventsAllNamespaces(_ context.Context) ([]kube.EventInfo, error) {
	return nil, nil
}
func (s *stubK8sClient) GetNamespaces(_ context.Context) ([]string, error) { return nil, nil }
func (s *stubK8sClient) GetResourceQuotas(_ context.Context, _ string) ([]kube.ResourceQuotaInfo, error) {
	return nil, nil
}
func (s *stubK8sClient) GetClusterInfo(_ context.Context) (*kube.ClusterInfo, error) {
	return &kube.ClusterInfo{Version: "v1.28"}, nil
}
func (s *stubK8sClient) GetResource(_ context.Context, _, _ string) ([]runtime.Object, error) {
	return nil, nil
}
func (s *stubK8sClient) HealthCheck(_ context.Context) error  { return s.healthErr }
func (s *stubK8sClient) GetRawInterface() clientset.Interface { return nil }

func newTestServer(t *testing.T) (*MCPServer, *history.Store) {
	t.Helper()
	logger := slog.Default()
	store, err := history.NewStore(t.TempDir())
	require.NoError(t, err)

	wm := kwatch.NewManager(&stubK8sClient{}, logger, 30*time.Second)

	srv, err := NewMCPServer(logger, Config{
		Version:      "test",
		GitCommit:    "abc",
		BuildDate:    "now",
		K8sClient:    &stubK8sClient{},
		HistoryStore: store,
		Watcher:      wm,
	})
	require.NoError(t, err)
	return srv, store
}

func TestNewMCPServer(t *testing.T) {
	tests := []struct {
		name    string
		logger  *slog.Logger
		wantErr error
	}{
		{"ValidLogger", slog.Default(), nil},
		{"NilLogger", nil, ErrLoggerRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := history.NewStore(t.TempDir())
			require.NoError(t, err)
			defer func() { _ = store.Close() }()

			wm := kwatch.NewManager(&stubK8sClient{}, slog.Default(), 30*time.Second)

			_, err = NewMCPServer(tt.logger, Config{
				Version:      "test",
				K8sClient:    &stubK8sClient{},
				HistoryStore: store,
				Watcher:      wm,
			})
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMCPServer_ExecuteTool(t *testing.T) {
	srv, store := newTestServer(t)
	defer func() { _ = store.Close() }()

	tests := []struct {
		name    string
		tool    string
		wantErr error
	}{
		{"ExistingTool", "get_server_version", nil},
		{"UnknownTool", "nonexistent", ErrToolNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := srv.ExecuteTool(context.Background(), tt.tool, nil)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMCPServer_HealthCheck(t *testing.T) {
	tests := []struct {
		name       string
		healthErr  error
		wantStatus string
	}{
		{"Healthy", nil, "healthy"},
		{"Degraded", errors.New("unreachable"), "degraded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.Default()
			store, err := history.NewStore(t.TempDir())
			require.NoError(t, err)
			defer func() { _ = store.Close() }()

			client := &stubK8sClient{healthErr: tt.healthErr}
			wm := kwatch.NewManager(client, logger, 30*time.Second)

			srv, err := NewMCPServer(logger, Config{
				Version:      "test",
				K8sClient:    client,
				HistoryStore: store,
				Watcher:      wm,
			})
			require.NoError(t, err)

			result := srv.HealthCheck(context.Background())
			assert.Equal(t, tt.wantStatus, result["status"])
		})
	}
}

func TestMCPServer_AlertsSnapshot(t *testing.T) {
	srv, store := newTestServer(t)
	defer func() { _ = store.Close() }()

	assert.Empty(t, srv.AlertsSnapshot(context.Background()))

	srv.processAlert(context.Background(), kwatch.Alert{
		Kind: kwatch.AlertKindPod, Name: "api", Severity: "high", OccurredAt: time.Now(),
	})

	alerts := srv.AlertsSnapshot(context.Background())
	assert.Len(t, alerts, 1)
	assert.Equal(t, "api", alerts[0].Alert.Name)
}

func TestMCPServer_IncidentHistory(t *testing.T) {
	srv, store := newTestServer(t)
	defer func() { _ = store.Close() }()

	tests := []struct {
		name   string
		window time.Duration
	}{
		{"DefaultWindow", 0},
		{"CustomWindow", 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := srv.IncidentHistory(context.Background(), tt.window)
			assert.NoError(t, err)
			assert.Empty(t, results)
		})
	}
}

func TestMCPServer_ToolSummaries(t *testing.T) {
	srv, store := newTestServer(t)
	defer func() { _ = store.Close() }()

	summaries := srv.ToolSummaries()
	assert.True(t, len(summaries) > 0, "should have registered tools")

	for i := 1; i < len(summaries); i++ {
		assert.True(t, summaries[i-1].Name <= summaries[i].Name, "summaries should be sorted by name")
	}
}

func TestMCPServer_ListTools(t *testing.T) {
	srv, store := newTestServer(t)
	defer func() { _ = store.Close() }()

	result := srv.ListTools()
	assert.Contains(t, result, "tools")
}
