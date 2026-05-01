package pkg

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/kube-watcher/internal/agentconn"
	"github.com/argues/kube-watcher/internal/ai"
	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/argues/kube-watcher/internal/anomaly"
	"github.com/argues/kube-watcher/internal/config"
	ctxassembly "github.com/argues/kube-watcher/internal/context"
	"github.com/argues/kube-watcher/internal/features"
	"github.com/argues/kube-watcher/internal/incidents"
	"github.com/argues/kube-watcher/internal/k8s"
	"github.com/argues/kube-watcher/internal/notebooks"
	"github.com/argues/kube-watcher/internal/popeye"
	"github.com/argues/kube-watcher/internal/runbooks"
	"github.com/argues/kube-watcher/internal/setup"
	"github.com/argues/kube-watcher/internal/terminal"
	"github.com/argues/kube-watcher/internal/vulnscan"
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
	Scanner   *vulnscan.Scanner
	Notebooks *notebooks.Store
	Runbooks  *runbooks.Store
	Setup     *setup.Manager
	Incidents *incidents.Store
	AppMode   string
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
	scanner   *vulnscan.Scanner
	notebooks *notebooks.Store
	runbooks  *runbooks.Store
	agentConn *agentconn.Connector
	term      *terminal.Terminal
	setup     *setup.Manager
	incidents *incidents.Store
	hub       *Hub

	appMode string

	// cachedMetrics holds the latest metrics for agent context.
	cachedMetrics *alerts.ClusterMetrics
}

// GetAppMode returns the frontend display mode (e.g., 'dashboard' or 'terminal').
func (a *App) GetAppMode() string {
	if a.appMode == "" {
		return "dashboard"
	}
	return a.appMode
}

func NewApp(ac AppConfig) *App {
	app := &App{
		ctx:       context.Background(),
		logger:    ac.Logger,
		cfg:       ac.Config,
		k8s:       ac.K8sClient,
		gate:      features.NewGate(ac.Config),
		assembler: ac.Assembler,
		detector:  ac.Detector,
		agent:     ac.Agent,
		popeye:    ac.Popeye,
		scanner:   ac.Scanner,
		notebooks: ac.Notebooks,
		runbooks:  ac.Runbooks,
		term:      terminal.New(ac.Logger),
		setup:     ac.Setup,
		incidents: ac.Incidents,
		appMode:   ac.AppMode,
		hub:       NewHub(ac.Logger.With("component", "saas-hub")),
	}

	// Initialize agent connector if k8s client is available.
	if ac.K8sClient != nil {
		app.agentConn = agentconn.New(
			ac.K8sClient.GetClientset(),
			ac.K8sClient.GetRestConfig(),
			ac.Logger.With("component", "agentconn"),
		)
	}

	return app
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

// DeletePod deletes a pod by namespace and name.
func (a *App) DeletePod(namespace, podName string) error {
	if a.k8s == nil {
		return errNoCluster
	}
	a.logger.Info("deleting pod", slog.String("namespace", namespace), slog.String("pod", podName))
	return a.k8s.DeletePod(a.ctx, namespace, podName)
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

// --- Metrics bindings ---

// QueryTimeSeriesMetrics returns time-series data points for a given query.
// Queries real metrics-server if available, falls back to core API derivation.
func (a *App) QueryTimeSeriesMetrics(query string, timeRange string) ([]float64, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.QueryTimeSeriesMetrics(a.ctx, query, timeRange)
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

// --- Notebooks bindings ---

// ListNotebooks returns the tree structure of all notebooks.
func (a *App) ListNotebooks() ([]notebooks.FileEntry, error) {
	if a.notebooks == nil {
		return []notebooks.FileEntry{}, nil
	}
	return a.notebooks.ListFiles(a.ctx)
}

// GetNotebook retrieves the content of a specific notebook file.
func (a *App) GetNotebook(path string) (string, error) {
	if a.notebooks == nil {
		return "", fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.GetFile(a.ctx, path)
}

// SaveNotebook saves notebook content and syncs to S3.
func (a *App) SaveNotebook(path, content string) error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.SaveFile(a.ctx, path, content)
}

// DeleteNotebook removes a notebook file.
func (a *App) DeleteNotebook(path string) error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.DeleteFile(a.ctx, path)
}

// CreateNotebookFolder creates a new folder in the notebooks hierarchy.
func (a *App) CreateNotebookFolder(path string) error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.CreateFolder(a.ctx, path)
}

// MoveNotebook moves a notebook from one path to another (copy + delete).
func (a *App) MoveNotebook(oldPath, newPath string) error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	content, err := a.notebooks.GetFile(a.ctx, oldPath)
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}
	if err := a.notebooks.SaveFile(a.ctx, newPath, content); err != nil {
		return fmt.Errorf("failed to write destination: %w", err)
	}
	return a.notebooks.DeleteFile(a.ctx, oldPath)
}

// TestS3Connection verifies S3 credentials and connectivity.
func (a *App) TestS3Connection() error {
	if a.notebooks == nil {
		return fmt.Errorf("notebooks not configured")
	}
	return a.notebooks.TestConnection(a.ctx)
}

// --- Runbooks bindings ---

// ListRunbooks returns all runbooks.
func (a *App) ListRunbooks() ([]runbooks.Runbook, error) {
	if a.runbooks == nil {
		return []runbooks.Runbook{}, nil
	}
	return a.runbooks.List(a.ctx)
}

// GetRunbook retrieves a runbook's full markdown content.
func (a *App) GetRunbook(id string) (string, error) {
	if a.runbooks == nil {
		return "", fmt.Errorf("runbooks not configured")
	}
	return a.runbooks.Get(a.ctx, id)
}

// SaveRunbook saves a runbook's content.
func (a *App) SaveRunbook(id, content string) error {
	if a.runbooks == nil {
		return fmt.Errorf("runbooks not configured")
	}
	return a.runbooks.Save(a.ctx, id, content)
}

// DeleteRunbook removes a runbook.
func (a *App) DeleteRunbook(id string) error {
	if a.runbooks == nil {
		return fmt.Errorf("runbooks not configured")
	}
	return a.runbooks.Delete(a.ctx, id)
}

// CreateRunbook creates a new runbook with the given name and trigger.
func (a *App) CreateRunbook(name, trigger string) (runbooks.Runbook, error) {
	if a.runbooks == nil {
		return runbooks.Runbook{}, fmt.Errorf("runbooks not configured")
	}
	return a.runbooks.Create(a.ctx, name, trigger)
}

// --- SaaS & Agent bindings ---

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

// --- Setup bindings ---

// CheckToolStatus returns the install status of all required external tools.
func (a *App) CheckToolStatus() []setup.ToolStatus {
	if a.setup == nil {
		return []setup.ToolStatus{
			{Name: "popeye", Installed: false, Message: "Setup manager not initialized"},
			{Name: "kubewatcher-agent", Installed: false, Message: "Setup manager not initialized"},
		}
	}
	return a.setup.CheckAllTools(a.ctx)
}

// InstallPopeye installs Popeye via go install or Docker pull.
func (a *App) InstallPopeye() (*setup.SetupResult, error) {
	if a.setup == nil {
		return &setup.SetupResult{Success: false, Message: "Setup manager not initialized"}, nil
	}
	return a.setup.InstallPopeye(a.ctx), nil
}

// DeployAgent deploys the KubeWatcher agent to the connected cluster.
func (a *App) DeployAgent(namespace string) (*setup.SetupResult, error) {
	if a.setup == nil {
		return &setup.SetupResult{Success: false, Message: "Setup manager not initialized"}, nil
	}
	return a.setup.DeployAgent(a.ctx, namespace), nil
}

// UndeployAgent removes the KubeWatcher agent from the cluster.
func (a *App) UndeployAgent(namespace string) (*setup.SetupResult, error) {
	if a.setup == nil {
		return &setup.SetupResult{Success: false, Message: "Setup manager not initialized"}, nil
	}
	return a.setup.UndeployAgent(a.ctx, namespace), nil
}

// --- Log query bindings ---

// QueryLogs searches pod logs across the cluster with text filter.
func (a *App) QueryLogs(query, namespace string, limit int) (*k8s.LogQueryResult, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.QueryLogs(a.ctx, query, namespace, limit)
}

// --- Incident CRUD bindings ---

// ListIncidents returns all incidents, newest first.
func (a *App) ListIncidents() []incidents.Incident {
	if a.incidents == nil {
		return nil
	}
	return a.incidents.List(a.ctx)
}

// CreateIncident creates a new incident.
func (a *App) CreateIncident(title, severity, incType, description, namespace string) (incidents.Incident, error) {
	if a.incidents == nil {
		return incidents.Incident{}, fmt.Errorf("incident store not initialized")
	}
	return a.incidents.Create(a.ctx, title, severity, incType, description, namespace)
}

// UpdateIncident updates an existing incident's status or description.
func (a *App) UpdateIncident(id, status, description string) (*incidents.Incident, error) {
	if a.incidents == nil {
		return nil, fmt.Errorf("incident store not initialized")
	}
	return a.incidents.Update(a.ctx, id, status, description)
}

// DeleteIncident removes an incident.
func (a *App) DeleteIncident(id string) error {
	if a.incidents == nil {
		return fmt.Errorf("incident store not initialized")
	}
	return a.incidents.Delete(a.ctx, id)
}

// --- Topology bindings ---

// GetTopology builds a topology graph from the live cluster state.
func (a *App) GetTopology(namespace string) (*k8s.TopologyResult, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.BuildTopology(a.ctx, namespace)
}

// --- ArgoCD bindings ---

// ListApplications returns deployment-based "applications" with rollout status.
func (a *App) ListApplications(namespace string) ([]k8s.Application, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.ListApplications(a.ctx, namespace)
}

// SyncApplication triggers a rollout restart on a deployment.
func (a *App) SyncApplication(namespace, name string) error {
	if a.k8s == nil {
		return errNoCluster
	}
	return a.k8s.RestartDeployment(a.ctx, namespace, name)
}

// --- Vulnerability bindings ---

// ListVulnerabilities returns cached scan results (or demo data if no scan has run).
func (a *App) ListVulnerabilities() ([]vulnscan.ScannedImage, error) {
	if a.scanner == nil {
		return vulnscan.DemoResults(), nil
	}
	return a.scanner.List(), nil
}

// ScanImage triggers a Trivy vulnerability scan for a single container image.
func (a *App) ScanImage(image string, engine string) (string, error) {
	if a.scanner == nil {
		return "Scanner not initialized — no cluster connection", nil
	}
	return a.scanner.ScanSingleImage(a.ctx, image, engine)
}

// ScanAllImages enumerates all images in the cluster and scans each via Trivy.
func (a *App) ScanAllImages(namespace string) ([]vulnscan.ScannedImage, error) {
	if a.scanner == nil {
		return vulnscan.DemoResults(), nil
	}
	if namespace == "" {
		namespace = ""
	}
	return a.scanner.ScanAll(a.ctx, namespace)
}

// --- Code Block bindings ---

// RunCodeSandbox executes code in a sandbox environment and returns the output.
func (a *App) RunCodeSandbox(code string, language string) (string, error) {
	// Mock implementation
	a.logger.InfoContext(a.ctx, "Running code sandbox", slog.String("language", language))
	return "> Execution started...\n> Loading dependencies...\n> Compilation successful.\n\nOutput:\nHello, KubeWatcher Sandbox Environment!\n\n> Exit code 0", nil
}

// GetCodeSuggestion returns an AI suggestion for the given code.
func (a *App) GetCodeSuggestion(code string, language string) (string, error) {
	// Mock implementation
	a.logger.InfoContext(a.ctx, "Getting code suggestion", slog.String("language", language))
	return "Consider adding error handling to this block. Extracting the hardcoded credentials into Kubernetes Secrets and loading them via environment variables would significantly improve security.", nil
}
