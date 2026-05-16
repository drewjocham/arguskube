package pkg

import (
	"errors"
	"fmt"

	"github.com/argues/argus/internal/workspace"
)

// app_workspace.go — Wails bindings for the Workspace integrations
// (Google Docs/Sheets/Tasks). Phase 1A surface only: listing,
// deleting, OAuth start/complete. Per-service operations (read doc,
// append row, create task, …) land in 1B+.
//
// User scoping: methods take the caller's sessionToken so userID is
// resolved consistently between desktop (Wails) and SaaS (HTTP). The
// frontend already holds the session token in secretstore.

// WorkspaceConnectionView is the redacted projection sent to the
// frontend. Identical to workspace.Connection (no tokens travel via
// this struct anyway — they live in the separate workspace_tokens
// table) but we keep the boundary explicit so a future field-add on
// Connection doesn't accidentally leak.
type WorkspaceConnectionView struct {
	ID                  string `json:"id"`
	Service             string `json:"service"`
	ExternalWorkspaceID string `json:"external_workspace_id,omitempty"`
	DisplayName         string `json:"display_name"`
	Email               string `json:"email,omitempty"`
	AvatarURL           string `json:"avatar_url,omitempty"`
	ConnectedAt         int64  `json:"connected_at"`
	UpdatedAt           int64  `json:"updated_at"`
}

func toWorkspaceView(c workspace.Connection) WorkspaceConnectionView {
	return WorkspaceConnectionView{
		ID:                  c.ID,
		Service:             string(c.Service),
		ExternalWorkspaceID: c.ExternalWorkspaceID,
		DisplayName:         c.DisplayName,
		Email:               c.Email,
		AvatarURL:           c.AvatarURL,
		ConnectedAt:         c.ConnectedAt,
		UpdatedAt:           c.UpdatedAt,
	}
}

// workspaceAvailable returns true once the App has a Manager wired —
// false in SaaS mode (where the desktop binary doesn't run a workspace
// stack) or before SetupAuth completes.
func (a *App) workspaceAvailable() bool {
	return a.workspace != nil
}

// devModeUserID is the stable synthetic user id workspace methods see
// when ARGUS_AUTH_DISABLED bypass is on. Stable across runs so the
// dev's connections persist in workspace_connections under one row
// per (service, externalWorkspaceID) instead of accumulating duplicates
// every restart. Format mirrors auth.Store's id shape (string opaque).
const devModeUserID = "local-dev-user"

// workspaceUserID resolves the caller's user id from the session
// token. Wails methods can't read HTTP headers so the frontend must
// pass the token explicitly — same pattern other gated methods use.
//
// When ARGUS_AUTH_DISABLED is on we short-circuit to a stable synthetic
// user. Without this, the frontend has no session token to send (the
// LoginView was bypassed) and every workspace call fails with
// "invalid session" — defeating the point of dev-mode.
func (a *App) workspaceUserID(token string) (string, error) {
	if a.auth == nil || a.auth.store == nil {
		return "", errors.New("workspace: auth not configured")
	}
	if a.auth.devMode {
		return devModeUserID, nil
	}
	user, err := a.auth.store.ValidateSession(token)
	if err != nil || user == nil {
		return "", errors.New("workspace: invalid session")
	}
	return user.ID, nil
}

// ListWorkspaceServices reports which services have a Provider wired
// in this build. The UI uses it to gray out "Connect Google Docs"
// before that adapter ships.
func (a *App) ListWorkspaceServices() []string {
	if !a.workspaceAvailable() {
		return []string{}
	}
	svcs := a.workspace.AvailableServices()
	out := make([]string, 0, len(svcs))
	for _, s := range svcs {
		out = append(out, string(s))
	}
	return out
}

// ListWorkspaceConnections returns every connection the calling user
// owns. Tokens are NOT included.
//
// Wails methods on App don't take context.Context — the runtime maps
// every Go parameter to a JS arg, and the codebase convention is to
// pull the App's context from a.appCtx() internally instead. Breaking
// that convention produces "received N arguments to method, expected
// N+1" on the JS side because Wails counts the ctx parameter.
func (a *App) ListWorkspaceConnections(sessionToken string) ([]WorkspaceConnectionView, error) {
	if !a.workspaceAvailable() {
		return nil, errors.New("workspace: not configured in this build mode")
	}
	userID, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return nil, err
	}
	conns, err := a.workspace.List(a.appCtx(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceConnectionView, 0, len(conns))
	for _, c := range conns {
		out = append(out, toWorkspaceView(c))
	}
	return out, nil
}

// StartWorkspaceConnect kicks off an OAuth flow for the given service.
// Returns the URL the desktop must open in the browser and the state
// nonce the callback will carry.
func (a *App) StartWorkspaceConnect(sessionToken, service, redirectURL string) (workspace.AuthURL, error) {
	if !a.workspaceAvailable() {
		return workspace.AuthURL{}, errors.New("workspace: not configured")
	}
	userID, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return workspace.AuthURL{}, err
	}
	return a.workspace.Start(a.appCtx(), userID, workspace.Service(service), redirectURL)
}

// CompleteWorkspaceConnect is the callback handler. The frontend
// (after the system browser redirects back to the local listener)
// hands us the state + code; we exchange and persist.
func (a *App) CompleteWorkspaceConnect(service, state, code string) (WorkspaceConnectionView, error) {
	if !a.workspaceAvailable() {
		return WorkspaceConnectionView{}, errors.New("workspace: not configured")
	}
	c, err := a.workspace.Complete(a.appCtx(), workspace.Service(service), state, code)
	if err != nil {
		return WorkspaceConnectionView{}, fmt.Errorf("workspace: %w", err)
	}
	return toWorkspaceView(c), nil
}

// DeleteWorkspaceConnection removes a connection. Idempotent — a
// missing id returns nil so double-clicks don't trip.
func (a *App) DeleteWorkspaceConnection(sessionToken, id string) error {
	if !a.workspaceAvailable() {
		return errors.New("workspace: not configured")
	}
	if _, err := a.workspaceUserID(sessionToken); err != nil {
		return err
	}
	if err := a.workspace.Delete(a.appCtx(), id); err != nil && !errors.Is(err, workspace.ErrNotFound) {
		return err
	}
	return nil
}
