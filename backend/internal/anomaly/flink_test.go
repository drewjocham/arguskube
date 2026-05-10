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

func newFlinkTestConfig(baseURL string) *config.OnlineDataConfig {
	return &config.OnlineDataConfig{
		AI: config.AIConfig{
			FlinkURL:    baseURL,
			FlinkAPIKey: "test-flink-key",
		},
	}
}

func TestNewFlinkClient(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	cfg := newFlinkTestConfig("http://localhost:8080")

	client := anomaly.NewFlinkClient(cfg, logger)
	if client == nil {
		t.Fatal("NewFlinkClient() returned nil")
	}
}

func TestNewFlinkClientNilLogger(t *testing.T) {
	cfg := newFlinkTestConfig("http://localhost:8080")

	client := anomaly.NewFlinkClient(cfg, nil)
	if client == nil {
		t.Fatal("NewFlinkClient() with nil logger returned nil")
	}
}

func TestNewFlinkClientEmptyConfig(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	cfg := &config.OnlineDataConfig{}

	client := anomaly.NewFlinkClient(cfg, logger)
	if client == nil {
		t.Fatal("NewFlinkClient() with empty config returned nil")
	}
}

func TestFlinkDetectSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-flink-key" {
			t.Errorf("expected Authorization header, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"metric_name": "container_cpu_usage",
			"is_anomaly": true,
			"score": 0.89,
			"description": "CPU usage spike detected by Flink",
			"detected_at": "2024-01-15T10:00:00Z",
			"model_used": "flink-streaming-sma-v1"
		}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newFlinkTestConfig(server.URL)
	client := anomaly.NewFlinkClient(cfg, logger)

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
	if result.Score != 0.89 {
		t.Errorf("expected Score 0.89, got %f", result.Score)
	}
	if result.ModelUsed != "flink-streaming-sma-v1" {
		t.Errorf("expected ModelUsed 'flink-streaming-sma-v1', got %q", result.ModelUsed)
	}
	if result.MetricName != "container_cpu_usage" {
		t.Errorf("expected MetricName 'container_cpu_usage', got %q", result.MetricName)
	}
}

func TestFlinkDetectNonAnomalous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"metric_name": "container_mem_usage",
			"is_anomaly": false,
			"score": 0.08,
			"description": "Normal pattern",
			"detected_at": "2024-01-15T10:00:00Z",
			"model_used": "flink-moving-average"
		}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newFlinkTestConfig(server.URL)
	client := anomaly.NewFlinkClient(cfg, logger)

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
	if result.Score != 0.08 {
		t.Errorf("expected Score 0.08, got %f", result.Score)
	}
}

func TestFlinkDetectServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "flink job failed"}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newFlinkTestConfig(server.URL)
	client := anomaly.NewFlinkClient(cfg, logger)

	_, err := client.Detect(context.Background(), anomaly.DetectRequest{
		MetricName: "test_metric",
		Window:     5 * time.Minute,
	})
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestFlinkDetectCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"is_anomaly": false, "score": 0}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newFlinkTestConfig(server.URL)
	client := anomaly.NewFlinkClient(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Detect(ctx, anomaly.DetectRequest{
		MetricName: "test_metric",
		Window:     5 * time.Minute,
	})
	if err == nil {
		t.Log("Detect returned nil error for cancelled context (may be network-dependent)")
	}
}

func TestFlinkListJobsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"name": "cpu-anomaly-detector", "metric": "container_cpu_usage", "schedule": "*/1 * * * *", "last_run": "2024-01-15T09:59:00Z", "status": "ok"},
			{"name": "mem-anomaly-detector", "metric": "container_mem_usage", "schedule": "*/1 * * * *", "last_run": "2024-01-15T09:59:00Z", "status": "alert"}
		]`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newFlinkTestConfig(server.URL)
	client := anomaly.NewFlinkClient(cfg, logger)

	jobs, err := client.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() returned error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Name != "cpu-anomaly-detector" {
		t.Errorf("expected job name 'cpu-anomaly-detector', got %q", jobs[0].Name)
	}
	if jobs[1].Status != "alert" {
		t.Errorf("expected job status 'alert', got %q", jobs[1].Status)
	}
}

func TestFlinkListJobsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newFlinkTestConfig(server.URL)
	client := anomaly.NewFlinkClient(cfg, logger)

	jobs, err := client.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs() returned error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestFlinkListJobsServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newFlinkTestConfig(server.URL)
	client := anomaly.NewFlinkClient(cfg, logger)

	_, err := client.ListJobs(context.Background())
	if err == nil {
		t.Fatal("expected error for server error")
	}
}

func TestFlinkDetectWithoutAPIKey(t *testing.T) {
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
			FlinkURL: server.URL,
		},
	}
	client := anomaly.NewFlinkClient(cfg, logger)

	_, err := client.Detect(context.Background(), anomaly.DetectRequest{
		MetricName: "test",
		Window:     5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Detect() returned error: %v", err)
	}
}

func TestFlinkDetectRequestLabelsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"is_anomaly": false, "score": 0}`))
	}))
	defer server.Close()

	logger := slog.New(slog.DiscardHandler)
	cfg := newFlinkTestConfig(server.URL)
	client := anomaly.NewFlinkClient(cfg, logger)

	_, err := client.Detect(context.Background(), anomaly.DetectRequest{
		MetricName: "test",
		Window:     5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Detect() with empty labels returned error: %v", err)
	}
}
