// Package dbtools is the MCP front end for the DBAgent analyzers.
//
// The repo has an orphaned kube/mcp/ tree that no go.mod currently
// owns; rather than wait for that cleanup to land, we define the
// minimal Tool surface here so the dbagent stack can ship inside
// kube/backend's module. When kube/mcp comes back online the only
// adjustment needed is an aliasing of these types (or a move).
package dbtools

import "context"

// ToolParameter mirrors kube/mcp/tools.ToolParameter exactly so the
// future merge is mechanical.
type ToolParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// Tool is the MCP-server contract every dbtools tool implements.
type Tool interface {
	Name() string
	Description() string
	Parameters() []ToolParameter
	Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)
}

func getString(args map[string]interface{}, key, def string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return def
}

func getInt(args map[string]interface{}, key string, def int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	if v, ok := args[key].(int); ok {
		return v
	}
	return def
}
