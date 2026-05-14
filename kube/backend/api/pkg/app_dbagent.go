package pkg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/argues/argus/internal/dbagent/clickhouse"
	"github.com/argues/argus/internal/dbagent/postgres"
	"github.com/argues/argus/internal/dbagent/sqlite"
	"github.com/argues/argus/internal/dbconfig"
)

// DBAgent — Wails bindings for the database analysis subsystem.
//
// The frontend's "Databases" view talks to these methods:
//
//   - ListDBConnections / GetDBConnection / UpsertDBConnection /
//     DeleteDBConnection: CRUD on registered DBs. Passwords are
//     redacted on read; the UI re-supplies on update only.
//   - TestDBConnection: opens a one-shot pool, runs Ping, returns
//     latency + server version (or the driver's error message).
//   - AnalyzeDB: routes to the per-dialect analyzer (postgres for now).
//
// All methods return ("", "DB not configured") if the app was started
// without a dbconfig store (e.g. SaaS mode), so the frontend can show
// a feature-disabled card rather than 500-ing.

// DBConnectionInput is the wire shape the UI POSTs for upsert. We
// don't accept DBConnection directly because the frontend would have
// to know the CreatedAt timestamp; here we let the store own those.
type DBConnectionInput struct {
	ID       string   `json:"id,omitempty"`
	Name     string   `json:"name"`
	DBType   string   `json:"db_type"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	User     string   `json:"user"`
	Password string   `json:"password,omitempty"`
	DBName   string   `json:"db_name"`
	SSLMode  string   `json:"ssl_mode"`
	PoolSize int      `json:"pool_size"`
	Tags     []string `json:"tags"`
	Enabled  bool     `json:"enabled"`
}

// DBConnectionView is what the frontend reads back — same shape as
// DBConnection but with Password always elided. The UI never sees a
// stored password; for edits it re-types in a dedicated input.
type DBConnectionView struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	DBType    string   `json:"db_type"`
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	User      string   `json:"user"`
	DBName    string   `json:"db_name"`
	SSLMode   string   `json:"ssl_mode"`
	PoolSize  int      `json:"pool_size"`
	Tags      []string `json:"tags"`
	Enabled   bool     `json:"enabled"`
	CreatedAt int64    `json:"created_at"`
	UpdatedAt int64    `json:"updated_at"`
	HasPassword bool   `json:"has_password"`
}

func toView(c dbconfig.DBConnection) DBConnectionView {
	return DBConnectionView{
		ID: c.ID, Name: c.Name, DBType: string(c.DBType),
		Host: c.Host, Port: c.Port, User: c.User,
		DBName: c.DBName, SSLMode: string(c.SSLMode),
		PoolSize: c.PoolSize, Tags: c.Tags, Enabled: c.Enabled,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		HasPassword: c.Password != "",
	}
}

// dbAgentAvailable returns true once the App has been wired with a
// dbconfig.Store. The Vue UI calls this before rendering the feature
// card so SaaS-mode users see a clean "feature unavailable" state.
func (a *App) dbAgentAvailable() bool {
	return a.dbConfigs != nil && a.dbPool != nil
}

// ListDBConnections returns every registered DB, sorted by name. The
// returned slice is JSON-safe (no encrypted blobs).
func (a *App) ListDBConnections(ctx context.Context) ([]DBConnectionView, error) {
	if !a.dbAgentAvailable() {
		return nil, errors.New("dbagent: not configured in this build mode")
	}
	all, err := a.dbConfigs.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]DBConnectionView, 0, len(all))
	for _, c := range all {
		out = append(out, toView(c))
	}
	return out, nil
}

// GetDBConnection returns one connection by ID.
func (a *App) GetDBConnection(ctx context.Context, id string) (DBConnectionView, error) {
	if !a.dbAgentAvailable() {
		return DBConnectionView{}, errors.New("dbagent: not configured")
	}
	c, err := a.dbConfigs.Get(ctx, id)
	if err != nil {
		return DBConnectionView{}, err
	}
	return toView(c), nil
}

// UpsertDBConnection creates or updates a connection. On update with
// an empty Password the existing stored password is preserved — this
// lets the UI re-save other fields without forcing the user to retype
// their credentials each time.
func (a *App) UpsertDBConnection(ctx context.Context, in DBConnectionInput) (DBConnectionView, error) {
	if !a.dbAgentAvailable() {
		return DBConnectionView{}, errors.New("dbagent: not configured")
	}

	cfg := dbconfig.DBConnection{
		ID: in.ID, Name: in.Name, DBType: dbconfig.DBType(in.DBType),
		Host: in.Host, Port: in.Port, User: in.User, Password: in.Password,
		DBName: in.DBName, SSLMode: dbconfig.SSLMode(in.SSLMode),
		PoolSize: in.PoolSize, Tags: in.Tags, Enabled: in.Enabled,
	}

	// Preserve existing password if the UI sent empty (edit-without-retype).
	if in.ID != "" && in.Password == "" {
		if existing, err := a.dbConfigs.Get(ctx, in.ID); err == nil {
			cfg.Password = existing.Password
		}
	}

	saved, err := a.dbConfigs.Upsert(ctx, cfg)
	if err != nil {
		return DBConnectionView{}, err
	}
	// Invalidate any cached pool so the next analysis call rebuilds
	// with the updated config. Close is idempotent.
	_ = a.dbPool.Close(saved.ID)
	return toView(saved), nil
}

// DeleteDBConnection removes a connection. Idempotent: deleting a
// missing ID returns nil so the UI doesn't choke on double-clicks.
func (a *App) DeleteDBConnection(ctx context.Context, id string) error {
	if !a.dbAgentAvailable() {
		return errors.New("dbagent: not configured")
	}
	_ = a.dbPool.Close(id)
	if err := a.dbConfigs.Delete(ctx, id); err != nil && !errors.Is(err, dbconfig.ErrNotFound) {
		return err
	}
	return nil
}

// DBConnectionTestResult is what the "Test connection" button gets
// back. Latency is wall-clock from sql.Open through Ping.
type DBConnectionTestResult struct {
	OK       bool   `json:"ok"`
	Message  string `json:"message"`
	LatencyMillis int64 `json:"latency_ms"`
}

// TestDBConnection opens (or reuses) the pool for one connection and
// runs Ping. Useful as a smoke test from the settings form before the
// user saves credentials they got wrong.
func (a *App) TestDBConnection(ctx context.Context, id string) DBConnectionTestResult {
	if !a.dbAgentAvailable() {
		return DBConnectionTestResult{OK: false, Message: "dbagent: not configured"}
	}
	start := nowMillis()
	if _, _, err := a.dbPool.Get(ctx, id); err != nil {
		return DBConnectionTestResult{OK: false, Message: err.Error(), LatencyMillis: nowMillis() - start}
	}
	return DBConnectionTestResult{OK: true, Message: "ok", LatencyMillis: nowMillis() - start}
}

// AnalyzeDB runs an analyzer section against a registered DB. Section
// values: "overview" (default), "resources", "connections", "indexes",
// "queries", "replication". Unsupported db_types return an error.
func (a *App) AnalyzeDB(ctx context.Context, id, section string) (map[string]interface{}, error) {
	if !a.dbAgentAvailable() {
		return nil, errors.New("dbagent: not configured")
	}
	db, cfg, err := a.dbPool.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	switch cfg.DBType {
	case dbconfig.DBPostgres:
		az := postgres.New(db, 0)
		data, err := az.Analyze(ctx, section)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"connection": cfg.Name,
			"db_type":    string(cfg.DBType),
			"section":    section,
			"data":       data,
		}, nil
	case dbconfig.DBSQLite:
		az := sqlite.New(db, 0)
		data, err := az.Analyze(ctx, section)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"connection": cfg.Name,
			"db_type":    string(cfg.DBType),
			"section":    section,
			"data":       data,
		}, nil
	case dbconfig.DBClickHouse:
		az := clickhouse.New(db, 0)
		data, err := az.Analyze(ctx, section)
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"connection": cfg.Name,
			"db_type":    string(cfg.DBType),
			"section":    section,
			"data":       data,
		}, nil
	}
	return nil, fmt.Errorf("dbagent: analyzer for %s is not in this build", cfg.DBType)
}

// nowMillis is a tiny helper. Var-injected so a test can freeze time.
var nowMillis = func() int64 {
	return time.Now().UnixMilli()
}
