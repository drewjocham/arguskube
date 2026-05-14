package dbtools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/argues/argus/internal/dbagent/connector"
	"github.com/argues/argus/internal/dbagent/postgres"
	"github.com/argues/argus/internal/dbconfig"
)

// PostgresAnalyzeTool is the MCP front for the PostgreSQL DBAgent
// analyzer. One MCP call → one analysis section → a JSON blob that the
// Python agent reads and renders into a recommendation.
//
// The tool keeps no state of its own; the connection pool lives in
// connector.Pool, which is shared with future db_* tools so all
// dialects use the same cache.
type PostgresAnalyzeTool struct {
	pool *connector.Pool
}

func NewPostgresAnalyzeTool(pool *connector.Pool) *PostgresAnalyzeTool {
	return &PostgresAnalyzeTool{pool: pool}
}

func (t *PostgresAnalyzeTool) Name() string { return "db_analyze_postgres" }

func (t *PostgresAnalyzeTool) Description() string {
	return "Analyze a registered PostgreSQL database. Sections: overview, resources, connections, indexes, queries, replication. Returns raw metrics — the agent ranks/recommends."
}

func (t *PostgresAnalyzeTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "connection_id", Type: "string", Required: true,
			Description: "ID of the registered DB connection (from dbconfig.Store)."},
		{Name: "section", Type: "string", Required: false,
			Description: "overview (default) | resources | connections | indexes | queries | replication",
			Default:     "overview"},
		{Name: "timeout_seconds", Type: "integer", Required: false,
			Description: "Per-query timeout. Default 10.", Default: 10},
	}
}

func (t *PostgresAnalyzeTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	id := getString(args, "connection_id", "")
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("db_analyze_postgres: connection_id is required")
	}
	section := getString(args, "section", "overview")
	timeoutSec := getInt(args, "timeout_seconds", 10)

	db, cfg, err := t.pool.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if cfg.DBType != dbconfig.DBPostgres {
		return nil, fmt.Errorf("db_analyze_postgres: connection %q is %s, not postgres", cfg.Name, cfg.DBType)
	}

	a := postgres.New(db, time.Duration(timeoutSec)*time.Second)
	result, err := a.Analyze(ctx, section)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"connection": cfg.Name,
		"db_type":    string(cfg.DBType),
		"section":    section,
		"collected":  time.Now().UTC().Format(time.RFC3339),
		"data":       result,
	}, nil
}
