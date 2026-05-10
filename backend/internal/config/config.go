package config

import (
	"fmt"
	"os"
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
	ArgoCD      ArgoCDConfig
	Security    SecurityConfig
	Features    FeaturesConfig
	Server      ServerConfig
	Logging     LoggingConfig
	DecisionLog DecisionLogConfig
	S3          S3Config
	Auth        AuthConfig
}

// AuthConfig holds sign-in settings. By default, every /api endpoint
// requires a valid session — register an account or sign in via OAuth.
// For local-development convenience there's a single escape hatch
// (DevMode) that disables the gate entirely; see field comment for the
// safety constraint.
type AuthConfig struct {
	// PublicBaseURL is the URL the OAuth callback will land on. For
	// the desktop app this is http://127.0.0.1:<port>; for a hosted
	// SaaS deployment, the public hostname.
	PublicBaseURL string `env:"KUBEWATCHER_AUTH_BASE_URL"`

	GoogleClientID     string `env:"KUBEWATCHER_GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"KUBEWATCHER_GOOGLE_CLIENT_SECRET"`

	OIDCIssuer       string `env:"KUBEWATCHER_OIDC_ISSUER"` // e.g. https://acme.okta.com
	OIDCClientID     string `env:"KUBEWATCHER_OIDC_CLIENT_ID"`
	OIDCClientSecret string `env:"KUBEWATCHER_OIDC_CLIENT_SECRET"`
	OIDCDisplayName  string `env:"KUBEWATCHER_OIDC_DISPLAY_NAME"` // e.g. "Acme SSO"

	// AllowLocalSignup controls whether the email/password registration
	// endpoint is open. Default is true for the desktop app; SaaS
	// admins typically flip it off and rely on SSO + invites.
	AllowLocalSignup bool `env:"KUBEWATCHER_AUTH_ALLOW_SIGNUP"`

	// DevMode disables the entire auth gate. Intended ONLY for local
	// development — every /api request is treated as authenticated and
	// the frontend skips the LoginView. SetupAuth refuses to honor this
	// flag when the API binds to anything other than loopback, so a
	// misconfigured public deploy can't accidentally leave auth off.
	DevMode bool `env:"KUBEWATCHER_AUTH_DISABLED"`
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
	DeepSeekAPIKey    string `env:"DEEPSEEK_API_KEY"`
	AnomstackURL      string `env:"ANOMSTACK_URL"`
	AnomstackAPIKey   string `env:"ANOMSTACK_API_KEY"`
	MCPServersConfig  string `env:"MCP_SERVERS_CONFIG"`
	AgentInstructions string `env:"AGENT_INSTRUCTIONS"`
	FlinkURL          string `env:"KUBEWATCHER_FLINK_URL"`
	FlinkAPIKey       string `env:"KUBEWATCHER_FLINK_API_KEY"`
	VertexProject     string `env:"VERTEX_PROJECT"`
	VertexLocation    string `env:"VERTEX_LOCATION"`
	PrometheusURL     string `env:"PROMETHEUS_URL"` // optional: enhances metrics if available
	PopeyeBinary      string `env:"KUBEWATCHER_POPEYE_BIN"`
	ContextTokenMax   int
	ContextTimeout    time.Duration
}

// ArgoCDConfig holds connection settings for an Argo CD server.
type ArgoCDConfig struct {
	URL      string `env:"ARGOCD_URL"`      // e.g. https://argocd.example.com
	Token    string `env:"ARGOCD_TOKEN"`    // API bearer token
	Insecure bool   `env:"ARGOCD_INSECURE"` // skip TLS verification
}

// SecurityConfig holds optional paths and tokens for security scanning tools.
// All fields are optional — features degrade gracefully when not configured.
type SecurityConfig struct {
	SnykToken   string `env:"SNYK_TOKEN"`            // API token for Snyk CLI
	TrivyBinary string `env:"KUBEWATCHER_TRIVY_BIN"` // path to trivy binary (default: "trivy")
	FalcoURL    string `env:"KUBEWATCHER_FALCO_URL"` // Falco gRPC/HTTP endpoint
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
			Context:   env("KUBEWATCHER_CONTEXT", ""),
			Config:    env("KUBEWATCHER_KUBECONFIG", env("KUBECONFIG", "")),
			Namespace: env("KUBEWATCHER_NAMESPACE", ""),
			InCluster: env("KUBEWATCHER_IN_CLUSTER", "false") == "true",
		},
		AI: AIConfig{
			DeepSeekAPIKey:    env("DEEPSEEK_API_KEY", ""),
			AnomstackURL:      env("ANOMSTACK_URL", "http://localhost:8087"),
			AnomstackAPIKey:   env("ANOMSTACK_API_KEY", ""),
			MCPServersConfig:  env("MCP_SERVERS_CONFIG", ""),
			AgentInstructions: env("AGENT_INSTRUCTIONS", "Analyze the cluster health based on recent events and alerts."),
			FlinkURL:          env("KUBEWATCHER_FLINK_URL", ""),
			FlinkAPIKey:       env("KUBEWATCHER_FLINK_API_KEY", ""),
			VertexProject:     env("VERTEX_PROJECT", ""),
			VertexLocation:    env("VERTEX_LOCATION", "europe-west3"),
			PrometheusURL:     env("PROMETHEUS_URL", ""), // auto-detected if empty
			PopeyeBinary:      env("KUBEWATCHER_POPEYE_BIN", "popeye"),
			ContextTokenMax:   8000,
			ContextTimeout:    3 * time.Second,
		},
		ArgoCD: ArgoCDConfig{
			URL:      env("ARGOCD_URL", ""),
			Token:    env("ARGOCD_TOKEN", ""),
			Insecure: env("ARGOCD_INSECURE", "false") == "true",
		},
		Security: SecurityConfig{
			SnykToken:   env("SNYK_TOKEN", ""),
			TrivyBinary: env("KUBEWATCHER_TRIVY_BIN", "trivy"),
			FalcoURL:    env("KUBEWATCHER_FALCO_URL", ""),
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
		Auth: AuthConfig{
			PublicBaseURL:      env("KUBEWATCHER_AUTH_BASE_URL", "http://127.0.0.1:8080"),
			GoogleClientID:     env("KUBEWATCHER_GOOGLE_CLIENT_ID", ""),
			GoogleClientSecret: env("KUBEWATCHER_GOOGLE_CLIENT_SECRET", ""),
			OIDCIssuer:         env("KUBEWATCHER_OIDC_ISSUER", ""),
			OIDCClientID:       env("KUBEWATCHER_OIDC_CLIENT_ID", ""),
			OIDCClientSecret:   env("KUBEWATCHER_OIDC_CLIENT_SECRET", ""),
			OIDCDisplayName:    env("KUBEWATCHER_OIDC_DISPLAY_NAME", "Corporate SSO"),
			AllowLocalSignup:   env("KUBEWATCHER_AUTH_ALLOW_SIGNUP", "true") != "false",
			DevMode:            env("KUBEWATCHER_AUTH_DISABLED", "false") == "true",
		},
	}

	// Layer in user-customized settings persisted from the desktop UI. Persisted
	// values override env vars where both are set, because the UI is the user's
	// most recent explicit choice. Failing to read the file is non-fatal: we
	// log later via the application logger; here we just fall back to env.
	if persisted, err := LoadPersistedSettings(); err == nil && persisted != nil {
		persisted.MergeInto(cfg)
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
