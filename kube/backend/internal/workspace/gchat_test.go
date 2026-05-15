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

func TestGChatAdapter_ListChannels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/spaces" {
			t.Errorf("wrong path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer ya29-tok" {
			t.Errorf("missing bearer auth: %q", got)
		}
		_, _ = w.Write([]byte(`{
			"spaces": [
				{"name":"spaces/AAA111","displayName":"Engineering","spaceType":"SPACE"},
				{"name":"spaces/BBB222","spaceType":"DIRECT_MESSAGE"},
				{"name":"spaces/CCC333","displayName":"","spaceType":"GROUP_CHAT"}
			]
		}`))
	}))
	defer srv.Close()

	a := &GChatAdapter{HTTPClient: http.DefaultClient, APIBaseURL: srv.URL}
	got, err := a.ListChannels(context.Background(), Token{AccessToken: "ya29-tok"})
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 spaces, got %d", len(got))
	}
	if got[0].ID != "spaces/AAA111" || got[0].Name != "Engineering" {
		t.Errorf("named space wrong: %+v", got[0])
	}
	if !strings.HasPrefix(got[1].Name, "Direct message") {
		t.Errorf("DM label wrong: %+v", got[1])
	}
	if !strings.HasPrefix(got[2].Name, "Group chat") {
		t.Errorf("group chat label wrong: %+v", got[2])
	}
}

func TestGChatAdapter_ListChannels_LegacyTypeField(t *testing.T) {
	// Older spaces.list responses returned `type` instead of `spaceType`.
	// We accept both so a partial rollout doesn't break the picker.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"spaces":[{"name":"spaces/X","type":"DM"}]}`))
	}))
	defer srv.Close()
	a := &GChatAdapter{HTTPClient: http.DefaultClient, APIBaseURL: srv.URL}
	got, _ := a.ListChannels(context.Background(), Token{AccessToken: "t"})
	if len(got) != 1 || !strings.HasPrefix(got[0].Name, "Direct message") {
		t.Fatalf("legacy type field not recognized: %+v", got)
	}
}

func TestGChatAdapter_Send(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		_, _ = w.Write([]byte(`{"name":"spaces/X/messages/Y","createTime":"2026-01-01T00:00:00Z"}`))
	}))
	defer srv.Close()

	a := &GChatAdapter{HTTPClient: http.DefaultClient, APIBaseURL: srv.URL}
	if err := a.Send(context.Background(), Token{AccessToken: "t"}, "spaces/X", "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotPath != "/spaces/X/messages" {
		t.Errorf("wrong path: %s", gotPath)
	}
	var b gchatMessageBody
	if err := json.Unmarshal([]byte(gotBody), &b); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if b.Text != "hello" {
		t.Errorf("text wrong: %q", b.Text)
	}
}

func TestGChatAdapter_Send_PrependsSpacesPrefix(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"name":"spaces/X/messages/Y"}`))
	}))
	defer srv.Close()
	a := &GChatAdapter{HTTPClient: http.DefaultClient, APIBaseURL: srv.URL}
	// Caller passes bare ID — adapter should add the "spaces/" prefix.
	_ = a.Send(context.Background(), Token{AccessToken: "t"}, "BAREID", "hi")
	if gotPath != "/spaces/BAREID/messages" {
		t.Errorf("prefix not added: %s", gotPath)
	}
}

func TestGChatAdapter_SendPropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403,"message":"permission denied"}}`))
	}))
	defer srv.Close()
	a := &GChatAdapter{HTTPClient: http.DefaultClient, APIBaseURL: srv.URL}
	err := a.Send(context.Background(), Token{AccessToken: "t"}, "spaces/X", "hi")
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestGChatAdapter_RejectsEmptySpace(t *testing.T) {
	a := NewGChatAdapter()
	if err := a.Send(context.Background(), Token{AccessToken: "t"}, "", "hi"); err == nil {
		t.Fatal("expected error for empty space")
	}
}
