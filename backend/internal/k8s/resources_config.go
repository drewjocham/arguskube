package k8s

import (
	"context"
	"fmt"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) listConfigMaps(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "ConfigMap",
		Columns: []ResourceColumn{
			{Key: "data_count", Header: "Data"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		cm := &list.Items[i]
		items = append(items, ResourceItem{
			Name:        cm.Name,
			Namespace:   cm.Namespace,
			Status:      "Active",
			StatusColor: "blue",
			Age:         fmtAge(cm.CreationTimestamp.Time),
			Fields: map[string]string{
				"data_count": fmt.Sprintf("%d keys", len(cm.Data)+len(cm.BinaryData)),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listSecrets(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Secret",
		Columns: []ResourceColumn{
			{Key: "type", Header: "Type"},
			{Key: "data_count", Header: "Data"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		s := &list.Items[i]
		items = append(items, ResourceItem{
			Name:        s.Name,
			Namespace:   s.Namespace,
			Status:      "Active",
			StatusColor: "blue",
			Age:         fmtAge(s.CreationTimestamp.Time),
			Fields: map[string]string{
				"type":       string(s.Type),
				"data_count": fmt.Sprintf("%d keys", len(s.Data)),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listHPAs(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.AutoscalingV1().HorizontalPodAutoscalers(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "HorizontalPodAutoscaler",
		Columns: []ResourceColumn{
			{Key: "reference", Header: "Reference"},
			{Key: "min_replicas", Header: "Min"},
			{Key: "max_replicas", Header: "Max"},
			{Key: "replicas", Header: "Replicas"},
			{Key: "cpu", Header: "CPU Target"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		h := &list.Items[i]
		cpuTarget := "—"
		if h.Spec.TargetCPUUtilizationPercentage != nil {
			cpuTarget = fmt.Sprintf("%d%%", *h.Spec.TargetCPUUtilizationPercentage)
		}
		minReplicas := int32(1)
		if h.Spec.MinReplicas != nil {
			minReplicas = *h.Spec.MinReplicas
		}

		items = append(items, ResourceItem{
			Name:        h.Name,
			Namespace:   h.Namespace,
			Status:      "Active",
			StatusColor: "green",
			Age:         fmtAge(h.CreationTimestamp.Time),
			Fields: map[string]string{
				"reference":    h.Spec.ScaleTargetRef.Kind + "/" + h.Spec.ScaleTargetRef.Name,
				"min_replicas": fmt.Sprintf("%d", minReplicas),
				"max_replicas": fmt.Sprintf("%d", h.Spec.MaxReplicas),
				"replicas":     fmt.Sprintf("%d", h.Status.CurrentReplicas),
				"cpu":          cpuTarget,
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}
