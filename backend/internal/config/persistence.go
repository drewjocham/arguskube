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
// Tier and license info are intentionally NOT persisted — those are gated by
// the license server, not user-editable.
type PersistedSettings struct {
	KubeconfigPath   string `json:"kubeconfigPath,omitempty"`
	Context          string `json:"context,omitempty"`
	Namespace        string `json:"namespace,omitempty"`
	DeepSeekAPIKey   string `json:"deepseekApiKey,omitempty"`
	AnomstackURL      string `json:"anomstackUrl,omitempty"`
	MCPServersConfig  string `json:"mcpServersConfig,omitempty"`
	AgentInstructions string `json:"agentInstructions,omitempty"`
	PrometheusURL     string `json:"prometheusUrl,omitempty"`
	ArgoCDURL      string `json:"argocdUrl,omitempty"`
	ArgoCDToken    string `json:"argocdToken,omitempty"`
	ArgoCDInsecure bool   `json:"argocdInsecure,omitempty"`
	SnykToken      string `json:"snykToken,omitempty"`
	TrivyBinary    string `json:"trivyBinary,omitempty"`
	FalcoURL       string `json:"falcoUrl,omitempty"`
	LogLevel       string `json:"logLevel,omitempty"`
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
	return filepath.Join(dir, "kubewatcher", "settings.json"), nil
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
}

// FromConfig captures the persistable subset of an OnlineDataConfig.
func FromConfig(cfg *OnlineDataConfig) *PersistedSettings {
	if cfg == nil {
		return &PersistedSettings{}
	}
	return &PersistedSettings{
		KubeconfigPath: cfg.Kubernetes.Config,
		Context:        cfg.Kubernetes.Context,
		Namespace:      cfg.Kubernetes.Namespace,
		DeepSeekAPIKey:   cfg.AI.DeepSeekAPIKey,
		AnomstackURL:      cfg.AI.AnomstackURL,
		MCPServersConfig:  cfg.AI.MCPServersConfig,
		AgentInstructions: cfg.AI.AgentInstructions,
		PrometheusURL:     cfg.AI.PrometheusURL,
		ArgoCDURL:      cfg.ArgoCD.URL,
		ArgoCDToken:    cfg.ArgoCD.Token,
		ArgoCDInsecure: cfg.ArgoCD.Insecure,
		SnykToken:      cfg.Security.SnykToken,
		TrivyBinary:    cfg.Security.TrivyBinary,
		FalcoURL:       cfg.Security.FalcoURL,
		LogLevel:       cfg.Logging.Level,
	}
}
