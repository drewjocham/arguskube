package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/djocham/kube-watcher/pkg/kube"
)

type PodResourcesTool struct {
	BaseTool
}

type PodIssueAnalysis struct {
	IsProblematic bool     `json:"is_problematic"`
	Issues        []string `json:"issues"`
}

func NewPodResourcesTool(k8sManager kube.ClientInterface) *PodResourcesTool {
	return &PodResourcesTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *PodResourcesTool) Name() string {
	return "get_pod_resources"
}

func (t *PodResourcesTool) Description() string {
	return "Get resource usage and status information for pods across the cluster."
}

func (t *PodResourcesTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "namespace", Type: "string"},
		{Name: "status_filter", Type: "string"},
		{Name: "high_restart_threshold", Type: "number", Default: 5},
		{Name: "include_containers", Type: "boolean", Default: true},
		{Name: "problematic_only", Type: "boolean", Default: false},
	}
}

func (t *PodResourcesTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	namespace := t.GetStringArg(args, "namespace", "")
	statusFilter := t.GetStringArg(args, "status_filter", "")
	threshold := t.GetIntArg(args, "high_restart_threshold", 5)
	includeContainers := t.GetBoolArg(args, "include_containers", true)
	problematicOnly := t.GetBoolArg(args, "problematic_only", false)

	var pods []kube.PodInfo
	var err error

	if namespace == "" || strings.EqualFold(namespace, "all") {
		pods, err = t.K8sManager.GetPodsAllNamespaces(ctx)
	} else {
		pods, err = t.K8sManager.GetPods(ctx, namespace)
	}

	if err != nil {
		return nil, err
	}

	var (
		processed    []map[string]interface{}
		highRestarts []string
		problematic  []string
		phaseCounts  = make(map[string]int)
		nsCounts     = make(map[string]int)
	)

	for _, pod := range pods {
		if statusFilter != "" && !strings.EqualFold(string(pod.Phase), statusFilter) {
			continue
		}

		analysis := t.analyzePod(pod, threshold)
		if problematicOnly && !analysis.IsProblematic {
			continue
		}

		phaseCounts[strings.ToLower(string(pod.Phase))]++
		nsCounts[pod.Namespace]++

		if pod.RestartCount >= threshold {
			highRestarts = append(highRestarts, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}
		if analysis.IsProblematic {
			problematic = append(problematic, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}

		detail := map[string]interface{}{
			"name":           pod.Name,
			"namespace":      pod.Namespace,
			"status":         pod.Status,
			"node":           pod.NodeName,
			"age":            pod.Age.String(),
			"restarts":       pod.RestartCount,
			"is_problematic": analysis.IsProblematic,
			"issues":         analysis.Issues,
		}

		if includeContainers {
			detail["containers"] = t.mapContainers(pod.Containers)
		}
		processed = append(processed, detail)
	}

	return map[string]interface{}{
		"summary": map[string]interface{}{
			"total":         len(processed),
			"phases":        phaseCounts,
			"problematic":   problematic,
			"high_restarts": highRestarts,
		},
		"namespaces": nsCounts,
		"pods":       processed,
		"analysis":   t.generateSummary(len(processed), phaseCounts["running"], problematic, nsCounts),
	}, nil
}

func (t *PodResourcesTool) analyzePod(pod kube.PodInfo, threshold int) PodIssueAnalysis {
	var issues []string

	if pod.RestartCount >= threshold {
		issues = append(issues, fmt.Sprintf("High restarts: %d", pod.RestartCount))
	}
	if pod.Phase != "Running" && pod.Phase != "Succeeded" {
		issues = append(issues, fmt.Sprintf("Phase: %s", pod.Phase))
	}
	if pod.NodeName == "" && pod.Phase == "Pending" {
		issues = append(issues, "Unscheduled")
	}

	for _, c := range pod.Containers {
		if !c.Ready && pod.Phase == "Running" {
			issues = append(issues, fmt.Sprintf("Container %s not ready", c.Name))
		}
		if c.State == "Waiting" {
			issues = append(issues, fmt.Sprintf("Container %s waiting", c.Name))
		}
	}

	return PodIssueAnalysis{
		IsProblematic: len(issues) > 0,
		Issues:        issues,
	}
}

func (t *PodResourcesTool) mapContainers(containers []kube.ContainerInfo) []map[string]interface{} {
	out := make([]map[string]interface{}, len(containers))
	for i, c := range containers {
		health := "healthy"
		if !c.Ready || c.State != "Running" {
			health = strings.ToLower(c.State)
			if health == "" {
				health = "unhealthy"
			}
		}
		out[i] = map[string]interface{}{
			"name":     c.Name,
			"image":    c.Image,
			"ready":    c.Ready,
			"restarts": c.RestartCount,
			"health":   health,
		}
	}
	return out
}

func (t *PodResourcesTool) generateSummary(total, running int, problematic []string, nsMap map[string]int) map[string]interface{} {
	pct := 0.0
	if total > 0 {
		pct = float64(running) / float64(total) * 100
	}

	status := "healthy"
	var recs []string

	if pct < 90 {
		status = "warning"
		recs = append(recs, "Low percentage of running pods.")
	}
	if len(problematic) > 0 {
		recs = append(recs, fmt.Sprintf("Investigate %d problematic pods.", len(problematic)))
	}
	if len(nsMap) > 20 {
		recs = append(recs, "High namespace count: verify if namespace sprawl is intentional.")
	}

	return map[string]interface{}{
		"status":            status,
		"health_percentage": pct,
		"recommendations":   recs,
	}
}
