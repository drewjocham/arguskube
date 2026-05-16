package auth

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// inMemPasskeyStore is a minimal PasskeyStore for unit tests. It
// exercises the same surface area the SQLite implementation has to
// satisfy, without dragging a real DB in for what is fundamentally a
// crypto-and-control-flow check.
type inMemPasskeyStore struct {
	mu          sync.Mutex
	users       map[string]*User
	credentials map[int64]StoredCredential
	byCredID    map[string]int64
	ceremonies  map[string]ceremonyRow
	nextID      int64
}

type ceremonyRow struct {
	userID    string
	data      []byte
	expiresAt time.Time
}

func newMemStore() *inMemPasskeyStore {
	return &inMemPasskeyStore{
		users:       map[string]*User{},
		credentials: map[int64]StoredCredential{},
		byCredID:    map[string]int64{},
		ceremonies:  map[string]ceremonyRow{},
	}
}

func (s *inMemPasskeyStore) UserByID(id string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func (s *inMemPasskeyStore) UserByCredentialID(credID []byte) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byCredID[string(credID)]
	if !ok {
		return nil, ErrPasskeyNotFound
	}
	c := s.credentials[id]
	return s.users[c.UserID], nil
}

func (s *inMemPasskeyStore) ListCredentialsForUser(userID string) ([]StoredCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []StoredCredential{}
	for _, c := range s.credentials {
		if c.UserID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *inMemPasskeyStore) InsertCredential(c StoredCredential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, dup := s.byCredID[string(c.CredentialID)]; dup {
		return errors.New("dup credential")
	}
	s.nextID++
	c.ID = s.nextID
	s.credentials[c.ID] = c
	s.byCredID[string(c.CredentialID)] = c.ID
	return nil
}

func (s *inMemPasskeyStore) UpdateCredentialUsage(credID []byte, signCount uint32, when time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.byCredID[string(credID)]
	if !ok {
		return ErrPasskeyNotFound
	}
	c := s.credentials[id]
	c.SignCount = signCount
	c.LastUsedAt = when
	s.credentials[id] = c
	return nil
}

func (s *inMemPasskeyStore) DeleteCredential(userID string, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.credentials[id]
	if !ok || c.UserID != userID {
		return ErrPasskeyNotFound
	}
	delete(s.byCredID, string(c.CredentialID))
	delete(s.credentials, id)
	return nil
}

func (s *inMemPasskeyStore) SaveCeremony(state, userID string, data []byte, exp time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ceremonies[state] = ceremonyRow{userID: userID, data: data, expiresAt: exp}
	return nil
}

func (s *inMemPasskeyStore) LoadCeremony(state string) (string, []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.ceremonies[state]
	if !ok {
		return "", nil, ErrPasskeySessionInvalid
	}
	if time.Now().After(row.expiresAt) {
		delete(s.ceremonies, state)
		return "", nil, ErrPasskeySessionInvalid
	}
	return row.userID, row.data, nil
}

func (s *inMemPasskeyStore) DeleteCeremony(state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ceremonies, state)
	return nil
}

func (s *inMemPasskeyStore) PurgeExpiredCeremonies() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, v := range s.ceremonies {
		if now.After(v.expiresAt) {
			delete(s.ceremonies, k)
		}
	}
	return nil
}

func mgrForTest(t *testing.T) (*PasskeyManager, *inMemPasskeyStore) {
	t.Helper()
	store := newMemStore()
	mgr, err := NewPasskeyManager("localhost", "Argus Test", "http://localhost:8080", store)
	if err != nil {
		t.Fatalf("NewPasskeyManager: %v", err)
	}
	return mgr, store
}

func TestPasskeyManager_Config_RejectsEmptyRPID(t *testing.T) {
	if _, err := NewPasskeyManager("", "x", "http://localhost", newMemStore()); err == nil {
		t.Error("expected error when RPID is empty")
	}
}

func TestPasskeyManager_Config_RejectsEmptyOrigin(t *testing.T) {
	if _, err := NewPasskeyManager("localhost", "x", "", newMemStore()); err == nil {
		t.Error("expected error when RPOrigin is empty")
	}
}

func TestPasskeyManager_Config_DefaultsRPName(t *testing.T) {
	if _, err := NewPasskeyManager("localhost", "", "http://localhost:8080", newMemStore()); err != nil {
		t.Errorf("empty RPName should default, got: %v", err)
	}
}

func TestPasskeyManager_BeginRegistration_PersistsCeremony(t *testing.T) {
	mgr, store := mgrForTest(t)
	store.users["u1"] = &User{ID: "u1", Email: "u@example.com", Name: "U"}

	options, state, err := mgr.BeginRegistration(store.users["u1"])
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	if state == "" {
		t.Fatal("state empty")
	}
	if options == nil || options.Response.Challenge == nil {
		t.Fatal("options/Challenge missing")
	}
	if _, _, err := store.LoadCeremony(state); err != nil {
		t.Errorf("ceremony not persisted: %v", err)
	}
}

func TestPasskeyManager_BeginLogin_DiscoverableFlow(t *testing.T) {
	mgr, store := mgrForTest(t)
	options, state, err := mgr.BeginLogin()
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	if state == "" || options == nil {
		t.Fatal("missing state/options")
	}
	// Discoverable flow: AllowedCredentials must be empty (browser
	// picks from any available passkey).
	if len(options.Response.AllowedCredentials) != 0 {
		t.Errorf("discoverable login should have no allowedCredentials, got %d", len(options.Response.AllowedCredentials))
	}
	uid, _, err := store.LoadCeremony(state)
	if err != nil {
		t.Fatalf("ceremony not persisted: %v", err)
	}
	if uid != "" {
		t.Errorf("discoverable ceremony should have empty userID; got %q", uid)
	}
}

func TestPasskeyManager_BeginRegistration_NilUser(t *testing.T) {
	mgr, _ := mgrForTest(t)
	if _, _, err := mgr.BeginRegistration(nil); err == nil {
		t.Error("expected error for nil user")
	}
}

func TestPasskeyManager_FinishLogin_RejectsExpiredCeremony(t *testing.T) {
	mgr, store := mgrForTest(t)
	// Plant an expired ceremony directly so we don't have to time-skew.
	store.ceremonies["s1"] = ceremonyRow{
		userID:    "",
		data:      []byte(`{}`),
		expiresAt: time.Now().Add(-1 * time.Minute),
	}
	if _, _, err := mgr.FinishLogin("s1", nil); err == nil {
		t.Error("expected error for expired ceremony")
	}
}

func TestPasskeyManager_FinishLogin_RejectsUnknownState(t *testing.T) {
	mgr, _ := mgrForTest(t)
	if _, _, err := mgr.FinishLogin("nope", nil); err == nil {
		t.Error("expected error for unknown state")
	}
}

func TestPasskeyManager_ListAndRevoke_RoundTrip(t *testing.T) {
	mgr, store := mgrForTest(t)
	store.users["u1"] = &User{ID: "u1", Email: "u@example.com"}
	// Insert a credential directly via the store, then exercise the
	// manager's list/revoke flow.
	c := StoredCredential{
		UserID:       "u1",
		CredentialID: []byte("cred-bytes"),
		PublicKey:    []byte("pk"),
		Name:         "Test Key",
		CreatedAt:    time.Now(),
	}
	if err := store.InsertCredential(c); err != nil {
		t.Fatal(err)
	}
	list, err := mgr.ListCredentials("u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 credential; got %d", len(list))
	}
	if err := mgr.RevokeCredential("u1", list[0].ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	list, _ = mgr.ListCredentials("u1")
	if len(list) != 0 {
		t.Errorf("expected 0 after revoke; got %d", len(list))
	}
	// Revoking another user's credential is a 404, not a silent
	// delete. Re-insert and try with a wrong userID.
	if err := store.InsertCredential(c); err != nil {
		t.Fatal(err)
	}
	list, _ = mgr.ListCredentials("u1")
	if err := mgr.RevokeCredential("not-u1", list[0].ID); err == nil {
		t.Error("revoke with wrong userID should fail")
	}
}

func TestPasskeyManager_PurgeExpired_DeletesOldCeremonies(t *testing.T) {
	mgr, store := mgrForTest(t)
	store.ceremonies["old"] = ceremonyRow{expiresAt: time.Now().Add(-time.Hour)}
	store.ceremonies["fresh"] = ceremonyRow{expiresAt: time.Now().Add(time.Hour)}
	mgr.PurgeExpired()
	if _, ok := store.ceremonies["old"]; ok {
		t.Error("expired ceremony should be purged")
	}
	if _, ok := store.ceremonies["fresh"]; !ok {
		t.Error("fresh ceremony was purged in error")
	}
}

func TestDeriveName_FallbackWhenEmpty(t *testing.T) {
	if got := deriveName(""); got == "" {
		t.Error("deriveName should never return empty")
	}
	if got := deriveName("my key"); got != "my key" {
		t.Errorf("provided name should pass through; got %q", got)
	}
}
