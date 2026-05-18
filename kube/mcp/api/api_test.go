package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argues/argus/mcp/monitoring/history"
	"github.com/argues/argus/mcp/server"
	"github.com/argues/argus/pkg/kube"

	clientset "k8s.io/client-go/kubernetes"

	kwatch "github.com/argues/argus/pkg/kube/watch"
)

// --- stubs ---

type stubK8sClient struct{}

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
func (s *stubK8sClient) HealthCheck(_ context.Context) error  { return nil }
func (s *stubK8sClient) GetRawInterface() clientset.Interface { return nil }

type stubChecker struct{ err error }

func (c *stubChecker) Check(_ context.Context) error { return c.err }

func newTestAPI(t *testing.T) (*API, *history.Store) {
	t.Helper()
	logger := slog.Default()
	store, err := history.NewStore(t.TempDir())
	require.NoError(t, err)

	wm := kwatch.NewManager(&stubK8sClient{}, logger, 30*time.Second)

	srv, err := server.NewMCPServer(logger, server.Config{
		Version:      "test",
		GitCommit:    "abc",
		BuildDate:    "now",
		K8sClient:    &stubK8sClient{},
		HistoryStore: store,
		Watcher:      wm,
	})
	require.NoError(t, err)

	a, err := New(Config{
		Server:      srv,
		Logger:      logger,
		ServiceName: "test-api",
		Version:     "0.1.0",
	})
	require.NoError(t, err)
	return a, store
}

// --- New() ---

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "NilServer",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name: "ValidConfig",
			cfg: Config{
				Server: func() *server.MCPServer {
					store, _ := history.NewStore(t.TempDir())
					wm := kwatch.NewManager(&stubK8sClient{}, slog.Default(), 30*time.Second)
					srv, _ := server.NewMCPServer(slog.Default(), server.Config{
						Version: "test", K8sClient: &stubK8sClient{}, HistoryStore: store, Watcher: wm,
					})
					t.Cleanup(func() { _ = store.Close() })
					return srv
				}(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- HTTP Handler tests ---

func TestHandleRoot(t *testing.T) {
	api, store := newTestAPI(t)
	defer func() { _ = store.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	api.Routes().ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test-api")
}

func TestHandleHealth(t *testing.T) {
	tests := []struct {
		name       string
		checker    Checker
		wantStatus int
	}{
		{"NoChecker", nil, http.StatusOK},
		{"HealthyChecker", &stubChecker{}, http.StatusOK},
		{"UnhealthyChecker", &stubChecker{err: errors.New("down")}, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, store := newTestAPI(t)
			defer func() { _ = store.Close() }()
			api.liveness = tt.checker

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			api.Routes().ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestHandleReady(t *testing.T) {
	tests := []struct {
		name       string
		checker    Checker
		wantStatus int
	}{
		{"NoChecker", nil, http.StatusOK},
		{"ReadyChecker", &stubChecker{}, http.StatusOK},
		{"NotReadyChecker", &stubChecker{err: errors.New("not ready")}, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, store := newTestAPI(t)
			defer func() { _ = store.Close() }()
			api.readiness = tt.checker

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			api.Routes().ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestHandleListTools(t *testing.T) {
	api, store := newTestAPI(t)
	defer func() { _ = store.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/tools", nil)
	api.Routes().ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Contains(t, body, "tools")
}

func TestHandleExecuteTool(t *testing.T) {
	tests := []struct {
		name       string
		tool       string
		body       any
		wantStatus int
	}{
		{"ValidTool", "get_server_version", map[string]any{}, http.StatusOK},
		{"UnknownTool", "nonexistent", map[string]any{}, http.StatusNotFound},
		{"InvalidBody", "get_server_version", "not json{{{", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, store := newTestAPI(t)
			defer func() { _ = store.Close() }()

			var buf bytes.Buffer
			if s, ok := tt.body.(string); ok {
				buf.WriteString(s)
			} else {
				_ = json.NewEncoder(&buf).Encode(tt.body)
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/v1/tools/"+tt.tool, &buf)
			r.Header.Set("Content-Type", "application/json")
			api.Routes().ServeHTTP(w, r)

			assert.Equal(t, tt.wantStatus, w.Code)
		})
	}
}

func TestHandleAlerts(t *testing.T) {
	api, store := newTestAPI(t)
	defer func() { _ = store.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/alerts", nil)
	api.Routes().ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Contains(t, body, "alerts")
}

func TestHandleHistory(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"DefaultWindow", ""},
		{"CustomWindow", "?window=24h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, store := newTestAPI(t)
			defer func() { _ = store.Close() }()

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/v1/history"+tt.query, nil)
			api.Routes().ServeHTTP(w, r)

			assert.Equal(t, http.StatusOK, w.Code)

			var body map[string]any
			require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
			assert.Contains(t, body, "window")
		})
	}
}

func TestHandleStatus(t *testing.T) {
	api, store := newTestAPI(t)
	defer func() { _ = store.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	api.Routes().ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
	assert.Equal(t, "0.1.0", body["version"])
}

func TestRespondErrorSanitized(t *testing.T) {
	api, store := newTestAPI(t)
	defer func() { _ = store.Close() }()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	api.respondError(w, r, http.StatusInternalServerError, "something went wrong", errors.New("secret db password"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "something went wrong", body["error"])
	assert.NotContains(t, body["error"], "secret db password")
}
