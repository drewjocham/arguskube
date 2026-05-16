package pkg

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/argus/internal/agentconn"
	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/alerts"
	"github.com/argues/argus/internal/argocd"
	"github.com/argues/argus/internal/config"
	"github.com/argues/argus/internal/features"
	"github.com/argues/argus/internal/k8s"
	"github.com/argues/argus/internal/terminal"
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

// AutoResolveContext probes every kubeconfig context, picks the best one,
// and switches the live k8s client to it. The whole flow is one binding
// so the frontend can run it as a single non-interactive bootstrap step.
//
// Status events fire into the bottom ribbon via "argus:status":
//   - "Checking N kubeconfig contexts…" at the start
//   - "context X reachable in Yms" / "context X unreachable: <err>" per probe
//   - "Connected to <chosen> (<confidence>)" at the end
//
// The selection priority lives in k8s.ChooseContext and is unit-tested
// without a real cluster. We honour the active-override stored on the
// current cfg so a user choice from a previous session beats kubeconfig's
// current-context. Returns the resolution unconditionally — callers (the
// settings checklist, the sidebar) can inspect Probes to render per-context
// status even when the chosen one is unreachable.
func (a *App) AutoResolveContext() (k8s.ContextResolution, error) {
	kubeconfigPath := ""
	activeOverride := ""
	if a.cfg != nil {
		kubeconfigPath = a.cfg.Kubernetes.Config
		activeOverride = a.cfg.Kubernetes.Context
	}

	a.emitStatus("k8s", "info", "Scanning kubeconfig for contexts…", "")

	// 2s per-context timeout strikes the right balance: longer than typical
	// LAN/VPN handshakes, shorter than a corp firewall timing out a RST.
	probes, err := k8s.ProbeContexts(a.ctx, kubeconfigPath, activeOverride, 2*time.Second)
	if err != nil {
		a.emitStatus("k8s", "error", "Could not read kubeconfig", err.Error())
		return k8s.ContextResolution{}, err
	}
	if len(probes) == 0 {
		a.emitStatus("k8s", "warn", "No kubeconfig contexts found", "Add a context with kubectl or via the settings checklist.")
		return k8s.ContextResolution{Confidence: "none", Probes: probes}, k8s.ErrNoContexts
	}

	// Per-probe ribbon line so the user sees the scan running, not a
	// 4-second silence. We keep it terse so the marquee stays readable.
	for _, p := range probes {
		if p.Reachable {
			a.emitStatus("k8s", "info",
				fmt.Sprintf("%s reachable (%dms, %s)", p.Name, p.LatencyMs, p.ServerVersion), "")
		} else {
			a.emitStatus("k8s", "warn",
				fmt.Sprintf("%s unreachable", p.Name), p.Error)
		}
	}

	res := k8s.ChooseContext(probes)
	if res.Chosen == "" {
		a.emitStatus("k8s", "warn", "No reachable contexts", "Argus will retry on the next manual switch.")
		return res, nil
	}

	// Bind the live client to the chosen context. Re-use the same path
	// SwitchContext uses so the agent connector is rebuilt consistently.
	if err := a.SwitchContext(res.Chosen); err != nil {
		a.emitStatus("k8s", "error",
			fmt.Sprintf("Could not switch to %s", res.Chosen), err.Error())
		return res, err
	}

	switch res.Confidence {
	case "active-reachable":
		a.emitStatus("k8s", "info",
			fmt.Sprintf("Connected to %s", res.Chosen), "")
	case "fallback-reachable":
		a.emitStatus("k8s", "warn",
			fmt.Sprintf("Active context unreachable — using %s", res.Chosen),
			"You can switch back via the sidebar context picker.")
	case "active-unreachable":
		a.emitStatus("k8s", "warn",
			fmt.Sprintf("%s is selected but unreachable", res.Chosen),
			"Argus will keep retrying. Common cause: VPN off or corporate proxy.")
	}
	return res, nil
}

// emitStatus publishes a StatusEvent onto the "argus:status" channel that
// the frontend <StatusRibbon> subscribes to. Safe to call from any goroutine
// — Wails' EventsEmit is concurrency-safe. No-op when ctx is nil (Startup
// hasn't run yet).
func (a *App) emitStatus(source, severity, message, detail string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "argus:status", map[string]any{
		"source":   source,
		"severity": severity,
		"message":  message,
		"detail":   detail,
	})
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
	// Persist the choice so the next launch lands on the same context
	// without re-running auto-resolve. Failure is logged but not
	// surfaced — the switch already worked in-memory and the user can
	// still operate; persistence is just continuity.
	if err := config.SavePersistedSettings(config.FromConfig(a.cfg)); err != nil {
		a.logger.Warn("could not persist context switch",
			slog.String("context", name),
			slog.String("error", err.Error()),
		)
	}
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
//     fall back to spawning this same binary with ARGUS_MODE=terminal.
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
	cmd.Env = append(os.Environ(), "ARGUS_MODE=terminal")
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

	// Sign-in providers (auth). Secrets are masked on read; on write
	// the masked sentinel skips re-applying so an unedited form
	// preserves the on-disk value.
	GoogleClientID     string `json:"googleClientId"`
	GoogleClientSecret string `json:"googleClientSecret"` // masked
	OIDCIssuer         string `json:"oidcIssuer"`
	OIDCClientID       string `json:"oidcClientId"`
	OIDCClientSecret   string `json:"oidcClientSecret"` // masked
	OIDCDisplayName    string `json:"oidcDisplayName"`
	AppleServicesID    string `json:"appleServicesId"`
	AppleTeamID        string `json:"appleTeamId"`
	AppleKeyID         string `json:"appleKeyId"`
	ApplePrivateKey    string `json:"applePrivateKey"` // masked
	AppleDisplayName   string `json:"appleDisplayName"`
	AllowLocalSignup   bool   `json:"allowLocalSignup"`
	PasskeyEnabled     bool   `json:"passkeyEnabled"`
	PasskeyRPID        string `json:"passkeyRpId"`
	PasskeyRPName      string `json:"passkeyRpName"`
	PasskeyRPOrigin    string `json:"passkeyRpOrigin"`

	// Workspace OAuth clients (separate from sign-in Google).
	WorkspaceGoogleClientID     string `json:"workspaceGoogleClientId"`
	WorkspaceGoogleClientSecret string `json:"workspaceGoogleClientSecret"` // masked
	SlackClientID               string `json:"slackClientId"`
	SlackClientSecret           string `json:"slackClientSecret"`  // masked
	SlackSigningSecret          string `json:"slackSigningSecret"` // masked
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

		GoogleClientID:     a.cfg.Auth.GoogleClientID,
		GoogleClientSecret: maskSecret(a.cfg.Auth.GoogleClientSecret),
		OIDCIssuer:         a.cfg.Auth.OIDCIssuer,
		OIDCClientID:       a.cfg.Auth.OIDCClientID,
		OIDCClientSecret:   maskSecret(a.cfg.Auth.OIDCClientSecret),
		OIDCDisplayName:    a.cfg.Auth.OIDCDisplayName,
		AppleServicesID:    a.cfg.Auth.AppleServicesID,
		AppleTeamID:        a.cfg.Auth.AppleTeamID,
		AppleKeyID:         a.cfg.Auth.AppleKeyID,
		ApplePrivateKey:    maskSecret(a.cfg.Auth.ApplePrivateKey),
		AppleDisplayName:   a.cfg.Auth.AppleDisplayName,
		AllowLocalSignup:   a.cfg.Auth.AllowLocalSignup,
		PasskeyEnabled:     a.cfg.Auth.PasskeyEnabled,
		PasskeyRPID:        a.cfg.Auth.PasskeyRPID,
		PasskeyRPName:      a.cfg.Auth.PasskeyRPName,
		PasskeyRPOrigin:    a.cfg.Auth.PasskeyRPOrigin,

		WorkspaceGoogleClientID:     a.cfg.Workspace.GoogleClientID,
		WorkspaceGoogleClientSecret: maskSecret(a.cfg.Workspace.GoogleClientSecret),
		SlackClientID:               a.cfg.Workspace.SlackClientID,
		SlackClientSecret:           maskSecret(a.cfg.Workspace.SlackClientSecret),
		SlackSigningSecret:          maskSecret(a.cfg.Workspace.SlackSigningSecret),
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

	// Sign-in providers + workspace OAuth clients. Strings apply when
	// non-empty; secrets additionally skip the masked sentinel so a
	// "view-then-save" cycle doesn't overwrite the on-disk value with
	// "••••". Booleans apply unconditionally so the user can flip them
	// off again.
	authChanged := false
	if s.GoogleClientID != "" && s.GoogleClientID != a.cfg.Auth.GoogleClientID {
		a.cfg.Auth.GoogleClientID = s.GoogleClientID
		authChanged = true
	}
	if s.GoogleClientSecret != "" && !containsMask(s.GoogleClientSecret) {
		a.cfg.Auth.GoogleClientSecret = s.GoogleClientSecret
		authChanged = true
	}
	if s.OIDCIssuer != "" && s.OIDCIssuer != a.cfg.Auth.OIDCIssuer {
		a.cfg.Auth.OIDCIssuer = s.OIDCIssuer
		authChanged = true
	}
	if s.OIDCClientID != "" && s.OIDCClientID != a.cfg.Auth.OIDCClientID {
		a.cfg.Auth.OIDCClientID = s.OIDCClientID
		authChanged = true
	}
	if s.OIDCClientSecret != "" && !containsMask(s.OIDCClientSecret) {
		a.cfg.Auth.OIDCClientSecret = s.OIDCClientSecret
		authChanged = true
	}
	if s.OIDCDisplayName != "" && s.OIDCDisplayName != a.cfg.Auth.OIDCDisplayName {
		a.cfg.Auth.OIDCDisplayName = s.OIDCDisplayName
		authChanged = true
	}
	if s.AppleServicesID != "" && s.AppleServicesID != a.cfg.Auth.AppleServicesID {
		a.cfg.Auth.AppleServicesID = s.AppleServicesID
		authChanged = true
	}
	if s.AppleTeamID != "" && s.AppleTeamID != a.cfg.Auth.AppleTeamID {
		a.cfg.Auth.AppleTeamID = s.AppleTeamID
		authChanged = true
	}
	if s.AppleKeyID != "" && s.AppleKeyID != a.cfg.Auth.AppleKeyID {
		a.cfg.Auth.AppleKeyID = s.AppleKeyID
		authChanged = true
	}
	if s.ApplePrivateKey != "" && !containsMask(s.ApplePrivateKey) {
		a.cfg.Auth.ApplePrivateKey = s.ApplePrivateKey
		authChanged = true
	}
	if s.AppleDisplayName != "" && s.AppleDisplayName != a.cfg.Auth.AppleDisplayName {
		a.cfg.Auth.AppleDisplayName = s.AppleDisplayName
		authChanged = true
	}
	if s.AllowLocalSignup != a.cfg.Auth.AllowLocalSignup {
		a.cfg.Auth.AllowLocalSignup = s.AllowLocalSignup
		authChanged = true
	}
	if s.PasskeyEnabled != a.cfg.Auth.PasskeyEnabled {
		a.cfg.Auth.PasskeyEnabled = s.PasskeyEnabled
		authChanged = true
	}
	if s.PasskeyRPID != "" && s.PasskeyRPID != a.cfg.Auth.PasskeyRPID {
		a.cfg.Auth.PasskeyRPID = s.PasskeyRPID
		authChanged = true
	}
	if s.PasskeyRPName != "" && s.PasskeyRPName != a.cfg.Auth.PasskeyRPName {
		a.cfg.Auth.PasskeyRPName = s.PasskeyRPName
		authChanged = true
	}
	if s.PasskeyRPOrigin != "" && s.PasskeyRPOrigin != a.cfg.Auth.PasskeyRPOrigin {
		a.cfg.Auth.PasskeyRPOrigin = s.PasskeyRPOrigin
		authChanged = true
	}
	if authChanged && a.auth != nil && a.auth.store != nil {
		a.SetupAuth(a.auth.store, a.cfg.Auth)
		a.logger.Info("auth providers reloaded from settings")
	}

	// Workspace OAuth clients.
	workspaceChanged := false
	if s.WorkspaceGoogleClientID != "" && s.WorkspaceGoogleClientID != a.cfg.Workspace.GoogleClientID {
		a.cfg.Workspace.GoogleClientID = s.WorkspaceGoogleClientID
		workspaceChanged = true
	}
	if s.WorkspaceGoogleClientSecret != "" && !containsMask(s.WorkspaceGoogleClientSecret) {
		a.cfg.Workspace.GoogleClientSecret = s.WorkspaceGoogleClientSecret
		workspaceChanged = true
	}
	if s.SlackClientID != "" && s.SlackClientID != a.cfg.Workspace.SlackClientID {
		a.cfg.Workspace.SlackClientID = s.SlackClientID
		workspaceChanged = true
	}
	if s.SlackClientSecret != "" && !containsMask(s.SlackClientSecret) {
		a.cfg.Workspace.SlackClientSecret = s.SlackClientSecret
		workspaceChanged = true
	}
	if s.SlackSigningSecret != "" && !containsMask(s.SlackSigningSecret) {
		a.cfg.Workspace.SlackSigningSecret = s.SlackSigningSecret
		workspaceChanged = true
	}
	if workspaceChanged && a.workspace != nil {
		a.workspace.ReregisterProviders(buildWorkspaceProviders(a.cfg))
		a.logger.Info("workspace providers reloaded from settings")
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

// StartTerminal opens a default PTY shell session (backward-compat).
func (a *App) StartTerminal(rows, cols int) error {
	return a.StartTerminalSession("default", "default", "Shell", rows, cols)
}

// StartTerminalSession opens a named PTY shell session with domain context.
func (a *App) StartTerminalSession(sessionId, domain, label string, rows, cols int) error {
	var d terminal.Domain
	switch domain {
	case "k8s":
		d = terminal.DomainK8s
	case "kafka":
		d = terminal.DomainKafka
	case "cloud":
		d = terminal.DomainCloud
	default:
		d = terminal.DomainDefault
	}

	extraEnv := buildDomainEnv(a, d)

	sess, err := a.sessions.NewSession(sessionId, d, label, uint16(rows), uint16(cols), extraEnv)
	if err != nil {
		return err
	}

	sess.Terminal.OnOutput = func(data string) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, EventTerminalOutput, map[string]string{
				"sessionId": sessionId,
				"data":      data,
			})
		}
	}

	terminal.WriteWelcome(sess.Terminal, d, extraEnv)

	return nil
}

// SendTerminalInput writes raw input to the default session (backward-compat).
func (a *App) SendTerminalInput(data string) error {
	return a.SendTerminalSessionInput("default", data)
}

// SendTerminalSessionInput writes raw input to a named terminal session.
func (a *App) SendTerminalSessionInput(sessionId, data string) error {
	sess := a.sessions.GetSession(sessionId)
	if sess == nil {
		return nil
	}
	return sess.Terminal.Write(data)
}

// ResizeTerminal updates dimensions for the default session (backward-compat).
func (a *App) ResizeTerminal(rows, cols int) error {
	return a.ResizeTerminalSession("default", rows, cols)
}

// ResizeTerminalSession updates terminal dimensions for a named session.
func (a *App) ResizeTerminalSession(sessionId string, rows, cols int) error {
	sess := a.sessions.GetSession(sessionId)
	if sess == nil {
		return nil
	}
	return sess.Terminal.Resize(uint16(rows), uint16(cols))
}

// CloseTerminalSession closes a named terminal session.
func (a *App) CloseTerminalSession(sessionId string) error {
	return a.sessions.CloseSession(sessionId)
}

// ListTerminalSessions returns all active terminal sessions.
func (a *App) ListTerminalSessions() []terminal.SessionInfo {
	return a.sessions.ListSessions()
}

// buildDomainEnv returns extra environment variables for a terminal domain.
func buildDomainEnv(a *App, d terminal.Domain) []string {
	var env []string
	switch d {
	case terminal.DomainK8s:
		if a.cfg != nil {
			if ctx := a.cfg.Kubernetes.Context; ctx != "" {
				env = append(env, "ARGUS_K8S_CONTEXT="+ctx)
			}
			if ns := a.cfg.Kubernetes.Namespace; ns != "" {
				env = append(env, "ARGUS_NAMESPACE="+ns)
			}
			if kc := a.cfg.Kubernetes.Config; kc != "" {
				env = append(env, "KUBECONFIG="+kc)
			}
		}
	case terminal.DomainCloud:
		if a.cfg != nil {
			if p := a.cfg.Pipelines.GCPProject; p != "" {
				env = append(env, "CLOUDSDK_CORE_PROJECT="+p)
			}
			if r := a.cfg.Pipelines.GCPRegion; r != "" {
				env = append(env, "CLOUDSDK_COMPUTE_REGION="+r)
			}
			if p := a.cfg.Pipelines.AWSProject; p != "" {
				env = append(env, "AWS_PROFILE="+p)
			}
			if r := a.cfg.Pipelines.AWSRegion; r != "" {
				env = append(env, "AWS_REGION="+r)
			}
		}
	}
	return env
}

// buildWorkspaceProviders constructs the list of workspace Providers
// to register from the live config. Returns only those whose
// credentials are fully populated. Used by UpdateSettings on hot-reload.
func buildWorkspaceProviders(cfg *config.OnlineDataConfig) []workspace.Provider {
	if cfg == nil {
		return nil
	}
	redirect := strings.TrimRight(cfg.Auth.PublicBaseURL, "/") + "/workspace/oauth/callback"
	out := []workspace.Provider{}
	if cfg.Workspace.SlackClientID != "" && cfg.Workspace.SlackClientSecret != "" {
		out = append(out, &workspace.SlackProvider{
			ClientID:     cfg.Workspace.SlackClientID,
			ClientSecret: cfg.Workspace.SlackClientSecret,
			RedirectURL:  redirect,
		})
	}
	if cfg.Workspace.GoogleClientID != "" && cfg.Workspace.GoogleClientSecret != "" {
		out = append(out, &workspace.GoogleProvider{
			ClientID:     cfg.Workspace.GoogleClientID,
			ClientSecret: cfg.Workspace.GoogleClientSecret,
			RedirectURL:  redirect,
		})
	}
	return out
}
