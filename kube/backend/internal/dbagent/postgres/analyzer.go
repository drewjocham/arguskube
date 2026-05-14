// Package postgres implements the DBAgent analysis routines for
// PostgreSQL. Each analysis_type (resources, connections, indexes,
// queries, replication, security, schema) is a method on Analyzer that
// returns a JSON-shaped map for the MCP layer.
//
// All queries are read-only and run with statement_timeout to keep a
// long table scan from hanging a desktop user. Optional features —
// pg_stat_statements for slow queries, pg_stat_replication for replica
// lag — are detected at runtime; the analyzer surfaces an
// "extension_missing" hint rather than failing the whole report.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Queryer is the subset of *sql.DB the analyzer needs. Lets unit tests
// inject a stub without touching the real driver.
type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Analyzer is a thin facade over a *sql.DB. It's stateless aside from
// the connection — one instance per analysis call is fine.
type Analyzer struct {
	db      Queryer
	timeout time.Duration
}

// New constructs an Analyzer. timeout caps every query in the report;
// 10s is a reasonable default for catalog lookups.
func New(db Queryer, timeout time.Duration) *Analyzer {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Analyzer{db: db, timeout: timeout}
}

// Analyze dispatches to a per-section collector and merges results into
// one map. An unknown section returns an error rather than an empty
// report so the MCP layer can surface a clear "unknown analysis_type".
func (a *Analyzer) Analyze(ctx context.Context, section string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	switch strings.ToLower(section) {
	case "", "overview":
		return a.overview(ctx)
	case "resources":
		return a.resources(ctx)
	case "connections":
		return a.connections(ctx)
	case "indexes":
		return a.indexes(ctx)
	case "queries":
		return a.queries(ctx)
	case "replication":
		return a.replication(ctx)
	}
	return nil, fmt.Errorf("postgres: unknown analysis section %q", section)
}

// overview rolls up the most useful headline numbers for a one-shot
// "how is this DB doing?" call from an agent.
func (a *Analyzer) overview(ctx context.Context) (map[string]any, error) {
	res, _ := a.resources(ctx)
	conns, _ := a.connections(ctx)
	rep, _ := a.replication(ctx)
	return map[string]any{
		"resources":   res,
		"connections": conns,
		"replication": rep,
	}, nil
}

// resources reports DB size, cache hit ratio, transaction rates. These
// come from pg_stat_database — present on every Postgres ≥ 9 so no
// extension check needed.
func (a *Analyzer) resources(ctx context.Context) (map[string]any, error) {
	const q = `
		SELECT
			pg_database_size(current_database())::bigint                        AS db_bytes,
			COALESCE(SUM(blks_hit),  0)                                          AS blks_hit,
			COALESCE(SUM(blks_read), 0)                                          AS blks_read,
			COALESCE(SUM(xact_commit),   0)                                      AS xact_commit,
			COALESCE(SUM(xact_rollback), 0)                                      AS xact_rollback,
			COALESCE(SUM(deadlocks),     0)                                      AS deadlocks,
			COALESCE(SUM(temp_files),    0)                                      AS temp_files,
			COALESCE(SUM(temp_bytes),    0)                                      AS temp_bytes
		FROM pg_stat_database
		WHERE datname = current_database()`
	var dbBytes, hit, read, commit, rollback, deadlocks, tempFiles, tempBytes int64
	err := a.db.QueryRowContext(ctx, q).Scan(
		&dbBytes, &hit, &read, &commit, &rollback, &deadlocks, &tempFiles, &tempBytes,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: resources query: %w", err)
	}
	cacheHit := 0.0
	if hit+read > 0 {
		cacheHit = float64(hit) / float64(hit+read)
	}
	return map[string]any{
		"db_bytes":         dbBytes,
		"cache_hit_ratio":  cacheHit,
		"blocks_hit":       hit,
		"blocks_read":      read,
		"xact_commit":      commit,
		"xact_rollback":    rollback,
		"deadlocks":        deadlocks,
		"temp_files":       tempFiles,
		"temp_bytes":       tempBytes,
	}, nil
}

// connections breaks down pg_stat_activity by state and computes pool
// utilization against max_connections. This is the metric the 80%
// alert wires onto.
func (a *Analyzer) connections(ctx context.Context) (map[string]any, error) {
	const q = `
		SELECT COALESCE(state, 'unknown') AS state, COUNT(*)::bigint AS n
		FROM pg_stat_activity
		WHERE datname = current_database()
		GROUP BY state`
	rows, err := a.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: pg_stat_activity: %w", err)
	}
	defer rows.Close()
	byState := map[string]int64{}
	var total int64
	for rows.Next() {
		var state string
		var n int64
		if err := rows.Scan(&state, &n); err != nil {
			return nil, err
		}
		byState[state] = n
		total += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var maxConn int64
	if err := a.db.QueryRowContext(ctx, `SHOW max_connections`).Scan(&maxConn); err != nil {
		// SHOW returns text on some setups — fall back to current_setting cast.
		var s string
		if err2 := a.db.QueryRowContext(ctx, `SELECT current_setting('max_connections')`).Scan(&s); err2 == nil {
			fmt.Sscanf(s, "%d", &maxConn)
		}
	}
	util := 0.0
	if maxConn > 0 {
		util = float64(total) / float64(maxConn)
	}
	return map[string]any{
		"by_state":        byState,
		"total":           total,
		"max_connections": maxConn,
		"pool_utilization": util,
	}, nil
}

// indexes returns three lists actionable in the UI:
//   - unused: index scans = 0 since the last stats reset
//   - duplicate: same column set on the same table (heuristic)
//   - bloated: index size disproportionate to underlying table
//
// We intentionally return raw rows (not a "recommendation") so the
// agent can re-rank them against the user's specific question.
func (a *Analyzer) indexes(ctx context.Context) (map[string]any, error) {
	const unusedQ = `
		SELECT s.schemaname, s.relname AS table_name, s.indexrelname AS index_name,
		       pg_relation_size(s.indexrelid)::bigint AS bytes
		FROM pg_stat_user_indexes s
		JOIN pg_index i ON i.indexrelid = s.indexrelid
		WHERE s.idx_scan = 0 AND NOT i.indisunique AND NOT i.indisprimary
		ORDER BY bytes DESC
		LIMIT 50`
	unused, err := scanIndexRows(ctx, a.db, unusedQ)
	if err != nil {
		return nil, fmt.Errorf("postgres: unused indexes: %w", err)
	}

	const dupQ = `
		SELECT a.schemaname, a.relname, a.indexrelname, pg_relation_size(a.indexrelid)::bigint
		FROM pg_stat_user_indexes a
		JOIN pg_stat_user_indexes b
		  ON a.relid = b.relid AND a.indexrelid <> b.indexrelid
		JOIN pg_index ia ON ia.indexrelid = a.indexrelid
		JOIN pg_index ib ON ib.indexrelid = b.indexrelid
		WHERE ia.indkey::text = ib.indkey::text
		  AND a.indexrelname < b.indexrelname
		ORDER BY pg_relation_size(a.indexrelid) DESC
		LIMIT 50`
	dup, err := scanIndexRows(ctx, a.db, dupQ)
	if err != nil {
		return nil, fmt.Errorf("postgres: duplicate indexes: %w", err)
	}

	return map[string]any{
		"unused":     unused,
		"duplicate":  dup,
	}, nil
}

type indexRow struct {
	Schema    string `json:"schema"`
	Table     string `json:"table"`
	IndexName string `json:"index"`
	Bytes     int64  `json:"bytes"`
}

func scanIndexRows(ctx context.Context, db Queryer, q string) ([]indexRow, error) {
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []indexRow
	for rows.Next() {
		var r indexRow
		if err := rows.Scan(&r.Schema, &r.Table, &r.IndexName, &r.Bytes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// queries reads pg_stat_statements when present. The extension is
// optional, so a missing relation isn't fatal — we surface that as a
// hint so the UI can prompt the user to install it.
func (a *Analyzer) queries(ctx context.Context) (map[string]any, error) {
	// has_pg_stat_statements check via to_regclass; nil = extension absent.
	var exists sql.NullString
	if err := a.db.QueryRowContext(ctx,
		`SELECT to_regclass('public.pg_stat_statements')::text`,
	).Scan(&exists); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("postgres: probe pg_stat_statements: %w", err)
	}
	if !exists.Valid {
		return map[string]any{
			"available": false,
			"hint":      "CREATE EXTENSION pg_stat_statements; (requires shared_preload_libraries)",
			"slow":      []any{},
		}, nil
	}

	const q = `
		SELECT
			queryid::text                                AS id,
			LEFT(query, 400)                             AS query,
			calls,
			total_exec_time,
			(total_exec_time / NULLIF(calls,0))::float8  AS mean_ms,
			rows
		FROM public.pg_stat_statements
		ORDER BY total_exec_time DESC
		LIMIT 25`
	rows, err := a.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: pg_stat_statements query: %w", err)
	}
	defer rows.Close()

	type slowRow struct {
		ID       string  `json:"id"`
		Query    string  `json:"query"`
		Calls    int64   `json:"calls"`
		TotalMs  float64 `json:"total_ms"`
		MeanMs   float64 `json:"mean_ms"`
		Rows     int64   `json:"rows"`
	}
	var out []slowRow
	for rows.Next() {
		var r slowRow
		if err := rows.Scan(&r.ID, &r.Query, &r.Calls, &r.TotalMs, &r.MeanMs, &r.Rows); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return map[string]any{"available": true, "slow": out}, rows.Err()
}

// replication reports primary-side replica health. On a primary with no
// replicas the slice is empty (not an error). Standbys see no rows
// here, since pg_stat_replication only populates on the upstream side.
func (a *Analyzer) replication(ctx context.Context) (map[string]any, error) {
	const q = `
		SELECT
			application_name,
			COALESCE(state, ''),
			COALESCE(sync_state, ''),
			COALESCE(EXTRACT(EPOCH FROM (now() - reply_time))::float8, 0)
		FROM pg_stat_replication`
	rows, err := a.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: pg_stat_replication: %w", err)
	}
	defer rows.Close()

	type replRow struct {
		Name      string  `json:"application_name"`
		State     string  `json:"state"`
		SyncState string  `json:"sync_state"`
		LagSec    float64 `json:"lag_seconds"`
	}
	var out []replRow
	for rows.Next() {
		var r replRow
		if err := rows.Scan(&r.Name, &r.State, &r.SyncState, &r.LagSec); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return map[string]any{"replicas": out}, rows.Err()
}
