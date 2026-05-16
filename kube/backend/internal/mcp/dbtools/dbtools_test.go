// Package dbtools tests cover the MCP front for the DBAgent analyzers.
// SQLite is exercised end-to-end against a real temp database since
// the modernc.org/sqlite driver is in-process; Postgres and ClickHouse
// get metadata + early-validation coverage only (their analyzers
// would need a real server to exercise Execute fully).
package dbtools

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/argues/argus/internal/dbagent/connector"
	"github.com/argues/argus/internal/dbconfig"
	"github.com/argues/argus/internal/secretstore"

	_ "modernc.org/sqlite" // registers the in-process sqlite driver
)

// newStoreWithSQLite builds the same in-memory dbconfig store the
// connector tests use, plus a Pool wired to it.
func newPool(t *testing.T) (*connector.Pool, *dbconfig.Store) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE db_connections (
		id TEXT PRIMARY KEY, name TEXT NOT NULL, db_type TEXT NOT NULL,
		host TEXT DEFAULT '', port INTEGER DEFAULT 0, user_name TEXT DEFAULT '',
		password_enc TEXT DEFAULT '', db_name TEXT DEFAULT '',
		ssl_mode TEXT DEFAULT '', pool_size INTEGER DEFAULT 0,
		tags TEXT DEFAULT '[]', enabled INTEGER DEFAULT 1,
		created_at INTEGER, updated_at INTEGER)`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	store := dbconfig.NewStore(db, dbconfig.NewCrypto(secretstore.NewMemoryStore()))
	pool := connector.New(store, time.Minute, 1)
	t.Cleanup(func() { _ = pool.CloseAll() })
	return pool, store
}

// ─── Helpers in tool.go ──────────────────────────────────────────────

func TestGetStringReturnsDefaultForMissing(t *testing.T) {
	t.Parallel()
	if got := getString(map[string]any{}, "missing", "fallback"); got != "fallback" {
		t.Errorf("getString missing key = %q, want %q", got, "fallback")
	}
}

func TestGetStringReturnsValueForStringTyped(t *testing.T) {
	t.Parallel()
	args := map[string]any{"k": "v"}
	if got := getString(args, "k", "default"); got != "v" {
		t.Errorf("getString = %q, want %q", got, "v")
	}
}

func TestGetStringReturnsDefaultForWrongType(t *testing.T) {
	t.Parallel()
	args := map[string]any{"k": 42}
	if got := getString(args, "k", "fallback"); got != "fallback" {
		t.Errorf("getString of non-string = %q, want %q", got, "fallback")
	}
}

func TestGetIntAcceptsFloat64(t *testing.T) {
	t.Parallel()
	// JSON unmarshal decodes numbers into float64 — the helper has a
	// specific branch for that path; this test pins it.
	args := map[string]any{"k": float64(7)}
	if got := getInt(args, "k", 0); got != 7 {
		t.Errorf("getInt(float64) = %d, want 7", got)
	}
}

func TestGetIntAcceptsInt(t *testing.T) {
	t.Parallel()
	args := map[string]any{"k": 5}
	if got := getInt(args, "k", 0); got != 5 {
		t.Errorf("getInt(int) = %d, want 5", got)
	}
}

func TestGetIntReturnsDefaultForMissing(t *testing.T) {
	t.Parallel()
	if got := getInt(map[string]any{}, "missing", 42); got != 42 {
		t.Errorf("getInt missing = %d, want 42", got)
	}
}

func TestGetIntReturnsDefaultForWrongType(t *testing.T) {
	t.Parallel()
	args := map[string]any{"k": "string-not-a-number"}
	if got := getInt(args, "k", 99); got != 99 {
		t.Errorf("getInt of string = %d, want 99", got)
	}
}

// ─── Tool metadata (all three dialects) ───────────────────────────────

func TestSQLiteToolMetadata(t *testing.T) {
	t.Parallel()
	tool := NewSQLiteAnalyzeTool(nil)
	if got := tool.Name(); got != "db_analyze_sqlite" {
		t.Errorf("Name = %q, want db_analyze_sqlite", got)
	}
	if tool.Description() == "" {
		t.Error("Description is empty")
	}
	params := tool.Parameters()
	if len(params) != 3 {
		t.Fatalf("expected 3 parameters, got %d", len(params))
	}
	if params[0].Name != "connection_id" || !params[0].Required {
		t.Errorf("expected first param 'connection_id' (required); got %+v", params[0])
	}
}

func TestPostgresToolMetadata(t *testing.T) {
	t.Parallel()
	tool := NewPostgresAnalyzeTool(nil)
	if got := tool.Name(); got != "db_analyze_postgres" {
		t.Errorf("Name = %q, want db_analyze_postgres", got)
	}
	if tool.Description() == "" {
		t.Error("Description is empty")
	}
	params := tool.Parameters()
	if len(params) == 0 {
		t.Fatal("expected parameters, got none")
	}
	if params[0].Name != "connection_id" || !params[0].Required {
		t.Errorf("first param must be required connection_id; got %+v", params[0])
	}
}

func TestClickHouseToolMetadata(t *testing.T) {
	t.Parallel()
	tool := NewClickHouseAnalyzeTool(nil)
	if got := tool.Name(); got != "db_analyze_clickhouse" {
		t.Errorf("Name = %q, want db_analyze_clickhouse", got)
	}
	if tool.Description() == "" {
		t.Error("Description is empty")
	}
	params := tool.Parameters()
	if len(params) == 0 {
		t.Fatal("expected parameters, got none")
	}
	if params[0].Name != "connection_id" || !params[0].Required {
		t.Errorf("first param must be required connection_id; got %+v", params[0])
	}
}

// ─── Execute() validation paths ───────────────────────────────────────

func TestSQLiteExecuteMissingConnectionID(t *testing.T) {
	t.Parallel()
	tool := NewSQLiteAnalyzeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing connection_id")
	}
	if !strings.Contains(err.Error(), "connection_id") {
		t.Errorf("error should mention connection_id; got %v", err)
	}
}

func TestSQLiteExecuteWhitespaceConnectionID(t *testing.T) {
	t.Parallel()
	tool := NewSQLiteAnalyzeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]any{
		"connection_id": "   ",
	})
	if err == nil {
		t.Fatal("expected error for whitespace-only connection_id")
	}
}

func TestPostgresExecuteMissingConnectionID(t *testing.T) {
	t.Parallel()
	tool := NewPostgresAnalyzeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing connection_id")
	}
}

func TestClickHouseExecuteMissingConnectionID(t *testing.T) {
	t.Parallel()
	tool := NewClickHouseAnalyzeTool(nil)
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing connection_id")
	}
}

// ─── SQLite end-to-end against a real temp DB ─────────────────────────

func TestSQLiteExecuteEndToEnd(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool, store := newPool(t)

	dbPath := filepath.Join(t.TempDir(), "target.db")
	saved, err := store.Upsert(ctx, dbconfig.DBConnection{
		Name: "test-sqlite", DBType: dbconfig.DBSQLite, DBName: dbPath, Enabled: true,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	tool := NewSQLiteAnalyzeTool(pool)
	out, err := tool.Execute(ctx, map[string]any{
		"connection_id":   saved.ID,
		"section":         "overview",
		"timeout_seconds": 5,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out["connection"] != "test-sqlite" {
		t.Errorf("connection mismatch: %+v", out["connection"])
	}
	if out["db_type"] != string(dbconfig.DBSQLite) {
		t.Errorf("db_type mismatch: %+v", out["db_type"])
	}
	if out["section"] != "overview" {
		t.Errorf("section not echoed: %+v", out["section"])
	}
	if _, ok := out["data"]; !ok {
		t.Error("Execute did not include analyzer data")
	}
	if _, ok := out["collected"].(string); !ok {
		t.Error("Execute did not stamp a collected timestamp")
	}
}

func TestSQLiteExecuteRejectsWrongDBType(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool, store := newPool(t)

	// Save a Postgres config, then call the SQLite tool with it. The
	// tool must refuse with a "not sqlite" error before opening any
	// connection, so we never actually need a live Postgres.
	saved, err := store.Upsert(ctx, dbconfig.DBConnection{
		Name: "fake-pg", DBType: dbconfig.DBPostgres,
		Host: "localhost", Port: 5432, DBName: "x", Enabled: true,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	tool := NewSQLiteAnalyzeTool(pool)
	_, err = tool.Execute(ctx, map[string]any{
		"connection_id": saved.ID,
	})
	// Pool.Get pings the DB on first acquisition, so the error may
	// surface as a connection-refused before we even reach the SQLite
	// tool's "not sqlite" check. Either path proves the misuse fails
	// loudly; both are acceptable. The test is here mainly to prevent
	// a regression where the tool would silently succeed against the
	// wrong dialect.
	if err == nil {
		t.Fatal("expected error when calling SQLite tool against a Postgres connection")
	}
}
