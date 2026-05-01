package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

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
	Kind        string                 `json:"kind"`
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	Created     string                 `json:"created"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`
	Properties  []KeyValue             `json:"properties"`
	Data        map[string]string      `json:"data"` // ConfigMaps/Secrets
	Conditions  []ResourceCondition    `json:"conditions"`
	Events      []ResourceEvent        `json:"events"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

// --- Dispatcher ---

// ListResources lists resources of the given kind in a namespace.
// Pass "" for namespace to list across all namespaces.
func (c *Client) ListResources(ctx context.Context, kind, namespace string) (*ResourceListResult, error) {
	ns := namespace
	if ns == "_all" {
		ns = "" // empty string = all namespaces in k8s API
	} else if ns == "" {
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
