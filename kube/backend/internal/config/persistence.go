package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// PersistedSettings is the subset of OnlineDataConfig that the desktop UI is
// allowed to mutate at runtime. It is serialized as JSON to a file under the
// user's config directory so that values typed into the Settings panel survive
// across restarts. Env vars still bootstrap config.New(); persisted values
// override env when present (the user's most recent UI choice wins).
//
// IMPORTANT: every field UpdateSettings (in api/pkg/app_desktop.go) can write
// must round-trip here, or it will be silently dropped on the next launch.
// A round-trip test in persistence_test.go is the guardrail against drift.
//
// Tier and license info are intentionally NOT persisted — those are gated by
// the license server, not user-editable.
type PersistedSettings struct {
	// Kubernetes
	KubeconfigPath string `json:"kubeconfigPath,omitempty"`
	Context        string `json:"context,omitempty"`
	Namespace      string `json:"namespace,omitempty"`

	// AI / LLM
	DeepSeekAPIKey    string `json:"deepseekApiKey,omitempty"`
	LLMBaseURL        string `json:"llmBaseUrl,omitempty"`
	LLMModel          string `json:"llmModel,omitempty"`
	AnomstackURL      string `json:"anomstackUrl,omitempty"`
	MCPServersConfig  string `json:"mcpServersConfig,omitempty"`
	AgentInstructions string `json:"agentInstructions,omitempty"`
	PrometheusURL     string `json:"prometheusUrl,omitempty"`

	// ArgoCD
	ArgoCDURL      string `json:"argocdUrl,omitempty"`
	ArgoCDToken    string `json:"argocdToken,omitempty"`
	ArgoCDInsecure bool   `json:"argocdInsecure,omitempty"`

	// Security
	SnykToken   string `json:"snykToken,omitempty"`
	TrivyBinary string `json:"trivyBinary,omitempty"`
	FalcoURL    string `json:"falcoUrl,omitempty"`

	// Pipelines. Nested as a snapshot so the "has the user ever
	// touched pipelines?" question has a single answer (HasPipelines).
	// Without that flag we couldn't distinguish "user explicitly set
	// Enabled=false" from "Enabled was never set, env default wins".
	HasPipelines bool                       `json:"hasPipelines,omitempty"`
	Pipelines    PersistedPipelinesSettings `json:"pipelines,omitempty"`

	// Logging
	LogLevel string `json:"logLevel,omitempty"`

	// Auth (sign-in providers). HasAuth gates whether to apply this
	// block on reload — same pattern as HasPipelines — so an explicit
	// "off" survives restart but an unset block lets env vars win.
	HasAuth bool                  `json:"hasAuth,omitempty"`
	Auth    PersistedAuthSettings `json:"auth,omitempty"`

	// Workspace OAuth client credentials. HasWorkspace gated for the
	// same reason as HasAuth.
	HasWorkspace bool                       `json:"hasWorkspace,omitempty"`
	Workspace    PersistedWorkspaceSettings `json:"workspace,omitempty"`
}

// PersistedAuthSettings mirrors the fields of AuthConfig the Settings
// UI is allowed to mutate. PublicBaseURL, DevMode, and the .p8 file
// path are deliberately omitted — those are infra config, not
// per-user settings.
type PersistedAuthSettings struct {
	GoogleClientID     string `json:"googleClientId,omitempty"`
	GoogleClientSecret string `json:"googleClientSecret,omitempty"`

	OIDCIssuer       string `json:"oidcIssuer,omitempty"`
	OIDCClientID     string `json:"oidcClientId,omitempty"`
	OIDCClientSecret string `json:"oidcClientSecret,omitempty"`
	OIDCDisplayName  string `json:"oidcDisplayName,omitempty"`

	AppleServicesID  string `json:"appleServicesId,omitempty"`
	AppleTeamID      string `json:"appleTeamId,omitempty"`
	AppleKeyID       string `json:"appleKeyId,omitempty"`
	ApplePrivateKey  string `json:"applePrivateKey,omitempty"`
	AppleDisplayName string `json:"appleDisplayName,omitempty"`

	AllowLocalSignup bool `json:"allowLocalSignup"`

	PasskeyEnabled  bool   `json:"passkeyEnabled"`
	PasskeyRPID     string `json:"passkeyRpId,omitempty"`
	PasskeyRPName   string `json:"passkeyRpName,omitempty"`
	PasskeyRPOrigin string `json:"passkeyRpOrigin,omitempty"`
}

// PersistedWorkspaceSettings holds the Slack + Google Workspace OAuth
// client credentials.
type PersistedWorkspaceSettings struct {
	GoogleClientID     string `json:"googleClientId,omitempty"`
	GoogleClientSecret string `json:"googleClientSecret,omitempty"`
	SlackClientID      string `json:"slackClientId,omitempty"`
	SlackClientSecret  string `json:"slackClientSecret,omitempty"`
	SlackSigningSecret string `json:"slackSigningSecret,omitempty"`
}

// PersistedPipelinesSettings mirrors PipelinesConfig 1:1 so every field the
// Settings UI mutates round-trips through disk. Booleans are persisted
// unconditionally (no `omitempty`) so an explicit "off" survives reload.
type PersistedPipelinesSettings struct {
	Enabled  bool             `json:"enabled"`
	Provider PipelineProvider `json:"provider,omitempty"`

	GitHubToken    string `json:"githubToken,omitempty"`
	GitHubOwner    string `json:"githubOwner,omitempty"`
	GitHubRepo     string `json:"githubRepo,omitempty"`
	GitHubWorkflow string `json:"githubWorkflow,omitempty"`

	GitLabURL       string `json:"gitlabUrl,omitempty"`
	GitLabToken     string `json:"gitlabToken,omitempty"`
	GitLabProjectID string `json:"gitlabProjectId,omitempty"`
	GitLabRef       string `json:"gitlabRef,omitempty"`

	AWSRegion    string `json:"awsRegion,omitempty"`
	AWSAccessKey string `json:"awsAccessKey,omitempty"`
	AWSSecretKey string `json:"awsSecretKey,omitempty"`
	AWSProject   string `json:"awsProject,omitempty"`

	GCPProject     string `json:"gcpProject,omitempty"`
	GCPRegion      string `json:"gcpRegion,omitempty"`
	GCPCredentials string `json:"gcpCredentials,omitempty"`

	CircleCIToken       string `json:"circleciToken,omitempty"`
	CircleCIProjectSlug string `json:"circleciProjectSlug,omitempty"`

	AzureOrganization string `json:"azureOrganization,omitempty"`
	AzureProject      string `json:"azureProject,omitempty"`
	AzurePipelineID   string `json:"azurePipelineId,omitempty"`
	AzureToken        string `json:"azureToken,omitempty"`
	AzureBranch       string `json:"azureBranch,omitempty"`

	NotifyOnPROpened    bool `json:"notifyOnPrOpened"`
	NotifyOnPRUpdated   bool `json:"notifyOnPrUpdated"`
	NotifyOnPRCommented bool `json:"notifyOnPrCommented"`
	NotifyOnPRMerged    bool `json:"notifyOnPrMerged"`

	AutoCodeReview        bool   `json:"autoCodeReview"`
	CodeReviewDestination string `json:"codeReviewDestination,omitempty"`
	GDriveFolderID        string `json:"gdriveFolderId,omitempty"`
	CodeReviewS3Prefix    string `json:"codeReviewS3Prefix,omitempty"`
	CodeReviewEmailTo     string `json:"codeReviewEmailTo,omitempty"`

	ConfluenceURL          string `json:"confluenceUrl,omitempty"`
	ConfluenceEmail        string `json:"confluenceEmail,omitempty"`
	ConfluenceToken        string `json:"confluenceToken,omitempty"`
	ConfluenceSpaceKey     string `json:"confluenceSpaceKey,omitempty"`
	ConfluenceParentPageID string `json:"confluenceParentPageId,omitempty"`
	NotionToken            string `json:"notionToken,omitempty"`
	NotionDatabaseID       string `json:"notionDatabaseId,omitempty"`
	EvernoteToken          string `json:"evernoteToken,omitempty"`
	EvernoteNotebookGUID   string `json:"evernoteNotebookGuid,omitempty"`
	OneNoteToken           string `json:"onenoteToken,omitempty"`
	OneNoteSectionID       string `json:"onenoteSectionId,omitempty"`
	AmplenoteAPIKey        string `json:"amplenoteApiKey,omitempty"`
	StandardNotesURL       string `json:"standardNotesUrl,omitempty"`
	StandardNotesToken     string `json:"standardNotesToken,omitempty"`
	ObsidianVaultPath      string `json:"obsidianVaultPath,omitempty"`
	JoplinURL              string `json:"joplinUrl,omitempty"`
	JoplinToken            string `json:"joplinToken,omitempty"`
	LogseqGraphPath        string `json:"logseqGraphPath,omitempty"`
	BearToken              string `json:"bearToken,omitempty"`
}

// settingsDirOverride lets tests redirect the persistence path without
// touching the real user config dir. Empty means "use os.UserConfigDir()".
var settingsDirOverride string

// SetSettingsDirForTest overrides the directory used for persisted settings.
// Tests should defer SetSettingsDirForTest("") to restore the default.
func SetSettingsDirForTest(dir string) { settingsDirOverride = dir }

// SettingsDirOverrideForTest returns the current test override (or "" when
// the production user-config-dir path is in effect). Tests use this to
// snapshot/restore the override around their own t.TempDir() redirection.
func SettingsDirOverrideForTest() string { return settingsDirOverride }

// SettingsPath returns the absolute path to the persisted settings file.
func SettingsPath() (string, error) {
	if settingsDirOverride != "" {
		return filepath.Join(settingsDirOverride, "settings.json"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "argus", "settings.json"), nil
}

// LoadPersistedSettings reads the persisted settings file, or returns a zero
// value (and no error) when the file does not yet exist. Malformed files
// return an error so the caller can decide whether to fall through.
func LoadPersistedSettings() (*PersistedSettings, error) {
	path, err := SettingsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &PersistedSettings{}, nil
		}
		return nil, fmt.Errorf("read settings: %w", err)
	}
	var s PersistedSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse settings %s: %w", path, err)
	}
	return &s, nil
}

// SavePersistedSettings writes the provided settings atomically to the
// persistence file. The directory is created with 0o700 and the file with
// 0o600 to keep tokens user-readable only.
func SavePersistedSettings(s *PersistedSettings) error {
	if s == nil {
		return errors.New("nil settings")
	}
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}

// MergeInto applies non-empty persisted settings over the provided config.
// Empty/zero persisted fields leave the existing config untouched, so env
// var defaults still apply when the user has not customized a setting.
//
// Pipelines is gated by HasPipelines: once the user has saved through the
// UI at least once, the persisted snapshot wins wholesale (including
// explicit `false` values). Before that, env defaults stay in place.
func (s *PersistedSettings) MergeInto(cfg *OnlineDataConfig) {
	if s == nil || cfg == nil {
		return
	}
	if s.KubeconfigPath != "" {
		cfg.Kubernetes.Config = s.KubeconfigPath
	}
	if s.Context != "" {
		cfg.Kubernetes.Context = s.Context
	}
	if s.Namespace != "" {
		cfg.Kubernetes.Namespace = s.Namespace
	}
	if s.DeepSeekAPIKey != "" {
		cfg.AI.DeepSeekAPIKey = s.DeepSeekAPIKey
	}
	if s.LLMBaseURL != "" {
		cfg.AI.LLMBaseURL = s.LLMBaseURL
	}
	if s.LLMModel != "" {
		cfg.AI.LLMModel = s.LLMModel
	}
	if s.AnomstackURL != "" {
		cfg.AI.AnomstackURL = s.AnomstackURL
	}
	if s.MCPServersConfig != "" {
		cfg.AI.MCPServersConfig = s.MCPServersConfig
	}
	if s.AgentInstructions != "" {
		cfg.AI.AgentInstructions = s.AgentInstructions
	}
	if s.PrometheusURL != "" {
		cfg.AI.PrometheusURL = s.PrometheusURL
	}
	if s.ArgoCDURL != "" {
		cfg.ArgoCD.URL = s.ArgoCDURL
		// Insecure is only meaningful in the context of a configured URL — tying
		// it to URL presence keeps an empty persisted file from clobbering the
		// ARGOCD_INSECURE env var when the user hasn't touched ArgoCD in the UI.
		cfg.ArgoCD.Insecure = s.ArgoCDInsecure
	}
	if s.ArgoCDToken != "" {
		cfg.ArgoCD.Token = s.ArgoCDToken
	}
	if s.SnykToken != "" {
		cfg.Security.SnykToken = s.SnykToken
	}
	if s.TrivyBinary != "" {
		cfg.Security.TrivyBinary = s.TrivyBinary
	}
	if s.FalcoURL != "" {
		cfg.Security.FalcoURL = s.FalcoURL
	}
	if s.LogLevel != "" {
		cfg.Logging.Level = s.LogLevel
	}

	// Pipelines: once the user has hit Save, the snapshot is
	// authoritative. We replace the whole nested struct so an explicit
	// off-toggle survives reload.
	if s.HasPipelines {
		cfg.Pipelines = s.Pipelines.toConfig()
	}

	// Auth: same authoritative-replace logic, but preserve infra-only
	// fields (PublicBaseURL, DevMode, ApplePrivateKeyFile) that the UI
	// never touches.
	if s.HasAuth {
		s.Auth.applyTo(&cfg.Auth)
	}

	// Workspace OAuth credentials: same gated replace.
	if s.HasWorkspace {
		cfg.Workspace = WorkspaceConfig{
			GoogleClientID:     s.Workspace.GoogleClientID,
			GoogleClientSecret: s.Workspace.GoogleClientSecret,
			SlackClientID:      s.Workspace.SlackClientID,
			SlackClientSecret:  s.Workspace.SlackClientSecret,
			SlackSigningSecret: s.Workspace.SlackSigningSecret,
		}
	}
}

// applyTo overlays the persisted auth fields onto an existing
// AuthConfig, leaving infra-only fields untouched.
func (a PersistedAuthSettings) applyTo(dst *AuthConfig) {
	dst.GoogleClientID = a.GoogleClientID
	dst.GoogleClientSecret = a.GoogleClientSecret
	dst.OIDCIssuer = a.OIDCIssuer
	dst.OIDCClientID = a.OIDCClientID
	dst.OIDCClientSecret = a.OIDCClientSecret
	dst.OIDCDisplayName = a.OIDCDisplayName
	dst.AppleServicesID = a.AppleServicesID
	dst.AppleTeamID = a.AppleTeamID
	dst.AppleKeyID = a.AppleKeyID
	dst.ApplePrivateKey = a.ApplePrivateKey
	dst.AppleDisplayName = a.AppleDisplayName
	dst.AllowLocalSignup = a.AllowLocalSignup
	dst.PasskeyEnabled = a.PasskeyEnabled
	if a.PasskeyRPID != "" {
		dst.PasskeyRPID = a.PasskeyRPID
	}
	if a.PasskeyRPName != "" {
		dst.PasskeyRPName = a.PasskeyRPName
	}
	if a.PasskeyRPOrigin != "" {
		dst.PasskeyRPOrigin = a.PasskeyRPOrigin
	}
}

// FromConfig captures the persistable subset of an OnlineDataConfig.
func FromConfig(cfg *OnlineDataConfig) *PersistedSettings {
	if cfg == nil {
		return &PersistedSettings{}
	}
	return &PersistedSettings{
		KubeconfigPath:    cfg.Kubernetes.Config,
		Context:           cfg.Kubernetes.Context,
		Namespace:         cfg.Kubernetes.Namespace,
		DeepSeekAPIKey:    cfg.AI.DeepSeekAPIKey,
		LLMBaseURL:        cfg.AI.LLMBaseURL,
		LLMModel:          cfg.AI.LLMModel,
		AnomstackURL:      cfg.AI.AnomstackURL,
		MCPServersConfig:  cfg.AI.MCPServersConfig,
		AgentInstructions: cfg.AI.AgentInstructions,
		PrometheusURL:     cfg.AI.PrometheusURL,
		ArgoCDURL:         cfg.ArgoCD.URL,
		ArgoCDToken:       cfg.ArgoCD.Token,
		ArgoCDInsecure:    cfg.ArgoCD.Insecure,
		SnykToken:         cfg.Security.SnykToken,
		TrivyBinary:       cfg.Security.TrivyBinary,
		FalcoURL:          cfg.Security.FalcoURL,
		HasPipelines:      true, // any save through this path is a user action
		Pipelines:         pipelinesFromConfig(cfg.Pipelines),
		LogLevel:          cfg.Logging.Level,
		HasAuth:           true,
		Auth:              authFromConfig(cfg.Auth),
		HasWorkspace:      true,
		Workspace: PersistedWorkspaceSettings{
			GoogleClientID:     cfg.Workspace.GoogleClientID,
			GoogleClientSecret: cfg.Workspace.GoogleClientSecret,
			SlackClientID:      cfg.Workspace.SlackClientID,
			SlackClientSecret:  cfg.Workspace.SlackClientSecret,
			SlackSigningSecret: cfg.Workspace.SlackSigningSecret,
		},
	}
}

func authFromConfig(a AuthConfig) PersistedAuthSettings {
	return PersistedAuthSettings{
		GoogleClientID:     a.GoogleClientID,
		GoogleClientSecret: a.GoogleClientSecret,
		OIDCIssuer:         a.OIDCIssuer,
		OIDCClientID:       a.OIDCClientID,
		OIDCClientSecret:   a.OIDCClientSecret,
		OIDCDisplayName:    a.OIDCDisplayName,
		AppleServicesID:    a.AppleServicesID,
		AppleTeamID:        a.AppleTeamID,
		AppleKeyID:         a.AppleKeyID,
		ApplePrivateKey:    a.ApplePrivateKey,
		AppleDisplayName:   a.AppleDisplayName,
		AllowLocalSignup:   a.AllowLocalSignup,
		PasskeyEnabled:     a.PasskeyEnabled,
		PasskeyRPID:        a.PasskeyRPID,
		PasskeyRPName:      a.PasskeyRPName,
		PasskeyRPOrigin:    a.PasskeyRPOrigin,
	}
}

func pipelinesFromConfig(p PipelinesConfig) PersistedPipelinesSettings {
	return PersistedPipelinesSettings{
		Enabled:                p.Enabled,
		Provider:               p.Provider,
		GitHubToken:            p.GitHubToken,
		GitHubOwner:            p.GitHubOwner,
		GitHubRepo:             p.GitHubRepo,
		GitHubWorkflow:         p.GitHubWorkflow,
		GitLabURL:              p.GitLabURL,
		GitLabToken:            p.GitLabToken,
		GitLabProjectID:        p.GitLabProjectID,
		GitLabRef:              p.GitLabRef,
		AWSRegion:              p.AWSRegion,
		AWSAccessKey:           p.AWSAccessKey,
		AWSSecretKey:           p.AWSSecretKey,
		AWSProject:             p.AWSProject,
		GCPProject:             p.GCPProject,
		GCPRegion:              p.GCPRegion,
		GCPCredentials:         p.GCPCredentials,
		CircleCIToken:          p.CircleCIToken,
		CircleCIProjectSlug:    p.CircleCIProjectSlug,
		AzureOrganization:      p.AzureOrganization,
		AzureProject:           p.AzureProject,
		AzurePipelineID:        p.AzurePipelineID,
		AzureToken:             p.AzureToken,
		AzureBranch:            p.AzureBranch,
		NotifyOnPROpened:       p.NotifyOnPROpened,
		NotifyOnPRUpdated:      p.NotifyOnPRUpdated,
		NotifyOnPRCommented:    p.NotifyOnPRCommented,
		NotifyOnPRMerged:       p.NotifyOnPRMerged,
		AutoCodeReview:         p.AutoCodeReview,
		CodeReviewDestination:  p.CodeReviewDestination,
		GDriveFolderID:         p.GDriveFolderID,
		CodeReviewS3Prefix:     p.CodeReviewS3Prefix,
		CodeReviewEmailTo:      p.CodeReviewEmailTo,
		ConfluenceURL:          p.ConfluenceURL,
		ConfluenceEmail:        p.ConfluenceEmail,
		ConfluenceToken:        p.ConfluenceToken,
		ConfluenceSpaceKey:     p.ConfluenceSpaceKey,
		ConfluenceParentPageID: p.ConfluenceParentPageID,
		NotionToken:            p.NotionToken,
		NotionDatabaseID:       p.NotionDatabaseID,
		EvernoteToken:          p.EvernoteToken,
		EvernoteNotebookGUID:   p.EvernoteNotebookGUID,
		OneNoteToken:           p.OneNoteToken,
		OneNoteSectionID:       p.OneNoteSectionID,
		AmplenoteAPIKey:        p.AmplenoteAPIKey,
		StandardNotesURL:       p.StandardNotesURL,
		StandardNotesToken:     p.StandardNotesToken,
		ObsidianVaultPath:      p.ObsidianVaultPath,
		JoplinURL:              p.JoplinURL,
		JoplinToken:            p.JoplinToken,
		LogseqGraphPath:        p.LogseqGraphPath,
		BearToken:              p.BearToken,
	}
}

func (p PersistedPipelinesSettings) toConfig() PipelinesConfig {
	return PipelinesConfig{
		Enabled:                p.Enabled,
		Provider:               p.Provider,
		GitHubToken:            p.GitHubToken,
		GitHubOwner:            p.GitHubOwner,
		GitHubRepo:             p.GitHubRepo,
		GitHubWorkflow:         p.GitHubWorkflow,
		GitLabURL:              p.GitLabURL,
		GitLabToken:            p.GitLabToken,
		GitLabProjectID:        p.GitLabProjectID,
		GitLabRef:              p.GitLabRef,
		AWSRegion:              p.AWSRegion,
		AWSAccessKey:           p.AWSAccessKey,
		AWSSecretKey:           p.AWSSecretKey,
		AWSProject:             p.AWSProject,
		GCPProject:             p.GCPProject,
		GCPRegion:              p.GCPRegion,
		GCPCredentials:         p.GCPCredentials,
		CircleCIToken:          p.CircleCIToken,
		CircleCIProjectSlug:    p.CircleCIProjectSlug,
		AzureOrganization:      p.AzureOrganization,
		AzureProject:           p.AzureProject,
		AzurePipelineID:        p.AzurePipelineID,
		AzureToken:             p.AzureToken,
		AzureBranch:            p.AzureBranch,
		NotifyOnPROpened:       p.NotifyOnPROpened,
		NotifyOnPRUpdated:      p.NotifyOnPRUpdated,
		NotifyOnPRCommented:    p.NotifyOnPRCommented,
		NotifyOnPRMerged:       p.NotifyOnPRMerged,
		AutoCodeReview:         p.AutoCodeReview,
		CodeReviewDestination:  p.CodeReviewDestination,
		GDriveFolderID:         p.GDriveFolderID,
		CodeReviewS3Prefix:     p.CodeReviewS3Prefix,
		CodeReviewEmailTo:      p.CodeReviewEmailTo,
		ConfluenceURL:          p.ConfluenceURL,
		ConfluenceEmail:        p.ConfluenceEmail,
		ConfluenceToken:        p.ConfluenceToken,
		ConfluenceSpaceKey:     p.ConfluenceSpaceKey,
		ConfluenceParentPageID: p.ConfluenceParentPageID,
		NotionToken:            p.NotionToken,
		NotionDatabaseID:       p.NotionDatabaseID,
		EvernoteToken:          p.EvernoteToken,
		EvernoteNotebookGUID:   p.EvernoteNotebookGUID,
		OneNoteToken:           p.OneNoteToken,
		OneNoteSectionID:       p.OneNoteSectionID,
		AmplenoteAPIKey:        p.AmplenoteAPIKey,
		StandardNotesURL:       p.StandardNotesURL,
		StandardNotesToken:     p.StandardNotesToken,
		ObsidianVaultPath:      p.ObsidianVaultPath,
		JoplinURL:              p.JoplinURL,
		JoplinToken:            p.JoplinToken,
		LogseqGraphPath:        p.LogseqGraphPath,
		BearToken:              p.BearToken,
	}
}
