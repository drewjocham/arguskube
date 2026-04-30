package tools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/djocham/kube-watcher/mcp/monitoring/history"
)

func TestHistoryInsightsTool_Execute(t *testing.T) {
	ctx := context.Background()
	store, err := history.NewStore(t.TempDir())
	require.NoError(t, err, "Failed to initialize history store")
	defer func() {
		require.NoError(t, store.Close())
	}()

	now := time.Now()
	seedIncidents(t, ctx, store, []history.Incident{
		{
			ID:        "1",
			Kind:      history.IncidentTypePod,
			Severity:  "high",
			Timestamp: now.Add(-1 * time.Hour),
			Namespace: "default",
			Name:      "api",
		},
		{
			ID:        "2",
			Kind:      history.IncidentTypePod,
			Severity:  "low",
			Timestamp: now.Add(-3 * time.Hour),
			Namespace: "default",
			Name:      "api",
		},
	})

	tool := NewHistoryInsightsTool(store)

	tests := []struct {
		name        string
		args        map[string]any
		wantCount   int
		wantInsight string
		wantErr     bool
	}{
		{
			name: "FilterBySeverityAndLimit",
			args: map[string]any{
				"kind":        string(history.IncidentTypePod),
				"since_hours": 4.0,
				"severity":    "high",
				"limit":       1,
			},
			wantCount:   1,
			wantInsight: "Critical: Significant spike in issue frequency detected compared to previous window.",
		},
		{
			name:        "ReturnAllWithoutFilters",
			args:        map[string]any{},
			wantCount:   2,
			wantInsight: "Critical: Significant spike in issue frequency detected compared to previous window.",
		},
		{
			name: "InvalidArgType",
			args: map[string]any{
				"limit": "not-an-int",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := tool.Execute(ctx, tt.args)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			results, ok := res["results"].(map[string]any)
			require.True(t, ok, "Results payload should be a map")

			assert.Equal(t, tt.wantCount, results["count"])
			assert.Equal(t, tt.wantInsight, res["insight"])
		})
	}
}

func seedIncidents(t *testing.T, ctx context.Context, s *history.Store, incs []history.Incident) {
	t.Helper()
	for _, inc := range incs {
		err := s.Record(ctx, inc)
		require.NoError(t, err, "Failed to seed incident ID: %s", inc.ID)
	}
}
