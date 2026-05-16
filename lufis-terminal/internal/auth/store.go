package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Credential struct {
	Service   string    `json:"service"`
	Label     string    `json:"label"`
	APIKey    string    `json:"api_key,omitempty"`
	Token     string    `json:"token,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	Connected bool      `json:"connected"`
}

type Store struct {
	mu     sync.RWMutex
	path   string
	creds  map[string]*Credential
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
		creds: make(map[string]*Credential),
	}, nil
}

func (s *Store) Set(service, apiKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLoaded()
	if s.creds[service] == nil {
		s.creds[service] = &Credential{Service: service}
	}
	s.creds[service].APIKey = apiKey
	s.creds[service].Connected = true
	return s.save()
}

func (s *Store) Get(service string) (*Credential, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureLoaded()
	cred, ok := s.creds[service]
	if !ok || !cred.Connected {
		return nil, false
	}
	return cred, true
}

func (s *Store) Delete(service string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLoaded()
	delete(s.creds, service)
	return s.save()
}

func (s *Store) List() []Credential {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureLoaded()
	result := make([]Credential, 0, len(s.creds))
	for _, c := range s.creds {
		result = append(result, *c)
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
	var creds []Credential
	if err := json.Unmarshal(data, &creds); err != nil {
		return
	}
	for i := range creds {
		s.creds[creds[i].Service] = &creds[i]
	}
}

func (s *Store) save() error {
	list := make([]Credential, 0, len(s.creds))
	for _, c := range s.creds {
		list = append(list, *c)
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
