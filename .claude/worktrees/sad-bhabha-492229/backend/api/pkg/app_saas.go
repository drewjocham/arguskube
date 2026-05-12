package pkg

import (
	"log/slog"

	"github.com/argues/kube-watcher/internal/agentconn"
)

// LoginSaaS authenticates the local client with the central KubeWatcher SaaS.
func (a *App) LoginSaaS(provider string) (string, error) {
	a.logger.InfoContext(a.ctx, "Initiating SaaS login", slog.String("provider", provider))
	// In a real implementation, this would open a browser, perform OAuth PKCE,
	// and capture the callback token on a localhost port.
	return "mock-jwt-token-from-" + provider, nil
}

// ConnectToAgent performs a port-forward to the in-cluster agent and fetches anomaly scores.
func (a *App) ConnectToAgent(namespace string) ([]agentconn.Anomaly, error) {
	if a.agentConn == nil {
		return nil, errNoCluster
	}
	a.logger.InfoContext(a.ctx, "Connecting to KubeWatcher agent", slog.String("namespace", namespace))
	return a.agentConn.GetAnomalies(a.ctx, namespace)
}

// GetAgentTopology fetches the topology graph from the in-cluster agent.
func (a *App) GetAgentTopology(namespace string) (*agentconn.TopologyGraph, error) {
	if a.agentConn == nil {
		return nil, errNoCluster
	}
	return a.agentConn.GetTopology(a.ctx, namespace)
}
