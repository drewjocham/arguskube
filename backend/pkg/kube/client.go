// Package kube provides a Kubernetes client interface and implementation for the
// MCP server subsystem. It is intentionally separate from internal/k8s (which
// serves the Wails desktop app) so the MCP binary can be built independently.
package kube

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argues/kube-watcher/pkg/audit"
)

// ClientInterface is the abstraction consumed by all MCP tools.
type ClientInterface interface {
	GetNodes(ctx context.Context) ([]NodeInfo, error)
	GetNode(ctx context.Context, name string) (*NodeInfo, error)
	GetPod(ctx context.Context, namespace, name string) (*PodInfo, error)
	GetPods(ctx context.Context, namespace string) ([]PodInfo, error)
	GetPodsAllNamespaces(ctx context.Context) ([]PodInfo, error)
	GetPodLogs(ctx context.Context, namespace, pod, container string, tailLines, sinceSeconds int64, previous bool) (string, error)
	GetServices(ctx context.Context, namespace string) ([]ServiceInfo, error)
	GetServicesAllNamespaces(ctx context.Context) ([]ServiceInfo, error)
	GetEvents(ctx context.Context, namespace string) ([]EventInfo, error)
	GetEventsAllNamespaces(ctx context.Context) ([]EventInfo, error)
	GetNamespaces(ctx context.Context) ([]string, error)
	GetResourceQuotas(ctx context.Context, namespace string) ([]ResourceQuotaInfo, error)
	GetClusterInfo(ctx context.Context) (*ClusterInfo, error)
	GetResource(ctx context.Context, kind, name string) ([]runtime.Object, error)
	HealthCheck(ctx context.Context) error
	GetRawInterface() kubernetes.Interface
}

// AuditOption configures audit behaviour.
type AuditOption func(*auditConfig)

type auditConfig struct {
	enabled bool
}

// AuditOptionsFromEnv returns AuditOptions derived from environment variables.
func AuditOptionsFromEnv() []AuditOption {
	var opts []AuditOption
	if os.Getenv("KUBEWATCHER_AUDIT") == "true" {
		opts = append(opts, func(c *auditConfig) { c.enabled = true })
	}
	return opts
}

// ---------------------------------------------------------------------------
// Default client (uses kubeconfig / in-cluster config)
// ---------------------------------------------------------------------------

// Client implements ClientInterface.
type Client struct {
	cs     kubernetes.Interface
	logger *slog.Logger
}

// NewClient creates a Client that resolves kubeconfig using the default loading
// rules (KUBECONFIG env, ~/.kube/config, in-cluster).
func NewClient(logger *slog.Logger) (ClientInterface, error) {
	var cfg *rest.Config
	var err error

	// Try in-cluster first, then fall back to kubeconfig.
	cfg, err = rest.InClusterConfig()
	if err != nil {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("kube: unable to build config: %w", err)
		}
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kube: client creation failed: %w", err)
	}

	return &Client{cs: cs, logger: logger}, nil
}

func NewAuditClient(base ClientInterface, auditLogger *audit.SlogLogger, logger *slog.Logger, opts ...AuditOption) ClientInterface {
	ac := &auditConfig{}
	for _, o := range opts {
		o(ac)
	}
	if !ac.enabled {
		return base
	}
	return &auditClient{inner: base, audit: auditLogger, logger: logger}
}

func (c *Client) GetRawInterface() kubernetes.Interface { return c.cs }

func (c *Client) HealthCheck(ctx context.Context) error {
	_, err := c.cs.Discovery().ServerVersion()
	return err
}

func (c *Client) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	sv, err := c.cs.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}
	nodes, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return &ClusterInfo{Version: sv.GitVersion, NodeCount: len(nodes.Items)}, nil
}

func (c *Client) GetNodes(ctx context.Context) ([]NodeInfo, error) {
	list, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]NodeInfo, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, mapNode(&list.Items[i]))
	}
	return out, nil
}

func (c *Client) GetNode(ctx context.Context, name string) (*NodeInfo, error) {
	n, err := c.cs.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	info := mapNode(n)
	return &info, nil
}

func (c *Client) GetPod(ctx context.Context, namespace, name string) (*PodInfo, error) {
	p, err := c.cs.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	info := mapPod(p)
	return &info, nil
}

func (c *Client) GetPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	list, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]PodInfo, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, mapPod(&list.Items[i]))
	}
	return out, nil
}

func (c *Client) GetPodsAllNamespaces(ctx context.Context) ([]PodInfo, error) {
	return c.GetPods(ctx, "")
}

func (c *Client) GetPodLogs(ctx context.Context, namespace, pod, container string, tailLines, sinceSeconds int64, previous bool) (string, error) {
	opts := &corev1.PodLogOptions{
		Container: container,
		Previous:  previous,
	}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}
	if sinceSeconds > 0 {
		opts.SinceSeconds = &sinceSeconds
	}
	req := c.cs.CoreV1().Pods(namespace).GetLogs(pod, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 4096)
	for {
		n, readErr := stream.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if readErr != nil {
			break
		}
	}
	return string(buf), nil
}

// GetServices returns services in a namespace.
func (c *Client) GetServices(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	list, err := c.cs.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ServiceInfo, 0, len(list.Items))
	for i := range list.Items {
		s := &list.Items[i]
		out = append(out, ServiceInfo{
			Name: s.Name, Namespace: s.Namespace,
			Type: string(s.Spec.Type), ClusterIP: s.Spec.ClusterIP,
		})
	}
	return out, nil
}

// GetServicesAllNamespaces returns services across all namespaces.
func (c *Client) GetServicesAllNamespaces(ctx context.Context) ([]ServiceInfo, error) {
	return c.GetServices(ctx, "")
}

// GetEvents returns events in a namespace.
func (c *Client) GetEvents(ctx context.Context, namespace string) ([]EventInfo, error) {
	list, err := c.cs.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]EventInfo, 0, len(list.Items))
	for i := range list.Items {
		e := &list.Items[i]
		out = append(out, EventInfo{
			Type:          e.Type,
			Reason:        e.Reason,
			ObjectKind:    e.InvolvedObject.Kind,
			ObjectName:    e.InvolvedObject.Name,
			Message:       e.Message,
			LastTimestamp: e.LastTimestamp.Time,
			Count:         e.Count,
		})
	}
	return out, nil
}

// GetEventsAllNamespaces returns events across all namespaces.
func (c *Client) GetEventsAllNamespaces(ctx context.Context) ([]EventInfo, error) {
	return c.GetEvents(ctx, "")
}

// GetNamespaces returns namespace names.
func (c *Client) GetNamespaces(ctx context.Context) ([]string, error) {
	list, err := c.cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		out = append(out, ns.Name)
	}
	return out, nil
}

// GetResourceQuotas returns resource quotas in a namespace.
func (c *Client) GetResourceQuotas(ctx context.Context, namespace string) ([]ResourceQuotaInfo, error) {
	list, err := c.cs.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]ResourceQuotaInfo, 0, len(list.Items))
	for i := range list.Items {
		q := &list.Items[i]
		hard := make(map[string]string)
		used := make(map[string]string)
		for k, v := range q.Status.Hard {
			hard[string(k)] = v.String()
		}
		for k, v := range q.Status.Used {
			used[string(k)] = v.String()
		}
		out = append(out, ResourceQuotaInfo{
			Name: q.Name, Namespace: q.Namespace,
			Hard: hard, Used: used,
		})
	}
	return out, nil
}

// GetResource is a generic resource getter. Currently returns nil (placeholder).
func (c *Client) GetResource(ctx context.Context, kind, name string) ([]runtime.Object, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mapNode(n *corev1.Node) NodeInfo {
	status := "NotReady"
	var conds []NodeCondition
	for _, c := range n.Status.Conditions {
		conds = append(conds, NodeCondition{Type: string(c.Type), Status: string(c.Status)})
		if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
			status = "Ready"
		}
	}
	alloc := make(map[string]string)
	for k, v := range n.Status.Allocatable {
		alloc[string(k)] = v.String()
	}
	return NodeInfo{
		Name:        n.Name,
		Status:      status,
		Age:         time.Since(n.CreationTimestamp.Time),
		Conditions:  conds,
		Labels:      n.Labels,
		Taints:      n.Spec.Taints,
		Allocatable: alloc,
	}
}

func mapPod(p *corev1.Pod) PodInfo {
	var totalRestarts int32
	var containers []ContainerInfo
	for _, cs := range p.Status.ContainerStatuses {
		totalRestarts += cs.RestartCount
		state := "unknown"
		if cs.State.Running != nil {
			state = "running"
		} else if cs.State.Waiting != nil {
			state = cs.State.Waiting.Reason
		} else if cs.State.Terminated != nil {
			state = cs.State.Terminated.Reason
		}
		containers = append(containers, ContainerInfo{
			Name:         cs.Name,
			Image:        cs.Image,
			Ready:        cs.Ready,
			RestartCount: cs.RestartCount,
			State:        state,
		})
	}

	phase := string(p.Status.Phase)
	status := phase
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			status = cs.State.Waiting.Reason
			break
		}
	}

	return PodInfo{
		Name:         p.Name,
		Namespace:    p.Namespace,
		Phase:        phase,
		Status:       status,
		NodeName:     p.Spec.NodeName,
		Age:          time.Since(p.CreationTimestamp.Time),
		RestartCount: totalRestarts,
		Labels:       p.Labels,
		Containers:   containers,
	}
}

// defaultKubeconfig returns ~/.kube/config.
func defaultKubeconfig() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", "config")
}

// ---------------------------------------------------------------------------
// Audit client wrapper
// ---------------------------------------------------------------------------

type auditClient struct {
	inner  ClientInterface
	audit  *audit.SlogLogger
	logger *slog.Logger
}

func (a *auditClient) GetNodes(ctx context.Context) ([]NodeInfo, error) {
	a.audit.Log("list", "nodes", "", "")
	return a.inner.GetNodes(ctx)
}
func (a *auditClient) GetNode(ctx context.Context, name string) (*NodeInfo, error) {
	a.audit.Log("get", "node", "", name)
	return a.inner.GetNode(ctx, name)
}
func (a *auditClient) GetPod(ctx context.Context, ns, name string) (*PodInfo, error) {
	a.audit.Log("get", "pod", ns, name)
	return a.inner.GetPod(ctx, ns, name)
}
func (a *auditClient) GetPods(ctx context.Context, ns string) ([]PodInfo, error) {
	a.audit.Log("list", "pods", ns, "")
	return a.inner.GetPods(ctx, ns)
}
func (a *auditClient) GetPodsAllNamespaces(ctx context.Context) ([]PodInfo, error) {
	a.audit.Log("list", "pods", "*", "")
	return a.inner.GetPodsAllNamespaces(ctx)
}
func (a *auditClient) GetPodLogs(ctx context.Context, ns, pod, container string, tail, since int64, prev bool) (string, error) {
	a.audit.Log("logs", "pod", ns, pod)
	return a.inner.GetPodLogs(ctx, ns, pod, container, tail, since, prev)
}
func (a *auditClient) GetServices(ctx context.Context, ns string) ([]ServiceInfo, error) {
	a.audit.Log("list", "services", ns, "")
	return a.inner.GetServices(ctx, ns)
}
func (a *auditClient) GetServicesAllNamespaces(ctx context.Context) ([]ServiceInfo, error) {
	a.audit.Log("list", "services", "*", "")
	return a.inner.GetServicesAllNamespaces(ctx)
}
func (a *auditClient) GetEvents(ctx context.Context, ns string) ([]EventInfo, error) {
	a.audit.Log("list", "events", ns, "")
	return a.inner.GetEvents(ctx, ns)
}
func (a *auditClient) GetEventsAllNamespaces(ctx context.Context) ([]EventInfo, error) {
	a.audit.Log("list", "events", "*", "")
	return a.inner.GetEventsAllNamespaces(ctx)
}
func (a *auditClient) GetNamespaces(ctx context.Context) ([]string, error) {
	a.audit.Log("list", "namespaces", "", "")
	return a.inner.GetNamespaces(ctx)
}
func (a *auditClient) GetResourceQuotas(ctx context.Context, ns string) ([]ResourceQuotaInfo, error) {
	a.audit.Log("list", "resourcequotas", ns, "")
	return a.inner.GetResourceQuotas(ctx, ns)
}
func (a *auditClient) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	a.audit.Log("get", "clusterinfo", "", "")
	return a.inner.GetClusterInfo(ctx)
}
func (a *auditClient) GetResource(ctx context.Context, kind, name string) ([]runtime.Object, error) {
	a.audit.Log("get", kind, "", name)
	return a.inner.GetResource(ctx, kind, name)
}
func (a *auditClient) HealthCheck(ctx context.Context) error {
	return a.inner.HealthCheck(ctx)
}

func (a *auditClient) GetRawInterface() kubernetes.Interface {
	return a.inner.GetRawInterface()
}
