// HTTP endpoints for the WebAuthn / passkey ceremony. The actual
// crypto and session-data handling lives in internal/auth/passkey.go;
// this file is just request parsing, auth gating, and JSON wiring.
package pkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	gochi "github.com/go-chi/chi/v5"

	"github.com/argues/argus/internal/auth"
)

// readCloser wraps a byte slice as an io.ReadCloser so we can feed it
// back as an http.Request.Body to the webauthn library (it expects a
// *http.Request whose body is the raw attestation/assertion JSON).
func readCloser(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
}

// passkeyAvailable is the common entrypoint guard. Centralizes the
// "feature disabled" 404 so each handler doesn't repeat the check.
func (a *App) passkeyAvailable(w http.ResponseWriter, r *http.Request) bool {
	if !a.authPreflight(w, r) {
		return false
	}
	if a.auth.passkey == nil {
		http.Error(w, "passkey sign-in is not enabled", http.StatusNotFound)
		return false
	}
	return true
}

// requireSession resolves the bearer token to a user, writing 401 if
// the session is missing or invalid. Used by the authenticated passkey
// endpoints (register, list, delete).
func (a *App) requireSession(w http.ResponseWriter, r *http.Request) *auth.User {
	user, err := a.auth.store.ValidateSession(bearerFromRequest(r))
	if err != nil {
		writeAuthError(w, err)
		return nil
	}
	return user
}

func (a *App) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	if !a.passkeyAvailable(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := a.requireSession(w, r)
	if user == nil {
		return
	}
	options, state, err := a.auth.passkey.BeginRegistration(user)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"publicKey": options.Response,
		"state":     state,
	})
}

type passkeyRegisterFinishRequest struct {
	State string          `json:"state"`
	Name  string          `json:"name"`
	Body  json.RawMessage `json:"credential"`
}

func (a *App) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	if !a.passkeyAvailable(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := a.requireSession(w, r)
	if user == nil {
		return
	}
	var req passkeyRegisterFinishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.State == "" || len(req.Body) == 0 {
		http.Error(w, "missing state or credential", http.StatusBadRequest)
		return
	}
	// FinishRegistration takes an *http.Request whose body is the
	// raw attestation JSON. We re-wrap the parsed `credential` field
	// into a synthetic request so the library can parse it the same
	// way it would a direct browser POST.
	inner := r.Clone(r.Context())
	inner.Body = readCloser(req.Body)
	inner.ContentLength = int64(len(req.Body))
	inner.Header.Set("Content-Type", "application/json")

	cred, err := a.auth.passkey.FinishRegistration(user, req.State, req.Name, inner)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         cred.ID,
		"name":       cred.Name,
		"createdAt":  cred.CreatedAt.Unix(),
		"lastUsedAt": cred.LastUsedAt.Unix(),
	})
}

func (a *App) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	if !a.passkeyAvailable(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	options, state, err := a.auth.passkey.BeginLogin()
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"publicKey": options.Response,
		"state":     state,
	})
}

type passkeyLoginFinishRequest struct {
	State string          `json:"state"`
	Body  json.RawMessage `json:"credential"`
}

func (a *App) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	if !a.passkeyAvailable(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req passkeyLoginFinishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.State == "" || len(req.Body) == 0 {
		http.Error(w, "missing state or credential", http.StatusBadRequest)
		return
	}
	inner := r.Clone(r.Context())
	inner.Body = readCloser(req.Body)
	inner.ContentLength = int64(len(req.Body))
	inner.Header.Set("Content-Type", "application/json")

	user, _, err := a.auth.passkey.FinishLogin(req.State, inner)
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

func (a *App) handlePasskeyList(w http.ResponseWriter, r *http.Request) {
	if !a.passkeyAvailable(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user := a.requireSession(w, r)
	if user == nil {
		return
	}
	creds, err := a.auth.passkey.ListCredentials(user.ID)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	out := make([]map[string]any, 0, len(creds))
	for _, c := range creds {
		out = append(out, map[string]any{
			"id":         c.ID,
			"name":       c.Name,
			"createdAt":  c.CreatedAt.Unix(),
			"lastUsedAt": c.LastUsedAt.Unix(),
			"transports": c.Transports,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"credentials": out})
}

// handlePasskeyDelete handles DELETE /auth/passkey/{id}. With the chi
// router registered in AuthRoutes, the {id} is a real URL parameter
// — no more r.URL.Path string-trimming. The method-allowed guard is
// also gone: chi enforces it at the route registration (r.Delete only
// matches DELETE).
func (a *App) handlePasskeyDelete(w http.ResponseWriter, r *http.Request) {
	if !a.passkeyAvailable(w, r) {
		return
	}
	user := a.requireSession(w, r)
	if user == nil {
		return
	}
	idStr := gochi.URLParam(r, urlParamPasskeyID)
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid credential id", http.StatusBadRequest)
		return
	}
	if err := a.auth.passkey.RevokeCredential(user.ID, id); err != nil {
		if errors.Is(err, auth.ErrPasskeyNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeAuthError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
