package pkg

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/argues/argus/internal/secretstore"
)

// SessionToken — Wails bindings for the frontend auth store. The
// frontend persists ONLY the session token, and only via this prefix.
// Two layers of safety:
//
//  1. The key the frontend passes is ignored — every call routes to
//     a single hard-coded keychain entry. The frontend can't enumerate
//     or address arbitrary entries.
//  2. The Wails methods are intentionally NOT in the httpExposedMethods
//     allowlist. Even if SaaS mode is reachable, /api/SetSessionToken
//     returns 403 from the existing dispatcher.
//
// The single supported entry is the active session JWT. Adding more
// secret kinds (refresh tokens, MCP secrets, …) is the right reason to
// generalize this; until then a fixed key keeps the surface tight.

const sessionTokenKey = "argus.session.token"
const secretStoreTimeout = 4 * time.Second

var (
	secretStoreMu       sync.Mutex
	secretStoreSingleton secretstore.Store
)

// secretStoreFor returns the process-wide Store. We init lazily so the
// Wails App doesn't take a hard dep on the secretstore package at
// construction time (keeps the App god-object slightly less god-like —
// the existing .context.md debt note already calls that out).
func secretStoreFor() secretstore.Store {
	secretStoreMu.Lock()
	defer secretStoreMu.Unlock()
	if secretStoreSingleton == nil {
		secretStoreSingleton = secretstore.New("Argus")
	}
	return secretStoreSingleton
}

// SecretStoreInfo describes which backend is providing secret storage.
// The frontend uses this to render a one-liner in Settings ("Session
// stored in macOS Keychain") so the user knows where their token lives.
type SecretStoreInfo struct {
	Backend   string `json:"backend"`
	Available bool   `json:"available"`
}

// GetSecretStoreInfo returns the current backend label.
func (a *App) GetSecretStoreInfo() SecretStoreInfo {
	s := secretStoreFor()
	return SecretStoreInfo{
		Backend:   s.Backend(),
		Available: s.Backend() != "in-memory only",
	}
}

// SetSessionToken persists the active session JWT in OS-native storage.
// The token argument is treated as opaque — no parsing, no validation.
// We deliberately don't return a structured error: a Keychain failure
// is rare and the frontend already keeps a Pinia in-memory copy that
// works for the current session.
func (a *App) SetSessionToken(token string) error {
	if token == "" {
		return a.ClearSessionToken()
	}
	ctx, cancel := context.WithTimeout(a.bgCtx(), secretStoreTimeout)
	defer cancel()
	return secretStoreFor().Set(ctx, sessionTokenKey, token)
}

// GetSessionToken returns the persisted session JWT, or "" if no token
// has been stored. The boolean distinguishes "no entry" from "empty
// token" — only the second is a programming error.
func (a *App) GetSessionToken() (string, error) {
	ctx, cancel := context.WithTimeout(a.bgCtx(), secretStoreTimeout)
	defer cancel()
	v, _, err := secretStoreFor().Get(ctx, sessionTokenKey)
	if err != nil {
		// Don't leak the underlying CLI error verbatim — it commonly
		// includes the user's keychain path. Strip down to a generic.
		return "", errors.New("could not read session from secret store")
	}
	return strings.TrimSpace(v), nil
}

// ClearSessionToken deletes the persisted session. Idempotent — calling
// it when no entry exists is a no-op success.
func (a *App) ClearSessionToken() error {
	ctx, cancel := context.WithTimeout(a.bgCtx(), secretStoreTimeout)
	defer cancel()
	return secretStoreFor().Delete(ctx, sessionTokenKey)
}

// bgCtx returns a usable context even before Startup runs. We can't
// rely on a.ctx for secret-store calls that happen during early-boot
// (e.g. the frontend asking for the session token before the runtime
// is fully attached).
func (a *App) bgCtx() context.Context {
	if a != nil && a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
