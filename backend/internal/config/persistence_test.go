package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPersistedSettings_MissingFile(t *testing.T) {
	dir := t.TempDir()
	prev := settingsDirOverride
	SetSettingsDirForTest(dir)
	t.Cleanup(func() { SetSettingsDirForTest(prev) })

	got, err := LoadPersistedSettings()
	if err != nil {
		t.Fatalf("LoadPersistedSettings on missing file: unexpected err: %v", err)
	}
	if got == nil {
		t.Fatal("expected zero-value PersistedSettings, got nil")
	}
	if got.DeepSeekAPIKey != "" || got.ArgoCDURL != "" {
		t.Errorf("expected zero value, got %+v", got)
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	prev := settingsDirOverride
	SetSettingsDirForTest(dir)
	t.Cleanup(func() { SetSettingsDirForTest(prev) })

	want := &PersistedSettings{
		DeepSeekAPIKey: "sk-test-secret",
		ArgoCDURL:      "https://argocd.example.com",
		ArgoCDToken:    "argo-token",
		ArgoCDInsecure: true,
		Namespace:      "kubewatcher",
		LogLevel:       "debug",
	}
	if err := SavePersistedSettings(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// File should exist with 0o600 perms.
	path := filepath.Join(dir, "settings.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat persisted file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("expected file mode 0600 (so tokens are not world-readable), got %o", mode)
	}

	got, err := LoadPersistedSettings()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.DeepSeekAPIKey != want.DeepSeekAPIKey ||
		got.ArgoCDURL != want.ArgoCDURL ||
		got.ArgoCDToken != want.ArgoCDToken ||
		got.ArgoCDInsecure != want.ArgoCDInsecure ||
		got.Namespace != want.Namespace ||
		got.LogLevel != want.LogLevel {
		t.Errorf("roundtrip mismatch.\nwant: %+v\ngot : %+v", want, got)
	}
}

func TestLoadPersistedSettings_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	prev := settingsDirOverride
	SetSettingsDirForTest(dir)
	t.Cleanup(func() { SetSettingsDirForTest(prev) })

	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte("not-json"), 0o600); err != nil {
		t.Fatalf("seed malformed file: %v", err)
	}
	_, err := LoadPersistedSettings()
	if err == nil {
		t.Fatal("expected error for malformed settings file, got nil")
	}
}

func TestMergeInto_OverridesNonEmptyFieldsOnly(t *testing.T) {
	cfg := &OnlineDataConfig{}
	cfg.AI.DeepSeekAPIKey = "from-env"
	cfg.AI.AnomstackURL = "http://from-env"
	cfg.ArgoCD.URL = "from-env-argo"

	persisted := &PersistedSettings{
		DeepSeekAPIKey: "from-ui",
		AnomstackURL:   "", // empty: should leave the env value intact
		ArgoCDURL:      "from-ui-argo",
	}
	persisted.MergeInto(cfg)

	if cfg.AI.DeepSeekAPIKey != "from-ui" {
		t.Errorf("expected DeepSeekAPIKey 'from-ui' (UI overrides env), got %q", cfg.AI.DeepSeekAPIKey)
	}
	if cfg.AI.AnomstackURL != "http://from-env" {
		t.Errorf("expected env AnomstackURL preserved, got %q", cfg.AI.AnomstackURL)
	}
	if cfg.ArgoCD.URL != "from-ui-argo" {
		t.Errorf("expected ArgoCD URL 'from-ui-argo', got %q", cfg.ArgoCD.URL)
	}
}

func TestFromConfig_RoundTripsThroughPersistence(t *testing.T) {
	dir := t.TempDir()
	prev := settingsDirOverride
	SetSettingsDirForTest(dir)
	t.Cleanup(func() { SetSettingsDirForTest(prev) })

	src := &OnlineDataConfig{}
	src.AI.DeepSeekAPIKey = "sk-abc"
	src.ArgoCD.URL = "https://argo"
	src.Kubernetes.Namespace = "ns1"

	if err := SavePersistedSettings(FromConfig(src)); err != nil {
		t.Fatalf("save: %v", err)
	}

	dst := &OnlineDataConfig{}
	loaded, err := LoadPersistedSettings()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	loaded.MergeInto(dst)

	if dst.AI.DeepSeekAPIKey != "sk-abc" || dst.ArgoCD.URL != "https://argo" || dst.Kubernetes.Namespace != "ns1" {
		t.Errorf("roundtrip lost data: %+v", dst)
	}
}

func TestSavePersistedSettings_NilReturnsError(t *testing.T) {
	dir := t.TempDir()
	prev := settingsDirOverride
	SetSettingsDirForTest(dir)
	t.Cleanup(func() { SetSettingsDirForTest(prev) })

	if err := SavePersistedSettings(nil); err == nil {
		t.Fatal("expected error for nil settings")
	}
}
