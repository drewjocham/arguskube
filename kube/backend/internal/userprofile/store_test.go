package userprofile

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// newTestDB spins up an in-memory SQLite with the user_activity +
// mute + suggestion-log tables. We deliberately don't run the full
// sqlitedb.Open() migration set — we only need the tables this package
// owns, and the smaller schema makes tests faster and self-contained.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	schema := []string{
		`CREATE TABLE user_activity (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			ts        INTEGER NOT NULL,
			kind      TEXT NOT NULL DEFAULT 'nav',
			view_id   TEXT NOT NULL DEFAULT '',
			context   TEXT NOT NULL DEFAULT '',
			namespace TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE user_profile_mutes (
			mute_key TEXT PRIMARY KEY,
			muted_at INTEGER NOT NULL
		)`,
		`CREATE TABLE user_suggestion_log (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			mute_key   TEXT NOT NULL DEFAULT '',
			kind       TEXT NOT NULL DEFAULT '',
			outcome    TEXT NOT NULL DEFAULT 'shown',
			created_at INTEGER NOT NULL
		)`,
	}
	for _, s := range schema {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestStore_RecordNav_RejectsEmptyViewID(t *testing.T) {
	s := NewStore(newTestDB(t), discardLogger())
	if err := s.RecordNav(context.Background(), "", "", ""); err == nil {
		t.Errorf("expected error for empty viewID")
	}
}

func TestStore_RecordNav_Persists(t *testing.T) {
	s := NewStore(newTestDB(t), discardLogger())
	ctx := context.Background()
	for _, v := range []string{"alerts", "pods", "alerts", "logs"} {
		if err := s.RecordNav(ctx, v, "prod", ""); err != nil {
			t.Fatalf("record %s: %v", v, err)
		}
	}
	got, err := s.Recent(ctx, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("want 4 rows, got %d", len(got))
	}
	if got[0].ViewID != "alerts" || got[3].ViewID != "logs" {
		t.Errorf("chronological order broken: %+v", got)
	}
}

func TestStore_PruneKeepsRingBufferBounded(t *testing.T) {
	db := newTestDB(t)
	s := NewStore(db, discardLogger())
	ctx := context.Background()
	// Insert MaxRetainedActivity + 50 rows; only the last MaxRetainedActivity
	// should survive. Also regression-locks the SQL performance: the prune
	// query must be O(log N), not O(N) — see pruneOld's comment for the
	// 120s CI timeout this used to hit.
	for i := 0; i < MaxRetainedActivity+50; i++ {
		if err := s.RecordNav(ctx, "v", "", ""); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_activity").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != MaxRetainedActivity {
		t.Errorf("retention sweep failed: want %d rows, got %d", MaxRetainedActivity, n)
	}
}

// Lock the off-by-one boundary. With exactly MaxRetainedActivity rows
// the prune is a no-op; with one more we delete exactly one row.
func TestStore_PruneBoundary(t *testing.T) {
	db := newTestDB(t)
	s := NewStore(db, discardLogger())
	ctx := context.Background()

	for i := 0; i < MaxRetainedActivity; i++ {
		if err := s.RecordNav(ctx, "v", "", ""); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	var n int
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_activity").Scan(&n)
	if n != MaxRetainedActivity {
		t.Fatalf("at cap: want %d, got %d", MaxRetainedActivity, n)
	}

	if err := s.RecordNav(ctx, "v", "", ""); err != nil {
		t.Fatalf("over-cap record: %v", err)
	}
	_ = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_activity").Scan(&n)
	if n != MaxRetainedActivity {
		t.Errorf("after +1: want %d (one row pruned), got %d", MaxRetainedActivity, n)
	}
}

func TestStore_Mute_IsIdempotent(t *testing.T) {
	s := NewStore(newTestDB(t), discardLogger())
	ctx := context.Background()
	if err := s.Mute(ctx, "k"); err != nil {
		t.Fatalf("mute: %v", err)
	}
	if err := s.Mute(ctx, "k"); err != nil {
		t.Fatalf("mute again: %v", err)
	}
	muted, err := s.IsMuted(ctx, "k")
	if err != nil || !muted {
		t.Errorf("want muted true, got %v %v", muted, err)
	}
	notMuted, _ := s.IsMuted(ctx, "other")
	if notMuted {
		t.Errorf("'other' should not be muted")
	}
}

func TestStore_Mute_RejectsEmptyKey(t *testing.T) {
	s := NewStore(newTestDB(t), discardLogger())
	if err := s.Mute(context.Background(), ""); err == nil {
		t.Errorf("expected error for empty mute key")
	}
}

func TestStore_OutcomeHelpersWriteRows(t *testing.T) {
	s := NewStore(newTestDB(t), discardLogger())
	ctx := context.Background()
	_ = s.RecordShown(ctx, "k", "kind")
	_ = s.RecordAccepted(ctx, "k", "kind")
	_ = s.RecordDismissed(ctx, "k", "kind")

	shown, err := s.CountShownSince(ctx, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("count shown: %v", err)
	}
	if shown != 1 {
		t.Errorf("want 1 shown row, got %d", shown)
	}
}

func TestStore_MuteRateSince(t *testing.T) {
	s := NewStore(newTestDB(t), discardLogger())
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = s.RecordShown(ctx, "k", "kind")
	}
	for i := 0; i < 2; i++ {
		_ = s.Mute(ctx, "k") // Mute itself records an OutcomeMuted row
	}

	shown, muted, err := s.MuteRateSince(ctx, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("mute rate: %v", err)
	}
	if shown != 3 || muted != 2 {
		t.Errorf("want 3 shown / 2 muted, got %d / %d", shown, muted)
	}
}

func TestStore_ClearActivity_KeepsMutes(t *testing.T) {
	s := NewStore(newTestDB(t), discardLogger())
	ctx := context.Background()
	_ = s.RecordNav(ctx, "alerts", "prod", "")
	_ = s.RecordShown(ctx, "k", "kind")
	_ = s.Mute(ctx, "k")

	if err := s.ClearActivity(ctx); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, _ := s.Recent(ctx, 10)
	if len(got) != 0 {
		t.Errorf("activity should be empty after clear, got %d rows", len(got))
	}
	shown, _ := s.CountShownSince(ctx, time.Unix(0, 0))
	if shown != 0 {
		t.Errorf("suggestion log should be empty, got %d", shown)
	}
	muted, _ := s.IsMuted(ctx, "k")
	if !muted {
		t.Errorf("mute should survive ClearActivity")
	}
}
