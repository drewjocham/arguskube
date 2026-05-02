package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/argues/kube-watcher/mcp/monitoring/history"
	"github.com/argues/kube-watcher/pkg/kube"
)

type HistoryInsightsTool struct {
	BaseTool
	store history.Recorder
}

func NewHistoryInsightsTool(store history.Recorder) *HistoryInsightsTool {
	return &HistoryInsightsTool{
		BaseTool: NewBaseTool(nil),
		store:    store,
	}
}

func NewHistoryInsightsToolWithClient(k8sManager kube.ClientInterface, store history.Recorder) *HistoryInsightsTool {
	return &HistoryInsightsTool{
		BaseTool: NewBaseTool(k8sManager),
		store:    store,
	}
}

func (t *HistoryInsightsTool) Name() string { return "list_repeating_issues" }
func (t *HistoryInsightsTool) Description() string {
	return "Analyze incident history to identify trends and recurring anomalies."
}

func (t *HistoryInsightsTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "kind", Type: "string", Description: "Filter: node_anomaly, pod_anomaly, or event_spike."},
		{Name: "since_hours", Type: "number", Default: 24},
		{Name: "severity", Type: "string", Description: "Filter: critical, high, low."},
		{Name: "limit", Type: "number", Default: 20},
	}
}

func (t *HistoryInsightsTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	if err := validateNumericArg(args, "limit"); err != nil {
		return nil, err
	}
	if err := validateNumericArg(args, "since_hours"); err != nil {
		return nil, err
	}
	sinceHours := t.getFloat64(args, "since_hours", 24.0)
	limit := t.GetIntArg(args, "limit", 20)
	kind := history.IssueKind(t.GetStringArg(args, "kind", ""))
	severity := t.GetStringArg(args, "severity", "")

	window := time.Duration(sinceHours) * time.Hour

	allIncidents, err := t.loadIncidents(ctx, kind, window)
	if err != nil {
		return nil, fmt.Errorf("loading incidents: %w", err)
	}

	filtered := t.filterIncidents(allIncidents, severity, limit)

	comparison, err := t.computeFrequency(ctx, kind, window)
	if err != nil {
		return nil, fmt.Errorf("computing frequency: %w", err)
	}

	return map[string]any{
		"query_context": map[string]any{
			"kind":        kind,
			"since_hours": sinceHours,
			"severity":    severity,
		},
		"results": map[string]any{
			"count":           len(filtered),
			"incidents":       filtered,
			"frequency_trend": comparison,
		},
		"insight": t.generateInsight(comparison),
	}, nil
}

func (t *HistoryInsightsTool) loadIncidents(ctx context.Context, kind history.IssueKind, window time.Duration) ([]history.Incident, error) {
	if kind != "" {
		return t.store.List(ctx, kind, window)
	}
	var all []history.Incident
	for _, k := range history.SupportedKinds {
		incidents, err := t.store.List(ctx, k, window)
		if err != nil {
			return nil, err
		}
		all = append(all, incidents...)
	}
	return all, nil
}

func (t *HistoryInsightsTool) filterIncidents(incidents []history.Incident, severity string, limit int) []history.Incident {
	out := make([]history.Incident, 0)
	for _, inc := range incidents {
		if severity != "" && !strings.EqualFold(inc.Severity, severity) {
			continue
		}
		out = append(out, inc)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (t *HistoryInsightsTool) computeFrequency(ctx context.Context, kind history.IssueKind, window time.Duration) (history.FrequencyComparison, error) {
	if kind != "" {
		return t.store.CompareFrequency(ctx, kind, window, window)
	}

	combined := history.FrequencyComparison{
		Kind:             "all",
		WindowHours:      window.Hours(),
		PreviousWindowHr: window.Hours(),
	}

	for _, k := range history.SupportedKinds {
		f, err := t.store.CompareFrequency(ctx, k, window, window)
		if err != nil {
			return combined, err
		}
		combined.RecentCount += f.RecentCount
		combined.PreviousCount += f.PreviousCount
	}

	combined.PercentChange = history.CalcChange(combined.PreviousCount, combined.RecentCount)
	return combined, nil
}

func (t *HistoryInsightsTool) generateInsight(f history.FrequencyComparison) string {
	switch {
	case f.RecentCount == 0:
		return "No issues detected in the current window."
	case f.PercentChange > 50:
		return "Critical: Significant spike in issue frequency detected compared to previous window."
	case f.PercentChange > 0:
		return "Warning: Issues are trending upward."
	default:
		return "Issue frequency is stable or declining."
	}
}

func validateNumericArg(args map[string]any, key string) error {
	v, ok := args[key]
	if !ok {
		return nil
	}
	switch v.(type) {
	case float64, float32, int, int32, int64:
		return nil
	default:
		return fmt.Errorf("invalid type for %q: expected number", key)
	}
}

func (t *HistoryInsightsTool) getFloat64(args map[string]any, key string, fallback float64) float64 {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return fallback
	}
}
