package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/djocham/kube-watcher/pkg/kube"
)

type NodeStatusTool struct {
	BaseTool
}

type NodeMetrics struct {
	CPU      string   `json:"cpu_allocatable"`
	Memory   string   `json:"memory_allocatable"`
	Storage  string   `json:"storage_allocatable"`
	Pods     string   `json:"pods_allocatable"`
	Score    int      `json:"pressure_score_pct"`
	Pressure []string `json:"active_pressure"`
}

func NewNodeStatusTool(k8sManager kube.ClientInterface) *NodeStatusTool {
	return &NodeStatusTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *NodeStatusTool) Name() string {
	return "get_node_status"
}

func (t *NodeStatusTool) Description() string {
	return "Get detailed status and health information about cluster nodes."
}

func (t *NodeStatusTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "include_metrics", Type: "boolean", Default: true},
		{Name: "taints_only", Type: "boolean", Default: false},
		{Name: "node_name", Type: "string"},
	}
}

func (t *NodeStatusTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	includeMetrics := t.GetBoolArg(args, "include_metrics", true)
	taintsOnly := t.GetBoolArg(args, "taints_only", false)
	nodeName := t.GetStringArg(args, "node_name", "")

	var nodes []kube.NodeInfo
	if nodeName != "" {
		node, err := t.K8sManager.GetNode(ctx, nodeName)
		if err != nil {
			return nil, err
		}
		nodes = []kube.NodeInfo{*node}
	} else {
		var err error
		nodes, err = t.K8sManager.GetNodes(ctx)
		if err != nil {
			return nil, err
		}
	}

	var (
		readyCount    int
		unhealthy     []string
		tainted       []string
		processedList []map[string]interface{}
	)

	for _, node := range nodes {
		if taintsOnly && len(node.Taints) == 0 {
			continue
		}

		isReady := node.Status == "Ready"
		if isReady {
			readyCount++
		} else {
			unhealthy = append(unhealthy, node.Name)
		}

		if len(node.Taints) > 0 {
			tainted = append(tainted, node.Name)
		}

		details := t.mapNodeToResult(node, includeMetrics)
		processedList = append(processedList, details)
	}

	return map[string]interface{}{
		"summary": map[string]interface{}{
			"total":     len(nodes),
			"ready":     readyCount,
			"unhealthy": unhealthy,
			"tainted":   tainted,
		},
		"nodes":    processedList,
		"analysis": t.analyzeHealth(len(nodes), readyCount, unhealthy, tainted),
	}, nil
}

func (t *NodeStatusTool) mapNodeToResult(node kube.NodeInfo, includeMetrics bool) map[string]interface{} {
	conditions := t.parseConditions(node.Conditions)

	res := map[string]interface{}{
		"name":       node.Name,
		"status":     node.Status,
		"age":        node.Age.String(),
		"conditions": conditions,
		"labels":     node.Labels,
	}

	if len(node.Taints) > 0 {
		res["taints"] = node.Taints
	}

	if includeMetrics {
		metrics := NodeMetrics{
			CPU:     node.Allocatable["cpu"],
			Memory:  node.Allocatable["memory"],
			Storage: node.Allocatable["ephemeral-storage"],
			Pods:    node.Allocatable["pods"],
		}

		score := 0
		if conditions["memory_pressure"] == true {
			score += 50
			metrics.Pressure = append(metrics.Pressure, "Memory")
		}
		if conditions["disk_pressure"] == true {
			score += 30
			metrics.Pressure = append(metrics.Pressure, "Disk")
		}
		if conditions["pid_pressure"] == true {
			score += 20
			metrics.Pressure = append(metrics.Pressure, "PID")
		}

		metrics.Score = score
		res["utilization"] = metrics
	}

	return res
}

func (t *NodeStatusTool) parseConditions(conditions []kube.NodeCondition) map[string]interface{} {
	summary := map[string]interface{}{
		"ready":               false,
		"memory_pressure":     false,
		"disk_pressure":       false,
		"pid_pressure":        false,
		"network_unavailable": false,
		"issues":              []string{},
	}

	for _, c := range conditions {
		isActive := c.Status == "True"

		switch c.Type {
		case "Ready":
			summary["ready"] = isActive
			if !isActive {
				summary["issues"] = append(summary["issues"].([]string), "NodeNotReady")
			}
		case "MemoryPressure", "DiskPressure", "PIDPressure", "NetworkUnavailable":
			summary[t.toSnakeCase(c.Type)] = isActive
			if isActive {
				summary["issues"] = append(summary["issues"].([]string), c.Type)
			}
		}
	}
	return summary
}

func (t *NodeStatusTool) toSnakeCase(s string) string {
	if s == "PIDPressure" {
		return "pid_pressure"
	}
	return strings.ToLower(strings.ReplaceAll(s, "Pressure", "_pressure"))
}

func (t *NodeStatusTool) analyzeHealth(total, ready int, unhealthy, tainted []string) map[string]interface{} {
	pct := 0.0
	if total > 0 {
		pct = float64(ready) / float64(total) * 100
	}

	status := "healthy"
	recs := []string{}

	if pct < 95 {
		status = "warning"
		recs = append(recs, "Investigate non-ready nodes.")
	}
	if pct < 80 {
		status = "critical"
		recs = append(recs, "Cluster capacity severely compromised.")
	}
	if len(tainted) > 0 {
		recs = append(recs, fmt.Sprintf("%d nodes have taints which may restrict scheduling.", len(tainted)))
	}
	if len(unhealthy) > 0 {
		recs = append(recs, fmt.Sprintf("Action required on: %s", strings.Join(unhealthy, ", ")))
	}

	return map[string]interface{}{
		"status":            status,
		"health_percentage": pct,
		"recommendations":   recs,
	}
}
