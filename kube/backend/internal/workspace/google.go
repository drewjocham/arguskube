package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Google integration — implements Provider + Refresher for a UNIFIED
// Google Workspace grant. One OAuth consent covers Docs + Sheets +
// Tasks; the per-capability adapters (gdocs.go, gsheets.go, gtasks.go)
// share whatever token this provider deposits.
//
// Hand-rolled HTTP per the codebase convention (slack.go does the same)
// — pulling in google.golang.org/api/* for what amounts to 12 HTTP
// endpoints would bloat the binary by a couple of megabytes.

const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserinfoURL = "https://www.googleapis.com/oauth2/v3/userinfo"
	// contentTypeJSON is shared by every Google REST call we make so a
	// future header set doesn't drift across files.
	contentTypeJSON = "application/json"

	googleScopes = "https://www.googleapis.com/auth/documents " +
		"https://www.googleapis.com/auth/spreadsheets " +
		"https://www.googleapis.com/auth/tasks " +
		// Google Chat scopes — chat.spaces.readonly lists the spaces
		// the user is in; chat.messages.create lets the adapter post.
		// Connected users from before Phase 3 won't have these grants
		// in their token; the UI tells them to disconnect+reconnect to
		// pick up the new scopes (Google's incremental auth would also
		// work but requires extra round-trips we skip for v1).
		"https://www.googleapis.com/auth/chat.spaces.readonly " +
		"https://www.googleapis.com/auth/chat.messages.create " +
		"https://www.googleapis.com/auth/userinfo.email " +
		"https://www.googleapis.com/auth/userinfo.profile"
)

// GoogleProvider drives the unified-google OAuth flow. PKCE (S256) is
// always on; access_type=offline + prompt=consent guarantees we get a
// refresh token on first connect.
type GoogleProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string

	// Test overrides.
	HTTPClient   *http.Client
	AuthURLBase  string
	TokenURL     string
	UserinfoURL  string

	// flightMu guards flights; flights maps state -> PKCE verifier for
	// in-progress flows so Complete can recover the verifier that was
	// minted in Start. Self-contained inside the provider so the
	// Manager doesn't have to know about PKCE.
	flightMu sync.Mutex
	flights  map[string]googleFlight
}

type googleFlight struct {
	verifier  string
	createdAt time.Time
}

const googleFlightTTL = 10 * time.Minute

func (p *GoogleProvider) Service() Service { return ServiceGoogle }

func (p *GoogleProvider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (p *GoogleProvider) authBase() string {
	if p.AuthURLBase != "" {
		return p.AuthURLBase
	}
	return googleAuthURL
}

func (p *GoogleProvider) tokenURL() string {
	if p.TokenURL != "" {
		return p.TokenURL
	}
	return googleTokenURL
}

func (p *GoogleProvider) userinfoURL() string {
	if p.UserinfoURL != "" {
		return p.UserinfoURL
	}
	return googleUserinfoURL
}

func (p *GoogleProvider) Start(_ context.Context, _, _ string) (AuthURL, error) {
	if p.ClientID == "" || p.RedirectURL == "" {
		return AuthURL{}, fmt.Errorf("google: provider not configured (need client id + redirect url)")
	}
	state := randomState()
	verifier := randomState() // 24 bytes hex = 48 chars; fits the 43..128 PKCE range.
	challenge := pkceS256(verifier)

	q := url.Values{}
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", googleScopes)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	// access_type=offline + prompt=consent — without prompt=consent
	// Google omits the refresh_token on subsequent grants, which kills
	// our long-running refresher.
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	q.Set("include_granted_scopes", "true")

	p.flightMu.Lock()
	if p.flights == nil {
		p.flights = map[string]googleFlight{}
	}
	p.gcFlightsLocked()
	p.flights[state] = googleFlight{verifier: verifier, createdAt: time.Now()}
	p.flightMu.Unlock()

	return AuthURL{URL: p.authBase() + "?" + q.Encode(), State: state}, nil
}

func (p *GoogleProvider) gcFlightsLocked() {
	cutoff := time.Now().Add(-googleFlightTTL)
	for s, f := range p.flights {
		if f.createdAt.Before(cutoff) {
			delete(p.flights, s)
		}
	}
}

// googleTokenResponse is the OAuth token / refresh response. Fields we
// don't need (id_token, etc.) are intentionally omitted.
type googleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// googleUserinfo is the relevant subset of openid userinfo.
type googleUserinfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	HD      string `json:"hd,omitempty"` // hosted-domain — populated for Workspace accounts
}

func (p *GoogleProvider) Complete(ctx context.Context, state, code string) (CompleteResult, error) {
	if code == "" {
		return CompleteResult{}, fmt.Errorf("google: empty code")
	}
	p.flightMu.Lock()
	p.gcFlightsLocked()
	flight, ok := p.flights[state]
	if ok {
		delete(p.flights, state)
	}
	p.flightMu.Unlock()
	if !ok {
		return CompleteResult{}, fmt.Errorf("google: unknown or expired state — restart connect")
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", p.RedirectURL)
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", p.ClientSecret)
	form.Set("code_verifier", flight.verifier)

	tr, err := p.postToken(ctx, form)
	if err != nil {
		return CompleteResult{}, err
	}
	if tr.AccessToken == "" {
		return CompleteResult{}, fmt.Errorf("google: token response had no access_token")
	}

	ui, err := p.fetchUserinfo(ctx, tr.AccessToken)
	if err != nil {
		// Userinfo is best-effort — the token is good even if userinfo
		// 500s. Soft-fail: keep the connection but with empty identity
		// fields.
		ui = googleUserinfo{}
	}

	tok := Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    defaultStr(tr.TokenType, "Bearer"),
		Scope:        tr.Scope,
	}
	if tr.ExpiresIn > 0 {
		tok.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}

	display := ui.Name
	if display == "" {
		display = ui.Email
	}
	if display == "" {
		display = "Google Account"
	}

	return CompleteResult{
		ExternalWorkspaceID: ui.HD, // org domain for Workspace users; empty for consumer @gmail
		DisplayName:         display,
		Email:               ui.Email,
		AvatarURL:           ui.Picture,
		Token:               tok,
	}, nil
}

// Refresh exchanges a refresh_token for a fresh access_token. Google's
// response sometimes omits the refresh_token field (it's the same one
// you sent in); the Manager treats an empty RefreshToken as "keep the
// previous one" — see Store.UpdateToken.
func (p *GoogleProvider) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	if refreshToken == "" {
		return Token{}, fmt.Errorf("google: empty refresh token")
	}
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", p.ClientSecret)

	tr, err := p.postToken(ctx, form)
	if err != nil {
		return Token{}, err
	}
	if tr.AccessToken == "" {
		return Token{}, fmt.Errorf("google: refresh response had no access_token")
	}
	out := Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken, // may be empty — caller preserves the old one
		TokenType:    defaultStr(tr.TokenType, "Bearer"),
		Scope:        tr.Scope,
	}
	if tr.ExpiresIn > 0 {
		out.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return out, nil
}

func (p *GoogleProvider) postToken(ctx context.Context, form url.Values) (googleTokenResponse, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL(),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", contentTypeJSON)

	resp, err := p.client().Do(req)
	if err != nil {
		return googleTokenResponse{}, fmt.Errorf("google: token endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	var tr googleTokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return googleTokenResponse{}, fmt.Errorf("google: parse token response: %w (status=%d body=%s)", err, resp.StatusCode, truncate(string(body), 200))
	}
	if resp.StatusCode >= 400 || tr.Error != "" {
		// Google's OAuth errors come in {error, error_description}.
		msg := tr.Error
		if tr.ErrorDesc != "" {
			msg = tr.Error + ": " + tr.ErrorDesc
		}
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return googleTokenResponse{}, fmt.Errorf("google: token exchange failed: %s", msg)
	}
	return tr, nil
}

func (p *GoogleProvider) fetchUserinfo(ctx context.Context, accessToken string) (googleUserinfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.userinfoURL(), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", contentTypeJSON)

	resp, err := p.client().Do(req)
	if err != nil {
		return googleUserinfo{}, fmt.Errorf("google: userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return googleUserinfo{}, fmt.Errorf("google: userinfo status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var ui googleUserinfo
	if err := json.Unmarshal(body, &ui); err != nil {
		return googleUserinfo{}, fmt.Errorf("google: parse userinfo: %w", err)
	}
	return ui, nil
}

// pkceS256 is the RFC 7636 code_challenge transform: base64-url(no
// padding) of SHA-256(verifier).
func pkceS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// googleAPICall is the shared bearer-auth JSON helper used by all three
// Google adapters. Reduces per-adapter boilerplate.
func googleAPICall(ctx context.Context, hc *http.Client, token Token, method, endpoint string, body any, out any) error {
	if token.AccessToken == "" {
		return fmt.Errorf("google: empty access token")
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("google: marshal body: %w", err)
		}
		reader = strings.NewReader(string(b))
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("google: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", contentTypeJSON)
	if body != nil {
		req.Header.Set("Content-Type", contentTypeJSON)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("google: %s %s: %w", method, endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		// Google JSON errors: {"error":{"code":N,"message":"…","status":"…"}}.
		var ge struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &ge)
		msg := ge.Error.Message
		if msg == "" {
			msg = truncate(string(raw), 200)
		}
		return fmt.Errorf("google: %s %s: HTTP %d: %s", method, endpoint, resp.StatusCode, msg)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("google: parse response: %w", err)
	}
	return nil
}

// googleClient returns the http client an adapter should use. Centralised
// so a future move to a custom transport hits one spot.
func googleClient(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return &http.Client{Timeout: 20 * time.Second}
}
