package k8s

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) listServices(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Service",
		Columns: []ResourceColumn{
			{Key: "type", Header: "Type"},
			{Key: "cluster_ip", Header: "Cluster IP"},
			{Key: "ports", Header: "Ports"},
			{Key: "selector", Header: "Selector"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		s := &list.Items[i]
		ports := fmtServicePorts(s.Spec.Ports)
		selector := fmtMapSlice(s.Spec.Selector)

		items = append(items, ResourceItem{
			Name:        s.Name,
			Namespace:   s.Namespace,
			Status:      string(s.Spec.Type),
			StatusColor: "blue",
			Age:         fmtAge(s.CreationTimestamp.Time),
			Fields: map[string]string{
				"type":       string(s.Spec.Type),
				"cluster_ip": orDash(s.Spec.ClusterIP),
				"ports":      orDash(ports),
				"selector":   orDash(selector),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listEndpoints(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().Endpoints(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Endpoints",
		Columns: []ResourceColumn{
			{Key: "endpoints", Header: "Endpoints"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		e := &list.Items[i]
		var addrs []string
		for _, sub := range e.Subsets {
			for _, addr := range sub.Addresses {
				addrs = append(addrs, addr.IP)
			}
		}
		epStr := strings.Join(addrs, ", ")
		if len(addrs) > 3 {
			epStr = fmt.Sprintf("%s +%d more", strings.Join(addrs[:3], ", "), len(addrs)-3)
		}

		items = append(items, ResourceItem{
			Name:        e.Name,
			Namespace:   e.Namespace,
			Status:      fmt.Sprintf("%d addresses", len(addrs)),
			StatusColor: "green",
			Age:         fmtAge(e.CreationTimestamp.Time),
			Fields: map[string]string{
				"endpoints": orDash(epStr),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listIngresses(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "Ingress",
		Columns: []ResourceColumn{
			{Key: "class", Header: "Class"},
			{Key: "hosts", Header: "Hosts"},
			{Key: "load_balancer", Header: "Load Balancer"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		ing := &list.Items[i]
		var hosts []string
		for _, rule := range ing.Spec.Rules {
			if rule.Host != "" {
				hosts = append(hosts, rule.Host)
			}
		}

		className := "—"
		if ing.Spec.IngressClassName != nil {
			className = *ing.Spec.IngressClassName
		}

		lb := "—"
		if len(ing.Status.LoadBalancer.Ingress) > 0 {
			lbi := ing.Status.LoadBalancer.Ingress[0]
			if lbi.IP != "" {
				lb = lbi.IP
			} else {
				lb = lbi.Hostname
			}
		}

		items = append(items, ResourceItem{
			Name:        ing.Name,
			Namespace:   ing.Namespace,
			Status:      "Active",
			StatusColor: "green",
			Age:         fmtAge(ing.CreationTimestamp.Time),
			Fields: map[string]string{
				"class":         className,
				"hosts":         orDash(strings.Join(hosts, ", ")),
				"load_balancer": lb,
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listNetworkPolicies(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "NetworkPolicy",
		Columns: []ResourceColumn{
			{Key: "pod_selector", Header: "Pod Selector"},
			{Key: "policy_types", Header: "Policy Types"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		np := &list.Items[i]
		selector := fmtMapSlice(np.Spec.PodSelector.MatchLabels)
		var types []string
		for _, pt := range np.Spec.PolicyTypes {
			types = append(types, string(pt))
		}

		items = append(items, ResourceItem{
			Name:        np.Name,
			Namespace:   np.Namespace,
			Status:      "Active",
			StatusColor: "blue",
			Age:         fmtAge(np.CreationTimestamp.Time),
			Fields: map[string]string{
				"pod_selector": orDash(selector),
				"policy_types": orDash(strings.Join(types, ", ")),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

// ServicePod is a simplified pod reference returned by GetServicePods.
type ServicePod struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Container string `json:"container"` // first container name
}

// GetServicePods resolves a service's selector to its backing pods.
func (c *Client) GetServicePods(ctx context.Context, namespace, serviceName string) ([]ServicePod, error) {
	svc, err := c.cs.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}

	if len(svc.Spec.Selector) == 0 {
		return nil, fmt.Errorf("service %s has no selector", serviceName)
	}

	// Build label selector from service spec.
	var parts []string
	for k, v := range svc.Spec.Selector {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	labelSelector := strings.Join(parts, ",")

	pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("list pods for service: %w", err)
	}

	result := make([]ServicePod, 0, len(pods.Items))
	for i := range pods.Items {
		p := &pods.Items[i]
		container := ""
		if len(p.Spec.Containers) > 0 {
			container = p.Spec.Containers[0].Name
		}
		result = append(result, ServicePod{
			Name:      p.Name,
			Namespace: p.Namespace,
			Status:    string(p.Status.Phase),
			Container: container,
		})
	}
	return result, nil
}
