// Package sqlitedb provides a shared SQLite database for local persistence,
// replacing individual JSON file stores with a single embedded database.
// Uses modernc.org/sqlite (pure-Go, no CGo required).
package sqlitedb

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// DB wraps a sql.DB with schema migration support.
type DB struct {
	*sql.DB
	logger *slog.Logger
	mu     sync.Mutex
}

// Open opens (or creates) a SQLite database at dataDir/kubewatcher.db.
// It applies WAL mode and busy timeout for safe concurrent access.
func Open(dataDir string, logger *slog.Logger) (*DB, error) {
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".kubewatcher")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("sqlitedb: mkdir %s: %w", dataDir, err)
	}

	dsn := filepath.Join(dataDir, "kubewatcher.db")
	raw, err := sql.Open("sqlite", dsn+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("sqlitedb: open %s: %w", dsn, err)
	}

	// Single writer, multiple readers is fine for our use case.
	raw.SetMaxOpenConns(1)

	db := &DB{DB: raw, logger: logger}
	if err := db.migrate(); err != nil {
		raw.Close()
		return nil, fmt.Errorf("sqlitedb: migrate: %w", err)
	}

	logger.Info("sqlitedb: opened", slog.String("path", dsn))
	return db, nil
}

// migrate applies all schema migrations in order.
func (db *DB) migrate() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Create migration tracking table.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create _migrations: %w", err)
	}

	for i, m := range migrations {
		ver := i + 1
		var applied int
		if err := db.QueryRow("SELECT COUNT(*) FROM _migrations WHERE version = ?", ver).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %d: %w", ver, err)
		}
		if applied > 0 {
			continue
		}

		db.logger.Info("sqlitedb: applying migration", slog.Int("version", ver), slog.String("name", m.name))
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %d (%s): %w", ver, m.name, err)
		}
		if _, err := db.Exec("INSERT INTO _migrations (version) VALUES (?)", ver); err != nil {
			return fmt.Errorf("record migration %d: %w", ver, err)
		}
	}
	return nil
}

type migration struct {
	name string
	sql  string
}

// migrations is the ordered list of schema changes. Append-only.
var migrations = []migration{
	{
		name: "create_incidents",
		sql: `CREATE TABLE incidents (
			id          TEXT PRIMARY KEY,
			title       TEXT NOT NULL,
			severity    TEXT NOT NULL DEFAULT 'info',
			status      TEXT NOT NULL DEFAULT 'open',
			type        TEXT NOT NULL DEFAULT 'alert',
			description TEXT NOT NULL DEFAULT '',
			namespace   TEXT NOT NULL DEFAULT '',
			alert_id    TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			updated_at  TEXT NOT NULL,
			resolved_at TEXT,
			tags        TEXT NOT NULL DEFAULT '[]'
		)`,
	},
	{
		name: "create_workflows",
		sql: `CREATE TABLE workflows (
			id         TEXT PRIMARY KEY,
			title      TEXT NOT NULL,
			steps      TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	},
}
