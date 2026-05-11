package auth

import (
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/argues/argus/internal/sqlitedb"
)

// helper to spin up an isolated DB per test. Sqlite-on-disk is fine
// here — t.TempDir cleans up automatically.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := sqlitedb.Open(dir, logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	_ = filepath.Clean(dir)
	return NewStore(db.DB, logger)
}

func TestCreateLocalUser_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	u, err := s.CreateLocalUser("Alice@Example.com", "Alice", "correcthorsebattery")
	if err != nil {
		t.Fatalf("CreateLocalUser: %v", err)
	}
	if u.Email != "alice@example.com" {
		t.Errorf("email should be lowercased; got %q", u.Email)
	}
	if u.Provider != ProviderLocal {
		t.Errorf("provider = %q, want local", u.Provider)
	}
	if u.ID == "" {
		t.Error("ID was not assigned")
	}
}

func TestCreateLocalUser_DuplicateEmail(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateLocalUser("a@b.com", "", "correcthorsebattery"); err != nil {
		t.Fatal(err)
	}
	_, err := s.CreateLocalUser("a@b.com", "", "anotherlongpass!")
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

func TestCreateLocalUser_RejectsShortPassword(t *testing.T) {
	s := newTestStore(t)
	_, err := s.CreateLocalUser("a@b.com", "", "short")
	if !errors.Is(err, ErrWeakPassword) {
		t.Errorf("expected ErrWeakPassword, got %v", err)
	}
}

func TestCreateLocalUser_RejectsBadEmail(t *testing.T) {
	s := newTestStore(t)
	for _, email := range []string{"", "noatsign", "@nope", "trailing@", "with space@x.com"} {
		_, err := s.CreateLocalUser(email, "", "correcthorsebattery")
		if !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("email %q: expected ErrInvalidEmail, got %v", email, err)
		}
	}
}

func TestAuthenticateLocal_HappyPath(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateLocalUser("user@x.com", "User", "correcthorsebattery"); err != nil {
		t.Fatal(err)
	}
	u, err := s.AuthenticateLocal("USER@x.com", "correcthorsebattery") // case-insensitive
	if err != nil {
		t.Fatalf("AuthenticateLocal: %v", err)
	}
	if u.Email != "user@x.com" {
		t.Errorf("got %q", u.Email)
	}
}

func TestAuthenticateLocal_WrongPassword(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateLocalUser("user@x.com", "", "correcthorsebattery"); err != nil {
		t.Fatal(err)
	}
	_, err := s.AuthenticateLocal("user@x.com", "wrongwrongwrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateLocal_UnknownEmailLooksIdenticalToWrongPassword(t *testing.T) {
	// Both paths must return the same sentinel — leaks otherwise let
	// an attacker enumerate accounts.
	s := newTestStore(t)
	_, err := s.AuthenticateLocal("ghost@x.com", "anything12345")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown email path returned %v, want ErrInvalidCredentials", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	s := newTestStore(t)
	u, err := s.CreateLocalUser("a@b.com", "", "correcthorsebattery")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := s.CreateSession(u.ID)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.Token == "" {
		t.Fatal("empty token")
	}
	got, err := s.ValidateSession(sess.Token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("validated user ID = %q, want %q", got.ID, u.ID)
	}
	if err := s.RevokeSession(sess.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ValidateSession(sess.Token); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("after revoke, expected ErrSessionInvalid, got %v", err)
	}
}

func TestValidateSession_RejectsEmptyAndUnknown(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ValidateSession(""); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("empty token: %v", err)
	}
	if _, err := s.ValidateSession("not-a-real-token"); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("unknown token: %v", err)
	}
}

func TestUpsertOAuthUser_CreatesThenUpdates(t *testing.T) {
	s := newTestStore(t)
	u1, err := s.UpsertOAuthUser(ProviderGoogle, "google-sub-12345", "alice@example.com", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if u1.Provider != ProviderGoogle {
		t.Errorf("provider = %q", u1.Provider)
	}
	// Same subject, updated name → same ID, name updated.
	u2, err := s.UpsertOAuthUser(ProviderGoogle, "google-sub-12345", "alice@example.com", "Alice Smith")
	if err != nil {
		t.Fatal(err)
	}
	if u2.ID != u1.ID {
		t.Errorf("subject collision should reuse ID; got %q vs %q", u2.ID, u1.ID)
	}
	if u2.Name != "Alice Smith" {
		t.Errorf("name not refreshed: %q", u2.Name)
	}
}

func TestUpsertOAuthUser_RejectsLocalProvider(t *testing.T) {
	s := newTestStore(t)
	_, err := s.UpsertOAuthUser(ProviderLocal, "x", "a@b.com", "")
	if err == nil {
		t.Error("UpsertOAuthUser must reject ProviderLocal — local accounts go through CreateLocalUser")
	}
}

func TestPasswordHash_RejectsEmpty(t *testing.T) {
	if verifyPassword("", "") {
		t.Error("empty hash + empty plain must NOT verify")
	}
	if verifyPassword("$2a$12$something", "") {
		t.Error("empty plain must NOT verify against any hash")
	}
}

func TestProvider_Valid(t *testing.T) {
	for _, ok := range []Provider{ProviderLocal, ProviderGoogle, ProviderOIDC} {
		if !ok.Valid() {
			t.Errorf("%q should be valid", ok)
		}
	}
	if Provider("attacker").Valid() {
		t.Error("unknown provider must NOT be valid — DB rows shouldn't be able to claim arbitrary methods")
	}
}
