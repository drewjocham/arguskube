package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// ProviderConfig describes a single OIDC issuer the user can sign in
// against. Both Google and the generic corporate provider use this
// shape — Google is just an OIDC provider whose issuer is well-known.
type ProviderConfig struct {
	Name         Provider // ProviderGoogle or ProviderOIDC
	DisplayName  string   // shown on the login button, e.g. "Google" or "Acme SSO"
	Issuer       string   // OIDC issuer URL — discovery doc lives at <issuer>/.well-known/openid-configuration
	ClientID     string
	ClientSecret string // optional for public clients; required for Google web app
	RedirectURL  string // e.g. http://127.0.0.1:8080/auth/google/callback
	Scopes       []string
}

// OIDCManager discovers providers lazily and serves login URLs +
// callback exchanges. Discovery is cached because issuers' JWKS need
// to refresh, but coreos/go-oidc handles that internally.
type OIDCManager struct {
	store    *Store
	logger   loggerLike
	configs  map[Provider]ProviderConfig
	mu       sync.Mutex
	provider map[Provider]*oidc.Provider // cache of resolved providers
}

type loggerLike interface {
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
	Error(msg string, args ...any)
}

func NewOIDCManager(store *Store, logger loggerLike, configs ...ProviderConfig) *OIDCManager {
	m := &OIDCManager{
		store:    store,
		logger:   logger,
		configs:  make(map[Provider]ProviderConfig, len(configs)),
		provider: make(map[Provider]*oidc.Provider),
	}
	for _, c := range configs {
		if !c.Name.Valid() || c.Name == ProviderLocal {
			continue
		}
		if c.ClientID == "" || c.Issuer == "" {
			continue
		}
		if len(c.Scopes) == 0 {
			c.Scopes = []string{oidc.ScopeOpenID, "email", "profile"}
		}
		m.configs[c.Name] = c
	}
	return m
}

func (m *OIDCManager) IsEnabled(p Provider) bool {
	_, ok := m.configs[p]
	return ok
}

// EnabledProviders returns the list of configured providers in a
// stable shape the frontend can render as login buttons.
func (m *OIDCManager) EnabledProviders() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(m.configs))
	for p, c := range m.configs {
		out = append(out, ProviderInfo{
			Name:        string(p),
			DisplayName: c.DisplayName,
		})
	}
	return out
}

// ProviderInfo is the JSON shape returned by /auth/providers.
type ProviderInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// StartLogin returns the upstream auth URL the user should visit, plus
// the state token the frontend will poll for completion. The caller
// is expected to open the auth URL in the system browser (Wails:
// runtime.BrowserOpenURL).
func (m *OIDCManager) StartLogin(ctx context.Context, p Provider) (authURL, state string, err error) {
	cfg, ok := m.configs[p]
	if !ok {
		return "", "", ErrOAuthDisabled
	}
	prov, err := m.resolveProvider(ctx, cfg)
	if err != nil {
		return "", "", fmt.Errorf("auth: discover %s: %w", p, err)
	}

	state, err = randomToken(24)
	if err != nil {
		return "", "", err
	}
	verifier, err := randomToken(32) // PKCE — defends against authorization-code interception on the loopback callback
	if err != nil {
		return "", "", err
	}
	challenge := pkceChallenge(verifier)

	if _, err := m.store.db.Exec(`INSERT INTO oauth_pending (state, pkce_verifier, provider, created_at)
		VALUES (?, ?, ?, ?)`, state, verifier, string(p), time.Now().Unix()); err != nil {
		return "", "", fmt.Errorf("auth: persist oauth state: %w", err)
	}

	oauthCfg := m.oauthConfig(cfg, prov)
	authURL = oauthCfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	return authURL, state, nil
}

// CompleteLogin exchanges the code for an ID token, verifies it, and
// upserts the user. Returns a fresh session — the same shape the
// local login path returns.
func (m *OIDCManager) CompleteLogin(ctx context.Context, state, code string) (*Session, error) {
	row := m.store.db.QueryRow(`SELECT pkce_verifier, provider, completed_at, session_token
		FROM oauth_pending WHERE state = ?`, state)
	var verifier, providerStr, sessionTok string
	var completedAt int64
	if err := row.Scan(&verifier, &providerStr, &completedAt, &sessionTok); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOAuthState
		}
		return nil, fmt.Errorf("auth: load oauth state: %w", err)
	}
	if completedAt > 0 {
		return nil, ErrOAuthState // replay
	}
	prov := Provider(providerStr)
	cfg, ok := m.configs[prov]
	if !ok {
		return nil, ErrOAuthDisabled
	}
	oidcProv, err := m.resolveProvider(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tok, err := m.oauthConfig(cfg, oidcProv).Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: token exchange: %w", err)
	}
	rawID, _ := tok.Extra("id_token").(string)
	if rawID == "" {
		return nil, errors.New("auth: provider returned no id_token")
	}
	verifier2 := oidcProv.Verifier(&oidc.Config{ClientID: cfg.ClientID})
	idTok, err := verifier2.Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("auth: verify id_token: %w", err)
	}
	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		PreferredName string `json:"preferred_username"`
	}
	if err := idTok.Claims(&claims); err != nil {
		return nil, fmt.Errorf("auth: parse id_token claims: %w", err)
	}
	if claims.Sub == "" {
		return nil, errors.New("auth: id_token missing sub claim")
	}
	// Email verification: required for Google so a malicious user
	// can't sign up with someone else's address. Generic OIDC providers
	// (corporate SSO) typically don't include the field; trust them.
	if prov == ProviderGoogle && !claims.EmailVerified {
		return nil, errors.New("auth: Google email not verified")
	}
	name := claims.Name
	if name == "" {
		name = claims.PreferredName
	}
	if name == "" {
		name = claims.Email
	}

	user, err := m.store.UpsertOAuthUser(prov, claims.Sub, claims.Email, name)
	if err != nil {
		return nil, err
	}
	sess, err := m.store.CreateSession(user.ID)
	if err != nil {
		return nil, err
	}

	// Mark this state used so the frontend's poll can pick it up,
	// and so the same code can't be replayed.
	if _, err := m.store.db.Exec(`UPDATE oauth_pending SET session_token = ?, completed_at = ? WHERE state = ?`,
		sess.Token, time.Now().Unix(), state); err != nil {
		return nil, fmt.Errorf("auth: mark oauth_pending: %w", err)
	}
	return sess, nil
}

// PollPending is called by the frontend after the user opens the
// browser. It returns the freshly-created session token if the
// callback has completed, or ("", nil) if still pending. After it
// returns a token, the row is cleared so a second poll won't return
// it again.
func (m *OIDCManager) PollPending(state string) (token, errMsg string, err error) {
	row := m.store.db.QueryRow(`SELECT session_token, error, created_at FROM oauth_pending WHERE state = ?`, state)
	var tok, e string
	var created int64
	if err := row.Scan(&tok, &e, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", ErrOAuthState
		}
		return "", "", err
	}
	if e != "" {
		_, _ = m.store.db.Exec(`DELETE FROM oauth_pending WHERE state = ?`, state)
		return "", e, nil
	}
	if tok == "" {
		// Pending — but enforce a 15-minute upper bound so abandoned
		// flows don't sit in the DB forever.
		if time.Since(time.Unix(created, 0)) > 15*time.Minute {
			_, _ = m.store.db.Exec(`DELETE FROM oauth_pending WHERE state = ?`, state)
			return "", "", ErrOAuthState
		}
		return "", "", nil
	}
	_, _ = m.store.db.Exec(`DELETE FROM oauth_pending WHERE state = ?`, state)
	return tok, "", nil
}

// MarkPendingError stamps an error message into oauth_pending so the
// poller can surface it to the user — much friendlier than a silent
// timeout when the callback failed.
func (m *OIDCManager) MarkPendingError(state, msg string) {
	if state == "" {
		return
	}
	_, _ = m.store.db.Exec(`UPDATE oauth_pending SET error = ? WHERE state = ?`, msg, state)
}

func (m *OIDCManager) resolveProvider(ctx context.Context, cfg ProviderConfig) (*oidc.Provider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.provider[cfg.Name]; ok {
		return p, nil
	}
	dctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	p, err := oidc.NewProvider(dctx, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	m.provider[cfg.Name] = p
	return p, nil
}

func (m *OIDCManager) oauthConfig(cfg ProviderConfig, prov *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     prov.Endpoint(),
		RedirectURL:  cfg.RedirectURL,
		Scopes:       cfg.Scopes,
	}
}

// pkceChallenge derives the S256 challenge from a verifier per RFC 7636.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// CallbackTemplate is the HTML returned to the browser when the OAuth
// callback completes. The desktop app's frontend is polling
// /auth/oauth/poll separately; this page just lets the user know
// they can close the tab.
const CallbackTemplate = `<!doctype html>
<html><head><meta charset="utf-8"><title>Sign-in complete</title>
<style>body{background:#1a1c1e;color:#e3e6eb;font-family:-apple-system,BlinkMacSystemFont,sans-serif;
display:flex;align-items:center;justify-content:center;height:100vh;margin:0;}
.box{max-width:32rem;text-align:center;padding:2rem;border:1px solid #2c2f33;border-radius:.5rem;background:#202326;}
h1{margin:0 0 .5rem;font-size:1.25rem;} p{margin:.25rem 0;color:#a3a8b1;}</style></head>
<body><div class="box"><h1>{{TITLE}}</h1><p>{{MSG}}</p>
<p style="margin-top:1.5rem;font-size:.85rem;">You can close this tab and return to KubeWatcher.</p>
</div></body></html>`

// RenderCallback writes a small confirmation page. msg is plain text;
// we escape it before injecting into the template.
func RenderCallback(w http.ResponseWriter, ok bool, msg string) {
	title := "Sign-in failed"
	if ok {
		title = "You're signed in"
	}
	body := strings.ReplaceAll(CallbackTemplate, "{{TITLE}}", htmlEscape(title))
	body = strings.ReplaceAll(body, "{{MSG}}", htmlEscape(msg))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if ok {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	_, _ = w.Write([]byte(body))
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

// MarshalProviders is a tiny helper so handlers stay JSON-encoder-free.
func MarshalProviders(p []ProviderInfo) ([]byte, error) {
	return json.Marshal(p)
}
