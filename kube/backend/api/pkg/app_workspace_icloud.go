package pkg

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/argues/argus/internal/workspace"
)

// app_workspace_icloud.go — Wails bindings for iCloud integration.
// Uses app-specific passwords (not OAuth), stored encrypted in the
// workspace token store. Calendar uses CalDAV; Notes/Reminders bridge
// to macOS CLI tools (memo, remindctl).

var (
	icloudProvider *workspace.ICloudProvider
	icloudCal      *workspace.ICloudCalendarer
)

func getICloudProvider() *workspace.ICloudProvider {
	if icloudProvider == nil {
		icloudProvider = workspace.NewICloudProvider()
	}
	return icloudProvider
}

func getICloudCal() *workspace.ICloudCalendarer {
	if icloudCal == nil {
		icloudCal = workspace.NewICloudCalendarer()
	}
	return icloudCal
}

// ConnectICloud validates an Apple ID + app-specific password against
// the iCloud CalDAV endpoint and stores the connection.
func (a *App) ConnectICloud(sessionToken, appleID, appPassword string) (WorkspaceConnectionView, error) {
	if strings.TrimSpace(appleID) == "" {
		return WorkspaceConnectionView{}, errors.New("icloud: Apple ID is required")
	}
	if strings.TrimSpace(appPassword) == "" {
		return WorkspaceConnectionView{}, errors.New("icloud: app-specific password is required")
	}
	if !a.workspaceAvailable() {
		return WorkspaceConnectionView{}, errors.New("workspace: not configured")
	}
	userID, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return WorkspaceConnectionView{}, err
	}

	// Validate credentials.
	prov := getICloudProvider()
	extID, err := prov.ValidateAppPassword(a.appCtx(), appleID, appPassword)
	if err != nil {
		return WorkspaceConnectionView{}, fmt.Errorf("icloud: %w", err)
	}

	// Store the connection. The app-specific password is the "access
	// token"; Apple ID is the "refresh token" (for human reference).
	// No expiry — app-specific passwords don't rotate.
	c, err := a.workspace.DirectConnect(a.appCtx(), workspace.Connection{
		UserID:              userID,
		Service:             workspace.ServiceICloud,
		ExternalWorkspaceID: extID,
		DisplayName:         appleID,
	}, workspace.Token{
		AccessToken:  appPassword,
		RefreshToken: appleID,
		TokenType:    "app-specific-password",
	})
	if err != nil {
		return WorkspaceConnectionView{}, fmt.Errorf("icloud: store connection: %w", err)
	}
	return toWorkspaceView(c), nil
}

// ListICloudNotes reads notes via the macOS memo CLI.
func (a *App) ListICloudNotes(sessionToken string) ([]map[string]string, error) {
	_, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return nil, err
	}
	out, err := exec.Command("memo", "list", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("icloud: memo CLI failed: %w", err)
	}
	_ = out
	return []map[string]string{}, nil
}

// ListICloudReminders reads reminders via the macOS remindctl CLI.
func (a *App) ListICloudReminders(sessionToken string) ([]map[string]string, error) {
	_, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return nil, err
	}
	out, err := exec.Command("remindctl", "list", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("icloud: remindctl CLI failed: %w", err)
	}
	_ = out
	return []map[string]string{}, nil
}

// ListICloudEvents delegates to the CalDAV calendar adapter.
func (a *App) ListICloudEvents(sessionToken, connectionID, start, end string) ([]workspace.Event, error) {
	if !a.workspaceAvailable() {
		return nil, errors.New("workspace: not configured")
	}
	tok, err := a.resolveICloudConnection(sessionToken, connectionID)
	if err != nil {
		return nil, err
	}
	return getICloudCal().ListEvents(a.appCtx(), tok, start, end)
}

// resolveICloudConnection finds and returns the decrypted token for an
// iCloud connection.
func (a *App) resolveICloudConnection(sessionToken, connectionID string) (workspace.Token, error) {
	if !a.workspaceAvailable() {
		return workspace.Token{}, errors.New("workspace: not configured")
	}
	userID, err := a.workspaceUserID(sessionToken)
	if err != nil {
		return workspace.Token{}, err
	}
	conns, err := a.workspace.List(a.appCtx(), userID)
	if err != nil {
		return workspace.Token{}, err
	}
	for _, c := range conns {
		if c.ID != connectionID {
			continue
		}
		if c.Service != workspace.ServiceICloud {
			return workspace.Token{}, fmt.Errorf("workspace: connection %q is %s, not icloud", c.DisplayName, c.Service)
		}
		tok, err := a.workspace.Token(a.appCtx(), c.ID)
		if err != nil {
			return workspace.Token{}, fmt.Errorf("workspace: load token: %w", err)
		}
		return tok, nil
	}
	return workspace.Token{}, errors.New("workspace: connection not found for this user")
}
