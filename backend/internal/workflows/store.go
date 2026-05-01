// Package workflows provides CRUD persistence for automation workflows.
package workflows

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Step is a single action in a workflow.
type Step struct {
	ID         int    `json:"id"`
	Type       string `json:"type"`       // "trigger" or "action"
	Name       string `json:"name"`
	Icon       string `json:"icon"`
	ActionType string `json:"actionType"` // e.g. "python", "slack", "http", "custom"
	Config     any    `json:"config,omitempty"`
}

// Workflow is a named automation consisting of ordered steps.
type Workflow struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Steps     []Step    `json:"steps"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// WorkflowSummary is returned by List (without full step configs).
type WorkflowSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	StepCount int       `json:"stepCount"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store persists workflows as individual JSON files in a directory.
type Store struct {
	dir    string
	mu     sync.RWMutex
	logger *slog.Logger
}

// New creates a workflow store backed by the given directory.
func New(dataDir string, logger *slog.Logger) (*Store, error) {
	dir := filepath.Join(dataDir, "workflows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("workflows: create dir: %w", err)
	}
	return &Store{dir: dir, logger: logger}, nil
}

// List returns summaries of all workflows, sorted by last update (newest first).
func (s *Store) List() ([]WorkflowSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	var out []WorkflowSummary
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		wf, err := s.readFile(e.Name())
		if err != nil {
			s.logger.Warn("workflows: skipping corrupt file", slog.String("file", e.Name()), slog.String("error", err.Error()))
			continue
		}
		out = append(out, WorkflowSummary{
			ID:        wf.ID,
			Title:     wf.Title,
			StepCount: len(wf.Steps),
			CreatedAt: wf.CreatedAt,
			UpdatedAt: wf.UpdatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Get returns a full workflow by ID.
func (s *Store) Get(id string) (*Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readFile(id + ".json")
}

// Save creates or updates a workflow. If wf.ID is empty, a new ID is assigned.
func (s *Store) Save(wf *Workflow) (*Workflow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if wf.ID == "" {
		wf.ID = uuid.NewString()
		wf.CreatedAt = now
	}
	wf.UpdatedAt = now

	data, err := json.MarshalIndent(wf, "", "  ")
	if err != nil {
		return nil, err
	}

	// Atomic write: temp file + rename to prevent corruption on partial writes.
	path := filepath.Join(s.dir, wf.ID+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, fmt.Errorf("workflows: write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return nil, fmt.Errorf("workflows: rename: %w", err)
	}
	s.logger.Info("workflow saved", slog.String("id", wf.ID), slog.String("title", wf.Title))
	return wf, nil
}

// Delete removes a workflow by ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	s.logger.Info("workflow deleted", slog.String("id", id))
	return nil
}

func (s *Store) readFile(name string) (*Workflow, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return nil, err
	}
	var wf Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		return nil, err
	}
	return &wf, nil
}
