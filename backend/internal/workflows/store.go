// Package workflows provides CRUD persistence for automation workflows.
package workflows

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
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

// Store persists workflows to SQLite via a shared database handle.
type Store struct {
	db     *sql.DB
	logger *slog.Logger
}

// New creates a workflow store backed by the given sql.DB.
// The workflows table must already exist (created by sqlitedb.Open migrations).
func New(db *sql.DB, logger *slog.Logger) (*Store, error) {
	return &Store{db: db, logger: logger}, nil
}

// List returns summaries of all workflows, sorted by last update (newest first).
func (s *Store) List() ([]WorkflowSummary, error) {
	rows, err := s.db.Query(`
		SELECT id, title, steps, created_at, updated_at
		FROM workflows ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("workflows: list: %w", err)
	}
	defer rows.Close()

	var out []WorkflowSummary
	for rows.Next() {
		var id, title, stepsJSON, createdAt, updatedAt string
		if err := rows.Scan(&id, &title, &stepsJSON, &createdAt, &updatedAt); err != nil {
			s.logger.Warn("workflows: scan row failed", slog.String("error", err.Error()))
			continue
		}
		var steps []Step
		_ = json.Unmarshal([]byte(stepsJSON), &steps)

		ca, _ := time.Parse(time.RFC3339Nano, createdAt)
		ua, _ := time.Parse(time.RFC3339Nano, updatedAt)

		out = append(out, WorkflowSummary{
			ID:        id,
			Title:     title,
			StepCount: len(steps),
			CreatedAt: ca,
			UpdatedAt: ua,
		})
	}
	return out, nil
}

// Get returns a full workflow by ID.
func (s *Store) Get(id string) (*Workflow, error) {
	row := s.db.QueryRow(`
		SELECT id, title, steps, created_at, updated_at
		FROM workflows WHERE id = ?`, id)

	var wf Workflow
	var stepsJSON, createdAt, updatedAt string
	err := row.Scan(&wf.ID, &wf.Title, &stepsJSON, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("workflow %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("workflows: get %q: %w", id, err)
	}

	_ = json.Unmarshal([]byte(stepsJSON), &wf.Steps)
	wf.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	wf.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return &wf, nil
}

// Save creates or updates a workflow. If wf.ID is empty, a new ID is assigned.
func (s *Store) Save(wf *Workflow) (*Workflow, error) {
	now := time.Now()
	if wf.ID == "" {
		wf.ID = uuid.NewString()
		wf.CreatedAt = now
	}
	wf.UpdatedAt = now

	stepsJSON, err := json.Marshal(wf.Steps)
	if err != nil {
		return nil, fmt.Errorf("workflows: marshal steps: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO workflows (id, title, steps, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			steps = excluded.steps,
			updated_at = excluded.updated_at`,
		wf.ID, wf.Title, string(stepsJSON),
		wf.CreatedAt.Format(time.RFC3339Nano),
		wf.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("workflows: save: %w", err)
	}

	s.logger.Info("workflow saved", slog.String("id", wf.ID), slog.String("title", wf.Title))
	return wf, nil
}

// Delete removes a workflow by ID.
func (s *Store) Delete(id string) error {
	res, err := s.db.Exec("DELETE FROM workflows WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("workflows: delete %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("workflow %q not found", id)
	}
	s.logger.Info("workflow deleted", slog.String("id", id))
	return nil
}
