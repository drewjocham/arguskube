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
