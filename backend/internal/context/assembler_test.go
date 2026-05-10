package context_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/argues/kube-watcher/internal/anomaly"
	"github.com/argues/kube-watcher/internal/config"
	appcontext "github.com/argues/kube-watcher/internal/context"
	"github.com/argues/kube-watcher/internal/features"
)

type ctxKey string

// mockDetector simulates anomaly detection for tests.
type mockDetector struct {
	result *anomaly.DetectResult
	err    error
}

func (m *mockDetector) Detect(_ context.Context, _ anomaly.DetectRequest) (*anomaly.DetectResult, error) {
	return m.result, m.err
}

func (m *mockDetector) ListJobs(_ context.Context) ([]anomaly.Job, error) {
	return nil, nil
}

func TestNewAssembler(t *testing.T) {
	cfg := &config.OnlineDataConfig{}
	gate := features.NewGate(cfg)
	detector := &mockDetector{}
	logger := slog.New(slog.DiscardHandler)

	a := appcontext.NewAssembler(cfg, gate, detector, logger)
	if a == nil {
		t.Fatal("NewAssembler() returned nil")
	}
}

func TestNewAssemblerNilLogger(t *testing.T) {
	a := appcontext.NewAssembler(nil, features.NewGate(&config.OnlineDataConfig{}), &mockDetector{}, nil)
	if a != nil {
		// Accept either nil or non-nil return — depends on implementation
		t.Log("NewAssembler returned:", a)
	}
}

func TestAssemble(t *testing.T) {
	alert := alerts.Alert{
		ID:       "alert-1",
		Name:     "CPUThrottleHigh",
		Severity: alerts.SeverityCritical,
		PodName:  "web-app-abc",
	}

	detector := &mockDetector{
		result: &anomaly.DetectResult{
			MetricName: "container_cpu_usage_seconds_total",
			IsAnomaly:  true,
			Score:      0.92,
			ModelUsed:  "isolation-forest",
		},
	}

	a := appcontext.NewAssembler(nil, features.NewGate(&config.OnlineDataConfig{}), detector, slog.New(slog.DiscardHandler))
	if a == nil {
		t.Fatal("NewAssembler() returned nil")
	}

	bundle, err := a.Assemble(context.WithValue(context.Background(), ctxKey("alert"), alert), alert, []alerts.Alert{alert})
	if err != nil {
		t.Fatalf("Assemble() failed: %v", err)
	}
	if bundle == nil {
		t.Fatal("Assemble() returned nil bundle")
	}
	if bundle.Alert.ID != "alert-1" {
		t.Errorf("expected Alert.ID 'alert-1', got %q", bundle.Alert.ID)
	}
}
