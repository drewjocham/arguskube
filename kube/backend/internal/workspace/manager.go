package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

// Manager is the entry point the App holds. It routes Start/Complete
// calls to the right Provider by Service, persists what Complete
// returns via the Store, and lists/deletes connections.
//
// Phase 1A wires the OAuth state-tracking and the Provider registry.
// Phase 1B+ implementations of Integration (Slack, Google, …) plug into
// the same Manager without changes.
type Manager struct {
	store     *Store
	logger    *slog.Logger
	providers map[Service]Provider

	// pendingMu guards pending; pending maps an opaque state nonce
	// (returned by Provider.Start) to the userID that initiated the
	// flow. The callback handler looks the state up to know who to
	// upsert the connection for. Entries stay until Complete consumes
	// them; we don't TTL them in 1A — that's a 1B follow-up.
	pendingMu sync.Mutex
	pending   map[string]string
}

func NewManager(store *Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		store:     store,
		logger:    logger,
		providers: map[Service]Provider{},
		pending:   map[string]string{},
	}
}

// Register wires a Provider. Re-registering the same Service panics so
// duplicate-init bugs surface at boot rather than at first OAuth
// attempt. Mirrors the broker.Register convention.
func (m *Manager) Register(p Provider) {
	if p == nil {
		panic("workspace: nil Provider")
	}
	svc := p.Service()
	if _, ok := m.providers[svc]; ok {
		panic(fmt.Sprintf("workspace: Provider for %q already registered", svc))
	}
	m.providers[svc] = p
}

// HasProvider tells the UI whether a given service is wired in this
// build. Phase 1A only ships the TestProvider (for dev/test); 1B/2
// add real providers and HasProvider lights up the corresponding
// connect button.
func (m *Manager) HasProvider(svc Service) bool {
	_, ok := m.providers[svc]
	return ok
}

// AvailableServices returns the closed enum of services this Manager
// can actually connect to right now (i.e. has a Provider registered
// for). The frontend uses this to build the connect-buttons list.
func (m *Manager) AvailableServices() []Service {
	out := make([]Service, 0, len(m.providers))
	for svc := range m.providers {
		out = append(out, svc)
	}
	return out
}

// Start initiates an OAuth flow. Returns the URL the desktop should
// open in the user's browser plus the state nonce the callback will
// echo back.
func (m *Manager) Start(ctx context.Context, userID string, svc Service, redirectURL string) (AuthURL, error) {
	p, ok := m.providers[svc]
	if !ok {
		return AuthURL{}, fmt.Errorf("workspace: no provider registered for %q", svc)
	}
	a, err := p.Start(ctx, userID, redirectURL)
	if err != nil {
		return AuthURL{}, err
	}

	m.pendingMu.Lock()
	m.pending[a.State] = userID
	m.pendingMu.Unlock()
	return a, nil
}

// Complete is the OAuth callback path. It claims the pending state,
// asks the Provider to finalize, then upserts a Connection + Token.
func (m *Manager) Complete(ctx context.Context, svc Service, state, code string) (Connection, error) {
	p, ok := m.providers[svc]
	if !ok {
		return Connection{}, fmt.Errorf("workspace: no provider registered for %q", svc)
	}

	m.pendingMu.Lock()
	userID, known := m.pending[state]
	if known {
		delete(m.pending, state)
	}
	m.pendingMu.Unlock()
	if !known {
		return Connection{}, errors.New("workspace: unknown or expired state — restart the connect flow")
	}

	res, err := p.Complete(ctx, state, code)
	if err != nil {
		return Connection{}, fmt.Errorf("workspace: provider %q complete: %w", svc, err)
	}

	// Provider may or may not echo UserID; we trust the Start-time
	// binding (state → user) regardless.
	res.UserID = userID

	c := Connection{
		UserID:              res.UserID,
		Service:             svc,
		ExternalWorkspaceID: res.ExternalWorkspaceID,
		DisplayName:         res.DisplayName,
		Email:               res.Email,
		AvatarURL:           res.AvatarURL,
	}
	saved, err := m.store.Upsert(ctx, c, res.Token)
	if err != nil {
		return Connection{}, err
	}
	m.logger.InfoContext(ctx, "workspace: connection saved",
		slog.String("service", string(svc)),
		slog.String("user_id", userID),
		slog.String("connection_id", saved.ID),
	)
	return saved, nil
}

// List returns every connection a user owns. Tokens are not included.
func (m *Manager) List(ctx context.Context, userID string) ([]Connection, error) {
	return m.store.List(ctx, userID)
}

// Delete removes a connection (cascade deletes its token).
func (m *Manager) Delete(ctx context.Context, id string) error {
	return m.store.Delete(ctx, id)
}

// Token decrypts the token for a connection. Reserved for adapter use;
// callers MUST NOT log or serialize the returned value.
func (m *Manager) Token(ctx context.Context, connectionID string) (Token, error) {
	return m.store.GetToken(ctx, connectionID)
}
