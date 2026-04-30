package tools

import (
	"context"
	"fmt"
	"github.com/djocham/kube-watcher/mcp/monitoring/recommendation"
	"time"

	"github.com/djocham/kube-watcher/pkg/kube"
	kwatch "github.com/djocham/kube-watcher/pkg/kube/watch"
)

type RecommendationTool struct {
	BaseTool
	engine *recommendation.Engine
}

func NewRecommendationTool(k8sManager kube.ClientInterface, engine *recommendation.Engine) *RecommendationTool {
	return &RecommendationTool{
		BaseTool: NewBaseTool(k8sManager),
		engine:   engine,
	}
}

func (t *RecommendationTool) Name() string {
	return "recommend_cluster_fix"
}

func (t *RecommendationTool) Description() string {
	return "Generate actionable remediation guidance and troubleshooting steps for a specific cluster alert."
}

func (t *RecommendationTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "kind", Type: "string", Description: "The alert category: 'node', 'pod', or 'event'."},
		{Name: "name", Type: "string", Description: "The name of the affected Kubernetes resource."},
		{Name: "namespace", Type: "string", Description: "Namespace of the resource (leave empty for nodes)."},
		{Name: "severity", Type: "string", Description: "Severity level: 'warning' or 'critical'."},
		{Name: "reason", Type: "string", Description: "The machine-readable reason (e.g., CrashLoopBackOff)."},
		{Name: "message", Type: "string", Description: "The human-readable log or status message."},
	}
}

func (t *RecommendationTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	kindStr := t.GetStringArg(args, "kind", "")
	alert := kwatch.Alert{
		Kind:       kwatch.AlertKind(kindStr),
		Name:       t.GetStringArg(args, "name", ""),
		Namespace:  t.GetStringArg(args, "namespace", ""),
		Severity:   t.GetStringArg(args, "severity", ""),
		Reason:     t.GetStringArg(args, "reason", ""),
		Message:    t.GetStringArg(args, "message", ""),
		OccurredAt: time.Now(),
	}

	if alert.Kind == "" || alert.Name == "" {
		return nil, fmt.Errorf("missing required fields: 'kind' and 'name' must be provided")
	}

	rec, err := t.engine.ForAlert(ctx, alert)
	if err != nil {
		return nil, fmt.Errorf("failed to generate recommendation: %w", err)
	}

	return map[string]interface{}{
		"recommendation": map[string]interface{}{
			"title":             rec.Title,
			"summary":           rec.Summary,
			"severity":          rec.Severity,
			"remediation_steps": rec.Steps,
			"evidence":          rec.Evidence,
			"stats": map[string]interface{}{
				"frequency_delta": rec.FrequencyDelta,
				"recent_count":    rec.LastSeenCount,
			},
		},
	}, nil
}
