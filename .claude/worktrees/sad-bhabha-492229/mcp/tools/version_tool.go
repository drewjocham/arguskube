package tools

import (
	"context"
	"runtime"
	"time"
)

type VersionTool struct {
	version   string
	gitCommit string
	buildDate string
}

func NewVersionTool(version, gitCommit, buildDate string) *VersionTool {
	return &VersionTool{
		version:   version,
		gitCommit: gitCommit,
		buildDate: buildDate,
	}
}

func (t *VersionTool) Name() string {
	return "get_server_version"
}

func (t *VersionTool) Description() string {
	return "Return kube-watcher build metadata."
}

func (t *VersionTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "include_build_metadata", Type: "boolean", Description: "Include git commit/build date (default true)."},
		{Name: "include_go_version", Type: "boolean", Description: "Include Go runtime version (default false)."},
	}
}

func (t *VersionTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	includeBuild := true
	if val, ok := args["include_build_metadata"].(bool); ok {
		includeBuild = val
	}
	includeGo := false
	if val, ok := args["include_go_version"].(bool); ok {
		includeGo = val
	}

	result := map[string]interface{}{
		"name":      "kube-watcher",
		"version":   t.version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	if includeBuild {
		result["git_commit"] = t.gitCommit
		result["build_date"] = t.buildDate
	}

	if includeGo {
		result["go_version"] = goVersion()
	}

	return result, nil
}

func goVersion() string {
	return runtimeVersion
}

var runtimeVersion = func() string {
	return runtime.Version()
}()
