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

// TestOriginAllowed_WebviewSchemes — the embedded Wails/Tauri webview
// doesn't talk to the API from localhost, it talks from wails://wails.
// Without this whitelist, the in-app fetch to /auth/providers gets a
// 403 and the dashboard is stuck on LoginView even when auth is off.
func TestOriginAllowed_WebviewSchemes(t *testing.T) {
	cases := []string{
		"wails://wails",
		"wails://wails.localhost",
		"https://wails.localhost",
		"tauri://localhost",
		"https://tauri.localhost",
		"null", // some sandboxed contexts send literal "null"
	}
	for _, origin := range cases {
		t.Run(origin, func(t *testing.T) {
			if !originAllowed(origin, nil) {
				t.Errorf("expected webview origin %q to be allowed", origin)
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
	allow := []string{"https://app.argus.dev"}
	if !originAllowed("https://app.argus.dev", allow) {
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

// authenticateService is the legacy CI / external-tooling path. After
// the auth subsystem landed, loopback callers no longer get a free
// pass — they must present either a session cookie or this static
// service token. These tests exercise the static-token path only.

func TestAuthenticateService_NoTokenConfiguredDenies(t *testing.T) {
	withEnv(t, "ARGUS_API_TOKEN=")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	if authenticateService(req) {
		t.Error("with no service token configured, the static path must deny — sessions are the only way in")
	}
}

func TestAuthenticateService_RejectsRemoteWithoutToken(t *testing.T) {
	withEnv(t, "ARGUS_API_TOKEN=")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	if authenticateService(req) {
		t.Error("remote caller without configured token should be denied")
	}
}

func TestAuthenticateService_AcceptsMatchingBearer(t *testing.T) {
	withEnv(t, "ARGUS_API_TOKEN=secret-abc")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req.Header.Set("Authorization", "Bearer secret-abc")
	if !authenticateService(req) {
		t.Error("matching bearer should be accepted")
	}
}

func TestAuthenticateService_RejectsWrongBearer(t *testing.T) {
	withEnv(t, "ARGUS_API_TOKEN=secret-abc")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req.Header.Set("Authorization", "Bearer wrong-token")
	if authenticateService(req) {
		t.Error("non-matching bearer should be rejected")
	}
}

func TestAuthenticateService_RejectsBareToken(t *testing.T) {
	withEnv(t, "ARGUS_API_TOKEN=secret-abc")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	req.Header.Set("Authorization", "secret-abc") // missing "Bearer "
	if authenticateService(req) {
		t.Error("token without Bearer prefix should be rejected")
	}
}

func TestAuthenticateService_LoopbackNoLongerBypasses(t *testing.T) {
	// Regression guard: the previous build let any loopback caller
	// reach /api/* without a token. Spec changed — no account, no
	// access — so loopback is no longer special.
	withEnv(t, "ARGUS_API_TOKEN=secret-abc")
	req := httptest.NewRequest("POST", "/api/Foo", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	if authenticateService(req) {
		t.Error("loopback caller with no Authorization header must NOT be auto-authenticated")
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
	withEnv(t, "ARGUS_API_BIND=")
	if got := envBindAddr(8080); got != "127.0.0.1:8080" {
		t.Errorf("default bind = %q, want 127.0.0.1:8080 (refusing to expose to all interfaces)", got)
	}
}

func TestEnvBindAddr_RespectsOverride(t *testing.T) {
	withEnv(t, "ARGUS_API_BIND=0.0.0.0")
	if got := envBindAddr(8080); got != "0.0.0.0:8080" {
		t.Errorf("override bind = %q, want 0.0.0.0:8080", got)
	}
}

// TestServeHTTP_DeniesUnknownOrigin is an end-to-end regression for the
// open-CORS hole — a remote browser MUST NOT be able to call any reflective
// method on App.
func TestServeHTTP_DeniesUnknownOrigin(t *testing.T) {
	withEnv(t, "ARGUS_API_TOKEN=", "ARGUS_API_ALLOWED_ORIGINS=")
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

// TestServeHTTP_LoopbackWithoutSessionIsUnauthorized is the regression
// guard for the spec change "no account = no access". Previously the
// embedded Wails webview got into /api/* simply by virtue of being on
// loopback. That bypass is gone — the frontend now logs in first and
// presents a session token on every request.
func TestServeHTTP_LoopbackWithoutSessionIsUnauthorized(t *testing.T) {
	withEnv(t, "ARGUS_API_TOKEN=", "ARGUS_API_ALLOWED_ORIGINS=")
	a := &App{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/GetClusterInfo", strings.NewReader(`{"args":[]}`))
	req.RemoteAddr = "127.0.0.1:5173"
	a.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("loopback without session must be 401; got %d", rec.Code)
	}
}

// TestServeHTTP_ServiceTokenReachesDispatcher confirms the legacy
// CI/external-tooling path still works: a request bearing the
// ARGUS_API_TOKEN bypasses session auth and reaches the
// reflective dispatcher.
func TestServeHTTP_ServiceTokenReachesDispatcher(t *testing.T) {
	withEnv(t, "ARGUS_API_TOKEN=svc-token", "ARGUS_API_ALLOWED_ORIGINS=")
	a := &App{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/GetClusterInfo", strings.NewReader(`{"args":[]}`))
	req.RemoteAddr = "127.0.0.1:5173"
	req.Header.Set("Authorization", "Bearer svc-token")
	a.ServeHTTP(rec, req)
	// Past auth + CORS, the method itself runs against a zero App
	// and may return 404 (method not found on bare struct) or 500;
	// what matters is we did NOT short-circuit at 401/403.
	if rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Errorf("service token should pass auth gates; got %d", rec.Code)
	}
}

// TestServeHTTP_BlocksMutatingMethodViaHTTP guarantees a method NOT on the
// allowlist is rejected even when origin + auth are otherwise fine. This is
// the core of the reflection-API fix. We need a passing auth path so we
// can test the LATER gate (method allowlist) — without the service
// token we'd just get 401 first.
func TestServeHTTP_BlocksMutatingMethodViaHTTP(t *testing.T) {
	withEnv(t, "ARGUS_API_TOKEN=svc-token", "ARGUS_API_ALLOWED_ORIGINS=")
	a := &App{}
	for _, name := range []string{"DeletePod", "ApplyYaml", "LaunchPopOutTerminal"} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			path := "/api/" + url.PathEscape(name)
			req := httptest.NewRequest("POST", path, strings.NewReader(`{"args":[]}`))
			req.RemoteAddr = "127.0.0.1:5173"
			req.Header.Set("Authorization", "Bearer svc-token")
			a.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403 for HTTP-blocked method %q; got %d", name, rec.Code)
			}
		})
	}
}
