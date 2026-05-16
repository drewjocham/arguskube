package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// PasskeyStoreSQL is the production *sql.DB-backed implementation of
// PasskeyStore. It reuses the Store's connection — both subsystems
// share the same single-writer SQLite handle so there's no need for a
// separate pool.
type PasskeyStoreSQL struct {
	s *Store
}

// PasskeyStore returns a sqlite-backed PasskeyStore wired against the
// same DB as the auth Store. Lives on Store so the wiring (DB handle +
// logger) doesn't have to be re-plumbed at construction sites.
func (s *Store) PasskeyStore() PasskeyStore {
	return &PasskeyStoreSQL{s: s}
}

func (p *PasskeyStoreSQL) UserByID(id string) (*User, error) {
	row := p.s.db.QueryRow(`SELECT id, email, name, provider, created_at, last_login_at
		FROM users WHERE id = ?`, id)
	var u User
	var prov string
	var createdUnix, lastLoginUnix int64
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &prov, &createdUnix, &lastLoginUnix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth: passkey UserByID: %w", err)
	}
	u.Provider = Provider(prov)
	u.CreatedAt = time.Unix(createdUnix, 0)
	if lastLoginUnix > 0 {
		u.LastLoginAt = time.Unix(lastLoginUnix, 0)
	}
	return &u, nil
}

func (p *PasskeyStoreSQL) UserByCredentialID(credID []byte) (*User, error) {
	row := p.s.db.QueryRow(`SELECT u.id, u.email, u.name, u.provider, u.created_at, u.last_login_at
		FROM users u JOIN passkey_credentials pc ON pc.user_id = u.id
		WHERE pc.credential_id = ?`, credID)
	var u User
	var prov string
	var createdUnix, lastLoginUnix int64
	if err := row.Scan(&u.ID, &u.Email, &u.Name, &prov, &createdUnix, &lastLoginUnix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPasskeyNotFound
		}
		return nil, fmt.Errorf("auth: passkey UserByCredentialID: %w", err)
	}
	u.Provider = Provider(prov)
	u.CreatedAt = time.Unix(createdUnix, 0)
	if lastLoginUnix > 0 {
		u.LastLoginAt = time.Unix(lastLoginUnix, 0)
	}
	return &u, nil
}

func (p *PasskeyStoreSQL) ListCredentialsForUser(userID string) ([]StoredCredential, error) {
	rows, err := p.s.db.Query(`SELECT id, user_id, credential_id, public_key, sign_count,
		COALESCE(transports, ''), COALESCE(aaguid, X''), COALESCE(name, ''), created_at, last_used_at
		FROM passkey_credentials WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("auth: passkey list: %w", err)
	}
	defer rows.Close()
	out := []StoredCredential{}
	for rows.Next() {
		var c StoredCredential
		var transports string
		var createdUnix, lastUsedUnix int64
		if err := rows.Scan(&c.ID, &c.UserID, &c.CredentialID, &c.PublicKey,
			&c.SignCount, &transports, &c.AAGUID, &c.Name, &createdUnix, &lastUsedUnix); err != nil {
			return nil, fmt.Errorf("auth: passkey scan: %w", err)
		}
		if transports != "" {
			c.Transports = strings.Split(transports, ",")
		}
		c.CreatedAt = time.Unix(createdUnix, 0)
		if lastUsedUnix > 0 {
			c.LastUsedAt = time.Unix(lastUsedUnix, 0)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (p *PasskeyStoreSQL) InsertCredential(c StoredCredential) error {
	_, err := p.s.db.Exec(`INSERT INTO passkey_credentials (
		user_id, credential_id, public_key, sign_count, transports, aaguid, name, created_at, last_used_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		c.UserID, c.CredentialID, c.PublicKey, c.SignCount,
		strings.Join(c.Transports, ","), c.AAGUID, c.Name, c.CreatedAt.Unix())
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("auth: credential already registered")
		}
		return fmt.Errorf("auth: passkey insert: %w", err)
	}
	return nil
}

func (p *PasskeyStoreSQL) UpdateCredentialUsage(credID []byte, signCount uint32, when time.Time) error {
	_, err := p.s.db.Exec(`UPDATE passkey_credentials SET sign_count = ?, last_used_at = ?
		WHERE credential_id = ?`, signCount, when.Unix(), credID)
	return err
}

func (p *PasskeyStoreSQL) DeleteCredential(userID string, id int64) error {
	res, err := p.s.db.Exec(`DELETE FROM passkey_credentials WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("auth: passkey delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrPasskeyNotFound
	}
	return nil
}

func (p *PasskeyStoreSQL) SaveCeremony(state, userID string, sessionData []byte, expiresAt time.Time) error {
	_, err := p.s.db.Exec(`INSERT INTO passkey_sessions (state, user_id, session_data, expires_at)
		VALUES (?, ?, ?, ?)`, state, userID, sessionData, expiresAt.Unix())
	if err != nil {
		return fmt.Errorf("auth: passkey save ceremony: %w", err)
	}
	return nil
}

func (p *PasskeyStoreSQL) LoadCeremony(state string) (string, []byte, error) {
	row := p.s.db.QueryRow(`SELECT user_id, session_data, expires_at
		FROM passkey_sessions WHERE state = ?`, state)
	var userID string
	var data []byte
	var expUnix int64
	if err := row.Scan(&userID, &data, &expUnix); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, ErrPasskeySessionInvalid
		}
		return "", nil, fmt.Errorf("auth: passkey load ceremony: %w", err)
	}
	if time.Now().Unix() >= expUnix {
		_, _ = p.s.db.Exec(`DELETE FROM passkey_sessions WHERE state = ?`, state)
		return "", nil, ErrPasskeySessionInvalid
	}
	return userID, data, nil
}

func (p *PasskeyStoreSQL) DeleteCeremony(state string) error {
	_, err := p.s.db.Exec(`DELETE FROM passkey_sessions WHERE state = ?`, state)
	return err
}

func (p *PasskeyStoreSQL) PurgeExpiredCeremonies() error {
	_, err := p.s.db.Exec(`DELETE FROM passkey_sessions WHERE expires_at < ?`, time.Now().Unix())
	return err
}
