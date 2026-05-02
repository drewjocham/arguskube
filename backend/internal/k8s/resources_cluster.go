package k8s

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) listNodes(ctx context.Context) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Node",
		Columns: []ResourceColumn{
			{Key: "roles", Header: "Roles"},
			{Key: "version", Header: "Version"},
			{Key: "internal_ip", Header: "Internal IP"},
			{Key: "os_image", Header: "OS"},
			{Key: "cpu_capacity", Header: "CPU"},
			{Key: "mem_capacity", Header: "Memory"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		n := &list.Items[i]
		status, color := nodeStatus(n)
		roles := nodeRoles(n)

		internalIP := "—"
		for _, addr := range n.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				internalIP = addr.Address
				break
			}
		}

		items = append(items, ResourceItem{
			Name:        n.Name,
			Namespace:   "",
			Status:      status,
			StatusColor: color,
			Age:         fmtAge(n.CreationTimestamp.Time),
			Fields: map[string]string{
				"roles":        orDash(roles),
				"version":      n.Status.NodeInfo.KubeletVersion,
				"internal_ip":  internalIP,
				"os_image":     n.Status.NodeInfo.OSImage,
				"cpu_capacity": n.Status.Capacity.Cpu().String(),
				"mem_capacity": formatBytes(n.Status.Capacity.Memory().Value()),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listNamespaces(ctx context.Context) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Namespace",
		Columns: []ResourceColumn{
			{Key: "ns_status", Header: "Status"},
			{Key: "labels", Header: "Labels"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		ns := &list.Items[i]
		color := "green"
		if ns.Status.Phase == corev1.NamespaceTerminating {
			color = "red"
		}

		items = append(items, ResourceItem{
			Name:        ns.Name,
			Namespace:   "",
			Status:      string(ns.Status.Phase),
			StatusColor: color,
			Age:         fmtAge(ns.CreationTimestamp.Time),
			Fields: map[string]string{
				"ns_status": string(ns.Status.Phase),
				"labels":    fmt.Sprintf("%d labels", len(ns.Labels)),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listEvents(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Event",
		Columns: []ResourceColumn{
			{Key: "type", Header: "Type"},
			{Key: "reason", Header: "Reason"},
			{Key: "object", Header: "Object"},
			{Key: "message", Header: "Message"},
			{Key: "count", Header: "Count"},
		},
	}

	// Sort by last timestamp descending.
	sort.Slice(list.Items, func(i, j int) bool {
		ti := list.Items[i].LastTimestamp.Time
		tj := list.Items[j].LastTimestamp.Time
		return ti.After(tj)
	})

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		ev := &list.Items[i]
		color := "blue"
		if ev.Type == "Warning" {
			color = "amber"
		}

		msg := ev.Message
		if len(msg) > 120 {
			msg = msg[:120] + "…"
		}

		items = append(items, ResourceItem{
			Name:        ev.Name,
			Namespace:   ev.Namespace,
			Status:      ev.Type,
			StatusColor: color,
			Age:         fmtAge(ev.LastTimestamp.Time),
			Fields: map[string]string{
				"type":    ev.Type,
				"reason":  ev.Reason,
				"object":  ev.InvolvedObject.Kind + "/" + ev.InvolvedObject.Name,
				"message": msg,
				"count":   fmt.Sprintf("%d", ev.Count),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}
