package k8s

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CostConfig defines per-unit pricing. Defaults match approximate AWS us-east-1
// on-demand pricing as of 2025.
type CostConfig struct {
	CPUPerCoreHour float64 // $ per vCPU hour (default ~$0.0425)
	MemPerGBHour   float64 // $ per GB-hour   (default ~$0.0053)
}

// DefaultCostConfig returns sensible defaults based on AWS on-demand pricing.
func DefaultCostConfig() CostConfig {
	return CostConfig{
		CPUPerCoreHour: 0.0425,
		MemPerGBHour:   0.0053,
	}
}

// CostBreakdown is the cost estimate for a single entity (namespace, deployment, pod).
type CostBreakdown struct {
	Name         string  `json:"name"`
	Namespace    string  `json:"namespace,omitempty"`
	Kind         string  `json:"kind"`          // "Namespace", "Deployment", "Pod"
	CPUCores     float64 `json:"cpuCores"`      // Total requested CPU cores
	MemoryGB     float64 `json:"memoryGB"`      // Total requested memory in GB
	CPUCostHr    float64 `json:"cpuCostHr"`     // $ per hour for CPU
	MemCostHr    float64 `json:"memCostHr"`     // $ per hour for memory
	TotalCostHr  float64 `json:"totalCostHr"`   // $ per hour total
	TotalCostDay float64 `json:"totalCostDay"`  // $ per day
	TotalCostMo  float64 `json:"totalCostMo"`   // $ per month (30 days)
	PodCount     int     `json:"podCount"`
}

// ClusterCostReport is the full FinOps cost estimate for the cluster.
type ClusterCostReport struct {
	Namespaces    []CostBreakdown `json:"namespaces"`
	TopDeployments []CostBreakdown `json:"topDeployments"` // Top 20 by cost
	TotalCostHr   float64         `json:"totalCostHr"`
	TotalCostDay  float64         `json:"totalCostDay"`
	TotalCostMo   float64         `json:"totalCostMo"`
	TotalCPU      float64         `json:"totalCpu"`
	TotalMemGB    float64         `json:"totalMemGb"`
	PodCount      int             `json:"podCount"`
}

// EstimateCosts builds a cost report from all pods' resource requests.
func (c *Client) EstimateCosts(ctx context.Context, costCfg CostConfig) (*ClusterCostReport, error) {
	pods, err := c.cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods for cost estimation: %w", err)
	}

	nsCosts := make(map[string]*CostBreakdown)
	deployCosts := make(map[string]*CostBreakdown)

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue // Skip completed/failed pods.
		}

		cpuCores, memGB := podResourceRequests(pod)

		// Aggregate by namespace.
		nsKey := pod.Namespace
		if _, ok := nsCosts[nsKey]; !ok {
			nsCosts[nsKey] = &CostBreakdown{
				Name: nsKey,
				Kind: "Namespace",
			}
		}
		ns := nsCosts[nsKey]
		ns.CPUCores += cpuCores
		ns.MemoryGB += memGB
		ns.PodCount++

		// Aggregate by deployment (via owner refs).
		deployName := ownerDeployment(pod)
		if deployName != "" {
			deployKey := pod.Namespace + "/" + deployName
			if _, ok := deployCosts[deployKey]; !ok {
				deployCosts[deployKey] = &CostBreakdown{
					Name:      deployName,
					Namespace: pod.Namespace,
					Kind:      "Deployment",
				}
			}
			d := deployCosts[deployKey]
			d.CPUCores += cpuCores
			d.MemoryGB += memGB
			d.PodCount++
		}
	}

	report := &ClusterCostReport{}

	// Finalize namespace costs.
	for _, ns := range nsCosts {
		applyCostRates(ns, costCfg)
		report.Namespaces = append(report.Namespaces, *ns)
		report.TotalCPU += ns.CPUCores
		report.TotalMemGB += ns.MemoryGB
		report.PodCount += ns.PodCount
	}

	// Sort namespaces by monthly cost descending.
	sort.Slice(report.Namespaces, func(i, j int) bool {
		return report.Namespaces[i].TotalCostMo > report.Namespaces[j].TotalCostMo
	})

	// Finalize deployment costs, take top 20.
	var deployList []CostBreakdown
	for _, d := range deployCosts {
		applyCostRates(d, costCfg)
		deployList = append(deployList, *d)
	}
	sort.Slice(deployList, func(i, j int) bool {
		return deployList[i].TotalCostMo > deployList[j].TotalCostMo
	})
	if len(deployList) > 20 {
		deployList = deployList[:20]
	}
	report.TopDeployments = deployList

	// Totals.
	report.TotalCostHr = report.TotalCPU*costCfg.CPUPerCoreHour + report.TotalMemGB*costCfg.MemPerGBHour
	report.TotalCostDay = report.TotalCostHr * 24
	report.TotalCostMo = report.TotalCostDay * 30

	return report, nil
}

// podResourceRequests sums CPU (cores) and memory (GB) requests across all containers.
func podResourceRequests(pod corev1.Pod) (cpuCores, memGB float64) {
	for _, c := range pod.Spec.Containers {
		if req, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
			cpuCores += float64(req.MilliValue()) / 1000.0
		}
		if req, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
			memGB += float64(req.Value()) / (1024 * 1024 * 1024)
		}
	}
	// Round to 3 decimals.
	cpuCores = math.Round(cpuCores*1000) / 1000
	memGB = math.Round(memGB*1000) / 1000
	return
}

// ownerDeployment traverses owner references to find the owning Deployment name.
func ownerDeployment(pod corev1.Pod) string {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "ReplicaSet" {
			// ReplicaSet name convention: <deployment>-<hash>
			parts := strings.Split(ref.Name, "-")
			if len(parts) >= 2 {
				return strings.Join(parts[:len(parts)-1], "-")
			}
			return ref.Name
		}
		if ref.Kind == "Deployment" || ref.Kind == "StatefulSet" || ref.Kind == "DaemonSet" {
			return ref.Name
		}
	}
	return ""
}

func applyCostRates(b *CostBreakdown, cfg CostConfig) {
	b.CPUCostHr = math.Round(b.CPUCores*cfg.CPUPerCoreHour*10000) / 10000
	b.MemCostHr = math.Round(b.MemoryGB*cfg.MemPerGBHour*10000) / 10000
	b.TotalCostHr = b.CPUCostHr + b.MemCostHr
	b.TotalCostDay = math.Round(b.TotalCostHr*24*100) / 100
	b.TotalCostMo = math.Round(b.TotalCostDay*30*100) / 100
}
