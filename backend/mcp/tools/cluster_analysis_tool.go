package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/djocham/kube-watcher/pkg/kube"
)

type ClusterAnalysisTool struct {
	BaseTool
}

type nodeHealthSummary struct {
	Status     string
	ReadyNodes int
	Issues     []string
}

type podHealthSummary struct {
	Total        int
	Running      int
	Pending      int
	Failed       int
	Succeeded    int
	Unknown      int
	HighRestarts []string
	Problematic  []string
}

type eventHealthSummary struct {
	RecentTotal   int
	RecentWarning int
	TopReasons    []map[string]any
}

type serviceHealthSummary struct {
	Total         int
	TypeBreakdown map[string]int
	HeadlessCount int
}

func NewClusterAnalysisTool(k8sManager kube.ClientInterface) *ClusterAnalysisTool {
	return &ClusterAnalysisTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *ClusterAnalysisTool) Name() string {
	return "analyze_cluster"
}

func (t *ClusterAnalysisTool) Description() string {
	return "Comprehensive cluster analysis with health scoring and recommendations."
}

func (t *ClusterAnalysisTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "include_pods", Type: "boolean", Default: true},
		{Name: "include_events", Type: "boolean", Default: true},
		{Name: "event_hours_back", Type: "number", Default: 24},
		{Name: "include_services", Type: "boolean", Default: false},
		{Name: "detailed_analysis", Type: "boolean", Default: true},
	}
}

func (t *ClusterAnalysisTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	includePods := t.GetBoolArg(args, "include_pods", true)
	includeEvents := t.GetBoolArg(args, "include_events", true)
	eventHoursBack := t.GetIntArg(args, "event_hours_back", 24)
	includeServices := t.GetBoolArg(args, "include_services", false)
	detailedAnalysis := t.GetBoolArg(args, "detailed_analysis", true)

	info, err := t.K8sManager.GetClusterInfo(ctx)
	if err != nil {
		return nil, err
	}

	nodes, err := t.K8sManager.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	nodeSummary := t.evaluateNodes(nodes)
	var (
		podSummary     *podHealthSummary
		eventSummary   *eventHealthSummary
		serviceSummary *serviceHealthSummary
	)

	if includePods {
		pods, podErr := t.K8sManager.GetPodsAllNamespaces(ctx)
		if podErr != nil {
			return nil, podErr
		}
		podSummary = t.evaluatePods(pods)
	}

	if includeEvents {
		events, eventsErr := t.K8sManager.GetEventsAllNamespaces(ctx)
		if eventsErr != nil {
			return nil, eventsErr
		}
		eventSummary = t.evaluateEvents(events, eventHoursBack)
	}

	if includeServices {
		services, svcErr := t.K8sManager.GetServicesAllNamespaces(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		serviceSummary = t.evaluateServices(services)
	}

	problematicPods := 0
	if podSummary != nil {
		problematicPods = len(podSummary.Problematic)
	}
	warningEvents := 0
	if eventSummary != nil {
		warningEvents = eventSummary.RecentWarning
	}

	result := map[string]any{
		"cluster_info": info,
		"health": map[string]any{
			"status":      nodeSummary.Status,
			"ready_nodes": fmt.Sprintf("%d/%d", nodeSummary.ReadyNodes, len(nodes)),
			"issues":      nodeSummary.Issues,
			"score":       t.scoreCluster(len(nodes), nodeSummary.ReadyNodes, problematicPods, warningEvents),
		},
	}

	if podSummary != nil {
		result["pods"] = map[string]any{
			"total":         podSummary.Total,
			"running":       podSummary.Running,
			"pending":       podSummary.Pending,
			"failed":        podSummary.Failed,
			"succeeded":     podSummary.Succeeded,
			"unknown":       podSummary.Unknown,
			"high_restarts": podSummary.HighRestarts,
			"problematic":   podSummary.Problematic,
		}
	}

	if eventSummary != nil {
		result["events"] = map[string]any{
			"hours_back":     eventHoursBack,
			"recent_total":   eventSummary.RecentTotal,
			"recent_warning": eventSummary.RecentWarning,
			"top_reasons":    eventSummary.TopReasons,
		}
	}

	if serviceSummary != nil {
		result["services"] = map[string]any{
			"total":             serviceSummary.Total,
			"type_breakdown":    serviceSummary.TypeBreakdown,
			"headless_services": serviceSummary.HeadlessCount,
		}
	}

	if detailedAnalysis {
		result["recommendations"] = t.buildRecommendations(nodeSummary, podSummary, eventSummary, serviceSummary)
	}

	return result, nil
}

func (t *ClusterAnalysisTool) evaluateNodes(nodes []kube.NodeInfo) nodeHealthSummary {
	issues := make([]string, 0)
	readyCount := 0

	for _, n := range nodes {
		if n.Status == "Ready" {
			readyCount++
			continue
		}
		issues = append(issues, fmt.Sprintf("Node %s is %s", n.Name, n.Status))
	}

	status := "healthy"
	if readyCount < len(nodes) {
		status = "degraded"
	}
	return nodeHealthSummary{
		Status:     status,
		ReadyNodes: readyCount,
		Issues:     issues,
	}
}

func (t *ClusterAnalysisTool) evaluatePods(pods []kube.PodInfo) *podHealthSummary {
	out := &podHealthSummary{Total: len(pods)}

	for _, p := range pods {
		switch strings.ToLower(string(p.Phase)) {
		case "running":
			out.Running++
		case "pending":
			out.Pending++
		case "failed":
			out.Failed++
		case "succeeded":
			out.Succeeded++
		default:
			out.Unknown++
		}

		if p.RestartCount >= 5 {
			out.HighRestarts = append(out.HighRestarts, fmt.Sprintf("%s/%s", p.Namespace, p.Name))
		}

		if p.Phase == "Pending" || p.Phase == "Failed" || p.RestartCount >= 5 {
			out.Problematic = append(out.Problematic, fmt.Sprintf("%s/%s", p.Namespace, p.Name))
		}
	}

	return out
}

func (t *ClusterAnalysisTool) evaluateEvents(events []kube.EventInfo, hoursBack int) *eventHealthSummary {
	cutoff := time.Now().Add(-time.Duration(hoursBack) * time.Hour)
	reasonCounts := map[string]int{}
	out := &eventHealthSummary{}

	for _, e := range events {
		last := e.LastTimestamp
		if last.IsZero() {
			last = e.FirstTimestamp
		}
		if !last.IsZero() && last.Before(cutoff) {
			continue
		}

		out.RecentTotal++
		if strings.EqualFold(e.Type, "Warning") {
			out.RecentWarning++
		}
		reasonCounts[e.Reason]++
	}

	out.TopReasons = topReasonCounts(reasonCounts, 5)
	return out
}

func (t *ClusterAnalysisTool) evaluateServices(services []kube.ServiceInfo) *serviceHealthSummary {
	out := &serviceHealthSummary{
		Total:         len(services),
		TypeBreakdown: map[string]int{},
	}
	for _, svc := range services {
		out.TypeBreakdown[strings.ToLower(string(svc.Type))]++
		if strings.EqualFold(svc.ClusterIP, "none") {
			out.HeadlessCount++
		}
	}
	return out
}

func (t *ClusterAnalysisTool) scoreCluster(totalNodes, readyNodes, problematicPods, warningEvents int) int {
	score := 100
	if totalNodes > 0 {
		readyPct := float64(readyNodes) / float64(totalNodes)
		switch {
		case readyPct < 0.8:
			score -= 40
		case readyPct < 1:
			score -= 20
		}
	}
	if problematicPods > 0 {
		score -= minInt(20, problematicPods*2)
	}
	if warningEvents > 0 {
		score -= minInt(20, warningEvents/5)
	}
	if score < 0 {
		return 0
	}
	return score
}

func (t *ClusterAnalysisTool) buildRecommendations(
	nodes nodeHealthSummary,
	pods *podHealthSummary,
	events *eventHealthSummary,
	services *serviceHealthSummary,
) []string {
	recs := make([]string, 0)
	if nodes.Status != "healthy" {
		recs = append(recs, "Investigate non-ready nodes and resolve scheduling/pressure conditions.")
	}
	if pods != nil && len(pods.Problematic) > 0 {
		recs = append(recs, fmt.Sprintf("Investigate %d problematic pods (pending/failed/high restarts).", len(pods.Problematic)))
	}
	if events != nil && events.RecentWarning > 0 {
		recs = append(recs, fmt.Sprintf("Review recent warning events (%d) for cluster instability patterns.", events.RecentWarning))
	}
	if services != nil && services.HeadlessCount > 20 {
		recs = append(recs, "Large number of headless services detected; review service discovery and DNS load.")
	}
	if len(recs) == 0 {
		recs = append(recs, "Cluster appears healthy based on current analysis scope.")
	}
	return recs
}

func topReasonCounts(input map[string]int, maxItems int) []map[string]any {
	type pair struct {
		reason string
		count  int
	}

	pairs := make([]pair, 0, len(input))
	for reason, count := range input {
		if strings.TrimSpace(reason) == "" {
			continue
		}
		pairs = append(pairs, pair{reason: reason, count: count})
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].count == pairs[j].count {
			return pairs[i].reason < pairs[j].reason
		}
		return pairs[i].count > pairs[j].count
	})

	limit := minInt(maxItems, len(pairs))
	out := make([]map[string]any, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, map[string]any{
			"reason": pairs[i].reason,
			"count":  pairs[i].count,
		})
	}
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
