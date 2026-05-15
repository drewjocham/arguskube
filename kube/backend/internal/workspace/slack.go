package workspace

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Slack integration — implements Provider (OAuth flow) and Messenger
// (channels + send) against the Slack Web API. Hand-rolled HTTP to
// avoid pulling in the 8k-loc slack-go module for two endpoints; the
// surface is small and the JSON shapes don't churn.
//
// OAuth flow uses Slack's v2 endpoints. The bot token returned has the
// shape `xoxb-…` and does NOT expire by default — Token.ExpiresAt
// stays zero so the (future) refresh worker skips it.

const (
	slackAuthURL     = "https://slack.com/oauth/v2/authorize"
	slackTokenURL    = "https://slack.com/api/oauth.v2.access"
	slackAPIBaseURL  = "https://slack.com/api"
	slackDefaultScopes = "chat:write,channels:read,groups:read,team:read,users:read"
)

// SlackProvider holds the OAuth client credentials and the redirect
// URL the public callback handler listens on.
type SlackProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       string // comma-separated; default if empty
	// HTTPClient is var-injected so tests can point at an httptest.Server.
	HTTPClient *http.Client
	// APIBaseURL overrides the Slack API base; tests set it to an
	// httptest.Server. Empty = slackAPIBaseURL.
	APIBaseURL string
	// AuthURLBase overrides the authorize endpoint for tests.
	AuthURLBase string
	// TokenURL overrides the token-exchange endpoint for tests.
	TokenURL string
}

func (p *SlackProvider) Service() Service { return ServiceSlack }

func (p *SlackProvider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (p *SlackProvider) Start(_ context.Context, _, _ string) (AuthURL, error) {
	if p.ClientID == "" || p.RedirectURL == "" {
		return AuthURL{}, fmt.Errorf("slack: provider not configured (need client id + redirect url)")
	}
	state := randomState()
	scopes := p.Scopes
	if scopes == "" {
		scopes = slackDefaultScopes
	}
	authBase := p.AuthURLBase
	if authBase == "" {
		authBase = slackAuthURL
	}
	q := url.Values{}
	q.Set("client_id", p.ClientID)
	q.Set("scope", scopes)
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("state", state)
	return AuthURL{URL: authBase + "?" + q.Encode(), State: state}, nil
}

// slackOAuthResponse is the subset of oauth.v2.access we care about.
// Slack returns top-level ok/error plus a nested team and authed_user.
type slackOAuthResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error,omitempty"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	BotUserID   string `json:"bot_user_id"`
	Team        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
	AuthedUser struct {
		ID    string `json:"id"`
		Scope string `json:"scope"`
	} `json:"authed_user"`
}

func (p *SlackProvider) Complete(ctx context.Context, _, code string) (CompleteResult, error) {
	if code == "" {
		return CompleteResult{}, fmt.Errorf("slack: empty code")
	}
	tokenURL := p.TokenURL
	if tokenURL == "" {
		tokenURL = slackTokenURL
	}
	form := url.Values{}
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", p.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", p.RedirectURL)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client().Do(req)
	if err != nil {
		return CompleteResult{}, fmt.Errorf("slack: token exchange: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	var or slackOAuthResponse
	if err := json.Unmarshal(body, &or); err != nil {
		return CompleteResult{}, fmt.Errorf("slack: parse token response: %w", err)
	}
	if !or.OK {
		// Slack-specific error strings are useful enough to surface
		// verbatim (e.g. "invalid_code", "bad_redirect_uri").
		return CompleteResult{}, fmt.Errorf("slack: token exchange failed: %s", or.Error)
	}
	if or.AccessToken == "" {
		return CompleteResult{}, fmt.Errorf("slack: token response had no access_token")
	}

	tokType := or.TokenType
	if tokType == "" {
		tokType = "bearer"
	}
	return CompleteResult{
		ExternalWorkspaceID: or.Team.ID,
		DisplayName:         or.Team.Name,
		Email:               "", // Slack OAuth v2 doesn't return an email for the bot install.
		Token: Token{
			AccessToken: or.AccessToken,
			TokenType:   tokType,
			Scope:       or.Scope,
			// Bot tokens don't expire; leaving ExpiresAt zero tells
			// the future refresh worker to skip this row.
		},
	}, nil
}

// SlackAdapter is the Messenger-shaped wrapper used by Wails methods
// post-OAuth. It calls Slack's Web API with the connection's stored
// bearer token. One adapter instance per process is fine — the only
// state is the (overridable) HTTP base URL.
type SlackAdapter struct {
	HTTPClient *http.Client
	APIBaseURL string // tests override
}

func NewSlackAdapter() *SlackAdapter {
	return &SlackAdapter{HTTPClient: &http.Client{Timeout: 15 * time.Second}}
}

func (a *SlackAdapter) Service() Service { return ServiceSlack }

func (a *SlackAdapter) base() string {
	if a.APIBaseURL != "" {
		return a.APIBaseURL
	}
	return slackAPIBaseURL
}

func (a *SlackAdapter) doForm(ctx context.Context, token Token, method, path string, form url.Values, out any) error {
	if token.AccessToken == "" {
		return fmt.Errorf("slack: empty access token")
	}
	endpoint := a.base() + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	resp, err := a.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("slack: call %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("slack: parse %s: %w", path, err)
	}
	return nil
}

// slackChannelsResponse mirrors conversations.list — only the fields
// the UI actually renders.
type slackChannelsResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
	Channels []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		IsChannel bool   `json:"is_channel"`
		IsPrivate bool   `json:"is_private"`
		IsArchived bool  `json:"is_archived"`
	} `json:"channels"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

// ListChannels returns up to 200 non-archived channels the bot can
// post to. Pagination cap is intentional — message-broker-style
// load-test usage doesn't need a thousand-channel picker, and the UI
// is a dropdown.
func (a *SlackAdapter) ListChannels(ctx context.Context, token Token) ([]Channel, error) {
	form := url.Values{}
	form.Set("types", "public_channel,private_channel")
	form.Set("exclude_archived", "true")
	form.Set("limit", "200")

	var resp slackChannelsResponse
	if err := a.doForm(ctx, token, http.MethodGet, "/conversations.list?"+form.Encode(), nil, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("slack: conversations.list: %s", resp.Error)
	}
	out := make([]Channel, 0, len(resp.Channels))
	for _, c := range resp.Channels {
		if c.IsArchived {
			continue
		}
		out = append(out, Channel{ID: c.ID, Name: c.Name})
	}
	return out, nil
}

// slackPostResponse is what chat.postMessage returns. ts is the
// timestamp we'd track for threading later; for v1 we only check ok.
type slackPostResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Channel string `json:"channel,omitempty"`
	TS      string `json:"ts,omitempty"`
}

// Send posts a plain-text message. Markdown is enabled by default
// (Slack's mrkdwn) so backticks/asterisks render — useful when Argus
// sends alert summaries.
func (a *SlackAdapter) Send(ctx context.Context, token Token, channelID, text string) error {
	form := url.Values{}
	form.Set("channel", channelID)
	form.Set("text", text)

	var resp slackPostResponse
	if err := a.doForm(ctx, token, http.MethodPost, "/chat.postMessage", form, &resp); err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("slack: chat.postMessage: %s", resp.Error)
	}
	return nil
}

// randomState mints an opaque 24-byte hex nonce. We don't use
// crypto/rand directly in the AuthURL — the helper centralises the
// "error means panic" decision.
func randomState() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
