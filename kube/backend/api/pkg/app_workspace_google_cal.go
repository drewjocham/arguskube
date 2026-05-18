package pkg

import (
	"github.com/argues/argus/internal/workspace"
)

// app_workspace_google_cal.go — per-capability Wails methods for
// Google Calendar, backed by the unified Google connection (one OAuth
// grant, shared token). Mirrors app_workspace_google.go's pattern.

func (a *App) ListGoogleCalendarEvents(sessionToken, connectionID, start, end string) ([]workspace.Event, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return nil, err
	}
	return getGCalAdapter().ListEvents(a.appCtx(), tok, start, end)
}

func (a *App) CreateGoogleCalendarEvent(sessionToken, connectionID string, ev workspace.Event) (workspace.Event, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.Event{}, err
	}
	return getGCalAdapter().CreateEvent(a.appCtx(), tok, ev)
}

func (a *App) UpdateGoogleCalendarEvent(sessionToken, connectionID, eventID string, ev workspace.Event) (workspace.Event, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.Event{}, err
	}
	return getGCalAdapter().UpdateEvent(a.appCtx(), tok, eventID, ev)
}

func (a *App) DeleteGoogleCalendarEvent(sessionToken, connectionID, eventID string) error {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return err
	}
	return getGCalAdapter().DeleteEvent(a.appCtx(), tok, eventID)
}
