package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) listPods(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Pod",
		Columns: []ResourceColumn{
			{Key: "status_icon", Header: ""},
			{Key: "cpu", Header: "CPU"},
			{Key: "memory", Header: "Memory"},
			{Key: "restarts", Header: "Restarts"},
			{Key: "controlled_by", Header: "Controlled By"},
			{Key: "node", Header: "Node"},
			{Key: "qos", Header: "QoS"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		p := &list.Items[i]
		status, color := podStatus(p)

		var totalRestarts int32
		var cpuReq, memReq string
		for _, cs := range p.Status.ContainerStatuses {
			totalRestarts += cs.RestartCount
		}
		for _, c := range p.Spec.Containers {
			if req := c.Resources.Requests; req != nil {
				if cpu, ok := req[corev1.ResourceCPU]; ok {
					cpuReq = cpu.String()
				}
				if mem, ok := req[corev1.ResourceMemory]; ok {
					memReq = mem.String()
				}
			}
		}

		controlledBy := ""
		if len(p.OwnerReferences) > 0 {
			controlledBy = p.OwnerReferences[0].Kind
		}

		items = append(items, ResourceItem{
			Name:        p.Name,
			Namespace:   p.Namespace,
			Status:      status,
			StatusColor: color,
			Age:         fmtAge(p.CreationTimestamp.Time),
			Fields: map[string]string{
				"cpu":           orDash(cpuReq),
				"memory":        orDash(memReq),
				"restarts":      fmt.Sprintf("%d", totalRestarts),
				"controlled_by": orDash(controlledBy),
				"node":          orDash(p.Spec.NodeName),
				"qos":           string(p.Status.QOSClass),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listDeployments(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Deployment",
		Columns: []ResourceColumn{
			{Key: "ready", Header: "Ready"},
			{Key: "up_to_date", Header: "Up-to-date"},
			{Key: "available", Header: "Available"},
			{Key: "images", Header: "Images"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		d := &list.Items[i]
		ready := fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, ptrInt32(d.Spec.Replicas))
		color := "green"
		if d.Status.ReadyReplicas < ptrInt32(d.Spec.Replicas) {
			color = "amber"
		}
		if d.Status.ReadyReplicas == 0 && ptrInt32(d.Spec.Replicas) > 0 {
			color = "red"
		}

		images := extractImages(d.Spec.Template.Spec.Containers)

		items = append(items, ResourceItem{
			Name:        d.Name,
			Namespace:   d.Namespace,
			Status:      deploymentStatus(d),
			StatusColor: color,
			Age:         fmtAge(d.CreationTimestamp.Time),
			Fields: map[string]string{
				"ready":      ready,
				"up_to_date": fmt.Sprintf("%d", d.Status.UpdatedReplicas),
				"available":  fmt.Sprintf("%d", d.Status.AvailableReplicas),
				"images":     images,
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listStatefulSets(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "StatefulSet",
		Columns: []ResourceColumn{
			{Key: "ready", Header: "Ready"},
			{Key: "images", Header: "Images"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		s := &list.Items[i]
		ready := fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, ptrInt32(s.Spec.Replicas))
		color := "green"
		if s.Status.ReadyReplicas < ptrInt32(s.Spec.Replicas) {
			color = "amber"
		}
		items = append(items, ResourceItem{
			Name:        s.Name,
			Namespace:   s.Namespace,
			Status:      "Running",
			StatusColor: color,
			Age:         fmtAge(s.CreationTimestamp.Time),
			Fields: map[string]string{
				"ready":  ready,
				"images": extractImages(s.Spec.Template.Spec.Containers),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listDaemonSets(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "DaemonSet",
		Columns: []ResourceColumn{
			{Key: "desired", Header: "Desired"},
			{Key: "current", Header: "Current"},
			{Key: "ready", Header: "Ready"},
			{Key: "node_selector", Header: "Node Selector"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		d := &list.Items[i]
		color := "green"
		if d.Status.NumberReady < d.Status.DesiredNumberScheduled {
			color = "amber"
		}
		nodeSelector := ""
		if d.Spec.Template.Spec.NodeSelector != nil {
			parts := make([]string, 0)
			for k, v := range d.Spec.Template.Spec.NodeSelector {
				parts = append(parts, k+"="+v)
			}
			nodeSelector = strings.Join(parts, ", ")
		}
		items = append(items, ResourceItem{
			Name:        d.Name,
			Namespace:   d.Namespace,
			Status:      "Running",
			StatusColor: color,
			Age:         fmtAge(d.CreationTimestamp.Time),
			Fields: map[string]string{
				"desired":       fmt.Sprintf("%d", d.Status.DesiredNumberScheduled),
				"current":       fmt.Sprintf("%d", d.Status.CurrentNumberScheduled),
				"ready":         fmt.Sprintf("%d", d.Status.NumberReady),
				"node_selector": orDash(nodeSelector),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listReplicaSets(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "ReplicaSet",
		Columns: []ResourceColumn{
			{Key: "desired", Header: "Desired"},
			{Key: "current", Header: "Current"},
			{Key: "ready", Header: "Ready"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		r := &list.Items[i]
		color := "green"
		if r.Status.ReadyReplicas < ptrInt32(r.Spec.Replicas) {
			color = "amber"
		}
		items = append(items, ResourceItem{
			Name:        r.Name,
			Namespace:   r.Namespace,
			Status:      "Running",
			StatusColor: color,
			Age:         fmtAge(r.CreationTimestamp.Time),
			Fields: map[string]string{
				"desired": fmt.Sprintf("%d", ptrInt32(r.Spec.Replicas)),
				"current": fmt.Sprintf("%d", r.Status.Replicas),
				"ready":   fmt.Sprintf("%d", r.Status.ReadyReplicas),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listJobs(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Job",
		Columns: []ResourceColumn{
			{Key: "completions", Header: "Completions"},
			{Key: "duration", Header: "Duration"},
			{Key: "conditions", Header: "Conditions"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		j := &list.Items[i]
		completions := fmt.Sprintf("%d/%d", j.Status.Succeeded, ptrInt32(j.Spec.Completions))
		status, color := jobStatus(j)

		duration := "—"
		if j.Status.StartTime != nil {
			end := time.Now()
			if j.Status.CompletionTime != nil {
				end = j.Status.CompletionTime.Time
			}
			duration = fmtDuration(end.Sub(j.Status.StartTime.Time))
		}

		condStr := ""
		for _, cond := range j.Status.Conditions {
			if cond.Status == corev1.ConditionTrue {
				condStr = string(cond.Type)
				break
			}
		}

		items = append(items, ResourceItem{
			Name:        j.Name,
			Namespace:   j.Namespace,
			Status:      status,
			StatusColor: color,
			Age:         fmtAge(j.CreationTimestamp.Time),
			Fields: map[string]string{
				"completions": completions,
				"duration":    duration,
				"conditions":  orDash(condStr),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listCronJobs(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.BatchV1().CronJobs(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "CronJob",
		Columns: []ResourceColumn{
			{Key: "schedule", Header: "Schedule"},
			{Key: "suspend", Header: "Suspend"},
			{Key: "active", Header: "Active"},
			{Key: "last_schedule", Header: "Last Schedule"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		cj := &list.Items[i]
		suspend := "false"
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			suspend = "true"
		}
		lastSchedule := "—"
		if cj.Status.LastScheduleTime != nil {
			lastSchedule = fmtAge(cj.Status.LastScheduleTime.Time)
		}

		items = append(items, ResourceItem{
			Name:        cj.Name,
			Namespace:   cj.Namespace,
			Status:      "Active",
			StatusColor: "green",
			Age:         fmtAge(cj.CreationTimestamp.Time),
			Fields: map[string]string{
				"schedule":      cj.Spec.Schedule,
				"suspend":       suspend,
				"active":        fmt.Sprintf("%d", len(cj.Status.Active)),
				"last_schedule": lastSchedule,
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}
