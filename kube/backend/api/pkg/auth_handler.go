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

	gochi "github.com/go-chi/chi/v5"

	"github.com/argues/argus/internal/auth"
	"github.com/argues/argus/internal/config"
)

// Auth route paths. Kept as constants so the canary tests + the
// frontend's hard-coded URLs can grep for them rather than chasing
// string literals scattered through the file.
const (
	authPathProviders            = "/auth/providers"
	authPathRegister             = "/auth/register"
	authPathLogin                = "/auth/login"
	authPathLogout               = "/auth/logout"
	authPathMe                   = "/auth/me"
	authPathOAuthStart           = "/auth/oauth/start"
	authPathOAuthPoll            = "/auth/oauth/poll"
	authPathOAuthGoogleCallback  = "/auth/google/callback"
	authPathOAuthOIDCCallback    = "/auth/oidc/callback"
	authPathAppleStart           = "/auth/apple/start"
	authPathAppleCallback        = "/auth/apple/callback"
	authPathPasskeyRegisterBegin = "/auth/passkey/register/begin"
	authPathPasskeyRegisterEnd   = "/auth/passkey/register/finish"
	authPathPasskeyLoginBegin    = "/auth/passkey/login/begin"
	authPathPasskeyLoginEnd      = "/auth/passkey/login/finish"
	authPathPasskeyList          = "/auth/passkey/list"
	authPathPasskeyDeleteByID    = "/auth/passkey/{id}"

	urlParamPasskeyID = "id"
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
	apple       *auth.AppleManager   // nil unless Sign in with Apple is configured
	passkey     *auth.PasskeyManager // nil when ARGUS_PASSKEY_ENABLED=false
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
	// Sign in with Apple is wired separately — its quirks (dynamic
	// JWT client_secret, form_post callback, first-auth-only email)
	// don't fit the generic OIDCManager. We attempt registration only
	// when all four required fields are present; partial config is
	// logged and skipped so a typo doesn't crash boot.
	if cfg.AppleServicesID != "" && cfg.AppleTeamID != "" && cfg.AppleKeyID != "" && cfg.ApplePrivateKey != "" {
		am, err := auth.NewAppleManager(store, a.logger, auth.AppleConfig{
			ServicesID:    cfg.AppleServicesID,
			TeamID:        cfg.AppleTeamID,
			KeyID:         cfg.AppleKeyID,
			PrivateKeyPEM: cfg.ApplePrivateKey,
			DisplayName:   cfg.AppleDisplayName,
			RedirectURL:   strings.TrimRight(cfg.PublicBaseURL, "/") + "/auth/apple/callback",
		})
		if err != nil {
			a.logger.Warn("Sign in with Apple not registered", slog.String("error", err.Error()))
		} else {
			a.auth.apple = am
			a.logger.Info("Sign in with Apple registered", slog.String("servicesID", cfg.AppleServicesID))
		}
	}
	if cfg.PasskeyEnabled {
		mgr, err := auth.NewPasskeyManager(cfg.PasskeyRPID, cfg.PasskeyRPName, cfg.PasskeyRPOrigin, store.PasskeyStore())
		if err != nil {
			a.logger.Error("passkey: disabled — config rejected",
				slog.String("error", err.Error()),
				slog.String("rp_id", cfg.PasskeyRPID),
				slog.String("rp_origin", cfg.PasskeyRPOrigin),
			)
		} else {
			a.auth.passkey = mgr
			a.logger.Info("passkey: enabled",
				slog.String("rp_id", cfg.PasskeyRPID),
				slog.String("rp_origin", cfg.PasskeyRPOrigin),
			)
		}
	}
	// Best-effort cleanup loop — runs every hour so the session table
	// doesn't grow without bound on long-running installs. Gated by
	// authJanitorOnce because SetupAuth is re-invoked on every Settings
	// save to hot-reload OAuth credentials; we don't want a new
	// goroutine leaking on each save. The goroutine re-reads
	// a.auth.passkey on each tick so the live (post-reload) value wins.
	a.authJanitorOnce.Do(func() {
		go func() {
			t := time.NewTicker(1 * time.Hour)
			defer t.Stop()
			for range t.C {
				store.PurgeExpired()
				if a.auth != nil && a.auth.passkey != nil {
					a.auth.passkey.PurgeExpired()
				}
			}
		}()
	})
}

// AuthRoutes builds the /auth/* router and returns it as a chi sub-
// tree the parent server mounts. The chi-style router (in place of
// the previous http.ServeMux-based registration) follows the same
// shape pim-agl-online-data-relay/internal/api uses: a Routes()-style
// builder that returns http.Handler so the parent owner can compose,
// wrap, and mount it freely.
func (a *App) AuthRoutes() http.Handler {
	r := gochi.NewRouter()

	r.Get(authPathProviders, a.handleAuthProviders)
	r.Post(authPathRegister, a.handleAuthRegister)
	r.Post(authPathLogin, a.handleAuthLogin)
	r.Post(authPathLogout, a.handleAuthLogout)
	r.Get(authPathMe, a.handleAuthMe)

	r.Get(authPathOAuthStart, a.handleOAuthStart)
	r.Get(authPathOAuthPoll, a.handleOAuthPoll)
	r.Get(authPathOAuthGoogleCallback, a.handleOAuthCallback)
	r.Get(authPathOAuthOIDCCallback, a.handleOAuthCallback)

	// Apple uses a separate start endpoint because its login URL is
	// hand-built (response_mode=form_post is non-standard) and a
	// separate POST callback because Apple form-posts instead of
	// redirecting via GET.
	r.Get(authPathAppleStart, a.handleAppleStart)
	r.Post(authPathAppleCallback, a.handleAppleCallback)

	// Passkey (WebAuthn) endpoints. Register-begin/finish are
	// authenticated (you can only add a passkey to your own account);
	// login-begin/finish are public so a user without a session can
	// sign in.
	r.Post(authPathPasskeyRegisterBegin, a.handlePasskeyRegisterBegin)
	r.Post(authPathPasskeyRegisterEnd, a.handlePasskeyRegisterFinish)
	r.Post(authPathPasskeyLoginBegin, a.handlePasskeyLoginBegin)
	r.Post(authPathPasskeyLoginEnd, a.handlePasskeyLoginFinish)
	r.Get(authPathPasskeyList, a.handlePasskeyList)
	r.Delete(authPathPasskeyDeleteByID, a.handlePasskeyDelete)

	return r
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
	providers := a.auth.oidc.EnabledProviders()
	// Surface Apple alongside the OIDC providers so the frontend
	// renders a single unified provider list. The frontend recognizes
	// "apple" by name and uses /auth/apple/start instead of the generic
	// /auth/oauth/start.
	if a.auth.apple.Configured() {
		providers = append(providers, auth.ProviderInfo{
			Name:        string(auth.ProviderApple),
			DisplayName: a.auth.apple.DisplayName(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"providers":      providers,
		"allowSignup":    a.auth.allowSignup,
		"authDisabled":   a.auth.devMode,
		"passkeyEnabled": a.auth.passkey != nil,
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
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrSessionInvalid),
		errors.Is(err, auth.ErrPasskeySessionInvalid), errors.Is(err, auth.ErrPasskeyNotFound):
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

// handleAppleStart returns the URL the browser should open to begin a
// Sign in with Apple flow. The frontend handles the URL-opening + poll
// dance via the existing /auth/oauth/poll endpoint — Apple writes the
// session token into oauth_pending under the same state nonce.
func (a *App) handleAppleStart(w http.ResponseWriter, r *http.Request) {
	if !a.authPreflight(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !a.auth.apple.Configured() {
		http.Error(w, "Sign in with Apple not configured", http.StatusBadRequest)
		return
	}
	url, state, err := a.auth.apple.Start()
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"authUrl": url, "state": state})
}

// handleAppleCallback receives Apple's form_post response. Unlike every
// other OIDC provider the body is form-encoded, not a query string.
// First-authorization responses include a `user` JSON blob with the
// display name; subsequent sign-ins do not.
func (a *App) handleAppleCallback(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil || !a.auth.apple.Configured() {
		auth.RenderCallback(w, false, "Sign in with Apple not configured")
		return
	}
	if r.Method != http.MethodPost {
		// Apple is strict about form_post; a GET here means a
		// misconfiguration (response_mode missing). Render a hint
		// rather than a generic 405 so operators can diagnose.
		auth.RenderCallback(w, false, "Apple callback must be POST (response_mode=form_post)")
		return
	}
	if err := r.ParseForm(); err != nil {
		auth.RenderCallback(w, false, "could not parse form body")
		return
	}
	state := r.PostForm.Get("state")
	if errParam := r.PostForm.Get("error"); errParam != "" {
		a.auth.oidc.MarkPendingError(state, errParam)
		auth.RenderCallback(w, false, errParam)
		return
	}
	code := r.PostForm.Get("code")
	userJSON := r.PostForm.Get("user")
	if code == "" || state == "" {
		auth.RenderCallback(w, false, "missing code or state in callback")
		return
	}
	if _, err := a.auth.apple.Complete(r.Context(), state, code, userJSON); err != nil {
		a.auth.oidc.MarkPendingError(state, err.Error())
		auth.RenderCallback(w, false, err.Error())
		return
	}
	auth.RenderCallback(w, true, "Sign-in complete.")
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
