// Package connector turns a dbconfig.DBConnection ID into a usable
// *sql.DB. It caches one pool per connection ID so repeated MCP tool
// calls don't pay the handshake cost, and centralizes the "look up
// connection -> decrypt password -> open driver" plumbing that would
// otherwise be smeared across every analyzer.
//
// Connectors are tied to one Argus process; the cache is not shared.
// Closing the connector closes every cached pool — the Wails app calls
// this on shutdown.
package connector

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/argues/argus/internal/dbconfig"

	// Side-effect imports register database/sql drivers. We register
	// every dialect dbconfig.DSN() can produce so the connector doesn't
	// have to know which one a given config wants.
	_ "github.com/jackc/pgx/v5/stdlib" // pgx => "pgx"
	_ "modernc.org/sqlite"             // pure-Go sqlite => "sqlite"
)

// Pool caches one *sql.DB per connection ID. *sql.DB is itself a pool,
// so cache hits avoid both the dbconfig lookup and the TCP handshake.
type Pool struct {
	configs *dbconfig.Store

	mu       sync.Mutex
	conns    map[string]*entry
	idleTime time.Duration
	maxOpen  int
}

type entry struct {
	db        *sql.DB
	openedAt  time.Time
	updatedAt int64 // dbconfig.UpdatedAt at open time; invalidates on edit
}

// New returns a Pool. idleTime caps SetConnMaxIdleTime on every cached
// *sql.DB so long-idle desktop sessions don't keep PG sessions open
// forever. maxOpen bounds SetMaxOpenConns; 0 lets database/sql decide.
func New(configs *dbconfig.Store, idleTime time.Duration, maxOpen int) *Pool {
	if idleTime <= 0 {
		idleTime = 5 * time.Minute
	}
	return &Pool{
		configs:  configs,
		conns:    map[string]*entry{},
		idleTime: idleTime,
		maxOpen:  maxOpen,
	}
}

// Get returns a cached *sql.DB for the given connection, or opens one.
// The returned *sql.DB is owned by the Pool — callers must not Close it.
// If the underlying dbconfig row has been updated since the pool was
// opened, the cached pool is discarded and a fresh one is built (so a
// rotated password takes effect on the next call).
func (p *Pool) Get(ctx context.Context, id string) (*sql.DB, dbconfig.DBConnection, error) {
	cfg, err := p.configs.Get(ctx, id)
	if err != nil {
		return nil, dbconfig.DBConnection{}, fmt.Errorf("connector: lookup %s: %w", id, err)
	}
	if !cfg.Enabled {
		return nil, cfg, fmt.Errorf("connector: connection %q is disabled", cfg.Name)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if e, ok := p.conns[id]; ok && e.updatedAt == cfg.UpdatedAt {
		return e.db, cfg, nil
	}
	// Either no cache, or the cached version is stale (config edited).
	if e, ok := p.conns[id]; ok {
		_ = e.db.Close()
		delete(p.conns, id)
	}

	driver, err := driverFor(cfg.DBType)
	if err != nil {
		return nil, cfg, err
	}
	dsn, err := cfg.DSN()
	if err != nil {
		return nil, cfg, err
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, cfg, fmt.Errorf("connector: open %s: %w", cfg.DBType, err)
	}
	if p.maxOpen > 0 {
		db.SetMaxOpenConns(p.maxOpen)
	}
	db.SetConnMaxIdleTime(p.idleTime)

	// Validate the connection eagerly. A bad password should surface
	// here, not on the user's first analysis call.
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, cfg, fmt.Errorf("connector: ping %s: %w", cfg.Name, err)
	}

	p.conns[id] = &entry{db: db, openedAt: time.Now(), updatedAt: cfg.UpdatedAt}
	return db, cfg, nil
}

// Close drops the cached pool for one connection (idempotent).
func (p *Pool) Close(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.conns[id]
	if !ok {
		return nil
	}
	delete(p.conns, id)
	return e.db.Close()
}

// CloseAll drains every cached pool. Returns the first error
// encountered but always attempts to close every entry.
func (p *Pool) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for id, e := range p.conns {
		if err := e.db.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("connector: close %s: %w", id, err)
		}
		delete(p.conns, id)
	}
	return firstErr
}

// driverFor maps dbconfig.DBType to a registered database/sql driver
// name. Dialects that aren't compiled in (no driver imported above)
// return an explicit error rather than a confusing "unknown driver"
// from database/sql.
func driverFor(t dbconfig.DBType) (string, error) {
	switch t {
	case dbconfig.DBPostgres:
		return "pgx", nil
	case dbconfig.DBSQLite:
		return "sqlite", nil
	}
	return "", fmt.Errorf("connector: no driver compiled in for %s (Phase 2+ adds the rest)", t)
}
