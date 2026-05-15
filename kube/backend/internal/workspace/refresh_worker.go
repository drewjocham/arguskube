package workspace

import (
	"context"
	"time"
)

// RefreshWorker proactively refreshes OAuth tokens that are about to
// expire. The on-demand refresh path in Manager.Token handles the
// "first call after expiry" case, but at the cost of one extra round-
// trip on whatever user action triggered it. This worker takes that
// hit in the background so the user-facing latency stays clean.
//
// Worker policy:
//   - Wake every `Interval` (default 5 min).
//   - Look up connections whose stored expires_at falls inside the
//     next `Threshold` (default 15 min).
//   - Call Manager.RefreshIfStale on each — single-flighted with the
//     on-demand path, so a worker tick + an adapter call at the same
//     moment coalesce.
//
// The worker is best-effort: a failed refresh is logged and the
// next tick will retry (until Manager.refreshNow's "refresh revoked
// => delete row" hits, after which the connection drops off the
// expiring list entirely).
type RefreshWorker struct {
	mgr       *Manager
	store     *Store
	logger    loggerLike
	interval  time.Duration
	threshold time.Duration
}

// NewRefreshWorker builds a worker. Zero values for interval/threshold
// fall back to defaults (5m / 15m).
func NewRefreshWorker(mgr *Manager, store *Store, logger loggerLike, interval, threshold time.Duration) *RefreshWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if threshold <= 0 {
		threshold = 15 * time.Minute
	}
	if logger == nil {
		logger = silentLogger{}
	}
	return &RefreshWorker{
		mgr:       mgr,
		store:     store,
		logger:    logger,
		interval:  interval,
		threshold: threshold,
	}
}

// Run blocks the calling goroutine; callers typically `go w.Run(ctx)`
// from main.go. Returns when ctx is canceled. An immediate first pass
// fires before the ticker so booting Argus near a token expiry doesn't
// wait a full interval before the first refresh attempt.
func (w *RefreshWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

// tick is one cycle. Exposed for tests so the time-loop can stay out
// of the unit tests.
func (w *RefreshWorker) tick(ctx context.Context) {
	cutoff := time.Now().Add(w.threshold).Unix()
	conns, err := w.store.ListExpiringSoon(ctx, cutoff)
	if err != nil {
		w.logger.Warn("workspace: refresh worker: list failed", "err", err.Error())
		return
	}
	for _, c := range conns {
		if err := w.mgr.RefreshIfStale(ctx, c.ID, w.threshold); err != nil {
			// Don't log the token itself or any header — RefreshIfStale's
			// error string is already user-safe.
			w.logger.Warn("workspace: refresh worker: refresh failed",
				"connection_id", c.ID,
				"service", string(c.Service),
				"err", err.Error())
			continue
		}
	}
}

// silentLogger is the package-internal fallback. Mirrors the one in
// slack_events.go but kept private so tests can pass nil and still get
// quiet behaviour.
type silentLogger struct{}

func (silentLogger) Info(msg string, args ...any) {}
func (silentLogger) Warn(msg string, args ...any) {}
