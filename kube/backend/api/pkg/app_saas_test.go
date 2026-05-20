package pkg

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/argues/argus/internal/alerts"
)

// ConnectToAgent now has a fallback path: when the in-cluster agent
// isn't reachable, the webhook-ingest buffer is the source. These
// tests pin both the projection shape and the namespace filter.

func newAppWithWebhookBuffer(buf []alerts.Alert) *App {
	a := &App{
		logger:        slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		webhookAlerts: buf,
	}
	return a
}

func TestAnomalyScoreFromSeverity(t *testing.T) {
	t.Parallel()
	cases := map[string]float64{
		"critical": 95,
		"CRITICAL": 95,
		"warning":  75,
		"high":     75,
		"info":     45,
		"":         60, // unknown → middle of road
		"weird":    60,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if got := anomalyScoreFromSeverity(in); got != want {
				t.Errorf("anomalyScoreFromSeverity(%q) = %v, want %v", in, got, want)
			}
		})
	}
}

func TestConnectToAgent_WebhookFallback_AllNamespaces(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	app := newAppWithWebhookBuffer([]alerts.Alert{
		{Name: "high-cpu", Severity: alerts.Severity("critical"), Namespace: "prod-api", Timestamp: now},
		{Name: "memory-spike", Severity: alerts.Severity("warning"), Namespace: "billing", Timestamp: now.Add(-1 * time.Minute)},
		{Name: "info-event", Severity: alerts.Severity("info"), Namespace: "default", Timestamp: now.Add(-2 * time.Minute)},
	})

	got, err := app.ConnectToAgent("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 anomalies across all namespaces, got %d", len(got))
	}

	// First entry should retain timestamp + map severity → score.
	if got[0].Score != 95 {
		t.Errorf("first entry score = %v, want 95 (critical)", got[0].Score)
	}
	if got[0].Target != "prod-api" {
		t.Errorf("first entry target = %q, want %q", got[0].Target, "prod-api")
	}
	if got[0].Rule != "high-cpu" {
		t.Errorf("first entry rule = %q, want %q", got[0].Rule, "high-cpu")
	}
}

func TestConnectToAgent_WebhookFallback_NamespaceFilter(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	app := newAppWithWebhookBuffer([]alerts.Alert{
		{Name: "x", Severity: alerts.Severity("critical"), Namespace: "prod-api", Timestamp: now},
		{Name: "y", Severity: alerts.Severity("warning"), Namespace: "billing", Timestamp: now},
		{Name: "z", Severity: alerts.Severity("info"), Namespace: "prod-api", Timestamp: now},
	})

	got, err := app.ConnectToAgent("prod-api")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 anomalies in prod-api, got %d", len(got))
	}
	for _, a := range got {
		if a.Target != "prod-api" {
			t.Errorf("got anomaly outside prod-api: %+v", a)
		}
	}
}

func TestConnectToAgent_EmptyWebhookBuffer(t *testing.T) {
	t.Parallel()

	app := newAppWithWebhookBuffer(nil)
	got, err := app.ConnectToAgent("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %d entries", len(got))
	}
}
