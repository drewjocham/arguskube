package pkg

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/argues/argus/internal/config"
)

// minimalLauncherApp constructs an App carrying only the bits the
// launcher inspects (config + logger). No K8s client, no auth, no
// hub.
func minimalLauncherApp(t *testing.T, cfg *config.OnlineDataConfig) *App {
	t.Helper()
	return &App{
		logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		cfg:       cfg,
		webhookMu: sync.RWMutex{},
	}
}

// ─── mergeEnv ────────────────────────────────────────────────────────

func TestMergeEnvOverlays(t *testing.T) {
	t.Parallel()
	parent := []string{
		"PATH=/usr/bin",
		"HOME=/Users/argus",
		"KUBECONFIG=/old/kube",
	}
	overlays := map[string]string{
		"KUBECONFIG":        "/new/kube",
		"ARGUS_K8S_CONTEXT": "prod",
	}
	got := mergeEnv(parent, overlays)
	sort.Strings(got)

	want := []string{
		"ARGUS_K8S_CONTEXT=prod",
		"HOME=/Users/argus",
		"KUBECONFIG=/new/kube",
		"PATH=/usr/bin",
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("mergeEnv len = %d, want %d:\n%v\nwant:\n%v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("mergeEnv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMergeEnvPreservesParentWhenNoOverride(t *testing.T) {
	t.Parallel()
	parent := []string{"PATH=/usr/bin", "HOME=/Users/argus"}
	got := mergeEnv(parent, nil)
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d", len(got))
	}
}

func TestMergeEnvHandlesMalformedEntries(t *testing.T) {
	t.Parallel()
	// Some launchers stuff valueless entries into os.Environ. We
	// don't try to be clever: preserve them as-is.
	parent := []string{"BAD_NO_EQUALS", "GOOD=value"}
	got := mergeEnv(parent, map[string]string{"GOOD": "overridden"})
	if !containsLine(got, "BAD_NO_EQUALS") {
		t.Error("malformed parent entry should be preserved")
	}
	if !containsLine(got, "GOOD=overridden") {
		t.Errorf("override missing; got %v", got)
	}
}

func TestEnvKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"FOO=bar", "FOO"},
		{"FOO=", "FOO"},
		{"FOO", "FOO"},
		{"=value", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := envKey(tc.in); got != tc.want {
				t.Errorf("envKey(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// ─── buildPopOutEnv ──────────────────────────────────────────────────

func TestBuildPopOutEnvAllFieldsFlowThrough(t *testing.T) {
	t.Parallel()
	cfg := &config.OnlineDataConfig{}
	cfg.Kubernetes.Config = "/etc/kube.yaml"
	cfg.Kubernetes.Context = "prod-us"
	cfg.Kubernetes.Namespace = "monitoring"
	cfg.AI.DeepSeekAPIKey = "sk-test"
	cfg.AI.LLMBaseURL = "http://vllm:8000/v1"
	cfg.AI.LLMModel = "deepseek-chat"

	a := minimalLauncherApp(t, cfg)
	got := a.buildPopOutEnv([]string{"PATH=/usr/bin"})

	checks := map[string]string{
		"KUBECONFIG":           "/etc/kube.yaml",
		"KUBECTX":              "prod-us",
		"ARGUS_K8S_CONTEXT":    "prod-us",
		"ARGUS_K8S_NAMESPACE":  "monitoring",
		"ARGUS_TERMINAL_TITLE": "Argus Terminal",
		"DEEPSEEK_API_KEY":     "sk-test",
		"ARGUS_LLM_BASE_URL":   "http://vllm:8000/v1",
		"ARGUS_LLM_MODEL":      "deepseek-chat",
	}
	for k, want := range checks {
		if !containsLine(got, k+"="+want) {
			t.Errorf("expected %s=%s in env; got:\n%s", k, want, strings.Join(got, "\n"))
		}
	}
	if !containsLine(got, "PATH=/usr/bin") {
		t.Error("parent PATH not preserved")
	}
}

func TestBuildPopOutEnvOmitsBlankFields(t *testing.T) {
	t.Parallel()
	// Empty config: only the always-set ARGUS_TERMINAL_TITLE shows up;
	// no blank KUBECONFIG= / KUBECTX= etc.
	cfg := &config.OnlineDataConfig{}
	a := minimalLauncherApp(t, cfg)
	got := a.buildPopOutEnv(nil)

	if !containsLine(got, "ARGUS_TERMINAL_TITLE=Argus Terminal") {
		t.Error("expected title overlay")
	}
	for _, key := range []string{
		"KUBECONFIG=", "KUBECTX=", "ARGUS_K8S_CONTEXT=",
		"ARGUS_K8S_NAMESPACE=", "DEEPSEEK_API_KEY=",
		"ARGUS_LLM_BASE_URL=", "ARGUS_LLM_MODEL=",
	} {
		for _, kv := range got {
			if strings.HasPrefix(kv, key) {
				t.Errorf("blank field leaked into env: %q", kv)
			}
		}
	}
}

func TestBuildPopOutEnvNilConfigDoesNotPanic(t *testing.T) {
	t.Parallel()
	a := minimalLauncherApp(t, nil)
	got := a.buildPopOutEnv([]string{"HOME=/Users/argus"})
	if !containsLine(got, "ARGUS_TERMINAL_TITLE=Argus Terminal") {
		t.Error("title overlay should land even with nil config")
	}
	if !containsLine(got, "HOME=/Users/argus") {
		t.Error("parent env should pass through with nil config")
	}
}

// ─── resolveLufisBinary ──────────────────────────────────────────────

func TestResolveLufisBinaryHonorsARGUS_LUFIS_PATH(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "fake-lufis")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARGUS_LUFIS_PATH", binPath)

	got, err := resolveLufisBinary()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != binPath {
		t.Errorf("resolveLufisBinary = %q, want %q", got, binPath)
	}
}

func TestResolveLufisBinaryRejectsMissingOverride(t *testing.T) {
	t.Setenv("ARGUS_LUFIS_PATH", "/does/not/exist/lufis")
	if _, err := resolveLufisBinary(); err == nil {
		t.Fatal("expected error for nonexistent ARGUS_LUFIS_PATH")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────

func containsLine(env []string, want string) bool {
	for _, kv := range env {
		if kv == want {
			return true
		}
	}
	return false
}
