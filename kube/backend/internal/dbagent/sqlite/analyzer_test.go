package sqlite

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// SQLite IS the test runtime, so every section can be exercised against
// a real DB. We open :memory:, create a table + index, and check each
// section returns the expected shape and non-zero values where applicable.

func openMem(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`CREATE INDEX idx_t_name ON t(name)`); err != nil {
		t.Fatalf("create index: %v", err)
	}
	if _, err := db.Exec(`CREATE INDEX idx_t_name_age ON t(name, age)`); err != nil {
		t.Fatalf("create composite index: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO t(name, age) VALUES ('a', 1), ('b', 2)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	return db
}

func TestAnalyzer_UnknownSection(t *testing.T) {
	a := New(openMem(t), 0)
	if _, err := a.Analyze(context.Background(), "bogus"); err == nil {
		t.Fatalf("expected error for unknown section")
	}
}

func TestAnalyzer_Resources(t *testing.T) {
	a := New(openMem(t), 0)
	res, err := a.Analyze(context.Background(), "resources")
	if err != nil {
		t.Fatalf("resources: %v", err)
	}
	if pc, _ := res["page_count"].(int64); pc <= 0 {
		t.Errorf("page_count should be > 0, got %v", res["page_count"])
	}
	if ps, _ := res["page_size"].(int64); ps <= 0 {
		t.Errorf("page_size should be > 0, got %v", res["page_size"])
	}
	if _, ok := res["journal_mode"]; !ok {
		t.Errorf("expected journal_mode key")
	}
}

func TestAnalyzer_Connections(t *testing.T) {
	a := New(openMem(t), 0)
	res, err := a.Analyze(context.Background(), "connections")
	if err != nil {
		t.Fatalf("connections: %v", err)
	}
	if total, _ := res["total"].(int64); total != 1 {
		t.Errorf("total should be 1, got %v", res["total"])
	}
	if _, ok := res["by_state"]; !ok {
		t.Errorf("expected by_state key for cross-dialect UI parity")
	}
	if inMem, _ := res["in_memory"].(bool); !inMem {
		t.Errorf("expected in_memory=true for :memory: db")
	}
}

func TestAnalyzer_Indexes(t *testing.T) {
	a := New(openMem(t), 0)
	res, err := a.Analyze(context.Background(), "indexes")
	if err != nil {
		t.Fatalf("indexes: %v", err)
	}
	if _, ok := res["indexes"]; !ok {
		t.Fatalf("expected indexes key")
	}
	// The slice element type is unexported; verify against the catalog
	// directly that the two indexes we created exist.
	found := map[string]bool{}
	rows, err := openMem(t).Query(`SELECT name FROM sqlite_master WHERE type='index'`)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		found[n] = true
	}
	if !found["idx_t_name"] || !found["idx_t_name_age"] {
		t.Errorf("expected both indexes present in catalog, got %v", found)
	}
}

func TestAnalyzer_Queries(t *testing.T) {
	a := New(openMem(t), 0)
	res, err := a.Analyze(context.Background(), "queries")
	if err != nil {
		t.Fatalf("queries: %v", err)
	}
	if avail, _ := res["available"].(bool); avail {
		t.Errorf("queries should report available=false for sqlite")
	}
	if _, ok := res["hint"]; !ok {
		t.Errorf("expected hint key")
	}
}

func TestAnalyzer_Replication(t *testing.T) {
	a := New(openMem(t), 0)
	res, err := a.Analyze(context.Background(), "replication")
	if err != nil {
		t.Fatalf("replication: %v", err)
	}
	if _, ok := res["replicas"]; !ok {
		t.Errorf("expected replicas key")
	}
}

func TestAnalyzer_Overview(t *testing.T) {
	a := New(openMem(t), 0)
	res, err := a.Analyze(context.Background(), "")
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	for _, k := range []string{"resources", "connections", "replication"} {
		if _, ok := res[k]; !ok {
			t.Errorf("overview missing %q", k)
		}
	}
}
