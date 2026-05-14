// Package clickhouse implements the DBAgent analysis routines for
// ClickHouse. ClickHouse exposes most operational state through the
// `system` database; the analyzer reads catalog tables read-only and
// gracefully degrades when an optional table (e.g. query_log) is
// disabled by config.
package clickhouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Queryer is the subset of *sql.DB the analyzer needs.
type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Analyzer is a thin facade over a *sql.DB.
type Analyzer struct {
	db      Queryer
	timeout time.Duration
}

// New constructs an Analyzer. system.* scans on a busy cluster can be
// slow; 10s is a reasonable cap.
func New(db Queryer, timeout time.Duration) *Analyzer {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Analyzer{db: db, timeout: timeout}
}

// Analyze dispatches to a per-section collector.
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
	return nil, fmt.Errorf("clickhouse: unknown analysis section %q", section)
}

// overview rolls up the headline numbers for a one-shot health call.
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

// resources reports on-disk bytes per database from system.parts plus
// the current memory tracker and a few cumulative events.
func (a *Analyzer) resources(ctx context.Context) (map[string]any, error) {
	const partsQ = `
		SELECT database, sum(bytes_on_disk)::Int64 AS bytes, sum(rows)::Int64 AS rows
		FROM system.parts
		WHERE active
		GROUP BY database
		ORDER BY bytes DESC`
	rows, err := a.db.QueryContext(ctx, partsQ)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: system.parts: %w", err)
	}
	defer rows.Close()

	type dbSize struct {
		Database string `json:"database"`
		Bytes    int64  `json:"bytes"`
		Rows     int64  `json:"rows"`
	}
	var sizes []dbSize
	var totalBytes int64
	for rows.Next() {
		var s dbSize
		if err := rows.Scan(&s.Database, &s.Bytes, &s.Rows); err != nil {
			return nil, err
		}
		sizes = append(sizes, s)
		totalBytes += s.Bytes
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	metrics := map[string]int64{}
	for _, name := range []string{"MemoryTracking", "Query"} {
		var v int64
		if err := a.db.QueryRowContext(ctx,
			`SELECT value FROM system.metrics WHERE metric = ?`, name).Scan(&v); err == nil {
			metrics[name] = v
		}
	}

	events := map[string]int64{}
	for _, name := range []string{"Query", "SelectQuery", "InsertQuery", "FailedQuery"} {
		var v int64
		if err := a.db.QueryRowContext(ctx,
			`SELECT value FROM system.events WHERE event = ?`, name).Scan(&v); err == nil {
			events[name] = v
		}
	}

	return map[string]any{
		"db_bytes":  totalBytes,
		"databases": sizes,
		"metrics":   metrics,
		"events":    events,
	}, nil
}

// connections breaks down system.processes by query_kind and user. The
// by_state map shape matches the Postgres analyzer for UI parity.
func (a *Analyzer) connections(ctx context.Context) (map[string]any, error) {
	const q = `
		SELECT coalesce(query_kind, 'unknown') AS kind, count()::Int64 AS n
		FROM system.processes
		GROUP BY kind`
	rows, err := a.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: system.processes: %w", err)
	}
	defer rows.Close()
	byState := map[string]int64{}
	var total int64
	for rows.Next() {
		var k string
		var n int64
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		byState[k] = n
		total += n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var maxConn int64
	_ = a.db.QueryRowContext(ctx,
		`SELECT toInt64(value) FROM system.settings WHERE name = 'max_concurrent_queries'`,
	).Scan(&maxConn)

	util := 0.0
	if maxConn > 0 {
		util = float64(total) / float64(maxConn)
	}

	return map[string]any{
		"by_state":         byState,
		"total":            total,
		"max_connections":  maxConn,
		"pool_utilization": util,
	}, nil
}

// indexes — ClickHouse doesn't have B-tree indexes; tables are sorted
// by ORDER BY with a sparse primary index. We surface per-column bytes
// from system.parts_columns (a useful proxy for "what's heavy in this
// table") plus any user-defined data-skipping indices.
func (a *Analyzer) indexes(ctx context.Context) (map[string]any, error) {
	const colQ = `
		SELECT database, table, column,
		       sum(column_bytes_on_disk)::Int64 AS bytes
		FROM system.parts_columns
		WHERE active
		GROUP BY database, table, column
		ORDER BY bytes DESC
		LIMIT 100`
	rows, err := a.db.QueryContext(ctx, colQ)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: system.parts_columns: %w", err)
	}
	defer rows.Close()

	type colRow struct {
		Database string `json:"database"`
		Table    string `json:"table"`
		Column   string `json:"column"`
		Bytes    int64  `json:"bytes"`
	}
	var cols []colRow
	for rows.Next() {
		var r colRow
		if err := rows.Scan(&r.Database, &r.Table, &r.Column, &r.Bytes); err != nil {
			return nil, err
		}
		cols = append(cols, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type skipIdx struct {
		Database string `json:"database"`
		Table    string `json:"table"`
		Name     string `json:"name"`
		Type     string `json:"type"`
	}
	var skips []skipIdx
	// data_skipping_indices was added in 20.x; guard against older builds.
	skRows, err := a.db.QueryContext(ctx,
		`SELECT database, table, name, type FROM system.data_skipping_indices LIMIT 200`)
	if err == nil {
		defer skRows.Close()
		for skRows.Next() {
			var s skipIdx
			if err := skRows.Scan(&s.Database, &s.Table, &s.Name, &s.Type); err != nil {
				return nil, err
			}
			skips = append(skips, s)
		}
	}

	return map[string]any{
		"note":                  "ClickHouse uses sparse primary keys; per-column bytes shown as a layout proxy.",
		"columns":               cols,
		"data_skipping_indices": skips,
	}, nil
}

// queries reads system.query_log when enabled. The table is optional
// (log_queries=0 disables it), so we probe its existence first and
// surface a hint when it's absent — same pattern as the Postgres
// pg_stat_statements check.
func (a *Analyzer) queries(ctx context.Context) (map[string]any, error) {
	var exists int64
	err := a.db.QueryRowContext(ctx,
		`SELECT count() FROM system.tables WHERE database = 'system' AND name = 'query_log'`,
	).Scan(&exists)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("clickhouse: probe query_log: %w", err)
	}
	if exists == 0 {
		return map[string]any{
			"available": false,
			"hint":      "Enable query logging via <query_log> in config.xml (log_queries=1 by default in recent builds).",
			"slow":      []any{},
		}, nil
	}

	const q = `
		SELECT query_id,
		       substring(query, 1, 400) AS query,
		       toInt64(query_duration_ms) AS duration_ms,
		       toInt64(read_rows)         AS read_rows,
		       toInt64(memory_usage)      AS memory_usage
		FROM system.query_log
		WHERE type = 'QueryFinish' AND event_time > now() - INTERVAL 1 DAY
		ORDER BY query_duration_ms DESC
		LIMIT 25`
	rows, err := a.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: query_log read: %w", err)
	}
	defer rows.Close()

	type slowRow struct {
		ID         string `json:"id"`
		Query      string `json:"query"`
		DurationMs int64  `json:"duration_ms"`
		ReadRows   int64  `json:"read_rows"`
		MemBytes   int64  `json:"memory_usage"`
	}
	var out []slowRow
	for rows.Next() {
		var r slowRow
		if err := rows.Scan(&r.ID, &r.Query, &r.DurationMs, &r.ReadRows, &r.MemBytes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return map[string]any{"available": true, "slow": out}, rows.Err()
}

// replication reads system.replicas; only Replicated* engines populate
// it, so an empty result on a single-node cluster is normal.
func (a *Analyzer) replication(ctx context.Context) (map[string]any, error) {
	const q = `
		SELECT
			database,
			table,
			replica_name,
			is_session_expired,
			toInt64(queue_size)                          AS queue_size,
			toInt64(log_max_index - log_pointer)         AS lag_log_entries
		FROM system.replicas`
	rows, err := a.db.QueryContext(ctx, q)
	if err != nil {
		// system.replicas isn't a hard requirement (only present when
		// the build has replication). Surface an empty list rather than
		// erroring out the whole section.
		return map[string]any{"replicas": []any{}}, nil
	}
	defer rows.Close()

	type replRow struct {
		Database         string `json:"database"`
		Table            string `json:"table"`
		ReplicaName      string `json:"replica_name"`
		IsSessionExpired bool   `json:"is_session_expired"`
		QueueSize        int64  `json:"queue_size"`
		LagLogEntries    int64  `json:"lag_log_entries"`
	}
	var out []replRow
	for rows.Next() {
		var r replRow
		if err := rows.Scan(&r.Database, &r.Table, &r.ReplicaName,
			&r.IsSessionExpired, &r.QueueSize, &r.LagLogEntries); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return map[string]any{"replicas": out}, rows.Err()
}
