package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// MicrosoftProvider drives the OAuth 2.0 flow for Microsoft 365 / Graph.
// PKCE (S256) always on; access_type=offline for refresh token.
//
// Setup guide: https://portal.azure.com → App registrations → New
// registration. Redirect URI: <ArgusBaseURL>/workspace/oauth/callback.
// Under "Certificates & secrets" create a client secret.
// Under "API permissions" add Microsoft Graph → delegated:
//   - Calendars.ReadWrite
//   - Files.ReadWrite
//   - Tasks.ReadWrite
//   - User.Read (for profile)

const (
	msAuthURL     = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
	msTokenURL    = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
	msUserinfoURL = "https://graph.microsoft.com/oidc/userinfo"

	msScopes = "https://graph.microsoft.com/Calendars.ReadWrite " +
		"https://graph.microsoft.com/Files.ReadWrite " +
		"https://graph.microsoft.com/Tasks.ReadWrite " +
		"https://graph.microsoft.com/User.Read " +
		"offline_access"
)

type MicrosoftProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string

	HTTPClient  *http.Client
	AuthURLBase string
	TokenURL    string
	UserinfoURL string

	flightMu sync.Mutex
	flights  map[string]msFlight
}

type msFlight struct {
	verifier  string
	createdAt time.Time
}

const msFlightTTL = 10 * time.Minute

func NewMicrosoftProvider() *MicrosoftProvider {
	return &MicrosoftProvider{}
}

func (p *MicrosoftProvider) Service() Service { return ServiceMicrosoft }

func (p *MicrosoftProvider) client() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: 20 * time.Second}
}

func (p *MicrosoftProvider) authBase() string {
	if p.AuthURLBase != "" {
		return p.AuthURLBase
	}
	return msAuthURL
}

func (p *MicrosoftProvider) tokenURL() string {
	if p.TokenURL != "" {
		return p.TokenURL
	}
	return msTokenURL
}

func (p *MicrosoftProvider) userinfoURL() string {
	if p.UserinfoURL != "" {
		return p.UserinfoURL
	}
	return msUserinfoURL
}

func (p *MicrosoftProvider) Start(_ context.Context, _, _ string) (AuthURL, error) {
	if p.ClientID == "" || p.RedirectURL == "" {
		return AuthURL{}, fmt.Errorf("microsoft: provider not configured (need client ID + redirect URL). Get them at https://portal.azure.com → App registrations")
	}
	state := randomState()
	verifier := randomState()
	challenge := pkceS256(verifier)

	q := url.Values{}
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", p.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", msScopes)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("prompt", "consent")

	p.flightMu.Lock()
	if p.flights == nil {
		p.flights = map[string]msFlight{}
	}
	cutoff := time.Now().Add(-msFlightTTL)
	for s, f := range p.flights {
		if f.createdAt.Before(cutoff) {
			delete(p.flights, s)
		}
	}
	p.flights[state] = msFlight{verifier: verifier, createdAt: time.Now()}
	p.flightMu.Unlock()

	return AuthURL{URL: p.authBase() + "?" + q.Encode(), State: state}, nil
}

func (p *MicrosoftProvider) Complete(ctx context.Context, state, code string) (CompleteResult, error) {
	if code == "" {
		return CompleteResult{}, fmt.Errorf("microsoft: empty code")
	}
	p.flightMu.Lock()
	flight, ok := p.flights[state]
	if ok {
		delete(p.flights, state)
	}
	p.flightMu.Unlock()
	if !ok {
		return CompleteResult{}, fmt.Errorf("microsoft: unknown or expired state — restart connect")
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
		return CompleteResult{}, fmt.Errorf("microsoft: token response had no access_token")
	}

	ui, err := p.fetchUserinfo(ctx, tr.AccessToken)
	if err != nil {
		ui = msUserinfo{}
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
		display = "Microsoft Account"
	}

	return CompleteResult{
		ExternalWorkspaceID: ui.Email,
		DisplayName:         display,
		Email:               ui.Email,
		Token:               tok,
	}, nil
}

func (p *MicrosoftProvider) Refresh(ctx context.Context, refreshToken string) (Token, error) {
	if refreshToken == "" {
		return Token{}, fmt.Errorf("microsoft: empty refresh token")
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
		return Token{}, fmt.Errorf("microsoft: refresh response had no access_token")
	}
	out := Token{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    defaultStr(tr.TokenType, "Bearer"),
		Scope:        tr.Scope,
	}
	if tr.ExpiresIn > 0 {
		out.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return out, nil
}

type msTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

type msUserinfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (p *MicrosoftProvider) postToken(ctx context.Context, form url.Values) (msTokenResponse, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL(),
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client().Do(req)
	if err != nil {
		return msTokenResponse{}, fmt.Errorf("microsoft: token endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	var tr msTokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return msTokenResponse{}, fmt.Errorf("microsoft: parse token response: %w (status=%d body=%s)", err, resp.StatusCode, truncate(string(body), 200))
	}
	if resp.StatusCode >= 400 || tr.Error != "" {
		msg := tr.Error
		if tr.ErrorDesc != "" {
			msg = tr.Error + ": " + tr.ErrorDesc
		}
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return msTokenResponse{}, fmt.Errorf("microsoft: token exchange failed: %s", msg)
	}
	return tr, nil
}

func (p *MicrosoftProvider) fetchUserinfo(ctx context.Context, accessToken string) (msUserinfo, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.userinfoURL(), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client().Do(req)
	if err != nil {
		return msUserinfo{}, fmt.Errorf("microsoft: userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return msUserinfo{}, fmt.Errorf("microsoft: userinfo status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var ui msUserinfo
	if err := json.Unmarshal(body, &ui); err != nil {
		return msUserinfo{}, fmt.Errorf("microsoft: parse userinfo: %w", err)
	}
	return ui, nil
}
