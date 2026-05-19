package workspace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// gcalTestHandler routes the four calendar endpoints the adapter calls.
func gcalTestHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	handleList := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("timeMin") != "" {
			t.Logf("list with timeMin=%s timeMax=%s", r.URL.Query().Get("timeMin"), r.URL.Query().Get("timeMax"))
		}
		_, _ = w.Write([]byte(`{
			"items": [
				{"id":"E1","summary":"Standup","start":{"dateTime":"2026-01-01T10:00:00Z"},"end":{"dateTime":"2026-01-01T10:30:00Z"},"htmlLink":"https://example.com/e1"},
				{"id":"E2","summary":"Lunch","description":"Team lunch","location":"Cafe","start":{"dateTime":"2026-01-01T12:00:00Z"},"end":{"dateTime":"2026-01-01T13:00:00Z"}}
			]
		}`))
	}
	handleCreate := func(w http.ResponseWriter, r *http.Request) {
		// echo back a fake created event
		_, _ = w.Write([]byte(`{
			"id":"E3","summary":"Review","start":{"dateTime":"2026-06-01T14:00:00Z"},"end":{"dateTime":"2026-06-01T15:00:00Z"},"htmlLink":"https://example.com/e3"
		}`))
	}
	handlePatch := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id":"E1","summary":"Standup (updated)","start":{"dateTime":"2026-01-01T10:00:00Z"},"end":{"dateTime":"2026-01-01T11:00:00Z"}
		}`))
	}
	handleDelete := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/calendars/primary/events":
			handleList(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/calendars/primary/events":
			handleCreate(w, r)
		case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/calendars/primary/events/"):
			handlePatch(w, r)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/calendars/primary/events/"):
			handleDelete(w, r)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}
}

func TestGCalAdapter_ListEvents(t *testing.T) {
	tests := []struct {
		name      string
		start     string
		end       string
		wantCount int
		wantFirst string
		err       bool
	}{
		{name: "full range", start: "2026-01-01T00:00:00Z", end: "2026-01-02T00:00:00Z", wantCount: 2, wantFirst: "Standup"},
		{name: "no time bounds", start: "", end: "", wantCount: 2, wantFirst: "Standup"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(gcalTestHandler(t))
			defer srv.Close()
			a := &GCalAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
			tok := Token{AccessToken: "ya29.x"}

			events, err := a.ListEvents(context.Background(), tok, tt.start, tt.end)
			if (err != nil) != tt.err {
				t.Fatalf("ListEvents err=%v want err=%v", err, tt.err)
			}
			if len(events) != tt.wantCount {
				t.Fatalf("got %d events, want %d", len(events), tt.wantCount)
			}
			if events[0].Summary != tt.wantFirst {
				t.Errorf("first event summary = %q, want %q", events[0].Summary, tt.wantFirst)
			}
		})
	}
}

func TestGCalAdapter_CreateEvent(t *testing.T) {
	tests := []struct {
		name    string
		ev      Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event",
			ev: Event{
				Summary: "Review",
				Start:   "2026-06-01T14:00:00Z",
				End:     "2026-06-01T15:00:00Z",
			},
		},
		{
			name:    "empty summary",
			ev:      Event{Start: "2026-06-01T14:00:00Z"},
			wantErr: true,
			errMsg:  "summary is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(gcalTestHandler(t))
			defer srv.Close()
			a := &GCalAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
			tok := Token{AccessToken: "ya29.x"}

			created, err := a.CreateEvent(context.Background(), tok, tt.ev)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("expected error containing %q, got %v", tt.errMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("CreateEvent: %v", err)
			}
			if created.ID != "E3" {
				t.Errorf("created.ID = %q, want E3", created.ID)
			}
		})
	}
}

func TestGCalAdapter_UpdateEvent(t *testing.T) {
	tests := []struct {
		name    string
		eventID string
		ev      Event
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid update",
			eventID: "E1",
			ev:      Event{Summary: "Standup (updated)"},
		},
		{
			name:    "empty eventID",
			eventID: "",
			ev:      Event{Summary: "x"},
			wantErr: true,
			errMsg:  "eventID required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(gcalTestHandler(t))
			defer srv.Close()
			a := &GCalAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
			tok := Token{AccessToken: "ya29.x"}

			updated, err := a.UpdateEvent(context.Background(), tok, tt.eventID, tt.ev)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("expected error containing %q, got %v", tt.errMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("UpdateEvent: %v", err)
			}
			if updated.ID != "E1" {
				t.Errorf("updated.ID = %q, want E1", updated.ID)
			}
		})
	}
}

func TestGCalAdapter_DeleteEvent(t *testing.T) {
	tests := []struct {
		name    string
		eventID string
		wantErr bool
		errMsg  string
	}{
		{name: "valid delete", eventID: "E1"},
		{name: "empty eventID", eventID: "", wantErr: true, errMsg: "eventID required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(gcalTestHandler(t))
			defer srv.Close()
			a := &GCalAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
			tok := Token{AccessToken: "ya29.x"}

			err := a.DeleteEvent(context.Background(), tok, tt.eventID)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("expected error containing %q, got %v", tt.errMsg, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("DeleteEvent: %v", err)
			}
		})
	}
}

func TestGCalAdapter_PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"error":{"code":403,"message":"PERMISSION_DENIED"}}`))
	}))
	defer srv.Close()
	a := &GCalAdapter{HTTPClient: http.DefaultClient, APIBase: srv.URL}
	tok := Token{AccessToken: "ya29.x"}

	_, err := a.ListEvents(context.Background(), tok, "", "")
	if err == nil || !strings.Contains(err.Error(), "PERMISSION_DENIED") {
		t.Fatalf("expected PERMISSION_DENIED, got %v", err)
	}
}

func TestGoogleProvider_StartIncludesCalendarScope(t *testing.T) {
	p := &GoogleProvider{
		ClientID:    "cid",
		RedirectURL: "https://argus.example/cb",
	}
	auth, err := p.Start(context.Background(), "u", "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !strings.Contains(auth.URL, "calendar") {
		t.Errorf("calendar scope missing from auth URL: %s", auth.URL)
	}
}
