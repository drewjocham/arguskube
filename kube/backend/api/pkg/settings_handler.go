package pkg

import (
	"log/slog"

	"github.com/argues/argus/internal/config"
)

type SettingsHandler struct {
	app *App
}

func NewSettingsHandler(app *App) *SettingsHandler {
	return &SettingsHandler{app: app}
}

func (h *SettingsHandler) GetFeatures() map[string]bool {
	if h.app.gate == nil {
		return nil
	}
	features := h.app.gate.AllFeatures()
	result := make(map[string]bool, len(features))
	for k, v := range features {
		result[string(k)] = v
	}
	return result
}

func (h *SettingsHandler) GetTier() config.Tier {
	if h.app.cfg == nil {
		return config.TierFree
	}
	return h.app.cfg.Features.Tier
}

func (h *SettingsHandler) GetSettings() SettingsPayload {
	return h.app.GetSettings()
}

func (h *SettingsHandler) UpdateSettings(s SettingsPayload) (SettingsResult, error) {
	h.app.logger.Info("applying settings", slog.String("tier", s.Tier))

	reconnectK8s := h.updateK8sSettings(s)
	rebuildAgent := h.updateAISettings(s)
	_ = h.updateArgoCDSettings(s)

	if s.PipelinesEnabled != h.app.cfg.Pipelines.Enabled {
		h.app.cfg.Pipelines.Enabled = s.PipelinesEnabled
	}
	if s.PipelinesProvider != "" && s.PipelinesProvider != h.app.cfg.Pipelines.Provider {
		h.app.cfg.Pipelines.Provider = s.PipelinesProvider
	}

	h.pipelineSecrets(s)
	h.updatePRNotifications(s)

	if rebuildAgent {
		h.app.rebuildAgent()
	}

	rebuildAuth := h.updateAuthSettings(s)
	rebuildWorkspace := h.updateWorkspaceSettings(s)

	if err := h.app.saveSettings(); err != nil {
		return SettingsResult{}, err
	}

	if rebuildAuth {
		h.app.rebuildAuth()
	}
	if rebuildWorkspace {
		h.app.rebuildWorkspaceProviders()
	}

	return SettingsResult{ReconnectK8s: reconnectK8s}, nil
}

type SettingsResult struct {
	ReconnectK8s bool `json:"reconnectK8s"`
}

func (h *SettingsHandler) updateK8sSettings(s SettingsPayload) bool {
	reconnect := false
	if s.KubeconfigPath != "" && s.KubeconfigPath != h.app.cfg.Kubernetes.Config {
		h.app.cfg.Kubernetes.Config = s.KubeconfigPath
		reconnect = true
	}
	if s.CurrentContext != "" && s.CurrentContext != h.app.cfg.Kubernetes.Context {
		h.app.cfg.Kubernetes.Context = s.CurrentContext
		reconnect = true
	}
	if s.Namespace != "" && s.Namespace != h.app.cfg.Kubernetes.Namespace {
		h.app.cfg.Kubernetes.Namespace = s.Namespace
		reconnect = true
	}
	return reconnect
}

func (h *SettingsHandler) updateAISettings(s SettingsPayload) bool {
	reconnect := false
	if s.LLMBaseURL != "" && s.LLMBaseURL != h.app.cfg.AI.LLMBaseURL {
		h.app.cfg.AI.LLMBaseURL = s.LLMBaseURL
		reconnect = true
	}
	if s.LLMModel != "" && s.LLMModel != h.app.cfg.AI.LLMModel {
		h.app.cfg.AI.LLMModel = s.LLMModel
		reconnect = true
	}
	if s.AgentInstructions != "" && s.AgentInstructions != h.app.cfg.AI.AgentInstructions {
		h.app.cfg.AI.AgentInstructions = s.AgentInstructions
	}
	if s.MCPServersConfig != "" && s.MCPServersConfig != h.app.cfg.AI.MCPServersConfig {
		h.app.cfg.AI.MCPServersConfig = s.MCPServersConfig
	}
	if s.DeepSeekAPIKey != "" && s.DeepSeekAPIKey != SentinelUnchanged {
		h.app.cfg.AI.DeepSeekAPIKey = s.DeepSeekAPIKey
		reconnect = true
		h.app.logger.Info("AI API key updated, will rebuild agent")
	}
	return reconnect
}

func (h *SettingsHandler) updateArgoCDSettings(s SettingsPayload) bool {
	reconnect := false
	if s.ArgoCDURL != "" && s.ArgoCDURL != h.app.cfg.ArgoCD.URL {
		h.app.cfg.ArgoCD.URL = s.ArgoCDURL
		reconnect = true
	}
	if s.ArgoCDToken != "" && s.ArgoCDToken != SentinelUnchanged {
		h.app.cfg.ArgoCD.Token = s.ArgoCDToken
		reconnect = true
	}
	h.app.cfg.ArgoCD.Insecure = s.ArgoCDInsecure
	return reconnect
}

func (h *SettingsHandler) pipelineSecrets(s SettingsPayload) {
	if s.GitHubToken != "" && s.GitHubToken != SentinelUnchanged {
		h.app.cfg.Pipelines.GitHubToken = s.GitHubToken
	}
	if s.GitLabToken != "" && s.GitLabToken != SentinelUnchanged {
		h.app.cfg.Pipelines.GitLabToken = s.GitLabToken
	}
	if s.CircleCIToken != "" && s.CircleCIToken != SentinelUnchanged {
		h.app.cfg.Pipelines.CircleCIToken = s.CircleCIToken
	}
	if s.AWSSecretKey != "" && s.AWSSecretKey != SentinelUnchanged {
		h.app.cfg.Pipelines.AWSSecretKey = s.AWSSecretKey
	}
	if s.AzureToken != "" && s.AzureToken != SentinelUnchanged {
		h.app.cfg.Pipelines.AzureToken = s.AzureToken
	}
	if s.GCPCredentials != "" && s.GCPCredentials != SentinelUnchanged {
		h.app.cfg.Pipelines.GCPCredentials = s.GCPCredentials
	}
	if s.ConfluenceToken != "" && s.ConfluenceToken != SentinelUnchanged {
		h.app.cfg.Pipelines.ConfluenceToken = s.ConfluenceToken
	}
	if s.NotionToken != "" && s.NotionToken != SentinelUnchanged {
		h.app.cfg.Pipelines.NotionToken = s.NotionToken
	}
}

// updatePRNotifications applies PR notification toggle settings.
func (h *SettingsHandler) updatePRNotifications(s SettingsPayload) {
	anyEnabled := s.NotifyOnPROpened || s.NotifyOnPRUpdated ||
		s.NotifyOnPRCommented || s.NotifyOnPRMerged
	h.app.cfg.Pipelines.NotifyOnPROpened = s.NotifyOnPROpened
	h.app.cfg.Pipelines.NotifyOnPRUpdated = s.NotifyOnPRUpdated
	h.app.cfg.Pipelines.NotifyOnPRCommented = s.NotifyOnPRCommented
	h.app.cfg.Pipelines.NotifyOnPRMerged = s.NotifyOnPRMerged
	_ = anyEnabled
}

func (h *SettingsHandler) updateAuthSettings(s SettingsPayload) bool {
	changed := false

	if s.GoogleClientID != "" && s.GoogleClientID != h.app.cfg.Auth.GoogleClientID {
		h.app.cfg.Auth.GoogleClientID = s.GoogleClientID
		changed = true
	}
	if s.GoogleClientSecret != "" && s.GoogleClientSecret != SentinelUnchanged {
		h.app.cfg.Auth.GoogleClientSecret = s.GoogleClientSecret
		changed = true
	}
	if s.OIDCIssuer != "" && s.OIDCIssuer != h.app.cfg.Auth.OIDCIssuer {
		h.app.cfg.Auth.OIDCIssuer = s.OIDCIssuer
		changed = true
	}
	if s.OIDCClientID != "" && s.OIDCClientID != h.app.cfg.Auth.OIDCClientID {
		h.app.cfg.Auth.OIDCClientID = s.OIDCClientID
		changed = true
	}
	if s.OIDCClientSecret != "" && s.OIDCClientSecret != SentinelUnchanged {
		h.app.cfg.Auth.OIDCClientSecret = s.OIDCClientSecret
		changed = true
	}
	if s.OIDCDisplayName != "" && s.OIDCDisplayName != h.app.cfg.Auth.OIDCDisplayName {
		h.app.cfg.Auth.OIDCDisplayName = s.OIDCDisplayName
		changed = true
	}
	if s.AppleServicesID != "" && s.AppleServicesID != h.app.cfg.Auth.AppleServicesID {
		h.app.cfg.Auth.AppleServicesID = s.AppleServicesID
		changed = true
	}
	if s.AppleTeamID != "" && s.AppleTeamID != h.app.cfg.Auth.AppleTeamID {
		h.app.cfg.Auth.AppleTeamID = s.AppleTeamID
		changed = true
	}
	if s.AppleKeyID != "" && s.AppleKeyID != h.app.cfg.Auth.AppleKeyID {
		h.app.cfg.Auth.AppleKeyID = s.AppleKeyID
		changed = true
	}
	if s.ApplePrivateKey != "" && s.ApplePrivateKey != SentinelUnchanged {
		h.app.cfg.Auth.ApplePrivateKey = s.ApplePrivateKey
		changed = true
	}
	if s.AppleDisplayName != "" && s.AppleDisplayName != h.app.cfg.Auth.AppleDisplayName {
		h.app.cfg.Auth.AppleDisplayName = s.AppleDisplayName
		changed = true
	}

	h.app.cfg.Auth.AllowLocalSignup = s.AllowLocalSignup

	if s.PasskeyEnabled != h.app.cfg.Auth.PasskeyEnabled {
		h.app.cfg.Auth.PasskeyEnabled = s.PasskeyEnabled
		changed = true
	}
	if s.PasskeyRPID != "" && s.PasskeyRPID != h.app.cfg.Auth.PasskeyRPID {
		h.app.cfg.Auth.PasskeyRPID = s.PasskeyRPID
		changed = true
	}
	if s.PasskeyRPName != "" && s.PasskeyRPName != h.app.cfg.Auth.PasskeyRPName {
		h.app.cfg.Auth.PasskeyRPName = s.PasskeyRPName
		changed = true
	}
	if s.PasskeyRPOrigin != "" && s.PasskeyRPOrigin != h.app.cfg.Auth.PasskeyRPOrigin {
		h.app.cfg.Auth.PasskeyRPOrigin = s.PasskeyRPOrigin
		changed = true
	}

	return changed
}

func (h *SettingsHandler) updateWorkspaceSettings(s SettingsPayload) bool {
	changed := false

	if s.WorkspaceGoogleClientID != "" && s.WorkspaceGoogleClientID != h.app.cfg.Workspace.GoogleClientID {
		h.app.cfg.Workspace.GoogleClientID = s.WorkspaceGoogleClientID
		changed = true
	}
	if s.WorkspaceGoogleClientSecret != "" && s.WorkspaceGoogleClientSecret != SentinelUnchanged {
		h.app.cfg.Workspace.GoogleClientSecret = s.WorkspaceGoogleClientSecret
		changed = true
	}
	if s.SlackClientID != "" && s.SlackClientID != h.app.cfg.Workspace.SlackClientID {
		h.app.cfg.Workspace.SlackClientID = s.SlackClientID
		changed = true
	}
	if s.SlackClientSecret != "" && s.SlackClientSecret != SentinelUnchanged {
		h.app.cfg.Workspace.SlackClientSecret = s.SlackClientSecret
		changed = true
	}
	if s.SlackSigningSecret != "" && s.SlackSigningSecret != SentinelUnchanged {
		h.app.cfg.Workspace.SlackSigningSecret = s.SlackSigningSecret
		changed = true
	}

	return changed
}
