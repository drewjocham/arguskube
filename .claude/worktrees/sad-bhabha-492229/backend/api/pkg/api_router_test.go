package pkg

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/argues/kube-watcher/internal/config"
)

// newRouterTestApp returns a minimally-initialized App for HTTP dispatch tests.
// It only wires the fields the typed adapters touch — most handlers will fail
// out internally with a "service not configured" error, which is fine; we're
// asserting on the dispatcher itself, not on business logic.
func newRouterTestApp() *App {
	return &App{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cfg:    &config.OnlineDataConfig{},
	}
}

func doAPI(t *testing.T, a *App, method string, args ...any) (*http.Response, []byte) {
	t.Helper()
	raws := make([]json.RawMessage, len(args))
	for i, v := range args {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("marshal arg %d: %v", i, err)
		}
		raws[i] = b
	}
	body, _ := json.Marshal(APIRequest{Args: raws})
	req := httptest.NewRequest(http.MethodPost, "/api/"+method, bytes.NewReader(body))
	rr := httptest.NewRecorder()
	a.ServeHTTP(rr, req)
	resp := rr.Result()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

func TestServeHTTP_UnknownMethodReturns404(t *testing.T) {
	a := newRouterTestApp()
	resp, body := doAPI(t, a, "TotallyNotARealMethod")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body=%s", resp.StatusCode, body)
	}
}

// Sensitive private-looking names like "logger" or "Shutdown" must not be
// callable. The reflective dispatch used to expose every exported method;
// the typed router restricts access to the explicit allowlist.
func TestServeHTTP_BlocksUnregisteredButExportedMethods(t *testing.T) {
	a := newRouterTestApp()
	for _, name := range []string{"Startup", "Shutdown", "StartHTTPServer", "EmitLogLine", "HandleURL", "HandleWebhook"} {
		resp, body := doAPI(t, a, name)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404 (body=%s)", name, resp.StatusCode, body)
		}
	}
}

func TestServeHTTP_KnownMethodHandled(t *testing.T) {
	a := newRouterTestApp()
	// GetAppMode is a registered, no-arg method that returns a string. It
	// shouldn't 404 even with a minimally-initialized app.
	resp, body := doAPI(t, a, "GetAppMode")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
	}
	var r APIResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("decode: %v; body=%s", err, body)
	}
	if r.Error != "" {
		t.Errorf("unexpected error: %q", r.Error)
	}
}

func TestServeHTTP_OptionsCORSPreflight(t *testing.T) {
	a := newRouterTestApp()
	req := httptest.NewRequest(http.MethodOptions, "/api/GetAppMode", nil)
	rr := httptest.NewRecorder()
	a.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("OPTIONS status = %d, want 200", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("missing CORS allow-origin header")
	}
}

func TestServeHTTP_BadArgArityReturnsErrorPayload(t *testing.T) {
	a := newRouterTestApp()
	// SwitchContext takes exactly 1 string. Send 0 args and assert the
	// adapter reports it via the response payload — never via 500.
	resp, body := doAPI(t, a, "SwitchContext")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 (errors travel inside JSON)", resp.StatusCode)
	}
	if !strings.Contains(string(body), "wrong number of arguments") {
		t.Errorf("expected arg-count error in body, got %s", body)
	}
}
