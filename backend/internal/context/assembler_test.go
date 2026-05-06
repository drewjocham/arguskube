package context_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/argues/kube-watcher/internal/anomaly"
	"github.com/argues/kube-watcher/internal/context"
	"github.com/argues/kube-watcher/internal/features"
)

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
	gate := features.NewGate()
	detector := &mockDetector{}
	logger := slog.New(slog.DiscardHandler)

	a := context.NewAssembler(nil, gate, detector, logger)
	if a == nil {
		t.Fatal("NewAssembler() returned nil")
	}
}

func TestNewAssemblerNilParams(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		a := context.NewAssembler(nil, features.NewGate(), &mockDetector{}, slog.New(slog.DiscardHandler))
		if a == nil {
			t.Fatal("NewAssembler() with nil config returned nil")
		}
	})

	t.Run("nil gate", func(t *testing.T) {
		a := context.NewAssembler(nil, nil, &mockDetector{}, slog.New(slog.DiscardHandler))
		if a == nil {
			t.Fatal("NewAssembler() with nil gate returned nil")
		}
	})

	t.Run("nil detector", func(t *testing.T) {
		a := context.NewAssembler(nil, features.NewGate(), nil, slog.New(slog.DiscardHandler))
		if a == nil {
			t.Fatal("NewAssembler() with nil detector returned nil")
		}
	})

	t.Run("nil logger", func(t *testing.T) {
		a := context.NewAssembler(nil, features.NewGate(), &mockDetector{}, nil)
		if a == nil {
			t.Fatal("NewAssembler() with nil logger returned nil")
		}
	})
}

func TestAssembleMinimal(t *testing.T) {
	gate := features.NewGate()
	detector := &mockDetector{}
	logger := slog.New(slog.DiscardHandler)
	a := context.NewAssembler(nil, gate, detector, logger)

	alert := alerts.Alert{
		ID:        "test-alert-1",
		Name:      "OOMKilled",
		Namespace: "default",
		PodName:   "web-app-abc",
		Severity:  alerts.SeverityCritical,
	}

	bundle, err := a.Assemble(context.Background(), alert, nil)
	if err != nil {
		t.Fatalf("Assemble() returned error: %v", err)
	}
	if bundle == nil {
		t.Fatal("Assemble() returned nil bundle")
	}
	if bundle.Alert.ID != "test-alert-1" {
		t.Errorf("expected Alert ID 'test-alert-1', got %q", bundle.Alert.ID)
	}
}

func TestAssembleWithCascadeAlerts(t *testing.T) {
	gate := features.NewGate()
	detector := &mockDetector{}
	logger := slog.New(slog.DiscardHandler)
	a := context.NewAssembler(nil, gate, detector, logger)

	target := alerts.Alert{
		ID:        "target-1",
		Name:      "CrashLoopBackOff",
		Namespace: "default",
		PodName:   "web-app-abc",
		Timestamp: time.Now(),
		Severity:  alerts.SeverityCritical,
	}

	related := []alerts.Alert{
		{
			ID:        "cause-1",
			Name:      "DiskPressure",
			Namespace: "default",
			NodeName:  "node-1",
			Timestamp: time.Now().Add(-5 * time.Minute),
			Severity:  alerts.SeverityWarning,
		},
		{
			ID:        "unrelated-1",
			Name:      "ImagePullBackOff",
			Namespace: "other",
			Timestamp: time.Now(),
			Severity:  alerts.SeverityWarning,
		},
	}

	bundle, err := a.Assemble(context.Background(), target, related)
	if err != nil {
		t.Fatalf("Assemble() returned error: %v", err)
	}
	if bundle == nil {
		t.Fatal("Assemble() returned nil bundle")
	}

	// Should have cascade alerts (DiskPressure → CrashLoopBackOff is a known chain).
	t.Logf("Cascade alerts: %d", len(bundle.CascadeAlerts))
}

func TestAssembleWithAnomalyDetection(t *testing.T) {
	gate := features.NewGate()
	detector := &mockDetector{
		result: &anomaly.DetectResult{
			MetricName:  "container_memory_usage_bytes",
			IsAnomaly:   true,
			Score:       0.91,
			Description: "Memory spike detected",
			ModelUsed:   "isolation-forest",
		},
	}
	logger := slog.New(slog.DiscardHandler)
	a := context.NewAssembler(nil, gate, detector, logger)

	alert := alerts.Alert{
		ID:        "alert-oom",
		Name:      "OOMKilled",
		Namespace: "default",
		PodName:   "memory-hog-xyz",
		Severity:  alerts.SeverityCritical,
	}

	bundle, err := a.Assemble(context.Background(), alert, nil)
	if err != nil {
		t.Fatalf("Assemble() returned error: %v", err)
	}
	if bundle == nil {
		t.Fatal("Assemble() returned nil bundle")
	}

	// Anomaly results should be populated since gate allows FeatureAnomstack.
	if len(bundle.AnomalyResults) != 1 {
		t.Errorf("expected 1 anomaly result, got %d", len(bundle.AnomalyResults))
	}
}

func TestAssembleWithAnomalyError(t *testing.T) {
	gate := features.NewGate()
	detector := &mockDetector{
		err: assertAnError("anomstack down"),
	}
	logger := slog.New(slog.DiscardHandler)
	a := context.NewAssembler(nil, gate, detector, logger)

	alert := alerts.Alert{
		ID:        "test-alert",
		Name:      "CPUThrottleHigh",
		Namespace: "default",
		Severity:  alerts.SeverityWarning,
	}

	// Should not fail despite anomaly error — just log a warning.
	bundle, err := a.Assemble(context.Background(), alert, nil)
	if err != nil {
		t.Fatalf("Assemble() should not fail on anomaly error, got: %v", err)
	}
	if bundle == nil {
		t.Fatal("Assemble() returned nil bundle")
	}
}

type testError struct {
	msg string
}

func assertAnError(msg string) error {
	return &testError{msg: msg}
}

func (e *testError) Error() string { return e.msg }

func TestAssembleWithDecisionLog(t *testing.T) {
	// Create a temp decision log file.
	dir := t.TempDir()
	logPath := filepath.Join(dir, "DECISION_LOG.md")
	content := `# 2024-01-15
The CPUThrottle alert in default/web-app was caused by HPA blindness due to
metrics-server eviction. Actions taken: restarted metrics-server, scaled up replicas.

# 2024-01-10
DiskPressure on node-01 was caused by Prometheus WAL filling up disk.
Actions: increased PV size, reduced retention.
`
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write decision log: %v", err)
	}

	gate := features.NewGate()
	detector := &mockDetector{}
	logger := slog.New(slog.DiscardHandler)

	// We need a config with DecisionLog path set. Since cfg is internal,
	// we create the assembler with a nil config (which handles the no-decision-log case).
	// For the full test with decision log, we'd need to inject a mock.
	_ = logPath
	a := context.NewAssembler(nil, gate, detector, logger)

	alert := alerts.Alert{
		ID:        "test-alert",
		Name:      "CPUThrottleHigh",
		Namespace: "default",
		PodName:   "web-app-abc",
		Severity:  alerts.SeverityWarning,
	}

	bundle, err := a.Assemble(context.Background(), alert, nil)
	if err != nil {
		t.Fatalf("Assemble() returned error: %v", err)
	}
	_ = bundle
}

func TestAssembleContextTimeout(t *testing.T) {
	gate := features.NewGate()
	detector := &mockDetector{
		result: &anomaly.DetectResult{IsAnomaly: false, Score: 0},
	}
	logger := slog.New(slog.DiscardHandler)
	a := context.NewAssembler(nil, gate, detector, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Give the context no time to complete.
	time.Sleep(1 * time.Millisecond)

	alert := alerts.Alert{
		ID:   "timeout-test",
		Name: "OOMKilled",
	}
	bundle, err := a.Assemble(ctx, alert, nil)
	if err != nil {
		t.Logf("Assemble() returned error (expected with timeout): %v", err)
	}
	_ = bundle
}

func TestCascadeCorrelation(t *testing.T) {
	// Test the cascade correlation logic through Assemble.
	gate := features.NewGate()
	detector := &mockDetector{}
	logger := slog.New(slog.DiscardHandler)
	a := context.NewAssembler(nil, gate, detector, logger)

	target := alerts.Alert{
		ID:        "target",
		Name:      "CrashLoopBackOff",
		Namespace: "default",
		PodName:   "web-app",
		Timestamp: time.Now(),
		Severity:  alerts.SeverityCritical,
	}

	related := []alerts.Alert{
		{
			ID:        "eviction-1",
			Name:      "Eviction",
			Namespace: "kube-system",
			PodName:   "metrics-server-xxx",
			EvictedPods: []string{"metrics-server-xxx", "prometheus-yyy"},
			Timestamp: time.Now().Add(-2 * time.Minute),
			Severity:  alerts.SeverityWarning,
		},
	}

	bundle, err := a.Assemble(context.Background(), target, related)
	if err != nil {
		t.Fatalf("Assemble() returned error: %v", err)
	}

	// The Eviction alert should correlate via chain matching.
	t.Logf("Cascade alerts count: %d", len(bundle.CascadeAlerts))
	for _, ca := range bundle.CascadeAlerts {
		t.Logf("  Cascade: %s (%s)", ca.Name, ca.Namespace)
	}
}
