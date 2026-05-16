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

func newDesktopAppForTest(cfg *config.OnlineDataConfig) *App {
	logger := slog.New(slog.DiscardHandler)
	return &App{
		ctx:    context.Background(),
		logger: logger,
		cfg:    cfg,
		gate:   features.NewGate(cfg),
	}
}

func settingsForTest(cfg *config.OnlineDataConfig) *SettingsHandler {
	return NewSettingsHandler(newDesktopAppForTest(cfg))
}

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

func TestUpdateSettings_PersistsToDiskAndAcrossLoad(t *testing.T) {
	dir := t.TempDir()
	prevOverride := config.SettingsDirOverrideForTest()
	config.SetSettingsDirForTest(dir)
	t.Cleanup(func() { config.SetSettingsDirForTest(prevOverride) })

	cfg := &config.OnlineDataConfig{}
	sh := settingsForTest(cfg)

	if _, err := sh.UpdateSettings(SettingsPayload{
		DeepSeekAPIKey: "sk-from-ui",
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
}

func TestUpdateSettings_BuildsAgentWhenNoneExisted(t *testing.T) {
	dir := t.TempDir()
	prevOverride := config.SettingsDirOverrideForTest()
	config.SetSettingsDirForTest(dir)
	t.Cleanup(func() { config.SetSettingsDirForTest(prevOverride) })

	cfg := &config.OnlineDataConfig{}
	app := newDesktopAppForTest(cfg)
	sh := NewSettingsHandler(app)

	if app.agent != nil {
		t.Fatal("precondition: expected app.agent == nil before update")
	}

	if _, err := sh.UpdateSettings(SettingsPayload{DeepSeekAPIKey: "sk-key"}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if app.agent == nil {
		t.Fatal("expected app.agent to be constructed after pasting an API key")
	}
}

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
	sh := NewSettingsHandler(app)

	if _, err := sh.UpdateSettings(SettingsPayload{DeepSeekAPIKey: "sk-new"}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if app.agent != originalAgent {
		t.Error("expected agent pointer preserved (history retained)")
	}
}

func TestUpdateSettings_IgnoresSentinelKey(t *testing.T) {
	dir := t.TempDir()
	prevOverride := config.SettingsDirOverrideForTest()
	config.SetSettingsDirForTest(dir)
	t.Cleanup(func() { config.SetSettingsDirForTest(prevOverride) })

	cfg := &config.OnlineDataConfig{}
	cfg.AI.DeepSeekAPIKey = "sk-real-key"
	sh := settingsForTest(cfg)

	if _, err := sh.UpdateSettings(SettingsPayload{DeepSeekAPIKey: SentinelUnchanged}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if cfg.AI.DeepSeekAPIKey != "sk-real-key" {
		t.Errorf("sentinel should be ignored, but key is now %q", cfg.AI.DeepSeekAPIKey)
	}
}

func TestGetSettings_MasksDeepSeekKey(t *testing.T) {
	cfg := &config.OnlineDataConfig{}
	cfg.AI.DeepSeekAPIKey = "sk-1234567890abcdef"
	app := newDesktopAppForTest(cfg)
	got := app.GetSettings().DeepSeekAPIKey
	if got == "" || got == cfg.AI.DeepSeekAPIKey {
		t.Errorf("expected masked DeepSeekAPIKey in GetSettings, got %q", got)
	}
	if !strings.Contains(got, "•") {
		t.Errorf("expected mask character in %q", got)
	}
}
