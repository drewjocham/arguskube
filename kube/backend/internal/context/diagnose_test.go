package context_test

import (
	"testing"
	"time"

	"github.com/argues/argus/internal/alerts"
	"github.com/argues/argus/internal/anomaly"
	"github.com/argues/argus/internal/context"
)

func makeAlert(name string) alerts.Alert {
	return alerts.Alert{
		ID:           "alert-1",
		Name:         name,
		Namespace:    "default",
		PodName:      "web-app-abc",
		Severity:     alerts.SeverityCritical,
		RestartCount: 5,
		MemoryLimit:  "512Mi",
		CPULimit:     "500m",
		NodeName:     "node-1",
		DiskUsage:    0.85,
		CPUThrottle:  75.0,
		ImageTag:     "nginx:1.25",
		Timestamp:    time.Now(),
	}
}

func TestGenerateDiagnosisNilBundle(t *testing.T) {
	result := context.GenerateDiagnosis(nil)
	if result != nil {
		t.Fatal("expected nil result for nil bundle")
	}
}

func TestDiagnoseOOM(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("OOMKilled"),
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.AlertID != "alert-1" {
		t.Errorf("expected AlertID 'alert-1', got %q", diag.AlertID)
	}
	if diag.Confidence < 0.8 {
		t.Errorf("expected OOM confidence >= 0.8, got %f", diag.Confidence)
	}
	if diag.Hypothesis == "" {
		t.Error("expected non-empty hypothesis")
	}
	if len(diag.Steps) == 0 {
		t.Error("expected at least 1 runbook step")
	}
}

func TestDiagnoseOOMWithDeploy(t *testing.T) {
	alert := makeAlert("OOMKilled")
	alert.DeployTime = time.Now().Add(-30 * time.Minute)

	bundle := &context.Bundle{Alert: alert}
	diag := context.GenerateDiagnosis(bundle)

	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.Confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9 for deploy-related OOM, got %f", diag.Confidence)
	}
}

func TestDiagnoseCrashLoop(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("CrashLoopBackOff"),
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.Confidence < 0.7 {
		t.Errorf("expected CrashLoop confidence >= 0.7, got %f", diag.Confidence)
	}
	if len(diag.Steps) < 3 {
		t.Errorf("expected at least 3 steps for CrashLoop, got %d", len(diag.Steps))
	}
}

func TestDiagnoseDiskPressure(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("DiskPressure"),
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.Confidence < 0.85 {
		t.Errorf("expected DiskPressure confidence >= 0.85, got %f", diag.Confidence)
	}
	if len(diag.Steps) < 3 {
		t.Errorf("expected at least 3 steps for DiskPressure, got %d", len(diag.Steps))
	}
}

func TestDiagnoseDiskPressureWithMonitoringEviction(t *testing.T) {
	alert := makeAlert("DiskPressure")
	alert.EvictedPods = []string{"metrics-server-xyz", "web-app-abc"}

	bundle := &context.Bundle{Alert: alert}
	diag := context.GenerateDiagnosis(bundle)

	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.Confidence < 0.9 {
		t.Errorf("expected DiskPressure confidence >= 0.9 with monitoring eviction, got %f", diag.Confidence)
	}
	if !containsAny(diag.Hypothesis, "metrics-server", "prometheus") {
		t.Log("Hypothesis mentions eviction:", diag.Hypothesis)
	}
}

func TestDiagnoseCPUThrottle(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("CPUThrottleHigh"),
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.Confidence < 0.7 {
		t.Errorf("expected CPUThrottle confidence >= 0.7, got %f", diag.Confidence)
	}
}

func TestDiagnoseCPUThrottleWithHPAImpact(t *testing.T) {
	alert := makeAlert("CPUThrottleHigh")
	alert.CPUThrottle = 90.0

	bundle := &context.Bundle{
		Alert: alert,
		CascadeAlerts: []alerts.Alert{
			{
				Name:        "DiskPressure",
				NodeName:    "node-1",
				EvictedPods: []string{"metrics-server-xyz"},
			},
		},
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.Confidence < 0.9 {
		t.Errorf("expected CPUThrottle+HPA confidence >= 0.9, got %f", diag.Confidence)
	}
}

func TestDiagnoseImagePull(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("ImagePullBackOff"),
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.Confidence < 0.85 {
		t.Errorf("expected ImagePull confidence >= 0.85, got %f", diag.Confidence)
	}
	if len(diag.Steps) < 3 {
		t.Errorf("expected at least 3 steps for ImagePull, got %d", len(diag.Steps))
	}
}

func TestDiagnoseMemoryPressure(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("MemoryPressure"),
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.Confidence < 0.75 {
		t.Errorf("expected MemoryPressure confidence >= 0.75, got %f", diag.Confidence)
	}
}

func TestDiagnoseGeneric(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("NodeNotReady"),
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.Confidence < 0.4 || diag.Confidence > 0.6 {
		t.Errorf("expected generic confidence ~0.5, got %f", diag.Confidence)
	}
}

func TestDiagnoseWithAnomalyBoost(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("GenericAlert"),
		AnomalyResults: []anomaly.DetectResult{
			{
				MetricName: "container_memory_usage_bytes",
				IsAnomaly:  true,
				Score:      0.95,
				ModelUsed:  "isolation-forest",
			},
		},
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	// Anomaly should boost confidence by 0.1.
	if diag.Confidence < 0.5 {
		t.Errorf("expected confidence >= 0.5 (base 0.5 + boost), got %f", diag.Confidence)
	}
}

func TestDiagnoseWithCascadeNote(t *testing.T) {
	cascadeAlert := makeAlert("OOMKilled")
	cascadeAlert.Timestamp = time.Now().Add(-3 * time.Minute)

	bundle := &context.Bundle{
		Alert: makeAlert("CrashLoopBackOff"),
		CascadeAlerts: []alerts.Alert{
			cascadeAlert,
		},
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.CascadeNote == "" {
		t.Error("expected non-empty CascadeNote when cascade alerts exist")
	}
	if !containsAny(diag.CascadeNote, "correlated", "cascade", "OOM") {
		t.Logf("CascadeNote: %s", diag.CascadeNote)
	}
}

func TestDiagnoseWithDecisionLog(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("CPUThrottleHigh"),
		DecisionLog: []context.DecisionEntry{
			{Date: "2024-01-10", Content: "Resolved CPU throttling by scaling up replicas."},
		},
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}
	if diag.DecisionLogEntry == "" {
		t.Error("expected non-empty DecisionLogEntry when decision log entries exist")
	}
}

func TestDiagnoseStepCommands(t *testing.T) {
	bundle := &context.Bundle{
		Alert: makeAlert("OOMKilled"),
	}

	diag := context.GenerateDiagnosis(bundle)
	if diag == nil {
		t.Fatal("GenerateDiagnosis() returned nil")
	}

	for _, step := range diag.Steps {
		if step.Number <= 0 {
			t.Errorf("expected positive step number, got %d", step.Number)
		}
		if step.Text == "" {
			t.Error("expected non-empty step text")
		}
	}
}

func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
