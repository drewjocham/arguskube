package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/argus/api-gov/internal/models"
)

type SpecStore struct {
	db *DB
}

func NewSpecStore(db *DB) *SpecStore {
	return &SpecStore{db: db}
}

func (s *SpecStore) Create(ctx context.Context, spec *models.APISpec) error {
	spec.ID = uuid.New().String()
	spec.CreatedAt = time.Now()
	spec.UpdatedAt = spec.CreatedAt

	q := InsertInto("api_specs", "id", "name", "version", "content", "format", "created_at", "updated_at")
	sql, _ := q.Build()

	_, err := s.db.Pool.Exec(ctx, sql,
		spec.ID, spec.Name, spec.Version, spec.Content, spec.Format, spec.CreatedAt, spec.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create spec: %w", err)
	}
	return nil
}

func (s *SpecStore) GetByID(ctx context.Context, id string) (*models.APISpec, error) {
	q := Select("api_specs", "id", "name", "version", "content", "format", "created_at", "updated_at").
		Where("id", OpEq, id).
		Limit(1)

	sql, args := q.Build()
	spec := &models.APISpec{}

	err := s.db.Pool.QueryRow(ctx, sql, args...).Scan(
		&spec.ID, &spec.Name, &spec.Version, &spec.Content, &spec.Format, &spec.CreatedAt, &spec.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get spec %s: %w", id, err)
	}
	return spec, nil
}

func (s *SpecStore) List(ctx context.Context, limit, offset int) ([]*models.APISpec, int, error) {
	countQ := Select("api_specs", "COUNT(*)")
	countSQL, countArgs := countQ.Build()

	var total int
	if err := s.db.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count specs: %w", err)
	}

	q := Select("api_specs", "id", "name", "version", "format", "created_at", "updated_at").
		OrderBy("updated_at", false).
		Limit(limit).
		Offset(offset)

	sql, args := q.Build()
	rows, err := s.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list specs: %w", err)
	}
	defer rows.Close()

	var specs []*models.APISpec
	for rows.Next() {
		spec := &models.APISpec{}
		if err := rows.Scan(&spec.ID, &spec.Name, &spec.Version, &spec.Format, &spec.CreatedAt, &spec.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan spec: %w", err)
		}
		specs = append(specs, spec)
	}
	return specs, total, nil
}

func (s *SpecStore) Delete(ctx context.Context, id string) error {
	q := DeleteFrom("api_specs").Where("id", OpEq, id)
	sql, args := q.Build()

	result, err := s.db.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("delete spec %s: %w", id, err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("spec %s not found", id)
	}
	return nil
}

type EndpointStore struct {
	db *DB
}

func NewEndpointStore(db *DB) *EndpointStore {
	return &EndpointStore{db: db}
}

// ── Admin / GDPR ───────────────────────────────────────────────

func (s *SpecStore) GDPRDelete(ctx context.Context, specID string) error {
	tx, err := s.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tables := []string{
		"generated_tests", "cross_stream_drifts", "alert_history",
		"user_feedback", "investigations", "anomaly_metrics",
		"llm_usage", "drift_reports", "endpoints", "streams", "api_specs",
	}
	for _, table := range tables {
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE spec_id = $1", table), specID); err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
	}
	return tx.Commit(ctx)
}

func (s *SpecStore) CountUserData(ctx context.Context, specID string) (map[string]int, error) {
	counts := make(map[string]int)
	tables := []string{
		"api_specs", "endpoints", "drift_reports", "generated_tests",
		"cross_stream_drifts", "alert_history", "user_feedback",
		"investigations", "anomaly_metrics", "llm_usage",
	}
	for _, table := range tables {
		var count int
		if err := s.db.Pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE spec_id = $1", table), specID).Scan(&count); err != nil {
			continue
		}
		if count > 0 {
			counts[table] = count
		}
	}
	return counts, nil
}

func (s *SpecStore) GetAnomalyMetrics(ctx context.Context, specID string) (any, error) {
	type metricRow struct {
		Date           string  `json:"date"`
		TotalAlerts    int     `json:"total_alerts"`
		TruePositives  int     `json:"true_positives"`
		FalsePositives int     `json:"false_positives"`
		Precision      float64 `json:"precision"`
		Recall         float64 `json:"recall"`
	}
	rows, err := s.db.Pool.Query(ctx,
		"SELECT date, total_alerts, true_positives, false_positives, precision, recall FROM anomaly_metrics WHERE spec_id = $1 ORDER BY date DESC LIMIT 30",
		specID,
	)
	if err != nil {
		return nil, fmt.Errorf("get anomaly metrics: %w", err)
	}
	defer rows.Close()

	var metrics []metricRow
	for rows.Next() {
		var m metricRow
		if err := rows.Scan(&m.Date, &m.TotalAlerts, &m.TruePositives, &m.FalsePositives, &m.Precision, &m.Recall); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}
		metrics = append(metrics, m)
	}
	return metrics, nil
}

// ── Endpoints ──────────────────────────────────────────────────

func (e *EndpointStore) Upsert(ctx context.Context, ep *models.Endpoint) error {
	if ep.ID == "" {
		ep.ID = uuid.New().String()
	}

	q := InsertInto("endpoints",
		"id", "spec_id", "method", "path", "summary", "operation_id",
		"request_body", "responses", "parameters", "security", "tags").
		OnConflict("id").
		DoUpdate("method", "path", "summary", "operation_id", "request_body", "responses", "parameters", "security", "tags")

	sql, _ := q.Build()
	_, err := e.db.Pool.Exec(ctx, sql,
		ep.ID, ep.SpecID, ep.Method, ep.Path, ep.Summary, ep.OperationID,
		ep.RequestBody, ep.Responses, ep.Parameters, ep.Security, ep.Tags,
	)
	if err != nil {
		return fmt.Errorf("upsert endpoint: %w", err)
	}
	return nil
}

func (e *EndpointStore) UpsertBatch(ctx context.Context, endpoints []*models.Endpoint) error {
	tx, err := e.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, ep := range endpoints {
		if ep.ID == "" {
			ep.ID = uuid.New().String()
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO endpoints (id, spec_id, method, path, summary, operation_id, request_body, responses, parameters, security, tags)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
			 ON CONFLICT (id) DO UPDATE SET
			   method=EXCLUDED.method, path=EXCLUDED.path, summary=EXCLUDED.summary,
			   operation_id=EXCLUDED.operation_id, request_body=EXCLUDED.request_body,
			   responses=EXCLUDED.responses, parameters=EXCLUDED.parameters,
			   security=EXCLUDED.security, tags=EXCLUDED.tags`,
			ep.ID, ep.SpecID, ep.Method, ep.Path, ep.Summary, ep.OperationID,
			ep.RequestBody, ep.Responses, ep.Parameters, ep.Security, ep.Tags,
		)
		if err != nil {
			return fmt.Errorf("upsert endpoint %s %s: %w", ep.Method, ep.Path, err)
		}
	}
	return tx.Commit(ctx)
}

func (e *EndpointStore) GetBySpec(ctx context.Context, specID string, limit, offset int) ([]*models.Endpoint, int, error) {
	countQ := Select("endpoints", "COUNT(*)").Where("spec_id", OpEq, specID)
	countSQL, countArgs := countQ.Build()

	var total int
	if err := e.db.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count endpoints: %w", err)
	}

	q := Select("endpoints",
		"id", "spec_id", "method", "path", "summary", "operation_id",
		"request_body", "responses", "parameters", "security", "tags").
		Where("spec_id", OpEq, specID).
		OrderBy("method", true).
		OrderBy("path", true).
		Limit(limit).
		Offset(offset)

	sql, args := q.Build()
	rows, err := e.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []*models.Endpoint
	for rows.Next() {
		ep := &models.Endpoint{}
		if err := rows.Scan(
			&ep.ID, &ep.SpecID, &ep.Method, &ep.Path, &ep.Summary, &ep.OperationID,
			&ep.RequestBody, &ep.Responses, &ep.Parameters, &ep.Security, &ep.Tags,
		); err != nil {
			return nil, 0, fmt.Errorf("scan endpoint: %w", err)
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, total, nil
}

func (e *EndpointStore) UpdateEmbedding(ctx context.Context, endpointID string, embedding []float32) error {
	_, err := e.db.Pool.Exec(ctx,
		`UPDATE endpoints SET embedding = $1::vector WHERE id = $2`,
		embedding, endpointID,
	)
	if err != nil {
		return fmt.Errorf("update endpoint embedding: %w", err)
	}
	return nil
}
