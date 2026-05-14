// Package workspace manages Argus' integrations with messaging
// (Slack, Google Chat) and document/task systems (Google Docs, Sheets,
// Tasks). Phase 1A — this commit — ships the foundation only:
//
//   - Connection + Token storage with AES-256-GCM at-rest encryption
//     (master key from secretstore, same chain as dbconfig).
//   - OAuth orchestration via a pluggable Provider interface so
//     subsequent phases bolt on Slack / Google without touching the
//     manager.
//   - A WorkspaceManager exposed through a small Wails API for
//     listing, deleting, and round-tripping connections.
//
// Per-service adapters (Slack messaging, Google Docs CRUD, …) land in
// Phase 1B+. The Integration interface stub below documents the
// contract those adapters will satisfy.
package workspace

import (
	"context"
	"time"
)

// Service is the canonical name of an integration. Frontend dropdowns
// and the Provider registry both key off this value, so changing it
// is a breaking change.
type Service string

const (
	ServiceSlack   Service = "slack"
	ServiceGChat   Service = "gchat"
	ServiceGDocs   Service = "gdocs"
	ServiceGSheets Service = "gsheets"
	ServiceGTasks  Service = "gtasks"
)

// supportedServices is the closed enum the manager accepts. Adding a
// new service means adding it here AND registering a Provider in
// WorkspaceManager.
var supportedServices = map[Service]bool{
	ServiceSlack: true, ServiceGChat: true,
	ServiceGDocs: true, ServiceGSheets: true, ServiceGTasks: true,
}

// Connection is one user's link to a specific external workspace —
// e.g. "user 42 → Slack team T012345". A single user can hold multiple
// connections per service (prod + corp Slack workspaces) thanks to the
// UNIQUE(user_id, service, external_workspace_id) index.
type Connection struct {
	ID                  string  `json:"id"`
	UserID              string  `json:"user_id"`
	Service             Service `json:"service"`
	ExternalWorkspaceID string  `json:"external_workspace_id,omitempty"`
	DisplayName         string  `json:"display_name"`
	Email               string  `json:"email,omitempty"`
	AvatarURL           string  `json:"avatar_url,omitempty"`
	ConnectedAt         int64   `json:"connected_at"`
	UpdatedAt           int64   `json:"updated_at"`
}

// Token is the OAuth credential pair for one Connection. AccessToken
// and RefreshToken are stored encrypted in the DB; this struct carries
// the decrypted values in-process only — callers must never log or
// JSON-serialize a Token. Use Connection (no tokens) for client
// responses.
type Token struct {
	ConnectionID string    `json:"-"`
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	TokenType    string    `json:"-"`
	ExpiresAt    time.Time `json:"-"`
	Scope        string    `json:"-"`
}

// Expired returns true when the token is past its expiry (with a
// 30-second skew so a near-expired token doesn't slip through to the
// caller).
func (t *Token) Expired() bool {
	if t.ExpiresAt.IsZero() {
		// Zero expiry = non-expiring (Slack bot tokens behave this way).
		return false
	}
	return time.Now().Add(30 * time.Second).After(t.ExpiresAt)
}

// AuthURL is what (a Provider).Start() returns: the URL the user's
// browser must visit to grant consent, plus a state nonce the caller
// stores so the callback can prove it belongs to this attempt.
type AuthURL struct {
	URL   string `json:"url"`
	State string `json:"state"`
}

// Provider abstracts an OAuth flow + identity lookup. Each integration
// (Slack, Google) implements this. The WorkspaceManager doesn't know
// which provider is which — it routes by Service.
//
// Phase 1A ships only a test provider; real Slack/Google providers
// land in 1B/2.
type Provider interface {
	Service() Service
	// Start returns the URL the user must visit to authorize the
	// integration. The opaque state is round-tripped to Complete.
	Start(ctx context.Context, userID, redirectURL string) (AuthURL, error)
	// Complete validates the callback parameters, exchanges the code
	// for tokens, and fetches the external identity (workspace ID,
	// display name, email, avatar). Returns the data the storage
	// layer needs to persist.
	Complete(ctx context.Context, state, code string) (CompleteResult, error)
}

// CompleteResult is what a Provider returns from a successful OAuth
// callback. The WorkspaceManager turns it into Connection + Token
// rows.
type CompleteResult struct {
	UserID              string
	ExternalWorkspaceID string
	DisplayName         string
	Email               string
	AvatarURL           string
	Token               Token
}

// Integration is the post-OAuth surface every adapter ships. Phase 1A
// defines the interface but no real adapters implement it yet — the
// shape captures the WHAT so future phases plug in without
// re-litigating the design.
//
// Adapters that only need a subset of these should satisfy the
// narrower interfaces (Messenger, DocEditor, etc.) instead of returning
// errors from no-op methods.
type Integration interface {
	Service() Service
}

// Messenger is implemented by chat integrations (Slack, Google Chat).
type Messenger interface {
	Integration
	ListChannels(ctx context.Context, token Token) ([]Channel, error)
	Send(ctx context.Context, token Token, channelID, text string) error
}

// Channel is a Slack channel or Google Chat space — the addressable
// destination for Send.
type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
