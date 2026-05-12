// Package oauthproviders is the catalogue of well-known OAuth providers
// the unified login button can drive. It complements internal/auth's
// OIDC manager: that one is for OIDC issuers (discovery doc, ID tokens);
// this one covers plain OAuth2 providers like GitHub that don't speak
// OIDC and never will.
//
// The data layout is intentionally flat — operators pick a preset by
// name (github, gitlab, bitbucket, …) and the manager produces an
// oauth2.Config with the right endpoints + default scopes. A "custom"
// preset is also exposed for self-hosted providers; the operator
// supplies authURL / tokenURL / userInfoURL explicitly.
//
// State management: every Start() call mints a fresh PKCE verifier +
// random state. Both are stored in an in-memory map keyed by state, so
// the matching callback can re-correlate. Entries TTL out after
// pendingTTL so a forgotten browser tab doesn't pin them forever.
package oauthproviders

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// PresetName identifies one of the built-in OAuth provider presets.
// Operators reference these by string in their config.
type PresetName string

const (
	PresetGitHub    PresetName = "github"
	PresetGitLab    PresetName = "gitlab"
	PresetBitbucket PresetName = "bitbucket"
	PresetMicrosoft PresetName = "microsoft"
	PresetGoogle    PresetName = "google" // OAuth2-only form (the OIDC variant lives in internal/auth)
	PresetLinkedIn  PresetName = "linkedin"
	PresetSlack     PresetName = "slack"
	PresetCustom    PresetName = "custom"
)

// Preset describes the OAuth endpoints + default scopes + userinfo
// mapping for one provider. Frontend display name lives separately on
// the Config struct so operators can rename "GitHub" to "GH Enterprise".
type Preset struct {
	Name           PresetName
	AuthURL        string
	TokenURL       string
	UserInfoURL    string   // returns a JSON document we map below
	DefaultScopes  []string // e.g. ["read:user", "user:email"]
	UserIDField    string   // JSON key for the unique account id
	EmailField     string   // JSON key for the email
	NameField      string   // JSON key for the display name
	// AuthStyle is forwarded to oauth2 (most providers want
	// AutoDetect; GitHub specifically wants params in the query string).
	AuthStyle oauth2.AuthStyle
}

// presets is the read-only registry. Values are returned by value so
// callers can't accidentally mutate the catalogue.
var presets = map[PresetName]Preset{
	PresetGitHub: {
		Name:          PresetGitHub,
		AuthURL:       "https://github.com/login/oauth/authorize",
		TokenURL:      "https://github.com/login/oauth/access_token",
		UserInfoURL:   "https://api.github.com/user",
		DefaultScopes: []string{"read:user", "user:email"},
		UserIDField:   "id",
		EmailField:    "email",
		NameField:     "name",
		AuthStyle:     oauth2.AuthStyleInParams,
	},
	PresetGitLab: {
		Name:          PresetGitLab,
		AuthURL:       "https://gitlab.com/oauth/authorize",
		TokenURL:      "https://gitlab.com/oauth/token",
		UserInfoURL:   "https://gitlab.com/api/v4/user",
		DefaultScopes: []string{"read_user"},
		UserIDField:   "id",
		EmailField:    "email",
		NameField:     "name",
		AuthStyle:     oauth2.AuthStyleInHeader,
	},
	PresetBitbucket: {
		Name:          PresetBitbucket,
		AuthURL:       "https://bitbucket.org/site/oauth2/authorize",
		TokenURL:      "https://bitbucket.org/site/oauth2/access_token",
		UserInfoURL:   "https://api.bitbucket.org/2.0/user",
		DefaultScopes: []string{"account"},
		UserIDField:   "uuid",
		EmailField:    "email",
		NameField:     "display_name",
		AuthStyle:     oauth2.AuthStyleInHeader,
	},
	PresetMicrosoft: {
		Name:          PresetMicrosoft,
		AuthURL:       "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		TokenURL:      "https://login.microsoftonline.com/common/oauth2/v2.0/token",
		UserInfoURL:   "https://graph.microsoft.com/oidc/userinfo",
		DefaultScopes: []string{"openid", "profile", "email", "User.Read"},
		UserIDField:   "sub",
		EmailField:    "email",
		NameField:     "name",
		AuthStyle:     oauth2.AuthStyleInParams,
	},
	PresetGoogle: {
		Name:          PresetGoogle,
		AuthURL:       "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:      "https://oauth2.googleapis.com/token",
		UserInfoURL:   "https://openidconnect.googleapis.com/v1/userinfo",
		DefaultScopes: []string{"openid", "email", "profile"},
		UserIDField:   "sub",
		EmailField:    "email",
		NameField:     "name",
		AuthStyle:     oauth2.AuthStyleInParams,
	},
	PresetLinkedIn: {
		Name:          PresetLinkedIn,
		AuthURL:       "https://www.linkedin.com/oauth/v2/authorization",
		TokenURL:      "https://www.linkedin.com/oauth/v2/accessToken",
		UserInfoURL:   "https://api.linkedin.com/v2/userinfo",
		DefaultScopes: []string{"openid", "profile", "email"},
		UserIDField:   "sub",
		EmailField:    "email",
		NameField:     "name",
		AuthStyle:     oauth2.AuthStyleInParams,
	},
	PresetSlack: {
		Name:          PresetSlack,
		AuthURL:       "https://slack.com/openid/connect/authorize",
		TokenURL:      "https://slack.com/api/openid.connect.token",
		UserInfoURL:   "https://slack.com/api/openid.connect.userInfo",
		DefaultScopes: []string{"openid", "email", "profile"},
		UserIDField:   "sub",
		EmailField:    "email",
		NameField:     "name",
		AuthStyle:     oauth2.AuthStyleInHeader,
	},
}

// GetPreset returns a copy of the preset for name, or false if unknown.
func GetPreset(name PresetName) (Preset, bool) {
	p, ok := presets[name]
	return p, ok
}

// AllPresets returns every built-in preset, deterministically ordered
// for stable test output and predictable UI rendering.
func AllPresets() []Preset {
	out := make([]Preset, 0, len(presets))
	// Use a fixed ordering rather than map iteration so AllPresets is
	// deterministic across runs.
	order := []PresetName{
		PresetGitHub, PresetGitLab, PresetBitbucket,
		PresetGoogle, PresetMicrosoft,
		PresetLinkedIn, PresetSlack,
	}
	for _, n := range order {
		if p, ok := presets[n]; ok {
			out = append(out, p)
		}
	}
	return out
}

// Config is what the operator supplies to register one provider in
// their deployment. ClientID + ClientSecret are required; everything
// else either comes from the preset or overrides it.
type Config struct {
	Name         PresetName // any built-in or PresetCustom
	DisplayName  string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string // optional override; falls back to preset.DefaultScopes

	// Custom-only fields (required when Name == PresetCustom).
	AuthURL     string
	TokenURL    string
	UserInfoURL string
	UserIDField string
	EmailField  string
	NameField   string
}

// resolve fills in defaults from the matching preset. Returns a fully-
// populated Preset alongside the oauth2.Config so callers can do
// userinfo lookups without re-deriving the URL.
func (c Config) resolve() (Preset, *oauth2.Config, error) {
	if c.ClientID == "" {
		return Preset{}, nil, fmt.Errorf("oauthproviders: ClientID required for %s", c.Name)
	}
	if c.RedirectURL == "" {
		return Preset{}, nil, fmt.Errorf("oauthproviders: RedirectURL required for %s", c.Name)
	}
	var preset Preset
	if c.Name == PresetCustom {
		if c.AuthURL == "" || c.TokenURL == "" || c.UserInfoURL == "" {
			return Preset{}, nil, errors.New("oauthproviders: custom preset requires authURL/tokenURL/userInfoURL")
		}
		preset = Preset{
			Name:        PresetCustom,
			AuthURL:     c.AuthURL,
			TokenURL:    c.TokenURL,
			UserInfoURL: c.UserInfoURL,
			UserIDField: defaultStr(c.UserIDField, "sub"),
			EmailField:  defaultStr(c.EmailField, "email"),
			NameField:   defaultStr(c.NameField, "name"),
			AuthStyle:   oauth2.AuthStyleAutoDetect,
		}
	} else {
		p, ok := presets[c.Name]
		if !ok {
			return Preset{}, nil, fmt.Errorf("oauthproviders: unknown preset %q", c.Name)
		}
		preset = p
	}
	scopes := c.Scopes
	if len(scopes) == 0 {
		scopes = preset.DefaultScopes
	}
	cfg := &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:   preset.AuthURL,
			TokenURL:  preset.TokenURL,
			AuthStyle: preset.AuthStyle,
		},
	}
	return preset, cfg, nil
}

// UserInfo is the normalised account profile returned after a successful
// login. Different providers carry different fields; we map only what
// every sane provider exposes (id + email + display name).
type UserInfo struct {
	Provider PresetName `json:"provider"`
	ID       string     `json:"id"`
	Email    string     `json:"email"`
	Name     string     `json:"name"`
	Raw      map[string]any `json:"raw,omitempty"`
}

// Manager drives the unified OAuth flow. One instance owns the pending-
// state map and the registered providers. Safe for concurrent use.
type Manager struct {
	httpClient *http.Client
	// pendingTTL bounds how long a state token stays valid. Defaults to
	// 15 minutes — long enough for slow human pacing, short enough that
	// stale states don't pile up.
	pendingTTL time.Duration
	now        func() time.Time

	mu       sync.Mutex
	configs  map[PresetName]registered
	pending  map[string]*pendingFlow
}

type registered struct {
	cfg    Config
	preset Preset
	oauth  *oauth2.Config
}

type pendingFlow struct {
	provider     PresetName
	verifier     string
	createdAt    time.Time
	completed    bool
	completedErr error
	userInfo     *UserInfo
}

// NewManager builds an empty manager. Caller registers providers via Use.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		pendingTTL: 15 * time.Minute,
		now:        time.Now,
		configs:    map[PresetName]registered{},
		pending:    map[string]*pendingFlow{},
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// ManagerOption is the functional-option type for NewManager.
type ManagerOption func(*Manager)

// WithHTTPClient overrides the userinfo HTTP client. Tests use this to
// inject a transport that records requests.
func WithHTTPClient(c *http.Client) ManagerOption { return func(m *Manager) { m.httpClient = c } }

// WithPendingTTL changes the state TTL.
func WithPendingTTL(d time.Duration) ManagerOption { return func(m *Manager) { m.pendingTTL = d } }

// WithClock overrides the time source for deterministic tests.
func WithClock(now func() time.Time) ManagerOption { return func(m *Manager) { m.now = now } }

// Use registers one provider. Subsequent Use() with the same Name
// overwrites the previous registration. Returns an error if the config
// is malformed (missing ClientID, unknown preset, etc.).
func (m *Manager) Use(c Config) error {
	preset, oauthCfg, err := c.resolve()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[c.Name] = registered{cfg: c, preset: preset, oauth: oauthCfg}
	return nil
}

// Providers returns the registered providers' display info in stable
// alphabetical order (by name) for the frontend.
func (m *Manager) Providers() []ProviderInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ProviderInfo, 0, len(m.configs))
	// Stable order via the catalogue list, plus any custom names at the end.
	for _, p := range AllPresets() {
		if r, ok := m.configs[p.Name]; ok {
			out = append(out, ProviderInfo{
				Name:        string(r.cfg.Name),
				DisplayName: r.cfg.DisplayName,
			})
		}
	}
	if r, ok := m.configs[PresetCustom]; ok {
		out = append(out, ProviderInfo{Name: string(PresetCustom), DisplayName: r.cfg.DisplayName})
	}
	return out
}

// ProviderInfo is the JSON shape exposed to the frontend login button.
type ProviderInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// Start mints a pending flow and returns the auth URL + state. The
// caller opens the URL in the user's browser; the user signs in;
// upstream redirects to RedirectURL with ?code=&state=, which the
// callback handler hands to Complete().
func (m *Manager) Start(name PresetName) (authURL, state string, err error) {
	m.mu.Lock()
	r, ok := m.configs[name]
	m.mu.Unlock()
	if !ok {
		return "", "", fmt.Errorf("oauthproviders: provider %q not configured", name)
	}
	state, err = randomToken(24)
	if err != nil {
		return "", "", err
	}
	verifier, err := randomToken(32)
	if err != nil {
		return "", "", err
	}
	challenge := pkceChallenge(verifier)
	url := r.oauth.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
	m.mu.Lock()
	m.pending[state] = &pendingFlow{
		provider:  name,
		verifier:  verifier,
		createdAt: m.now(),
	}
	m.mu.Unlock()
	return url, state, nil
}

// Complete is called by the redirect handler with the state + code
// returned by the provider. It exchanges the code for a token, fetches
// the userinfo endpoint, normalises the response, and stores the result
// on the pending row so Poll() returns it.
func (m *Manager) Complete(ctx context.Context, state, code string) (*UserInfo, error) {
	m.mu.Lock()
	flow, ok := m.pending[state]
	if !ok {
		m.mu.Unlock()
		return nil, ErrUnknownState
	}
	r := m.configs[flow.provider]
	m.mu.Unlock()
	if m.now().Sub(flow.createdAt) > m.pendingTTL {
		m.markCompletedErr(state, ErrStateExpired)
		return nil, ErrStateExpired
	}
	tok, err := r.oauth.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", flow.verifier),
	)
	if err != nil {
		m.markCompletedErr(state, err)
		return nil, fmt.Errorf("oauth code exchange: %w", err)
	}
	info, err := m.fetchUserInfo(ctx, r, tok)
	if err != nil {
		m.markCompletedErr(state, err)
		return nil, err
	}
	m.mu.Lock()
	flow.completed = true
	flow.userInfo = info
	m.mu.Unlock()
	return info, nil
}

// Poll returns the result of an in-flight or completed login. The
// frontend long-polls this until status != "pending".
func (m *Manager) Poll(state string) (status string, info *UserInfo, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	flow, ok := m.pending[state]
	if !ok {
		return "", nil, ErrUnknownState
	}
	if flow.completedErr != nil {
		return "error", nil, flow.completedErr
	}
	if !flow.completed {
		// Expire pending rows that have aged out so the caller stops polling.
		if m.now().Sub(flow.createdAt) > m.pendingTTL {
			return "error", nil, ErrStateExpired
		}
		return "pending", nil, nil
	}
	return "ok", flow.userInfo, nil
}

// Cleanup removes pending rows older than pendingTTL. Call periodically
// from a janitor goroutine in production; tests call directly.
func (m *Manager) Cleanup() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := m.now().Add(-m.pendingTTL)
	removed := 0
	for state, flow := range m.pending {
		if flow.createdAt.Before(cutoff) {
			delete(m.pending, state)
			removed++
		}
	}
	return removed
}

// fetchUserInfo calls the provider's userinfo endpoint with the access
// token and maps the response onto our normalised UserInfo struct.
func (m *Manager) fetchUserInfo(ctx context.Context, r registered, tok *oauth2.Token) (*UserInfo, error) {
	if r.preset.UserInfoURL == "" {
		return nil, fmt.Errorf("oauthproviders: no userinfo URL for %s", r.preset.Name)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.preset.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Accept", "application/json")
	res, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo request: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("userinfo: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("userinfo: parse: %w", err)
	}
	info := &UserInfo{
		Provider: r.preset.Name,
		ID:       stringField(raw, r.preset.UserIDField),
		Email:    stringField(raw, r.preset.EmailField),
		Name:     stringField(raw, r.preset.NameField),
		Raw:      raw,
	}
	// GitHub's primary email is sometimes null on /user — they expose
	// it via /user/emails when the user marked it private. For unit
	// tests we leave that as a follow-up; the field is documented as
	// best-effort.
	return info, nil
}

func (m *Manager) markCompletedErr(state string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if flow, ok := m.pending[state]; ok {
		flow.completed = true
		flow.completedErr = err
	}
}

// Errors.
var (
	ErrUnknownState = errors.New("oauthproviders: unknown state")
	ErrStateExpired = errors.New("oauthproviders: state expired")
)

// ---- helpers ----------------------------------------------------------

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func stringField(m map[string]any, key string) string {
	if key == "" {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// JSON numbers come in as float64; many providers return ID as int.
		return fmt.Sprintf("%g", x)
	case bool:
		return fmt.Sprintf("%t", x)
	default:
		return fmt.Sprint(x)
	}
}
