package connector

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/argues/argus/internal/dbconfig"
	"github.com/argues/argus/internal/secretstore"
)

// Real DB driver smoke test: open a SQLite file via the registered
// "sqlite" driver and verify caching + invalidation on config edit. We
// use SQLite because pgx requires a live Postgres; the cache behavior
// is the same regardless of dialect.

func newStore(t *testing.T) *dbconfig.Store {
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
	return dbconfig.NewStore(db, dbconfig.NewCrypto(secretstore.NewMemoryStore()))
}

func TestPool_GetAndCache(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	dbPath := filepath.Join(t.TempDir(), "target.db")
	saved, err := store.Upsert(ctx, dbconfig.DBConnection{
		Name: "local-sqlite", DBType: dbconfig.DBSQLite, DBName: dbPath, Enabled: true,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	pool := New(store, time.Minute, 1)
	t.Cleanup(func() { pool.CloseAll() })

	db1, _, err := pool.Get(ctx, saved.ID)
	if err != nil {
		t.Fatalf("first get: %v", err)
	}
	db2, _, err := pool.Get(ctx, saved.ID)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}
	if db1 != db2 {
		t.Fatalf("cache miss: pool returned different *sql.DB instances")
	}
}

func TestPool_InvalidatesOnConfigEdit(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	dbPath := filepath.Join(t.TempDir(), "target.db")
	saved, _ := store.Upsert(ctx, dbconfig.DBConnection{
		Name: "x", DBType: dbconfig.DBSQLite, DBName: dbPath, Enabled: true,
	})

	pool := New(store, time.Minute, 1)
	t.Cleanup(func() { pool.CloseAll() })

	first, _, _ := pool.Get(ctx, saved.ID)
	// Force a config edit (different timestamp).
	time.Sleep(1100 * time.Millisecond) // sqlite stores second precision
	saved.PoolSize = 5
	if _, err := store.Upsert(ctx, saved); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	second, _, err := pool.Get(ctx, saved.ID)
	if err != nil {
		t.Fatalf("get after edit: %v", err)
	}
	if first == second {
		t.Fatalf("pool reused cached DB after config edit; should have rebuilt")
	}
}

func TestPool_RejectsDisabled(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	saved, _ := store.Upsert(ctx, dbconfig.DBConnection{
		Name: "x", DBType: dbconfig.DBSQLite,
		DBName:  filepath.Join(t.TempDir(), "x.db"),
		Enabled: false,
	})
	pool := New(store, time.Minute, 1)
	t.Cleanup(func() { pool.CloseAll() })
	if _, _, err := pool.Get(ctx, saved.ID); err == nil {
		t.Fatalf("expected error on disabled connection")
	}
}
