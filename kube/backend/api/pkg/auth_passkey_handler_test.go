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
	"time"

	"github.com/argues/argus/internal/auth"
	"github.com/argues/argus/internal/config"
	"github.com/argues/argus/internal/sqlitedb"
)

// newAppWithPasskey wires the auth subsystem with passkeys enabled
// against the standard test RP (localhost / http://localhost:8080).
func newAppWithPasskey(t *testing.T) (*App, *auth.Store) {
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
		PasskeyEnabled:   true,
		PasskeyRPID:      "localhost",
		PasskeyRPName:    "Argus",
		PasskeyRPOrigin:  "http://localhost:8080",
	})
	return a, store
}

func TestAuthProviders_AdvertisesPasskeyFlag(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a, _ := newAppWithPasskey(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/providers", nil)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handleAuthProviders(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["passkeyEnabled"] != true {
		t.Errorf("passkeyEnabled should be true; got %v", resp["passkeyEnabled"])
	}
}

func TestAuthProviders_PasskeyDisabledWhenFlagOff(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a := newAppWithAuth(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/providers", nil)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handleAuthProviders(rec, req)
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["passkeyEnabled"] != false {
		t.Errorf("passkeyEnabled should be false; got %v", resp["passkeyEnabled"])
	}
}

func TestPasskeyLoginBegin_ReturnsOptions(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a, _ := newAppWithPasskey(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/passkey/login/begin", nil)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handlePasskeyLoginBegin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["state"] == nil || resp["state"] == "" {
		t.Error("missing state in response")
	}
	if resp["publicKey"] == nil {
		t.Error("missing publicKey in response")
	}
}

func TestPasskeyEndpoints_DisabledWhenFeatureOff(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a := newAppWithAuth(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/passkey/login/begin", nil)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handlePasskeyLoginBegin(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 when passkey feature disabled; got %d", rec.Code)
	}
}

func TestPasskeyRegisterBegin_RequiresSession(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a, _ := newAppWithPasskey(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/passkey/register/begin", nil)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handlePasskeyRegisterBegin(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session; got %d", rec.Code)
	}
}

func TestPasskeyRegisterBegin_HappyPath(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a, store := newAppWithPasskey(t)
	// Create a user + session via the underlying store, since the
	// register handler requires an authenticated caller.
	u, err := store.CreateLocalUser("user@example.com", "User", "correcthorsebattery!")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := store.CreateSession(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/passkey/register/begin", nil)
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handlePasskeyRegisterBegin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["state"] == nil {
		t.Error("missing state in response")
	}
}

func TestPasskeyList_RequiresSession(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a, _ := newAppWithPasskey(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/passkey/list", nil)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handlePasskeyList(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session; got %d", rec.Code)
	}
}

func TestPasskeyList_EmptyForNewUser(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a, store := newAppWithPasskey(t)
	u, _ := store.CreateLocalUser("u@x.com", "", "correcthorsebattery!")
	sess, _ := store.CreateSession(u.ID)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth/passkey/list", nil)
	req.Header.Set("Authorization", "Bearer "+sess.Token)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handlePasskeyList(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	creds, _ := resp["credentials"].([]any)
	if len(creds) != 0 {
		t.Errorf("expected empty list, got %d", len(creds))
	}
}

func TestPasskeyLoginFinish_RejectsBogusState(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a, _ := newAppWithPasskey(t)
	body, _ := json.Marshal(map[string]any{
		"state":      "does-not-exist",
		"credential": json.RawMessage(`{}`),
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/passkey/login/finish", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5173"
	a.handlePasskeyLoginFinish(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown state; got %d", rec.Code)
	}
}

func TestPasskeyDelete_RequiresSession(t *testing.T) {
	withEnv(t, "ARGUS_API_ALLOWED_ORIGINS=")
	a, _ := newAppWithPasskey(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/auth/passkey/123", nil)
	req.RemoteAddr = "127.0.0.1:5173"
	a.handlePasskeyDelete(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without session; got %d", rec.Code)
	}
}

func TestPasskeyStoreSQL_CRUD(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := sqlitedb.Open(t.TempDir(), logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store := auth.NewStore(db.DB, logger)
	u, err := store.CreateLocalUser("u@x.com", "U", "correcthorsebattery!")
	if err != nil {
		t.Fatal(err)
	}
	ps := store.PasskeyStore()

	// Insert
	c := auth.StoredCredential{
		UserID:       u.ID,
		CredentialID: []byte("cred-1"),
		PublicKey:    []byte("pk"),
		SignCount:    0,
		Transports:   []string{"internal", "hybrid"},
		AAGUID:       []byte("aaguid"),
		Name:         "My Key",
		CreatedAt:    time.Now(),
	}
	if err := ps.InsertCredential(c); err != nil {
		t.Fatalf("InsertCredential: %v", err)
	}
	// Duplicate -> error
	if err := ps.InsertCredential(c); err == nil {
		t.Error("expected duplicate-credential error")
	}

	// List
	list, err := ps.ListCredentialsForUser(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 credential; got %d", len(list))
	}
	if list[0].Name != "My Key" || len(list[0].Transports) != 2 {
		t.Errorf("round-trip data lost: %+v", list[0])
	}

	// UserByCredentialID
	got, err := ps.UserByCredentialID([]byte("cred-1"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID {
		t.Errorf("wrong user: %v", got.ID)
	}

	// UpdateCredentialUsage
	if err := ps.UpdateCredentialUsage([]byte("cred-1"), 42, time.Now()); err != nil {
		t.Fatal(err)
	}
	list, _ = ps.ListCredentialsForUser(u.ID)
	if list[0].SignCount != 42 {
		t.Errorf("sign_count not updated: %d", list[0].SignCount)
	}

	// Delete
	if err := ps.DeleteCredential(u.ID, list[0].ID); err != nil {
		t.Fatal(err)
	}
	list, _ = ps.ListCredentialsForUser(u.ID)
	if len(list) != 0 {
		t.Error("credential not deleted")
	}

	// Ceremony round-trip
	if err := ps.SaveCeremony("st-1", u.ID, []byte(`{"x":1}`), time.Now().Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	uid, data, err := ps.LoadCeremony("st-1")
	if err != nil {
		t.Fatal(err)
	}
	if uid != u.ID || string(data) != `{"x":1}` {
		t.Errorf("ceremony round-trip lost data: uid=%q data=%s", uid, string(data))
	}
	if err := ps.DeleteCeremony("st-1"); err != nil {
		t.Fatal(err)
	}
}

