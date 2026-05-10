package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/kube-watcher/internal/agentconn"
	"github.com/argues/kube-watcher/internal/ai"
	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/argues/kube-watcher/internal/argocd"
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

// GetAlerts returns enriched alerts from the cluster merged with webhook-received alerts.
func (a *App) GetAlerts() ([]alerts.Alert, error) {
	var result []alerts.Alert

	// Get live k8s alerts.
	if a.k8s != nil {
		k8sAlerts, err := a.k8s.DetectAlerts(a.ctx)
		if err != nil {
			a.logger.WarnContext(a.ctx, "k8s alert detection failed", "error", err)
		} else {
			result = append(result, k8sAlerts...)
		}
	}

	// Merge in webhook-received alerts.
	a.webhookMu.RLock()
	result = append(result, a.webhookAlerts...)
	a.webhookMu.RUnlock()

	return result, nil
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

// GetServicePods resolves a service's label selector to its backing pods.
func (a *App) GetServicePods(namespace, serviceName string) ([]k8s.ServicePod, error) {
	if a.k8s == nil {
		return nil, errNoCluster
	}
	return a.k8s.GetServicePods(a.ctx, namespace, serviceName)
}

// GetFeatures returns all features and their availability for the current tier.
func (a *App) GetFeatures() map[features.Feature]bool {
	return a.gate.AllFeatures()
}

// GetTier returns the current subscription tier.
func (a *App) GetTier() config.Tier {
	return a.gate.Tier()
}

// LaunchPopOutTerminal opens the standalone terminal as a SEPARATE macOS
// application — its own Dock icon, its own Cmd+Tab entry, its own Mission
// Control window. This is what the user wants from "Pop out": not just a
// new window, but a different app altogether.
//
// Strategy:
//   1. Production: `KubeWatcherTerminal.app` is bundled inside the main
//      .app at Contents/Resources/. Different CFBundleIdentifier, so macOS
//      treats it as a separate application. Launched via `open -n` which
//      forces a fresh instance and respects the bundle's app-ness.
//   2. Dev (`wails dev` / `make run`): no second .app bundle exists, so
//      fall back to spawning this same binary with KUBEWATCHER_MODE=terminal.
//      The user gets a normal multi-window experience but it's still the
//      same Dock entry. Acceptable for development.
func (a *App) LaunchPopOutTerminal() error {
	if path, ok := siblingTerminalAppPath(); ok {
		// Production path — open as a real macOS app via `open -n`.
		cmd := exec.Command("open", "-n", "-a", path)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("open terminal app at %s: %w", path, err)
		}
		a.logger.Info("launched pop-out terminal as separate app", slog.String("path", path))
		return nil
	}

	// Dev fallback — spawn the same binary in terminal mode.
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self: %w", err)
	}
	cmd := exec.Command(self)
	cmd.Env = append(os.Environ(), "KUBEWATCHER_MODE=terminal")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch terminal window: %w", err)
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	a.logger.Info("launched pop-out terminal (dev fallback, same binary)",
		slog.String("path", self),
	)
	return nil
}

// siblingTerminalAppPath looks for the bundled standalone terminal .app
// next to the running executable. In production the running binary lives at
//   <App>.app/Contents/MacOS/kubewatcher
// and the terminal app is at
//   <App>.app/Contents/Resources/KubeWatcherTerminal.app
// Returns ("", false) when not present (dev mode, non-mac builds, etc.).
func siblingTerminalAppPath() (string, bool) {
	self, err := os.Executable()
	if err != nil {
		return "", false
	}
	// Walk up from MacOS/kubewatcher to <App>.app, then into Resources.
	// We don't use filepath.Dir twice because it's clearer to express what
	// we're matching against and we want to bail on unexpected layouts.
	const suffix = "/Contents/MacOS/"
	idx := -1
	for i := 0; i+len(suffix) <= len(self); i++ {
		if self[i:i+len(suffix)] == suffix {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", false
	}
	appBundle := self[:idx+len("/Contents")] // <App>.app/Contents
	candidate := appBundle + "/Resources/KubeWatcherTerminal.app"
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate, true
	}
	return "", false
}

// --- Settings bindings ---

// SettingsPayload is the JSON structure for runtime config visible in the settings view.
type SettingsPayload struct {
	KubeconfigPath   string `json:"kubeconfigPath"`
	CurrentContext   string `json:"currentContext"`
	Namespace        string `json:"namespace"`
	DeepSeekAPIKey   string `json:"deepseekApiKey"` // masked for display
	AnomstackURL     string `json:"anomstackUrl"`
	MCPServersConfig string `json:"mcpServersConfig"`
	AgentInstructions string `json:"agentInstructions"`
	PrometheusURL    string `json:"prometheusUrl"`
	ArgoCDURL        string `json:"argocdUrl"`
	ArgoCDToken      string `json:"argocdToken"`    // masked for display
	ArgoCDInsecure   bool   `json:"argocdInsecure"`
	// Security scanning tools (all optional).
	SnykToken   string `json:"snykToken"`   // masked for display
	TrivyBinary string `json:"trivyBinary"` // path to trivy binary
	FalcoURL    string `json:"falcoUrl"`    // Falco gRPC/HTTP endpoint
	Tier        string `json:"tier"`
	LogLevel    string `json:"logLevel"`
}

// TriggerAgentAnalysis manually fires the periodic agent analysis.
func (a *App) TriggerAgentAnalysis() {
	if a.periodicAgent != nil {
		a.periodicAgent.TriggerAnalysis()
	}
}

// GetSettings returns the current runtime configuration for display in the settings view.
func (a *App) GetSettings() SettingsPayload {
	masked := ""
	if a.cfg.AI.DeepSeekAPIKey != "" {
		k := a.cfg.AI.DeepSeekAPIKey
		if len(k) > 8 {
			masked = k[:4] + "…" + k[len(k)-4:]
		} else {
			masked = "••••"
		}
	}
	maskedArgoToken := ""
	if a.cfg.ArgoCD.Token != "" {
		t := a.cfg.ArgoCD.Token
		if len(t) > 8 {
			maskedArgoToken = t[:4] + "…" + t[len(t)-4:]
		} else {
			maskedArgoToken = "••••"
		}
	}

	maskedSnyk := ""
	if a.cfg.Security.SnykToken != "" {
		s := a.cfg.Security.SnykToken
		if len(s) > 8 {
			maskedSnyk = s[:4] + "…" + s[len(s)-4:]
		} else {
			maskedSnyk = "••••"
		}
	}

	return SettingsPayload{
		KubeconfigPath:   a.cfg.Kubernetes.Config,
		CurrentContext:   a.cfg.Kubernetes.Context,
		Namespace:        a.cfg.Kubernetes.Namespace,
		DeepSeekAPIKey:    masked,
		AnomstackURL:      a.cfg.AI.AnomstackURL,
		MCPServersConfig:  a.cfg.AI.MCPServersConfig,
		AgentInstructions: a.cfg.AI.AgentInstructions,
		PrometheusURL:     a.cfg.AI.PrometheusURL,
		ArgoCDURL:      a.cfg.ArgoCD.URL,
		ArgoCDToken:    maskedArgoToken,
		ArgoCDInsecure: a.cfg.ArgoCD.Insecure,
		SnykToken:      maskedSnyk,
		TrivyBinary:    a.cfg.Security.TrivyBinary,
		FalcoURL:       a.cfg.Security.FalcoURL,
		Tier:           string(a.cfg.Features.Tier),
		LogLevel:       a.cfg.Logging.Level,
	}
}

// UpdateSettings applies runtime setting overrides. Only non-empty fields are applied.
// Kubeconfig and context changes trigger a k8s client reconnect.
func (a *App) UpdateSettings(s SettingsPayload) error {
	reconnect := false

	if s.KubeconfigPath != "" && s.KubeconfigPath != a.cfg.Kubernetes.Config {
		a.cfg.Kubernetes.Config = s.KubeconfigPath
		reconnect = true
	}
	if s.CurrentContext != "" && s.CurrentContext != a.cfg.Kubernetes.Context {
		a.cfg.Kubernetes.Context = s.CurrentContext
		reconnect = true
	}
	if s.Namespace != a.cfg.Kubernetes.Namespace {
		a.cfg.Kubernetes.Namespace = s.Namespace
		// Namespace change doesn't need full reconnect, just update config.
	}
	rebuildAgent := false
	if s.DeepSeekAPIKey != "" && !containsMask(s.DeepSeekAPIKey) {
		if s.DeepSeekAPIKey != a.cfg.AI.DeepSeekAPIKey {
			a.cfg.AI.DeepSeekAPIKey = s.DeepSeekAPIKey
			rebuildAgent = true
		}
	}
	if s.AnomstackURL != "" {
		a.cfg.AI.AnomstackURL = s.AnomstackURL
	}
	if s.MCPServersConfig != "" {
		a.cfg.AI.MCPServersConfig = s.MCPServersConfig
	}
	if s.AgentInstructions != "" {
		a.cfg.AI.AgentInstructions = s.AgentInstructions
	}
	if s.PrometheusURL != "" {
		a.cfg.AI.PrometheusURL = s.PrometheusURL
	}

	if rebuildAgent {
		dsClient := ai.NewDeepSeekClient(a.cfg.AI.DeepSeekAPIKey, a.logger)
		if a.agent != nil {
			a.agent.SetClient(dsClient)
			a.logger.Info("AI agent client updated with new DeepSeek API key")
		} else {
			a.agent = ai.NewAgent(dsClient, a.logger)
			a.logger.Info("AI agent initialized from runtime DeepSeek API key")
		}
	}

	// Argo CD settings — rebuild client if URL or token changes.
	rebuildArgoCD := false
	if s.ArgoCDURL != "" && s.ArgoCDURL != a.cfg.ArgoCD.URL {
		a.cfg.ArgoCD.URL = s.ArgoCDURL
		rebuildArgoCD = true
	}
	if s.ArgoCDToken != "" && !containsMask(s.ArgoCDToken) {
		a.cfg.ArgoCD.Token = s.ArgoCDToken
		rebuildArgoCD = true
	}
	if s.ArgoCDInsecure != a.cfg.ArgoCD.Insecure {
		a.cfg.ArgoCD.Insecure = s.ArgoCDInsecure
		rebuildArgoCD = true
	}
	if rebuildArgoCD {
		a.argoCD = argocd.New(argocd.Config{
			URL:      a.cfg.ArgoCD.URL,
			Token:    a.cfg.ArgoCD.Token,
			Insecure: a.cfg.ArgoCD.Insecure,
		}, a.logger)
		a.logger.Info("ArgoCD client rebuilt",
			slog.String("url", a.cfg.ArgoCD.URL),
		)
	}

	// Security scanning tools (all optional).
	if s.SnykToken != "" && !containsMask(s.SnykToken) {
		a.cfg.Security.SnykToken = s.SnykToken
	}
	if s.TrivyBinary != "" {
		a.cfg.Security.TrivyBinary = s.TrivyBinary
	}
	if s.FalcoURL != "" {
		a.cfg.Security.FalcoURL = s.FalcoURL
	}

	if reconnect {
		a.logger.Info("settings changed — reconnecting k8s client",
			slog.String("kubeconfig", a.cfg.Kubernetes.Config),
			slog.String("context", a.cfg.Kubernetes.Context),
		)
		client, err := k8s.NewClient(a.cfg, a.logger)
		if err != nil {
			return fmt.Errorf("reconnect failed: %w", err)
		}
		a.k8s = client

		// Rebuild agent connector with new client.
		a.agentConn = agentconn.New(
			a.k8s.GetClientset(),
			a.k8s.GetRestConfig(),
			a.logger.With("component", "agentconn"),
		)

		// Restart event loop with new client.
		go a.StartEventLoop(a.ctx)
	}

	// Persist user-customized settings so they survive across restarts.
	// Failure to persist is logged but not fatal — the in-memory update has
	// already taken effect for this session.
	if err := config.SavePersistedSettings(config.FromConfig(a.cfg)); err != nil {
		a.logger.Warn("failed to persist settings to disk", slog.String("error", err.Error()))
	} else {
		a.logger.Info("settings persisted to disk")
	}

	return nil
}

func containsMask(s string) bool {
	for _, r := range s {
		if r == '•' || r == '…' || r == '*' {
			return true
		}
	}
	return false
}

// --- Pod Exec (Shell) bindings ---

const EventExecOutput = "exec:output"
const EventExecExit = "exec:exit"

// ExecPodShell starts an interactive shell session in a pod container.
func (a *App) ExecPodShell(namespace, podName, container string, rows, cols int) error {
	if a.k8s == nil {
		return errNoCluster
	}

	// Close any existing exec session.
	a.closeExecSession()

	sess, err := a.k8s.ExecPodShell(a.ctx, namespace, podName, container, uint16(rows), uint16(cols))
	if err != nil {
		return fmt.Errorf("exec shell: %w", err)
	}

	sess.OnOutput = func(data string) {
		runtime.EventsEmit(a.ctx, EventExecOutput, data)
	}

	a.execMu.Lock()
	a.execSession = sess
	a.execMu.Unlock()

	// Watch for session end.
	go func() {
		<-sess.Done()
		a.execMu.Lock()
		a.execSession = nil
		a.execMu.Unlock()
		runtime.EventsEmit(a.ctx, EventExecExit, nil)
	}()

	a.logger.Info("exec shell started",
		slog.String("namespace", namespace),
		slog.String("pod", podName),
		slog.String("container", container),
	)
	return nil
}

// SendExecInput writes raw input to the active exec session.
func (a *App) SendExecInput(data string) error {
	a.execMu.RLock()
	sess := a.execSession
	a.execMu.RUnlock()

	if sess == nil {
		return nil
	}
	return sess.Write(data)
}

// ResizeExec updates the active exec session's terminal dimensions.
func (a *App) ResizeExec(rows, cols int) error {
	a.execMu.RLock()
	sess := a.execSession
	a.execMu.RUnlock()

	if sess == nil {
		return nil
	}
	sess.Resize(uint16(rows), uint16(cols))
	return nil
}

// CloseExecSession terminates the active exec session.
func (a *App) CloseExecSession() {
	a.closeExecSession()
}

func (a *App) closeExecSession() {
	a.execMu.Lock()
	sess := a.execSession
	a.execSession = nil
	a.execMu.Unlock()

	if sess != nil {
		sess.Close()
	}
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
