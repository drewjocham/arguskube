package postgres

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// The analyzer queries pg_stat_* — SQLite obviously doesn't have those.
// We test the dispatch + section-name handling here; query content is
// covered by an integration test (skipped unless ARGUS_PG_TEST_DSN is
// set) so CI doesn't need a live Postgres.

func openMem(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestAnalyzer_UnknownSection(t *testing.T) {
	a := New(openMem(t), 0)
	if _, err := a.Analyze(context.Background(), "bogus"); err == nil {
		t.Fatalf("expected error for unknown section")
	}
}

func TestAnalyzer_DefaultsToOverview(t *testing.T) {
	// We can't run the actual queries against sqlite, but we can verify
	// the empty-section path takes the overview branch (it tolerates
	// per-section errors and returns a map).
	a := New(openMem(t), 0)
	res, err := a.Analyze(context.Background(), "")
	if err != nil {
		t.Fatalf("overview should swallow per-section errors: %v", err)
	}
	if _, ok := res["resources"]; !ok {
		t.Fatalf("overview should have a resources key")
	}
}
