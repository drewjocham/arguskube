package pkg

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argues/kube-watcher/internal/auth"
	"github.com/argues/kube-watcher/internal/config"
	"github.com/argues/kube-watcher/internal/sqlitedb"
)

// newAppWithAuth wires a minimal App + auth subsystem against a fresh
// SQLite DB. We don't need any of the cluster/AI dependencies for
// these tests — only the HTTP routing surface.
func newAppWithAuth(t *testing.T) *App {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := sqlitedb.Open(t.TempDir(), logger)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	a := &App{logger: logger, ctx: context.Background()}
	store := auth.NewStore(db.DB, logger)
	a.SetupAuth(store, config.AuthConfig{
		AllowLocalSignup: true,
	})
	return a
}

func mustJSON(t *testing.T, body io.Reader) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return m
}

func TestAuthRegister_HappyPath(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := newAppWithAuth(t)
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"name":     "User",
		"password": "correcthorsebattery!",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5173"
	a.handleAuthRegister(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	resp := mustJSON(t, rec.Body)
	if _, ok := resp["token"].(string); !ok {
		t.Error("response missing token")
	}
}

func TestAuthRegister_Disabled(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := sqlitedb.Open(t.TempDir(), logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	a := &App{logger: logger, ctx: context.Background()}
	a.SetupAuth(auth.NewStore(db.DB, logger), config.AuthConfig{AllowLocalSignup: false})

	body, _ := json.Marshal(map[string]string{
		"email": "u@x.com", "password": "correcthorsebattery!",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5173"
	a.handleAuthRegister(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 when signup is off; got %d", rec.Code)
	}
}

func TestAuthLogin_HappyPath(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := newAppWithAuth(t)
	if _, err := a.auth.store.CreateLocalUser("user@x.com", "User", "correcthorsebattery!"); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"email": "user@x.com", "password": "correcthorsebattery!"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5173"
	a.handleAuthLogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	resp := mustJSON(t, rec.Body)
	tok, _ := resp["token"].(string)
	if tok == "" {
		t.Error("missing token")
	}
}

func TestAuthLogin_WrongPassword(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := newAppWithAuth(t)
	if _, err := a.auth.store.CreateLocalUser("user@x.com", "User", "correcthorsebattery!"); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"email": "user@x.com", "password": "wrongwrongwrong"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5173"
	a.handleAuthLogin(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password; got %d", rec.Code)
	}
}

func TestServeHTTP_AcceptsValidSessionToken(t *testing.T) {
	// End-to-end: a fresh registration produces a token that gets
	// past authorizeAPIRequest, so the user can hit /api/* without
	// the static service token. This is the actual happy path the
	// frontend uses.
	withEnv(t, "KUBEWATCHER_API_TOKEN=", "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := newAppWithAuth(t)
	user, err := a.auth.store.CreateLocalUser("u@x.com", "U", "correcthorsebattery!")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := a.auth.store.CreateSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/GetClusterInfo", bytes.NewReader([]byte(`{"args":[]}`)))
	req.RemoteAddr = "127.0.0.1:5173"
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	a.ServeHTTP(rec, req)
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Errorf("session token should pass auth gates; got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestServeHTTP_RejectsRevokedToken(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=", "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := newAppWithAuth(t)
	user, _ := a.auth.store.CreateLocalUser("u@x.com", "U", "correcthorsebattery!")
	sess, _ := a.auth.store.CreateSession(user.ID)
	if err := a.auth.store.RevokeSession(sess.Token); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/GetClusterInfo", bytes.NewReader([]byte(`{"args":[]}`)))
	req.RemoteAddr = "127.0.0.1:5173"
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	a.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked token should be 401; got %d", rec.Code)
	}
}

func TestAuthMe_RequiresValidSession(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := newAppWithAuth(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handleAuthMe(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session token; got %d", rec.Code)
	}
}

func TestAuthLogout_RevokesSession(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := newAppWithAuth(t)
	user, _ := a.auth.store.CreateLocalUser("u@x.com", "U", "correcthorsebattery!")
	sess, _ := a.auth.store.CreateSession(user.ID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.RemoteAddr = "127.0.0.1:5173"
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	a.handleAuthLogout(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := a.auth.store.ValidateSession(sess.Token); err == nil {
		t.Error("session should be revoked after /auth/logout")
	}
}
