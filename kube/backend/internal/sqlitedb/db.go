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

// Open opens (or creates) a SQLite database at dataDir/argus.db.
// It applies WAL mode and busy timeout for safe concurrent access.
func Open(dataDir string, logger *slog.Logger) (*DB, error) {
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".argus")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("sqlitedb: mkdir %s: %w", dataDir, err)
	}

	dsn := filepath.Join(dataDir, "argus.db")
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
	{
		name: "create_auth_users",
		sql: `CREATE TABLE users (
			id               TEXT PRIMARY KEY,
			email            TEXT NOT NULL COLLATE NOCASE,
			name             TEXT NOT NULL DEFAULT '',
			password_hash    TEXT NOT NULL DEFAULT '',
			provider         TEXT NOT NULL DEFAULT 'local',
			provider_subject TEXT NOT NULL DEFAULT '',
			created_at       INTEGER NOT NULL,
			last_login_at    INTEGER NOT NULL DEFAULT 0,
			UNIQUE(email, provider),
			UNIQUE(provider, provider_subject)
		)`,
	},
	{
		name: "create_auth_users_index",
		sql:  `CREATE INDEX idx_users_provider_subject ON users(provider, provider_subject)`,
	},
	{
		name: "create_auth_sessions",
		sql: `CREATE TABLE sessions (
			token_hash TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		)`,
	},
	{
		name: "create_auth_sessions_index_user",
		sql:  `CREATE INDEX idx_sessions_user_id ON sessions(user_id)`,
	},
	{
		name: "create_auth_sessions_index_exp",
		sql:  `CREATE INDEX idx_sessions_expires ON sessions(expires_at)`,
	},
	{
		name: "create_oauth_pending",
		sql: `CREATE TABLE oauth_pending (
			state         TEXT PRIMARY KEY,
			pkce_verifier TEXT NOT NULL DEFAULT '',
			provider      TEXT NOT NULL,
			session_token TEXT NOT NULL DEFAULT '',
			error         TEXT NOT NULL DEFAULT '',
			created_at    INTEGER NOT NULL,
			completed_at  INTEGER NOT NULL DEFAULT 0
		)`,
	},
	{
		// Single-row table holding the agent's permission profile.
		// Stored as JSON so we can add fields without schema bumps.
		name: "create_agent_profile",
		sql: `CREATE TABLE agent_profile (
			id         INTEGER PRIMARY KEY,
			body       TEXT    NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
	},
	{
		// Append-only log of every alert lifecycle event: fired,
		// investigated, ack'd, silenced. Used by the alert-detail UI
		// to show "what Argus did" and by the fatigue detector to
		// count silences/ignores.
		name: "create_alert_events",
		sql: `CREATE TABLE alert_events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			signature  TEXT NOT NULL DEFAULT '',
			alert_id   TEXT NOT NULL,
			kind       TEXT NOT NULL,
			body       TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		)`,
	},
	{
		name: "create_alert_events_index",
		sql:  `CREATE INDEX idx_alert_events_alert ON alert_events(alert_id, created_at DESC)`,
	},
	{
		name: "create_alert_events_sig_index",
		sql:  `CREATE INDEX idx_alert_events_sig ON alert_events(signature, created_at DESC)`,
	},
	{
		// Append-only nav log driving the userprofile suggester (§6).
		// One row per view the user actually visits. Tiny rows so
		// retention is cheap; the store keeps the most recent 5000.
		name: "create_user_activity",
		sql: `CREATE TABLE user_activity (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			ts         INTEGER NOT NULL,
			kind       TEXT NOT NULL DEFAULT 'nav',
			view_id    TEXT NOT NULL DEFAULT '',
			context    TEXT NOT NULL DEFAULT '',
			namespace  TEXT NOT NULL DEFAULT ''
		)`,
	},
	{
		name: "create_user_activity_index",
		sql:  `CREATE INDEX idx_user_activity_ts ON user_activity(ts DESC)`,
	},
	{
		// Persistent mute set so a "Don't ask again" survives restart.
		// Keyed by a stable suggestion key produced by the suggester.
		name: "create_user_profile_mutes",
		sql: `CREATE TABLE user_profile_mutes (
			mute_key  TEXT PRIMARY KEY,
			muted_at  INTEGER NOT NULL
		)`,
	},
	{
		// Audit trail of what the suggester actually showed, used by
		// the annoyance budget (3/day cap) and the auto-self-throttle
		// (silence for a week if user mutes >50% over 14 days).
		name: "create_user_suggestion_log",
		sql: `CREATE TABLE user_suggestion_log (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			mute_key   TEXT NOT NULL DEFAULT '',
			kind       TEXT NOT NULL DEFAULT '',
			outcome    TEXT NOT NULL DEFAULT 'shown',
			created_at INTEGER NOT NULL
		)`,
	},
	{
		name: "create_user_suggestion_log_index",
		sql:  `CREATE INDEX idx_user_suggestion_log_created ON user_suggestion_log(created_at DESC)`,
	},
}
