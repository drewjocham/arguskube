package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SessionTTL is how long a session token stays valid before the user
// has to log in again. 14 days mirrors major SaaS defaults — long
// enough that typing a password becomes rare, short enough that a
// stolen laptop's tokens expire while the user is on vacation.
const SessionTTL = 14 * 24 * time.Hour

// Store is the persistence layer for users + sessions + OAuth state.
// Wraps *sql.DB and is safe for concurrent use because the underlying
// SQLite connection is single-writer.
type Store struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewStore(db *sql.DB, logger *slog.Logger) *Store {
	return &Store{db: db, logger: logger}
}

// CreateLocalUser registers a new email/password account. Returns
// ErrEmailTaken if the address is already in use under any provider —
// we never silently merge providers, since that would let an attacker
// who controls an email's OAuth provider take over a local account
// (and vice-versa).
func (s *Store) CreateLocalUser(email, name, password string) (*User, error) {
	email = normalizeEmail(email)
	if !validEmail(email) {
		return nil, ErrInvalidEmail
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &User{
		ID:        uuid.New().String(),
		Email:     email,
		Name:      name,
		Provider:  ProviderLocal,
		CreatedAt: time.Now(),
	}
	_, err = s.db.Exec(`INSERT INTO users (
		id, email, name, password_hash, provider, provider_subject, created_at, last_login_at
	) VALUES (?, ?, ?, ?, ?, '', ?, 0)`,
		u.ID, u.Email, u.Name, hash, string(u.Provider), u.CreatedAt.Unix(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, fmt.Errorf("auth: insert user: %w", err)
	}
	return u, nil
}

// AuthenticateLocal verifies email/password and returns the user.
// Returns ErrInvalidCredentials for any failure path so we don't
// reveal whether the email exists (timing attacks are not in scope —
// bcrypt dominates the response time anyway).
func (s *Store) AuthenticateLocal(email, password string) (*User, error) {
	email = normalizeEmail(email)
	row := s.db.QueryRow(`SELECT id, email, name, password_hash, provider, created_at, last_login_at
		FROM users WHERE email = ? AND provider = ?`, email, string(ProviderLocal))

	var u User
	var hash string
	var provider string
	var createdUnix, lastLoginUnix int64
	err := row.Scan(&u.ID, &u.Email, &u.Name, &hash, &provider, &createdUnix, &lastLoginUnix)
	if errors.Is(err, sql.ErrNoRows) {
		// Run a dummy bcrypt to make timing roughly equal between
		// "no such user" and "wrong password".
		_ = verifyPassword("$2a$12$abcdefghijklmnopqrstuv.placeholderplaceholderplaceho", password)
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, fmt.Errorf("auth: lookup user: %w", err)
	}
	if !verifyPassword(hash, password) {
		return nil, ErrInvalidCredentials
	}
	u.Provider = Provider(provider)
	u.CreatedAt = time.Unix(createdUnix, 0)
	if lastLoginUnix > 0 {
		u.LastLoginAt = time.Unix(lastLoginUnix, 0)
	}
	return &u, nil
}

// UpsertOAuthUser is called from OAuth callbacks: if a user with this
// (provider, subject) tuple exists, return it. Otherwise create a new
// account. We do NOT match on email — different providers may legit
// share an email (e.g. work Google + personal Auth0), and matching on
// email creates a takeover hole if a provider lets someone change
// their email to one already used by a local account.
func (s *Store) UpsertOAuthUser(provider Provider, subject, email, name string) (*User, error) {
	if !provider.Valid() || provider == ProviderLocal {
		return nil, fmt.Errorf("auth: invalid provider %q", provider)
	}
	if subject == "" {
		return nil, fmt.Errorf("auth: empty OAuth subject")
	}
	email = normalizeEmail(email)

	row := s.db.QueryRow(`SELECT id, email, name, provider, created_at, last_login_at
		FROM users WHERE provider = ? AND provider_subject = ?`, string(provider), subject)
	var u User
	var prov string
	var createdUnix, lastLoginUnix int64
	err := row.Scan(&u.ID, &u.Email, &u.Name, &prov, &createdUnix, &lastLoginUnix)
	if err == nil {
		u.Provider = Provider(prov)
		u.CreatedAt = time.Unix(createdUnix, 0)
		if lastLoginUnix > 0 {
			u.LastLoginAt = time.Unix(lastLoginUnix, 0)
		}
		// Refresh email/name in case the upstream changed them.
		if email != "" && (u.Email != email || u.Name != name) {
			_, _ = s.db.Exec(`UPDATE users SET email = ?, name = ? WHERE id = ?`, email, name, u.ID)
			u.Email = email
			u.Name = name
		}
		return &u, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("auth: lookup oauth user: %w", err)
	}

	// New account.
	u = User{
		ID:              uuid.New().String(),
		Email:           email,
		Name:            name,
		Provider:        provider,
		ProviderSubject: subject,
		CreatedAt:       time.Now(),
	}
	_, err = s.db.Exec(`INSERT INTO users (
		id, email, name, password_hash, provider, provider_subject, created_at, last_login_at
	) VALUES (?, ?, ?, '', ?, ?, ?, 0)`,
		u.ID, u.Email, u.Name, string(u.Provider), u.ProviderSubject, u.CreatedAt.Unix(),
	)
	if err != nil {
		if isUniqueViolation(err) {
			// Race: another request created the same (provider,subject).
			// Re-read.
			return s.UpsertOAuthUser(provider, subject, email, name)
		}
		return nil, fmt.Errorf("auth: insert oauth user: %w", err)
	}
	return &u, nil
}

// CreateSession mints a fresh opaque token and persists its hash. The
// caller-visible token is returned exactly once; we never store or
// log the raw token.
func (s *Store) CreateSession(userID string) (*Session, error) {
	tok, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	exp := now.Add(SessionTTL)
	if _, err := s.db.Exec(`INSERT INTO sessions (token_hash, user_id, created_at, expires_at)
		VALUES (?, ?, ?, ?)`, hashToken(tok), userID, now.Unix(), exp.Unix()); err != nil {
		return nil, fmt.Errorf("auth: insert session: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, now.Unix(), userID); err != nil {
		s.logger.Warn("auth: update last_login_at failed", slog.String("error", err.Error()))
	}
	return &Session{Token: tok, UserID: userID, ExpiresAt: exp}, nil
}

// ValidateSession looks up an opaque token and returns the owning
// user. Expired sessions are deleted and treated as invalid.
func (s *Store) ValidateSession(token string) (*User, error) {
	if token == "" {
		return nil, ErrSessionInvalid
	}
	h := hashToken(token)
	row := s.db.QueryRow(`SELECT u.id, u.email, u.name, u.provider, u.created_at, u.last_login_at, sx.expires_at
		FROM sessions sx JOIN users u ON u.id = sx.user_id
		WHERE sx.token_hash = ?`, h)
	var u User
	var prov string
	var createdUnix, lastLoginUnix, expUnix int64
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &prov, &createdUnix, &lastLoginUnix, &expUnix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionInvalid
		}
		return nil, fmt.Errorf("auth: lookup session: %w", err)
	}
	if time.Now().Unix() >= expUnix {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, h)
		return nil, ErrSessionInvalid
	}
	u.Provider = Provider(prov)
	u.CreatedAt = time.Unix(createdUnix, 0)
	if lastLoginUnix > 0 {
		u.LastLoginAt = time.Unix(lastLoginUnix, 0)
	}
	return &u, nil
}

// RevokeSession deletes the row, idempotent.
func (s *Store) RevokeSession(token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(token))
	return err
}

// PurgeExpired runs a best-effort cleanup of stale sessions and pending OAuth states.
func (s *Store) PurgeExpired() {
	now := time.Now().Unix()
	if _, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now); err != nil {
		s.logger.Warn("auth: purge sessions failed", slog.String("error", err.Error()))
	}
	cutoff := now - int64(15*time.Minute/time.Second)
	if _, err := s.db.Exec(`DELETE FROM oauth_pending WHERE created_at < ?`, cutoff); err != nil {
		s.logger.Warn("auth: purge oauth_pending failed", slog.String("error", err.Error()))
	}
}

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

func randomToken(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// isUniqueViolation pattern-matches the modernc/sqlite UNIQUE error.
// Driver doesn't expose typed error codes for the constraint kind,
// so we fall back to message inspection.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "constraint failed: UNIQUE")
}
