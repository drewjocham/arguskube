package pkg

import (
	"fmt"

	"github.com/argues/kube-watcher/internal/ai"
	"github.com/argues/kube-watcher/internal/alerts"
	ctxassembly "github.com/argues/kube-watcher/internal/context"
	"github.com/argues/kube-watcher/internal/features"
	"github.com/argues/kube-watcher/internal/popeye"
)

// DiagnoseAlert assembles context and generates a diagnosis for a specific alert.
func (a *App) DiagnoseAlert(alertID string) (*ctxassembly.Bundle, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	if !a.gate.Allowed(features.FeatureAIDiagnostics) {
		return nil, features.ErrProRequired
	}

	allAlerts, err := a.k8s.DetectAlerts(a.ctx)
	if err != nil {
		return nil, err
	}

	var target *alerts.Alert
	for i := range allAlerts {
		if allAlerts[i].ID == alertID {
			target = &allAlerts[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("alert %q not found", alertID)
	}

	return a.assembler.Assemble(a.ctx, *target, allAlerts)
}

// SendChatMessage sends a user message to the AI agent for a given alert context.
func (a *App) SendChatMessage(alertID string, message string) (string, error) {
	if a.agent == nil {
		return "", fmt.Errorf("AI agent not configured — set DEEPSEEK_API_KEY")
	}
	if !a.gate.Allowed(features.FeatureAIDiagnostics) {
		return "", features.ErrProRequired
	}

	// Build full diagnostic context for the AI agent.
	var alert *alerts.Alert
	var allAlerts []alerts.Alert

	if a.k8s != nil {
		var err error
		allAlerts, err = a.k8s.DetectAlerts(a.ctx)
		if err == nil && alertID != "global" {
			for i := range allAlerts {
				if allAlerts[i].ID == alertID {
					alert = &allAlerts[i]
					break
				}
			}
		}
	}

	// Assemble the enriched diagnostic context.
	diagCtx := &ai.DiagnosticContext{
		Metrics: a.cachedMetrics,
	}

	// Add recent warning events from the cluster.
	if a.k8s != nil {
		if events, err := a.k8s.GetWarningEvents(a.ctx, 20); err == nil {
			diagCtx.RecentEvents = events
		}

		// Add namespace pod counts for situational awareness.
		if nsSummary, err := a.k8s.GetNamespacePodCounts(a.ctx); err == nil {
			diagCtx.NamespaceSummary = nsSummary
		}

		// Add pods with high restart counts.
		if restarters, err := a.k8s.GetTopRestarters(a.ctx, 5); err == nil {
			diagCtx.TopRestarters = restarters
		}
	}

	// Add cascade-correlated alerts if we have a target alert.
	if alert != nil && a.assembler != nil {
		if bundle, err := a.assembler.Assemble(a.ctx, *alert, allAlerts); err == nil && bundle != nil {
			diagCtx.CascadeAlerts = bundle.CascadeAlerts
		}
	}

	return a.agent.SendMessage(a.ctx, alertID, message, alert, diagCtx)
}

// GetChatHistory returns the conversation history for an alert.
func (a *App) GetChatHistory(alertID string) []ai.ChatEntry {
	if a.agent == nil {
		return nil
	}
	return a.agent.GetChatHistory(alertID)
}

// GetAutoSummary returns the auto-investigation summary for an alert.
func (a *App) GetAutoSummary(alertID string) *ai.AutoSummary {
	if a.agent == nil {
		return nil
	}
	return a.agent.GetAutoSummary(alertID)
}

// GetAgentEventLog returns the agent's tracked events and patterns.
func (a *App) GetAgentEventLog() []ai.AgentEvent {
	if a.agent == nil {
		return nil
	}
	return a.agent.GetEventLog()
}

// RunArgusScan executes a cluster scan and returns structured findings.
func (a *App) RunArgusScan() (*popeye.Report, error) {
	if a.popeye == nil {
		return nil, fmt.Errorf("argus scan not configured — install argus scan CLI (popeye)")
	}
	return a.popeye.Run(a.ctx)
}
