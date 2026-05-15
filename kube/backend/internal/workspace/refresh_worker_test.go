package workspace

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argues/argus/internal/secretstore"
)

// mockRefresher implements Provider + Refresher. Counts refresh calls
// so the tests can assert "exactly one call" or "single-flight
// coalesced N concurrent calls into one".
type mockRefresher struct {
	svc       Service
	calls     atomic.Int64
	delay     time.Duration
	failFirst atomic.Bool
}

func (p *mockRefresher) Service() Service { return p.svc }
func (p *mockRefresher) Start(context.Context, string, string) (AuthURL, error) {
	return AuthURL{State: "s"}, nil
}
func (p *mockRefresher) Complete(context.Context, string, string) (CompleteResult, error) {
	return CompleteResult{}, nil
}
func (p *mockRefresher) Refresh(_ context.Context, _ string) (Token, error) {
	p.calls.Add(1)
	if p.delay > 0 {
		time.Sleep(p.delay)
	}
	if p.failFirst.CompareAndSwap(true, false) {
		return Token{}, errors.New("provider: 401 invalid_grant")
	}
	return Token{
		AccessToken: "fresh-access",
		// Empty RefreshToken so the manager exercises its "preserve
		// previous" branch.
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Scope:     "rw",
	}, nil
}

func newWorkerFixture(t *testing.T) (*Manager, *Store, *mockRefresher) {
	t.Helper()
	store := newStore(t)
	logger := slog.New(slog.NewTextHandler(testDiscard{}, nil))
	m := NewManager(store, logger)
	p := &mockRefresher{svc: ServiceSlack}
	m.Register(p)
	return m, store, p
}

func seedConnection(t *testing.T, store *Store, expiresIn time.Duration) Connection {
	t.Helper()
	ctx := context.Background()
	c, err := store.Upsert(ctx, Connection{
		UserID:              "u",
		Service:             ServiceSlack,
		ExternalWorkspaceID: "T1",
		DisplayName:         "Test",
	}, Token{
		AccessToken:  "stale-access",
		RefreshToken: "rt",
		TokenType:    "bearer",
		ExpiresAt:    time.Now().Add(expiresIn),
		Scope:        "r",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return c
}

func TestStore_ListExpiringSoon(t *testing.T) {
	store := newStore(t)
	// Three connections: one expired, one about to, one fresh.
	store.crypto = NewCrypto(secretstore.NewMemoryStore())
	ctx := context.Background()
	for i, expIn := range []time.Duration{-1 * time.Hour, 5 * time.Minute, 24 * time.Hour} {
		_, err := store.Upsert(ctx, Connection{
			UserID:              "u",
			Service:             ServiceSlack,
			ExternalWorkspaceID: "T" + string(rune('A'+i)),
			DisplayName:         "Test",
		}, Token{
			AccessToken: "a", RefreshToken: "r",
			ExpiresAt: time.Now().Add(expIn),
		})
		if err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	// Cutoff = now + 15m → catches the first two, not the 24h one.
	cutoff := time.Now().Add(15 * time.Minute).Unix()
	got, err := store.ListExpiringSoon(ctx, cutoff)
	if err != nil {
		t.Fatalf("ListExpiringSoon: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 expiring, got %d: %+v", len(got), got)
	}
}

func TestRefreshWorker_TickRefreshesStale(t *testing.T) {
	m, store, p := newWorkerFixture(t)
	seedConnection(t, store, 5*time.Minute) // inside 15m threshold
	w := NewRefreshWorker(m, store, silentLogger{}, time.Hour, 15*time.Minute)
	w.tick(context.Background())
	if p.calls.Load() != 1 {
		t.Fatalf("expected 1 refresh call, got %d", p.calls.Load())
	}
	tok, err := store.GetToken(context.Background(), "")
	_ = err
	_ = tok
}

func TestRefreshWorker_SkipsFresh(t *testing.T) {
	m, store, p := newWorkerFixture(t)
	seedConnection(t, store, 1*time.Hour) // outside 15m threshold
	w := NewRefreshWorker(m, store, silentLogger{}, time.Hour, 15*time.Minute)
	w.tick(context.Background())
	if p.calls.Load() != 0 {
		t.Fatalf("expected 0 refresh calls (token still fresh), got %d", p.calls.Load())
	}
}

func TestRefreshWorker_SkipsNonExpiringTokens(t *testing.T) {
	// Slack bot tokens (ExpiresAt zero) should never be touched by the
	// worker — ListExpiringSoon's `expires_at > 0` clause excludes
	// them, and even if a row leaked through, RefreshIfStale bails on
	// IsZero.
	store := newStore(t)
	logger := slog.New(slog.NewTextHandler(testDiscard{}, nil))
	m := NewManager(store, logger)
	p := &mockRefresher{svc: ServiceSlack}
	m.Register(p)

	ctx := context.Background()
	_, err := store.Upsert(ctx, Connection{
		UserID: "u", Service: ServiceSlack, ExternalWorkspaceID: "T",
	}, Token{AccessToken: "bot", RefreshToken: "r"}) // ExpiresAt zero
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	w := NewRefreshWorker(m, store, silentLogger{}, time.Hour, 15*time.Minute)
	w.tick(ctx)
	if p.calls.Load() != 0 {
		t.Fatalf("non-expiring token was refreshed: %d calls", p.calls.Load())
	}
}

func TestRefreshWorker_FailedRefreshDeletesRow(t *testing.T) {
	m, store, p := newWorkerFixture(t)
	c := seedConnection(t, store, 5*time.Minute)
	p.failFirst.Store(true)
	w := NewRefreshWorker(m, store, silentLogger{}, time.Hour, 15*time.Minute)
	w.tick(context.Background())
	if _, err := store.Get(context.Background(), c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected connection deleted after refresh failure, got %v", err)
	}
}

func TestRefreshWorker_SingleFlightWithConcurrentTokenCall(t *testing.T) {
	m, store, p := newWorkerFixture(t)
	c := seedConnection(t, store, 5*time.Minute)
	p.delay = 50 * time.Millisecond

	// One worker tick + one adapter call hitting refreshNow simultaneously
	// should coalesce into a single upstream Refresh.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = m.RefreshIfStale(context.Background(), c.ID, 15*time.Minute) }()
	go func() { defer wg.Done(); _, _ = m.Token(context.Background(), c.ID) }()
	wg.Wait()

	if got := p.calls.Load(); got != 1 {
		t.Fatalf("singleflight failed: %d refresh calls", got)
	}
}

func TestRefreshWorker_RunStopsOnContextCancel(t *testing.T) {
	m, store, _ := newWorkerFixture(t)
	seedConnection(t, store, 5*time.Minute)
	w := NewRefreshWorker(m, store, silentLogger{}, 10*time.Millisecond, 15*time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop on context cancel")
	}
}

func TestRefreshWorker_DefaultIntervalsApplied(t *testing.T) {
	// Zero values fall back to defaults rather than tight-looping.
	store := newStore(t)
	m := NewManager(store, nil)
	w := NewRefreshWorker(m, store, nil, 0, 0)
	if w.interval != 5*time.Minute {
		t.Errorf("default interval wrong: %v", w.interval)
	}
	if w.threshold != 15*time.Minute {
		t.Errorf("default threshold wrong: %v", w.threshold)
	}
}
