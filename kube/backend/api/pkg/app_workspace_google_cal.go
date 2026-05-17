package pkg

import (
	"github.com/argues/argus/internal/workspace"
)

// app_workspace_google_cal.go — per-capability Wails methods for
// Google Calendar, backed by the unified Google connection (one OAuth
// grant, shared token). Mirrors app_workspace_google.go's pattern.

// --- Calendar --------------------------------------------------------------

// ListGoogleCalendarEvents returns events in the given RFC 3339 time
// window from the user's primary Google Calendar.
func (a *App) ListGoogleCalendarEvents(sessionToken, connectionID, start, end string) ([]workspace.Event, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return nil, err
	}
	return getGCalAdapter().ListEvents(a.appCtx(), tok, start, end)
}

// CreateGoogleCalendarEvent creates an event on the user's primary
// Google Calendar. Summary is required.
func (a *App) CreateGoogleCalendarEvent(sessionToken, connectionID string, ev workspace.Event) (workspace.Event, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.Event{}, err
	}
	return getGCalAdapter().CreateEvent(a.appCtx(), tok, ev)
}

// UpdateGoogleCalendarEvent patches an event on the user's primary
// Google Calendar. Only supplied non-zero fields are applied.
func (a *App) UpdateGoogleCalendarEvent(sessionToken, connectionID, eventID string, ev workspace.Event) (workspace.Event, error) {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return workspace.Event{}, err
	}
	return getGCalAdapter().UpdateEvent(a.appCtx(), tok, eventID, ev)
}

// DeleteGoogleCalendarEvent removes an event from the user's primary
// Google Calendar.
func (a *App) DeleteGoogleCalendarEvent(sessionToken, connectionID, eventID string) error {
	tok, err := a.resolveGoogleConnection(sessionToken, connectionID)
	if err != nil {
		return err
	}
	return getGCalAdapter().DeleteEvent(a.appCtx(), tok, eventID)
}
