package pkg

import (
	"crypto/subtle"
	"net"
	"net/http"
	"os"
	"strings"
)

// Security primitives for the SaaS HTTP API.
//
// Defaults are deny-by-default for non-localhost callers:
//
//   - Bind:        127.0.0.1 only (override with KUBEWATCHER_API_BIND)
//   - CORS:        no Access-Control-Allow-Origin echoed unless the request's
//                  Origin is in KUBEWATCHER_API_ALLOWED_ORIGINS (comma list)
//                  OR is a localhost/127.0.0.1 origin.
//   - Auth:        when KUBEWATCHER_API_TOKEN is set, every non-OPTIONS,
//                  non-localhost request must carry a matching
//                  "Authorization: Bearer <token>" header. Localhost calls
//                  bypass to keep the embedded Wails frontend working.
//
// The previous implementation was open-by-default: `Access-Control-Allow-Origin: *`,
// no auth, reflective dispatch on every public method of the App. That was
// fine on a developer laptop but fatal as soon as the binary listened on
// any non-loopback interface.

// envBindAddr returns the address the SaaS HTTP server should listen on.
// Default is loopback so a misconfigured firewall can't expose the API.
func envBindAddr(port int) string {
	host := strings.TrimSpace(os.Getenv("KUBEWATCHER_API_BIND"))
	if host == "" {
		host = "127.0.0.1"
	}
	return formatAddr(host, port)
}

func formatAddr(host string, port int) string {
	// IPv6 hosts need brackets.
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]:" + itoa(port)
	}
	return host + ":" + itoa(port)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// allowedOrigins returns the parsed comma-separated env list. An empty
// slice means "no cross-origin browsers allowed except localhost."
func allowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("KUBEWATCHER_API_ALLOWED_ORIGINS"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// originAllowed reports whether `origin` (the value of the request's Origin
// header) should be echoed back in CORS responses. Localhost / loopback
// origins always pass — the embedded Wails webview and `wails dev` both
// hit us from there.
func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		// Same-origin requests omit the Origin header; nothing to echo.
		return true
	}
	if isLocalOrigin(origin) {
		return true
	}
	for _, a := range allowed {
		if origin == a {
			return true
		}
	}
	return false
}

func isLocalOrigin(origin string) bool {
	// "null" is what some webviews (file://, sandboxed iframes) send.
	// Treating it as local is fine for our threat model: the only way
	// to land on a `null` origin is to already be inside our own
	// process (Wails / Tauri / a saved HTML file we wrote ourselves).
	if origin == "null" {
		return true
	}
	// Wails 2.x webviews load the embedded SPA from `wails://wails` /
	// `wails://` (Linux), and Tauri uses `tauri://localhost` /
	// `https://tauri.localhost`. These aren't HTTP "remote" origins —
	// they're our own webview talking to our own loopback API. Accept
	// the schemes wholesale; otherwise the embedded UI can't reach
	// /auth/* and the LoginView never knows auth is disabled.
	if strings.HasPrefix(origin, "wails://") || strings.HasPrefix(origin, "tauri://") {
		return true
	}

	// Origins are typed as scheme://host[:port]; we only need host.
	// IPv6 hosts use the [::1]:port form, so strip brackets carefully.
	host := origin
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if strings.HasPrefix(host, "[") {
		// Bracketed IPv6: [::1]:port → strip the bracketed segment as host.
		end := strings.Index(host, "]")
		if end > 0 {
			host = host[1:end]
		}
	} else if i := strings.LastIndex(host, ":"); i >= 0 {
		// Plain host or IPv4: strip the trailing :port.
		host = host[:i]
	}
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0",
		"wails", "wails.localhost", "tauri.localhost":
		return true
	}
	return false
}

// remoteIsLocal reports whether the inbound request originated on the
// loopback interface. Used to bypass token auth for the embedded Wails
// webview, which always connects from 127.0.0.1.
func remoteIsLocal(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// applyCORS sets only the headers the policy says are safe. Returns true
// if the policy allowed the request to proceed.
func applyCORS(w http.ResponseWriter, r *http.Request, allowed []string) bool {
	origin := r.Header.Get("Origin")
	w.Header().Set("Vary", "Origin")
	if !originAllowed(origin, allowed) {
		return false
	}
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Max-Age", "300")
	return true
}

// authorizeAPIRequest gates /api/* on a valid session token. The
// service-token path is also accepted so CI jobs and external tooling
// can hit read-only endpoints without going through the user-account
// flow. Loopback alone is no longer sufficient: the user must be
// signed in (or carry a service token) to reach the API.
//
// When KUBEWATCHER_AUTH_DISABLED is on AND the API listens on
// loopback (enforced in SetupAuth), this returns true unconditionally.
// That's the local-dev escape hatch.
func (a *App) authorizeAPIRequest(r *http.Request) bool {
	if a.auth != nil && a.auth.devMode {
		return true
	}
	if a.auth != nil {
		if user, err := a.auth.store.ValidateSession(bearerFromRequest(r)); err == nil && user != nil {
			return true
		}
	}
	return authenticateService(r)
}

// authenticateService is the legacy CI / external-tooling path: a static
// token in KUBEWATCHER_API_TOKEN. Kept so service accounts and CI jobs
// can call read-only endpoints without going through the user-account
// flow. Returns true if a service token matches.
//
// Prefer session auth (validateSession on App) for human callers — that
// path is what the LoginView, OAuth callbacks, and the embedded webview
// all use. The legacy bypass for "loopback without any token" is GONE:
// users must log in even on the desktop, by design.
func authenticateService(r *http.Request) bool {
	token := strings.TrimSpace(os.Getenv("KUBEWATCHER_API_TOKEN"))
	if token == "" {
		return false
	}
	hdr := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(hdr, prefix) {
		return false
	}
	got := strings.TrimSpace(hdr[len(prefix):])
	return subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}
