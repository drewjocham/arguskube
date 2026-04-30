package tools

import (
	"context"

	"github.com/djocham/kube-watcher/pkg/kube"
)

type ToolParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

type BaseTool struct {
	K8sManager kube.ClientInterface
}

func NewBaseTool(k8sManager kube.ClientInterface) BaseTool {
	return BaseTool{K8sManager: k8sManager}
}

func (bt BaseTool) GetBoolArg(args map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}
	return defaultVal
}

func (bt BaseTool) GetIntArg(args map[string]interface{}, key string, defaultVal int) int {
	if val, ok := args[key].(float64); ok {
		return int(val)
	}
	if val, ok := args[key].(int); ok {
		return val
	}
	return defaultVal
}

func (bt BaseTool) GetStringArg(args map[string]interface{}, key string, defaultVal string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return defaultVal
}

func (bt BaseTool) Execute(ctx context.Context, fn func(context.Context) (map[string]interface{}, error)) (map[string]interface{}, error) {
	return fn(ctx)
}
