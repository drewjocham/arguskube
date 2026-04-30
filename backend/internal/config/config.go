package config

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"
)

// Tier represents the subscription tier for feature gating.
type Tier string

const (
	TierFree Tier = "free"
	TierPro  Tier = "pro"
)

// OnlineDataConfig is the top-level configuration loaded once at startup.
type OnlineDataConfig struct {
	Kubernetes  KubernetesConfig
	AI          AIConfig
	Features    FeaturesConfig
	Server      ServerConfig
	Logging     LoggingConfig
	DecisionLog DecisionLogConfig
	S3          S3Config
}

// KubernetesConfig holds cluster connection settings.
type KubernetesConfig struct {
	Context   string `env:"KUBEWATCHER_CONTEXT"`
	Config    string `env:"KUBEWATCHER_KUBECONFIG"` // explicit override; empty = client-go default rules
	Namespace string `env:"KUBEWATCHER_NAMESPACE"`
	InCluster bool   `env:"KUBEWATCHER_IN_CLUSTER"`
}

// AIConfig holds settings for AI diagnostics and anomaly detection.
type AIConfig struct {
	DeepSeekAPIKey  string `env:"DEEPSEEK_API_KEY"`
	AnomstackURL    string `env:"ANOMSTACK_URL"`
	AnomstackAPIKey string `env:"ANOMSTACK_API_KEY"`
	VertexProject   string `env:"VERTEX_PROJECT"`
	VertexLocation  string `env:"VERTEX_LOCATION"`
	PrometheusURL   string `env:"PROMETHEUS_URL"` // optional: enhances metrics if available
	PopeyeBinary    string `env:"KUBEWATCHER_POPEYE_BIN"`
	ContextTokenMax int
	ContextTimeout  time.Duration
}

// FeaturesConfig holds tier and license info.
type FeaturesConfig struct {
	Tier       Tier   `env:"KUBEWATCHER_TIER"`
	LicenseKey string `env:"KUBEWATCHER_LICENSE"`
}

// ServerConfig holds HTTP/API server settings.
type ServerConfig struct {
	Port         string `env:"KUBEWATCHER_PORT"`
	MetricsPort  string `env:"KUBEWATCHER_METRICS_PORT"`
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// LoggingConfig holds structured logging settings.
type LoggingConfig struct {
	Level  string `env:"KUBEWATCHER_LOG_LEVEL"`
	Format string `env:"KUBEWATCHER_LOG_FORMAT"` // "json" or "text"
}

// DecisionLogConfig holds the path and parsing settings for DECISION_LOG.md.
type DecisionLogConfig struct {
	Path string `env:"KUBEWATCHER_DECISION_LOG"`
}

// S3Config holds S3 bucket settings for notebook storage.
type S3Config struct {
	Bucket    string `env:"KUBEWATCHER_S3_BUCKET"`
	Region    string `env:"KUBEWATCHER_S3_REGION"`
	Endpoint  string `env:"KUBEWATCHER_S3_ENDPOINT"`
	AccessKey string `env:"KUBEWATCHER_S3_ACCESS_KEY"`
	SecretKey string `env:"KUBEWATCHER_S3_SECRET_KEY"`
}

func New() (*OnlineDataConfig, error) {
	cfg := &OnlineDataConfig{
		Kubernetes: KubernetesConfig{
			Context:   env("KUBEWATCHER_CONTEXT", "default"),
			Config:    env("KUBEWATCHER_KUBECONFIG", homeDir()+"/.kube/k3s-config"),
			Namespace: env("KUBEWATCHER_NAMESPACE", ""),
			InCluster: env("KUBEWATCHER_IN_CLUSTER", "false") == "true",
		},
		AI: AIConfig{
			DeepSeekAPIKey:  env("DEEPSEEK_API_KEY", ""),
			AnomstackURL:    env("ANOMSTACK_URL", "http://localhost:8087"),
			AnomstackAPIKey: env("ANOMSTACK_API_KEY", ""),
			VertexProject:   env("VERTEX_PROJECT", ""),
			VertexLocation:  env("VERTEX_LOCATION", "europe-west3"),
			PrometheusURL:   env("PROMETHEUS_URL", ""), // auto-detected if empty
			PopeyeBinary:    env("KUBEWATCHER_POPEYE_BIN", "popeye"),
			ContextTokenMax: 8000,
			ContextTimeout:  3 * time.Second,
		},
		Features: FeaturesConfig{
			Tier:       parseTier(env("KUBEWATCHER_TIER", "pro")), // pro for dev; production gates via license
			LicenseKey: env("KUBEWATCHER_LICENSE", ""),
		},
		Server: ServerConfig{
			Port:         env("KUBEWATCHER_PORT", "8080"),
			MetricsPort:  env("KUBEWATCHER_METRICS_PORT", "9090"),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  env("KUBEWATCHER_LOG_LEVEL", "info"),
			Format: env("KUBEWATCHER_LOG_FORMAT", "text"),
		},
		DecisionLog: DecisionLogConfig{
			Path: env("KUBEWATCHER_DECISION_LOG", "DECISION_LOG.md"),
		},
		S3: S3Config{
			Bucket:    env("KUBEWATCHER_S3_BUCKET", ""),
			Region:    env("KUBEWATCHER_S3_REGION", "us-east-1"),
			Endpoint:  env("KUBEWATCHER_S3_ENDPOINT", ""),
			AccessKey: env("KUBEWATCHER_S3_ACCESS_KEY", ""),
			SecretKey: env("KUBEWATCHER_S3_SECRET_KEY", ""),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

func (c *OnlineDataConfig) validate() error {
	// Minimal validation — extend as needed.
	if c.Kubernetes.InCluster && c.Kubernetes.Config != "" {
		return fmt.Errorf("cannot set both KUBECONFIG and in-cluster mode")
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseTier(s string) Tier {
	switch strings.ToLower(s) {
	case "pro":
		return TierPro
	default:
		return TierFree
	}
}

func homeDir() string {
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "."
}
