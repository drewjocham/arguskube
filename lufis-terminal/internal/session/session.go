package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Pane struct {
	ID         string `json:"id"`
	Command    string `json:"command,omitempty"`
	CWD        string `json:"cwd,omitempty"`
	Rows       uint16 `json:"rows"`
	Cols       uint16 `json:"cols"`
	SplitDir   string `json:"split_dir,omitempty"`
	SplitSize  int    `json:"split_size,omitempty"`
	OpenCodeID string `json:"opencode_id,omitempty"`
}

type Tab struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Panes  []Pane `json:"panes"`
	Active int    `json:"active"`
}

type State struct {
	Tabs    []Tab     `json:"tabs"`
	Active  int       `json:"active"`
	SavedAt time.Time `json:"saved_at"`
	Version string    `json:"version"`
}

type Store struct {
	mu   sync.Mutex
	path string
}

func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home: %w", err)
		}
		dataDir = filepath.Join(home, ".config", "argus-terminal")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	return &Store{path: filepath.Join(dataDir, "session.json")}, nil
}

func (s *Store) Save(state State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state.SavedAt = time.Now()
	state.Version = "1"
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Load() (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &state, nil
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Remove(s.path)
}

func (s *Store) Exists() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := os.Stat(s.path)
	return err == nil
}
