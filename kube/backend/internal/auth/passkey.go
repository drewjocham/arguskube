// Passkey (WebAuthn) sign-in. Discoverable credentials only — the
// browser surfaces a passkey picker without the user typing a username,
// which is the modern "Sign in with a passkey" UX. We never store
// shared secrets; each credential is a public key bound to an
// authenticator the user already possesses.
//
// The PasskeyManager wraps a *webauthn.WebAuthn instance and a small
// persistence interface (PasskeyStore) so the same code path works for
// both the production sqlite store and unit tests with an in-memory
// implementation.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	wa "github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

// Sentinel errors so the HTTP layer can map cleanly to status codes.
var (
	ErrPasskeySessionInvalid = errors.New("auth: passkey ceremony session invalid or expired")
	ErrPasskeyNotFound       = errors.New("auth: passkey credential not found")
	ErrPasskeyDisabled       = errors.New("auth: passkey sign-in is not enabled")
)

// PasskeyCeremonyTTL bounds how long a registration/login ceremony can
// stay open. WebAuthn ceremonies are typically completed in seconds;
// five minutes is plenty of slack for the user to tap their key while
// keeping replay-window pressure low.
const PasskeyCeremonyTTL = 5 * time.Minute

// StoredCredential is the persistence-facing shape — we keep what the
// webauthn library needs to verify subsequent assertions, plus a few
// fields for the management UI (name, last_used_at).
type StoredCredential struct {
	ID           int64
	UserID       string
	CredentialID []byte
	PublicKey    []byte
	SignCount    uint32
	Transports   []string
	AAGUID       []byte
	Name         string
	CreatedAt    time.Time
	LastUsedAt   time.Time
}

// passkeyUser adapts auth.User + its credentials to the webauthn.User
// interface. WebAuthnID is a stable opaque byte sequence; we use the
// UUID-as-bytes representation of the user ID so it's <=64 bytes per
// the spec and never collides across accounts.
type passkeyUser struct {
	id          string
	email       string
	name        string
	credentials []wa.Credential
}

func (u *passkeyUser) WebAuthnID() []byte                       { return []byte(u.id) }
func (u *passkeyUser) WebAuthnName() string                     { return u.email }
func (u *passkeyUser) WebAuthnDisplayName() string {
	if u.name != "" {
		return u.name
	}
	return u.email
}
func (u *passkeyUser) WebAuthnCredentials() []wa.Credential { return u.credentials }

// PasskeyStore is what the manager needs from persistence. Implemented
// by *Store in db_passkey.go; tests use an in-memory fake.
type PasskeyStore interface {
	UserByID(id string) (*User, error)
	UserByCredentialID(credID []byte) (*User, error)
	ListCredentialsForUser(userID string) ([]StoredCredential, error)
	InsertCredential(c StoredCredential) error
	UpdateCredentialUsage(credID []byte, signCount uint32, when time.Time) error
	DeleteCredential(userID string, id int64) error

	// Ceremony state — opaque session JSON keyed by a random state
	// token, optionally bound to a user (registration) or unbound
	// (discoverable login).
	SaveCeremony(state, userID string, sessionData []byte, expiresAt time.Time) error
	LoadCeremony(state string) (userID string, sessionData []byte, err error)
	DeleteCeremony(state string) error
	PurgeExpiredCeremonies() error
}

// PasskeyManager owns the WebAuthn relying-party state and brokers
// access to the store. Safe for concurrent use — the underlying
// webauthn.WebAuthn is a configuration value and the store implements
// its own locking.
type PasskeyManager struct {
	wa    *wa.WebAuthn
	store PasskeyStore
}

// NewPasskeyManager constructs the manager from the four RP knobs an
// operator sets via env vars. Returns an error early if the config is
// internally inconsistent (e.g. an origin that doesn't include scheme)
// — better to fail at startup than at the first registration attempt.
func NewPasskeyManager(rpID, rpName, rpOrigin string, store PasskeyStore) (*PasskeyManager, error) {
	if rpID == "" {
		return nil, fmt.Errorf("passkey: RPID is required")
	}
	if rpOrigin == "" {
		return nil, fmt.Errorf("passkey: RPOrigin is required")
	}
	if rpName == "" {
		rpName = "Argus"
	}
	w, err := wa.New(&wa.Config{
		RPID:          rpID,
		RPDisplayName: rpName,
		RPOrigins:     []string{rpOrigin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationPreferred,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("passkey: configure RP: %w", err)
	}
	return &PasskeyManager{wa: w, store: store}, nil
}

// PurgeExpired hooks into the existing sessions janitor.
func (m *PasskeyManager) PurgeExpired() {
	_ = m.store.PurgeExpiredCeremonies()
}

// BeginRegistration starts the credential-creation ceremony for an
// already-authenticated user. The returned state token is what the
// client echoes back to FinishRegistration so we can look up the
// ceremony's persisted session_data.
func (m *PasskeyManager) BeginRegistration(user *User) (*protocol.CredentialCreation, string, error) {
	if user == nil {
		return nil, "", ErrUserNotFound
	}
	creds, err := m.userCredentials(user.ID)
	if err != nil {
		return nil, "", err
	}
	wu := &passkeyUser{id: user.ID, email: user.Email, name: user.Name, credentials: creds}
	options, session, err := m.wa.BeginRegistration(wu)
	if err != nil {
		return nil, "", fmt.Errorf("passkey: begin registration: %w", err)
	}
	state, err := randomToken(16)
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return nil, "", fmt.Errorf("passkey: marshal session: %w", err)
	}
	if err := m.store.SaveCeremony(state, user.ID, raw, time.Now().Add(PasskeyCeremonyTTL)); err != nil {
		return nil, "", err
	}
	return options, state, nil
}

// FinishRegistration validates the authenticator's attestation and
// persists the new credential. `name` is a user-facing label
// ("MacBook Touch ID", "YubiKey 5C") shown in the management UI.
func (m *PasskeyManager) FinishRegistration(user *User, state, name string, r *http.Request) (*StoredCredential, error) {
	if user == nil {
		return nil, ErrUserNotFound
	}
	uid, raw, err := m.store.LoadCeremony(state)
	if err != nil {
		return nil, ErrPasskeySessionInvalid
	}
	if uid != user.ID {
		return nil, ErrPasskeySessionInvalid
	}
	var session wa.SessionData
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, ErrPasskeySessionInvalid
	}
	defer func() { _ = m.store.DeleteCeremony(state) }()

	creds, err := m.userCredentials(user.ID)
	if err != nil {
		return nil, err
	}
	wu := &passkeyUser{id: user.ID, email: user.Email, name: user.Name, credentials: creds}
	cred, err := m.wa.FinishRegistration(wu, session, r)
	if err != nil {
		return nil, fmt.Errorf("passkey: finish registration: %w", err)
	}
	stored := StoredCredential{
		UserID:       user.ID,
		CredentialID: cred.ID,
		PublicKey:    cred.PublicKey,
		SignCount:    cred.Authenticator.SignCount,
		Transports:   transportsAsStrings(cred.Transport),
		AAGUID:       cred.Authenticator.AAGUID,
		Name:         deriveName(name),
		CreatedAt:    time.Now(),
	}
	if err := m.store.InsertCredential(stored); err != nil {
		return nil, err
	}
	return &stored, nil
}

// BeginLogin starts a discoverable (usernameless) login ceremony. The
// browser will surface the passkey picker; no user-identifying data is
// sent until the ceremony completes.
func (m *PasskeyManager) BeginLogin() (*protocol.CredentialAssertion, string, error) {
	options, session, err := m.wa.BeginDiscoverableLogin()
	if err != nil {
		return nil, "", fmt.Errorf("passkey: begin login: %w", err)
	}
	state, err := randomToken(16)
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(session)
	if err != nil {
		return nil, "", fmt.Errorf("passkey: marshal session: %w", err)
	}
	if err := m.store.SaveCeremony(state, "", raw, time.Now().Add(PasskeyCeremonyTTL)); err != nil {
		return nil, "", err
	}
	return options, state, nil
}

// FinishLogin validates the assertion. We look up the user by the raw
// credential ID returned by the authenticator — this is the discoverable
// flow's contract — and refuse any credential we don't know.
func (m *PasskeyManager) FinishLogin(state string, r *http.Request) (*User, *StoredCredential, error) {
	_, raw, err := m.store.LoadCeremony(state)
	if err != nil {
		return nil, nil, ErrPasskeySessionInvalid
	}
	var session wa.SessionData
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, nil, ErrPasskeySessionInvalid
	}
	defer func() { _ = m.store.DeleteCeremony(state) }()

	var foundUser *User
	handler := func(rawID, userHandle []byte) (wa.User, error) {
		u, err := m.store.UserByCredentialID(rawID)
		if err != nil {
			return nil, err
		}
		foundUser = u
		creds, err := m.userCredentials(u.ID)
		if err != nil {
			return nil, err
		}
		return &passkeyUser{id: u.ID, email: u.Email, name: u.Name, credentials: creds}, nil
	}
	cred, err := m.wa.FinishDiscoverableLogin(handler, session, r)
	if err != nil {
		return nil, nil, fmt.Errorf("passkey: finish login: %w", err)
	}
	if foundUser == nil {
		return nil, nil, ErrPasskeyNotFound
	}
	now := time.Now()
	_ = m.store.UpdateCredentialUsage(cred.ID, cred.Authenticator.SignCount, now)
	stored := StoredCredential{
		UserID:       foundUser.ID,
		CredentialID: cred.ID,
		SignCount:    cred.Authenticator.SignCount,
		LastUsedAt:   now,
	}
	return foundUser, &stored, nil
}

// ListCredentials returns the user-facing view of stored credentials
// for the management UI. Public key / sign count are intentionally
// omitted — the browser doesn't need them and they add no value.
func (m *PasskeyManager) ListCredentials(userID string) ([]StoredCredential, error) {
	return m.store.ListCredentialsForUser(userID)
}

// RevokeCredential deletes one credential row. We scope by user_id so a
// malicious caller can't pass another user's id.
func (m *PasskeyManager) RevokeCredential(userID string, id int64) error {
	return m.store.DeleteCredential(userID, id)
}

// userCredentials fetches the user's existing credentials and converts
// them to the webauthn.Credential shape the library expects. We pass
// the full list during BeginRegistration so the same authenticator
// can't be registered twice, and during BeginLogin (when not
// discoverable) to scope the allowCredentials list.
func (m *PasskeyManager) userCredentials(userID string) ([]wa.Credential, error) {
	rows, err := m.store.ListCredentialsForUser(userID)
	if err != nil {
		return nil, err
	}
	out := make([]wa.Credential, 0, len(rows))
	for _, c := range rows {
		tr := make([]protocol.AuthenticatorTransport, 0, len(c.Transports))
		for _, s := range c.Transports {
			tr = append(tr, protocol.AuthenticatorTransport(s))
		}
		out = append(out, wa.Credential{
			ID:        c.CredentialID,
			PublicKey: c.PublicKey,
			Transport: tr,
			Authenticator: wa.Authenticator{
				AAGUID:    c.AAGUID,
				SignCount: c.SignCount,
			},
		})
	}
	return out, nil
}

func transportsAsStrings(in []protocol.AuthenticatorTransport) []string {
	out := make([]string, 0, len(in))
	for _, t := range in {
		out = append(out, string(t))
	}
	return out
}

// deriveName supplies a friendly fallback so the UI never shows a row
// with an empty label. We don't introspect the AAGUID against the FIDO
// metadata service — keeping the dep surface small.
func deriveName(provided string) string {
	if provided != "" {
		return provided
	}
	return "Passkey " + uuid.New().String()[:8]
}
