package incidents

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Incident represents a tracked incident.
type Incident struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Severity    string    `json:"severity"` // critical, warning, info
	Status      string    `json:"status"`   // open, investigating, resolved
	Type        string    `json:"type"`     // alert, resolution, investigation, pattern
	Description string    `json:"description"`
	Namespace   string    `json:"namespace,omitempty"`
	AlertID     string    `json:"alertId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
}

// Store persists incidents to a local JSON file.
type Store struct {
	mu        sync.RWMutex
	incidents []Incident
	filePath  string
	logger    *slog.Logger
}

// NewStore creates an incident store. If dataDir is empty, uses ~/.kubewatcher/incidents.json.
func NewStore(dataDir string, logger *slog.Logger) *Store {
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".kubewatcher")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		logger.Error("incidents: failed to create data directory", slog.String("path", dataDir), slog.String("error", err.Error()))
	}

	s := &Store{
		filePath: filepath.Join(dataDir, "incidents.json"),
		logger:   logger,
	}
	s.load()
	return s
}

// List returns all incidents, newest first.
func (s *Store) List(_ context.Context) []Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Incident, len(s.incidents))
	copy(out, s.incidents)
	sort.Slice(out, func(i, j int) bool {
		return out[j].CreatedAt.Before(out[i].CreatedAt)
	})
	return out
}

// Get returns a single incident by ID.
func (s *Store) Get(_ context.Context, id string) (*Incident, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.incidents {
		if s.incidents[i].ID == id {
			inc := s.incidents[i]
			return &inc, nil
		}
	}
	return nil, fmt.Errorf("incident %q not found", id)
}

// Create adds a new incident and persists.
func (s *Store) Create(_ context.Context, title, severity, incType, description, namespace string) (Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	id := fmt.Sprintf("inc-%d", now.UnixMilli())

	if severity == "" {
		severity = "info"
	}
	if incType == "" {
		incType = "alert"
	}

	inc := Incident{
		ID:          id,
		Title:       title,
		Severity:    severity,
		Status:      "open",
		Type:        incType,
		Description: description,
		Namespace:   namespace,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.incidents = append(s.incidents, inc)
	s.persist()
	return inc, nil
}

// Update modifies an existing incident.
func (s *Store) Update(_ context.Context, id, status, description string) (*Incident, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.incidents {
		if s.incidents[i].ID == id {
			now := time.Now()
			if status != "" {
				s.incidents[i].Status = status
				if status == "resolved" {
					s.incidents[i].ResolvedAt = &now
				}
			}
			if description != "" {
				s.incidents[i].Description = description
			}
			s.incidents[i].UpdatedAt = now
			s.persist()
			inc := s.incidents[i]
			return &inc, nil
		}
	}
	return nil, fmt.Errorf("incident %q not found", id)
}

// Delete removes an incident.
func (s *Store) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.incidents {
		if s.incidents[i].ID == id {
			s.incidents = append(s.incidents[:i], s.incidents[i+1:]...)
			s.persist()
			return nil
		}
	}
	return fmt.Errorf("incident %q not found", id)
}

// persist writes the incidents to disk atomically (write temp + rename).
func (s *Store) persist() {
	data, err := json.MarshalIndent(s.incidents, "", "  ")
	if err != nil {
		s.logger.Error("failed to marshal incidents", "error", err)
		return
	}

	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		s.logger.Error("failed to write incidents temp file", "error", err)
		return
	}
	if err := os.Rename(tmp, s.filePath); err != nil {
		s.logger.Error("failed to rename incidents temp file", "error", err)
	}
}

// load reads incidents from disk.
func (s *Store) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			s.logger.Error("failed to read incidents file", "error", err)
		}
		return
	}

	var loaded []Incident
	if err := json.Unmarshal(data, &loaded); err != nil {
		s.logger.Error("failed to parse incidents file", "error", err)
		return
	}
	s.incidents = loaded
}
