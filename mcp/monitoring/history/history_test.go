package history

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalcChange(t *testing.T) {
	tests := []struct {
		name     string
		prev     int
		curr     int
		expected float64
	}{
		{"Increase", 10, 15, 50.0},
		{"Decrease", 10, 5, -50.0},
		{"NoChange", 10, 10, 0.0},
		{"FromZero", 0, 5, 100.0},
		{"BothZero", 0, 0, 0.0},
		{"LargeIncrease", 1, 10, 900.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcChange(tt.prev, tt.curr)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestStore_Integration(t *testing.T) {
	// Use a background context for setup, but specialized ones for subtests
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	require.NoError(t, err, "Failed to initialize store in temp dir")
	defer func() { _ = store.Close() }()

	now := time.Now().Truncate(time.Millisecond) // Truncate to avoid nano-precision jitter in some environments

	t.Run("RecordAndRetrieve", func(t *testing.T) {
		inc := Incident{
			ID:        "pod-123",
			Timestamp: now,
			Kind:      IncidentTypePod,
			Severity:  "warning",
			Message:   "OOMKilled",
		}

		err := store.Record(ctx, inc)
		assert.NoError(t, err)

		res, err := store.List(ctx, IncidentTypePod, 1*time.Minute)
		assert.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, inc.ID, res[0].ID)
		assert.Equal(t, inc.Message, res[0].Message)
	})

	t.Run("ListFilteringByKind", func(t *testing.T) {
		nodeInc := Incident{ID: "node-1", Timestamp: now, Kind: IncidentTypeNode}
		require.NoError(t, store.Record(ctx, nodeInc))

		podList, _ := store.List(ctx, IncidentTypePod, 1*time.Hour)
		nodeList, _ := store.List(ctx, IncidentTypeNode, 1*time.Hour)

		// Based on previous test + this one:
		assert.Len(t, podList, 1)
		assert.Len(t, nodeList, 1)
	})

	t.Run("FrequencyAnalysis", func(t *testing.T) {
		// Clean start for frequency test
		fStore, _ := NewStore(t.TempDir())
		defer func() { _ = fStore.Close() }()

		// 3 incidents in the last hour (Recent)
		for i := 0; i < 3; i++ {
			_ = fStore.Record(ctx, Incident{ID: string(rune(i)), Timestamp: now.Add(-10 * time.Minute), Kind: IncidentTypeEvent})
		}
		// 1 incident 3 hours ago (Previous)
		_ = fStore.Record(ctx, Incident{ID: "old-1", Timestamp: now.Add(-3 * time.Hour), Kind: IncidentTypeEvent})

		comp, err := fStore.CompareFrequency(ctx, IncidentTypeEvent, 1*time.Hour, 5*time.Hour)
		assert.NoError(t, err)
		assert.Equal(t, 3, comp.RecentCount)
		assert.Equal(t, 1, comp.PreviousCount)
		assert.Equal(t, 200.0, comp.PercentChange)
	})

	t.Run("Cleanup", func(t *testing.T) {
		cStore, _ := NewStore(t.TempDir())
		defer func() { _ = cStore.Close() }()

		old := now.Add(-24 * time.Hour)
		fresh := now.Add(-5 * time.Minute)

		_ = cStore.Record(ctx, Incident{ID: "expired", Timestamp: old, Kind: IncidentTypePod})
		_ = cStore.Record(ctx, Incident{ID: "keep", Timestamp: fresh, Kind: IncidentTypePod})

		cStore.performCleanup(ctx, 1*time.Hour)

		res, _ := cStore.List(ctx, IncidentTypePod, 48*time.Hour)
		assert.Len(t, res, 1)
		assert.Equal(t, "keep", res[0].ID)
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		cancelledCtx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := store.List(cancelledCtx, IncidentTypePod, 1*time.Hour)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
