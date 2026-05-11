package config

import (
	"fmt"
	"os"
	"strconv"
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
	Pipelines   PipelinesConfig
	Billing     BillingConfig
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
	PublicBaseURL string `env:"argus_AUTH_BASE_URL"`

	GoogleClientID     string `env:"argus_GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"argus_GOOGLE_CLIENT_SECRET"`

	OIDCIssuer       string `env:"argus_OIDC_ISSUER"` // e.g. https://acme.okta.com
	OIDCClientID     string `env:"argus_OIDC_CLIENT_ID"`
	OIDCClientSecret string `env:"argus_OIDC_CLIENT_SECRET"`
	OIDCDisplayName  string `env:"argus_OIDC_DISPLAY_NAME"` // e.g. "Acme SSO"

	// AllowLocalSignup controls whether the email/password registration
	// endpoint is open. Default is true for the desktop app; SaaS
	// admins typically flip it off and rely on SSO + invites.
	AllowLocalSignup bool `env:"argus_AUTH_ALLOW_SIGNUP"`

	// DevMode disables the entire auth gate. Intended ONLY for local
	// development — every /api request is treated as authenticated and
	// the frontend skips the LoginView. SetupAuth refuses to honor this
	// flag when the API binds to anything other than loopback, so a
	// misconfigured public deploy can't accidentally leave auth off.
	DevMode bool `env:"argus_AUTH_DISABLED"`
}

// KubernetesConfig holds cluster connection settings.
type KubernetesConfig struct {
	Context   string `env:"argus_CONTEXT"`
	Config    string `env:"argus_KUBECONFIG"` // explicit override; empty = client-go default rules
	Namespace string `env:"argus_NAMESPACE"`
	InCluster bool   `env:"argus_IN_CLUSTER"`
}

// AIConfig holds settings for AI diagnostics and anomaly detection.
//
// LLMBaseURL and LLMModel let the runtime point at any OpenAI-compatible
// inference server — DeepSeek's hosted API by default, but also a self-hosted
// vLLM running on vast.ai (see infra/vastai) or GCP (see infra/gcp). When
// LLMBaseURL is set, DeepSeekAPIKey is reused as the bearer token.
type AIConfig struct {
	DeepSeekAPIKey    string `env:"DEEPSEEK_API_KEY"`
	LLMBaseURL        string `env:"argus_LLM_BASE_URL"`
	LLMModel          string `env:"argus_LLM_MODEL"`
	AnomstackURL      string `env:"ANOMSTACK_URL"`
	AnomstackAPIKey   string `env:"ANOMSTACK_API_KEY"`
	MCPServersConfig  string `env:"MCP_SERVERS_CONFIG"`
	AgentInstructions string `env:"AGENT_INSTRUCTIONS"`
	FlinkURL          string `env:"argus_FLINK_URL"`
	FlinkAPIKey       string `env:"argus_FLINK_API_KEY"`
	VertexProject     string `env:"VERTEX_PROJECT"`
	VertexLocation    string `env:"VERTEX_LOCATION"`
	PrometheusURL     string `env:"PROMETHEUS_URL"` // optional: enhances metrics if available
	PopeyeBinary      string `env:"argus_POPEYE_BIN"`
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
	TrivyBinary string `env:"argus_TRIVY_BIN"` // path to trivy binary (default: "trivy")
	FalcoURL    string `env:"argus_FALCO_URL"` // Falco gRPC/HTTP endpoint
}

// PipelineProvider is the chosen CI/CD backend.
type PipelineProvider = string

// PipelinesConfig holds CI/CD provider settings + auto code-review
// destinations + PR notification toggles. Pipelines are opt-in; when
// Enabled is false the integration is dormant regardless of which
// provider fields are populated.
type PipelinesConfig struct {
	Enabled  bool             `env:"argus_PIPELINES_ENABLED"`
	Provider PipelineProvider `env:"argus_PIPELINES_PROVIDER"`

	// GitHub Actions
	GitHubToken    string `env:"argus_PIPELINES_GITHUB_TOKEN"`
	GitHubOwner    string `env:"argus_PIPELINES_GITHUB_OWNER"`
	GitHubRepo     string `env:"argus_PIPELINES_GITHUB_REPO"`
	GitHubWorkflow string `env:"argus_PIPELINES_GITHUB_WORKFLOW"`

	// GitLab CI/CD
	GitLabURL       string `env:"argus_PIPELINES_GITLAB_URL"`
	GitLabToken     string `env:"argus_PIPELINES_GITLAB_TOKEN"`
	GitLabProjectID string `env:"argus_PIPELINES_GITLAB_PROJECT_ID"`
	GitLabRef       string `env:"argus_PIPELINES_GITLAB_REF"`

	// AWS CodeBuild
	AWSRegion    string `env:"argus_PIPELINES_AWS_REGION"`
	AWSAccessKey string `env:"argus_PIPELINES_AWS_ACCESS_KEY"`
	AWSSecretKey string `env:"argus_PIPELINES_AWS_SECRET_KEY"`
	AWSProject   string `env:"argus_PIPELINES_AWS_PROJECT"`

	// Google Cloud Build
	GCPProject     string `env:"argus_PIPELINES_GCP_PROJECT"`
	GCPRegion      string `env:"argus_PIPELINES_GCP_REGION"`
	GCPCredentials string `env:"argus_PIPELINES_GCP_CREDENTIALS"`

	// CircleCI
	CircleCIToken       string `env:"argus_PIPELINES_CIRCLECI_TOKEN"`
	CircleCIProjectSlug string `env:"argus_PIPELINES_CIRCLECI_SLUG"`

	// Azure Pipelines (Azure DevOps)
	AzureOrganization string `env:"argus_PIPELINES_AZURE_ORG"`
	AzureProject      string `env:"argus_PIPELINES_AZURE_PROJECT"`
	AzurePipelineID   string `env:"argus_PIPELINES_AZURE_PIPELINE_ID"`
	AzureToken        string `env:"argus_PIPELINES_AZURE_TOKEN"`
	AzureBranch       string `env:"argus_PIPELINES_AZURE_BRANCH"`

	// PR notifications.
	NotifyOnPROpened    bool `env:"argus_PIPELINES_NOTIFY_OPENED"`
	NotifyOnPRUpdated   bool `env:"argus_PIPELINES_NOTIFY_UPDATED"`
	NotifyOnPRCommented bool `env:"argus_PIPELINES_NOTIFY_COMMENTED"`
	NotifyOnPRMerged    bool `env:"argus_PIPELINES_NOTIFY_MERGED"`

	// Auto code review + destination.
	AutoCodeReview        bool   `env:"argus_PIPELINES_AUTO_REVIEW"`
	CodeReviewDestination string `env:"argus_PIPELINES_REVIEW_DEST"`
	GDriveFolderID        string `env:"argus_PIPELINES_GDRIVE_FOLDER"`
	CodeReviewS3Prefix    string `env:"argus_PIPELINES_REVIEW_S3_PREFIX"`
	CodeReviewEmailTo     string `env:"argus_PIPELINES_REVIEW_EMAIL_TO"`

	// Documentation destinations.
	ConfluenceURL          string `env:"argus_PIPELINES_CONFLUENCE_URL"`
	ConfluenceEmail        string `env:"argus_PIPELINES_CONFLUENCE_EMAIL"`
	ConfluenceToken        string `env:"argus_PIPELINES_CONFLUENCE_TOKEN"`
	ConfluenceSpaceKey     string `env:"argus_PIPELINES_CONFLUENCE_SPACE"`
	ConfluenceParentPageID string `env:"argus_PIPELINES_CONFLUENCE_PARENT"`
	NotionToken            string `env:"argus_PIPELINES_NOTION_TOKEN"`
	NotionDatabaseID       string `env:"argus_PIPELINES_NOTION_DATABASE"`
	EvernoteToken          string `env:"argus_PIPELINES_EVERNOTE_TOKEN"`
	EvernoteNotebookGUID   string `env:"argus_PIPELINES_EVERNOTE_NOTEBOOK"`
	OneNoteToken           string `env:"argus_PIPELINES_ONENOTE_TOKEN"`
	OneNoteSectionID       string `env:"argus_PIPELINES_ONENOTE_SECTION"`
	AmplenoteAPIKey        string `env:"argus_PIPELINES_AMPLENOTE_KEY"`
	StandardNotesURL       string `env:"argus_PIPELINES_STDNOTES_URL"`
	StandardNotesToken     string `env:"argus_PIPELINES_STDNOTES_TOKEN"`
	ObsidianVaultPath      string `env:"argus_PIPELINES_OBSIDIAN_VAULT"`
	JoplinURL              string `env:"argus_PIPELINES_JOPLIN_URL"`
	JoplinToken            string `env:"argus_PIPELINES_JOPLIN_TOKEN"`
	LogseqGraphPath        string `env:"argus_PIPELINES_LOGSEQ_GRAPH"`
	BearToken              string `env:"argus_PIPELINES_BEAR_TOKEN"`
}

// BillingConfig drives the pay-as-you-go usage tracker.
type BillingConfig struct {
	InputCostPer1M  float64 `env:"argus_BILLING_INPUT_PER_1M"`
	OutputCostPer1M float64 `env:"argus_BILLING_OUTPUT_PER_1M"`
	MonthlyBudget   float64 `env:"argus_BILLING_MONTHLY_BUDGET"`
}

// FeaturesConfig holds tier and license info.
type FeaturesConfig struct {
	Tier       Tier   `env:"argus_TIER"`
	LicenseKey string `env:"argus_LICENSE"`
}

// ServerConfig holds HTTP/API server settings.
type ServerConfig struct {
	Port         string `env:"argus_PORT"`
	MetricsPort  string `env:"argus_METRICS_PORT"`
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// LoggingConfig holds structured logging settings.
type LoggingConfig struct {
	Level  string `env:"argus_LOG_LEVEL"`
	Format string `env:"argus_LOG_FORMAT"` // "json" or "text"
}

// DecisionLogConfig holds the path and parsing settings for DECISION_LOG.md.
type DecisionLogConfig struct {
	Path string `env:"argus_DECISION_LOG"`
}

// S3Config holds S3 bucket settings for notebook storage.
type S3Config struct {
	Bucket    string `env:"argus_S3_BUCKET"`
	Region    string `env:"argus_S3_REGION"`
	Endpoint  string `env:"argus_S3_ENDPOINT"`
	AccessKey string `env:"argus_S3_ACCESS_KEY"`
	SecretKey string `env:"argus_S3_SECRET_KEY"`
}

func New() (*OnlineDataConfig, error) {
	cfg := &OnlineDataConfig{
		Kubernetes: KubernetesConfig{
			Context:   env("argus_CONTEXT", ""),
			Config:    env("argus_KUBECONFIG", env("KUBECONFIG", "")),
			Namespace: env("argus_NAMESPACE", ""),
			InCluster: env("argus_IN_CLUSTER", "false") == "true",
		},
		AI: AIConfig{
			DeepSeekAPIKey:    env("DEEPSEEK_API_KEY", ""),
			LLMBaseURL:        env("argus_LLM_BASE_URL", ""),
			LLMModel:          env("argus_LLM_MODEL", ""),
			AnomstackURL:      env("ANOMSTACK_URL", "http://localhost:8087"),
			AnomstackAPIKey:   env("ANOMSTACK_API_KEY", ""),
			MCPServersConfig:  env("MCP_SERVERS_CONFIG", ""),
			AgentInstructions: env("AGENT_INSTRUCTIONS", "Analyze the cluster health based on recent events and alerts."),
			FlinkURL:          env("argus_FLINK_URL", ""),
			FlinkAPIKey:       env("argus_FLINK_API_KEY", ""),
			VertexProject:     env("VERTEX_PROJECT", ""),
			VertexLocation:    env("VERTEX_LOCATION", "europe-west3"),
			PrometheusURL:     env("PROMETHEUS_URL", ""), // auto-detected if empty
			PopeyeBinary:      env("argus_POPEYE_BIN", "popeye"),
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
			TrivyBinary: env("argus_TRIVY_BIN", "trivy"),
			FalcoURL:    env("argus_FALCO_URL", ""),
		},
		Pipelines: PipelinesConfig{
			Enabled:                env("argus_PIPELINES_ENABLED", "false") == "true",
			Provider:               env("argus_PIPELINES_PROVIDER", ""),
			GitHubToken:            env("argus_PIPELINES_GITHUB_TOKEN", ""),
			GitHubOwner:            env("argus_PIPELINES_GITHUB_OWNER", ""),
			GitHubRepo:             env("argus_PIPELINES_GITHUB_REPO", ""),
			GitHubWorkflow:         env("argus_PIPELINES_GITHUB_WORKFLOW", ""),
			GitLabURL:              env("argus_PIPELINES_GITLAB_URL", "https://gitlab.com"),
			GitLabToken:            env("argus_PIPELINES_GITLAB_TOKEN", ""),
			GitLabProjectID:        env("argus_PIPELINES_GITLAB_PROJECT_ID", ""),
			GitLabRef:              env("argus_PIPELINES_GITLAB_REF", "main"),
			AWSRegion:              env("argus_PIPELINES_AWS_REGION", "us-east-1"),
			AWSAccessKey:           env("argus_PIPELINES_AWS_ACCESS_KEY", ""),
			AWSSecretKey:           env("argus_PIPELINES_AWS_SECRET_KEY", ""),
			AWSProject:             env("argus_PIPELINES_AWS_PROJECT", ""),
			GCPProject:             env("argus_PIPELINES_GCP_PROJECT", ""),
			GCPRegion:              env("argus_PIPELINES_GCP_REGION", "global"),
			GCPCredentials:         env("argus_PIPELINES_GCP_CREDENTIALS", ""),
			CircleCIToken:          env("argus_PIPELINES_CIRCLECI_TOKEN", ""),
			CircleCIProjectSlug:    env("argus_PIPELINES_CIRCLECI_SLUG", ""),
			AzureOrganization:      env("argus_PIPELINES_AZURE_ORG", ""),
			AzureProject:           env("argus_PIPELINES_AZURE_PROJECT", ""),
			AzurePipelineID:        env("argus_PIPELINES_AZURE_PIPELINE_ID", ""),
			AzureToken:             env("argus_PIPELINES_AZURE_TOKEN", ""),
			AzureBranch:            env("argus_PIPELINES_AZURE_BRANCH", "refs/heads/main"),
			NotifyOnPROpened:       env("argus_PIPELINES_NOTIFY_OPENED", "false") == "true",
			NotifyOnPRUpdated:      env("argus_PIPELINES_NOTIFY_UPDATED", "false") == "true",
			NotifyOnPRCommented:    env("argus_PIPELINES_NOTIFY_COMMENTED", "false") == "true",
			NotifyOnPRMerged:       env("argus_PIPELINES_NOTIFY_MERGED", "false") == "true",
			AutoCodeReview:         env("argus_PIPELINES_AUTO_REVIEW", "false") == "true",
			CodeReviewDestination:  env("argus_PIPELINES_REVIEW_DEST", "local"),
			GDriveFolderID:         env("argus_PIPELINES_GDRIVE_FOLDER", ""),
			CodeReviewS3Prefix:     env("argus_PIPELINES_REVIEW_S3_PREFIX", "code-reviews/"),
			CodeReviewEmailTo:      env("argus_PIPELINES_REVIEW_EMAIL_TO", ""),
			ConfluenceURL:          env("argus_PIPELINES_CONFLUENCE_URL", ""),
			ConfluenceEmail:        env("argus_PIPELINES_CONFLUENCE_EMAIL", ""),
			ConfluenceToken:        env("argus_PIPELINES_CONFLUENCE_TOKEN", ""),
			ConfluenceSpaceKey:     env("argus_PIPELINES_CONFLUENCE_SPACE", ""),
			ConfluenceParentPageID: env("argus_PIPELINES_CONFLUENCE_PARENT", ""),
			NotionToken:            env("argus_PIPELINES_NOTION_TOKEN", ""),
			NotionDatabaseID:       env("argus_PIPELINES_NOTION_DATABASE", ""),
			EvernoteToken:          env("argus_PIPELINES_EVERNOTE_TOKEN", ""),
			EvernoteNotebookGUID:   env("argus_PIPELINES_EVERNOTE_NOTEBOOK", ""),
			OneNoteToken:           env("argus_PIPELINES_ONENOTE_TOKEN", ""),
			OneNoteSectionID:       env("argus_PIPELINES_ONENOTE_SECTION", ""),
			AmplenoteAPIKey:        env("argus_PIPELINES_AMPLENOTE_KEY", ""),
			StandardNotesURL:       env("argus_PIPELINES_STDNOTES_URL", "https://api.standardnotes.com"),
			StandardNotesToken:     env("argus_PIPELINES_STDNOTES_TOKEN", ""),
			ObsidianVaultPath:      env("argus_PIPELINES_OBSIDIAN_VAULT", ""),
			JoplinURL:              env("argus_PIPELINES_JOPLIN_URL", "http://127.0.0.1:41184"),
			JoplinToken:            env("argus_PIPELINES_JOPLIN_TOKEN", ""),
			LogseqGraphPath:        env("argus_PIPELINES_LOGSEQ_GRAPH", ""),
			BearToken:              env("argus_PIPELINES_BEAR_TOKEN", ""),
		},
		Billing: BillingConfig{
			InputCostPer1M:  envFloat("argus_BILLING_INPUT_PER_1M", 0),
			OutputCostPer1M: envFloat("argus_BILLING_OUTPUT_PER_1M", 0),
			MonthlyBudget:   envFloat("argus_BILLING_MONTHLY_BUDGET", 0),
		},
		Features: FeaturesConfig{
			Tier:       parseTier(env("argus_TIER", "pro")), // pro for dev; production gates via license
			LicenseKey: env("argus_LICENSE", ""),
		},
		Server: ServerConfig{
			Port:         env("argus_PORT", "8080"),
			MetricsPort:  env("argus_METRICS_PORT", "9090"),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  env("argus_LOG_LEVEL", "info"),
			Format: env("argus_LOG_FORMAT", "text"),
		},
		DecisionLog: DecisionLogConfig{
			Path: env("argus_DECISION_LOG", "DECISION_LOG.md"),
		},
		S3: S3Config{
			Bucket:    env("argus_S3_BUCKET", ""),
			Region:    env("argus_S3_REGION", "us-east-1"),
			Endpoint:  env("argus_S3_ENDPOINT", ""),
			AccessKey: env("argus_S3_ACCESS_KEY", ""),
			SecretKey: env("argus_S3_SECRET_KEY", ""),
		},
		Auth: AuthConfig{
			PublicBaseURL:      env("argus_AUTH_BASE_URL", "http://127.0.0.1:8080"),
			GoogleClientID:     env("argus_GOOGLE_CLIENT_ID", ""),
			GoogleClientSecret: env("argus_GOOGLE_CLIENT_SECRET", ""),
			OIDCIssuer:         env("argus_OIDC_ISSUER", ""),
			OIDCClientID:       env("argus_OIDC_CLIENT_ID", ""),
			OIDCClientSecret:   env("argus_OIDC_CLIENT_SECRET", ""),
			OIDCDisplayName:    env("argus_OIDC_DISPLAY_NAME", "Corporate SSO"),
			AllowLocalSignup:   env("argus_AUTH_ALLOW_SIGNUP", "true") != "false",
			DevMode:            env("argus_AUTH_DISABLED", "false") == "true",
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

func envFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func parseTier(s string) Tier {
	switch strings.ToLower(s) {
	case "pro":
		return TierPro
	default:
		return TierFree
	}
}
