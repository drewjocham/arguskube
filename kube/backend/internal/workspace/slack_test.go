package workspace

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlackProvider_StartBuildsAuthURL(t *testing.T) {
	p := &SlackProvider{
		ClientID:    "cid",
		RedirectURL: "https://argus.example/workspace/oauth/callback",
	}
	auth, err := p.Start(context.Background(), "u", "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !strings.HasPrefix(auth.URL, "https://slack.com/oauth/v2/authorize?") {
		t.Errorf("auth url has wrong base: %s", auth.URL)
	}
	if !strings.Contains(auth.URL, "client_id=cid") {
		t.Errorf("client_id missing: %s", auth.URL)
	}
	if !strings.Contains(auth.URL, "scope=chat") {
		t.Errorf("default scope missing: %s", auth.URL)
	}
	if !strings.Contains(auth.URL, "state="+auth.State) {
		t.Errorf("state not in URL: %s", auth.URL)
	}
}

func TestSlackProvider_RejectsUnconfigured(t *testing.T) {
	p := &SlackProvider{}
	if _, err := p.Start(context.Background(), "u", ""); err == nil {
		t.Fatal("expected error for unconfigured provider")
	}
}

func TestSlackProvider_CompleteParsesToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slack expects form-encoded body. We assert on it so a
		// future refactor doesn't accidentally switch to JSON without
		// updating the upstream contract.
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "code=test-code") {
			t.Errorf("body missing code: %s", body)
		}
		_, _ = w.Write([]byte(`{
			"ok": true,
			"access_token": "xoxb-test-token",
			"token_type": "bot",
			"scope": "chat:write,channels:read",
			"bot_user_id": "U-bot",
			"team": {"id": "T0001", "name": "Test Team"},
			"authed_user": {"id": "U-installer"}
		}`))
	}))
	defer srv.Close()

	p := &SlackProvider{
		ClientID: "cid", ClientSecret: "csec",
		RedirectURL: "https://argus.example/cb",
		TokenURL:    srv.URL,
	}
	res, err := p.Complete(context.Background(), "state", "test-code")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.ExternalWorkspaceID != "T0001" {
		t.Errorf("team id: got %q want T0001", res.ExternalWorkspaceID)
	}
	if res.DisplayName != "Test Team" {
		t.Errorf("team name: got %q", res.DisplayName)
	}
	if res.Token.AccessToken != "xoxb-test-token" {
		t.Errorf("access token: got %q", res.Token.AccessToken)
	}
	if !res.Token.ExpiresAt.IsZero() {
		t.Errorf("bot tokens should have zero expiry, got %v", res.Token.ExpiresAt)
	}
}

func TestSlackProvider_CompleteSurfacesSlackError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok": false, "error": "invalid_code"}`))
	}))
	defer srv.Close()

	p := &SlackProvider{
		ClientID: "cid", ClientSecret: "csec",
		RedirectURL: "https://argus.example/cb",
		TokenURL:    srv.URL,
	}
	_, err := p.Complete(context.Background(), "state", "bad")
	if err == nil || !strings.Contains(err.Error(), "invalid_code") {
		t.Fatalf("expected invalid_code in error, got %v", err)
	}
}

func TestSlackAdapter_ListChannels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer xoxb-tok" {
			t.Errorf("missing bearer auth: %q", got)
		}
		if !strings.HasPrefix(r.URL.Path, "/conversations.list") {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"ok": true,
			"channels": [
				{"id":"C1","name":"general","is_channel":true,"is_archived":false},
				{"id":"C2","name":"old","is_channel":true,"is_archived":true},
				{"id":"C3","name":"random","is_channel":true,"is_archived":false}
			]
		}`))
	}))
	defer srv.Close()

	a := &SlackAdapter{HTTPClient: http.DefaultClient, APIBaseURL: srv.URL}
	channels, err := a.ListChannels(context.Background(), Token{AccessToken: "xoxb-tok"})
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("expected 2 non-archived channels, got %d: %+v", len(channels), channels)
	}
	if channels[0].Name != "general" || channels[1].Name != "random" {
		t.Errorf("wrong channels: %+v", channels)
	}
}

func TestSlackAdapter_Send(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat.postMessage" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_, _ = w.Write([]byte(`{"ok": true, "ts": "1234.0001", "channel": "C1"}`))
	}))
	defer srv.Close()

	a := &SlackAdapter{HTTPClient: http.DefaultClient, APIBaseURL: srv.URL}
	if err := a.Send(context.Background(), Token{AccessToken: "xoxb-t"}, "C1", "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(gotBody, "channel=C1") || !strings.Contains(gotBody, "text=hello") {
		t.Fatalf("form body wrong: %s", gotBody)
	}
}

func TestSlackAdapter_SendPropagatesSlackError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok": false, "error": "not_in_channel"}`))
	}))
	defer srv.Close()
	a := &SlackAdapter{HTTPClient: http.DefaultClient, APIBaseURL: srv.URL}
	err := a.Send(context.Background(), Token{AccessToken: "xoxb-t"}, "C1", "hi")
	if err == nil || !strings.Contains(err.Error(), "not_in_channel") {
		t.Fatalf("expected not_in_channel, got %v", err)
	}
}

func TestSlackProvider_FullFlowThroughManager(t *testing.T) {
	// End-to-end through the manager: Start → fake user redirect →
	// LookupPendingService → Complete (mock token) → Store.Upsert →
	// Token round-trip.
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"ok": true,
			"access_token": "xoxb-flow",
			"token_type": "bot",
			"scope": "chat:write",
			"team": {"id": "T-flow", "name": "Flow Team"},
			"authed_user": {"id": "U1"}
		}`))
	}))
	defer tokenSrv.Close()

	provider := &SlackProvider{
		ClientID: "cid", ClientSecret: "csec",
		RedirectURL: "https://argus.example/cb",
		TokenURL:    tokenSrv.URL,
	}

	m := newManager(t)
	// newManager registered TestProvider for slack; replace with the
	// real one for this test.
	m.providers[ServiceSlack] = provider

	auth, err := m.Start(context.Background(), "user-1", ServiceSlack, "https://cb")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	svc, err := m.LookupPendingService(auth.State)
	if err != nil || svc != ServiceSlack {
		t.Fatalf("LookupPendingService: %v %v", svc, err)
	}
	c, err := m.Complete(context.Background(), ServiceSlack, auth.State, "code-123")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if c.ExternalWorkspaceID != "T-flow" || c.DisplayName != "Flow Team" {
		t.Fatalf("connection identity wrong: %+v", c)
	}
	tok, err := m.Token(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok.AccessToken != "xoxb-flow" {
		t.Fatalf("token round-trip lost data: %q", tok.AccessToken)
	}
}

// satisfy the unused-import linter when we add tests with json.* later.
var _ = json.Marshal
