package config

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	// Clear any existing env vars that might interfere.
	clearEnv(t)
	defer clearEnv(t)

	cfg, err := New()
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("New() returned nil")
	}

	t.Run("kubernetes defaults", func(t *testing.T) {
		if cfg.Kubernetes.Context != "" {
			t.Errorf("default Context = %q, want empty", cfg.Kubernetes.Context)
		}
		if cfg.Kubernetes.InCluster {
			t.Error("default InCluster should be false")
		}
	})

	t.Run("AI defaults", func(t *testing.T) {
		if cfg.AI.DeepSeekAPIKey != "" {
			t.Errorf("default DeepSeekAPIKey = %q, want empty", cfg.AI.DeepSeekAPIKey)
		}
		if cfg.AI.AnomstackURL != "http://localhost:8087" {
			t.Errorf("default AnomstackURL = %q, want %q", cfg.AI.AnomstackURL, "http://localhost:8087")
		}
		if cfg.AI.VertexLocation != "europe-west3" {
			t.Errorf("default VertexLocation = %q, want %q", cfg.AI.VertexLocation, "europe-west3")
		}
		if cfg.AI.PopeyeBinary != "popeye" {
			t.Errorf("default PopeyeBinary = %q, want %q", cfg.AI.PopeyeBinary, "popeye")
		}
		if cfg.AI.ContextTokenMax != 8000 {
			t.Errorf("default ContextTokenMax = %d, want 8000", cfg.AI.ContextTokenMax)
		}
		if cfg.AI.ContextTimeout != 3*time.Second {
			t.Errorf("default ContextTimeout = %v, want 3s", cfg.AI.ContextTimeout)
		}
	})

	t.Run("ArgoCD defaults", func(t *testing.T) {
		if cfg.ArgoCD.URL != "" {
			t.Errorf("default ArgoCD URL = %q, want empty", cfg.ArgoCD.URL)
		}
		if cfg.ArgoCD.Insecure {
			t.Error("default ArgoCD Insecure should be false")
		}
	})

	t.Run("security defaults", func(t *testing.T) {
		if cfg.Security.SnykToken != "" {
			t.Errorf("default SnykToken = %q, want empty", cfg.Security.SnykToken)
		}
		if cfg.Security.TrivyBinary != "trivy" {
			t.Errorf("default TrivyBinary = %q, want %q", cfg.Security.TrivyBinary, "trivy")
		}
	})

	t.Run("features defaults", func(t *testing.T) {
		if cfg.Features.Tier != TierPro {
			t.Errorf("default Tier = %q, want %q", cfg.Features.Tier, TierPro)
		}
		if cfg.Features.LicenseKey != "" {
			t.Errorf("default LicenseKey = %q, want empty", cfg.Features.LicenseKey)
		}
	})

	t.Run("server defaults", func(t *testing.T) {
		if cfg.Server.Port != "8080" {
			t.Errorf("default Port = %q, want %q", cfg.Server.Port, "8080")
		}
		if cfg.Server.MetricsPort != "9090" {
			t.Errorf("default MetricsPort = %q, want %q", cfg.Server.MetricsPort, "9090")
		}
		if cfg.Server.ReadTimeout != 15*time.Second {
			t.Errorf("default ReadTimeout = %v, want 15s", cfg.Server.ReadTimeout)
		}
		if cfg.Server.WriteTimeout != 15*time.Second {
			t.Errorf("default WriteTimeout = %v, want 15s", cfg.Server.WriteTimeout)
		}
	})

	t.Run("logging defaults", func(t *testing.T) {
		if cfg.Logging.Level != "info" {
			t.Errorf("default LogLevel = %q, want %q", cfg.Logging.Level, "info")
		}
		if cfg.Logging.Format != "text" {
			t.Errorf("default LogFormat = %q, want %q", cfg.Logging.Format, "text")
		}
	})

	t.Run("decision log defaults", func(t *testing.T) {
		if cfg.DecisionLog.Path != "DECISION_LOG.md" {
			t.Errorf("default DecisionLog Path = %q, want %q", cfg.DecisionLog.Path, "DECISION_LOG.md")
		}
	})

	t.Run("S3 defaults", func(t *testing.T) {
		if cfg.S3.Bucket != "" {
			t.Errorf("default S3 Bucket = %q, want empty", cfg.S3.Bucket)
		}
		if cfg.S3.Region != "us-east-1" {
			t.Errorf("default S3 Region = %q, want %q", cfg.S3.Region, "us-east-1")
		}
		if cfg.S3.Endpoint != "" {
			t.Errorf("default S3 Endpoint = %q, want empty", cfg.S3.Endpoint)
		}
	})
}

func TestEnvOverrides(t *testing.T) {
	clearEnv(t)
	defer clearEnv(t)

	os.Setenv("argus_PORT", "3000")
	os.Setenv("argus_LOG_LEVEL", "debug")
	os.Setenv("argus_LOG_FORMAT", "json")
	os.Setenv("argus_TIER", "free")
	os.Setenv("DEEPSEEK_API_KEY", "sk-test-key")
	os.Setenv("argus_CONTEXT", "my-cluster")
	os.Setenv("argus_IN_CLUSTER", "true")
	os.Setenv("argus_S3_BUCKET", "my-bucket")
	os.Setenv("ARGOCD_INSECURE", "true")
	os.Setenv("argus_TRIVY_BIN", "/usr/local/bin/trivy")

	cfg, err := New()
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}

	tests := []struct {
		name     string
		got      interface{}
		want     interface{}
	}{
		{"port", cfg.Server.Port, "3000"},
		{"log level", cfg.Logging.Level, "debug"},
		{"log format", cfg.Logging.Format, "json"},
		{"tier", string(cfg.Features.Tier), "free"},
		{"deepseek key", cfg.AI.DeepSeekAPIKey, "sk-test-key"},
		{"k8s context", cfg.Kubernetes.Context, "my-cluster"},
		{"in cluster", cfg.Kubernetes.InCluster, true},
		{"s3 bucket", cfg.S3.Bucket, "my-bucket"},
		{"argocd insecure", cfg.ArgoCD.Insecure, true},
		{"trivy binary", cfg.Security.TrivyBinary, "/usr/local/bin/trivy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "in cluster", "argocd insecure":
				if got, ok := tt.got.(bool); ok {
					if want, ok2 := tt.want.(bool); ok2 && got != want {
						t.Errorf("got %v, want %v", got, want)
					}
				}
			default:
				if got, ok := tt.got.(string); ok {
					if want, ok2 := tt.want.(string); ok2 && got != want {
						t.Errorf("got %q, want %q", got, want)
					}
				}
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	t.Run("rejects both kubeconfig and in-cluster", func(t *testing.T) {
		clearEnv(t)
		defer clearEnv(t)

		os.Setenv("argus_KUBECONFIG", "/tmp/kubeconfig")
		os.Setenv("argus_IN_CLUSTER", "true")

		_, err := New()
		if err == nil {
			t.Fatal("expected error for both KUBECONFIG and IN_CLUSTER set simultaneously")
		}
	})

	t.Run("allows kubeconfig without in-cluster", func(t *testing.T) {
		clearEnv(t)
		defer clearEnv(t)

		os.Setenv("argus_KUBECONFIG", "/tmp/kubeconfig")

		cfg, err := New()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Kubernetes.Config != "/tmp/kubeconfig" {
			t.Errorf("Config = %q, want %q", cfg.Kubernetes.Config, "/tmp/kubeconfig")
		}
	})

	t.Run("allows in-cluster without kubeconfig", func(t *testing.T) {
		clearEnv(t)
		defer clearEnv(t)

		os.Setenv("argus_IN_CLUSTER", "true")

		cfg, err := New()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.Kubernetes.InCluster {
			t.Error("expected InCluster to be true")
		}
	})
}

func TestParseTier(t *testing.T) {
	tests := []struct {
		input string
		want  Tier
	}{
		{"pro", TierPro},
		{"Pro", TierPro},
		{"PRO", TierPro},
		{"free", TierFree},
		{"Free", TierFree},
		{"FREE", TierFree},
		{"", TierFree},
		{"invalid", TierFree},
		{"enterprise", TierFree},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseTier(tt.input)
			if got != tt.want {
				t.Errorf("parseTier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEnvFunction(t *testing.T) {
	key := "argus_TEST_ENV_VAR"
	// Should return fallback when env var is not set.
	os.Unsetenv(key)
	if got := env(key, "fallback"); got != "fallback" {
		t.Errorf("env(%q, \"fallback\") = %q, want %q", key, got, "fallback")
	}

	// Should return value when env var is set.
	os.Setenv(key, "custom")
	if got := env(key, "fallback"); got != "custom" {
		t.Errorf("env(%q, \"fallback\") = %q, want %q", key, got, "custom")
	}
	os.Unsetenv(key)
}

// clearEnv removes all argus_* and related env vars that could affect config.
func clearEnv(t testing.TB) {
	t.Helper()
	vars := []string{
		"argus_CONTEXT", "argus_KUBECONFIG", "KUBECONFIG",
		"argus_NAMESPACE", "argus_IN_CLUSTER",
		"DEEPSEEK_API_KEY", "ANOMSTACK_URL", "ANOMSTACK_API_KEY",
		"VERTEX_PROJECT", "VERTEX_LOCATION", "PROMETHEUS_URL",
		"argus_POPEYE_BIN",
		"ARGOCD_URL", "ARGOCD_TOKEN", "ARGOCD_INSECURE",
		"SNYK_TOKEN", "argus_TRIVY_BIN", "argus_FALCO_URL",
		"argus_TIER", "argus_LICENSE",
		"argus_PORT", "argus_METRICS_PORT",
		"argus_LOG_LEVEL", "argus_LOG_FORMAT",
		"argus_DECISION_LOG",
		"argus_S3_BUCKET", "argus_S3_REGION",
		"argus_S3_ENDPOINT", "argus_S3_ACCESS_KEY", "argus_S3_SECRET_KEY",
	}
	for _, v := range vars {
		os.Unsetenv(v)
	}
}

// TestMain isolates the persisted-settings file under a temp dir so that the
// developer's real ~/.config/argus/settings.json (or macOS equivalent)
// cannot leak into config tests.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "argus-config-tests-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: failed to create temp dir:", err)
		os.Exit(1)
	}
	SetSettingsDirForTest(tmp)
	code := m.Run()
	SetSettingsDirForTest("")
	os.RemoveAll(tmp)
	os.Exit(code)
}
