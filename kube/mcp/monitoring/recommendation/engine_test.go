package recommendation

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argues/argus/mcp/monitoring/history"
	"github.com/argues/argus/pkg/kube/watch"
)

func TestEngine_ForAlert(t *testing.T) {
	store, err := history.NewStore(t.TempDir())
	require.NoError(t, err)
	defer func() { _ = store.Close() }()

	logger := slog.Default()
	engine := NewEngine(store, logger)

	tests := []struct {
		name      string
		alert     watch.Alert
		wantTitle string
		wantSteps int
	}{
		{
			name:      "NodeAlert",
			alert:     watch.Alert{Kind: watch.AlertKindNode, Name: "worker-1", Severity: "critical", Message: "not ready"},
			wantTitle: "node: critical",
			wantSteps: 3,
		},
		{
			name:      "PodAlert",
			alert:     watch.Alert{Kind: watch.AlertKindPod, Name: "api", Severity: "high", Message: "crash loop"},
			wantTitle: "pod: high",
			wantSteps: 3,
		},
		{
			name:      "EventAlert",
			alert:     watch.Alert{Kind: watch.AlertKindEvent, Name: "scheduler", Severity: "warning", Message: "failed scheduling"},
			wantTitle: "event: warning",
			wantSteps: 2,
		},
		{
			name:      "UnknownKind",
			alert:     watch.Alert{Kind: "unknown", Name: "x", Severity: "low"},
			wantTitle: "unknown: low",
			wantSteps: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := engine.ForAlert(context.Background(), tt.alert)
			require.NoError(t, err)
			assert.Equal(t, tt.wantTitle, rec.Title)
			assert.Len(t, rec.Steps, tt.wantSteps)
			assert.Equal(t, tt.alert.Severity, rec.Severity)
			assert.Equal(t, tt.alert.Message, rec.Summary)
		})
	}
}
