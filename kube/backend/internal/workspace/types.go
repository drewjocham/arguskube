// Package workspace manages Argus' integrations with Google
// document/task systems (Docs, Sheets, Tasks). Phase 1A — already
// landed — shipped the foundation:
//
//   - Connection + Token storage with AES-256-GCM at-rest encryption
//     (master key from secretstore, same chain as dbconfig).
//   - OAuth orchestration via a pluggable Provider interface so
//     per-service adapters bolt on without touching the manager.
//   - A WorkspaceManager exposed through a small Wails API for
//     listing, deleting, and round-tripping connections.
//
// Phase 1B+ wires real Google adapters against the narrow
// capability interfaces below (DocEditor, SheetEditor, TaskManager).
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
	// Messaging services. Slack uses bot tokens (never expire); Google
	// Chat shares the unified Google grant via additional scopes.
	ServiceSlack Service = "slack"
	ServiceGChat Service = "gchat"
	// ServiceGoogle is the unified Google Workspace service: one OAuth
	// grant covers Docs + Sheets + Tasks. Adapters key off this single
	// connection rather than three separate ones.
	ServiceGoogle Service = "google"

	// ServiceGCal is the Google Calendar capability. Like Docs/Sheets/Tasks,
	// it shares the unified Google OAuth grant under ServiceGoogle.
	ServiceGCal Service = "gcal"

	// ServiceGDocs/GSheets/GTasks are retained as per-capability
	// aliases so callers that only need one slice (a Docs-only test
	// fixture, e.g.) can name it directly. The active OAuth grant
	// lives under ServiceGoogle when the full unified flow is wired.
	ServiceGDocs   Service = "gdocs"
	ServiceGSheets Service = "gsheets"
	ServiceGTasks  Service = "gtasks"
	// ServiceICloud is the Apple iCloud integration. Uses app-specific
	// passwords (not OAuth) — stored in the same encrypted token store
	// with no refresh token or expiry.
	ServiceICloud Service = "icloud"
)

// supportedServices is the closed enum the manager accepts. Adding a
// new service means adding it here AND registering a Provider in
// WorkspaceManager.
var supportedServices = map[Service]bool{
	ServiceSlack:   true,
	ServiceGChat:   true,
	ServiceGoogle:  true,
	ServiceGCal:    true,
	ServiceGDocs:   true,
	ServiceGSheets: true,
	ServiceGTasks:  true,
	ServiceICloud:  true,
}

// Connection is one user's link to a specific external workspace —
// e.g. "user 42 → Google account alice@example.com". A user can hold
// multiple connections per service (work + personal Google accounts)
// thanks to the UNIQUE(user_id, service, external_workspace_id) index.
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
// implements this. The WorkspaceManager doesn't know which provider is
// which — it routes by Service.
type Provider interface {
	Service() Service
	// Start returns the URL the user must visit to authorize the
	// integration. The opaque state is round-tripped to Complete.
	Start(ctx context.Context, userID, redirectURL string) (AuthURL, error)
	// Complete validates the callback parameters, exchanges the code
	// for tokens, and fetches the external identity. Returns the data
	// the storage layer needs to persist.
	Complete(ctx context.Context, state, code string) (CompleteResult, error)
}

// Refresher is implemented by Providers whose access tokens expire.
// Manager.Token() calls Refresh transparently when the cached token is
// past its expiry, persisting the new token before returning it.
//
// Implementations MUST tolerate an empty refresh-token response field
// (Google preserves the original on rotation); callers in Manager keep
// the previous refresh token if Refresh returns an empty one.
type Refresher interface {
	Refresh(ctx context.Context, refreshToken string) (Token, error)
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

// Integration is the marker every post-OAuth adapter satisfies. The
// narrower capability interfaces (Messenger, DocEditor, SheetEditor,
// TaskManager) live in the adapter source files so each one can carry
// the richer shape its API actually returns (Doc with title+url, Task
// with status+updated, etc.) without bloating this types file.
type Integration interface {
	Service() Service
}

// Messenger is implemented by chat integrations (Slack, Google Chat).
// Adapter files (slack.go, gchat.go) implement it with a per-service
// channel/space ID convention.
type Messenger interface {
	Integration
	ListChannels(ctx context.Context, token Token) ([]Channel, error)
	Send(ctx context.Context, token Token, channelID, text string) error
}

// Channel is the addressable destination a Messenger sends into. The
// ID is opaque to callers — Slack channel IDs (`C…`), Google Chat
// space resource names (`spaces/…`).
type Channel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Calendarer is implemented by calendar integrations (Google Calendar,
// iCloud CalDAV). Following the same pattern as Messenger/DocEditor/
// SheetEditor — the narrower interface lives here, the richer Event
// struct with provider-specific fields lives in the adapter file.
type Calendarer interface {
	Integration
	ListEvents(ctx context.Context, token Token, start, end string) ([]Event, error)
	CreateEvent(ctx context.Context, token Token, ev Event) (Event, error)
	UpdateEvent(ctx context.Context, token Token, eventID string, ev Event) (Event, error)
	DeleteEvent(ctx context.Context, token Token, eventID string) error
}

// Event is the normalised calendar event returned by Calendarer
// implementations. Times are RFC 3339 strings (what Google Calendar v3
// and CalDAV both speak natively). Provider-specific extra fields
// (attendees, recurrence, etc.) are surfaced by the adapter's own
// richer type — this is the common subset every calendar adapter must
// return.
type Event struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	Location    string `json:"location,omitempty"`
	Start       string `json:"start"` // RFC 3339
	End         string `json:"end"`   // RFC 3339
	HTMLink     string `json:"htmlLink,omitempty"`
}
