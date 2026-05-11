package incidents

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

// incidentSeq disambiguates IDs created within the same nanosecond — concurrent
// Create() calls would otherwise collide on the UNIQUE primary key.
var incidentSeq atomic.Uint64

// Incident represents a tracked incident.
type Incident struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Severity    string     `json:"severity"`
	Status      string     `json:"status"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Namespace   string     `json:"namespace,omitempty"`
	AlertID     string     `json:"alertId,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
}

// Store persists incidents to SQLite via a shared database handle.
type Store struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewStore creates an incident store backed by the given sql.DB.
// The incidents table must already exist (created by sqlitedb.Open migrations).
func NewStore(db *sql.DB, logger *slog.Logger) *Store {
	return &Store{db: db, logger: logger}
}

// List returns all incidents, newest first.
func (s *Store) List(_ context.Context) []Incident {
	rows, err := s.db.Query(`
		SELECT id, title, severity, status, type, description, namespace,
		       alert_id, created_at, updated_at, resolved_at, tags
		FROM incidents ORDER BY created_at DESC`)
	if err != nil {
		s.logger.Error("incidents: list query failed", slog.String("error", err.Error()))
		return nil
	}
	defer rows.Close()

	var out []Incident
	for rows.Next() {
		inc, err := scanIncident(rows)
		if err != nil {
			s.logger.Warn("incidents: scan row failed", slog.String("error", err.Error()))
			continue
		}
		out = append(out, inc)
	}
	return out
}

// Get returns a single incident by ID.
func (s *Store) Get(_ context.Context, id string) (*Incident, error) {
	row := s.db.QueryRow(`
		SELECT id, title, severity, status, type, description, namespace,
		       alert_id, created_at, updated_at, resolved_at, tags
		FROM incidents WHERE id = ?`, id)

	inc, err := scanIncidentRow(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("incident %q not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("incidents: get %q: %w", id, err)
	}
	return &inc, nil
}

// Create adds a new incident and persists it.
func (s *Store) Create(_ context.Context, title, severity, incType, description, namespace string) (Incident, error) {
	now := time.Now()
	id := fmt.Sprintf("inc-%d-%d", now.UnixNano(), incidentSeq.Add(1))

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

	tagsJSON, _ := json.Marshal(inc.Tags)
	_, err := s.db.Exec(`
		INSERT INTO incidents (id, title, severity, status, type, description, namespace,
		                       alert_id, created_at, updated_at, resolved_at, tags)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		inc.ID, inc.Title, inc.Severity, inc.Status, inc.Type,
		inc.Description, inc.Namespace, inc.AlertID,
		inc.CreatedAt.Format(time.RFC3339Nano),
		inc.UpdatedAt.Format(time.RFC3339Nano),
		nil, // resolved_at
		string(tagsJSON),
	)
	if err != nil {
		return Incident{}, fmt.Errorf("incidents: create: %w", err)
	}
	return inc, nil
}

// Update modifies an existing incident.
func (s *Store) Update(_ context.Context, id, status, description string) (*Incident, error) {
	inc, err := s.Get(context.Background(), id)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if status != "" {
		inc.Status = status
		if status == "resolved" {
			inc.ResolvedAt = &now
		}
	}
	if description != "" {
		inc.Description = description
	}
	inc.UpdatedAt = now

	var resolvedStr *string
	if inc.ResolvedAt != nil {
		s := inc.ResolvedAt.Format(time.RFC3339Nano)
		resolvedStr = &s
	}

	_, err = s.db.Exec(`
		UPDATE incidents SET status = ?, description = ?, updated_at = ?, resolved_at = ?
		WHERE id = ?`,
		inc.Status, inc.Description,
		inc.UpdatedAt.Format(time.RFC3339Nano),
		resolvedStr,
		inc.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("incidents: update %q: %w", id, err)
	}
	return inc, nil
}

// Delete removes an incident.
func (s *Store) Delete(_ context.Context, id string) error {
	res, err := s.db.Exec("DELETE FROM incidents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("incidents: delete %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("incident %q not found", id)
	}
	return nil
}

// scanIncident scans a row from a *sql.Rows into an Incident.
func scanIncident(rows *sql.Rows) (Incident, error) {
	var inc Incident
	var createdAt, updatedAt string
	var resolvedAt sql.NullString
	var tagsJSON string
	var alertID string

	err := rows.Scan(
		&inc.ID, &inc.Title, &inc.Severity, &inc.Status, &inc.Type,
		&inc.Description, &inc.Namespace, &alertID,
		&createdAt, &updatedAt, &resolvedAt, &tagsJSON,
	)
	if err != nil {
		return inc, err
	}

	inc.AlertID = alertID
	inc.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	inc.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if resolvedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, resolvedAt.String)
		inc.ResolvedAt = &t
	}
	if tagsJSON != "" && tagsJSON != "null" {
		_ = json.Unmarshal([]byte(tagsJSON), &inc.Tags)
	}
	return inc, nil
}

// scanIncidentRow scans a single *sql.Row into an Incident.
func scanIncidentRow(row *sql.Row) (Incident, error) {
	var inc Incident
	var createdAt, updatedAt string
	var resolvedAt sql.NullString
	var tagsJSON string
	var alertID string

	err := row.Scan(
		&inc.ID, &inc.Title, &inc.Severity, &inc.Status, &inc.Type,
		&inc.Description, &inc.Namespace, &alertID,
		&createdAt, &updatedAt, &resolvedAt, &tagsJSON,
	)
	if err != nil {
		return inc, err
	}

	inc.AlertID = alertID
	inc.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	inc.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	if resolvedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, resolvedAt.String)
		inc.ResolvedAt = &t
	}
	if tagsJSON != "" && tagsJSON != "null" {
		_ = json.Unmarshal([]byte(tagsJSON), &inc.Tags)
	}
	return inc, nil
}
