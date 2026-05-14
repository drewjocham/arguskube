package dbtools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/argues/argus/internal/dbagent/connector"
	"github.com/argues/argus/internal/dbagent/sqlite"
	"github.com/argues/argus/internal/dbconfig"
)

// SQLiteAnalyzeTool is the MCP front for the SQLite DBAgent analyzer.
type SQLiteAnalyzeTool struct {
	pool *connector.Pool
}

func NewSQLiteAnalyzeTool(pool *connector.Pool) *SQLiteAnalyzeTool {
	return &SQLiteAnalyzeTool{pool: pool}
}

func (t *SQLiteAnalyzeTool) Name() string { return "db_analyze_sqlite" }

func (t *SQLiteAnalyzeTool) Description() string {
	return "Analyze a registered SQLite database. Sections: overview, resources, connections, indexes, queries, replication. Some sections (queries, replication) are not applicable and return a hint."
}

func (t *SQLiteAnalyzeTool) Parameters() []ToolParameter {
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

func (t *SQLiteAnalyzeTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	id := getString(args, "connection_id", "")
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("db_analyze_sqlite: connection_id is required")
	}
	section := getString(args, "section", "overview")
	timeoutSec := getInt(args, "timeout_seconds", 10)

	db, cfg, err := t.pool.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if cfg.DBType != dbconfig.DBSQLite {
		return nil, fmt.Errorf("db_analyze_sqlite: connection %q is %s, not sqlite", cfg.Name, cfg.DBType)
	}

	a := sqlite.New(db, time.Duration(timeoutSec)*time.Second)
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
