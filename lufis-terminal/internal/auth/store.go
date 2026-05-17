// Package auth persists per-service credentials for the terminal.
//
// Secrets (api_key, token) live in the OS keychain via zalando/go-keyring.
// The JSON file on disk holds only non-secret metadata (service, label,
// expires_at, connected) so the on-disk artefact never contains plaintext
// credentials, even if its 0600 permissions are bypassed (backup, sync, etc.).
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zalando/go-keyring"
)

// keyringService is the namespace used in the OS keychain. All entries belong
// to this service; per-credential identity is the user field (service name).
const keyringService = "argus-terminal"

type Credential struct {
	Service   string    `json:"service"`
	Label     string    `json:"label"`
	APIKey    string    `json:"api_key,omitempty"`
	Token     string    `json:"token,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	Connected bool      `json:"connected"`
}

// storedMeta is the on-disk record. Note the absence of APIKey/Token —
// those live in the keychain and are loaded lazily on Get.
type storedMeta struct {
	Service   string    `json:"service"`
	Label     string    `json:"label"`
	ExpiresAt time.Time `json:"expires_at"`
	Connected bool      `json:"connected"`
}

type Store struct {
	mu     sync.RWMutex
	path   string
	creds  map[string]*storedMeta
	loaded bool
}

func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home dir: %w", err)
		}
		dataDir = filepath.Join(home, ".config", "argus-terminal")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	return &Store{
		path:  filepath.Join(dataDir, "auth.json"),
		creds: make(map[string]*storedMeta),
	}, nil
}

func (s *Store) Set(service, apiKey string) error {
	if service == "" {
		return errors.New("service required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLoaded()

	if err := keyring.Set(keyringService, service, apiKey); err != nil {
		return fmt.Errorf("keyring set: %w", err)
	}

	if s.creds[service] == nil {
		s.creds[service] = &storedMeta{Service: service}
	}
	s.creds[service].Connected = true
	return s.save()
}

func (s *Store) Get(service string) (*Credential, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureLoaded()

	meta, ok := s.creds[service]
	if !ok || !meta.Connected {
		return nil, false
	}

	apiKey, err := keyring.Get(keyringService, service)
	if err != nil {
		// Treat a missing keychain entry as disconnected — the on-disk
		// metadata is stale (user revoked from the keychain GUI, OS reset).
		return nil, false
	}

	return &Credential{
		Service:   meta.Service,
		Label:     meta.Label,
		APIKey:    apiKey,
		ExpiresAt: meta.ExpiresAt,
		Connected: meta.Connected,
	}, true
}

func (s *Store) Delete(service string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLoaded()

	// Best-effort keychain delete. ErrNotFound is fine — we're tearing down.
	if err := keyring.Delete(keyringService, service); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("keyring delete: %w", err)
	}
	delete(s.creds, service)
	return s.save()
}

func (s *Store) List() []Credential {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureLoaded()

	result := make([]Credential, 0, len(s.creds))
	for _, m := range s.creds {
		// List intentionally omits secret material — callers wanting the
		// secret should call Get(service) explicitly.
		result = append(result, Credential{
			Service:   m.Service,
			Label:     m.Label,
			ExpiresAt: m.ExpiresAt,
			Connected: m.Connected,
		})
	}
	return result
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return s.save()
	}
	return nil
}

func (s *Store) ensureLoaded() {
	if s.loaded {
		return
	}
	s.loaded = true
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var metas []storedMeta
	if err := json.Unmarshal(data, &metas); err != nil {
		return
	}
	for i := range metas {
		s.creds[metas[i].Service] = &metas[i]
	}
}

func (s *Store) save() error {
	list := make([]storedMeta, 0, len(s.creds))
	for _, m := range s.creds {
		list = append(list, *m)
	}
	data, err := json.Marshal(list)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return os.Rename(tmp, s.path)
}
