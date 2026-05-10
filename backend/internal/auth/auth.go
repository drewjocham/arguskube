// Package auth provides user accounts, sessions, and OAuth/OIDC sign-in
// for the KubeWatcher SaaS API.
//
// Three sign-in paths are supported:
//
//  1. Local — email + password, hashed with bcrypt.
//  2. Google — OAuth 2.0 Authorization Code flow with PKCE, against
//     accounts.google.com discovery doc. Loopback callback (RFC 8252).
//  3. Generic OIDC — corporate SSO (Okta, Azure AD, Auth0, Keycloak…),
//     configured via KUBEWATCHER_OIDC_ISSUER + client id/secret.
//
// Sessions are opaque random tokens. The DB stores only their SHA-256
// hash so a leaked snapshot can't be used to log in.
package auth

import (
	"errors"
	"strings"
	"time"
)

// Provider identifies how a user authenticated. The set is closed —
// new providers must be added explicitly so DB rows can't claim
// arbitrary methods.
type Provider string

const (
	ProviderLocal  Provider = "local"
	ProviderGoogle Provider = "google"
	ProviderOIDC   Provider = "oidc"
)

func (p Provider) Valid() bool {
	switch p {
	case ProviderLocal, ProviderGoogle, ProviderOIDC:
		return true
	}
	return false
}

// User is the canonical account record. Password hash is empty for
// OAuth-only users; we never store an OAuth refresh/access token.
type User struct {
	ID              string    `json:"id"`
	Email           string    `json:"email"`
	Name            string    `json:"name"`
	Provider        Provider  `json:"provider"`
	ProviderSubject string    `json:"-"` // 'sub' claim from OIDC; never returned
	CreatedAt       time.Time `json:"createdAt"`
	LastLoginAt     time.Time `json:"lastLoginAt"`
}

// Session is what the frontend gets back after login. Only the raw
// token leaves the server; the DB has just its hash.
type Session struct {
	Token     string    `json:"token"`
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Standard sentinel errors so handlers can map cleanly to HTTP codes
// without relying on string matching.
var (
	ErrInvalidCredentials = errors.New("auth: invalid email or password")
	ErrEmailTaken         = errors.New("auth: email already registered")
	ErrUserNotFound       = errors.New("auth: user not found")
	ErrSessionInvalid     = errors.New("auth: session invalid or expired")
	ErrWeakPassword       = errors.New("auth: password must be at least 12 characters")
	ErrInvalidEmail       = errors.New("auth: email format invalid")
	ErrProviderMismatch   = errors.New("auth: account exists with a different sign-in method")
	ErrOAuthDisabled      = errors.New("auth: OAuth provider not configured")
	ErrOAuthState         = errors.New("auth: OAuth state mismatch or expired")
)

// normalizeEmail canonicalizes email for lookup. Stored as the raw
// lowercase form; we don't try to be clever about plus-aliases or
// dot-collapsing — those would create silent collisions for
// legitimately distinct accounts on some providers.
func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// validEmail is intentionally permissive — RFC 5321 is awful, and
// upstream OIDC providers do their own validation. We just rule out
// blatantly broken values that would crash downstream queries.
func validEmail(s string) bool {
	if len(s) < 3 || len(s) > 254 {
		return false
	}
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return false
	}
	return true
}
