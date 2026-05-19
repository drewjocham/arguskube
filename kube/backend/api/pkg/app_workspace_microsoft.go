package pkg

import (
	"errors"
	"fmt"
	"strings"

	"github.com/argues/argus/internal/workspace"
)

// app_workspace_microsoft.go — Wails bindings for Microsoft 365.
// OAuth 2.0 flow via the standard Start/Complete pattern.

// ConnectMicrosoft initiates the OAuth flow for Microsoft 365.
// Returns the auth URL the frontend should open in the system browser.
func (a *App) ConnectMicrosoft(sessionToken string) (string, string, error) {
	if !a.workspaceAvailable() {
		return "", "", errors.New("workspace: not configured")
	}
	userID, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return "", "", err
	}
	auth, err := a.workspace.Start(a.appCtx(), userID, workspace.ServiceMicrosoft, "")
	if err != nil {
		return "", "", err
	}
	return auth.URL, auth.State, nil
}

// CompleteMicrosoftConnect finishes the Microsoft OAuth flow.
func (a *App) CompleteMicrosoftConnect(state, code string) (WorkspaceConnectionView, error) {
	if !a.workspaceAvailable() {
		return WorkspaceConnectionView{}, errors.New("workspace: not configured")
	}
	c, err := a.workspace.Complete(a.appCtx(), workspace.ServiceMicrosoft, state, code)
	if err != nil {
		return WorkspaceConnectionView{}, err
	}
	return toWorkspaceView(c), nil
}

// app_workspace_custom.go — Wails bindings for manual/custom connections.

// ConnectCustom stores a manual connection with a display name and optional notes.
func (a *App) ConnectCustom(sessionToken, displayName, notes string) (WorkspaceConnectionView, error) {
	if strings.TrimSpace(displayName) == "" {
		return WorkspaceConnectionView{}, errors.New("custom: display name is required")
	}
	if !a.workspaceAvailable() {
		return WorkspaceConnectionView{}, errors.New("workspace: not configured")
	}
	userID, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return WorkspaceConnectionView{}, err
	}
	c, err := a.workspace.DirectConnect(a.appCtx(), workspace.Connection{
		UserID:      userID,
		Service:     workspace.ServiceCustom,
		DisplayName: displayName,
	}, workspace.Token{
		AccessToken:  notes,
		RefreshToken: displayName,
		TokenType:    "manual",
	})
	if err != nil {
		return WorkspaceConnectionView{}, fmt.Errorf("custom: store connection: %w", err)
	}
	return toWorkspaceView(c), nil
}
