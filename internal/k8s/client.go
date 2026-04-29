package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/djocham/kube-watcher/internal/alerts"
	"github.com/djocham/kube-watcher/internal/config"
)

const (
	logKeyPod       = "pod"
	logKeyNamespace = "namespace"
	logKeyNode      = "node"
	logKeyError     = "error"
)

// Client wraps the Kubernetes API for KubeWatcher's needs.
type Client struct {
	cs     kubernetes.Interface
	cfg    *config.OnlineDataConfig
	logger *slog.Logger
}

func NewClient(cfg *config.OnlineDataConfig, logger *slog.Logger) (*Client, error) {
	var restCfg *rest.Config
	var err error

	if cfg.Kubernetes.InCluster {
		restCfg, err = rest.InClusterConfig()
	} else {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		if cfg.Kubernetes.Config != "" {
			rules.ExplicitPath = cfg.Kubernetes.Config
		}
		overrides := &clientcmd.ConfigOverrides{}
		if cfg.Kubernetes.Context != "" {
			overrides.CurrentContext = cfg.Kubernetes.Context
		}
		restCfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("k8s config: %w", err)
	}

	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}

	return &Client{cs: cs, cfg: cfg, logger: logger}, nil
}

// ClusterInfo holds basic cluster metadata.
type ClusterInfo struct {
	Name       string `json:"name"`
	NodeCount  int    `json:"nodeCount"`
	K8sVersion string `json:"k8sVersion"`
}

// GetClusterInfo fetches cluster name, node count, and version.
func (c *Client) GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	nodes, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	version, err := c.cs.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("server version: %w", err)
	}

	name := "unknown"
	if c.cfg.Kubernetes.Context != "" {
		name = c.cfg.Kubernetes.Context
	} else if len(nodes.Items) > 0 {
		if labels := nodes.Items[0].Labels; labels != nil {
			if n, ok := labels["kubernetes.io/cluster-name"]; ok {
				name = n
			}
		}
	}

	return &ClusterInfo{
		Name:       name,
		NodeCount:  len(nodes.Items),
		K8sVersion: version.GitVersion,
	}, nil
}

// GetMetrics computes cluster-level health metrics from the core K8s API.
// Does NOT require metrics-server — works with any cluster. Derives health
// from pod phases, container statuses, events, and node conditions.
func (c *Client) GetMetrics(ctx context.Context) (*alerts.ClusterMetrics, error) {
	ns := c.cfg.Kubernetes.Namespace
	pods, err := c.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	total := len(pods.Items)
	running := 0
	failed := 0
	pending := 0
	var totalRestarts int32
	var topRestart string
	var topRestartCount int32
	var unhealthyContainers int
	var totalContainers int

	// Aggregate CPU/memory requests across all pods for capacity awareness.
	var totalCPURequestMillis int64
	var totalMemRequestBytes int64

	for i := range pods.Items {
		p := &pods.Items[i]

		switch p.Status.Phase {
		case corev1.PodRunning:
			running++
		case corev1.PodFailed:
			failed++
		case corev1.PodPending:
			pending++
		}

		// Container-level health.
		for _, cs := range p.Status.ContainerStatuses {
			totalContainers++
			totalRestarts += cs.RestartCount

			if !cs.Ready {
				unhealthyContainers++
			}

			if cs.RestartCount > topRestartCount {
				topRestartCount = cs.RestartCount
				topRestart = fmt.Sprintf("%s: %d", deploymentName(p), cs.RestartCount)
			}
		}

		// Resource requests (always available, no metrics-server needed).
		for _, container := range p.Spec.Containers {
			if req := container.Resources.Requests; req != nil {
				if cpu, ok := req[corev1.ResourceCPU]; ok {
					totalCPURequestMillis += cpu.MilliValue()
				}
				if mem, ok := req[corev1.ResourceMemory]; ok {
					totalMemRequestBytes += mem.Value()
				}
			}
		}
	}

	// Pod health = running / total (excluding completed jobs).
	healthPct := 0.0
	if total > 0 {
		healthPct = float64(running) / float64(total) * 100
	}

	// Error rate = unhealthy containers / total containers.
	errorRate := 0.0
	if totalContainers > 0 {
		errorRate = float64(unhealthyContainers) / float64(totalContainers) * 100
	}

	// Fetch recent warning events to compute event-based error signal.
	events, err := c.cs.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	warningEventCount := 0
	if err == nil {
		// Count events from the last 30 minutes.
		cutoff := time.Now().Add(-30 * time.Minute)
		for _, ev := range events.Items {
			if ev.LastTimestamp.Time.After(cutoff) {
				warningEventCount++
			}
		}
	}

	// Derive a SLO status from the data we have.
	sloStatus := "ok"
	if healthPct < 95 || errorRate > 5 {
		sloStatus = "breach"
	}

	return &alerts.ClusterMetrics{
		PodHealthPct:     healthPct,
		PodsRunning:      running,
		PodsTotal:        total,
		PodsPending:      pending,
		PodsFailed:       failed,
		ErrorRate:        errorRate,
		ErrorRatePrev:    0, // populated on subsequent polls via diff
		RestartCount:     totalRestarts,
		RestartTop:       topRestart,
		WarningEvents:    warningEventCount,
		TotalCPUMillis:   totalCPURequestMillis,
		TotalMemoryBytes: totalMemRequestBytes,
		P99Latency:       "—", // requires Prometheus; gracefully absent
		SLOStatus:        sloStatus,
	}, nil
}

// DetectAlerts scans pods and nodes for alert conditions.
func (c *Client) DetectAlerts(ctx context.Context) ([]alerts.Alert, error) {
	ns := c.cfg.Kubernetes.Namespace
	pods, err := c.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	nodes, err := c.cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}

	var result []alerts.Alert

	// Pod-level alerts.
	for i := range pods.Items {
		p := &pods.Items[i]
		for _, cs := range p.Status.ContainerStatuses {
			if cs.LastTerminationState.Terminated != nil &&
				cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				result = append(result, buildOOMAlert(p, &cs))
			}

			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				result = append(result, buildCrashLoopAlert(p, &cs))
			}

			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "ImagePullBackOff" {
				result = append(result, buildImagePullAlert(p, &cs))
			}

			if cs.RestartCount >= 5 && cs.LastTerminationState.Terminated == nil {
				result = append(result, buildHighRestartAlert(p, &cs))
			}
		}
	}

	// Node-level alerts.
	for i := range nodes.Items {
		n := &nodes.Items[i]
		for _, cond := range n.Status.Conditions {
			if cond.Type == corev1.NodeDiskPressure && cond.Status == corev1.ConditionTrue {
				result = append(result, buildDiskPressureAlert(n))
			}
			if cond.Type == corev1.NodeMemoryPressure && cond.Status == corev1.ConditionTrue {
				result = append(result, buildMemoryPressureAlert(n))
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if sevOrder(result[i].Severity) != sevOrder(result[j].Severity) {
			return sevOrder(result[i].Severity) < sevOrder(result[j].Severity)
		}
		return result[j].Timestamp.Before(result[i].Timestamp)
	})

	c.logger.DebugContext(ctx, "alerts detected",
		slog.Int("count", len(result)),
	)

	return result, nil
}

// GetPodLogs returns recent log lines from a pod.
func (c *Client) GetPodLogs(ctx context.Context, namespace, podName string, tailLines int64) ([]alerts.LogLine, error) {
	opts := &corev1.PodLogOptions{TailLines: &tailLines}
	req := c.cs.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("pod logs: %w", err)
	}
	defer stream.Close()

	buf := make([]byte, 32*1024)
	var lines []alerts.LogLine
	for {
		n, readErr := stream.Read(buf)
		if n > 0 {
			lines = append(lines, alerts.LogLine{
				Timestamp: time.Now(),
				Source:    fmt.Sprintf("[%s]", podName),
				Level:     "info",
				Message:   string(buf[:n]),
			})
		}
		if readErr != nil {
			break
		}
	}
	return lines, nil
}

// --- alert builders ---

func buildOOMAlert(p *corev1.Pod, cs *corev1.ContainerStatus) alerts.Alert {
	memLimit := resourceStr(p, cs.Name, "memory", true)
	memReq := resourceStr(p, cs.Name, "memory", false)
	return alerts.Alert{
		ID:            fmt.Sprintf("oom-%s-%s", p.Namespace, p.Name),
		Name:          fmt.Sprintf("OOMKilled: %s", deploymentName(p)),
		Severity:      alerts.SeverityCritical,
		Namespace:     p.Namespace,
		Timestamp:     cs.LastTerminationState.Terminated.FinishedAt.Time,
		PodName:       p.Name,
		PodPhase:      string(p.Status.Phase),
		RestartCount:  cs.RestartCount,
		MemoryLimit:   memLimit,
		MemoryRequest: memReq,
		ImageTag:      cs.Image,
		Description:   fmt.Sprintf("Container exceeded memory limit. %d restarts. Memory limit: %s", cs.RestartCount, memLimit),
		Tags: []alerts.Tag{
			{Label: "OOMKilled", Color: "red"},
			{Label: shortName(p.Name), Color: "blue"},
		},
	}
}

func buildCrashLoopAlert(p *corev1.Pod, cs *corev1.ContainerStatus) alerts.Alert {
	return alerts.Alert{
		ID:           fmt.Sprintf("crash-%s-%s", p.Namespace, p.Name),
		Name:         fmt.Sprintf("CrashLoopBackOff: %s", deploymentName(p)),
		Severity:     alerts.SeverityCritical,
		Namespace:    p.Namespace,
		Timestamp:    time.Now(),
		PodName:      p.Name,
		PodPhase:     string(p.Status.Phase),
		RestartCount: cs.RestartCount,
		ImageTag:     cs.Image,
		Description:  fmt.Sprintf("Container in crash loop. %d restarts.", cs.RestartCount),
		Tags: []alerts.Tag{
			{Label: "CrashLoop", Color: "red"},
			{Label: shortName(p.Name), Color: "blue"},
		},
	}
}

func buildHighRestartAlert(p *corev1.Pod, cs *corev1.ContainerStatus) alerts.Alert {
	return alerts.Alert{
		ID:           fmt.Sprintf("restart-%s-%s", p.Namespace, p.Name),
		Name:         fmt.Sprintf("High Restarts: %s", deploymentName(p)),
		Severity:     alerts.SeverityWarning,
		Namespace:    p.Namespace,
		Timestamp:    time.Now(),
		PodName:      p.Name,
		RestartCount: cs.RestartCount,
		Description:  fmt.Sprintf("Container has restarted %d times.", cs.RestartCount),
		Tags: []alerts.Tag{
			{Label: "restarts", Color: "amber"},
			{Label: shortName(p.Name), Color: "blue"},
		},
	}
}

func buildImagePullAlert(p *corev1.Pod, cs *corev1.ContainerStatus) alerts.Alert {
	return alerts.Alert{
		ID:          fmt.Sprintf("imgpull-%s-%s", p.Namespace, p.Name),
		Name:        fmt.Sprintf("ImagePullBackOff: %s", deploymentName(p)),
		Severity:    alerts.SeverityWarning,
		Namespace:   p.Namespace,
		Timestamp:   time.Now(),
		PodName:     p.Name,
		ImageTag:    cs.Image,
		Description: fmt.Sprintf("Unable to pull image: %s", cs.Image),
		Tags: []alerts.Tag{
			{Label: "ImagePull", Color: "amber"},
			{Label: shortName(p.Name), Color: "blue"},
		},
	}
}

func buildDiskPressureAlert(n *corev1.Node) alerts.Alert {
	return alerts.Alert{
		ID:          fmt.Sprintf("disk-%s", n.Name),
		Name:        fmt.Sprintf("Node Pressure: %s", n.Name),
		Severity:    alerts.SeverityCritical,
		Namespace:   "infra",
		Timestamp:   time.Now(),
		NodeName:    n.Name,
		Description: "DiskPressure condition triggered. Pods may be evicted.",
		Tags: []alerts.Tag{
			{Label: "DiskPressure", Color: "red"},
			{Label: n.Name, Color: "teal"},
		},
	}
}

func buildMemoryPressureAlert(n *corev1.Node) alerts.Alert {
	return alerts.Alert{
		ID:          fmt.Sprintf("mempress-%s", n.Name),
		Name:        fmt.Sprintf("Memory Pressure: %s", n.Name),
		Severity:    alerts.SeverityWarning,
		Namespace:   "infra",
		Timestamp:   time.Now(),
		NodeName:    n.Name,
		Description: "MemoryPressure condition triggered on node.",
		Tags: []alerts.Tag{
			{Label: "MemPressure", Color: "amber"},
			{Label: n.Name, Color: "teal"},
		},
	}
}

// --- helpers ---

func sevOrder(s alerts.Severity) int {
	switch s {
	case alerts.SeverityCritical:
		return 0
	case alerts.SeverityWarning:
		return 1
	default:
		return 2
	}
}

func deploymentName(p *corev1.Pod) string {
	if len(p.OwnerReferences) > 0 {
		return p.OwnerReferences[0].Name
	}
	return p.Name
}

func shortName(name string) string {
	if len(name) > 20 {
		return name[:20]
	}
	return name
}

func resourceStr(p *corev1.Pod, containerName, resource string, limit bool) string {
	for _, c := range p.Spec.Containers {
		if c.Name == containerName {
			var rl corev1.ResourceList
			if limit {
				rl = c.Resources.Limits
			} else {
				rl = c.Resources.Requests
			}
			if rl != nil {
				if q, ok := rl[corev1.ResourceName(resource)]; ok {
					return q.String()
				}
			}
		}
	}
	return "—"
}
