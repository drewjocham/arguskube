package notes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Note struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id,omitempty"`
	Command   string    `json:"command,omitempty"`
	ExitCode  int       `json:"exit_code"`
	Timestamp time.Time `json:"timestamp"`
	Body      string    `json:"body"`
	Tags      []string  `json:"tags,omitempty"`
}

type Store struct {
	mu     sync.RWMutex
	path   string
	notes  map[string]*Note
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
		path:  filepath.Join(dataDir, "notes.json"),
		notes: make(map[string]*Note),
	}, nil
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
	var list []*Note
	if err := json.Unmarshal(data, &list); err != nil {
		return
	}
	for _, n := range list {
		s.notes[n.ID] = n
	}
}

func (s *Store) save() error {
	list := make([]*Note, 0, len(s.notes))
	for _, n := range s.notes {
		list = append(list, n)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Timestamp.Before(list[j].Timestamp)
	})
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Save(note Note) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLoaded()
	s.notes[note.ID] = &note
	return s.save()
}

func (s *Store) Get(id string) (*Note, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureLoaded()
	n, ok := s.notes[id]
	if !ok {
		return nil, fmt.Errorf("note %s: %w", id, os.ErrNotExist)
	}
	return n, nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLoaded()
	delete(s.notes, id)
	return s.save()
}

func (s *Store) List(filter NoteFilter) []Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureLoaded()

	result := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		if filter.Match(n) {
			result = append(result, *n)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	return result
}

func (s *Store) Search(query string) []Note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.ensureLoaded()

	q := strings.ToLower(query)
	result := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		if strings.Contains(strings.ToLower(n.Body), q) ||
			strings.Contains(strings.ToLower(n.Command), q) {
			result = append(result, *n)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
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

type NoteFilter struct {
	SessionID string
	Tag       string
	Since     time.Time
	Limit     int
}

func (f NoteFilter) Match(n *Note) bool {
	if f.SessionID != "" && n.SessionID != f.SessionID {
		return false
	}
	if f.Tag != "" {
		found := false
		for _, t := range n.Tags {
			if t == f.Tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if !f.Since.IsZero() && n.Timestamp.Before(f.Since) {
		return false
	}
	return true
}
