package database

import (
	"testing"
)

func TestSelectQueryBuilder(t *testing.T) {
	tests := []struct {
		name     string
		builder  *QueryBuilder
		wantSQL  string
		wantArgs int
	}{
		{
			name:    "select all from table",
			builder: Select("api_specs"),
			wantSQL: "SELECT * FROM api_specs",
		},
		{
			name:    "select specific columns",
			builder: Select("endpoints", "id", "method", "path"),
			wantSQL: "SELECT id, method, path FROM endpoints",
		},
		{
			name:    "select with where clause",
			builder: Select("api_specs", "id", "name").Where("id", OpEq, "abc-123"),
			wantSQL: "SELECT id, name FROM api_specs WHERE id = $1",
			wantArgs: 1,
		},
		{
			name:    "select with multiple where clauses",
			builder: Select("endpoints").Where("method", OpEq, "GET").Where("path", OpEq, "/users"),
			wantSQL: "SELECT * FROM endpoints WHERE method = $1 AND path = $2",
			wantArgs: 2,
		},
		{
			name:    "select with order by",
			builder: Select("drift_reports", "id").OrderBy("created_at", false),
			wantSQL: "SELECT id FROM drift_reports ORDER BY created_at DESC",
		},
		{
			name:    "select with limit and offset",
			builder: Select("api_specs").Limit(10).Offset(20),
			wantSQL: "SELECT * FROM api_specs LIMIT 10 OFFSET 20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.builder.Build()
			if sql != tt.wantSQL {
				t.Errorf("Build() SQL = %q, want %q", sql, tt.wantSQL)
			}
			if tt.wantArgs > 0 && len(args) != tt.wantArgs {
				t.Errorf("Build() args = %d, want %d", len(args), tt.wantArgs)
			}
		})
	}
}

func TestInsertQueryBuilder(t *testing.T) {
	tests := []struct {
		name    string
		builder *QueryBuilder
		wantSQL string
	}{
		{
			name:    "insert into table",
			builder: InsertInto("api_specs", "id", "name", "version"),
			wantSQL: "INSERT INTO api_specs (id, name, version) VALUES ($1, $2, $3)",
		},
		{
			name:    "insert with on conflict do nothing",
			builder: InsertInto("endpoints", "id", "method").OnConflict("id"),
			wantSQL: "INSERT INTO endpoints (id, method) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING",
		},
		{
			name:    "insert with on conflict do update",
			builder: InsertInto("endpoints", "id", "method", "path").OnConflict("id").DoUpdate("method", "path"),
			wantSQL: "INSERT INTO endpoints (id, method, path) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET method = EXCLUDED.method, path = EXCLUDED.path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, _ := tt.builder.Build()
			if sql != tt.wantSQL {
				t.Errorf("Build() SQL = %q, want %q", sql, tt.wantSQL)
			}
		})
	}
}

func TestUpdateQueryBuilder(t *testing.T) {
	q := Update("endpoints", "method", "path").Where("id", OpEq, "ep-1")
	sql, args := q.Build()

	wantSQL := "UPDATE endpoints SET method = $1, path = $2 WHERE id = $3"
	if sql != wantSQL {
		t.Errorf("Build() SQL = %q, want %q", sql, wantSQL)
	}

	if len(args) != 1 {
		t.Errorf("Build() args = %d, want 1", len(args))
	}
}

func TestDeleteQueryBuilder(t *testing.T) {
	q := DeleteFrom("drift_reports").Where("id", OpEq, "dr-1")
	sql, args := q.Build()

	wantSQL := "DELETE FROM drift_reports WHERE id = $1"
	if sql != wantSQL {
		t.Errorf("Build() SQL = %q, want %q", sql, wantSQL)
	}

	if len(args) != 1 {
		t.Errorf("Build() args = %d, want 1", len(args))
	}
}

func TestWhereRaw(t *testing.T) {
	q := Select("endpoints").WhereRaw("embedding IS NOT NULL")
	sql, args := q.Build()

	wantSQL := "SELECT * FROM endpoints WHERE embedding IS NOT NULL"
	if sql != wantSQL {
		t.Errorf("Build() SQL = %q, want %q", sql, wantSQL)
	}

	if len(args) != 0 {
		t.Errorf("Build() args = %d, want 0", len(args))
	}
}

func TestGroupBy(t *testing.T) {
	q := Select("drift_reports", "category", "COUNT(*)").
		Where("resolved", OpEq, false).
		GroupBy("category")

	sql, _ := q.Build()

	wantSQL := "SELECT category, COUNT(*) FROM drift_reports WHERE resolved = $1 GROUP BY category"
	if sql != wantSQL {
		t.Errorf("Build() SQL = %q, want %q", sql, wantSQL)
	}
}
