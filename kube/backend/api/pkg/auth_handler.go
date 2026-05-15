package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/argues/argus/internal/auth"
	"github.com/argues/argus/internal/config"
)

// isLoopbackBind reports whether the configured API bind address keeps
// the listener on a host-only interface. Used to gate the dev-mode
// bypass so a public deployment can't accidentally have it on.
func isLoopbackBind(bind string) bool {
	bind = strings.TrimSpace(bind)
	if bind == "" {
		return true // default is 127.0.0.1
	}
	return strings.HasPrefix(bind, "127.") ||
		bind == "localhost" ||
		bind == "::1" ||
		strings.HasPrefix(bind, "[::1]")
}

// authState bundles the user-account dependencies the HTTP layer needs.
// Lives on App so /api/* and /auth/* share the same store.
type authState struct {
	store       *auth.Store
	oidc        *auth.OIDCManager
	allowSignup bool
	// devMode disables the entire gate. Only honored when the API
	// binds to loopback — see SetupAuth for the safety check.
	devMode bool
}

// SetupAuth wires the auth store + OIDC providers from config. Called
// from main.go right after the App is constructed and the DB is open.
func (a *App) SetupAuth(store *auth.Store, cfg config.AuthConfig) {
	providers := []auth.ProviderConfig{}
	if cfg.GoogleClientID != "" {
		providers = append(providers, auth.ProviderConfig{
			Name:         auth.ProviderGoogle,
			DisplayName:  "Google",
			Issuer:       "https://accounts.google.com",
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  strings.TrimRight(cfg.PublicBaseURL, "/") + "/auth/google/callback",
		})
	}
	if cfg.OIDCIssuer != "" && cfg.OIDCClientID != "" {
		display := cfg.OIDCDisplayName
		if display == "" {
			display = "Corporate SSO"
		}
		providers = append(providers, auth.ProviderConfig{
			Name:         auth.ProviderOIDC,
			DisplayName:  display,
			Issuer:       cfg.OIDCIssuer,
			ClientID:     cfg.OIDCClientID,
			ClientSecret: cfg.OIDCClientSecret,
			RedirectURL:  strings.TrimRight(cfg.PublicBaseURL, "/") + "/auth/oidc/callback",
		})
	}
	// DevMode is a local-development convenience — it bypasses the
	// entire auth gate. We refuse to honor it when ARGUS_API_BIND
	// points at anything other than loopback, so a deploy that
	// accidentally inherits this env var doesn't ship without a gate.
	//
	// We log both the raw env value AND the resolved DevMode so an
	// operator running `make no-auth-run` can confirm at a glance
	// whether the env reached the binary. The prior single-line log
	// was easy to miss in noisy startup output.
	rawEnv := os.Getenv("ARGUS_AUTH_DISABLED")
	devMode := cfg.DevMode
	a.logger.Info("auth: resolving devMode",
		slog.String("ARGUS_AUTH_DISABLED", rawEnv),
		slog.Bool("cfg.DevMode", cfg.DevMode),
		slog.String("ARGUS_API_BIND", os.Getenv("ARGUS_API_BIND")),
	)
	if devMode && !isLoopbackBind(os.Getenv("ARGUS_API_BIND")) {
		a.logger.Warn("ARGUS_AUTH_DISABLED ignored — API is not bound to loopback",
			slog.String("bind", os.Getenv("ARGUS_API_BIND")),
		)
		devMode = false
	}
	if devMode {
		a.logger.Warn("════════════════════════════════════════════════════════════")
		a.logger.Warn("AUTH IS DISABLED — every /api request is unauthenticated.")
		a.logger.Warn("Local-dev mode only. /auth/providers returns authDisabled=true.")
		a.logger.Warn("════════════════════════════════════════════════════════════")
	} else {
		a.logger.Info("auth: dev-mode OFF — login required")
	}
	a.auth = &authState{
		store:       store,
		oidc:        auth.NewOIDCManager(store, a.logger, providers...),
		allowSignup: cfg.AllowLocalSignup,
		devMode:     devMode,
	}
	// Best-effort cleanup loop — runs every hour so the session table
	// doesn't grow without bound on long-running installs.
	go func() {
		t := time.NewTicker(1 * time.Hour)
		defer t.Stop()
		for range t.C {
			store.PurgeExpired()
		}
	}()
}

// AuthRoutes returns a function suitable for http.HandleFunc-style
// registration. Keeps the routing surface in one place.
func (a *App) AuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth/providers", a.handleAuthProviders)
	mux.HandleFunc("/auth/register", a.handleAuthRegister)
	mux.HandleFunc("/auth/login", a.handleAuthLogin)
	mux.HandleFunc("/auth/logout", a.handleAuthLogout)
	mux.HandleFunc("/auth/me", a.handleAuthMe)
	mux.HandleFunc("/auth/oauth/start", a.handleOAuthStart)
	mux.HandleFunc("/auth/oauth/poll", a.handleOAuthPoll)
	mux.HandleFunc("/auth/google/callback", a.handleOAuthCallback)
	mux.HandleFunc("/auth/oidc/callback", a.handleOAuthCallback)
}

// preflight applies the same CORS gate as /api/*. Auth endpoints don't
// require a session token — they're how you GET one — but we still
// reject unknown origins to stop random sites from probing them.
func (a *App) authPreflight(w http.ResponseWriter, r *http.Request) bool {
	if !applyCORS(w, r, allowedOrigins()) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return false
	}
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return false
	}
	if a.auth == nil {
		http.Error(w, "auth subsystem not initialized", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().WarnContext(context.Background(), "encode response failed",
			slog.String("error", err.Error()),
		)
	}
}

func (a *App) handleAuthProviders(w http.ResponseWriter, r *http.Request) {
	if !a.authPreflight(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"providers":    a.auth.oidc.EnabledProviders(),
		"allowSignup":  a.auth.allowSignup,
		"authDisabled": a.auth.devMode,
	})
}

type registerRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (a *App) handleAuthRegister(w http.ResponseWriter, r *http.Request) {
	if !a.authPreflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.auth.allowSignup {
		http.Error(w, "self-registration is disabled — ask your administrator", http.StatusForbidden)
		return
	}
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	user, err := a.auth.store.CreateLocalUser(req.Email, req.Name, req.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	sess, err := a.auth.store.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, loginResponse(user, sess))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *App) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if !a.authPreflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	user, err := a.auth.store.AuthenticateLocal(req.Email, req.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	sess, err := a.auth.store.CreateSession(user.ID)
	if err != nil {
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, loginResponse(user, sess))
}

func (a *App) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if !a.authPreflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tok := bearerFromRequest(r)
	if err := a.auth.store.RevokeSession(tok); err != nil {
		http.Error(w, "logout failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *App) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if !a.authPreflight(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, err := a.auth.store.ValidateSession(bearerFromRequest(r))
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

type oauthStartRequest struct {
	Provider string `json:"provider"` // "google" or "oidc"
}

func (a *App) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !a.authPreflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req oauthStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	prov := auth.Provider(req.Provider)
	if !prov.Valid() || prov == auth.ProviderLocal {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}
	if !a.auth.oidc.IsEnabled(prov) {
		http.Error(w, "provider not configured", http.StatusBadRequest)
		return
	}
	url, state, err := a.auth.oidc.StartLogin(r.Context(), prov)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"authUrl": url,
		"state":   state,
	})
}

func (a *App) handleOAuthPoll(w http.ResponseWriter, r *http.Request) {
	if !a.authPreflight(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if state == "" {
		http.Error(w, "missing state", http.StatusBadRequest)
		return
	}
	tok, errMsg, err := a.auth.oidc.PollPending(state)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	if errMsg != "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "error", "error": errMsg})
		return
	}
	if tok == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "pending"})
		return
	}
	user, err := a.auth.store.ValidateSession(tok)
	if err != nil {
		http.Error(w, "session lost", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"user":   user,
		"token":  tok,
	})
}

// handleOAuthCallback is hit by the user's browser after the upstream
// provider redirects back. We don't apply the same JSON CORS dance —
// this is a top-level navigation, not an XHR. We render a small HTML
// page; the desktop frontend is polling /auth/oauth/poll separately.
func (a *App) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		auth.RenderCallback(w, false, "auth subsystem not initialized")
		return
	}
	q := r.URL.Query()
	state := q.Get("state")
	if errParam := q.Get("error"); errParam != "" {
		desc := q.Get("error_description")
		if desc == "" {
			desc = errParam
		}
		a.auth.oidc.MarkPendingError(state, desc)
		auth.RenderCallback(w, false, desc)
		return
	}
	code := q.Get("code")
	if code == "" || state == "" {
		auth.RenderCallback(w, false, "missing code or state in callback")
		return
	}
	if _, err := a.auth.oidc.CompleteLogin(r.Context(), state, code); err != nil {
		a.auth.oidc.MarkPendingError(state, err.Error())
		auth.RenderCallback(w, false, err.Error())
		return
	}
	auth.RenderCallback(w, true, "Login complete.")
}

// loginResponse is the shared shape returned by /auth/login,
// /auth/register, and the OAuth poll endpoint.
func loginResponse(user *auth.User, sess *auth.Session) map[string]any {
	return map[string]any{
		"user":      user,
		"token":     sess.Token,
		"expiresAt": sess.ExpiresAt.Unix(),
	}
}

// writeAuthError maps the auth package's sentinel errors to HTTP codes.
// Anything we don't recognize becomes 500 — internal errors shouldn't
// leak to the client as 4xx.
func writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrSessionInvalid):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, auth.ErrEmailTaken), errors.Is(err, auth.ErrProviderMismatch):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, auth.ErrInvalidEmail), errors.Is(err, auth.ErrWeakPassword):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, auth.ErrOAuthDisabled), errors.Is(err, auth.ErrOAuthState):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// bearerFromRequest extracts the session token from Authorization header.
// Empty string if absent or malformed.
func bearerFromRequest(r *http.Request) string {
	hdr := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(hdr, prefix) {
		return ""
	}
	return strings.TrimSpace(hdr[len(prefix):])
}
