package pkg

import (
	"sync"
	"testing"

	"github.com/argues/argus/internal/alerts"
)

// TestCachedMetricsConcurrent is a regression test for the data race between
// pollAlerts (reading cachedMetrics to hand to AutoInvestigate) and
// pollMetrics (writing cachedMetrics on each tick). Before the fix the field
// was a plain *alerts.ClusterMetrics; the race detector would flag this test.
// Run with: go test -race ./api/pkg/ -run TestCachedMetricsConcurrent
func TestCachedMetricsConcurrent(t *testing.T) {
	t.Parallel()

	app := &App{}

	const writers = 4
	const readers = 8
	const iterations = 5000

	var wg sync.WaitGroup
	wg.Add(writers + readers)

	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				app.cachedMetrics.Store(&alerts.ClusterMetrics{SLOStatus: "ok"})
			}
		}()
	}

	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = app.cachedMetrics.Load()
			}
		}()
	}

	wg.Wait()
}
