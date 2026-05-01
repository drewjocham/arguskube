package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/kube-watcher/internal/agentconn"
	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/argues/kube-watcher/internal/config"
	"github.com/argues/kube-watcher/internal/features"
	"github.com/argues/kube-watcher/internal/k8s"
)

// GetClusterInfo returns cluster metadata.
func (a *App) GetClusterInfo() (*k8s.ClusterInfo, error) {
	if a.k8s == nil {
		return &k8s.ClusterInfo{Name: "not connected", NodeCount: 0, K8sVersion: "—"}, nil
	}
	return a.k8s.GetClusterInfo(a.ctx)
}

// ListContexts returns all available kubeconfig contexts. Works even when no
// cluster is connected — it reads the kubeconfig file directly.
func (a *App) ListContexts() ([]k8s.ContextInfo, error) {
	if a.k8s != nil {
		return a.k8s.ListContexts()
	}
	// No client yet — read kubeconfig directly so the user can pick a context.
	kubeconfigPath := ""
	if a.cfg != nil {
		kubeconfigPath = a.cfg.Kubernetes.Config
	}
	return k8s.ListContextsFromKubeconfig(kubeconfigPath, "")
}

// SwitchContext changes the active kubeconfig context at runtime. If no k8s
// client exists yet (e.g. initial connection failed), it creates one.
func (a *App) SwitchContext(name string) error {
	if a.k8s == nil {
		// First connection — bootstrap a client targeting this context.
		a.logger.Info("bootstrapping k8s client on first context switch",
			slog.String("context", name),
		)
		client, err := k8s.NewClient(a.cfg, a.logger)
		if err != nil {
			return fmt.Errorf("create k8s client: %w", err)
		}
		if err := client.SwitchContext(name); err != nil {
			return err
		}
		a.k8s = client
	} else {
		if err := a.k8s.SwitchContext(name); err != nil {
			return err
		}
	}
	// Rebuild the agent connector with the new client.
	a.agentConn = agentconn.New(
		a.k8s.GetClientset(),
		a.k8s.GetRestConfig(),
		a.logger.With("component", "agentconn"),
	)
	return nil
}

// GetMetrics returns cluster health metrics.
func (a *App) GetMetrics() (*alerts.ClusterMetrics, error) {
	if a.k8s == nil {
		return &alerts.ClusterMetrics{SLOStatus: "unknown"}, nil
	}
	m, err := a.k8s.GetMetrics(a.ctx)
	if err == nil && m != nil {
		a.cachedMetrics = m
	}
	return m, err
}

// HandleURL handles deep links from custom URL schemes like kubewatcher://
func (a *App) HandleURL(u string) {
	a.logger.Info("Received custom URL", slog.String("url", u))
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "deep-link", u)
	}
}

// GetAlerts returns enriched alerts from the cluster.
func (a *App) GetAlerts() ([]alerts.Alert, error) {
	if a.k8s == nil {
		return nil, nil
	}
	return a.k8s.DetectAlerts(a.ctx)
}

// GetPodLogs returns recent log lines for a pod.
func (a *App) GetPodLogs(namespace, podName string, tailLines int64) ([]alerts.LogLine, error) {
	if a.k8s == nil {
		return nil, nil
	}
	return a.k8s.GetPodLogs(a.ctx, namespace, podName, tailLines)
}

// StreamPodLogsFollow returns log lines with follow enabled (streams until context canceled).
func (a *App) StreamPodLogsFollow(namespace, podName, container string, tailLines int64) ([]string, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	// For Wails bindings we collect into a buffer (WebSocket streaming is via the agent).
	// Limit to 500 lines to prevent memory issues in the desktop app.
	var lines []string
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	err := a.k8s.StreamPodLogs(ctx, namespace, podName, container, tailLines, false, func(line string) {
		if len(lines) < 500 {
			lines = append(lines, line)
		}
	})
	return lines, err
}

// GetDeploymentRevisions returns the rollout history for a deployment.
func (a *App) GetDeploymentRevisions(namespace, deployment string, limit int) ([]k8s.DeploymentRevision, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	if limit <= 0 {
		limit = 25
	}
	return a.k8s.GetDeploymentRevisions(a.ctx, namespace, deployment, limit)
}

// GetNodeLogs fetches real kubelet/containerd/kube-proxy logs from a cluster node
// via the kubelet proxy API.
func (a *App) GetNodeLogs(nodeName string, tailLines int) ([]k8s.NodeLogEntry, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.GetNodeLogs(a.ctx, nodeName, nil, tailLines)
}

// GetVPARecommendations returns VerticalPodAutoscaler recommendations from the cluster.
func (a *App) GetVPARecommendations(namespace string) ([]k8s.VPARecommendation, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.GetVPARecommendations(a.ctx, namespace)
}

// GetFeatures returns all features and their availability for the current tier.
func (a *App) GetFeatures() map[features.Feature]bool {
	return a.gate.AllFeatures()
}

// GetTier returns the current subscription tier.
func (a *App) GetTier() config.Tier {
	return a.gate.Tier()
}

// --- Terminal bindings ---

const EventTerminalOutput = "terminal:output"

// StartTerminal opens a PTY shell session.
func (a *App) StartTerminal(rows, cols int) error {
	a.term.OnOutput = func(data string) {
		runtime.EventsEmit(a.ctx, EventTerminalOutput, data)
	}
	return a.term.Start("", uint16(rows), uint16(cols))
}

// SendTerminalInput writes raw input to the terminal.
func (a *App) SendTerminalInput(data string) error {
	return a.term.Write(data)
}

// ResizeTerminal updates the terminal dimensions.
func (a *App) ResizeTerminal(rows, cols int) error {
	return a.term.Resize(uint16(rows), uint16(cols))
}
