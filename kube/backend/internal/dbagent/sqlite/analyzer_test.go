package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "modernc.org/sqlite"
)

// SQLite IS the test runtime, so every section can be exercised against
// a real DB. We open :memory:, create a table + index, and check each
// section returns the expected shape and non-zero values where applicable.

// openMem returns a :memory: SQLite with the single-writer pool config
// production uses (sqlitedb.Open pins MaxOpenConns to 1). Without this,
// a second goroutine opens a NEW empty :memory: DB and the analyzer's
// pragma_index_info() sub-query sees an empty catalog.
func openMem(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
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
	raw, ok := res["indexes"]
	if !ok {
		t.Fatalf("expected indexes key")
	}
	// Marshal/unmarshal so the test isn't coupled to the unexported
	// element type — but DOES exercise the analyzer's output rather
	// than just smoke-testing the dispatcher.
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := map[string]map[string]any{}
	for _, r := range rows {
		name, _ := r["index"].(string)
		got[name] = r
	}
	if _, ok := got["idx_t_name"]; !ok {
		t.Errorf("idx_t_name missing from analyzer output: %v", rows)
	}
	if r := got["idx_t_name_age"]; r == nil {
		t.Errorf("idx_t_name_age missing from analyzer output: %v", rows)
	} else {
		// Two-column composite index: pragma_index_info should report 2.
		if cols, _ := r["columns"].(float64); cols != 2 {
			t.Errorf("idx_t_name_age columns = %v, want 2", r["columns"])
		}
		if tbl, _ := r["table"].(string); tbl != "t" {
			t.Errorf("idx_t_name_age table = %q, want \"t\"", tbl)
		}
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
