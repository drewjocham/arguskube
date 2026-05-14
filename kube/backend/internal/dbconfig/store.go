package dbconfig

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotFound is returned by Get/Delete when the ID is unknown. The
// alertproc & incidents packages use a similar sentinel; we mirror that
// convention so callers can switch on error identity cleanly.
var ErrNotFound = errors.New("dbconfig: connection not found")

// ErrDuplicateName is returned by Upsert when the requested Name
// collides with an existing row (UNIQUE index on db_connections.name).
// The UI uses this to surface a clear "pick a different name" instead
// of a raw driver error.
var ErrDuplicateName = errors.New("dbconfig: connection name already in use")

// querier is the subset of *sql.DB / *sql.Tx that Store needs. Lets
// tests pass an in-memory database without depending on sqlitedb's
// migration system.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store persists DBConnections to SQLite, transparently encrypting and
// decrypting credentials. The on-disk representation is the
// db_connections table created by sqlitedb migration #15.
type Store struct {
	db     querier
	crypto *Crypto
	now    func() time.Time
}

// NewStore wires Store onto the shared sqlitedb. The Crypto can be a
// real one (production) or one backed by an in-memory secretstore
// (tests) — Store only sees the Encrypt/Decrypt surface.
func NewStore(db querier, crypto *Crypto) *Store {
	return &Store{db: db, crypto: crypto, now: time.Now}
}

// List returns every connection in name order. Passwords are decrypted
// before return; callers that don't need them should call Redact.
func (s *Store) List(ctx context.Context) ([]DBConnection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, db_type, host, port, user_name, password_enc,
		       db_name, ssl_mode, pool_size, tags, enabled, created_at, updated_at
		FROM db_connections
		ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("dbconfig: list query: %w", err)
	}
	defer rows.Close()

	var out []DBConnection
	for rows.Next() {
		c, err := s.scan(ctx, rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get returns one connection by ID. The returned password is decrypted.
func (s *Store) Get(ctx context.Context, id string) (DBConnection, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, db_type, host, port, user_name, password_enc,
		       db_name, ssl_mode, pool_size, tags, enabled, created_at, updated_at
		FROM db_connections WHERE id = ?`, id)
	c, err := s.scan(ctx, row)
	if errors.Is(err, sql.ErrNoRows) {
		return DBConnection{}, ErrNotFound
	}
	return c, err
}

// Upsert creates or updates a connection. If c.ID is empty a new one is
// generated. Validation is the caller's responsibility *except* for the
// shape checks in Validate(), which we run again here so an unvetted
// connection can never reach SQLite.
func (s *Store) Upsert(ctx context.Context, c DBConnection) (DBConnection, error) {
	if err := c.Validate(); err != nil {
		return DBConnection{}, err
	}
	encPwd, err := s.crypto.Encrypt(ctx, c.Password)
	if err != nil {
		return DBConnection{}, err
	}
	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return DBConnection{}, fmt.Errorf("dbconfig: marshal tags: %w", err)
	}
	now := s.now().Unix()
	if strings.TrimSpace(c.ID) == "" {
		// Create path: assign a fresh ID and stamp CreatedAt.
		c.ID = newID()
		c.CreatedAt = now
	} else {
		// Update path: the ID must reference an existing row. Without
		// this guard a caller could resurrect a deleted row by passing
		// its old ID, which is a data-integrity hazard the reviewer
		// flagged as a blocker.
		existing, gerr := s.Get(ctx, c.ID)
		if errors.Is(gerr, ErrNotFound) {
			return DBConnection{}, ErrNotFound
		}
		if gerr != nil {
			return DBConnection{}, gerr
		}
		c.CreatedAt = existing.CreatedAt
	}
	c.UpdatedAt = now

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO db_connections (
			id, name, db_type, host, port, user_name, password_enc,
			db_name, ssl_mode, pool_size, tags, enabled, created_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			db_type=excluded.db_type,
			host=excluded.host,
			port=excluded.port,
			user_name=excluded.user_name,
			password_enc=excluded.password_enc,
			db_name=excluded.db_name,
			ssl_mode=excluded.ssl_mode,
			pool_size=excluded.pool_size,
			tags=excluded.tags,
			enabled=excluded.enabled,
			updated_at=excluded.updated_at`,
		c.ID, c.Name, string(c.DBType), c.Host, c.Port, c.User, encPwd,
		c.DBName, string(c.SSLMode), c.PoolSize, string(tagsJSON),
		boolToInt(c.Enabled), c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		// Map the driver's UNIQUE-constraint error onto a typed
		// sentinel so the UI can show a friendly "name in use" message
		// instead of leaking SQL details.
		if strings.Contains(err.Error(), "UNIQUE constraint failed: db_connections.name") {
			return DBConnection{}, ErrDuplicateName
		}
		return DBConnection{}, fmt.Errorf("dbconfig: upsert: %w", err)
	}
	return c, nil
}

// Delete removes a connection by ID. Missing IDs return ErrNotFound so
// the UI can distinguish "already gone" from "succeeded".
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM db_connections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("dbconfig: delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("dbconfig: delete rowcount: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// scanner is what Row and Rows both satisfy. Keeps Get and List on the
// same code path.
type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) scan(ctx context.Context, sc scanner) (DBConnection, error) {
	var (
		c        DBConnection
		encPwd   string
		tagsRaw  string
		enabled  int
		dbType   string
		sslMode  string
	)
	err := sc.Scan(
		&c.ID, &c.Name, &dbType, &c.Host, &c.Port, &c.User, &encPwd,
		&c.DBName, &sslMode, &c.PoolSize, &tagsRaw, &enabled,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return DBConnection{}, err
	}
	c.DBType = DBType(dbType)
	c.SSLMode = SSLMode(sslMode)
	c.Enabled = enabled != 0
	if tagsRaw != "" {
		if err := json.Unmarshal([]byte(tagsRaw), &c.Tags); err != nil {
			return DBConnection{}, fmt.Errorf("dbconfig: unmarshal tags: %w", err)
		}
	}
	pwd, err := s.crypto.Decrypt(ctx, encPwd)
	if err != nil {
		return DBConnection{}, err
	}
	c.Password = pwd
	return c, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// newID returns a 16-byte hex string. We don't pull in uuid for this —
// the only requirement is uniqueness within one Argus install.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
