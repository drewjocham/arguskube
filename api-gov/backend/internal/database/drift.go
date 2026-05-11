package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/argus/api-gov/internal/models"
)

type DriftStore struct {
	db *DB
}

func NewDriftStore(db *DB) *DriftStore {
	return &DriftStore{db: db}
}

func (d *DriftStore) Create(ctx context.Context, r *models.DriftReport) error {
	r.ID = uuid.New().String()
	r.CreatedAt = time.Now()

	q := InsertInto("drift_reports",
		"id", "spec_id", "endpoint_id", "severity", "category", "score",
		"source", "observed", "expected", "actual", "suggestion", "created_at")

	sql, _ := q.Build()
	_, err := d.db.Pool.Exec(ctx, sql,
		r.ID, r.SpecID, r.EndpointID, r.Severity, r.Category, r.Score,
		r.Source, r.Observed, r.Expected, r.Actual, r.Suggestion, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create drift report: %w", err)
	}
	return nil
}

func (d *DriftStore) CreateBatch(ctx context.Context, reports []*models.DriftReport) error {
	tx, err := d.db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, r := range reports {
		r.ID = uuid.New().String()
		r.CreatedAt = time.Now()

		_, err := tx.Exec(ctx,
			`INSERT INTO drift_reports (id, spec_id, endpoint_id, severity, category, score, source, observed, expected, actual, suggestion, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
			r.ID, r.SpecID, r.EndpointID, r.Severity, r.Category, r.Score,
			r.Source, r.Observed, r.Expected, r.Actual, r.Suggestion, r.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert drift report: %w", err)
		}
	}
	return tx.Commit(ctx)
}

func (d *DriftStore) List(ctx context.Context, specID string, filter *models.DriftFilter) ([]*models.DriftReport, int, error) {
	q := Select("drift_reports",
		"id", "spec_id", "endpoint_id", "severity", "category", "score",
		"source", "observed", "expected", "actual", "suggestion", "resolved", "created_at", "resolved_at").
		Where("spec_id", OpEq, specID).
		OrderBy("created_at", false).
		Limit(filter.Limit).
		Offset(filter.Offset())

	if filter.Resolved != nil {
		q.Where("resolved", OpEq, *filter.Resolved)
	}
	if filter.Severity != "" {
		q.Where("severity", OpEq, filter.Severity)
	}
	if filter.Category != "" {
		q.Where("category", OpEq, filter.Category)
	}

	countQ := Select("drift_reports", "COUNT(*)").Where("spec_id", OpEq, specID)
	if filter.Resolved != nil {
		countQ.Where("resolved", OpEq, *filter.Resolved)
	}
	if filter.Severity != "" {
		countQ.Where("severity", OpEq, filter.Severity)
	}
	if filter.Category != "" {
		countQ.Where("category", OpEq, filter.Category)
	}

	countSQL, countArgs := countQ.Build()
	var total int
	if err := d.db.Pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count drift reports: %w", err)
	}

	sql, args := q.Build()
	rows, err := d.db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list drift reports: %w", err)
	}
	defer rows.Close()

	var reports []*models.DriftReport
	for rows.Next() {
		r := &models.DriftReport{}
		if err := rows.Scan(
			&r.ID, &r.SpecID, &r.EndpointID, &r.Severity, &r.Category, &r.Score,
			&r.Source, &r.Observed, &r.Expected, &r.Actual, &r.Suggestion,
			&r.Resolved, &r.CreatedAt, &r.ResolvedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan drift report: %w", err)
		}
		reports = append(reports, r)
	}
	return reports, total, nil
}

func (d *DriftStore) Summary(ctx context.Context, specID string) (*models.DriftSummary, error) {
	q := Select("drift_reports",
		"COUNT(*)", "COALESCE(SUM(CASE WHEN severity='critical' THEN 1 ELSE 0 END), 0)",
		"COALESCE(SUM(CASE WHEN severity='high' THEN 1 ELSE 0 END), 0)",
		"COALESCE(AVG(score), 0)", "COALESCE(MAX(created_at), '1970-01-01'::timestamptz)").
		Where("spec_id", OpEq, specID).
		Where("resolved", OpEq, false)

	sql, args := q.Build()
	summary := &models.DriftSummary{SpecID: specID}

	if err := d.db.Pool.QueryRow(ctx, sql, args...).Scan(
		&summary.TotalDrifts, &summary.CriticalCount, &summary.HighCount,
		&summary.AvgScore, &summary.LastDetected,
	); err != nil {
		return summary, nil
	}

	catQ := Select("drift_reports", "category", "COUNT(*)").
		Where("spec_id", OpEq, specID).
		Where("resolved", OpEq, false).
		GroupBy("category")

	catSQL, catArgs := catQ.Build()
	rows, err := d.db.Pool.Query(ctx, catSQL, catArgs...)
	if err != nil {
		return summary, nil
	}
	defer rows.Close()

	summary.ByCategory = make(map[string]int)
	for rows.Next() {
		var cat string
		var count int
		if err := rows.Scan(&cat, &count); err != nil {
			break
		}
		summary.ByCategory[cat] = count
	}
	return summary, nil
}

func (d *DriftStore) Resolve(ctx context.Context, id string) error {
	now := time.Now()
	_, err := d.db.Pool.Exec(ctx,
		`UPDATE drift_reports SET resolved = TRUE, resolved_at = $1 WHERE id = $2`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("resolve drift report %s: %w", id, err)
	}
	return nil
}

func (d *DriftStore) VectorSearch(ctx context.Context, embedding []float32, limit int) ([]*models.Endpoint, error) {
	rows, err := d.db.Pool.Query(ctx,
		`SELECT id, spec_id, method, path, summary, operation_id,
		        request_body, responses, parameters, security, tags,
		        1 - (embedding <=> $1::vector) AS similarity
		 FROM endpoints
		 WHERE embedding IS NOT NULL
		 ORDER BY embedding <=> $1::vector
		 LIMIT $2`,
		embedding, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	var endpoints []*models.Endpoint
	for rows.Next() {
		ep := &models.Endpoint{}
		var similarity float64
		if err := rows.Scan(
			&ep.ID, &ep.SpecID, &ep.Method, &ep.Path, &ep.Summary, &ep.OperationID,
			&ep.RequestBody, &ep.Responses, &ep.Parameters, &ep.Security, &ep.Tags,
			&similarity,
		); err != nil {
			return nil, fmt.Errorf("scan vector search: %w", err)
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints, nil
}
