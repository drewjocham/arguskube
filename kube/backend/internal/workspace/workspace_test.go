package workspace

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"testing"

	"github.com/argues/argus/internal/secretstore"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE workspace_connections (
		id                     TEXT PRIMARY KEY,
		user_id                TEXT NOT NULL,
		service                TEXT NOT NULL,
		external_workspace_id  TEXT NOT NULL DEFAULT '',
		display_name           TEXT NOT NULL DEFAULT '',
		email                  TEXT NOT NULL DEFAULT '',
		avatar_url             TEXT NOT NULL DEFAULT '',
		connected_at           INTEGER NOT NULL,
		updated_at             INTEGER NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create connections: %v", err)
	}
	if _, err := db.Exec(`CREATE UNIQUE INDEX idx_workspace_conn_unique ON workspace_connections(user_id, service, external_workspace_id)`); err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE workspace_tokens (
		connection_id     TEXT PRIMARY KEY REFERENCES workspace_connections(id) ON DELETE CASCADE,
		access_token_enc  TEXT NOT NULL,
		refresh_token_enc TEXT NOT NULL DEFAULT '',
		token_type        TEXT NOT NULL DEFAULT 'bearer',
		expires_at        INTEGER NOT NULL DEFAULT 0,
		scope             TEXT NOT NULL DEFAULT '',
		updated_at        INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create tokens: %v", err)
	}
	// SQLite needs explicit pragma to honor foreign keys for ON DELETE CASCADE.
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("enable fk: %v", err)
	}
	return db
}

func newStore(t *testing.T) *Store {
	return NewStore(openTestDB(t), NewCrypto(secretstore.NewMemoryStore()))
}

func newManager(t *testing.T) *Manager {
	logger := slog.New(slog.NewTextHandler(testDiscard{}, nil))
	m := NewManager(newStore(t), logger)
	m.Register(NewTestProvider(ServiceGDocs))
	return m
}

// testDiscard sinks slog output so tests don't pollute stderr.
type testDiscard struct{}

func (testDiscard) Write(p []byte) (int, error) { return len(p), nil }

func TestCrypto_RoundTrip(t *testing.T) {
	c := NewCrypto(secretstore.NewMemoryStore())
	ctx := context.Background()
	enc, err := c.Encrypt(ctx, "xoxb-secret-token")
	if err != nil || enc == "" || enc == "xoxb-secret-token" {
		t.Fatalf("encrypt: %q err=%v", enc, err)
	}
	got, err := c.Decrypt(ctx, enc)
	if err != nil || got != "xoxb-secret-token" {
		t.Fatalf("decrypt: %q err=%v", got, err)
	}
}

func TestCrypto_EmptyPassthrough(t *testing.T) {
	c := NewCrypto(secretstore.NewMemoryStore())
	ctx := context.Background()
	if got, err := c.Encrypt(ctx, ""); err != nil || got != "" {
		t.Fatalf("empty encrypt: %q %v", got, err)
	}
	if got, err := c.Decrypt(ctx, ""); err != nil || got != "" {
		t.Fatalf("empty decrypt: %q %v", got, err)
	}
}

func TestManager_StartAndCompleteFlow(t *testing.T) {
	m := newManager(t)
	ctx := context.Background()

	auth, err := m.Start(ctx, "user-1", ServiceGDocs, "https://callback.example/cb")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if auth.State == "" || auth.URL == "" {
		t.Fatalf("auth url incomplete: %+v", auth)
	}

	c, err := m.Complete(ctx, ServiceGDocs, auth.State, "ok-code")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if c.UserID != "user-1" || c.Service != ServiceGDocs {
		t.Fatalf("connection has wrong identity: %+v", c)
	}

	tok, err := m.Token(ctx, c.ID)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if tok.AccessToken != "test-access-ok-code" {
		t.Fatalf("access token round-trip wrong: %q", tok.AccessToken)
	}
}

func TestManager_RejectsUnknownState(t *testing.T) {
	m := newManager(t)
	ctx := context.Background()
	_, err := m.Complete(ctx, ServiceGDocs, "never-issued-state", "code")
	if err == nil {
		t.Fatal("expected error for unknown state")
	}
}

func TestManager_ProviderFailurePropagates(t *testing.T) {
	m := newManager(t)
	ctx := context.Background()
	auth, _ := m.Start(ctx, "user-1", ServiceGDocs, "https://cb")
	_, err := m.Complete(ctx, ServiceGDocs, auth.State, "fail")
	if err == nil {
		t.Fatal("expected error from TestProvider when code=fail")
	}
}

func TestStore_Reauth_KeepsID(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	c := Connection{
		UserID: "u", Service: ServiceGDocs, ExternalWorkspaceID: "T1",
		DisplayName: "First",
	}
	tok := Token{AccessToken: "v1"}
	first, err := s.Upsert(ctx, c, tok)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	c2 := Connection{
		UserID: "u", Service: ServiceGDocs, ExternalWorkspaceID: "T1",
		DisplayName: "Updated",
	}
	tok2 := Token{AccessToken: "v2"}
	second, err := s.Upsert(ctx, c2, tok2)
	if err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("re-auth got a new ID: %s -> %s", first.ID, second.ID)
	}
	if second.DisplayName != "Updated" {
		t.Fatalf("display name not refreshed: %q", second.DisplayName)
	}
	loaded, _ := s.GetToken(ctx, second.ID)
	if loaded.AccessToken != "v2" {
		t.Fatalf("token not refreshed: %q", loaded.AccessToken)
	}
}

func TestStore_MultipleWorkspacesPerService(t *testing.T) {
	// The reviewer flagged this as a missing requirement in the
	// original design: a user CAN connect two accounts of the same
	// service (e.g. work + personal Google).
	s := newStore(t)
	ctx := context.Background()
	for _, ext := range []string{"T-prod", "T-corp"} {
		if _, err := s.Upsert(ctx, Connection{
			UserID: "u", Service: ServiceGDocs, ExternalWorkspaceID: ext, DisplayName: ext,
		}, Token{AccessToken: "tok"}); err != nil {
			t.Fatalf("upsert %s: %v", ext, err)
		}
	}
	list, err := s.List(ctx, "u")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(list))
	}
}

func TestStore_TokenIsEncryptedAtRest(t *testing.T) {
	db := openTestDB(t)
	s := NewStore(db, NewCrypto(secretstore.NewMemoryStore()))
	ctx := context.Background()
	c, err := s.Upsert(ctx, Connection{
		UserID: "u", Service: ServiceGDocs, ExternalWorkspaceID: "T",
	}, Token{AccessToken: "xoxb-plaintext-leak"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	var stored string
	if err := db.QueryRow(`SELECT access_token_enc FROM workspace_tokens WHERE connection_id = ?`, c.ID).Scan(&stored); err != nil {
		t.Fatalf("read stored token: %v", err)
	}
	if stored == "xoxb-plaintext-leak" || stored == "" {
		t.Fatalf("token not encrypted at rest: %q", stored)
	}
}

func TestStore_DeleteCascadesToken(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	c, _ := s.Upsert(ctx, Connection{
		UserID: "u", Service: ServiceGDocs, ExternalWorkspaceID: "T",
	}, Token{AccessToken: "v"})
	if err := s.Delete(ctx, c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetToken(ctx, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("token survived cascade: %v", err)
	}
	if err := s.Delete(ctx, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete should be ErrNotFound, got %v", err)
	}
}

func TestStore_RejectsUnsupportedService(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_, err := s.Upsert(ctx, Connection{
		UserID: "u", Service: "myspace", ExternalWorkspaceID: "x",
	}, Token{AccessToken: "v"})
	if !errors.Is(err, ErrInvalidService) {
		t.Fatalf("expected ErrInvalidService, got %v", err)
	}
}

func TestManager_AvailableServices(t *testing.T) {
	m := newManager(t)
	got := m.AvailableServices()
	if len(got) != 1 || got[0] != ServiceGDocs {
		t.Fatalf("expected [gdocs], got %v", got)
	}
	if !m.HasProvider(ServiceGDocs) {
		t.Fatal("HasProvider(gdocs) should be true")
	}
	if m.HasProvider(ServiceGSheets) {
		t.Fatal("HasProvider(gsheets) should be false when no provider is registered")
	}
}
