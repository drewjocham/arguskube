package pkg

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argues/argus/internal/oauthproviders"
)

// Minimal App constructions — only oauthManager is needed for these
// tests, the rest of the App's dependencies are ignored.

func TestApp_ListOAuthProviders_NilManager_ReturnsNil(t *testing.T) {
	a := &App{}
	if got := a.ListOAuthProviders(); got != nil {
		t.Errorf("nil manager should return nil, got %+v", got)
	}
}

func TestApp_ListOAuthProviders_DelegatesToManager(t *testing.T) {
	m := oauthproviders.NewManager()
	if err := m.Use(oauthproviders.Config{
		Name: oauthproviders.PresetGitHub, DisplayName: "GitHub",
		ClientID: "c", RedirectURL: "http://x/cb",
	}); err != nil {
		t.Fatal(err)
	}
	a := &App{oauthManager: m}
	infos := a.ListOAuthProviders()
	if len(infos) != 1 || infos[0].Name != "github" {
		t.Errorf("infos = %+v", infos)
	}
}

func TestApp_StartOAuthFlow_NilManager(t *testing.T) {
	a := &App{}
	authURL, state, errMsg := a.StartOAuthFlow("github")
	if errMsg == "" || authURL != "" || state != "" {
		t.Errorf("expected error: url=%q state=%q err=%q", authURL, state, errMsg)
	}
}

func TestApp_StartOAuthFlow_UnknownProvider(t *testing.T) {
	a := &App{oauthManager: oauthproviders.NewManager()}
	_, _, errMsg := a.StartOAuthFlow("nope")
	if errMsg == "" {
		t.Error("expected error for unknown provider")
	}
}

func TestApp_StartOAuthFlow_HappyPath(t *testing.T) {
	m := oauthproviders.NewManager()
	if err := m.Use(oauthproviders.Config{
		Name: oauthproviders.PresetGitHub, DisplayName: "GitHub",
		ClientID: "c", RedirectURL: "http://x/cb",
	}); err != nil {
		t.Fatal(err)
	}
	a := &App{oauthManager: m}
	authURL, state, errMsg := a.StartOAuthFlow("github")
	if errMsg != "" {
		t.Fatalf("errMsg = %q", errMsg)
	}
	if state == "" {
		t.Error("state should be non-empty")
	}
	if authURL == "" {
		t.Error("authURL should be non-empty")
	}
}

func TestApp_PollOAuthFlow_NilManager(t *testing.T) {
	a := &App{}
	res := a.PollOAuthFlow("anything")
	if res.Status != "error" || res.Error == "" {
		t.Errorf("res = %+v", res)
	}
}

func TestApp_PollOAuthFlow_UnknownState(t *testing.T) {
	a := &App{oauthManager: oauthproviders.NewManager()}
	res := a.PollOAuthFlow("missing")
	if res.Status != "error" {
		t.Errorf("status = %q", res.Status)
	}
	if res.Error == "" {
		t.Error("error should be non-empty for unknown state")
	}
}

func TestApp_PollOAuthFlow_PendingThenOK(t *testing.T) {
	// Stand up a fake provider so the full Start→Complete→Poll dance works.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"access_token":"tok","token_type":"bearer"}`)
		case "/me":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"sub":   "u-1",
				"email": "u@example.com",
				"name":  "U",
			})
		}
	}))
	defer srv.Close()

	m := oauthproviders.NewManager()
	if err := m.Use(oauthproviders.Config{
		Name: oauthproviders.PresetCustom, DisplayName: "Test",
		ClientID: "c", RedirectURL: "http://app/cb",
		AuthURL: srv.URL + "/a", TokenURL: srv.URL + "/token",
		UserInfoURL: srv.URL + "/me",
	}); err != nil {
		t.Fatal(err)
	}
	a := &App{oauthManager: m}

	_, state, errMsg := a.StartOAuthFlow("custom")
	if errMsg != "" {
		t.Fatalf("start errMsg = %q", errMsg)
	}
	// Pre-completion: status should be "pending"
	if res := a.PollOAuthFlow(state); res.Status != "pending" {
		t.Errorf("pre-complete status = %q", res.Status)
	}
	// Complete the flow.
	info, err := a.CompleteOAuthFlow(state, "code-1")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if info.Email != "u@example.com" {
		t.Errorf("email = %q", info.Email)
	}
	// Post-completion: status should be "ok" with user populated.
	res := a.PollOAuthFlow(state)
	if res.Status != "ok" {
		t.Errorf("post-complete status = %q", res.Status)
	}
	if res.User == nil || res.User.Email != "u@example.com" {
		t.Errorf("poll result user = %+v", res.User)
	}
	if res.Error != "" {
		t.Errorf("error should be empty on ok, got %q", res.Error)
	}
}

func TestApp_CompleteOAuthFlow_NilManager(t *testing.T) {
	a := &App{}
	_, err := a.CompleteOAuthFlow("state", "code")
	if err == nil {
		t.Error("expected error for nil manager")
	}
}

func TestApp_CompleteOAuthFlow_UnknownState(t *testing.T) {
	a := &App{oauthManager: oauthproviders.NewManager()}
	_, err := a.CompleteOAuthFlow("missing", "code")
	if !errors.Is(err, oauthproviders.ErrUnknownState) {
		t.Errorf("want ErrUnknownState, got %v", err)
	}
}

func TestApp_CancelOAuthFlow_NilManagerReturnsFalse(t *testing.T) {
	a := &App{}
	if a.CancelOAuthFlow("any") {
		t.Error("nil manager should return false")
	}
}

func TestApp_CancelOAuthFlow_UnknownState(t *testing.T) {
	a := &App{oauthManager: oauthproviders.NewManager()}
	if a.CancelOAuthFlow("missing") {
		t.Error("unknown state should return false")
	}
}

func TestResolveOAuthFlowError_NilToEmpty(t *testing.T) {
	if got := ResolveOAuthFlowError(nil); got != "" {
		t.Errorf("nil error → %q", got)
	}
	if got := ResolveOAuthFlowError(errors.New("boom")); got != "boom" {
		t.Errorf("error → %q", got)
	}
}

func TestApp_OAuthPollResult_JSONShape(t *testing.T) {
	// The frontend depends on the exact JSON tags so we lock them in.
	r := OAuthPollResult{Status: "ok", User: &oauthproviders.UserInfo{Email: "x"}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !contains(got, `"status":"ok"`) {
		t.Errorf("missing status tag: %s", got)
	}
	if !contains(got, `"user":`) {
		t.Errorf("missing user tag: %s", got)
	}
	// Empty error field omits via omitempty.
	if contains(got, `"error":""`) {
		t.Errorf("error tag should be omitted: %s", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
