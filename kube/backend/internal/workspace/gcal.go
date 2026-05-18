package workspace

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Google Calendar adapter — list / create / update / delete against
// the Calendar v3 API. Hand-rolled HTTP like every other Google adapter;
// pulling in google.golang.org/api/calendar for 4 endpoints is overkill.
//
// We target the user's primary calendar by default. Multi-calendar
// support can be added later by exposing the calendarList endpoint.

const (
	gcalAPIBase = "https://www.googleapis.com/calendar/v3"
	// gcalCalPath is the per-calendar prefix. Hard-wired to "primary"
	// for v1; a future param can swap it.
	gcalCalPath = "/calendars/primary/events"
)

// GCalEvent is the richer Calendar v3 event shape, with fields beyond
// the common Event struct (conference data, attendees, recurrence).
type GCalEvent struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
	Start       *struct {
		DateTime string `json:"dateTime,omitempty"`
		Date     string `json:"date,omitempty"`
	} `json:"start"`
	End *struct {
		DateTime string `json:"dateTime,omitempty"`
		Date     string `json:"date,omitempty"`
	} `json:"end"`
	HTMLLink string `json:"htmlLink,omitempty"`
}

// gcalListResponse is the v3 events.list envelope.
type gcalListResponse struct {
	Items []GCalEvent `json:"items"`
}

type GCalAdapter struct {
	HTTPClient *http.Client
	APIBase    string // tests override
}

func NewGCalAdapter() *GCalAdapter { return &GCalAdapter{} }

func (a *GCalAdapter) Service() Service { return ServiceGCal }

func (a *GCalAdapter) base() string {
	if a.APIBase != "" {
		return a.APIBase
	}
	return gcalAPIBase
}

// ListEvents returns all events in the given time window from the
// user's primary calendar. start and end are RFC 3339 strings.
func (a *GCalAdapter) ListEvents(ctx context.Context, token Token, start, end string) ([]Event, error) {
	hc := googleClient(a.HTTPClient)
	u, err := url.Parse(a.base() + gcalCalPath)
	if err != nil {
		return nil, fmt.Errorf("gcal: build url: %w", err)
	}
	q := u.Query()
	if start != "" {
		q.Set("timeMin", start)
	}
	if end != "" {
		q.Set("timeMax", end)
	}
	q.Set("singleEvents", "true")
	q.Set("orderBy", "startTime")
	q.Set("maxResults", "250")
	u.RawQuery = q.Encode()

	var page gcalListResponse
	if err := googleAPICall(ctx, hc, token, http.MethodGet, u.String(), nil, &page); err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(page.Items))
	for _, it := range page.Items {
		out = append(out, toEvent(it))
	}
	return out, nil
}

// CreateEvent creates an event on the user's primary calendar.
func (a *GCalAdapter) CreateEvent(ctx context.Context, token Token, ev Event) (Event, error) {
	if strings.TrimSpace(ev.Summary) == "" {
		return Event{}, fmt.Errorf("gcal: summary is required")
	}
	hc := googleClient(a.HTTPClient)
	body := fromEvent(ev)
	var created GCalEvent
	if err := googleAPICall(ctx, hc, token, http.MethodPost,
		a.base()+gcalCalPath, body, &created); err != nil {
		return Event{}, err
	}
	return toEvent(created), nil
}

// UpdateEvent patches an event. Only the supplied non-zero fields are
// applied; Google's PATCH semantics handle the rest.
func (a *GCalAdapter) UpdateEvent(ctx context.Context, token Token, eventID string, ev Event) (Event, error) {
	if strings.TrimSpace(eventID) == "" {
		return Event{}, fmt.Errorf("gcal: eventID required")
	}
	hc := googleClient(a.HTTPClient)
	body := fromEvent(ev)
	var updated GCalEvent
	if err := googleAPICall(ctx, hc, token, http.MethodPatch,
		a.base()+gcalCalPath+"/"+url.PathEscape(eventID), body, &updated); err != nil {
		return Event{}, err
	}
	return toEvent(updated), nil
}

// DeleteEvent removes an event. Returns nil on success (HTTP 204).
func (a *GCalAdapter) DeleteEvent(ctx context.Context, token Token, eventID string) error {
	if strings.TrimSpace(eventID) == "" {
		return fmt.Errorf("gcal: eventID required")
	}
	hc := googleClient(a.HTTPClient)
	// googleAPICall tries to JSON-unmarshal empty 204 responses and
	// fails silently; we build a manual request for DELETE.
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		a.base()+gcalCalPath+"/"+url.PathEscape(eventID), nil)
	if err != nil {
		return fmt.Errorf("gcal: build delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", contentTypeJSON)
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("gcal: delete event: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("gcal: delete event: HTTP %d", resp.StatusCode)
	}
	return nil
}

// toEvent normalises a GCalEvent into the common Event shape.
func toEvent(ge GCalEvent) Event {
	ev := Event{
		ID:          ge.ID,
		Summary:     ge.Summary,
		Description: ge.Description,
		Location:    ge.Location,
		HTMLink:     ge.HTMLLink,
	}
	if ge.Start != nil {
		ev.Start = ge.Start.DateTime
		if ev.Start == "" {
			ev.Start = ge.Start.Date
		}
	}
	if ge.End != nil {
		ev.End = ge.End.DateTime
		if ev.End == "" {
			ev.End = ge.End.Date
		}
	}
	return ev
}

// fromEvent converts our common Event into the shape Google Calendar v3
// expects in create/update request bodies.
func fromEvent(ev Event) map[string]any {
	m := map[string]any{}
	if ev.Summary != "" {
		m["summary"] = ev.Summary
	}
	if ev.Description != "" {
		m["description"] = ev.Description
	}
	if ev.Location != "" {
		m["location"] = ev.Location
	}
	if ev.Start != "" {
		m["start"] = map[string]string{"dateTime": ev.Start}
	}
	if ev.End != "" {
		m["end"] = map[string]string{"dateTime": ev.End}
	}
	return m
}
