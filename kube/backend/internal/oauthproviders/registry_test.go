package oauthproviders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGetPreset_KnownAndUnknown(t *testing.T) {
	p, ok := GetPreset(PresetGitHub)
	if !ok {
		t.Fatal("expected GitHub preset")
	}
	if p.AuthURL == "" || p.TokenURL == "" || p.UserInfoURL == "" {
		t.Errorf("GitHub preset has empty URL: %+v", p)
	}
	if _, ok := GetPreset("bogus"); ok {
		t.Error("expected !ok for unknown preset")
	}
}

func TestAllPresets_Deterministic(t *testing.T) {
	a := AllPresets()
	b := AllPresets()
	if len(a) != len(b) {
		t.Fatalf("len mismatch %d/%d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			t.Errorf("order mismatch at %d: %s vs %s", i, a[i].Name, b[i].Name)
		}
	}
	// GitHub is first per the ordering list.
	if a[0].Name != PresetGitHub {
		t.Errorf("first preset = %s, want github", a[0].Name)
	}
}

func TestConfig_Resolve_PresetSuccess(t *testing.T) {
	c := Config{
		Name: PresetGitHub, ClientID: "abc", ClientSecret: "shh",
		RedirectURL: "http://127.0.0.1:8080/cb",
	}
	preset, oauthCfg, err := c.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if preset.Name != PresetGitHub {
		t.Errorf("preset name = %s", preset.Name)
	}
	if oauthCfg.ClientID != "abc" || oauthCfg.RedirectURL != "http://127.0.0.1:8080/cb" {
		t.Errorf("oauthCfg: %+v", oauthCfg)
	}
	// Defaults scope to preset.DefaultScopes when none specified.
	if len(oauthCfg.Scopes) == 0 {
		t.Error("expected default scopes inherited from preset")
	}
}

func TestConfig_Resolve_ScopeOverride(t *testing.T) {
	c := Config{
		Name: PresetGitHub, ClientID: "abc",
		RedirectURL: "http://127.0.0.1/cb",
		Scopes:      []string{"read:org"},
	}
	_, cfg, err := c.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Scopes) != 1 || cfg.Scopes[0] != "read:org" {
		t.Errorf("scopes = %v", cfg.Scopes)
	}
}

func TestConfig_Resolve_CustomPreset(t *testing.T) {
	c := Config{
		Name: PresetCustom, ClientID: "id", RedirectURL: "https://app/cb",
		AuthURL:     "https://idp/auth",
		TokenURL:    "https://idp/token",
		UserInfoURL: "https://idp/me",
	}
	preset, cfg, err := c.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if preset.Name != PresetCustom {
		t.Errorf("preset.Name = %s", preset.Name)
	}
	if cfg.Endpoint.AuthURL != "https://idp/auth" {
		t.Errorf("endpoint auth = %s", cfg.Endpoint.AuthURL)
	}
	// Default field names should be sub/email/name when not specified.
	if preset.UserIDField != "sub" {
		t.Errorf("UserIDField default = %s", preset.UserIDField)
	}
}

func TestConfig_Resolve_RejectsMissingFields(t *testing.T) {
	cases := []Config{
		{Name: PresetGitHub, RedirectURL: "x"},                 // missing ClientID
		{Name: PresetGitHub, ClientID: "id"},                   // missing RedirectURL
		{Name: PresetCustom, ClientID: "id", RedirectURL: "x"}, // custom missing URLs
		{Name: "unknown", ClientID: "id", RedirectURL: "x"},    // unknown preset
	}
	for i, c := range cases {
		if _, _, err := c.resolve(); err == nil {
			t.Errorf("case %d: expected error, got nil", i)
		}
	}
}

func TestManager_Use_RegistersProvider(t *testing.T) {
	m := NewManager()
	err := m.Use(Config{
		Name: PresetGitHub, DisplayName: "GitHub",
		ClientID: "abc", RedirectURL: "http://x/cb",
	})
	if err != nil {
		t.Fatal(err)
	}
	infos := m.Providers()
	if len(infos) != 1 || infos[0].Name != string(PresetGitHub) {
		t.Errorf("providers = %+v", infos)
	}
}

func TestManager_Use_ErrorOnBadConfig(t *testing.T) {
	m := NewManager()
	if err := m.Use(Config{Name: PresetGitHub}); err == nil {
		t.Error("expected error for missing ClientID")
	}
}

func TestManager_Start_BuildsValidURLAndMintsState(t *testing.T) {
	m := NewManager()
	_ = m.Use(Config{
		Name: PresetGitHub, DisplayName: "GitHub",
		ClientID: "abc", RedirectURL: "http://x/cb",
	})
	authURL, state, err := m.Start(PresetGitHub)
	if err != nil {
		t.Fatal(err)
	}
	if state == "" {
		t.Error("expected non-empty state")
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("state") != state {
		t.Errorf("state mismatch in URL")
	}
	if u.Query().Get("code_challenge") == "" {
		t.Error("expected PKCE challenge in URL")
	}
	if u.Query().Get("code_challenge_method") != "S256" {
		t.Error("expected S256 PKCE method")
	}
	if u.Query().Get("client_id") != "abc" {
		t.Error("missing client_id")
	}
}

func TestManager_Start_UnknownProvider(t *testing.T) {
	m := NewManager()
	if _, _, err := m.Start(PresetGitLab); err == nil {
		t.Error("expected error for unregistered provider")
	}
}

func TestManager_Poll_UnknownState(t *testing.T) {
	m := NewManager()
	_, _, err := m.Poll("missing")
	if !errors.Is(err, ErrUnknownState) {
		t.Errorf("want ErrUnknownState, got %v", err)
	}
}

func TestManager_Poll_PendingBeforeComplete(t *testing.T) {
	m := NewManager()
	_ = m.Use(Config{Name: PresetGitHub, ClientID: "abc", RedirectURL: "http://x/cb"})
	_, state, _ := m.Start(PresetGitHub)
	status, info, err := m.Poll(state)
	if err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Errorf("status = %q", status)
	}
	if info != nil {
		t.Errorf("info should be nil for pending")
	}
}

// End-to-end: a fake provider's token + userinfo endpoint, driven through
// the Manager. Exercises the OAuth code-exchange path.
func TestManager_Complete_FullFlow(t *testing.T) {
	// Token endpoint: returns an access token for any code.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.FormValue("code") != "real-code" {
				http.Error(w, "bad code", 400)
				return
			}
			if r.FormValue("code_verifier") == "" {
				http.Error(w, "missing PKCE verifier", 400)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"access_token":"tok-1","token_type":"bearer"}`)
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer tok-1" {
				http.Error(w, "bad auth", 401)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    12345,
				"email": "alice@example.com",
				"name":  "Alice",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := NewManager()
	_ = m.Use(Config{
		Name:        PresetCustom,
		DisplayName: "Test IdP",
		ClientID:    "client", ClientSecret: "secret",
		RedirectURL: "http://app/cb",
		AuthURL:     srv.URL + "/auth",
		TokenURL:    srv.URL + "/token",
		UserInfoURL: srv.URL + "/userinfo",
		UserIDField: "id", EmailField: "email", NameField: "name",
	})
	_, state, err := m.Start(PresetCustom)
	if err != nil {
		t.Fatal(err)
	}
	info, err := m.Complete(context.Background(), state, "real-code")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if info.Email != "alice@example.com" {
		t.Errorf("email = %q", info.Email)
	}
	if info.Name != "Alice" {
		t.Errorf("name = %q", info.Name)
	}
	if info.ID != "12345" {
		t.Errorf("id = %q (numeric coercion broken)", info.ID)
	}
	// Poll should now return "ok" with the same info.
	status, polled, err := m.Poll(state)
	if err != nil {
		t.Fatal(err)
	}
	if status != "ok" {
		t.Errorf("post-complete status = %q", status)
	}
	if polled == nil || polled.Email != info.Email {
		t.Error("poll returned different user")
	}
}

func TestManager_Complete_BadCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad request", 400)
	}))
	defer srv.Close()
	m := NewManager()
	_ = m.Use(Config{
		Name: PresetCustom, ClientID: "c", RedirectURL: "http://app/cb",
		AuthURL: srv.URL + "/auth", TokenURL: srv.URL + "/token",
		UserInfoURL: srv.URL + "/me",
	})
	_, state, _ := m.Start(PresetCustom)
	_, err := m.Complete(context.Background(), state, "any")
	if err == nil {
		t.Error("expected error from bad code")
	}
	// Poll should now reflect the error.
	status, _, err := m.Poll(state)
	if status != "error" || err == nil {
		t.Errorf("post-error status=%q err=%v", status, err)
	}
}

func TestManager_Complete_UnknownState(t *testing.T) {
	m := NewManager()
	_, err := m.Complete(context.Background(), "missing-state", "code")
	if !errors.Is(err, ErrUnknownState) {
		t.Errorf("want ErrUnknownState, got %v", err)
	}
}

func TestManager_Complete_ExpiredState(t *testing.T) {
	now := time.Now()
	m := NewManager(WithPendingTTL(1*time.Second), WithClock(func() time.Time { return now }))
	_ = m.Use(Config{Name: PresetGitHub, ClientID: "c", RedirectURL: "http://x/cb"})
	_, state, _ := m.Start(PresetGitHub)
	// Advance time well past TTL.
	now = now.Add(5 * time.Second)
	_, err := m.Complete(context.Background(), state, "any")
	if !errors.Is(err, ErrStateExpired) {
		t.Errorf("want ErrStateExpired, got %v", err)
	}
}

func TestManager_Poll_ExpiredStateMaterialisesError(t *testing.T) {
	now := time.Now()
	m := NewManager(WithPendingTTL(1*time.Second), WithClock(func() time.Time { return now }))
	_ = m.Use(Config{Name: PresetGitHub, ClientID: "c", RedirectURL: "http://x/cb"})
	_, state, _ := m.Start(PresetGitHub)
	now = now.Add(5 * time.Second)
	status, _, err := m.Poll(state)
	if status != "error" || !errors.Is(err, ErrStateExpired) {
		t.Errorf("status=%q err=%v", status, err)
	}
}

func TestManager_Cleanup_RemovesExpiredOnly(t *testing.T) {
	now := time.Now()
	m := NewManager(WithPendingTTL(1*time.Second), WithClock(func() time.Time { return now }))
	_ = m.Use(Config{Name: PresetGitHub, ClientID: "c", RedirectURL: "http://x/cb"})
	_, _, _ = m.Start(PresetGitHub)
	// Advance only a little — not enough to age out.
	now = now.Add(500 * time.Millisecond)
	if removed := m.Cleanup(); removed != 0 {
		t.Errorf("removed %d, want 0", removed)
	}
	// Now age it out.
	now = now.Add(2 * time.Second)
	if removed := m.Cleanup(); removed != 1 {
		t.Errorf("removed %d, want 1", removed)
	}
}

func TestManager_FetchUserInfo_RejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", 403)
	}))
	defer srv.Close()
	m := NewManager()
	_ = m.Use(Config{
		Name: PresetCustom, ClientID: "c", RedirectURL: "http://app/cb",
		AuthURL: srv.URL + "/a", TokenURL: srv.URL + "/t",
		UserInfoURL: srv.URL + "/me",
	})
	// We can't call Complete without a real token exchange; call
	// fetchUserInfo directly via the exported behaviour by registering
	// a custom token endpoint that returns success but userinfo fails.
	tokSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"access_token":"tok","token_type":"bearer"}`)
	}))
	defer tokSrv.Close()
	m = NewManager()
	_ = m.Use(Config{
		Name: PresetCustom, ClientID: "c", RedirectURL: "http://app/cb",
		AuthURL: srv.URL + "/a", TokenURL: tokSrv.URL,
		UserInfoURL: srv.URL,
	})
	_, state, _ := m.Start(PresetCustom)
	_, err := m.Complete(context.Background(), state, "any")
	if err == nil || !strings.Contains(err.Error(), "userinfo") {
		t.Errorf("expected userinfo error, got %v", err)
	}
}

func TestRandomToken_UniqueAndLongEnough(t *testing.T) {
	a, err := randomToken(24)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := randomToken(24)
	if a == b {
		t.Error("randomToken produced duplicates")
	}
	if len(a) < 32 {
		t.Errorf("token length %d, want >=32", len(a))
	}
}

func TestPKCEChallenge_DeterministicFromVerifier(t *testing.T) {
	if pkceChallenge("abc") != pkceChallenge("abc") {
		t.Error("PKCE challenge not deterministic")
	}
	if pkceChallenge("a") == pkceChallenge("b") {
		t.Error("PKCE challenge same for different verifiers")
	}
}

func TestStringField_HandlesAllTypes(t *testing.T) {
	m := map[string]any{
		"s": "string",
		"n": float64(42),
		"b": true,
		"nil": nil,
		"obj": map[string]any{"x": 1},
	}
	if stringField(m, "s") != "string" {
		t.Error("string mapping")
	}
	if stringField(m, "n") != "42" {
		t.Error("number mapping")
	}
	if stringField(m, "b") != "true" {
		t.Error("bool mapping")
	}
	if stringField(m, "nil") != "" {
		t.Error("nil mapping")
	}
	if stringField(m, "missing") != "" {
		t.Error("missing mapping")
	}
	if stringField(m, "") != "" {
		t.Error("empty key")
	}
	if stringField(m, "obj") == "" {
		t.Error("nested object should at least stringify")
	}
}

func TestProviders_AlphabeticalOrderViaCatalogue(t *testing.T) {
	m := NewManager()
	_ = m.Use(Config{Name: PresetGitLab, ClientID: "a", RedirectURL: "x"})
	_ = m.Use(Config{Name: PresetGitHub, ClientID: "a", RedirectURL: "x"})
	infos := m.Providers()
	// Catalogue order has GitHub before GitLab.
	if infos[0].Name != string(PresetGitHub) || infos[1].Name != string(PresetGitLab) {
		t.Errorf("order = %+v", infos)
	}
}

func TestProviders_CustomEntryComesLast(t *testing.T) {
	m := NewManager()
	_ = m.Use(Config{
		Name: PresetCustom, ClientID: "c", RedirectURL: "x",
		AuthURL: "https://a", TokenURL: "https://t", UserInfoURL: "https://u",
	})
	_ = m.Use(Config{Name: PresetGitHub, ClientID: "c", RedirectURL: "x"})
	infos := m.Providers()
	if infos[len(infos)-1].Name != string(PresetCustom) {
		t.Errorf("custom not last: %+v", infos)
	}
}

func TestManager_DefaultsToReasonableTTL(t *testing.T) {
	m := NewManager()
	if m.pendingTTL <= 0 || m.pendingTTL > time.Hour {
		t.Errorf("pendingTTL = %v", m.pendingTTL)
	}
}
