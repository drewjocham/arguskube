package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/argus/internal/agentconn"
	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/alerts"
	"github.com/argues/argus/internal/argocd"
	"github.com/argues/argus/internal/config"
	"github.com/argues/argus/internal/features"
	"github.com/argues/argus/internal/k8s"
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

// HandleURL handles deep links from custom URL schemes like argus://
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
//  1. Production: `ArgusTerminal.app` is bundled inside the main
//     .app at Contents/Resources/. Different CFBundleIdentifier, so macOS
//     treats it as a separate application. Launched via `open -n` which
//     forces a fresh instance and respects the bundle's app-ness.
//  2. Dev (`wails dev` / `make run`): no second .app bundle exists, so
//     fall back to spawning this same binary with argus_MODE=terminal.
//     The user gets a normal multi-window experience but it's still the
//     same Dock entry. Acceptable for development.
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
	cmd.Env = append(os.Environ(), "argus_MODE=terminal")
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
//
//	<App>.app/Contents/MacOS/argus
//
// and the terminal app is at
//
//	<App>.app/Contents/Resources/ArgusTerminal.app
//
// Returns ("", false) when not present (dev mode, non-mac builds, etc.).
func siblingTerminalAppPath() (string, bool) {
	self, err := os.Executable()
	if err != nil {
		return "", false
	}
	// Walk up from MacOS/argus to <App>.app, then into Resources.
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
	candidate := appBundle + "/Resources/ArgusTerminal.app"
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate, true
	}
	return "", false
}

// --- Settings bindings ---

// SettingsPayload is the JSON structure for runtime config visible in the settings view.
type SettingsPayload struct {
	KubeconfigPath    string `json:"kubeconfigPath"`
	CurrentContext    string `json:"currentContext"`
	Namespace         string `json:"namespace"`
	DeepSeekAPIKey    string `json:"deepseekApiKey"` // masked for display
	LLMBaseURL        string `json:"llmBaseUrl"`     // override for self-hosted vLLM (vast.ai/GCP)
	LLMModel          string `json:"llmModel"`       // model id served by the endpoint
	MCPServersConfig  string `json:"mcpServersConfig"`
	AgentInstructions string `json:"agentInstructions"`
	AnomstackURL      string `json:"anomstackUrl"`
	PrometheusURL     string `json:"prometheusUrl"`
	ArgoCDURL         string `json:"argocdUrl"`
	ArgoCDToken       string `json:"argocdToken"` // masked for display
	ArgoCDInsecure    bool   `json:"argocdInsecure"`
	// Security scanning tools (all optional).
	SnykToken   string `json:"snykToken"`   // masked for display
	TrivyBinary string `json:"trivyBinary"` // path to trivy binary
	FalcoURL    string `json:"falcoUrl"`    // Falco gRPC/HTTP endpoint
	// Pipelines / CI-CD (all optional).
	PipelinesEnabled    bool   `json:"pipelinesEnabled"`
	PipelinesProvider   string `json:"pipelinesProvider"` // "" | github | gitlab | aws-codebuild | gcp-cloudbuild | circleci
	GitHubToken         string `json:"githubToken"`       // masked for display
	GitHubOwner         string `json:"githubOwner"`
	GitHubRepo          string `json:"githubRepo"`
	GitHubWorkflow      string `json:"githubWorkflow"`
	GitLabURL           string `json:"gitlabUrl"`
	GitLabToken         string `json:"gitlabToken"` // masked for display
	GitLabProjectID     string `json:"gitlabProjectId"`
	GitLabRef           string `json:"gitlabRef"`
	AWSRegion           string `json:"awsRegion"`
	AWSAccessKey        string `json:"awsAccessKey"`
	AWSSecretKey        string `json:"awsSecretKey"` // masked for display
	AWSProject          string `json:"awsProject"`
	GCPProject          string `json:"gcpProject"`
	GCPRegion           string `json:"gcpRegion"`
	GCPCredentials      string `json:"gcpCredentials"`
	CircleCIToken       string `json:"circleciToken"` // masked for display
	CircleCIProjectSlug string `json:"circleciProjectSlug"`
	AzureOrganization   string `json:"azureOrganization"`
	AzureProject        string `json:"azureProject"`
	AzurePipelineID     string `json:"azurePipelineId"`
	AzureToken          string `json:"azureToken"` // masked for display
	AzureBranch         string `json:"azureBranch"`
	// PR notification toggles.
	NotifyOnPROpened    bool `json:"notifyOnPrOpened"`
	NotifyOnPRUpdated   bool `json:"notifyOnPrUpdated"`
	NotifyOnPRCommented bool `json:"notifyOnPrCommented"`
	NotifyOnPRMerged    bool `json:"notifyOnPrMerged"`
	// Auto code review.
	AutoCodeReview        bool   `json:"autoCodeReview"`
	CodeReviewDestination string `json:"codeReviewDestination"`
	GDriveFolderID        string `json:"gdriveFolderId"`
	CodeReviewS3Prefix    string `json:"codeReviewS3Prefix"`
	CodeReviewEmailTo     string `json:"codeReviewEmailTo"`
	// Documentation destinations (all optional, tokens masked on read).
	ConfluenceURL          string `json:"confluenceUrl"`
	ConfluenceEmail        string `json:"confluenceEmail"`
	ConfluenceToken        string `json:"confluenceToken"`
	ConfluenceSpaceKey     string `json:"confluenceSpaceKey"`
	ConfluenceParentPageID string `json:"confluenceParentPageId"`
	NotionToken            string `json:"notionToken"`
	NotionDatabaseID       string `json:"notionDatabaseId"`
	EvernoteToken          string `json:"evernoteToken"`
	EvernoteNotebookGUID   string `json:"evernoteNotebookGuid"`
	OneNoteToken           string `json:"onenoteToken"`
	OneNoteSectionID       string `json:"onenoteSectionId"`
	AmplenoteAPIKey        string `json:"amplenoteApiKey"`
	StandardNotesURL       string `json:"standardNotesUrl"`
	StandardNotesToken     string `json:"standardNotesToken"`
	ObsidianVaultPath      string `json:"obsidianVaultPath"`
	JoplinURL              string `json:"joplinUrl"`
	JoplinToken            string `json:"joplinToken"`
	LogseqGraphPath        string `json:"logseqGraphPath"`
	BearToken              string `json:"bearToken"`
	Tier                   string `json:"tier"`
	LogLevel               string `json:"logLevel"`
}

// maskSecret returns a display-safe rendering of a secret string. Empty input
// returns empty (so the UI can show "Not configured"); short secrets become
// "••••"; longer secrets show the first and last four characters.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) > 8 {
		return s[:4] + "…" + s[len(s)-4:]
	}
	return "••••"
}

// GetSettings returns the current runtime configuration for display in the settings view.
func (a *App) GetSettings() SettingsPayload {
	p := a.cfg.Pipelines
	return SettingsPayload{
		KubeconfigPath:    a.cfg.Kubernetes.Config,
		CurrentContext:    a.cfg.Kubernetes.Context,
		Namespace:         a.cfg.Kubernetes.Namespace,
		DeepSeekAPIKey:    maskSecret(a.cfg.AI.DeepSeekAPIKey),
		LLMBaseURL:        a.cfg.AI.LLMBaseURL,
		LLMModel:          a.cfg.AI.LLMModel,
		MCPServersConfig:  a.cfg.AI.MCPServersConfig,
		AgentInstructions: a.cfg.AI.AgentInstructions,
		AnomstackURL:      a.cfg.AI.AnomstackURL,
		PrometheusURL:     a.cfg.AI.PrometheusURL,
		ArgoCDURL:         a.cfg.ArgoCD.URL,
		ArgoCDToken:       maskSecret(a.cfg.ArgoCD.Token),
		ArgoCDInsecure:    a.cfg.ArgoCD.Insecure,
		SnykToken:         maskSecret(a.cfg.Security.SnykToken),
		TrivyBinary:       a.cfg.Security.TrivyBinary,
		FalcoURL:          a.cfg.Security.FalcoURL,

		PipelinesEnabled:    p.Enabled,
		PipelinesProvider:   p.Provider,
		GitHubToken:         maskSecret(p.GitHubToken),
		GitHubOwner:         p.GitHubOwner,
		GitHubRepo:          p.GitHubRepo,
		GitHubWorkflow:      p.GitHubWorkflow,
		GitLabURL:           p.GitLabURL,
		GitLabToken:         maskSecret(p.GitLabToken),
		GitLabProjectID:     p.GitLabProjectID,
		GitLabRef:           p.GitLabRef,
		AWSRegion:           p.AWSRegion,
		AWSAccessKey:        p.AWSAccessKey, // identifier, not a secret — shown as-is
		AWSSecretKey:        maskSecret(p.AWSSecretKey),
		AWSProject:          p.AWSProject,
		GCPProject:          p.GCPProject,
		GCPRegion:           p.GCPRegion,
		GCPCredentials:      p.GCPCredentials,
		CircleCIToken:       maskSecret(p.CircleCIToken),
		CircleCIProjectSlug: p.CircleCIProjectSlug,
		AzureOrganization:   p.AzureOrganization,
		AzureProject:        p.AzureProject,
		AzurePipelineID:     p.AzurePipelineID,
		AzureToken:          maskSecret(p.AzureToken),
		AzureBranch:         p.AzureBranch,

		NotifyOnPROpened:    p.NotifyOnPROpened,
		NotifyOnPRUpdated:   p.NotifyOnPRUpdated,
		NotifyOnPRCommented: p.NotifyOnPRCommented,
		NotifyOnPRMerged:    p.NotifyOnPRMerged,

		AutoCodeReview:        p.AutoCodeReview,
		CodeReviewDestination: p.CodeReviewDestination,
		GDriveFolderID:        p.GDriveFolderID,
		CodeReviewS3Prefix:    p.CodeReviewS3Prefix,
		CodeReviewEmailTo:     p.CodeReviewEmailTo,

		ConfluenceURL:          p.ConfluenceURL,
		ConfluenceEmail:        p.ConfluenceEmail,
		ConfluenceToken:        maskSecret(p.ConfluenceToken),
		ConfluenceSpaceKey:     p.ConfluenceSpaceKey,
		ConfluenceParentPageID: p.ConfluenceParentPageID,
		NotionToken:            maskSecret(p.NotionToken),
		NotionDatabaseID:       p.NotionDatabaseID,
		EvernoteToken:          maskSecret(p.EvernoteToken),
		EvernoteNotebookGUID:   p.EvernoteNotebookGUID,
		OneNoteToken:           maskSecret(p.OneNoteToken),
		OneNoteSectionID:       p.OneNoteSectionID,
		AmplenoteAPIKey:        maskSecret(p.AmplenoteAPIKey),
		StandardNotesURL:       p.StandardNotesURL,
		StandardNotesToken:     maskSecret(p.StandardNotesToken),
		ObsidianVaultPath:      p.ObsidianVaultPath,
		JoplinURL:              p.JoplinURL,
		JoplinToken:            maskSecret(p.JoplinToken),
		LogseqGraphPath:        p.LogseqGraphPath,
		BearToken:              maskSecret(p.BearToken),

		Tier:     string(a.cfg.Features.Tier),
		LogLevel: a.cfg.Logging.Level,
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
	if s.LLMBaseURL != "" {
		a.cfg.AI.LLMBaseURL = s.LLMBaseURL
	}
	if s.LLMModel != "" {
		a.cfg.AI.LLMModel = s.LLMModel
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

	// Pipelines / CI-CD (all optional). Booleans and the provider selector
	// are always applied; tokens skip the masked sentinel; other fields apply
	// when non-empty so an empty payload doesn't silently wipe stored config.
	a.cfg.Pipelines.Enabled = s.PipelinesEnabled
	a.cfg.Pipelines.Provider = s.PipelinesProvider

	if s.GitHubToken != "" && !containsMask(s.GitHubToken) {
		a.cfg.Pipelines.GitHubToken = s.GitHubToken
	}
	if s.GitHubOwner != "" {
		a.cfg.Pipelines.GitHubOwner = s.GitHubOwner
	}
	if s.GitHubRepo != "" {
		a.cfg.Pipelines.GitHubRepo = s.GitHubRepo
	}
	if s.GitHubWorkflow != "" {
		a.cfg.Pipelines.GitHubWorkflow = s.GitHubWorkflow
	}

	if s.GitLabURL != "" {
		a.cfg.Pipelines.GitLabURL = s.GitLabURL
	}
	if s.GitLabToken != "" && !containsMask(s.GitLabToken) {
		a.cfg.Pipelines.GitLabToken = s.GitLabToken
	}
	if s.GitLabProjectID != "" {
		a.cfg.Pipelines.GitLabProjectID = s.GitLabProjectID
	}
	if s.GitLabRef != "" {
		a.cfg.Pipelines.GitLabRef = s.GitLabRef
	}

	if s.AWSRegion != "" {
		a.cfg.Pipelines.AWSRegion = s.AWSRegion
	}
	if s.AWSAccessKey != "" {
		a.cfg.Pipelines.AWSAccessKey = s.AWSAccessKey
	}
	if s.AWSSecretKey != "" && !containsMask(s.AWSSecretKey) {
		a.cfg.Pipelines.AWSSecretKey = s.AWSSecretKey
	}
	if s.AWSProject != "" {
		a.cfg.Pipelines.AWSProject = s.AWSProject
	}

	if s.GCPProject != "" {
		a.cfg.Pipelines.GCPProject = s.GCPProject
	}
	if s.GCPRegion != "" {
		a.cfg.Pipelines.GCPRegion = s.GCPRegion
	}
	if s.GCPCredentials != "" {
		a.cfg.Pipelines.GCPCredentials = s.GCPCredentials
	}

	if s.CircleCIToken != "" && !containsMask(s.CircleCIToken) {
		a.cfg.Pipelines.CircleCIToken = s.CircleCIToken
	}
	if s.CircleCIProjectSlug != "" {
		a.cfg.Pipelines.CircleCIProjectSlug = s.CircleCIProjectSlug
	}

	if s.AzureOrganization != "" {
		a.cfg.Pipelines.AzureOrganization = s.AzureOrganization
	}
	if s.AzureProject != "" {
		a.cfg.Pipelines.AzureProject = s.AzureProject
	}
	if s.AzurePipelineID != "" {
		a.cfg.Pipelines.AzurePipelineID = s.AzurePipelineID
	}
	if s.AzureToken != "" && !containsMask(s.AzureToken) {
		a.cfg.Pipelines.AzureToken = s.AzureToken
	}
	if s.AzureBranch != "" {
		a.cfg.Pipelines.AzureBranch = s.AzureBranch
	}

	// PR notification toggles — booleans applied unconditionally so users can
	// turn each off again from a previous on state.
	a.cfg.Pipelines.NotifyOnPROpened = s.NotifyOnPROpened
	a.cfg.Pipelines.NotifyOnPRUpdated = s.NotifyOnPRUpdated
	a.cfg.Pipelines.NotifyOnPRCommented = s.NotifyOnPRCommented
	a.cfg.Pipelines.NotifyOnPRMerged = s.NotifyOnPRMerged

	// Auto code review — bool + string selector applied unconditionally; the
	// destination-specific config strings overwrite when non-empty so a
	// partial save doesn't wipe a previously-set folder/recipient list.
	a.cfg.Pipelines.AutoCodeReview = s.AutoCodeReview
	if s.CodeReviewDestination != "" {
		a.cfg.Pipelines.CodeReviewDestination = s.CodeReviewDestination
	}
	if s.GDriveFolderID != "" {
		a.cfg.Pipelines.GDriveFolderID = s.GDriveFolderID
	}
	if s.CodeReviewS3Prefix != "" {
		a.cfg.Pipelines.CodeReviewS3Prefix = s.CodeReviewS3Prefix
	}
	if s.CodeReviewEmailTo != "" {
		a.cfg.Pipelines.CodeReviewEmailTo = s.CodeReviewEmailTo
	}

	// Documentation destinations — strings apply when non-empty, tokens skip
	// the masked sentinel so reading-then-saving an unedited form preserves
	// the existing secret on disk.
	if s.ConfluenceURL != "" {
		a.cfg.Pipelines.ConfluenceURL = s.ConfluenceURL
	}
	if s.ConfluenceEmail != "" {
		a.cfg.Pipelines.ConfluenceEmail = s.ConfluenceEmail
	}
	if s.ConfluenceToken != "" && !containsMask(s.ConfluenceToken) {
		a.cfg.Pipelines.ConfluenceToken = s.ConfluenceToken
	}
	if s.ConfluenceSpaceKey != "" {
		a.cfg.Pipelines.ConfluenceSpaceKey = s.ConfluenceSpaceKey
	}
	if s.ConfluenceParentPageID != "" {
		a.cfg.Pipelines.ConfluenceParentPageID = s.ConfluenceParentPageID
	}
	if s.NotionToken != "" && !containsMask(s.NotionToken) {
		a.cfg.Pipelines.NotionToken = s.NotionToken
	}
	if s.NotionDatabaseID != "" {
		a.cfg.Pipelines.NotionDatabaseID = s.NotionDatabaseID
	}
	if s.EvernoteToken != "" && !containsMask(s.EvernoteToken) {
		a.cfg.Pipelines.EvernoteToken = s.EvernoteToken
	}
	if s.EvernoteNotebookGUID != "" {
		a.cfg.Pipelines.EvernoteNotebookGUID = s.EvernoteNotebookGUID
	}
	if s.OneNoteToken != "" && !containsMask(s.OneNoteToken) {
		a.cfg.Pipelines.OneNoteToken = s.OneNoteToken
	}
	if s.OneNoteSectionID != "" {
		a.cfg.Pipelines.OneNoteSectionID = s.OneNoteSectionID
	}
	if s.AmplenoteAPIKey != "" && !containsMask(s.AmplenoteAPIKey) {
		a.cfg.Pipelines.AmplenoteAPIKey = s.AmplenoteAPIKey
	}
	if s.StandardNotesURL != "" {
		a.cfg.Pipelines.StandardNotesURL = s.StandardNotesURL
	}
	if s.StandardNotesToken != "" && !containsMask(s.StandardNotesToken) {
		a.cfg.Pipelines.StandardNotesToken = s.StandardNotesToken
	}
	if s.ObsidianVaultPath != "" {
		a.cfg.Pipelines.ObsidianVaultPath = s.ObsidianVaultPath
	}
	if s.JoplinURL != "" {
		a.cfg.Pipelines.JoplinURL = s.JoplinURL
	}
	if s.JoplinToken != "" && !containsMask(s.JoplinToken) {
		a.cfg.Pipelines.JoplinToken = s.JoplinToken
	}
	if s.LogseqGraphPath != "" {
		a.cfg.Pipelines.LogseqGraphPath = s.LogseqGraphPath
	}
	if s.BearToken != "" && !containsMask(s.BearToken) {
		a.cfg.Pipelines.BearToken = s.BearToken
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
