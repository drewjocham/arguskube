package clickhouse

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// The analyzer queries system.* — SQLite obviously doesn't have those.
// We test dispatch + section-name handling here; query content is
// covered by integration tests run only against a live ClickHouse.

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
	a := New(openMem(t), 0)
	// overview tolerates per-section errors and returns a map shell.
	res, err := a.Analyze(context.Background(), "")
	if err != nil {
		t.Fatalf("overview should swallow per-section errors: %v", err)
	}
	for _, k := range []string{"resources", "connections", "replication"} {
		if _, ok := res[k]; !ok {
			t.Errorf("overview missing %q", k)
		}
	}
}
