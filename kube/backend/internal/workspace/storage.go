package workspace

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sentinel errors the UI / callers can switch on.
var (
	ErrNotFound       = errors.New("workspace: connection not found")
	ErrDuplicate      = errors.New("workspace: a connection with these identifiers already exists")
	ErrInvalidService = errors.New("workspace: unsupported service")
)

// querier is the subset of *sql.DB / *sql.Tx the storage layer needs.
// Mirrors dbconfig.querier for the same reason: lets tests inject an
// in-memory database without bringing the migration system along.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store persists Connections and (encrypted) Tokens. Tokens are kept
// in their own table so a `SELECT * FROM workspace_connections` for
// debugging never returns ciphertext.
type Store struct {
	db     querier
	crypto *Crypto
	now    func() time.Time
}

func NewStore(db querier, crypto *Crypto) *Store {
	return &Store{db: db, crypto: crypto, now: time.Now}
}

// List returns every connection for a given user, ordered by service +
// display_name so the UI gets a stable grouping.
func (s *Store) List(ctx context.Context, userID string) ([]Connection, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, service, external_workspace_id,
		       display_name, email, avatar_url, connected_at, updated_at
		FROM workspace_connections
		WHERE user_id = ?
		ORDER BY service, display_name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, fmt.Errorf("workspace: list: %w", err)
	}
	defer rows.Close()

	var out []Connection
	for rows.Next() {
		c, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) Get(ctx context.Context, id string) (Connection, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, service, external_workspace_id,
		       display_name, email, avatar_url, connected_at, updated_at
		FROM workspace_connections WHERE id = ?`, id)
	c, err := scanConnection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Connection{}, ErrNotFound
	}
	return c, err
}

// Upsert is the OAuth-callback path: we have a UserID + Service +
// ExternalWorkspaceID from the Provider's Complete call; if a matching
// row exists we update it (re-auth) — otherwise we insert.
//
// The Token argument is encrypted and stored in workspace_tokens.
// Caller passes a *plaintext* Token; ciphertext never leaves Store.
func (s *Store) Upsert(ctx context.Context, c Connection, tok Token) (Connection, error) {
	if !supportedServices[c.Service] {
		return Connection{}, ErrInvalidService
	}
	if strings.TrimSpace(c.UserID) == "" {
		return Connection{}, fmt.Errorf("workspace: user_id required")
	}
	now := s.now().Unix()

	existing, err := s.findByIdentity(ctx, c.UserID, c.Service, c.ExternalWorkspaceID)
	switch {
	case err == nil:
		// Re-auth on an existing connection: keep the ID + connected_at.
		c.ID = existing.ID
		c.ConnectedAt = existing.ConnectedAt
	case errors.Is(err, ErrNotFound):
		c.ID = newID()
		c.ConnectedAt = now
	default:
		return Connection{}, err
	}
	c.UpdatedAt = now

	accessEnc, err := s.crypto.Encrypt(ctx, tok.AccessToken)
	if err != nil {
		return Connection{}, err
	}
	refreshEnc, err := s.crypto.Encrypt(ctx, tok.RefreshToken)
	if err != nil {
		return Connection{}, err
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO workspace_connections (
			id, user_id, service, external_workspace_id,
			display_name, email, avatar_url, connected_at, updated_at
		) VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			display_name = excluded.display_name,
			email        = excluded.email,
			avatar_url   = excluded.avatar_url,
			updated_at   = excluded.updated_at`,
		c.ID, c.UserID, string(c.Service), c.ExternalWorkspaceID,
		c.DisplayName, c.Email, c.AvatarURL, c.ConnectedAt, c.UpdatedAt,
	); err != nil {
		// Map the UNIQUE-constraint error onto a typed sentinel so the
		// UI can render "already connected" instead of a raw SQL error.
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return Connection{}, ErrDuplicate
		}
		return Connection{}, fmt.Errorf("workspace: upsert connection: %w", err)
	}

	expires := int64(0)
	if !tok.ExpiresAt.IsZero() {
		expires = tok.ExpiresAt.Unix()
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO workspace_tokens (
			connection_id, access_token_enc, refresh_token_enc,
			token_type, expires_at, scope, updated_at
		) VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(connection_id) DO UPDATE SET
			access_token_enc  = excluded.access_token_enc,
			refresh_token_enc = excluded.refresh_token_enc,
			token_type        = excluded.token_type,
			expires_at        = excluded.expires_at,
			scope             = excluded.scope,
			updated_at        = excluded.updated_at`,
		c.ID, accessEnc, refreshEnc, tok.TokenType, expires, tok.Scope, now,
	); err != nil {
		return Connection{}, fmt.Errorf("workspace: upsert token: %w", err)
	}
	return c, nil
}

// GetToken fetches and decrypts the token for one connection. The
// caller is responsible for never logging the returned value.
func (s *Store) GetToken(ctx context.Context, connectionID string) (Token, error) {
	var (
		accessEnc, refreshEnc, tokenType, scope string
		expiresAt                               int64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT access_token_enc, refresh_token_enc, token_type, expires_at, scope
		FROM workspace_tokens WHERE connection_id = ?`, connectionID,
	).Scan(&accessEnc, &refreshEnc, &tokenType, &expiresAt, &scope)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, ErrNotFound
	}
	if err != nil {
		return Token{}, fmt.Errorf("workspace: get token: %w", err)
	}

	access, err := s.crypto.Decrypt(ctx, accessEnc)
	if err != nil {
		return Token{}, err
	}
	refresh, err := s.crypto.Decrypt(ctx, refreshEnc)
	if err != nil {
		return Token{}, err
	}
	tok := Token{
		ConnectionID: connectionID,
		AccessToken:  access,
		RefreshToken: refresh,
		TokenType:    tokenType,
		Scope:        scope,
	}
	if expiresAt > 0 {
		tok.ExpiresAt = time.Unix(expiresAt, 0)
	}
	return tok, nil
}

// Delete removes a connection and its token (CASCADE). Missing IDs
// return ErrNotFound so the UI can distinguish "already gone" from
// "succeeded".
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM workspace_connections WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("workspace: delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("workspace: delete rowcount: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// findByIdentity is the per-(user, service, external_workspace_id)
// lookup used by Upsert to detect re-auth vs new-connect.
func (s *Store) findByIdentity(ctx context.Context, userID string, svc Service, externalID string) (Connection, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, service, external_workspace_id,
		       display_name, email, avatar_url, connected_at, updated_at
		FROM workspace_connections
		WHERE user_id = ? AND service = ? AND external_workspace_id = ?`,
		userID, string(svc), externalID,
	)
	c, err := scanConnection(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Connection{}, ErrNotFound
	}
	return c, err
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanConnection(sc scanner) (Connection, error) {
	var (
		c   Connection
		svc string
	)
	err := sc.Scan(
		&c.ID, &c.UserID, &svc, &c.ExternalWorkspaceID,
		&c.DisplayName, &c.Email, &c.AvatarURL, &c.ConnectedAt, &c.UpdatedAt,
	)
	if err != nil {
		return Connection{}, err
	}
	c.Service = Service(svc)
	return c, nil
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
