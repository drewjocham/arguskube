package k8s

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type WasteProfile struct {
	Namespace     string         `json:"namespace"`
	Deployments   []WasteItem    `json:"deployments"`
	TotalWasteCPU string         `json:"totalWasteCPU"`
	TotalWasteMem string         `json:"totalWasteMem"`
	Score         string         `json:"score"`
}

type WasteItem struct {
	Name          string `json:"name"`
	CPURequest    string `json:"cpuRequest"`
	MemoryRequest string `json:"memoryRequest"`
	CPUUsage      string `json:"cpuUsage,omitempty"`
	MemoryUsage   string `json:"memoryUsage,omitempty"`
	WasteCPU      string `json:"wasteCPU,omitempty"`
	WasteMem      string `json:"wasteMem,omitempty"`
	Ratio         string `json:"ratio,omitempty"`
}

func (c *Client) ProfileWaste(ctx context.Context, namespace string) (*WasteProfile, error) {
	profile := &WasteProfile{Namespace: namespace}

	deps, err := c.cs.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	var totalWasteCPU, totalWasteMem int64

	for i := range deps.Items {
		dep := &deps.Items[i]
		for _, container := range dep.Spec.Template.Spec.Containers {
			cpuReq := container.Resources.Requests.Cpu()
			memReq := container.Resources.Requests.Memory()
			if cpuReq == nil || cpuReq.IsZero() {
				continue
			}

			cpuMilli := cpuReq.MilliValue()
			memBytes := memReq.Value()

			wasteCPU := int64(0)
			wasteMem := int64(0)

			estimatedCPUMilli := cpuMilli / 10
			estimatedMemBytes := memBytes / 10

			if cpuMilli > 100 && estimatedCPUMilli < cpuMilli/5 {
				wasteCPU = cpuMilli - estimatedCPUMilli
			}
			if memBytes > 128*1024*1024 && estimatedMemBytes < memBytes/5 {
				wasteMem = memBytes - estimatedMemBytes
			}

			totalWasteCPU += wasteCPU
			totalWasteMem += wasteMem

			ratio := float64(cpuMilli) / 100.0
			if estimatedCPUMilli > 0 {
				ratio = float64(cpuMilli) / float64(estimatedCPUMilli)
			}

			item := WasteItem{
				Name:          dep.Name + "/" + container.Name,
				CPURequest:    cpuReq.String(),
				MemoryRequest: memReq.String(),
				WasteCPU:      formatResourceMilli(wasteCPU),
				WasteMem:      formatResourceBytes(wasteMem),
				Ratio:         fmt.Sprintf("%.1fx", ratio),
			}

			if wasteCPU > 500 || wasteMem > 512*1024*1024 {
				item.CPUUsage = "estimated: " + formatResourceMilli(estimatedCPUMilli)
				item.MemoryUsage = "estimated: " + formatResourceBytes(estimatedMemBytes)
			}

			profile.Deployments = append(profile.Deployments, item)
		}
	}

	profile.TotalWasteCPU = formatResourceMilli(totalWasteCPU)
	profile.TotalWasteMem = formatResourceBytes(totalWasteMem)

	switch {
	case totalWasteCPU > 2000 || totalWasteMem > 4*1024*1024*1024:
		profile.Score = "critical"
	case totalWasteCPU > 500 || totalWasteMem > 1*1024*1024*1024:
		profile.Score = "high"
	case totalWasteCPU > 100 || totalWasteMem > 256*1024*1024:
		profile.Score = "medium"
	default:
		profile.Score = "low"
	}

	sort.Slice(profile.Deployments, func(i, j int) bool {
		return profile.Deployments[i].WasteCPU > profile.Deployments[j].WasteCPU
	})

	return profile, nil
}

func formatResourceMilli(milli int64) string {
	if milli >= 1000 {
		return fmt.Sprintf("%.1f CPU", float64(milli)/1000)
	}
	return fmt.Sprintf("%dm", milli)
}

func formatResourceBytes(b int64) string {
	if b >= 1024*1024*1024 {
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(1024*1024*1024))
	}
	if b >= 1024*1024 {
		return fmt.Sprintf("%d MiB", b/(1024*1024))
	}
	return fmt.Sprintf("%d KiB", b/1024)
}

