// Package sqlite implements the DBAgent analysis routines for SQLite.
//
// SQLite is single-process and lacks server-side observability (no
// pg_stat_activity, no slow query log, no replication). We still
// surface the same set of sections as the Postgres analyzer so the MCP
// consumer can render one UI across dialects — sections that don't
// apply return an explicit {"available": false, "hint": "..."} blob
// rather than an error.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Queryer is the subset of *sql.DB the analyzer needs. Mirrors the
// Postgres analyzer's interface so unit tests can swap stubs.
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

// New constructs an Analyzer. PRAGMA reads are cheap; 10s is generous.
func New(db Queryer, timeout time.Duration) *Analyzer {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Analyzer{db: db, timeout: timeout}
}

// Analyze dispatches to a per-section collector. Unknown sections
// surface as errors so MCP can return a clear "unknown analysis_type".
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
	return nil, fmt.Errorf("sqlite: unknown analysis section %q", section)
}

// overview rolls up the headline numbers for a one-shot health call.
func (a *Analyzer) overview(ctx context.Context) (map[string]any, error) {
	res, _ := a.resources(ctx)
	conns, _ := a.connections(ctx)
	return map[string]any{
		"resources":   res,
		"connections": conns,
		"replication": map[string]any{"replicas": []any{}},
	}, nil
}

// resources reports DB size from page_count*page_size, free pages from
// freelist_count, and journaling mode. All values come from PRAGMAs so
// no catalog access is needed.
func (a *Analyzer) resources(ctx context.Context) (map[string]any, error) {
	pageCount := readPragmaInt(ctx, a.db, "page_count")
	pageSize := readPragmaInt(ctx, a.db, "page_size")
	freelist := readPragmaInt(ctx, a.db, "freelist_count")
	cacheSize := readPragmaInt(ctx, a.db, "cache_size")
	journal := readPragmaString(ctx, a.db, "journal_mode")

	out := map[string]any{
		"db_bytes":     pageCount * pageSize,
		"page_count":   pageCount,
		"page_size":    pageSize,
		"free_pages":   freelist,
		"free_bytes":   freelist * pageSize,
		"cache_size":   cacheSize,
		"journal_mode": journal,
	}

	// In WAL mode, expose the checkpoint state. We read PASSIVE which
	// is the lowest-impact form — it doesn't acquire any write locks
	// when no checkpoint is needed. Result columns: busy, log, checkpointed.
	if strings.EqualFold(journal, "wal") {
		var busy, log, ckpt int64
		err := a.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(PASSIVE)`).
			Scan(&busy, &log, &ckpt)
		if err == nil {
			out["wal_busy"] = busy
			out["wal_pages"] = log
			out["wal_checkpointed"] = ckpt
		}
	}

	return out, nil
}

// connections reports the few process-level knobs SQLite exposes. The
// shape mirrors the Postgres analyzer's by_state/total/max_connections
// layout so the UI can render one component for both dialects.
func (a *Analyzer) connections(ctx context.Context) (map[string]any, error) {
	busy := readPragmaInt(ctx, a.db, "busy_timeout")

	// :memory: databases are single-handle by definition; on-disk DBs
	// can have many readers but SQLite doesn't track them.
	var dbList string
	_ = a.db.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name='main'`).Scan(&dbList)
	inMemory := dbList == "" || dbList == ":memory:"

	return map[string]any{
		"by_state":         map[string]int64{"active": 1},
		"total":            int64(1),
		"max_connections":  int64(0), // unbounded at the SQLite layer
		"pool_utilization": 0.0,
		"busy_timeout_ms":  busy,
		"in_memory":        inMemory,
	}, nil
}

// indexes lists every index from sqlite_master and computes a column
// count from PRAGMA index_info. Auto-created indexes (PK/UNIQUE) are
// flagged so the UI can deprioritize them.
func (a *Analyzer) indexes(ctx context.Context) (map[string]any, error) {
	const q = `SELECT name, tbl_name FROM sqlite_master WHERE type='index' ORDER BY tbl_name, name`
	rows, err := a.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list indexes: %w", err)
	}

	type indexRow struct {
		Name    string `json:"index"`
		Table   string `json:"table"`
		Columns int64  `json:"columns"`
		Auto    bool   `json:"auto_created"`
	}
	var out []indexRow
	// Drain the outer query FIRST before issuing the per-index column
	// counts. SQLite's pool with MaxOpenConns(1) (the production
	// configuration in sqlitedb.Open) would deadlock if we held the
	// outer Rows open while waiting for a connection to run the inner
	// query. Gather names, close, then enrich.
	for rows.Next() {
		var r indexRow
		if err := rows.Scan(&r.Name, &r.Table); err != nil {
			rows.Close()
			return nil, err
		}
		r.Auto = strings.HasPrefix(r.Name, "sqlite_autoindex_")
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	for i := range out {
		// pragma_index_info returns one row per indexed column. We count
		// rather than scan columns because callers don't usually need
		// the column names — just enough to spot "this is a 5-col index".
		if n, err := countRows(ctx, a.db,
			fmt.Sprintf(`SELECT COUNT(*) FROM pragma_index_info(%s)`, sqliteQuote(out[i].Name))); err == nil {
			out[i].Columns = n
		}
	}
	return map[string]any{"indexes": out}, nil
}

// queries — SQLite has no built-in slow-query log. The MCP tool surfaces
// the limitation rather than returning an empty success.
func (a *Analyzer) queries(ctx context.Context) (map[string]any, error) {
	return map[string]any{
		"available": false,
		"hint":      "SQLite has no built-in slow-query log; enable -wal mode and use the .timer in sqlite3 CLI for ad-hoc profiling.",
		"slow":      []any{},
	}, nil
}

// replication — SQLite has no native replication; LiteFS/rqlite live
// above SQLite and would be queried separately.
func (a *Analyzer) replication(ctx context.Context) (map[string]any, error) {
	return map[string]any{"replicas": []any{}}, nil
}

// readPragmaInt runs `PRAGMA <name>` and returns the integer value, or 0
// if the pragma is unrecognized or returns a non-integer.
func readPragmaInt(ctx context.Context, db Queryer, name string) int64 {
	var v sql.NullInt64
	if err := db.QueryRowContext(ctx, "PRAGMA "+name).Scan(&v); err != nil {
		return 0
	}
	if !v.Valid {
		return 0
	}
	return v.Int64
}

// readPragmaString runs `PRAGMA <name>` and returns the text value.
func readPragmaString(ctx context.Context, db Queryer, name string) string {
	var v sql.NullString
	if err := db.QueryRowContext(ctx, "PRAGMA "+name).Scan(&v); err != nil {
		return ""
	}
	return v.String
}

func countRows(ctx context.Context, db Queryer, q string) (int64, error) {
	var n int64
	if err := db.QueryRowContext(ctx, q).Scan(&n); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return n, nil
}

// sqliteQuote produces a single-quoted SQL string literal. The index
// names come from sqlite_master (not user input), but we still quote
// to handle names with unusual characters cleanly.
func sqliteQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
