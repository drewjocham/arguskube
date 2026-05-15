package pkg

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/argues/argus/internal/workspace"
)

// app_workspace_gchat.go — Wails methods for the Google Chat adapter.
// Chat lives behind the same OAuth grant as Docs/Sheets/Tasks (the
// scopes were added to GoogleProvider in Phase 3), so the connection
// shape is identical to those panels: service == ServiceGoogle.

// GChatSpaceView is the redacted shape the UI consumes. Identical to
// workspace.Channel but explicitly named so the front-end's typing
// stays unambiguous between Slack channels and Chat spaces.
type GChatSpaceView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var (
	gchatAdapterOnce sync.Once
	gchatAdapter     *workspace.GChatAdapter
)

func getGChatAdapter() *workspace.GChatAdapter {
	gchatAdapterOnce.Do(func() {
		gchatAdapter = workspace.NewGChatAdapter()
	})
	return gchatAdapter
}

// resolveGChatConnection validates the connection exists and belongs
// to the calling user. Returns a usable token (refreshed if needed by
// Manager.Token under the hood) plus the connection's service so the
// caller can assert it's Google.
func (a *App) resolveGChatConnection(sessionToken, connectionID string) (workspace.Token, error) {
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
		if c.ID == connectionID {
			if c.Service != workspace.ServiceGoogle {
				return workspace.Token{}, fmt.Errorf("workspace: connection %q is %s, not google", c.DisplayName, c.Service)
			}
			tok, err := a.workspace.Token(a.appCtx(), c.ID)
			if err != nil {
				return workspace.Token{}, fmt.Errorf("workspace: load token: %w", err)
			}
			return tok, nil
		}
	}
	return workspace.Token{}, errors.New("workspace: connection not found for this user")
}

// ListGoogleChatSpaces returns up to 200 spaces the user can post in.
// Used by the GChatPanel's space picker.
func (a *App) ListGoogleChatSpaces(sessionToken, connectionID string) ([]GChatSpaceView, error) {
	tok, err := a.resolveGChatConnection(sessionToken, connectionID)
	if err != nil {
		return nil, err
	}
	spaces, err := getGChatAdapter().ListChannels(a.appCtx(), tok)
	if err != nil {
		return nil, err
	}
	out := make([]GChatSpaceView, 0, len(spaces))
	for _, s := range spaces {
		out = append(out, GChatSpaceView{ID: s.ID, Name: s.Name})
	}
	return out, nil
}

// SendGoogleChatMessage posts plain text to a space. 4 KB sane cap —
// Google Chat's hard cap is 32 KB but UI rendering degrades long
// before that, and an alert summary shouldn't blow past 4k.
func (a *App) SendGoogleChatMessage(sessionToken, connectionID, spaceID, text string) error {
	if strings.TrimSpace(spaceID) == "" {
		return errors.New("gchat: space is required")
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("gchat: text is required")
	}
	tok, err := a.resolveGChatConnection(sessionToken, connectionID)
	if err != nil {
		return err
	}
	if len(text) > 4096 {
		text = text[:4096-12] + "\n\n…truncated"
	}
	return getGChatAdapter().Send(a.appCtx(), tok, spaceID, text)
}
