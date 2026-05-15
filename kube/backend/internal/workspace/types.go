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
	ServiceGDocs   Service = "gdocs"
	ServiceGSheets Service = "gsheets"
	ServiceGTasks  Service = "gtasks"
)

// supportedServices is the closed enum the manager accepts. Adding a
// new service means adding it here AND registering a Provider in
// WorkspaceManager.
var supportedServices = map[Service]bool{
	ServiceGDocs: true, ServiceGSheets: true, ServiceGTasks: true,
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

// Integration is the marker every post-OAuth adapter satisfies.
// Adapters should also satisfy one or more of the narrow capability
// interfaces below (DocEditor, SheetEditor, TaskManager) rather than
// returning errors from no-op methods.
type Integration interface {
	Service() Service
}

// DocEditor is implemented by Google Docs adapters.
type DocEditor interface {
	Integration
	CreateDoc(ctx context.Context, token Token, title string) (docID string, err error)
	ReadDoc(ctx context.Context, token Token, docID string) (text string, err error)
	AppendText(ctx context.Context, token Token, docID, text string) error
}

// SheetEditor is implemented by Google Sheets adapters. Ranges use A1
// notation ("Sheet1!A1:C10").
type SheetEditor interface {
	Integration
	CreateSheet(ctx context.Context, token Token, title string) (sheetID string, err error)
	ReadRange(ctx context.Context, token Token, sheetID, a1Range string) (values [][]string, err error)
	WriteRange(ctx context.Context, token Token, sheetID, a1Range string, values [][]string) error
}

// TaskManager is implemented by Google Tasks adapters.
type TaskManager interface {
	Integration
	ListTaskLists(ctx context.Context, token Token) ([]TaskList, error)
	ListTasks(ctx context.Context, token Token, listID string) ([]Task, error)
	CreateTask(ctx context.Context, token Token, listID string, t Task) (taskID string, err error)
	CompleteTask(ctx context.Context, token Token, listID, taskID string) error
}

// TaskList is a Google Tasks list (e.g. "My Tasks", "Work").
type TaskList struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Task is one entry within a TaskList. Due is the RFC3339 timestamp
// the Google API returns; empty when no due date is set.
type Task struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
	Notes string `json:"notes,omitempty"`
	Due   string `json:"due,omitempty"`
	Done  bool   `json:"done,omitempty"`
}
