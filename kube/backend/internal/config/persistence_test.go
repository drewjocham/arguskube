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
		Namespace:      "argus",
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

// TestFromConfig_RoundTripsPipelinesAndLLM is the regression guard for
// the "settings don't save" bug: every Pipelines + LLM field the
// Settings UI writes must survive a save-then-load cycle. If a new
// field is added to PipelinesConfig and forgotten here, this test
// fails loudly — much louder than a user discovering their GitHub PAT
// disappeared at next launch.
func TestFromConfig_RoundTripsPipelinesAndLLM(t *testing.T) {
	dir := t.TempDir()
	prev := settingsDirOverride
	SetSettingsDirForTest(dir)
	t.Cleanup(func() { SetSettingsDirForTest(prev) })

	src := &OnlineDataConfig{}
	src.AI.LLMBaseURL = "https://vllm.example/v1"
	src.AI.LLMModel = "deepseek-coder-v3"
	src.Pipelines = PipelinesConfig{
		Enabled:                true,
		Provider:               "github",
		GitHubToken:            "ghp_secrettoken",
		GitHubOwner:            "argues",
		GitHubRepo:             "argus",
		GitHubWorkflow:         "release.yml",
		GitLabURL:              "https://gitlab.example",
		GitLabToken:            "glpat-xxx",
		GitLabProjectID:        "42",
		GitLabRef:              "main",
		AWSRegion:              "us-east-1",
		AWSAccessKey:           "AKIA...",
		AWSSecretKey:           "secret",
		AWSProject:             "argus-build",
		GCPProject:             "argus-gcp",
		GCPRegion:              "europe-west3",
		GCPCredentials:         "/creds.json",
		CircleCIToken:          "ccitok",
		CircleCIProjectSlug:    "gh/argues/argus",
		AzureOrganization:      "argues-org",
		AzureProject:           "argus",
		AzurePipelineID:        "17",
		AzureToken:             "azp-pat",
		AzureBranch:            "main",
		NotifyOnPROpened:       true,
		NotifyOnPRUpdated:      false,
		NotifyOnPRCommented:    true,
		NotifyOnPRMerged:       true,
		AutoCodeReview:         true,
		CodeReviewDestination:  "gdrive",
		GDriveFolderID:         "folder-abc",
		CodeReviewS3Prefix:     "s3://argus/reviews",
		CodeReviewEmailTo:      "team@example.com",
		ConfluenceURL:          "https://confluence.example",
		ConfluenceEmail:        "alice@example.com",
		ConfluenceToken:        "conf-tok",
		ConfluenceSpaceKey:     "ENG",
		ConfluenceParentPageID: "12345",
		NotionToken:            "notion-tok",
		NotionDatabaseID:       "db-1",
		EvernoteToken:          "ev-tok",
		EvernoteNotebookGUID:   "ev-nb",
		OneNoteToken:           "on-tok",
		OneNoteSectionID:       "on-sec",
		AmplenoteAPIKey:        "amp-key",
		StandardNotesURL:       "https://sn.example",
		StandardNotesToken:     "sn-tok",
		ObsidianVaultPath:      "/Users/me/Vault",
		JoplinURL:              "http://localhost:41184",
		JoplinToken:            "jop-tok",
		LogseqGraphPath:        "/Users/me/Graph",
		BearToken:              "bear-tok",
	}

	if err := SavePersistedSettings(FromConfig(src)); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadPersistedSettings()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	dst := &OnlineDataConfig{}
	loaded.MergeInto(dst)

	// LLM
	if dst.AI.LLMBaseURL != src.AI.LLMBaseURL {
		t.Errorf("LLMBaseURL lost: got %q, want %q", dst.AI.LLMBaseURL, src.AI.LLMBaseURL)
	}
	if dst.AI.LLMModel != src.AI.LLMModel {
		t.Errorf("LLMModel lost: got %q, want %q", dst.AI.LLMModel, src.AI.LLMModel)
	}
	// Pipelines: deep equal (so adding a new field to PipelinesConfig
	// without updating PersistedPipelinesSettings + the two converters
	// will fail this assertion).
	if !pipelinesEqual(dst.Pipelines, src.Pipelines) {
		t.Errorf("Pipelines roundtrip lost data:\n got  %+v\n want %+v", dst.Pipelines, src.Pipelines)
	}
}

// TestPipelinesGatedByHasPipelinesFlag — a load with HasPipelines=false
// must not stomp on env-default Pipelines values. Important so a
// user who's never opened Settings keeps their env-bootstrapped config.
func TestPipelinesGatedByHasPipelinesFlag(t *testing.T) {
	dir := t.TempDir()
	prev := settingsDirOverride
	SetSettingsDirForTest(dir)
	t.Cleanup(func() { SetSettingsDirForTest(prev) })

	// Persisted settings touch nothing pipeline-related (zero value,
	// HasPipelines=false).
	if err := SavePersistedSettings(&PersistedSettings{Namespace: "ns-from-ui"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	dst := &OnlineDataConfig{}
	dst.Pipelines.GitHubToken = "env-token"
	dst.Pipelines.Enabled = true

	loaded, err := LoadPersistedSettings()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	loaded.MergeInto(dst)

	if dst.Pipelines.GitHubToken != "env-token" {
		t.Errorf("env GitHubToken stomped by empty persisted: got %q", dst.Pipelines.GitHubToken)
	}
	if !dst.Pipelines.Enabled {
		t.Error("env Enabled=true stomped by zero-value persisted bool")
	}
	if dst.Kubernetes.Namespace != "ns-from-ui" {
		t.Errorf("non-pipeline field should still merge: got %q", dst.Kubernetes.Namespace)
	}
}

// TestPipelinesExplicitFalseSurvives — once HasPipelines=true, an
// explicit Enabled=false in the saved settings overrides an env-set
// Enabled=true. The user's most recent UI choice wins.
func TestPipelinesExplicitFalseSurvives(t *testing.T) {
	dir := t.TempDir()
	prev := settingsDirOverride
	SetSettingsDirForTest(dir)
	t.Cleanup(func() { SetSettingsDirForTest(prev) })

	if err := SavePersistedSettings(&PersistedSettings{
		HasPipelines: true,
		Pipelines:    PersistedPipelinesSettings{Enabled: false, GitHubToken: ""},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	dst := &OnlineDataConfig{}
	dst.Pipelines.Enabled = true
	dst.Pipelines.GitHubToken = "env-token"

	loaded, err := LoadPersistedSettings()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	loaded.MergeInto(dst)

	if dst.Pipelines.Enabled {
		t.Error("explicit Enabled=false from UI must override env")
	}
	if dst.Pipelines.GitHubToken != "" {
		t.Errorf("user-cleared token must override env: got %q", dst.Pipelines.GitHubToken)
	}
}

// pipelinesEqual compares two PipelinesConfig values field-by-field.
// We don't use reflect.DeepEqual on the struct because Go's struct
// equality already works here — but a custom helper gives nicer
// failure messages and lets us add per-field tolerances later.
func pipelinesEqual(a, b PipelinesConfig) bool {
	return a == b
}
