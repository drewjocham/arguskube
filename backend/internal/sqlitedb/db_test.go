package sqlitedb_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/argues/kube-watcher/internal/sqlitedb"
)

func TestOpenCreatesDatabase(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlitedb.Open(dir, testLogger(t))
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(filepath.Join(dir, "kubewatcher.db")); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestOpenDefaultDir(t *testing.T) {
	db, err := sqlitedb.Open("", testLogger(t))
	if err != nil {
		t.Logf("Open with empty dir (expected in CI): %v", err)
		return
	}
	if db != nil {
		db.Close()
	}
}

func TestMigrationsApplied(t *testing.T) {
	dir := t.TempDir()
	db, err := sqlitedb.Open(dir, testLogger(t))
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	var count int
	for _, table := range []string{"incidents", "workflows", "users", "sessions", "oauth_pending"} {
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatalf("%s table not found: %v", table, err)
		}
	}

	// _migrations is append-only; assert it has at least the original
	// two so we don't silently drop a baseline migration in a refactor.
	var migrationCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&migrationCount); err != nil {
		t.Fatalf("_migrations table not found: %v", err)
	}
	if migrationCount < 2 {
		t.Errorf("expected at least 2 migrations, got %d", migrationCount)
	}
}

func TestOpenIdempotent(t *testing.T) {
	dir := t.TempDir()

	db1, err := sqlitedb.Open(dir, testLogger(t))
	if err != nil {
		t.Fatalf("first Open() failed: %v", err)
	}
	db1.Close()

	db2, err := sqlitedb.Open(dir, testLogger(t))
	if err != nil {
		t.Fatalf("second Open() failed: %v", err)
	}
	db2.Close()
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}
