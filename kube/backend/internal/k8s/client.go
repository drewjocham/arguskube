package k8s

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argues/argus/internal/alerts"
	"github.com/argues/argus/internal/config"
)

// Client wraps the Kubernetes API for Argus's needs.
type Client struct {
	cs              kubernetes.Interface
	restCfg         *rest.Config
	cfg             *config.OnlineDataConfig
	logger          *slog.Logger
	metricsProvider MetricsProvider

	// runExec is the seam pod-network-diag tests use to inject canned
	// exec output. nil in production — execInPod falls back to the real
	// SPDY-over-API exec path when this is unset. See pod_network_diag.go
	// for the rationale.
	runExec podExecFn
}

// kubeconfigLoadingRules builds the client-go loading rules from the given
// config path, supporting colon-separated multi-file KUBECONFIG values
// (e.g. "/path/a:/path/b"). When kubeconfigPath is empty, the standard
// KUBECONFIG env var and ~/.kube/config are used automatically.
func kubeconfigLoadingRules(kubeconfigPath string) *clientcmd.ClientConfigLoadingRules {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath == "" {
		return rules
	}

	// Multi-file: "/path/a:/path/b" — set Precedence so client-go merges them.
	if strings.Contains(kubeconfigPath, ":") {
		rules.Precedence = strings.Split(kubeconfigPath, ":")
		return rules
	}

	// Single file.
	rules.ExplicitPath = kubeconfigPath
	return rules
}

func NewClient(cfg *config.OnlineDataConfig, logger *slog.Logger) (*Client, error) {
	var restCfg *rest.Config
	var err error

	if cfg.Kubernetes.InCluster {
		restCfg, err = rest.InClusterConfig()
	} else {
		rules := kubeconfigLoadingRules(cfg.Kubernetes.Config)
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

	c := &Client{cs: cs, restCfg: restCfg, cfg: cfg, logger: logger}

	if url := cfg.AI.PrometheusURL; url != "" {
		c.metricsProvider = newPrometheusProvider(url, logger)
		c.logger.Info("using prometheus metrics provider", slog.String("url", url))
	} else {
		c.metricsProvider = newMetricsServerProvider(c, logger)
		c.logger.Info("using metrics-server metrics provider (set PROMETHEUS_URL for Prometheus/VictoriaMetrics)")
	}

	return c, nil
}

// GetRestConfig returns the underlying REST config for port-forwarding etc.
func (c *Client) GetRestConfig() *rest.Config {
	return c.restCfg
}

// GetClientset returns the kubernetes.Interface for direct API calls.
func (c *Client) GetClientset() kubernetes.Interface {
	return c.cs
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

	// Derive a SLO status from the data we have. An empty cluster has no
	// running workloads to breach against — treat it as ok.
	sloStatus := "ok"
	if total > 0 && (healthPct < 95 || errorRate > 5) {
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

			// Skip high-restart alert if CrashLoopBackOff is already firing for
			// this container — the CrashLoop alert is more specific and the
			// restart count is implied.
			inCrashLoop := cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff"
			if cs.RestartCount >= 5 && cs.LastTerminationState.Terminated == nil && !inCrashLoop {
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

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 256*1024), 256*1024) // handle long lines

	var lines []alerts.LogLine
	source := fmt.Sprintf("[%s]", podName)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		lines = append(lines, alerts.LogLine{
			Timestamp: time.Now(),
			Source:    source,
			Level:     inferLogLevel(line),
			Message:   line,
		})
	}
	return lines, nil
}

// inferLogLevel guesses a severity from common log patterns.
func inferLogLevel(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic"):
		return "error"
	case strings.Contains(lower, "warn"):
		return "warning"
	case strings.Contains(lower, "debug") || strings.Contains(lower, "trace"):
		return "debug"
	default:
		return "info"
	}
}

// DeletePod deletes a pod by name and namespace.
func (c *Client) DeletePod(ctx context.Context, namespace, podName string) error {
	return c.cs.CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
}

// GetWarningEvents returns the most recent warning events as formatted strings.
func (c *Client) GetWarningEvents(ctx context.Context, limit int) ([]string, error) {
	events, err := c.cs.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: "type=Warning",
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range events.Items {
		out = append(out, fmt.Sprintf("[%s] %s/%s: %s (%s, count: %d)",
			e.LastTimestamp.Format("15:04"),
			e.InvolvedObject.Namespace, e.InvolvedObject.Name,
			e.Message, e.Reason, e.Count,
		))
	}
	return out, nil
}

// GetNamespacePodCounts returns a map of namespace → running pod count.
func (c *Client) GetNamespacePodCounts(ctx context.Context) (map[string]int, error) {
	pods, err := c.cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodRunning {
			counts[p.Namespace]++
		}
	}
	return counts, nil
}

// GetTopRestarters returns the pods with the highest restart counts, formatted as strings.
func (c *Client) GetTopRestarters(ctx context.Context, limit int) ([]string, error) {
	pods, err := c.cs.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	type restartEntry struct {
		name      string
		ns        string
		container string
		restarts  int32
	}
	var entries []restartEntry

	for _, p := range pods.Items {
		for _, cs := range p.Status.ContainerStatuses {
			if cs.RestartCount > 0 {
				entries = append(entries, restartEntry{
					name: p.Name, ns: p.Namespace,
					container: cs.Name, restarts: cs.RestartCount,
				})
			}
		}
	}

	// Sort descending by restarts.
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].restarts > entries[i].restarts {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	var out []string
	for i, e := range entries {
		if i >= limit {
			break
		}
		out = append(out, fmt.Sprintf("%s/%s (container: %s) — %d restarts", e.ns, e.name, e.container, e.restarts))
	}
	return out, nil
}

// GetDeploymentRevisions returns the revision history for a deployment as ReplicaSets.
func (c *Client) GetDeploymentRevisions(ctx context.Context, namespace, deploymentName string, limit int) ([]DeploymentRevision, error) {
	dep, err := c.cs.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get deployment: %w", err)
	}

	// Find all ReplicaSets owned by this deployment.
	rsList, err := c.cs.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list replicasets: %w", err)
	}

	var revisions []DeploymentRevision
	for _, rs := range rsList.Items {
		// Check ownership.
		owned := false
		for _, ref := range rs.OwnerReferences {
			if ref.UID == dep.UID {
				owned = true
				break
			}
		}
		if !owned {
			continue
		}

		rev := DeploymentRevision{
			Revision:      rs.Annotations["deployment.kubernetes.io/revision"],
			ReplicaSet:    rs.Name,
			Replicas:      int(rs.Status.Replicas),
			ReadyReplicas: int(rs.Status.ReadyReplicas),
			CreatedAt:     rs.CreationTimestamp.Time,
			Active:        rs.Status.Replicas > 0,
		}

		// Extract image from pod template.
		if len(rs.Spec.Template.Spec.Containers) > 0 {
			rev.Image = rs.Spec.Template.Spec.Containers[0].Image
		}

		// Extract change cause annotation.
		if cause, ok := rs.Annotations["kubernetes.io/change-cause"]; ok {
			rev.ChangeCause = cause
		}

		revisions = append(revisions, rev)
	}

	// Sort by revision number descending.
	for i := 0; i < len(revisions); i++ {
		for j := i + 1; j < len(revisions); j++ {
			if revisions[j].Revision > revisions[i].Revision {
				revisions[i], revisions[j] = revisions[j], revisions[i]
			}
		}
	}

	if limit > 0 && len(revisions) > limit {
		revisions = revisions[:limit]
	}

	return revisions, nil
}

// DeploymentRevision represents one ReplicaSet revision of a Deployment.
type DeploymentRevision struct {
	Revision      string    `json:"revision"`
	ReplicaSet    string    `json:"replicaSet"`
	Image         string    `json:"image"`
	Replicas      int       `json:"replicas"`
	ReadyReplicas int       `json:"readyReplicas"`
	Active        bool      `json:"active"`
	ChangeCause   string    `json:"changeCause,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// StreamPodLogs streams logs from a pod with follow support via a callback.
func (c *Client) StreamPodLogs(ctx context.Context, namespace, podName, container string, tailLines int64, follow bool, callback func(line string)) error {
	opts := &corev1.PodLogOptions{
		Follow:    follow,
		TailLines: &tailLines,
	}
	if container != "" {
		opts.Container = container
	}

	stream, err := c.cs.CoreV1().Pods(namespace).GetLogs(podName, opts).Stream(ctx)
	if err != nil {
		return fmt.Errorf("open log stream: %w", err)
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line
	for scanner.Scan() {
		callback(scanner.Text())
	}
	return scanner.Err()
}

// ContextInfo describes a kubeconfig context entry.
type ContextInfo struct {
	Name    string `json:"name"`
	Cluster string `json:"cluster"`
	Active  bool   `json:"active"`
}

// ListContexts reads the kubeconfig and returns all available contexts.
func (c *Client) ListContexts() ([]ContextInfo, error) {
	kubeconfigPath := ""
	activeOverride := ""
	if c.cfg != nil {
		kubeconfigPath = c.cfg.Kubernetes.Config
		activeOverride = c.cfg.Kubernetes.Context
	}
	return ListContextsFromKubeconfig(kubeconfigPath, activeOverride)
}

// ListContextsFromKubeconfig reads the kubeconfig file and returns all available
// contexts. It does not require an active k8s client, so it works even when the
// cluster is unreachable. kubeconfigPath may be empty (uses default loading
// rules). activeOverride, if non-empty, marks that context as active.
func ListContextsFromKubeconfig(kubeconfigPath, activeOverride string) ([]ContextInfo, error) {
	rules := kubeconfigLoadingRules(kubeconfigPath)

	rawCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).RawConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	activeCtx := activeOverride
	if activeCtx == "" || activeCtx == "default" {
		activeCtx = rawCfg.CurrentContext
	}

	var out []ContextInfo
	for name, ctx := range rawCfg.Contexts {
		out = append(out, ContextInfo{
			Name:    name,
			Cluster: ctx.Cluster,
			Active:  name == activeCtx,
		})
	}

	// Sort alphabetically, active first.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		return out[i].Name < out[j].Name
	})

	return out, nil
}

// SwitchContext reinitializes the client with a different kubeconfig context.
func (c *Client) SwitchContext(contextName string) error {
	rules := kubeconfigLoadingRules(c.cfg.Kubernetes.Config)
	overrides := &clientcmd.ConfigOverrides{CurrentContext: contextName}

	restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return fmt.Errorf("switch context %q: %w", contextName, err)
	}

	cs, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("new clientset for %q: %w", contextName, err)
	}

	c.cs = cs
	c.restCfg = restCfg
	c.cfg.Kubernetes.Context = contextName
	c.logger.Info("switched k8s context", slog.String("context", contextName))
	return nil
}

// VPARecommendation holds a single VPA object's recommendations.
type VPARecommendation struct {
	Name       string                  `json:"name"`
	Namespace  string                  `json:"namespace"`
	TargetRef  string                  `json:"targetRef"` // e.g. "Deployment/web-app"
	UpdateMode string                  `json:"updateMode"`
	Containers []VPAContainerRecommend `json:"containers"`
	CreatedAt  time.Time               `json:"createdAt"`
}

// VPAContainerRecommend holds per-container resource recommendations.
type VPAContainerRecommend struct {
	ContainerName  string `json:"containerName"`
	LowerCPU       string `json:"lowerCpu"`
	LowerMemory    string `json:"lowerMemory"`
	TargetCPU      string `json:"targetCpu"`
	TargetMemory   string `json:"targetMemory"`
	UpperCPU       string `json:"upperCpu"`
	UpperMemory    string `json:"upperMemory"`
	UncappedCPU    string `json:"uncappedCpu,omitempty"`
	UncappedMemory string `json:"uncappedMemory,omitempty"`
}

// GetVPARecommendations reads VerticalPodAutoscaler objects from the cluster
// using the dynamic client (autoscaling.k8s.io/v1).
func (c *Client) GetVPARecommendations(ctx context.Context, namespace string) ([]VPARecommendation, error) {
	dynClient, err := dynamic.NewForConfig(c.restCfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	vpaGVR := schema.GroupVersionResource{
		Group:    "autoscaling.k8s.io",
		Version:  "v1",
		Resource: "verticalpodautoscalers",
	}

	var list *unstructured.UnstructuredList
	if namespace != "" {
		list, err = dynClient.Resource(vpaGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		list, err = dynClient.Resource(vpaGVR).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("list VPAs: %w", err)
	}

	var out []VPARecommendation
	for _, item := range list.Items {
		vpa := VPARecommendation{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			CreatedAt: item.GetCreationTimestamp().Time,
		}

		// Parse spec.targetRef
		if spec, ok := item.Object["spec"].(map[string]interface{}); ok {
			if tr, ok := spec["targetRef"].(map[string]interface{}); ok {
				kind, _ := tr["kind"].(string)
				name, _ := tr["name"].(string)
				vpa.TargetRef = kind + "/" + name
			}
			if up, ok := spec["updatePolicy"].(map[string]interface{}); ok {
				if mode, ok := up["updateMode"].(string); ok {
					vpa.UpdateMode = mode
				}
			}
		}

		// Parse status.recommendation.containerRecommendations
		if status, ok := item.Object["status"].(map[string]interface{}); ok {
			if rec, ok := status["recommendation"].(map[string]interface{}); ok {
				if containers, ok := rec["containerRecommendations"].([]interface{}); ok {
					for _, cr := range containers {
						crMap, ok := cr.(map[string]interface{})
						if !ok {
							continue
						}
						vcr := VPAContainerRecommend{}
						vcr.ContainerName, _ = crMap["containerName"].(string)

						extractRes := func(key string) (string, string) {
							if m, ok := crMap[key].(map[string]interface{}); ok {
								cpu, _ := m["cpu"].(string)
								mem, _ := m["memory"].(string)
								return cpu, mem
							}
							return "", ""
						}

						vcr.LowerCPU, vcr.LowerMemory = extractRes("lowerBound")
						vcr.TargetCPU, vcr.TargetMemory = extractRes("target")
						vcr.UpperCPU, vcr.UpperMemory = extractRes("upperBound")
						vcr.UncappedCPU, vcr.UncappedMemory = extractRes("uncappedTarget")

						vpa.Containers = append(vpa.Containers, vcr)
					}
				}
			}
		}

		out = append(out, vpa)
	}

	return out, nil
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
