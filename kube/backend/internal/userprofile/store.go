// Package userprofile implements the §6 "learning agents" surface:
//
//   - Observers record every navigation the user makes into a small
//     SQLite log (kind='nav' today, action/incident kinds reserved).
//   - The Suggester mines that log into one — and exactly one — actionable
//     "Argus suggests" card at a time, with hard annoyance limits.
//   - Persistent mute keys and a suggestion audit trail enforce those
//     limits across restarts.
//
// Privacy is load-bearing: every byte produced by this package stays in
// the user's local argus.db. No telemetry, no remote sync. The
// ClearActivity() method gives the user a single, irrevocable wipe.
//
// The package is split:
//
//   * Store      — thin SQL wrapper, NO suggestion logic.
//   * Suggester  — pure-ish profiler that reads from the store and
//                  emits Suggestion structs.
//
// Tests cover both layers separately; the store uses an in-memory
// SQLite DSN so it never touches the user's filesystem.
package userprofile

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// MaxRetainedActivity caps the user_activity table. The suggester only
// looks back a few hundred rows in practice, so keeping ~5000 is plenty
// for the morning-playbook and next-view detection while bounding disk.
const MaxRetainedActivity = 5000

// Kind enumerates the rows the store can hold. Frozen here so the
// frontend and the suggester agree on the wire shape.
const (
	KindNav     = "nav"
	KindAction  = "action"   // reserved for §6.2 action-observer
	KindIncident = "incident" // reserved for §6.3 incident-observer
)

// Activity is one observation. Keep small — written on every nav.
type Activity struct {
	ID        int64
	Ts        time.Time
	Kind      string
	ViewID    string
	Context   string
	Namespace string
}

// SuggestionOutcome describes what happened to a shown suggestion. The
// self-throttle uses these to compute "muted >50% in 14d".
const (
	OutcomeShown    = "shown"
	OutcomeAccepted = "accepted"
	OutcomeMuted    = "muted"
	OutcomeDismissed = "dismissed"
)

// Store is the SQLite-backed activity log. Construction is a one-liner
// off the existing *sql.DB — the package never owns the connection.
type Store struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewStore(db *sql.DB, logger *slog.Logger) *Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &Store{db: db, logger: logger}
}

// RecordNav inserts a nav-observation row. Empty viewID is rejected so
// we never accumulate junk rows the suggester has to filter out.
func (s *Store) RecordNav(ctx context.Context, viewID, kubeCtx, namespace string) error {
	if viewID == "" {
		return errors.New("viewID required")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_activity (ts, kind, view_id, context, namespace) VALUES (?, ?, ?, ?, ?)`,
		time.Now().Unix(), KindNav, viewID, kubeCtx, namespace,
	)
	if err != nil {
		return fmt.Errorf("insert activity: %w", err)
	}
	// Best-effort retention sweep. We don't fail the insert on cleanup
	// failure — the user shouldn't lose their fresh observation because
	// the prune lost a race.
	if err := s.pruneOld(ctx); err != nil {
		s.logger.Warn("userprofile: retention sweep failed", "err", err)
	}
	return nil
}

// pruneOld keeps the table bounded at MaxRetainedActivity by deleting
// rows older than the (MaxRetainedActivity+1)-th most-recent one.
func (s *Store) pruneOld(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM user_activity
		WHERE id NOT IN (
			SELECT id FROM user_activity ORDER BY id DESC LIMIT ?
		)`, MaxRetainedActivity)
	return err
}

// Recent returns the most-recent N nav rows in chronological order
// (oldest → newest). Chronological because every consumer (suggester,
// debug UI) wants to reason about sequence.
func (s *Store) Recent(ctx context.Context, limit int) ([]Activity, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, ts, kind, view_id, context, namespace
		   FROM user_activity
		   ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("select activity: %w", err)
	}
	defer rows.Close()

	out := make([]Activity, 0, limit)
	for rows.Next() {
		var a Activity
		var tsUnix int64
		if err := rows.Scan(&a.ID, &tsUnix, &a.Kind, &a.ViewID, &a.Context, &a.Namespace); err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}
		a.Ts = time.Unix(tsUnix, 0)
		out = append(out, a)
	}
	// Reverse so the caller gets oldest-first.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// Mute records a "don't ask again" decision. Idempotent.
func (s *Store) Mute(ctx context.Context, key string) error {
	if key == "" {
		return errors.New("mute key required")
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO user_profile_mutes (mute_key, muted_at) VALUES (?, ?)`,
		key, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("insert mute: %w", err)
	}
	return s.recordOutcome(ctx, key, "", OutcomeMuted)
}

// IsMuted reports whether the user has previously muted this key.
func (s *Store) IsMuted(ctx context.Context, key string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_profile_mutes WHERE mute_key = ?`, key).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("count mute: %w", err)
	}
	return n > 0, nil
}

// recordOutcome appends one row to the suggestion log. Used by the
// suggester after every show + by the public Mute/Accept/Dismiss APIs.
func (s *Store) recordOutcome(ctx context.Context, muteKey, kind, outcome string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_suggestion_log (mute_key, kind, outcome, created_at) VALUES (?, ?, ?, ?)`,
		muteKey, kind, outcome, time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("insert suggestion log: %w", err)
	}
	return nil
}

// RecordShown / RecordAccepted / RecordDismissed are the public outcome
// helpers consumed by the Wails layer. Each is a one-line wrapper.
func (s *Store) RecordShown(ctx context.Context, muteKey, kind string) error {
	return s.recordOutcome(ctx, muteKey, kind, OutcomeShown)
}
func (s *Store) RecordAccepted(ctx context.Context, muteKey, kind string) error {
	return s.recordOutcome(ctx, muteKey, kind, OutcomeAccepted)
}
func (s *Store) RecordDismissed(ctx context.Context, muteKey, kind string) error {
	return s.recordOutcome(ctx, muteKey, kind, OutcomeDismissed)
}

// CountShownSince returns how many suggestions were SHOWN to the user
// since the cutoff. Drives the 3/day cap.
func (s *Store) CountShownSince(ctx context.Context, since time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM user_suggestion_log WHERE outcome = ? AND created_at >= ?`,
		OutcomeShown, since.Unix(),
	).Scan(&n)
	return n, err
}

// MuteRateSince returns muted/shown over the lookback. Drives the
// auto-self-throttle (silence for a week if >50% over 14 days).
func (s *Store) MuteRateSince(ctx context.Context, since time.Time) (shown int, muted int, err error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT outcome, COUNT(*) FROM user_suggestion_log
		   WHERE created_at >= ? AND outcome IN (?, ?)
		   GROUP BY outcome`, since.Unix(), OutcomeShown, OutcomeMuted)
	if err != nil {
		return 0, 0, fmt.Errorf("group log: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var out string
		var n int
		if err := rows.Scan(&out, &n); err != nil {
			return 0, 0, fmt.Errorf("scan log group: %w", err)
		}
		switch out {
		case OutcomeShown:
			shown = n
		case OutcomeMuted:
			muted = n
		}
	}
	return shown, muted, rows.Err()
}

// ClearActivity is the user-initiated wipe surfaced as "Forget my
// activity" in Settings. Drops every observation row + every
// suggestion-log row but keeps mutes — a user who's muted a noisy card
// probably still wants the mute to persist across a wipe.
func (s *Store) ClearActivity(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM user_activity`); err != nil {
		return fmt.Errorf("delete user_activity: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM user_suggestion_log`); err != nil {
		return fmt.Errorf("delete user_suggestion_log: %w", err)
	}
	return nil
}
