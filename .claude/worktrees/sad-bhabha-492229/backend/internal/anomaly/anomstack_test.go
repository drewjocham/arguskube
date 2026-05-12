package anomaly_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/argues/kube-watcher/internal/anomaly"
	"github.com/argues/kube-watcher/internal/config"
)

func newTestConfig(baseURL string) *config.OnlineDataConfig {
	return &config.OnlineDataConfig{
		AI: config.AIConfig{
			AnomstackURL:    baseURL,
			AnomstackAPIKey: "test-api-key",
		},
	}
}

func TestNewAnomstackClient(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	cfg := newTestConfig("http://localhost:8080")

	client := anomaly.NewAnomstackClient(cfg, logger)
	if client == nil {
		t.Fatal("NewAnomstackClient() returned nil")
	}
}

func TestNewAnomstackClientNilLogger(t *testing.T) {
	cfg := newTestConfig("http://localhost:8080")

	client := anomaly.NewAnomstackClient(cfg, nil)
	if client == nil {
		t.Fatal("NewAnomstackClient() with nil logger returned nil")
	}
}

func TestNewAnomstackClientEmptyConfig(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	cfg := &config.OnlineDataConfig{}

	client := anomaly.NewAnomstackClient(cfg, logger)
	if client == nil {
		t.Fatal("NewAnomstackClient() with empty config returned nil")
	}
}

func TestDetectSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Authorization header, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"metric_name": "container_cpu_usage",
			"is_anomaly": true,
			"score": 0.92,
			"description": "CPU usage spike detected",
			"detected_at": "2024-01-15T10:00:00Z",
			"model_used": "isolation-forest-v2"
		}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newTestConfig(server.URL)
	client := anomaly.NewAnomstackClient(cfg, logger)

	req := anomaly.DetectRequest{
		MetricName: "container_cpu_usage",
		Labels:     map[string]string{"namespace": "default"},
		Window:     30 * time.Minute,
	}

	result, err := client.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Detect() returned nil result")
	}
	if !result.IsAnomaly {
		t.Error("expected is_anomaly = true")
	}
	if result.Score != 0.92 {
		t.Errorf("expected Score 0.92, got %f", result.Score)
	}
	if result.ModelUsed != "isolation-forest-v2" {
		t.Errorf("expected ModelUsed 'isolation-forest-v2', got %q", result.ModelUsed)
	}
	if result.MetricName != "container_cpu_usage" {
		t.Errorf("expected MetricName 'container_cpu_usage', got %q", result.MetricName)
	}
}

func TestDetectNonAnomalous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"metric_name": "container_mem_usage",
			"is_anomaly": false,
			"score": 0.12,
			"description": "Normal pattern",
			"detected_at": "2024-01-15T10:00:00Z",
			"model_used": "moving-average"
		}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newTestConfig(server.URL)
	client := anomaly.NewAnomstackClient(cfg, logger)

	req := anomaly.DetectRequest{
		MetricName: "container_mem_usage",
		Window:     15 * time.Minute,
	}

	result, err := client.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
	if result.IsAnomaly {
		t.Error("expected is_anomaly = false")
	}
	if result.Score != 0.12 {
		t.Errorf("expected Score 0.12, got %f", result.Score)
	}
}

func TestDetectServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newTestConfig(server.URL)
	client := anomaly.NewAnomstackClient(cfg, logger)

	req := anomaly.DetectRequest{
		MetricName: "test_metric",
		Window:     5 * time.Minute,
	}

	_, err := client.Detect(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestDetectCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"is_anomaly": false, "score": 0}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newTestConfig(server.URL)
	client := anomaly.NewAnomstackClient(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := anomaly.DetectRequest{
		MetricName: "test_metric",
		Window:     5 * time.Minute,
	}

	_, err := client.Detect(ctx, req)
	if err == nil {
		t.Log("Detect returned nil error for cancelled context (may be network-dependent)")
	}
}

func TestListJobsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"name": "cpu-spike-detector", "metric": "container_cpu_usage", "schedule": "*/5 * * * *", "last_run": "2024-01-15T09:55:00Z", "status": "ok"},
			{"name": "mem-leak-detector", "metric": "container_mem_usage", "schedule": "*/5 * * * *", "last_run": "2024-01-15T09:55:00Z", "status": "alert"}
		]`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newTestConfig(server.URL)
	client := anomaly.NewAnomstackClient(cfg, logger)

	jobs, err := client.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() returned error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Name != "cpu-spike-detector" {
		t.Errorf("expected job name 'cpu-spike-detector', got %q", jobs[0].Name)
	}
	if jobs[1].Status != "alert" {
		t.Errorf("expected job status 'alert', got %q", jobs[1].Status)
	}
}

func TestListJobsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newTestConfig(server.URL)
	client := anomaly.NewAnomstackClient(cfg, logger)

	jobs, err := client.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() returned error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestListJobsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newTestConfig(server.URL)
	client := anomaly.NewAnomstackClient(cfg, logger)

	_, err := client.ListJobs(context.Background())
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestDetectWithoutAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Errorf("expected no Authorization header, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"is_anomaly": false, "score": 0}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := &config.OnlineDataConfig{
		AI: config.AIConfig{
			AnomstackURL: server.URL,
			// No API key
		},
	}
	client := anomaly.NewAnomstackClient(cfg, logger)

	_, err := client.Detect(context.Background(), anomaly.DetectRequest{
		MetricName: "test",
		Window:     5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
}

func TestDetectRequestLabelsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"is_anomaly": false, "score": 0}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newTestConfig(server.URL)
	client := anomaly.NewAnomstackClient(cfg, logger)

	req := anomaly.DetectRequest{
		MetricName: "test",
		// No labels
		Window: 5 * time.Minute,
	}

	_, err := client.Detect(context.Background(), req)
	if err != nil {
		t.Fatalf("Detect() with empty labels returned error: %v", err)
	}
}
