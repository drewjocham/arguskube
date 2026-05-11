package database

import (
	"context"
	"fmt"
)

// CleanupOrphanedRedisKeys removes Redis keys for specs that no longer exist in PG.
// Called from admin endpoint or scheduled job.
func (db *DB) GetActiveSpecIDs(ctx context.Context) ([]string, error) {
	rows, err := db.Pool.Query(ctx, "SELECT id FROM api_specs")
	if err != nil {
		return nil, fmt.Errorf("get active spec ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan spec id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// GDPRDeleteSpec completely removes all data associated with a spec.
// This handles the right to erasure (GDPR Article 17).
func (db *DB) GDPRDeleteSpec(ctx context.Context, specID string) error {
	// Delete in order of dependency
	tables := []string{
		"generated_tests",
		"cross_stream_drifts",
		"alert_history",
		"user_feedback",
		"investigations",
		"anomaly_metrics",
		"llm_usage",
		"drift_reports",
		"endpoints",
		"streams",
		"api_specs",
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, table := range tables {
		_, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE spec_id = $1", table), specID)
		if err != nil {
			return fmt.Errorf("delete from %s: %w", table, err)
		}
	}

	return tx.Commit(ctx)
}

// CountUserData returns count of data rows for a spec (GDPR right to access).
func (db *DB) CountUserData(ctx context.Context, specID string) (map[string]int, error) {
	counts := make(map[string]int)
	tables := []string{
		"api_specs", "endpoints", "drift_reports", "generated_tests",
		"cross_stream_drifts", "alert_history", "user_feedback",
		"investigations", "anomaly_metrics", "llm_usage",
	}

	for _, table := range tables {
		var count int
		err := db.Pool.QueryRow(ctx,
			fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE spec_id = $1", table), specID,
		).Scan(&count)
		if err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}
		if count > 0 {
			counts[table] = count
		}
	}
	return counts, nil
}
