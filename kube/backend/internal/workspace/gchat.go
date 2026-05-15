package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Google Chat adapter — implements Messenger against
// https://chat.googleapis.com/v1/. Two endpoints we care about:
//
//   - spaces.list  → enumerate spaces the user can post to
//   - spaces.messages.create → send a text message into a space
//
// The Service() return value is ServiceGoogle because Chat lives
// behind the same OAuth grant as Docs/Sheets/Tasks. The Slack adapter
// returns ServiceSlack; we don't have a "gchat-only" service.

const (
	gchatAPIBaseURL = "https://chat.googleapis.com/v1"
	// gchatSpacePrefix is the resource-name prefix every Google Chat
	// space ID carries. Extracted once so we trim/prepend consistently
	// instead of sprinkling the literal across the file.
	gchatSpacePrefix = "spaces/"
)

// GChatAdapter satisfies the Messenger interface for Google Chat.
type GChatAdapter struct {
	HTTPClient *http.Client
	APIBaseURL string // tests override
}

func NewGChatAdapter() *GChatAdapter {
	return &GChatAdapter{HTTPClient: &http.Client{Timeout: 15 * time.Second}}
}

func (a *GChatAdapter) Service() Service { return ServiceGoogle }

func (a *GChatAdapter) base() string {
	if a.APIBaseURL != "" {
		return a.APIBaseURL
	}
	return gchatAPIBaseURL
}

// gchatSpacesResponse mirrors the parts of spaces.list we render.
// Spaces have either a `displayName` (named rooms) or are direct
// messages (no display name). We show a synthetic "Direct message"
// label so the UI dropdown isn't full of blanks.
type gchatSpacesResponse struct {
	Spaces []struct {
		Name        string `json:"name"`        // "spaces/AAA…"
		DisplayName string `json:"displayName"` // empty for DMs
		Type        string `json:"type"`        // ROOM | DM | GROUP_DM (legacy)
		SpaceType   string `json:"spaceType"`   // SPACE | DIRECT_MESSAGE | GROUP_CHAT (newer)
	} `json:"spaces"`
	NextPageToken string `json:"nextPageToken"`
}

// ListChannels returns up to 200 spaces. Pagination cap is the same
// rationale as Slack: a dropdown picker shouldn't enumerate 10k spaces.
func (a *GChatAdapter) ListChannels(ctx context.Context, token Token) ([]Channel, error) {
	q := url.Values{}
	q.Set("pageSize", "200")
	endpoint := a.base() + "/spaces?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("gchat: build list request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gchat: spaces.list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gchat: spaces.list %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var sr gchatSpacesResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("gchat: parse spaces.list: %w", err)
	}
	out := make([]Channel, 0, len(sr.Spaces))
	for _, s := range sr.Spaces {
		name := s.DisplayName
		if name == "" {
			// DMs / unnamed group chats — label them so the dropdown
			// doesn't show a blank. The space resource name is
			// "spaces/<opaque>" — surface the suffix as a hint.
			suffix := strings.TrimPrefix(s.Name, gchatSpacePrefix)
			if a.isDirectMessage(s.SpaceType, s.Type) {
				name = "Direct message · " + shortID(suffix)
			} else {
				name = "Group chat · " + shortID(suffix)
			}
		}
		out = append(out, Channel{ID: s.Name, Name: name})
	}
	return out, nil
}

func (a *GChatAdapter) isDirectMessage(spaceType, legacyType string) bool {
	if spaceType != "" {
		return spaceType == "DIRECT_MESSAGE"
	}
	return legacyType == "DM"
}

// gchatMessageBody is the request body for messages.create. We only
// fill `text`; rich-card payloads are a v2 feature.
type gchatMessageBody struct {
	Text string `json:"text"`
}

// gchatMessageResponse is just enough to confirm the POST succeeded.
type gchatMessageResponse struct {
	Name       string `json:"name"`       // "spaces/X/messages/Y"
	CreateTime string `json:"createTime"`
}

// Send posts a plain-text message into the space. Google Chat renders
// limited markdown (asterisks, backticks, line breaks) — same as
// Slack's behaviour, so an alert summary lands looking sensible.
func (a *GChatAdapter) Send(ctx context.Context, token Token, spaceID, text string) error {
	if spaceID == "" {
		return fmt.Errorf("gchat: space is required")
	}
	// The resource name is "spaces/AAA…"; the URL is
	// /v1/{name=spaces/*}/messages. Trim a stray gchatSpacePrefix if the
	// caller supplied the bare ID.
	if !strings.HasPrefix(spaceID, gchatSpacePrefix) {
		spaceID = gchatSpacePrefix + spaceID
	}
	endpoint := a.base() + "/" + spaceID + "/messages"
	body, _ := json.Marshal(gchatMessageBody{Text: text})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gchat: build send request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("gchat: messages.create: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gchat: messages.create %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	// Decode just for shape validation — we don't surface the message
	// resource name yet.
	var mr gchatMessageResponse
	_ = json.Unmarshal(respBody, &mr)
	return nil
}

// shortID returns a friendly prefix of a space resource ID for the
// fallback label. Spaces IDs are opaque 14-22 char strings; first 6
// is enough to disambiguate in a dropdown without being noisy.
func shortID(s string) string {
	if len(s) <= 6 {
		return s
	}
	return s[:6]
}
