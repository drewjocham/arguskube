package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- Common types for the resource browser ---

// ResourceColumn describes a column in the resource table.
type ResourceColumn struct {
	Key    string `json:"key"`
	Header string `json:"header"`
}

// ResourceSchema describes the table layout for a resource type.
type ResourceSchema struct {
	Kind    string           `json:"kind"`
	Columns []ResourceColumn `json:"columns"`
}

// ResourceItem is a single row in the resource table.
type ResourceItem struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Status      string            `json:"status"`
	StatusColor string            `json:"statusColor"` // green, red, amber, blue, gray
	Age         string            `json:"age"`
	Fields      map[string]string `json:"fields"` // keyed by column key
}

// ResourceListResult is the response for ListResources.
type ResourceListResult struct {
	Schema ResourceSchema `json:"schema"`
	Items  []ResourceItem `json:"items"`
	Total  int            `json:"total"`
}

// KeyValue is a labelled key-value pair for detail views.
type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ResourceCondition is a condition on a resource.
type ResourceCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Age     string `json:"age"`
}

// ResourceEvent is a Kubernetes event associated with a resource.
type ResourceEvent struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Count   int32  `json:"count"`
	Age     string `json:"age"`
}

// ResourceDetailResult is the full detail view for a single resource.
type ResourceDetailResult struct {
	Kind        string              `json:"kind"`
	Name        string              `json:"name"`
	Namespace   string              `json:"namespace"`
	Created     string              `json:"created"`
	Labels      map[string]string   `json:"labels"`
	Annotations map[string]string   `json:"annotations"`
	Properties  []KeyValue          `json:"properties"`
	Data        map[string]string   `json:"data"` // ConfigMaps/Secrets
	Conditions  []ResourceCondition `json:"conditions"`
	Events      []ResourceEvent     `json:"events"`
}

// --- Dispatcher ---

// ListResources lists resources of the given kind in a namespace.
// Pass "" for namespace to list across all namespaces.
func (c *Client) ListResources(ctx context.Context, kind, namespace string) (*ResourceListResult, error) {
	ns := namespace
	if ns == "" {
		ns = c.cfg.Kubernetes.Namespace
	}

	c.logger.DebugContext(ctx, "listing resources",
		slog.String("kind", kind),
		slog.String("namespace", ns),
	)

	switch kind {
	case "pods":
		return c.listPods(ctx, ns)
	case "deployments":
		return c.listDeployments(ctx, ns)
	case "statefulsets":
		return c.listStatefulSets(ctx, ns)
	case "daemonsets":
		return c.listDaemonSets(ctx, ns)
	case "replicasets":
		return c.listReplicaSets(ctx, ns)
	case "jobs":
		return c.listJobs(ctx, ns)
	case "cronjobs":
		return c.listCronJobs(ctx, ns)
	case "services":
		return c.listServices(ctx, ns)
	case "endpoints":
		return c.listEndpoints(ctx, ns)
	case "ingresses":
		return c.listIngresses(ctx, ns)
	case "networkpolicies":
		return c.listNetworkPolicies(ctx, ns)
	case "configmaps":
		return c.listConfigMaps(ctx, ns)
	case "secrets":
		return c.listSecrets(ctx, ns)
	case "hpas":
		return c.listHPAs(ctx, ns)
	case "pvcs":
		return c.listPVCs(ctx, ns)
	case "pvs":
		return c.listPVs(ctx)
	case "storageclasses":
		return c.listStorageClasses(ctx)
	case "nodes":
		return c.listNodes(ctx)
	case "namespaces":
		return c.listNamespaces(ctx)
	case "events":
		return c.listEvents(ctx, ns)
	default:
		return nil, fmt.Errorf("unknown resource kind: %s", kind)
	}
}

// GetResourceDetail returns the detail view for a single resource.
func (c *Client) GetResourceDetail(ctx context.Context, kind, namespace, name string) (*ResourceDetailResult, error) {
	switch kind {
	case "pods":
		return c.getPodDetail(ctx, namespace, name)
	case "deployments":
		return c.getDeploymentDetail(ctx, namespace, name)
	case "services":
		return c.getServiceDetail(ctx, namespace, name)
	case "configmaps":
		return c.getConfigMapDetail(ctx, namespace, name)
	case "secrets":
		return c.getSecretDetail(ctx, namespace, name)
	case "nodes":
		return c.getNodeDetail(ctx, name)
	case "namespaces":
		return c.getNamespaceDetail(ctx, name)
	case "pvcs":
		return c.getPVCDetail(ctx, namespace, name)
	case "ingresses":
		return c.getIngressDetail(ctx, namespace, name)
	default:
		return c.getGenericDetail(ctx, kind, namespace, name)
	}
}

// ListAllNamespaces returns a list of namespace names for the namespace picker.
func (c *Client) ListAllNamespaces(ctx context.Context) ([]string, error) {
	list, err := c.cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		names = append(names, ns.Name)
	}
	sort.Strings(names)
	return names, nil
}

// --- Resource listers ---

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

func (c *Client) listPVCs(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "PersistentVolumeClaim",
		Columns: []ResourceColumn{
			{Key: "pvc_status", Header: "Status"},
			{Key: "volume", Header: "Volume"},
			{Key: "capacity", Header: "Capacity"},
			{Key: "access_modes", Header: "Access Modes"},
			{Key: "storage_class", Header: "Storage Class"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		pvc := &list.Items[i]
		status := string(pvc.Status.Phase)
		color := "green"
		if pvc.Status.Phase == corev1.ClaimPending {
			color = "amber"
		}

		capacity := "—"
		if pvc.Status.Capacity != nil {
			if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
				capacity = q.String()
			}
		}

		modes := fmtAccessModes(pvc.Spec.AccessModes)
		sc := "—"
		if pvc.Spec.StorageClassName != nil {
			sc = *pvc.Spec.StorageClassName
		}

		items = append(items, ResourceItem{
			Name:        pvc.Name,
			Namespace:   pvc.Namespace,
			Status:      status,
			StatusColor: color,
			Age:         fmtAge(pvc.CreationTimestamp.Time),
			Fields: map[string]string{
				"pvc_status":    status,
				"volume":        orDash(pvc.Spec.VolumeName),
				"capacity":      capacity,
				"access_modes":  modes,
				"storage_class": sc,
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listPVs(ctx context.Context) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "PersistentVolume",
		Columns: []ResourceColumn{
			{Key: "capacity", Header: "Capacity"},
			{Key: "access_modes", Header: "Access Modes"},
			{Key: "reclaim_policy", Header: "Reclaim Policy"},
			{Key: "pv_status", Header: "Status"},
			{Key: "claim", Header: "Claim"},
			{Key: "storage_class", Header: "Storage Class"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		pv := &list.Items[i]
		capacity := "—"
		if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = q.String()
		}

		claim := "—"
		if pv.Spec.ClaimRef != nil {
			claim = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
		}

		color := "green"
		if pv.Status.Phase == corev1.VolumeReleased {
			color = "amber"
		}
		if pv.Status.Phase == corev1.VolumeFailed {
			color = "red"
		}

		items = append(items, ResourceItem{
			Name:        pv.Name,
			Namespace:   "",
			Status:      string(pv.Status.Phase),
			StatusColor: color,
			Age:         fmtAge(pv.CreationTimestamp.Time),
			Fields: map[string]string{
				"capacity":       capacity,
				"access_modes":   fmtAccessModes(pv.Spec.AccessModes),
				"reclaim_policy": string(pv.Spec.PersistentVolumeReclaimPolicy),
				"pv_status":      string(pv.Status.Phase),
				"claim":          claim,
				"storage_class":  orDash(pv.Spec.StorageClassName),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listStorageClasses(ctx context.Context) (*ResourceListResult, error) {
	list, err := c.cs.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "StorageClass",
		Columns: []ResourceColumn{
			{Key: "provisioner", Header: "Provisioner"},
			{Key: "reclaim_policy", Header: "Reclaim Policy"},
			{Key: "binding_mode", Header: "Binding Mode"},
			{Key: "default", Header: "Default"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		sc := &list.Items[i]
		reclaimPolicy := "Delete"
		if sc.ReclaimPolicy != nil {
			reclaimPolicy = string(*sc.ReclaimPolicy)
		}
		bindingMode := "Immediate"
		if sc.VolumeBindingMode != nil {
			bindingMode = string(*sc.VolumeBindingMode)
		}
		isDefault := "false"
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			isDefault = "true"
		}

		items = append(items, ResourceItem{
			Name:        sc.Name,
			Namespace:   "",
			Status:      "Active",
			StatusColor: "blue",
			Age:         fmtAge(sc.CreationTimestamp.Time),
			Fields: map[string]string{
				"provisioner":    sc.Provisioner,
				"reclaim_policy": reclaimPolicy,
				"binding_mode":   bindingMode,
				"default":        isDefault,
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

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

// --- Detail methods ---

func (c *Client) getPodDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	p, err := c.cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status, _ := podStatus(p)

	props := []KeyValue{
		{Key: "Status", Value: status},
		{Key: "Node", Value: orDash(p.Spec.NodeName)},
		{Key: "Pod IP", Value: orDash(p.Status.PodIP)},
		{Key: "QoS Class", Value: string(p.Status.QOSClass)},
		{Key: "Service Account", Value: orDash(p.Spec.ServiceAccountName)},
		{Key: "Restart Policy", Value: string(p.Spec.RestartPolicy)},
	}

	if len(p.OwnerReferences) > 0 {
		props = append(props, KeyValue{Key: "Controlled By", Value: p.OwnerReferences[0].Kind + "/" + p.OwnerReferences[0].Name})
	}

	for _, c := range p.Spec.Containers {
		props = append(props, KeyValue{Key: "Container: " + c.Name, Value: c.Image})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range p.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, ns, "Pod", name)

	return &ResourceDetailResult{
		Kind:        "Pod",
		Name:        p.Name,
		Namespace:   p.Namespace,
		Created:     fmtTimestamp(p.CreationTimestamp.Time),
		Labels:      p.Labels,
		Annotations: p.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}, nil
}

func (c *Client) getDeploymentDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	d, err := c.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Status", Value: deploymentStatus(d)},
		{Key: "Replicas", Value: fmt.Sprintf("%d desired / %d ready / %d available", ptrInt32(d.Spec.Replicas), d.Status.ReadyReplicas, d.Status.AvailableReplicas)},
		{Key: "Strategy", Value: string(d.Spec.Strategy.Type)},
		{Key: "Selector", Value: fmtMapSlice(d.Spec.Selector.MatchLabels)},
	}

	for _, c := range d.Spec.Template.Spec.Containers {
		props = append(props, KeyValue{Key: "Container: " + c.Name, Value: c.Image})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range d.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, ns, "Deployment", name)

	return &ResourceDetailResult{
		Kind:        "Deployment",
		Name:        d.Name,
		Namespace:   d.Namespace,
		Created:     fmtTimestamp(d.CreationTimestamp.Time),
		Labels:      d.Labels,
		Annotations: d.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}, nil
}

func (c *Client) getServiceDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	s, err := c.cs.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Type", Value: string(s.Spec.Type)},
		{Key: "Cluster IP", Value: orDash(s.Spec.ClusterIP)},
		{Key: "Ports", Value: fmtServicePorts(s.Spec.Ports)},
		{Key: "Selector", Value: fmtMapSlice(s.Spec.Selector)},
		{Key: "Session Affinity", Value: string(s.Spec.SessionAffinity)},
	}

	if s.Spec.ExternalName != "" {
		props = append(props, KeyValue{Key: "External Name", Value: s.Spec.ExternalName})
	}

	events := c.getResourceEvents(ctx, ns, "Service", name)

	return &ResourceDetailResult{
		Kind:        "Service",
		Name:        s.Name,
		Namespace:   s.Namespace,
		Created:     fmtTimestamp(s.CreationTimestamp.Time),
		Labels:      s.Labels,
		Annotations: s.Annotations,
		Properties:  props,
		Events:      events,
	}, nil
}

func (c *Client) getConfigMapDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	cm, err := c.cs.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Data Keys", Value: fmt.Sprintf("%d", len(cm.Data))},
		{Key: "Binary Data Keys", Value: fmt.Sprintf("%d", len(cm.BinaryData))},
	}

	return &ResourceDetailResult{
		Kind:        "ConfigMap",
		Name:        cm.Name,
		Namespace:   cm.Namespace,
		Created:     fmtTimestamp(cm.CreationTimestamp.Time),
		Labels:      cm.Labels,
		Annotations: cm.Annotations,
		Properties:  props,
		Data:        cm.Data,
	}, nil
}

func (c *Client) getSecretDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	s, err := c.cs.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Type", Value: string(s.Type)},
		{Key: "Data Keys", Value: fmt.Sprintf("%d", len(s.Data))},
	}

	// Show key names but mask values for security.
	maskedData := make(map[string]string, len(s.Data))
	for k, v := range s.Data {
		maskedData[k] = fmt.Sprintf("(%d bytes)", len(v))
	}

	return &ResourceDetailResult{
		Kind:        "Secret",
		Name:        s.Name,
		Namespace:   s.Namespace,
		Created:     fmtTimestamp(s.CreationTimestamp.Time),
		Labels:      s.Labels,
		Annotations: s.Annotations,
		Properties:  props,
		Data:        maskedData,
	}, nil
}

func (c *Client) getNodeDetail(ctx context.Context, name string) (*ResourceDetailResult, error) {
	n, err := c.cs.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status, _ := nodeStatus(n)
	props := []KeyValue{
		{Key: "Status", Value: status},
		{Key: "Roles", Value: orDash(nodeRoles(n))},
		{Key: "Version", Value: n.Status.NodeInfo.KubeletVersion},
		{Key: "OS", Value: n.Status.NodeInfo.OSImage},
		{Key: "Kernel", Value: n.Status.NodeInfo.KernelVersion},
		{Key: "Container Runtime", Value: n.Status.NodeInfo.ContainerRuntimeVersion},
		{Key: "CPU Capacity", Value: n.Status.Capacity.Cpu().String()},
		{Key: "Memory Capacity", Value: formatBytes(n.Status.Capacity.Memory().Value())},
		{Key: "Pods Capacity", Value: n.Status.Capacity.Pods().String()},
	}

	for _, addr := range n.Status.Addresses {
		props = append(props, KeyValue{Key: string(addr.Type), Value: addr.Address})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range n.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, "", "Node", name)

	return &ResourceDetailResult{
		Kind:        "Node",
		Name:        n.Name,
		Namespace:   "",
		Created:     fmtTimestamp(n.CreationTimestamp.Time),
		Labels:      n.Labels,
		Annotations: n.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}, nil
}

func (c *Client) getNamespaceDetail(ctx context.Context, name string) (*ResourceDetailResult, error) {
	ns, err := c.cs.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Status", Value: string(ns.Status.Phase)},
	}

	return &ResourceDetailResult{
		Kind:        "Namespace",
		Name:        ns.Name,
		Namespace:   "",
		Created:     fmtTimestamp(ns.CreationTimestamp.Time),
		Labels:      ns.Labels,
		Annotations: ns.Annotations,
		Properties:  props,
	}, nil
}

func (c *Client) getPVCDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	pvc, err := c.cs.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	capacity := "—"
	if pvc.Status.Capacity != nil {
		if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capacity = q.String()
		}
	}
	sc := "—"
	if pvc.Spec.StorageClassName != nil {
		sc = *pvc.Spec.StorageClassName
	}

	props := []KeyValue{
		{Key: "Status", Value: string(pvc.Status.Phase)},
		{Key: "Volume", Value: orDash(pvc.Spec.VolumeName)},
		{Key: "Capacity", Value: capacity},
		{Key: "Access Modes", Value: fmtAccessModes(pvc.Spec.AccessModes)},
		{Key: "Storage Class", Value: sc},
	}

	events := c.getResourceEvents(ctx, ns, "PersistentVolumeClaim", name)

	return &ResourceDetailResult{
		Kind:        "PersistentVolumeClaim",
		Name:        pvc.Name,
		Namespace:   pvc.Namespace,
		Created:     fmtTimestamp(pvc.CreationTimestamp.Time),
		Labels:      pvc.Labels,
		Annotations: pvc.Annotations,
		Properties:  props,
		Events:      events,
	}, nil
}

func (c *Client) getIngressDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	ing, err := c.cs.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	className := "—"
	if ing.Spec.IngressClassName != nil {
		className = *ing.Spec.IngressClassName
	}

	props := []KeyValue{
		{Key: "Ingress Class", Value: className},
	}

	for _, rule := range ing.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				props = append(props, KeyValue{
					Key:   fmt.Sprintf("Rule: %s%s", orDash(rule.Host), path.Path),
					Value: fmt.Sprintf("%s:%d", path.Backend.Service.Name, path.Backend.Service.Port.Number),
				})
			}
		}
	}

	for _, tls := range ing.Spec.TLS {
		props = append(props, KeyValue{
			Key:   "TLS",
			Value: fmt.Sprintf("hosts=%s secret=%s", strings.Join(tls.Hosts, ","), tls.SecretName),
		})
	}

	events := c.getResourceEvents(ctx, ns, "Ingress", name)

	return &ResourceDetailResult{
		Kind:        "Ingress",
		Name:        ing.Name,
		Namespace:   ing.Namespace,
		Created:     fmtTimestamp(ing.CreationTimestamp.Time),
		Labels:      ing.Labels,
		Annotations: ing.Annotations,
		Properties:  props,
		Events:      events,
	}, nil
}

func (c *Client) getGenericDetail(ctx context.Context, kind, ns, name string) (*ResourceDetailResult, error) {
	return &ResourceDetailResult{
		Kind:      kind,
		Name:      name,
		Namespace: ns,
		Created:   "—",
		Properties: []KeyValue{
			{Key: "Note", Value: "Detail view not yet implemented for this resource type."},
		},
	}, nil
}

// --- Event helper ---

func (c *Client) getResourceEvents(ctx context.Context, ns, kind, name string) []ResourceEvent {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s", name, kind)
	list, err := c.cs.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil
	}

	events := make([]ResourceEvent, 0, len(list.Items))
	for _, ev := range list.Items {
		events = append(events, ResourceEvent{
			Type:    ev.Type,
			Reason:  ev.Reason,
			Message: ev.Message,
			Count:   ev.Count,
			Age:     fmtAge(ev.LastTimestamp.Time),
		})
	}
	return events
}

// --- Formatting helpers ---

func fmtAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m > 0 {
			return fmt.Sprintf("%dh%dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	default:
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func fmtTimestamp(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return fmt.Sprintf("%s (%s)", fmtAge(t), t.Format("Jan 02, 2006 3:04:05 PM MST"))
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func fmtMapSlice(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func fmtServicePorts(ports []corev1.ServicePort) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		s := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		if p.Name != "" {
			s = p.Name + ":" + s
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func fmtAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
	parts := make([]string, 0, len(modes))
	for _, m := range modes {
		switch m {
		case corev1.ReadWriteOnce:
			parts = append(parts, "RWO")
		case corev1.ReadOnlyMany:
			parts = append(parts, "ROX")
		case corev1.ReadWriteMany:
			parts = append(parts, "RWX")
		default:
			parts = append(parts, string(m))
		}
	}
	return strings.Join(parts, ", ")
}

func formatBytes(b int64) string {
	const (
		gi = 1 << 30
		mi = 1 << 20
	)
	switch {
	case b >= gi:
		return fmt.Sprintf("%.1fGi", float64(b)/float64(gi))
	case b >= mi:
		return fmt.Sprintf("%.0fMi", float64(b)/float64(mi))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func extractImages(containers []corev1.Container) string {
	parts := make([]string, 0, len(containers))
	for _, c := range containers {
		img := c.Image
		// Show just the image name + tag, not the full registry path.
		if idx := strings.LastIndex(img, "/"); idx >= 0 {
			img = img[idx+1:]
		}
		parts = append(parts, img)
	}
	return strings.Join(parts, ", ")
}

func ptrInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

// --- Status derivation ---

func podStatus(p *corev1.Pod) (string, string) {
	// Check container states first.
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			switch reason {
			case "CrashLoopBackOff":
				return "CrashLoopBackOff", "red"
			case "ImagePullBackOff", "ErrImagePull":
				return reason, "red"
			case "ContainerCreating":
				return reason, "amber"
			default:
				return reason, "amber"
			}
		}
		if cs.State.Terminated != nil {
			reason := cs.State.Terminated.Reason
			if reason == "OOMKilled" {
				return "OOMKilled", "red"
			}
			if reason == "Completed" {
				return "Completed", "blue"
			}
			return reason, "red"
		}
	}

	switch p.Status.Phase {
	case corev1.PodRunning:
		return "Running", "green"
	case corev1.PodSucceeded:
		return "Completed", "blue"
	case corev1.PodFailed:
		return "Failed", "red"
	case corev1.PodPending:
		return "Pending", "amber"
	default:
		return string(p.Status.Phase), "gray"
	}
}

func deploymentStatus(d *appsv1.Deployment) string {
	for _, cond := range d.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionFalse {
			return "Progressing"
		}
	}
	if d.Status.ReadyReplicas == ptrInt32(d.Spec.Replicas) {
		return "Running"
	}
	return "Updating"
}

func jobStatus(j *batchv1.Job) (string, string) {
	for _, cond := range j.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			return "Complete", "green"
		}
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return "Failed", "red"
		}
	}
	if j.Status.Active > 0 {
		return "Running", "blue"
	}
	return "Pending", "amber"
}

func nodeStatus(n *corev1.Node) (string, string) {
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				return "Ready", "green"
			}
			return "NotReady", "red"
		}
	}
	return "Unknown", "gray"
}

func nodeRoles(n *corev1.Node) string {
	roles := make([]string, 0)
	for k := range n.Labels {
		if strings.HasPrefix(k, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	return strings.Join(roles, ", ")
}

// Suppress unused import warnings — these types are used via the API but the
// compiler needs to see references in this file.
var (
	_ = (*appsv1.Deployment)(nil)
	_ = (*autoscalingv1.HorizontalPodAutoscaler)(nil)
	_ = (*batchv1.Job)(nil)
	_ = (*networkingv1.Ingress)(nil)
	_ = (*storagev1.StorageClass)(nil)
)
