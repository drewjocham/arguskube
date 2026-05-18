package pkg

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/argus/internal/agentconn"
	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/config"
	"github.com/argues/argus/internal/k8s"
	"github.com/argues/argus/internal/workspace"
)

func (a *App) emitStatus(source, severity, message, detail string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, EventArgusStatus, map[string]any{
		"source":   source,
		"severity": severity,
		"message":  message,
		"detail":   detail,
	})
}

func (a *App) HandleURL(u string) {
	a.logger.Info("Received custom URL", slog.String("url", u))
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, EventDeepLink, u)
	}
}

func (a *App) SwitchContext(name string) error {
	if a.k8s == nil {
		a.logger.Info("bootstrapping k8s client on first context switch",
			slog.String("context", name))
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
	a.rebuildAgentConn()
	a.persistContext(name)
	return nil
}

func (a *App) GetSettings() SettingsPayload {
	payload := SettingsPayload{
		KubeconfigPath:    a.cfg.Kubernetes.Config,
		CurrentContext:    a.cfg.Kubernetes.Context,
		Namespace:         a.cfg.Kubernetes.Namespace,
		LLMBaseURL:        a.cfg.AI.LLMBaseURL,
		LLMModel:          a.cfg.AI.LLMModel,
		AgentInstructions: a.cfg.AI.AgentInstructions,
		MCPServersConfig:  a.cfg.AI.MCPServersConfig,
		Tier:              string(a.cfg.Features.Tier),
		LogLevel:          a.cfg.Logging.Level,

		// Sign-in provider fields
		GoogleClientID:   a.cfg.Auth.GoogleClientID,
		OIDCIssuer:       a.cfg.Auth.OIDCIssuer,
		OIDCClientID:     a.cfg.Auth.OIDCClientID,
		OIDCDisplayName:  a.cfg.Auth.OIDCDisplayName,
		AppleServicesID:  a.cfg.Auth.AppleServicesID,
		AppleTeamID:      a.cfg.Auth.AppleTeamID,
		AppleKeyID:       a.cfg.Auth.AppleKeyID,
		AppleDisplayName: a.cfg.Auth.AppleDisplayName,
		AllowLocalSignup: a.cfg.Auth.AllowLocalSignup,
		PasskeyEnabled:   a.cfg.Auth.PasskeyEnabled,
		PasskeyRPID:      a.cfg.Auth.PasskeyRPID,
		PasskeyRPName:    a.cfg.Auth.PasskeyRPName,
		PasskeyRPOrigin:  a.cfg.Auth.PasskeyRPOrigin,

		// Workspace OAuth client fields
		WorkspaceGoogleClientID: a.cfg.Workspace.GoogleClientID,
		SlackClientID:           a.cfg.Workspace.SlackClientID,
	}
	if a.cfg.AI.DeepSeekAPIKey != "" {
		payload.DeepSeekAPIKey = maskSecret(a.cfg.AI.DeepSeekAPIKey)
	}
	if a.cfg.ArgoCD.Token != "" {
		payload.ArgoCDToken = maskSecret(a.cfg.ArgoCD.Token)
	}
	if a.cfg.Auth.GoogleClientSecret != "" {
		payload.GoogleClientSecret = maskSecret(a.cfg.Auth.GoogleClientSecret)
	}
	if a.cfg.Auth.OIDCClientSecret != "" {
		payload.OIDCClientSecret = maskSecret(a.cfg.Auth.OIDCClientSecret)
	}
	if a.cfg.Auth.ApplePrivateKey != "" {
		payload.ApplePrivateKey = maskSecret(a.cfg.Auth.ApplePrivateKey)
	}
	if a.cfg.Workspace.GoogleClientSecret != "" {
		payload.WorkspaceGoogleClientSecret = maskSecret(a.cfg.Workspace.GoogleClientSecret)
	}
	if a.cfg.Workspace.SlackClientSecret != "" {
		payload.SlackClientSecret = maskSecret(a.cfg.Workspace.SlackClientSecret)
	}
	if a.cfg.Workspace.SlackSigningSecret != "" {
		payload.SlackSigningSecret = maskSecret(a.cfg.Workspace.SlackSigningSecret)
	}
	payload.PipelinesEnabled = a.cfg.Pipelines.Enabled
	payload.PipelinesProvider = a.cfg.Pipelines.Provider
	payload.GitHubOwner = a.cfg.Pipelines.GitHubOwner
	payload.GitHubRepo = a.cfg.Pipelines.GitHubRepo
	payload.GitLabURL = a.cfg.Pipelines.GitLabURL
	payload.GitLabProjectID = a.cfg.Pipelines.GitLabProjectID
	payload.ArgoCDURL = a.cfg.ArgoCD.URL
	payload.ArgoCDInsecure = a.cfg.ArgoCD.Insecure
	return payload
}

func (a *App) saveSettings() error {
	if err := config.SavePersistedSettings(config.FromConfig(a.cfg)); err != nil {
		a.logger.Warn("could not persist settings", slog.String("error", err.Error()))
		return err
	}
	return nil
}

// UpdateSettings is a thin wrapper so tests and Wails bindings can call
// it directly on *App. Delegates to SettingsHandler.
func (a *App) UpdateSettings(s SettingsPayload) (SettingsResult, error) {
	return NewSettingsHandler(a).UpdateSettings(s)
}

func (a *App) rebuildAgent() {
	if a.cfg.AI.DeepSeekAPIKey == "" {
		a.agent = nil
		return
	}
	client := ai.NewDeepSeekClient(a.cfg.AI.DeepSeekAPIKey, a.logger)
	if a.agent != nil {
		a.agent.SetClient(client)
		return
	}
	a.agent = ai.NewAgent(client, a.logger)
	a.logger.Info("AI agent rebuilt")
}

// rebuildAuth hot-reloads the OIDC provider(s) + Apple + passkey config
// without a backend restart. Called after settings update applies new
// Google / OIDC / Apple / passkey credentials.
func (a *App) rebuildAuth() {
	if a.auth == nil || a.auth.store == nil {
		return
	}
	a.SetupAuth(a.auth.store, a.cfg.Auth)
	a.logger.Info("auth providers rebuilt")
}

// rebuildWorkspaceProviders hot-reloads workspace OAuth providers
// (Google Docs/Sheets/Tasks, Slack) without a backend restart.
func (a *App) rebuildWorkspaceProviders() {
	if a.workspace == nil {
		return
	}
	providers := []workspace.Provider{}
	id, sec := a.cfg.Workspace.GoogleClientID, a.cfg.Workspace.GoogleClientSecret
	providers = append(providers, &workspace.GoogleProvider{
		ClientID:     id,
		ClientSecret: sec,
		RedirectURL:  strings.TrimRight(a.cfg.Auth.PublicBaseURL, "/") + "/workspace/oauth/callback",
	})
	if id, sec := a.cfg.Workspace.SlackClientID, a.cfg.Workspace.SlackClientSecret; id != "" && sec != "" {
		providers = append(providers, &workspace.SlackProvider{
			ClientID:     id,
			ClientSecret: sec,
			RedirectURL:  strings.TrimRight(a.cfg.Auth.PublicBaseURL, "/") + "/workspace/oauth/callback",
		})
	}
	// iCloud is always available — it doesn't need OAuth client credentials
	// (uses app-specific passwords validated via CalDAV).
	providers = append(providers, workspace.NewICloudProvider())
	a.workspace.ReregisterProviders(providers)
	if len(providers) > 0 {
		a.logger.Info("workspace providers rebuilt", slog.Int("count", len(providers)))
	}
}

func (a *App) rebuildAgentConn() {
	if a.k8s == nil {
		return
	}
	a.agentConn = agentconn.New(
		a.k8s.GetClientset(),
		a.k8s.GetRestConfig(),
		a.logger.With("component", "agentconn"),
	)
}

func (a *App) persistContext(name string) {
	if err := config.SavePersistedSettings(config.FromConfig(a.cfg)); err != nil {
		a.logger.Warn("could not persist context switch",
			slog.String("context", name),
			slog.String("error", err.Error()))
	}
}

func (a *App) closeExecSession() {
	if a.execSession != nil {
		a.execSession.Close()
		a.execSession = nil
	}
}

func (a *App) findTerminalBinary() string {
	path := filepath.Join("build", "bin", "argus.app",
		"Contents", "Resources", "ArgusTerminal.app",
		"Contents", "MacOS", "ArgusTerminal")
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return path
	}
	return ""
}

type SettingsPayload struct {
	KubeconfigPath      string `json:"kubeconfigPath"`
	CurrentContext      string `json:"currentContext"`
	Namespace           string `json:"namespace"`
	DeepSeekAPIKey      string `json:"deepseekApiKey"`
	LLMBaseURL          string `json:"llmBaseUrl"`
	LLMModel            string `json:"llmModel"`
	MCPServersConfig    string `json:"mcpServersConfig"`
	AgentInstructions   string `json:"agentInstructions"`
	ArgoCDURL           string `json:"argocdUrl"`
	ArgoCDToken         string `json:"argocdToken"`
	ArgoCDInsecure      bool   `json:"argocdInsecure"`
	PipelinesEnabled    bool   `json:"pipelinesEnabled"`
	PipelinesProvider   string `json:"pipelinesProvider"`
	GitHubOwner         string `json:"githubOwner"`
	GitHubRepo          string `json:"githubRepo"`
	GitHubWorkflow      string `json:"githubWorkflow"`
	GitHubToken         string `json:"githubToken"`
	GitLabURL           string `json:"gitlabUrl"`
	GitLabProjectID     string `json:"gitlabProjectId"`
	GitLabRef           string `json:"gitlabRef"`
	GitLabToken         string `json:"gitlabToken"`
	AWSRegion           string `json:"awsRegion"`
	AWSAccessKey        string `json:"awsAccessKey"`
	AWSSecretKey        string `json:"awsSecretKey"`
	GCPProject          string `json:"gcpProject"`
	GCPRegion           string `json:"gcpRegion"`
	GCPCredentials      string `json:"gcpCredentials"`
	CircleCIToken       string `json:"circleciToken"`
	CircleCIProjectSlug string `json:"circleciProjectSlug"`
	AzureOrganization   string `json:"azureOrganization"`
	AzureProject        string `json:"azureProject"`
	AzurePipelineID     string `json:"azurePipelineId"`
	AzureToken          string `json:"azureToken"`
	AzureBranch         string `json:"azureBranch"`
	NotifyOnPROpened    bool   `json:"notifyOnPrOpened"`
	NotifyOnPRUpdated   bool   `json:"notifyOnPrUpdated"`
	NotifyOnPRCommented bool   `json:"notifyOnPrCommented"`
	NotifyOnPRMerged    bool   `json:"notifyOnPrMerged"`
	ConfluenceURL       string `json:"confluenceUrl"`
	ConfluenceToken     string `json:"confluenceToken"`
	NotionToken         string `json:"notionToken"`
	Tier                string `json:"tier"`
	LogLevel            string `json:"logLevel"`

	// Sign-in providers (auth)
	GoogleClientID     string `json:"googleClientId"`
	GoogleClientSecret string `json:"googleClientSecret"`
	OIDCIssuer         string `json:"oidcIssuer"`
	OIDCClientID       string `json:"oidcClientId"`
	OIDCClientSecret   string `json:"oidcClientSecret"`
	OIDCDisplayName    string `json:"oidcDisplayName"`
	AppleServicesID    string `json:"appleServicesId"`
	AppleTeamID        string `json:"appleTeamId"`
	AppleKeyID         string `json:"appleKeyId"`
	ApplePrivateKey    string `json:"applePrivateKey"`
	AppleDisplayName   string `json:"appleDisplayName"`
	AllowLocalSignup   bool   `json:"allowLocalSignup"`
	PasskeyEnabled     bool   `json:"passkeyEnabled"`
	PasskeyRPID        string `json:"passkeyRpId"`
	PasskeyRPName      string `json:"passkeyRpName"`
	PasskeyRPOrigin    string `json:"passkeyRpOrigin"`

	// Workspace OAuth clients
	WorkspaceGoogleClientID     string `json:"workspaceGoogleClientId"`
	WorkspaceGoogleClientSecret string `json:"workspaceGoogleClientSecret"`
	SlackClientID               string `json:"slackClientId"`
	SlackClientSecret           string `json:"slackClientSecret"`
	SlackSigningSecret          string `json:"slackSigningSecret"`
	// Microsoft 365
	WorkspaceMicrosoftClientID     string `json:"workspaceMicrosoftClientId"`
	WorkspaceMicrosoftClientSecret string `json:"workspaceMicrosoftClientSecret"`
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return s[:1] + "•••"
	}
	return s[:4] + "•••" + s[len(s)-4:]
}
