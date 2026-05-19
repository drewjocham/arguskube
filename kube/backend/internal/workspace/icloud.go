package workspace

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ICloudProvider manages iCloud connections using Apple app-specific
// passwords. iCloud doesn't offer a standard OAuth flow for third-party
// apps, so this provider bypasses the OAuth Start/Complete dance and
// uses DirectConnect instead.
//
// Validation works by making a CalDAV PROPFIND request to
// caldav.icloud.com — if the app-specific password is valid, Apple
// returns 207 Multi-Status; an invalid password gets 401.

const (
	icloudCalDAVHost = "caldav.icloud.com"
	icloudCalDAVURL  = "https://caldav.icloud.com/"
)

type ICloudProvider struct {
	HTTPClient *http.Client
}

func NewICloudProvider() *ICloudProvider {
	return &ICloudProvider{}
}

func (p *ICloudProvider) Service() Service { return ServiceICloud }

func (p *ICloudProvider) Start(_ context.Context, _, _ string) (AuthURL, error) {
	return AuthURL{}, fmt.Errorf("icloud: use DirectConnect, not OAuth")
}

func (p *ICloudProvider) Complete(_ context.Context, _, _ string) (CompleteResult, error) {
	return CompleteResult{}, fmt.Errorf("icloud: use DirectConnect, not OAuth")
}

func (p *ICloudProvider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				// iCloud CalDAV requires TLS 1.2+.
				MinVersion: tls.VersionTLS12,
			},
		},
	}
}

// ValidateAppPassword makes a CalDAV PROPFIND request to verify the
// Apple ID and app-specific password are valid. Returns the principal
// URL on success (used as ExternalWorkspaceID).
func (p *ICloudProvider) ValidateAppPassword(ctx context.Context, appleID, appPassword string) (string, error) {
	if appleID == "" || appPassword == "" {
		return "", fmt.Errorf("icloud: apple ID and app-specific password are required")
	}

	// CalDAV PROPFIND to discover the principal. A minimal request is
	// sufficient — we only need to check if the credentials work.
	body := `<?xml version="1.0" encoding="UTF-8"?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`

	req, err := http.NewRequestWithContext(ctx, "PROPFIND", icloudCalDAVURL,
		strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("icloud: build PROPFIND: %w", err)
	}
	req.SetBasicAuth(appleID, appPassword)
	req.Header.Set("Depth", "0")
	req.Header.Set("Content-Type", "application/xml")

	resp, err := p.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("icloud: CalDAV request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case 207: // Multi-Status — credentials valid
		// Use the appleID as the external workspace identifier.
		return appleID, nil
	case 401, 403:
		return "", fmt.Errorf("icloud: invalid Apple ID or app-specific password (HTTP %d)", resp.StatusCode)
	default:
		return "", fmt.Errorf("icloud: unexpected CalDAV response (HTTP %d)", resp.StatusCode)
	}
}

// ICloudCalendarer is the CalDAV-based calendar adapter for iCloud.
// Implements Calendarer for the workspace types.
type ICloudCalendarer struct {
	HTTPClient *http.Client
}

func NewICloudCalendarer() *ICloudCalendarer { return &ICloudCalendarer{} }

func (a *ICloudCalendarer) Service() Service { return ServiceICloud }

func (a *ICloudCalendarer) getClient() *http.Client {
	if a.HTTPClient != nil {
		return a.HTTPClient
	}
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
}

func (a *ICloudCalendarer) ListEvents(ctx context.Context, token Token, start, end string) ([]Event, error) {
	// CalDAV REPORT for time-range query. The token's AccessToken IS the
	// app-specific password; the "refresh token" is the Apple ID.
	return nil, fmt.Errorf("icloud: ListEvents not yet implemented — CalDAV REPORT pending")
}

func (a *ICloudCalendarer) CreateEvent(ctx context.Context, token Token, ev Event) (Event, error) {
	return Event{}, fmt.Errorf("icloud: CreateEvent not yet implemented")
}

func (a *ICloudCalendarer) UpdateEvent(ctx context.Context, token Token, eventID string, ev Event) (Event, error) {
	return Event{}, fmt.Errorf("icloud: UpdateEvent not yet implemented")
}

func (a *ICloudCalendarer) DeleteEvent(ctx context.Context, token Token, eventID string) error {
	return fmt.Errorf("icloud: DeleteEvent not yet implemented")
}
