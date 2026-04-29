package pkg

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/djocham/kube-watcher/internal/ai"
	"github.com/djocham/kube-watcher/internal/alerts"
	"github.com/djocham/kube-watcher/internal/anomaly"
	"github.com/djocham/kube-watcher/internal/config"
	ctxassembly "github.com/djocham/kube-watcher/internal/context"
	"github.com/djocham/kube-watcher/internal/features"
	"github.com/djocham/kube-watcher/internal/k8s"
	"github.com/djocham/kube-watcher/internal/popeye"
	"github.com/djocham/kube-watcher/internal/terminal"
)

// AppConfig holds all dependencies for the application. Flat, explicit.
type AppConfig struct {
	Logger    *slog.Logger
	Config    *config.OnlineDataConfig
	K8sClient *k8s.Client
	Gate      *features.Gate
	Assembler *ctxassembly.Assembler
	Detector  anomaly.Detector
	Agent     *ai.Agent
	Popeye    *popeye.Runner
}

// App is the main application struct exposed to Wails as bindings.
type App struct {
	ctx       context.Context
	logger    *slog.Logger
	cfg       *config.OnlineDataConfig
	k8s       *k8s.Client
	gate      *features.Gate
	assembler *ctxassembly.Assembler
	detector  anomaly.Detector
	agent     *ai.Agent
	popeye    *popeye.Runner
	term      *terminal.Terminal

	// cachedMetrics holds the latest metrics for agent context.
	cachedMetrics *alerts.ClusterMetrics
}

// NewApp creates the application from assembled dependencies.
func NewApp(ac AppConfig) *App {
	return &App{
		logger:    ac.Logger,
		cfg:       ac.Config,
		k8s:       ac.K8sClient,
		gate:      ac.Gate,
		assembler: ac.Assembler,
		detector:  ac.Detector,
		agent:     ac.Agent,
		popeye:    ac.Popeye,
		term:      terminal.New(ac.Logger),
	}
}

// Startup is called by Wails when the app starts. Stores the runtime context
// and kicks off background event polling.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.logger.InfoContext(ctx, "kubewatcher started",
		slog.String("tier", string(a.cfg.Features.Tier)),
	)
	a.StartEventLoop(ctx)
}

// Shutdown is called by Wails when the app closes.
func (a *App) Shutdown(ctx context.Context) {
	a.term.Close()
	a.logger.InfoContext(ctx, "kubewatcher shutting down")
}

var errNoCluster = fmt.Errorf("no cluster connected — check kubeconfig")

// GetClusterInfo returns cluster metadata.
func (a *App) GetClusterInfo() (*k8s.ClusterInfo, error) {
	if a.k8s == nil {
		return &k8s.ClusterInfo{Name: "not connected", NodeCount: 0, K8sVersion: "—"}, nil
	}
	return a.k8s.GetClusterInfo(a.ctx)
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

// GetAlerts returns enriched alerts from the cluster.
func (a *App) GetAlerts() ([]alerts.Alert, error) {
	if a.k8s == nil {
		return nil, nil
	}
	return a.k8s.DetectAlerts(a.ctx)
}

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

// GetPodLogs returns recent log lines for a pod.
func (a *App) GetPodLogs(namespace, podName string, tailLines int64) ([]alerts.LogLine, error) {
	if a.k8s == nil {
		return nil, nil
	}
	return a.k8s.GetPodLogs(a.ctx, namespace, podName, tailLines)
}

// GetFeatures returns all features and their availability for the current tier.
func (a *App) GetFeatures() map[features.Feature]bool {
	return a.gate.AllFeatures()
}

// GetTier returns the current subscription tier.
func (a *App) GetTier() config.Tier {
	return a.gate.Tier()
}

// GetAnomalyJobs returns the configured Anomstack jobs.
func (a *App) GetAnomalyJobs() ([]anomaly.Job, error) {
	if !a.gate.Allowed(features.FeatureAnomstack) {
		return nil, features.ErrProRequired
	}
	if a.detector == nil {
		return nil, nil
	}
	return a.detector.ListJobs(a.ctx)
}

// --- AI Agent bindings ---

// SendChatMessage sends a user message to the AI agent for a given alert context.
func (a *App) SendChatMessage(alertID string, message string) (string, error) {
	if a.agent == nil {
		return "", fmt.Errorf("AI agent not configured — set DEEPSEEK_API_KEY")
	}
	if !a.gate.Allowed(features.FeatureAIDiagnostics) {
		return "", features.ErrProRequired
	}

	// Find the alert for context.
	var alert *alerts.Alert
	if a.k8s != nil && alertID != "global" {
		allAlerts, err := a.k8s.DetectAlerts(a.ctx)
		if err == nil {
			for i := range allAlerts {
				if allAlerts[i].ID == alertID {
					alert = &allAlerts[i]
					break
				}
			}
		}
	}

	return a.agent.SendMessage(a.ctx, alertID, message, alert, a.cachedMetrics)
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

// --- Popeye bindings ---

// RunPopeye executes a Popeye cluster scan and returns structured findings.
func (a *App) RunPopeye() (*popeye.Report, error) {
	if a.popeye == nil {
		return nil, fmt.Errorf("popeye not configured — install popeye CLI")
	}
	return a.popeye.Run(a.ctx)
}

// --- Resource browser bindings ---

// ListResources lists resources of the given kind in a namespace.
func (a *App) ListResources(kind, namespace string) (*k8s.ResourceListResult, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.ListResources(a.ctx, kind, namespace)
}

// GetResourceDetail returns full details for a specific resource.
func (a *App) GetResourceDetail(kind, namespace, name string) (*k8s.ResourceDetailResult, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.GetResourceDetail(a.ctx, kind, namespace, name)
}

// ListAllNamespaces returns all namespace names for the namespace picker.
func (a *App) ListAllNamespaces() ([]string, error) {
	if a.k8s == nil {
		return nil, nil
	}
	return a.k8s.ListAllNamespaces(a.ctx)
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

// --- SaaS & Agent bindings ---

// LoginSaaS authenticates the local client with the central KubeWatcher SaaS.
func (a *App) LoginSaaS(provider string) (string, error) {
	a.logger.InfoContext(a.ctx, "Initiating SaaS login", slog.String("provider", provider))
	// In a real implementation, this would open a browser, perform OAuth PKCE,
	// and capture the callback token on a localhost port.
	return "mock-jwt-token-from-" + provider, nil
}

// AgentAnomaly represents the payload returned by the in-cluster agent.
type AgentAnomaly struct {
	Timestamp string  `json:"timestamp"`
	Score     float64 `json:"score"`
	Target    string  `json:"target"`
	Rule      string  `json:"rule"`
}

// ConnectToAgent performs a port-forward to the in-cluster agent and fetches ML metrics.
func (a *App) ConnectToAgent(namespace string) ([]AgentAnomaly, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	a.logger.InfoContext(a.ctx, "Connecting to KubeWatcher agent", slog.String("namespace", namespace))

	// In a real implementation, this would establish a dynamic port-forward
	// using client-go to the kubewatcher-agent pod and make an HTTP GET request
	// to /api/v1/anomalies.

	// Mocking the agent response for the prototype:
	return []AgentAnomaly{
		{Timestamp: "2 Mins Ago", Score: 94.5, Target: "aws-prod-db", Rule: "Sudden Memory Spike"},
		{Timestamp: "1 Hour Ago", Score: 88.2, Target: "ingress/traefik", Rule: "High Error Rate"},
	}, nil
}
