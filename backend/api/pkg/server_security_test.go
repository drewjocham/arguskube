package pkg

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// withEnv sets env vars for the duration of the test, restoring originals
// at end. Each tuple is "KEY=value"; an empty value clears the variable.
func withEnv(t *testing.T, kv ...string) {
	t.Helper()
	for _, pair := range kv {
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			continue
		}
		key, val := pair[:eq], pair[eq+1:]
		old, was := os.LookupEnv(key)
		if val == "" {
			_ = os.Unsetenv(key)
		} else {
			_ = os.Setenv(key, val)
		}
		t.Cleanup(func() {
			if was {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
}

// TestOriginAllowed_LocalhostAlways covers the case where no allowlist is
// set: localhost variants are always permitted.
func TestOriginAllowed_LocalhostAlways(t *testing.T) {
	cases := []string{
		"http://localhost",
		"http://localhost:5173",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
		"https://localhost",
	}
	for _, origin := range cases {
		t.Run(origin, func(t *testing.T) {
			if !originAllowed(origin, nil) {
				t.Errorf("expected localhost origin %q to be allowed", origin)
			}
		})
	}
}

func TestOriginAllowed_RejectsRemoteWithoutAllowlist(t *testing.T) {
	if originAllowed("https://evil.example.com", nil) {
		t.Error("remote origin should be rejected when no allowlist is set")
	}
}

func TestOriginAllowed_AllowsExactAllowlistMatch(t *testing.T) {
	allow := []string{"https://app.kubewatcher.dev"}
	if !originAllowed("https://app.kubewatcher.dev", allow) {
		t.Error("origin in allowlist should be permitted")
	}
	if originAllowed("https://attacker.example", allow) {
		t.Error("origin not in allowlist should be denied")
	}
}

func TestOriginAllowed_EmptyOriginIsSameOrigin(t *testing.T) {
	// Same-origin requests omit Origin entirely. We must accept them.
	if !originAllowed("", nil) {
		t.Error("empty Origin (same-origin) should be permitted")
	}
}

func TestApplyCORS_DeniesUnauthorizedOrigin(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/Foo", nil)
	req.Header.Set("Origin", "https://evil.example.com")

	if applyCORS(rec, req, nil) {
		t.Error("expected applyCORS to deny unknown origin")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ACAO header should NOT be set for denied origin; got %q", got)
	}
}

func TestApplyCORS_EchoesAllowedOrigin(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/Foo", nil)
	req.Header.Set("Origin", "http://localhost:5173")

	if !applyCORS(rec, req, nil) {
		t.Fatal("localhost origin should be allowed")
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("ACAO header = %q, want %q", got, "http://localhost:5173")
	}
	if rec.Header().Get("Vary") != "Origin" {
		t.Error("Vary: Origin should be set so caches don't poison cross-origin")
	}
}

func TestAuthenticate_LoopbackBypass(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	if !authenticate(req) {
		t.Error("loopback caller without token should be permitted")
	}
}

func TestAuthenticate_RejectsRemoteWithoutToken(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	if authenticate(req) {
		t.Error("remote caller without configured token should be denied")
	}
}

func TestAuthenticate_AcceptsMatchingBearer(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=secret-abc")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req.Header.Set("Authorization", "Bearer secret-abc")
	if !authenticate(req) {
		t.Error("matching bearer should be accepted")
	}
}

func TestAuthenticate_RejectsWrongBearer(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=secret-abc")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req.Header.Set("Authorization", "Bearer wrong-token")
	if authenticate(req) {
		t.Error("non-matching bearer should be rejected")
	}
}

func TestAuthenticate_RejectsBareToken(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=secret-abc")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req.Header.Set("Authorization", "secret-abc") // missing "Bearer "
	if authenticate(req) {
		t.Error("token without Bearer prefix should be rejected")
	}
}

func TestAuthenticate_LoopbackBypassesEvenWhenTokenSet(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=secret-abc")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	if !authenticate(req) {
		t.Error("loopback should bypass token auth so the embedded webview keeps working")
	}
}

func TestMethodAllowedOverHTTP_Whitelist(t *testing.T) {
	// Spot-check: read-only methods are allowed.
	for _, name := range []string{"GetClusterInfo", "ListResources", "DiagnoseAlert"} {
		if !methodAllowedOverHTTP(name) {
			t.Errorf("expected %q to be HTTP-exposed", name)
		}
	}
	// Mutating / sensitive methods MUST stay Wails-only.
	for _, name := range []string{
		"DeletePod",
		"DeleteResource",
		"ApplyYaml",
		"SwitchContext",
		"UpdateSettings",
		"LaunchPopOutTerminal",
		"StartTerminal",
		"SendTerminalInput",
		"ExecPodShell",
		"InstallArgusScan",
		"DeployAgent",
	} {
		if methodAllowedOverHTTP(name) {
			t.Errorf("dangerous method %q must NOT be HTTP-exposed", name)
		}
	}
	// Empty / unknown returns false.
	if methodAllowedOverHTTP("") {
		t.Error("empty method name must be denied")
	}
	if methodAllowedOverHTTP("DropTablesPlease") {
		t.Error("unknown method name must be denied")
	}
}

func TestEnvBindAddr_DefaultsToLoopback(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_BIND=")
	if got := envBindAddr(8080); got != "127.0.0.1:8080" {
		t.Errorf("default bind = %q, want 127.0.0.1:8080 (refusing to expose to all interfaces)", got)
	}
}

func TestEnvBindAddr_RespectsOverride(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_BIND=0.0.0.0")
	if got := envBindAddr(8080); got != "0.0.0.0:8080" {
		t.Errorf("override bind = %q, want 0.0.0.0:8080", got)
	}
}

// TestServeHTTP_DeniesUnknownOrigin is an end-to-end regression for the
// open-CORS hole — a remote browser MUST NOT be able to call any reflective
// method on App.
func TestServeHTTP_DeniesUnknownOrigin(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=", "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := &App{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/GetClusterInfo", strings.NewReader(`{"args":[]}`))
	req.Header.Set("Origin", "https://attacker.example")
	req.RemoteAddr = "203.0.113.9:1234"
	a.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for cross-origin attacker; got %d", rec.Code)
	}
}

// TestServeHTTP_AllowsLocalhostSameOrigin covers the embedded Wails webview
// path — same-origin local request, no token configured, must succeed in
// reaching the dispatcher (here the method itself isn't bound on a zero
// App so we expect 500/501 from the call rather than 401/403 from the
// gates).
func TestServeHTTP_AllowsLocalhostThroughGates(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=", "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := &App{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/GetClusterInfo", strings.NewReader(`{"args":[]}`))
	req.RemoteAddr = "127.0.0.1:5173"
	// No Origin header (same-origin).
	a.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Errorf("loopback same-origin should pass auth/CORS gates; got %d", rec.Code)
	}
}

// TestServeHTTP_BlocksMutatingMethodViaHTTP guarantees a method NOT on the
// allowlist is rejected even when origin + auth are otherwise fine. This is
// the core of the reflection-API fix.
func TestServeHTTP_BlocksMutatingMethodViaHTTP(t *testing.T) {
	withEnv(t, "KUBEWATCHER_API_TOKEN=", "KUBEWATCHER_API_ALLOWED_ORIGINS=")
	a := &App{}
	for _, name := range []string{"DeletePod", "ApplyYaml", "LaunchPopOutTerminal"} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			path := "/api/" + url.PathEscape(name)
			req := httptest.NewRequest("POST", path, strings.NewReader(`{"args":[]}`))
			req.RemoteAddr = "127.0.0.1:5173"
			a.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 for HTTP-blocked method %q; got %d", name, rec.Code)
			}
		})
	}
}
