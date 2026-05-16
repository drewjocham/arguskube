package pkg

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/argues/argus/internal/agentconn"
	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/config"
	"github.com/argues/argus/internal/k8s"
)

// emitStatus publishes a StatusEvent for the frontend StatusRibbon.
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

// HandleURL handles deep links from custom URL schemes like argus://
func (a *App) HandleURL(u string) {
	a.logger.Info("Received custom URL", slog.String("url", u))
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, EventDeepLink, u)
	}
}

// SwitchContext changes the active kubeconfig context at runtime.
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

// GetSettings returns the current settings payload.
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
	}
	if a.cfg.AI.DeepSeekAPIKey != "" {
		payload.DeepSeekAPIKey = maskSecret(a.cfg.AI.DeepSeekAPIKey)
	}
	if a.cfg.ArgoCD.Token != "" {
		payload.ArgoCDToken = maskSecret(a.cfg.ArgoCD.Token)
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

// SettingsPayload is the JSON structure for runtime config.
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
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return s[:1] + "•••"
	}
	return s[:4] + "•••" + s[len(s)-4:]
}
