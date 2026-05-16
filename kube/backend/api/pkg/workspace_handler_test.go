package pkg

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func newWorkspaceTestApp(t *testing.T) *App {
	t.Helper()
	return &App{
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		webhookMu: sync.RWMutex{},
	}
}

// TestRedactState pins the log-redaction shape — the OAuth state nonce
// is a secret that the upstream provider echoes back, and we only want
// a short identifying prefix in logs.
func TestRedactState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"", "…"},
		{"abc", "abc…"},
		{"abcd1234", "abcd1234…"},
		{"abcd1234extra", "abcd1234…"},
		{"verylongstatevalueXYZ", "verylong…"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := redactState(tc.in); got != tc.want {
				t.Errorf("redactState(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestRenderWorkspaceErrorPostsMessageToOpener verifies the body of
// the rendered HTML — the regression we're guarding against is the
// old behavior where error paths rendered a static page with no
// postMessage, so the parent window timed out without learning why.
func TestRenderWorkspaceErrorPostsMessageToOpener(t *testing.T) {
	t.Parallel()
	a := newWorkspaceTestApp(t)
	rec := httptest.NewRecorder()
	a.renderWorkspaceError(rec, "google", "consent denied")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	body := rec.Body.String()

	// The fix is the postMessage call AND the type: 'workspace:error'
	// shape the frontend listens for.
	if !strings.Contains(body, "window.opener.postMessage") {
		t.Error("expected the page to postMessage to the opener so the parent window learns about the failure")
	}
	if !strings.Contains(body, "'workspace:error'") {
		t.Error("expected type: 'workspace:error' so frontend's onMessage branches correctly")
	}
	// User-facing copy matches the message we passed in (htmlEscape
	// preserves common ASCII).
	if !strings.Contains(body, "consent denied") {
		t.Errorf("expected the error message in the rendered body; got %q", body)
	}
	// Service name flows through too — frontend uses it to filter
	// stale messages from previous attempts.
	if !strings.Contains(body, "'google'") {
		t.Error("expected the service name in the postMessage payload")
	}
}

func TestRenderWorkspaceErrorHTMLEscapes(t *testing.T) {
	t.Parallel()
	a := newWorkspaceTestApp(t)
	rec := httptest.NewRecorder()
	// An upstream-provider error string could in principle contain HTML
	// or quotes that would break out of the script context. The
	// htmlEscape helper has to keep that closed.
	a.renderWorkspaceError(rec, "google", `<script>alert('xss')</script>`)

	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(") {
		t.Errorf("htmlEscape failed to neutralize injected script tag; body = %q", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("expected the angle brackets to be escaped to entities")
	}
}
