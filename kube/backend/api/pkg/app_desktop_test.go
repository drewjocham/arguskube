package pkg

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/config"
	"github.com/argues/argus/internal/features"
)

func TestGetAppMode_Default(t *testing.T) {
	a := &App{}
	mode := a.GetAppMode()
	if mode != "dashboard" {
		t.Errorf("expected 'dashboard', got %q", mode)
	}
}

func TestGetAppMode_Custom(t *testing.T) {
	a := &App{appMode: "terminal"}
	mode := a.GetAppMode()
	if mode != "terminal" {
		t.Errorf("expected 'terminal', got %q", mode)
	}
}

func TestSetPaused(t *testing.T) {
	a := &App{}
	a.SetPaused(true)
	if !a.paused.Load() {
		t.Error("expected paused to be true")
	}
	a.SetPaused(false)
	if a.paused.Load() {
		t.Error("expected paused to be false")
	}
}

// newDesktopAppForTest builds a minimal *App suitable for exercising
// UpdateSettings/GetSettings behaviour without bringing up a real Wails
// runtime, k8s client, or argocd client.
func newDesktopAppForTest(cfg *config.OnlineDataConfig) *App {
	logger := slog.New(slog.DiscardHandler)
	return &App{
		ctx:    context.Background(),
		logger: logger,
		cfg:    cfg,
		gate:   features.NewGate(cfg),
	}
}

// TestUpdateSettings_PersistsToDiskAndAcrossLoad verifies that values typed
// into the Settings panel are written to settings.json and resurface on the
// next config.New() call (simulated restart).
func TestUpdateSettings_PersistsToDiskAndAcrossLoad(t *testing.T) {
	dir := t.TempDir()
	prevOverride := config.SettingsDirOverrideForTest()
	config.SetSettingsDirForTest(dir)
	t.Cleanup(func() { config.SetSettingsDirForTest(prevOverride) })

	cfg := &config.OnlineDataConfig{}
	app := newDesktopAppForTest(cfg)

	if err := app.UpdateSettings(SettingsPayload{
		DeepSeekAPIKey: "sk-from-ui",
		AnomstackURL:   "http://anom",
		PrometheusURL:  "http://prom",
		Namespace:      "kw",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	path := filepath.Join(dir, "settings.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected settings.json at %s: %v", path, err)
	}

	loaded, err := config.LoadPersistedSettings()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.DeepSeekAPIKey != "sk-from-ui" {
		t.Errorf("expected persisted DeepSeekAPIKey 'sk-from-ui', got %q", loaded.DeepSeekAPIKey)
	}
	if loaded.AnomstackURL != "http://anom" {
		t.Errorf("expected persisted AnomstackURL, got %q", loaded.AnomstackURL)
	}
}

// TestUpdateSettings_BuildsAgentWhenNoneExisted verifies the bug fix: when
// the user pastes an API key into the UI and the agent is currently nil
// (no key at startup), UpdateSettings constructs a new agent so the next
// chat call doesn't error with "AI agent not configured".
func TestUpdateSettings_BuildsAgentWhenNoneExisted(t *testing.T) {
	dir := t.TempDir()
	prevOverride := config.SettingsDirOverrideForTest()
	config.SetSettingsDirForTest(dir)
	t.Cleanup(func() { config.SetSettingsDirForTest(prevOverride) })

	cfg := &config.OnlineDataConfig{}
	app := newDesktopAppForTest(cfg)
	if app.agent != nil {
		t.Fatal("precondition: expected app.agent == nil before update")
	}

	if err := app.UpdateSettings(SettingsPayload{DeepSeekAPIKey: "sk-key"}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if app.agent == nil {
		t.Fatal("expected app.agent to be constructed after pasting an API key")
	}
	if !app.agent.HasClient() {
		t.Error("expected new agent to have a client")
	}
}

// TestUpdateSettings_HotSwapsExistingAgentClient verifies that with an
// already-running agent, updating the API key swaps the client in place
// instead of replacing the whole agent (preserves chat history).
func TestUpdateSettings_HotSwapsExistingAgentClient(t *testing.T) {
	dir := t.TempDir()
	prevOverride := config.SettingsDirOverrideForTest()
	config.SetSettingsDirForTest(dir)
	t.Cleanup(func() { config.SetSettingsDirForTest(prevOverride) })

	cfg := &config.OnlineDataConfig{}
	cfg.AI.DeepSeekAPIKey = "sk-old"
	logger := slog.New(slog.DiscardHandler)
	app := newDesktopAppForTest(cfg)
	app.agent = ai.NewAgent(ai.NewDeepSeekClient("sk-old", logger), logger)
	originalAgent := app.agent

	if err := app.UpdateSettings(SettingsPayload{DeepSeekAPIKey: "sk-new"}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if app.agent != originalAgent {
		t.Error("expected SetClient hot-swap, but app.agent pointer was replaced (history would be lost)")
	}
	if !app.agent.HasClient() {
		t.Error("expected agent to still have a client after swap")
	}
	if cfg.AI.DeepSeekAPIKey != "sk-new" {
		t.Errorf("expected cfg.AI.DeepSeekAPIKey 'sk-new', got %q", cfg.AI.DeepSeekAPIKey)
	}
}

// TestUpdateSettings_IgnoresMaskedAPIKey verifies that posting back the
// masked display value doesn't overwrite the real key with the masked
// string (which would break the next request).
func TestUpdateSettings_IgnoresMaskedAPIKey(t *testing.T) {
	dir := t.TempDir()
	prevOverride := config.SettingsDirOverrideForTest()
	config.SetSettingsDirForTest(dir)
	t.Cleanup(func() { config.SetSettingsDirForTest(prevOverride) })

	cfg := &config.OnlineDataConfig{}
	cfg.AI.DeepSeekAPIKey = "sk-real-key"
	app := newDesktopAppForTest(cfg)

	if err := app.UpdateSettings(SettingsPayload{DeepSeekAPIKey: "sk-r…-key"}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if cfg.AI.DeepSeekAPIKey != "sk-real-key" {
		t.Errorf("masked submission should be ignored, but key is now %q", cfg.AI.DeepSeekAPIKey)
	}
}

// TestGetSettings_MasksAuthAndWorkspaceSecrets verifies all the new
// sign-in & integration secrets are masked on read so a shoulder-surfer
// can't read them from the Settings panel.
func TestGetSettings_MasksAuthAndWorkspaceSecrets(t *testing.T) {
	cfg := &config.OnlineDataConfig{}
	cfg.Auth.GoogleClientID = "client-id-public"
	cfg.Auth.GoogleClientSecret = "google-very-secret-value"
	cfg.Auth.OIDCClientSecret = "oidc-very-secret-value"
	cfg.Auth.ApplePrivateKey = "-----BEGIN PRIVATE KEY-----\nstuff\n-----END PRIVATE KEY-----"
	cfg.Workspace.GoogleClientSecret = "ws-google-secret-value"
	cfg.Workspace.SlackClientSecret = "slack-secret-value"
	cfg.Workspace.SlackSigningSecret = "slack-signing-secret"

	app := newDesktopAppForTest(cfg)
	got := app.GetSettings()

	if got.GoogleClientID != "client-id-public" {
		t.Errorf("expected googleClientId echoed plain, got %q", got.GoogleClientID)
	}
	for name, v := range map[string]string{
		"googleClientSecret":          got.GoogleClientSecret,
		"oidcClientSecret":            got.OIDCClientSecret,
		"applePrivateKey":             got.ApplePrivateKey,
		"workspaceGoogleClientSecret": got.WorkspaceGoogleClientSecret,
		"slackClientSecret":           got.SlackClientSecret,
		"slackSigningSecret":          got.SlackSigningSecret,
	} {
		if v == "" {
			t.Errorf("%s is empty — expected a mask", name)
			continue
		}
		if !strings.Contains(v, "…") && !strings.Contains(v, "•") {
			t.Errorf("%s expected mask character, got %q", name, v)
		}
	}
}

// TestUpdateSettings_AppliesAuthAndWorkspace verifies the new fields
// land on cfg.Auth / cfg.Workspace after a save.
func TestUpdateSettings_AppliesAuthAndWorkspace(t *testing.T) {
	dir := t.TempDir()
	prevOverride := config.SettingsDirOverrideForTest()
	config.SetSettingsDirForTest(dir)
	t.Cleanup(func() { config.SetSettingsDirForTest(prevOverride) })

	cfg := &config.OnlineDataConfig{}
	app := newDesktopAppForTest(cfg)

	if err := app.UpdateSettings(SettingsPayload{
		GoogleClientID:              "g-id",
		GoogleClientSecret:          "g-secret",
		OIDCIssuer:                  "https://acme.okta.com",
		OIDCClientID:                "o-id",
		OIDCClientSecret:            "o-secret",
		WorkspaceGoogleClientID:     "ws-g-id",
		WorkspaceGoogleClientSecret: "ws-g-secret",
		SlackClientID:               "s-id",
		SlackClientSecret:           "s-secret",
		SlackSigningSecret:          "s-sig",
		PasskeyEnabled:              true,
		PasskeyRPID:                 "localhost",
		PasskeyRPName:               "Argus",
		PasskeyRPOrigin:             "http://localhost:8080",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if cfg.Auth.GoogleClientID != "g-id" || cfg.Auth.GoogleClientSecret != "g-secret" {
		t.Errorf("Google sign-in creds not applied: %+v", cfg.Auth)
	}
	if cfg.Auth.OIDCIssuer != "https://acme.okta.com" || cfg.Auth.OIDCClientSecret != "o-secret" {
		t.Errorf("OIDC creds not applied: %+v", cfg.Auth)
	}
	if !cfg.Auth.PasskeyEnabled || cfg.Auth.PasskeyRPID != "localhost" {
		t.Errorf("passkey config not applied: %+v", cfg.Auth)
	}
	if cfg.Workspace.GoogleClientID != "ws-g-id" || cfg.Workspace.SlackClientSecret != "s-secret" {
		t.Errorf("workspace creds not applied: %+v", cfg.Workspace)
	}
	if cfg.Workspace.SlackSigningSecret != "s-sig" {
		t.Errorf("slack signing secret not applied: %+v", cfg.Workspace)
	}

	loaded, err := config.LoadPersistedSettings()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.HasAuth || loaded.Auth.GoogleClientSecret != "g-secret" {
		t.Errorf("auth not persisted: %+v", loaded.Auth)
	}
	if !loaded.HasWorkspace || loaded.Workspace.SlackClientSecret != "s-secret" {
		t.Errorf("workspace not persisted: %+v", loaded.Workspace)
	}
}

// TestUpdateSettings_IgnoresMaskedAuthSecrets verifies the masked-sentinel
// guard prevents a view-then-save cycle from clobbering the real value
// on disk with "••••".
func TestUpdateSettings_IgnoresMaskedAuthSecrets(t *testing.T) {
	dir := t.TempDir()
	prevOverride := config.SettingsDirOverrideForTest()
	config.SetSettingsDirForTest(dir)
	t.Cleanup(func() { config.SetSettingsDirForTest(prevOverride) })

	cfg := &config.OnlineDataConfig{}
	cfg.Auth.GoogleClientSecret = "real-google-secret"
	cfg.Workspace.SlackClientSecret = "real-slack-secret"
	app := newDesktopAppForTest(cfg)

	if err := app.UpdateSettings(SettingsPayload{
		GoogleClientSecret: "real…cret",
		SlackClientSecret:  "real…cret",
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if cfg.Auth.GoogleClientSecret != "real-google-secret" {
		t.Errorf("masked submission should be ignored, got %q", cfg.Auth.GoogleClientSecret)
	}
	if cfg.Workspace.SlackClientSecret != "real-slack-secret" {
		t.Errorf("masked submission should be ignored, got %q", cfg.Workspace.SlackClientSecret)
	}
}

// TestGetSettings_MasksDeepSeekKey ensures the displayed value is masked so a
// shoulder-surfer can't read it from the UI.
func TestGetSettings_MasksDeepSeekKey(t *testing.T) {
	cfg := &config.OnlineDataConfig{}
	cfg.AI.DeepSeekAPIKey = "sk-1234567890abcdef"
	app := newDesktopAppForTest(cfg)
	got := app.GetSettings().DeepSeekAPIKey
	if got == "" || got == cfg.AI.DeepSeekAPIKey {
		t.Errorf("expected masked DeepSeekAPIKey in GetSettings, got %q", got)
	}
	if !strings.Contains(got, "…") && !strings.Contains(got, "•") {
		t.Errorf("expected mask character in %q", got)
	}
}
