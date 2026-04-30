package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/djocham/kube-watcher/pkg/kube"
)

var ErrFailedToGetClusterEvents = errors.New("failed to get cluster events")

type ClusterEventsTool struct {
	BaseTool
	logger *slog.Logger
}

func NewClusterEventsTool(k8sManager kube.ClientInterface, logger *slog.Logger) *ClusterEventsTool {
	return &ClusterEventsTool{BaseTool: NewBaseTool(k8sManager), logger: logger}
}

func (t *ClusterEventsTool) Name() string        { return "get_cluster_events" }
func (t *ClusterEventsTool) Description() string { return "Get and analyze recent cluster events" }

func (t *ClusterEventsTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "namespace", Type: "string", Description: "Specific namespace (optional)"},
		{Name: "limit", Type: "number", Description: "Max number of events to return"},
		{Name: "event_type", Type: "string", Description: "Filter by Warning, Normal, or all"},
		{Name: "hours_back", Type: "number", Description: "Hours back to look"},
	}
}

func (t *ClusterEventsTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	ns := t.GetStringArg(args, "namespace", "")
	limit := float64(t.GetIntArg(args, "limit", 50))
	evType := t.GetStringArg(args, "event_type", "all")
	hours := t.getFloat(args, "hours_back", 24.0)

	events, err := t.fetchEvents(ctx, ns)
	if err != nil {
		t.logger.Error("failed to get events", "error", err)
		return nil, ErrFailedToGetClusterEvents
	}

	filtered := t.filterAndSort(events, evType, hours)

	n := int(limit)
	if n > len(filtered) {
		n = len(filtered)
	}

	return map[string]any{
		"summary":      t.analyzeEvents(filtered),
		"events":       t.formatEvents(filtered[:n], 20),
		"total_found":  len(filtered),
		"filter_range": hours,
	}, nil
}

func (t *ClusterEventsTool) fetchEvents(ctx context.Context, ns string) ([]kube.EventInfo, error) {
	if ns == "" {
		return t.K8sManager.GetEventsAllNamespaces(ctx)
	}
	return t.K8sManager.GetEvents(ctx, ns)
}

func (t *ClusterEventsTool) filterAndSort(events []kube.EventInfo, evType string, hours float64) []kube.EventInfo {
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	isAll := strings.EqualFold(evType, "all")

	var res []kube.EventInfo
	for _, e := range events {
		if e.LastTimestamp.After(cutoff) && (isAll || strings.EqualFold(e.Type, evType)) {
			res = append(res, e)
		}
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].LastTimestamp.After(res[j].LastTimestamp)
	})
	return res
}

func (t *ClusterEventsTool) analyzeEvents(events []kube.EventInfo) map[string]any {
	if len(events) == 0 {
		return map[string]any{"status": "no events found"}
	}

	reasons := make(map[string]int)
	groups := make(map[string]int)
	warnings := 0

	for _, e := range events {
		reasons[e.Reason]++
		groups[e.Reason]++
		if strings.EqualFold(e.Type, "Warning") {
			warnings++
		}
	}

	patterns := t.findPatterns(groups)
	return map[string]any{
		"total":           len(events),
		"warnings":        warnings,
		"top_reasons":     reasons,
		"patterns":        patterns,
		"recommendations": t.generateRecommendations(patterns, warnings),
		"frequency":       t.calculateFrequency(events),
	}
}

func (t *ClusterEventsTool) findPatterns(groups map[string]int) []map[string]any {
	var patterns []map[string]any
	for reason, count := range groups {
		if count >= 3 {
			patterns = append(patterns, map[string]any{
				"reason": reason,
				"count":  count,
				"type":   "frequent_event",
			})
		}
	}
	return patterns
}

func (t *ClusterEventsTool) generateRecommendations(patterns []map[string]any, warnings int) []string {
	recs := []string{}
	if warnings > 0 {
		recs = append(recs, "review individual Warning events for critical failures.")
	}
	for _, p := range patterns {
		recs = append(recs, fmt.Sprintf("investigate repeated '%s' events.", p["reason"]))
	}
	return recs
}

func (t *ClusterEventsTool) calculateFrequency(events []kube.EventInfo) string {
	if len(events) < 10 {
		return "stable"
	}
	mid := len(events) / 2
	recentRate := events[0].LastTimestamp.Sub(events[mid].LastTimestamp)
	priorRate := events[mid].LastTimestamp.Sub(events[len(events)-1].LastTimestamp)

	if recentRate < priorRate {
		return "increasing"
	}
	return "stable"
}

func (t *ClusterEventsTool) formatEvents(events []kube.EventInfo, max int) []map[string]any {
	limit := len(events)
	if limit > max {
		limit = max
	}

	out := make([]map[string]any, limit)
	for i := 0; i < limit; i++ {
		e := events[i]
		out[i] = map[string]any{
			"type":      e.Type,
			"reason":    e.Reason,
			"object":    fmt.Sprintf("%s/%s", e.ObjectKind, e.ObjectName),
			"message":   e.Message,
			"timestamp": e.LastTimestamp.Format(time.RFC3339),
			"count":     e.Count,
		}
	}
	return out
}

func (t *ClusterEventsTool) getFloat(args map[string]any, key string, fallback float64) float64 {
	if val, ok := args[key].(float64); ok && val != 0 {
		return val
	}
	return fallback
}
