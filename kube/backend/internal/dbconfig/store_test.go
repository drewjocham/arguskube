package dbconfig

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/argues/argus/internal/secretstore"
	_ "modernc.org/sqlite"
)

// openTestDB creates an in-memory SQLite with just the db_connections
// schema. We do NOT pull in sqlitedb.Open here because that would
// couple every dbconfig test to every migration in the system.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE db_connections (
		id            TEXT    PRIMARY KEY,
		name          TEXT    NOT NULL,
		db_type       TEXT    NOT NULL,
		host          TEXT    NOT NULL DEFAULT '',
		port          INTEGER NOT NULL DEFAULT 0,
		user_name     TEXT    NOT NULL DEFAULT '',
		password_enc  TEXT    NOT NULL DEFAULT '',
		db_name       TEXT    NOT NULL DEFAULT '',
		ssl_mode      TEXT    NOT NULL DEFAULT '',
		pool_size     INTEGER NOT NULL DEFAULT 0,
		tags          TEXT    NOT NULL DEFAULT '[]',
		enabled       INTEGER NOT NULL DEFAULT 1,
		created_at    INTEGER NOT NULL,
		updated_at    INTEGER NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	// Mirror sqlitedb migration #16 — the UNIQUE index that
	// ErrDuplicateName depends on.
	if _, err := db.Exec(`CREATE UNIQUE INDEX idx_db_connections_name ON db_connections(name)`); err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestStore(t *testing.T) *Store {
	return NewStore(openTestDB(t), NewCrypto(secretstore.NewMemoryStore()))
}

func TestStore_UpsertAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	in := DBConnection{
		Name: "prod-pg", DBType: DBPostgres,
		Host: "10.0.0.1", Port: 5432, User: "argus",
		Password: "s3cret", DBName: "app", SSLMode: SSLRequire,
		Tags: []string{"prod", "primary"}, Enabled: true,
	}
	saved, err := s.Upsert(ctx, in)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if saved.ID == "" {
		t.Fatalf("upsert did not assign an ID")
	}
	if saved.CreatedAt == 0 || saved.UpdatedAt == 0 {
		t.Fatalf("upsert did not stamp timestamps")
	}

	got, err := s.Get(ctx, saved.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Password != "s3cret" {
		t.Fatalf("password round-trip failed: %q", got.Password)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "prod" {
		t.Fatalf("tags lost: %v", got.Tags)
	}
}

func TestStore_PasswordIsEncryptedAtRest(t *testing.T) {
	db := openTestDB(t)
	s := NewStore(db, NewCrypto(secretstore.NewMemoryStore()))
	ctx := context.Background()

	_, err := s.Upsert(ctx, DBConnection{
		Name: "x", DBType: DBPostgres, Host: "h", Port: 5432,
		Password: "plaintext-leak",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var stored string
	if err := db.QueryRow(`SELECT password_enc FROM db_connections`).Scan(&stored); err != nil {
		t.Fatalf("read password_enc: %v", err)
	}
	if stored == "plaintext-leak" || stored == "" {
		t.Fatalf("password not encrypted at rest: %q", stored)
	}
}

func TestStore_UpsertUpdatesExisting(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	saved, _ := s.Upsert(ctx, DBConnection{
		Name: "a", DBType: DBPostgres, Host: "h", Port: 5432, Password: "p1",
	})
	saved.Password = "p2"
	saved.Host = "h2"
	updated, err := s.Upsert(ctx, saved)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ID != saved.ID {
		t.Fatalf("ID changed across update: %s -> %s", saved.ID, updated.ID)
	}
	if updated.CreatedAt != saved.CreatedAt {
		t.Fatalf("created_at must not change on update")
	}

	got, _ := s.Get(ctx, saved.ID)
	if got.Password != "p2" || got.Host != "h2" {
		t.Fatalf("update did not persist: %+v", got)
	}
}

func TestStore_List(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_, _ = s.Upsert(ctx, DBConnection{Name: "zeta", DBType: DBPostgres, Host: "h", Port: 5432})
	_, _ = s.Upsert(ctx, DBConnection{Name: "alpha", DBType: DBPostgres, Host: "h", Port: 5432})

	list, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].Name != "alpha" {
		t.Fatalf("list order/contents wrong: %+v", list)
	}
}

func TestStore_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	saved, _ := s.Upsert(ctx, DBConnection{Name: "a", DBType: DBPostgres, Host: "h", Port: 5432})
	if err := s.Delete(ctx, saved.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.Delete(ctx, saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing should be ErrNotFound, got %v", err)
	}
	if _, err := s.Get(ctx, saved.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete should be ErrNotFound, got %v", err)
	}
}

// TestStore_UpsertRejectsUnknownID guards the resurrection blocker the
// reviewer flagged: passing a non-empty ID that doesn't reference an
// existing row used to silently create a new row under the caller's ID,
// which let a deleted connection's ID become a path to a fresh row.
func TestStore_UpsertRejectsUnknownID(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_, err := s.Upsert(ctx, DBConnection{
		ID:   "stale-id-from-deleted-row",
		Name: "x", DBType: DBPostgres, Host: "h", Port: 5432,
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for unknown ID, got %v", err)
	}
}

// TestStore_DuplicateName maps the driver's UNIQUE constraint error
// onto the typed ErrDuplicateName sentinel so the UI can render a
// friendly message.
func TestStore_DuplicateName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.Upsert(ctx, DBConnection{
		Name: "prod", DBType: DBPostgres, Host: "h", Port: 5432,
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	_, err := s.Upsert(ctx, DBConnection{
		Name: "prod", DBType: DBPostgres, Host: "h2", Port: 5432,
	})
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName, got %v", err)
	}
}

func TestStore_RejectsInvalid(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cases := []DBConnection{
		{Name: "", DBType: DBPostgres, Host: "h", Port: 5432},
		{Name: "x", DBType: "mariadb", Host: "h", Port: 5432},
		{Name: "x", DBType: DBPostgres, Host: "", Port: 5432},
		{Name: "x", DBType: DBPostgres, Host: "h", Port: 0},
		{Name: "x", DBType: DBSQLite, DBName: ""},
	}
	for i, c := range cases {
		if _, err := s.Upsert(ctx, c); err == nil {
			t.Fatalf("case %d: expected validation error, got nil", i)
		}
	}
}
