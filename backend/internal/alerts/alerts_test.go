package alerts

import (
	"testing"
	"time"
)

func TestAlertCreation(t *testing.T) {
	tests := []struct {
		name     string
		alert    Alert
		wantSev  Severity
		wantName string
	}{
		{
			name: "critical alert",
			alert: Alert{
				ID:          "alert-1",
				Name:        "Pod CrashLoopBackOff",
				Severity:    SeverityCritical,
				Namespace:   "production",
				Description: "pod payments-api is in CrashLoopBackOff",
				Timestamp:   time.Now(),
				Tags:        []Tag{{Label: "prod", Color: "red"}},
			},
			wantSev:  SeverityCritical,
			wantName: "Pod CrashLoopBackOff",
		},
		{
			name: "warning alert",
			alert: Alert{
				ID:          "alert-2",
				Name:        "High Memory Usage",
				Severity:    SeverityWarning,
				Namespace:   "staging",
				Description: "memory usage above 85%",
				PodName:     "web-0",
				PodPhase:    "Running",
				RestartCount: 3,
				Timestamp:    time.Now(),
			},
			wantSev:  SeverityWarning,
			wantName: "High Memory Usage",
		},
		{
			name: "info alert with related alerts",
			alert: Alert{
				ID:            "alert-3",
				Name:          "Deployment Rollout",
				Severity:      SeverityInfo,
				Namespace:     "default",
				Description:   "new image deployed",
				ImageTag:      "v2.1.0",
				PreviousImage: "v2.0.0",
				DeployTime:    time.Now(),
				RelatedAlerts: []string{"alert-1"},
			},
			wantSev:  SeverityInfo,
			wantName: "Deployment Rollout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.alert
			if got.Severity != tt.wantSev {
				t.Errorf("alert.Severity = %q, want %q", got.Severity, tt.wantSev)
			}
			if got.Name != tt.wantName {
				t.Errorf("alert.Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.ID == "" {
				t.Error("alert.ID should not be empty")
			}
			// Verify all severity constants are valid
			switch got.Severity {
			case SeverityCritical, SeverityWarning, SeverityInfo:
				// valid
			default:
				t.Errorf("unexpected severity: %q", got.Severity)
			}
		})
	}
}

func TestTagCreation(t *testing.T) {
	tag := Tag{Label: "production", Color: "red"}
	if tag.Label != "production" {
		t.Errorf("tag.Label = %q, want %q", tag.Label, "production")
	}
	if tag.Color != "red" {
		t.Errorf("tag.Color = %q, want %q", tag.Color, "red")
	}
}

func TestDiagnosisCreation(t *testing.T) {
	d := Diagnosis{
		AlertID:    "alert-1",
		Hypothesis: "OOMKilled due to memory leak",
		Confidence: 0.92,
		Steps: []RunbookStep{
			{Number: 1, Text: "Check memory usage", Command: "kubectl top pod"},
			{Number: 2, Text: "Check logs", Command: "kubectl logs"},
		},
		DecisionLogEntry: "DECISION_LOG.md entry #42",
	}
	if d.AlertID != "alert-1" {
		t.Errorf("Diagnosis.AlertID = %q, want %q", d.AlertID, "alert-1")
	}
	if d.Confidence < 0 || d.Confidence > 1.0 {
		t.Errorf("Diagnosis.Confidence out of range [0,1]: %f", d.Confidence)
	}
	if len(d.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(d.Steps))
	}
}

func TestRunbookStepCreation(t *testing.T) {
	step := RunbookStep{
		Number:  1,
		Text:    "Check pod status",
		Command: "kubectl get pods -n default",
	}
	if step.Number != 1 {
		t.Errorf("step.Number = %d, want 1", step.Number)
	}
	if step.Text == "" {
		t.Error("step.Text should not be empty")
	}
	if step.Command == "" {
		t.Error("step.Command should not be empty")
	}
}

func TestClusterMetricsDefaults(t *testing.T) {
	cm := ClusterMetrics{}
	if cm.PodHealthPct != 0 {
		t.Errorf("expected 0 PodHealthPct, got %f", cm.PodHealthPct)
	}
	if cm.SLOStatus != "" {
		t.Errorf("expected empty SLOStatus, got %q", cm.SLOStatus)
	}
}

func TestClusterMetricsValues(t *testing.T) {
	cm := ClusterMetrics{
		PodHealthPct:     95.5,
		PodsRunning:      48,
		PodsTotal:        50,
		PodsPending:      1,
		PodsFailed:       1,
		ErrorRate:        0.04,
		ErrorRatePrev:    0.02,
		RestartCount:     12,
		RestartTop:       "payments-api: 8",
		WarningEvents:    3,
		TotalCPUMillis:   24000,
		TotalMemoryBytes: 68719476736,
		SLOStatus:        "ok",
	}
	health := float64(cm.PodsRunning) / float64(cm.PodsTotal) * 100
	if cm.PodHealthPct != health {
		t.Errorf("PodHealthPct = %f, want computed %f", cm.PodHealthPct, health)
	}
	if cm.ErrorRate <= cm.ErrorRatePrev {
		t.Error("expected error rate to have increased from previous poll")
	}
	if cm.PodsRunning+cm.PodsPending+cm.PodsFailed != cm.PodsTotal {
		t.Errorf("pod counts don't add up: %d+%d+%d != %d",
			cm.PodsRunning, cm.PodsPending, cm.PodsFailed, cm.PodsTotal)
	}
}

func TestLogLineCreation(t *testing.T) {
	now := time.Now()
	ll := LogLine{
		Timestamp: now,
		Source:    "kubelet",
		Level:     "error",
		Message:   "Back-off restarting failed container",
	}
	if ll.Source != "kubelet" {
		t.Errorf("LogLine.Source = %q, want %q", ll.Source, "kubelet")
	}
	if ll.Timestamp.IsZero() {
		t.Error("LogLine.Timestamp should not be zero")
	}
	switch ll.Level {
	case "error", "warn", "info", "ok":
		// valid
	default:
		t.Errorf("unexpected log level: %q", ll.Level)
	}
}

func TestServiceNodeAndTopologyEdge(t *testing.T) {
	node := ServiceNode{Name: "payments-api", Status: "ok"}
	if node.Name != "payments-api" {
		t.Errorf("ServiceNode.Name = %q, want %q", node.Name, "payments-api")
	}

	edge := TopologyEdge{From: "payments-api", To: "user-service"}
	if edge.From != "payments-api" || edge.To != "user-service" {
		t.Errorf("TopologyEdge = %+v, want From=payments-api To=user-service", edge)
	}
}

func TestSeverityConstants(t *testing.T) {
	if SeverityCritical != "critical" {
		t.Errorf("SeverityCritical = %q, want %q", SeverityCritical, "critical")
	}
	if SeverityWarning != "warning" {
		t.Errorf("SeverityWarning = %q, want %q", SeverityWarning, "warning")
	}
	if SeverityInfo != "info" {
		t.Errorf("SeverityInfo = %q, want %q", SeverityInfo, "info")
	}
}

func TestAlertTimeSortOrder(t *testing.T) {
	earlier := Alert{ID: "a1", Timestamp: time.Now().Add(-10 * time.Minute)}
	later := Alert{ID: "a2", Timestamp: time.Now()}
	if !earlier.Timestamp.Before(later.Timestamp) {
		t.Error("expected earlier alert before later alert by timestamp")
	}
}
