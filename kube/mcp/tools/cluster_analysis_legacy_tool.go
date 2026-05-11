package tools

import (
	"context"
)

type ClusterAnalysisLegacyTool struct {
	delegate *ClusterAnalysisTool
}

func NewClusterAnalysisLegacyTool(delegate *ClusterAnalysisTool) *ClusterAnalysisLegacyTool {
	return &ClusterAnalysisLegacyTool{delegate: delegate}
}

func (t *ClusterAnalysisLegacyTool) Name() string {
	return "analyze_cluster_health"
}

func (t *ClusterAnalysisLegacyTool) Description() string {
	return "Legacy alias for analyze_cluster."
}

func (t *ClusterAnalysisLegacyTool) Parameters() []ToolParameter {
	return t.delegate.Parameters()
}

func (t *ClusterAnalysisLegacyTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return t.delegate.Execute(ctx, args)
}
