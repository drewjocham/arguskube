package workspace

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
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
	// (returned by Provider.Start) to the binding the callback needs
	// to know which user + service the flow belongs to. Entries TTL
	// out after 10 minutes — a slow user shouldn't tie up state forever
	// and a stale entry shouldn't be reusable.
	pendingMu sync.Mutex
	pending   map[string]pendingFlow
	now       func() time.Time

	// refreshSF coalesces concurrent token refreshes per connectionID so
	// a burst of adapter calls produces one Refresh roundtrip, not N.
	refreshSF singleflight.Group
}

// pendingFlow is the (userID, service) tuple bound to an OAuth state
// nonce. The callback handler resolves the binding by state without
// trusting the redirect URL params.
type pendingFlow struct {
	userID    string
	service   Service
	createdAt time.Time
}

// pendingTTL is how long an unredeemed pending state stays valid.
// Slack/Google's own OAuth round-trip rarely exceeds 60s; 10m covers
// the user-fumbles-the-password case.
const pendingTTL = 10 * time.Minute

func NewManager(store *Store, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		store:     store,
		logger:    logger,
		providers: map[Service]Provider{},
		pending:   map[string]pendingFlow{},
		now:       time.Now,
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
	m.gcPendingLocked()
	m.pending[a.State] = pendingFlow{userID: userID, service: svc, createdAt: m.now()}
	m.pendingMu.Unlock()
	return a, nil
}

// LookupPendingService returns the service a pending state belongs to
// without consuming it — used by the HTTP callback handler so the
// confirmation page can postMessage the right service back to the
// opener. Returns ErrNotFound if state is unknown or expired.
func (m *Manager) LookupPendingService(state string) (Service, error) {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()
	m.gcPendingLocked()
	flow, ok := m.pending[state]
	if !ok {
		return "", ErrNotFound
	}
	return flow.service, nil
}

// gcPendingLocked drops expired entries. Caller must hold pendingMu.
func (m *Manager) gcPendingLocked() {
	cutoff := m.now().Add(-pendingTTL)
	for state, flow := range m.pending {
		if flow.createdAt.Before(cutoff) {
			delete(m.pending, state)
		}
	}
}

// Complete is the OAuth callback path. It claims the pending state,
// asks the Provider to finalize, then upserts a Connection + Token.
func (m *Manager) Complete(ctx context.Context, svc Service, state, code string) (Connection, error) {
	p, ok := m.providers[svc]
	if !ok {
		return Connection{}, fmt.Errorf("workspace: no provider registered for %q", svc)
	}

	m.pendingMu.Lock()
	m.gcPendingLocked()
	flow, known := m.pending[state]
	if known {
		delete(m.pending, state)
	}
	m.pendingMu.Unlock()
	if !known {
		return Connection{}, errors.New("workspace: unknown or expired state — restart the connect flow")
	}
	userID := flow.userID

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

// Token decrypts the token for a connection, transparently refreshing
// it when expired and the provider implements Refresher. Reserved for
// adapter use; callers MUST NOT log or serialize the returned value.
//
// Concurrent calls for the same connection coalesce into a single
// Refresh via singleflight so a stampede of adapter calls doesn't
// hammer the upstream token endpoint.
func (m *Manager) Token(ctx context.Context, connectionID string) (Token, error) {
	tok, err := m.store.GetToken(ctx, connectionID)
	if err != nil {
		return Token{}, err
	}
	if !tok.Expired() {
		return tok, nil
	}
	return m.refreshNow(ctx, connectionID, tok)
}

// RefreshIfStale proactively refreshes a token when it expires within
// the threshold window. The background worker calls this on every
// connection it knows about so the first user action of the day
// doesn't pay synchronous refresh latency.
//
// When the token is still fresh, the refresh is a no-op (returns nil
// without contacting the upstream). When the threshold trips, the
// refresh runs through the same singleflight key as Token's on-demand
// path — concurrent worker + adapter calls coalesce into one request.
func (m *Manager) RefreshIfStale(ctx context.Context, connectionID string, threshold time.Duration) error {
	tok, err := m.store.GetToken(ctx, connectionID)
	if err != nil {
		return err
	}
	if tok.ExpiresAt.IsZero() {
		// Non-expiring tokens (Slack bot tokens) — nothing to do.
		return nil
	}
	if time.Until(tok.ExpiresAt) > threshold {
		return nil
	}
	if tok.RefreshToken == "" {
		// Without a refresh token there's nothing the worker can do;
		// the next adapter call will surface a 401 and the UI will
		// prompt the user to reconnect.
		return nil
	}
	_, err = m.refreshNow(ctx, connectionID, tok)
	return err
}

// refreshNow runs the actual refresh round-trip. Single-flighted per
// connectionID so the background worker and an on-demand adapter call
// arriving in the same second don't hammer the upstream token endpoint.
func (m *Manager) refreshNow(ctx context.Context, connectionID string, tok Token) (Token, error) {
	if tok.RefreshToken == "" {
		// No refresh material; nothing to do. Return the (expired) token
		// and let the adapter surface the upstream 401 — that's the
		// signal to the UI that the user must reconnect.
		return tok, nil
	}
	svc, err := m.store.GetService(ctx, connectionID)
	if err != nil {
		return Token{}, err
	}
	prov, ok := m.providers[svc]
	if !ok {
		return tok, nil
	}
	rf, ok := prov.(Refresher)
	if !ok {
		return tok, nil
	}

	v, err, _ := m.refreshSF.Do(connectionID, func() (interface{}, error) {
		fresh, err := rf.Refresh(ctx, tok.RefreshToken)
		if err != nil {
			// A failed refresh almost always means the refresh_token was
			// revoked (user de-authorised, password reset, scope change).
			// Drop the stale row so the UI prompts a reconnect rather
			// than retrying with dead credentials forever.
			_ = m.store.Delete(ctx, connectionID)
			return Token{}, fmt.Errorf("workspace: refresh failed — reconnect required: %w", err)
		}
		// Preserve the previous refresh token if the provider didn't
		// rotate one (Google's typical behaviour).
		if fresh.RefreshToken == "" {
			fresh.RefreshToken = tok.RefreshToken
		}
		fresh.ConnectionID = connectionID
		if err := m.store.UpdateToken(ctx, connectionID, fresh); err != nil {
			return Token{}, fmt.Errorf("workspace: persist refreshed token: %w", err)
		}
		return fresh, nil
	})
	if err != nil {
		return Token{}, err
	}
	return v.(Token), nil
}
