package agentconn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

const (
	agentLabelSelector = "app.kubernetes.io/name=argus-agent"
	agentContainerPort = 8080
	defaultNamespace   = "argus"
)

// Anomaly represents an anomaly score from the in-cluster agent.
type Anomaly struct {
	Timestamp string  `json:"timestamp"`
	Score     float64 `json:"score"`
	Target    string  `json:"target"`
	Rule      string  `json:"rule"`
}

// TopologyGraph is the node-edge graph from /api/v1/topology.
type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// TopologyNode represents a vertex in the topology graph.
type TopologyNode struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
}

// TopologyEdge connects two topology nodes.
type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Connector manages a port-forward to the argus-agent pod.
type Connector struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
	logger     *slog.Logger
	namespace  string
}

// New creates an agent connector.
func New(cs kubernetes.Interface, restCfg *rest.Config, logger *slog.Logger) *Connector {
	return &Connector{
		clientset:  cs,
		restConfig: restCfg,
		logger:     logger,
		namespace:  defaultNamespace,
	}
}

// findAgentPod locates a running argus-agent pod.
func (c *Connector) findAgentPod(ctx context.Context, namespace string) (*corev1.Pod, error) {
	ns := namespace
	if ns == "" || ns == "all" {
		ns = c.namespace
	}

	pods, err := c.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: agentLabelSelector,
		Limit:         1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list agent pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no argus-agent pod found in namespace %s", ns)
	}

	pod := &pods.Items[0]
	if pod.Status.Phase != corev1.PodRunning {
		return nil, fmt.Errorf("agent pod %s is not running (phase: %s)", pod.Name, pod.Status.Phase)
	}
	return pod, nil
}

// portForwardAndCall sets up a temporary port-forward, calls the given path, and returns the body.
func (c *Connector) portForwardAndCall(ctx context.Context, namespace, path string) ([]byte, error) {
	pod, err := c.findAgentPod(ctx, namespace)
	if err != nil {
		return nil, err
	}

	// Pick a free local port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to find free port: %w", err)
	}
	localPort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Build the port-forward request.
	reqURL := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(c.restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPDY transport: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, reqURL)

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})

	// stopCh has three would-be closers (timeout branch, defer, future
	// ctx-cancel paths) and closing a closed channel panics. Wrap the
	// close in a sync.Once so any reachable branch is safe — and add an
	// unconditional defer so error paths can't leak the ForwardPorts
	// goroutine the way the old code did when the error branch returned
	// without closing stopCh at all.
	var stopOnce sync.Once
	closeStop := func() { stopOnce.Do(func() { close(stopCh) }) }
	defer closeStop()

	ports := []string{fmt.Sprintf("%d:%d", localPort, agentContainerPort)}
	fw, err := portforward.New(dialer, ports, stopCh, readyCh, io.Discard, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("failed to create port-forward: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- fw.ForwardPorts()
	}()

	// Wait for port-forward to be ready.
	select {
	case <-readyCh:
	case err := <-errCh:
		return nil, fmt.Errorf("port-forward failed: %w", err)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("port-forward timed out")
	}

	// Make the HTTP call through the forwarded port.
	url := fmt.Sprintf("http://127.0.0.1:%d%s", localPort, path)
	c.logger.Info("calling agent", slog.String("url", url))

	httpCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent HTTP call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent returned %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// GetAnomalies fetches anomaly scores from the agent.
func (c *Connector) GetAnomalies(ctx context.Context, namespace string) ([]Anomaly, error) {
	data, err := c.portForwardAndCall(ctx, namespace, "/api/v1/anomalies")
	if err != nil {
		return nil, err
	}

	var anomalies []Anomaly
	if err := json.Unmarshal(data, &anomalies); err != nil {
		return nil, fmt.Errorf("failed to decode anomalies: %w", err)
	}
	return anomalies, nil
}

// GetTopology fetches the topology graph from the agent.
func (c *Connector) GetTopology(ctx context.Context, namespace string) (*TopologyGraph, error) {
	data, err := c.portForwardAndCall(ctx, namespace, "/api/v1/topology")
	if err != nil {
		return nil, err
	}

	var graph TopologyGraph
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil, fmt.Errorf("failed to decode topology: %w", err)
	}
	return &graph, nil
}

// GetEvents fetches cluster events from the agent.
func (c *Connector) GetEvents(ctx context.Context, namespace string) (json.RawMessage, error) {
	data, err := c.portForwardAndCall(ctx, namespace, "/api/v1/events")
	if err != nil {
		return nil, err
	}
	return data, nil
}
