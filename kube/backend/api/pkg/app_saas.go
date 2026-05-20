package pkg

import (
	"log/slog"
	"strings"

	"github.com/argues/argus/internal/agentconn"
)

// LoginSaaS authenticates the local client with the central Argus SaaS.
func (a *App) LoginSaaS(provider string) (string, error) {
	a.logger.InfoContext(a.ctx, "Initiating SaaS login", slog.String("provider", provider))
	// In a real implementation, this would open a browser, perform OAuth PKCE,
	// and capture the callback token on a localhost port.
	return "mock-jwt-token-from-" + provider, nil
}

// ConnectToAgent returns anomaly entries the Argus Alerting dashboard
// can render. Sources, in order:
//
//  1. The in-cluster agent at agentConn.GetAnomalies — the canonical
//     live source. When the agent is reachable we use only that.
//  2. The webhook ingest buffer — populated by POST /webhooks/anomstack
//     when an external system (Anomstack, Grafana, custom) pushes
//     anomalies to Argus. We fall through to this when the agent
//     isn't deployed yet so the UI still reflects every detection
//     the operator has plumbed in.
//
// `namespace` filters in either source. "all" or empty returns every
// namespace. A nil/missing agentConn doesn't block the call — it
// just means the agent isn't deployed; webhook data still flows.
func (a *App) ConnectToAgent(namespace string) ([]agentconn.Anomaly, error) {
	if a.agentConn != nil {
		a.logger.InfoContext(a.ctx, "Connecting to Argus agent", slog.String("namespace", namespace))
		out, err := a.agentConn.GetAnomalies(a.ctx, namespace)
		if err == nil && len(out) > 0 {
			return out, nil
		}
		// Agent reachable but reported nothing, or unreachable — either
		// way, fall through to the webhook buffer. Don't surface the
		// agent error because the buffer is a legitimate alt source.
	}
	return a.webhookAnomalies(namespace), nil
}

// webhookAnomalies projects the rolling webhook-alert buffer into the
// AgentConn.Anomaly shape the frontend expects. Filters by namespace
// when not "all"/empty.
func (a *App) webhookAnomalies(namespace string) []agentconn.Anomaly {
	a.webhookMu.RLock()
	src := a.webhookAlerts
	out := make([]agentconn.Anomaly, 0, len(src))
	ns := strings.TrimSpace(namespace)
	wantAll := ns == "" || ns == "all"
	for _, al := range src {
		if !wantAll && al.Namespace != ns {
			continue
		}
		out = append(out, agentconn.Anomaly{
			Timestamp: al.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			Score:     anomalyScoreFromSeverity(string(al.Severity)),
			Target:    al.Namespace,
			Rule:      al.Name,
		})
	}
	a.webhookMu.RUnlock()
	return out
}

// anomalyScoreFromSeverity gives the chart a numeric 0–100 score
// when the upstream webhook didn't carry one — derived from the
// alert's discrete severity. The chart's "high-severity spike"
// threshold is 90, so critical lands above it.
func anomalyScoreFromSeverity(sev string) float64 {
	switch strings.ToLower(sev) {
	case "critical":
		return 95
	case "warning", "high":
		return 75
	case "info":
		return 45
	default:
		return 60
	}
}

// GetAgentTopology fetches the topology graph from the in-cluster agent.
func (a *App) GetAgentTopology(namespace string) (*agentconn.TopologyGraph, error) {
	if a.agentConn == nil {
		return nil, errNoCluster
	}
	return a.agentConn.GetTopology(a.ctx, namespace)
}
