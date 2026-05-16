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
	SaaS        SaaSConfig
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
	PublicBaseURL string `env:"ARGUS_AUTH_BASE_URL"`

	GoogleClientID     string `env:"ARGUS_GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"ARGUS_GOOGLE_CLIENT_SECRET"`

	OIDCIssuer       string `env:"ARGUS_OIDC_ISSUER"` // e.g. https://acme.okta.com
	OIDCClientID     string `env:"ARGUS_OIDC_CLIENT_ID"`
	OIDCClientSecret string `env:"ARGUS_OIDC_CLIENT_SECRET"`
	OIDCDisplayName  string `env:"ARGUS_OIDC_DISPLAY_NAME"` // e.g. "Acme SSO"

	// AllowLocalSignup controls whether the email/password registration
	// endpoint is open. Default is true for the desktop app; SaaS
	// admins typically flip it off and rely on SSO + invites.
	AllowLocalSignup bool `env:"ARGUS_AUTH_ALLOW_SIGNUP"`

	// DevMode disables the entire auth gate. Intended ONLY for local
	// development — every /api request is treated as authenticated and
	// the frontend skips the LoginView. SetupAuth refuses to honor this
	// flag when the API binds to anything other than loopback, so a
	// misconfigured public deploy can't accidentally leave auth off.
	DevMode bool `env:"ARGUS_AUTH_DISABLED"`

	// Passkey (WebAuthn) configuration. PasskeyEnabled toggles the
	// entire feature; the three RP_* knobs configure the Relying
	// Party. RPID is the eTLD+1 of the origin (e.g. "argus.example.com"
	// or "localhost" for dev); RPOrigin is the fully-qualified origin
	// including scheme + port the browser actually serves Argus from.
	// Mismatched RPID/RPOrigin pairs are the #1 source of WebAuthn
	// confusion — see kube/docs/auth-passkeys.md.
	PasskeyEnabled  bool   `env:"ARGUS_PASSKEY_ENABLED"`
	PasskeyRPID     string `env:"ARGUS_PASSKEY_RP_ID"`
	PasskeyRPName   string `env:"ARGUS_PASSKEY_RP_NAME"`
	PasskeyRPOrigin string `env:"ARGUS_PASSKEY_RP_ORIGIN"`
}

// KubernetesConfig holds cluster connection settings.
type KubernetesConfig struct {
	Context   string `env:"ARGUS_CONTEXT"`
	Config    string `env:"ARGUS_KUBECONFIG"` // explicit override; empty = client-go default rules
	Namespace string `env:"ARGUS_NAMESPACE"`
	InCluster bool   `env:"ARGUS_IN_CLUSTER"`
}

// AIConfig holds settings for AI diagnostics and anomaly detection.
//
// LLMBaseURL and LLMModel let the runtime point at any OpenAI-compatible
// inference server — DeepSeek's hosted API by default, but also a self-hosted
// vLLM running on vast.ai (see infra/vastai) or GCP (see infra/gcp). When
// LLMBaseURL is set, DeepSeekAPIKey is reused as the bearer token.
type AIConfig struct {
	DeepSeekAPIKey    string `env:"DEEPSEEK_API_KEY"`
	LLMBaseURL        string `env:"ARGUS_LLM_BASE_URL"`
	LLMModel          string `env:"ARGUS_LLM_MODEL"`
	AnomstackURL      string `env:"ANOMSTACK_URL"`
	AnomstackAPIKey   string `env:"ANOMSTACK_API_KEY"`
	MCPServersConfig  string `env:"MCP_SERVERS_CONFIG"`
	AgentInstructions string `env:"AGENT_INSTRUCTIONS"`
	FlinkURL          string `env:"ARGUS_FLINK_URL"`
	FlinkAPIKey       string `env:"ARGUS_FLINK_API_KEY"`
	VertexProject     string `env:"VERTEX_PROJECT"`
	VertexLocation    string `env:"VERTEX_LOCATION"`
	PrometheusURL     string `env:"PROMETHEUS_URL"` // optional: enhances metrics if available
	PopeyeBinary      string `env:"ARGUS_POPEYE_BIN"`
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
	TrivyBinary string `env:"ARGUS_TRIVY_BIN"` // path to trivy binary (default: "trivy")
	FalcoURL    string `env:"ARGUS_FALCO_URL"` // Falco gRPC/HTTP endpoint
}

// PipelineProvider is the chosen CI/CD backend.
type PipelineProvider = string

// PipelinesConfig holds CI/CD provider settings + auto code-review
// destinations + PR notification toggles. Pipelines are opt-in; when
// Enabled is false the integration is dormant regardless of which
// provider fields are populated.
type PipelinesConfig struct {
	Enabled  bool             `env:"ARGUS_PIPELINES_ENABLED"`
	Provider PipelineProvider `env:"ARGUS_PIPELINES_PROVIDER"`

	// GitHub Actions
	GitHubToken    string `env:"ARGUS_PIPELINES_GITHUB_TOKEN"`
	GitHubOwner    string `env:"ARGUS_PIPELINES_GITHUB_OWNER"`
	GitHubRepo     string `env:"ARGUS_PIPELINES_GITHUB_REPO"`
	GitHubWorkflow string `env:"ARGUS_PIPELINES_GITHUB_WORKFLOW"`

	// GitLab CI/CD
	GitLabURL       string `env:"ARGUS_PIPELINES_GITLAB_URL"`
	GitLabToken     string `env:"ARGUS_PIPELINES_GITLAB_TOKEN"`
	GitLabProjectID string `env:"ARGUS_PIPELINES_GITLAB_PROJECT_ID"`
	GitLabRef       string `env:"ARGUS_PIPELINES_GITLAB_REF"`

	// AWS CodeBuild
	AWSRegion    string `env:"ARGUS_PIPELINES_AWS_REGION"`
	AWSAccessKey string `env:"ARGUS_PIPELINES_AWS_ACCESS_KEY"`
	AWSSecretKey string `env:"ARGUS_PIPELINES_AWS_SECRET_KEY"`
	AWSProject   string `env:"ARGUS_PIPELINES_AWS_PROJECT"`

	// Google Cloud Build
	GCPProject     string `env:"ARGUS_PIPELINES_GCP_PROJECT"`
	GCPRegion      string `env:"ARGUS_PIPELINES_GCP_REGION"`
	GCPCredentials string `env:"ARGUS_PIPELINES_GCP_CREDENTIALS"`

	// CircleCI
	CircleCIToken       string `env:"ARGUS_PIPELINES_CIRCLECI_TOKEN"`
	CircleCIProjectSlug string `env:"ARGUS_PIPELINES_CIRCLECI_SLUG"`

	// Azure Pipelines (Azure DevOps)
	AzureOrganization string `env:"ARGUS_PIPELINES_AZURE_ORG"`
	AzureProject      string `env:"ARGUS_PIPELINES_AZURE_PROJECT"`
	AzurePipelineID   string `env:"ARGUS_PIPELINES_AZURE_PIPELINE_ID"`
	AzureToken        string `env:"ARGUS_PIPELINES_AZURE_TOKEN"`
	AzureBranch       string `env:"ARGUS_PIPELINES_AZURE_BRANCH"`

	// PR notifications.
	NotifyOnPROpened    bool `env:"ARGUS_PIPELINES_NOTIFY_OPENED"`
	NotifyOnPRUpdated   bool `env:"ARGUS_PIPELINES_NOTIFY_UPDATED"`
	NotifyOnPRCommented bool `env:"ARGUS_PIPELINES_NOTIFY_COMMENTED"`
	NotifyOnPRMerged    bool `env:"ARGUS_PIPELINES_NOTIFY_MERGED"`

	// Auto code review + destination.
	AutoCodeReview        bool   `env:"ARGUS_PIPELINES_AUTO_REVIEW"`
	CodeReviewDestination string `env:"ARGUS_PIPELINES_REVIEW_DEST"`
	GDriveFolderID        string `env:"ARGUS_PIPELINES_GDRIVE_FOLDER"`
	CodeReviewS3Prefix    string `env:"ARGUS_PIPELINES_REVIEW_S3_PREFIX"`
	CodeReviewEmailTo     string `env:"ARGUS_PIPELINES_REVIEW_EMAIL_TO"`

	// Documentation destinations.
	ConfluenceURL          string `env:"ARGUS_PIPELINES_CONFLUENCE_URL"`
	ConfluenceEmail        string `env:"ARGUS_PIPELINES_CONFLUENCE_EMAIL"`
	ConfluenceToken        string `env:"ARGUS_PIPELINES_CONFLUENCE_TOKEN"`
	ConfluenceSpaceKey     string `env:"ARGUS_PIPELINES_CONFLUENCE_SPACE"`
	ConfluenceParentPageID string `env:"ARGUS_PIPELINES_CONFLUENCE_PARENT"`
	NotionToken            string `env:"ARGUS_PIPELINES_NOTION_TOKEN"`
	NotionDatabaseID       string `env:"ARGUS_PIPELINES_NOTION_DATABASE"`
	EvernoteToken          string `env:"ARGUS_PIPELINES_EVERNOTE_TOKEN"`
	EvernoteNotebookGUID   string `env:"ARGUS_PIPELINES_EVERNOTE_NOTEBOOK"`
	OneNoteToken           string `env:"ARGUS_PIPELINES_ONENOTE_TOKEN"`
	OneNoteSectionID       string `env:"ARGUS_PIPELINES_ONENOTE_SECTION"`
	AmplenoteAPIKey        string `env:"ARGUS_PIPELINES_AMPLENOTE_KEY"`
	StandardNotesURL       string `env:"ARGUS_PIPELINES_STDNOTES_URL"`
	StandardNotesToken     string `env:"ARGUS_PIPELINES_STDNOTES_TOKEN"`
	ObsidianVaultPath      string `env:"ARGUS_PIPELINES_OBSIDIAN_VAULT"`
	JoplinURL              string `env:"ARGUS_PIPELINES_JOPLIN_URL"`
	JoplinToken            string `env:"ARGUS_PIPELINES_JOPLIN_TOKEN"`
	LogseqGraphPath        string `env:"ARGUS_PIPELINES_LOGSEQ_GRAPH"`
	BearToken              string `env:"ARGUS_PIPELINES_BEAR_TOKEN"`
}

// BillingConfig drives the pay-as-you-go usage tracker.
type BillingConfig struct {
	InputCostPer1M  float64 `env:"ARGUS_BILLING_INPUT_PER_1M"`
	OutputCostPer1M float64 `env:"ARGUS_BILLING_OUTPUT_PER_1M"`
	MonthlyBudget   float64 `env:"ARGUS_BILLING_MONTHLY_BUDGET"`
}

// FeaturesConfig holds tier and license info.
type FeaturesConfig struct {
	Tier       Tier   `env:"ARGUS_TIER"`
	LicenseKey string `env:"ARGUS_LICENSE"`
}

// ServerConfig holds HTTP/API server settings.
type ServerConfig struct {
	Port         string `env:"ARGUS_PORT"`
	MetricsPort  string `env:"ARGUS_METRICS_PORT"`
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// LoggingConfig holds structured logging settings.
type LoggingConfig struct {
	Level  string `env:"ARGUS_LOG_LEVEL"`
	Format string `env:"ARGUS_LOG_FORMAT"` // "json" or "text"
}

// DecisionLogConfig holds the path and parsing settings for DECISION_LOG.md.
type DecisionLogConfig struct {
	Path string `env:"ARGUS_DECISION_LOG"`
}

// SaaSConfig holds connection settings for the Argus SaaS platform.
type SaaSConfig struct {
	BaseURL string `env:"ARGUS_SAAS_BASE_URL"`
	APIKey  string `env:"ARGUS_SAAS_API_KEY"`
}

// S3Config holds S3 bucket settings for notebook storage.
type S3Config struct {
	Bucket    string `env:"ARGUS_S3_BUCKET"`
	Region    string `env:"ARGUS_S3_REGION"`
	Endpoint  string `env:"ARGUS_S3_ENDPOINT"`
	AccessKey string `env:"ARGUS_S3_ACCESS_KEY"`
	SecretKey string `env:"ARGUS_S3_SECRET_KEY"`
}

func New() (*OnlineDataConfig, error) {
	cfg := &OnlineDataConfig{
		Kubernetes: KubernetesConfig{
			Context:   env("ARGUS_CONTEXT", ""),
			Config:    env("ARGUS_KUBECONFIG", env("KUBECONFIG", "")),
			Namespace: env("ARGUS_NAMESPACE", ""),
			InCluster: env("ARGUS_IN_CLUSTER", "false") == "true",
		},
		AI: AIConfig{
			DeepSeekAPIKey:    env("DEEPSEEK_API_KEY", ""),
			LLMBaseURL:        env("ARGUS_LLM_BASE_URL", ""),
			LLMModel:          env("ARGUS_LLM_MODEL", ""),
			AnomstackURL:      env("ANOMSTACK_URL", "http://localhost:8087"),
			AnomstackAPIKey:   env("ANOMSTACK_API_KEY", ""),
			MCPServersConfig:  env("MCP_SERVERS_CONFIG", ""),
			AgentInstructions: env("AGENT_INSTRUCTIONS", "Analyze the cluster health based on recent events and alerts."),
			FlinkURL:          env("ARGUS_FLINK_URL", ""),
			FlinkAPIKey:       env("ARGUS_FLINK_API_KEY", ""),
			VertexProject:     env("VERTEX_PROJECT", ""),
			VertexLocation:    env("VERTEX_LOCATION", "europe-west3"),
			PrometheusURL:     env("PROMETHEUS_URL", ""), // auto-detected if empty
			PopeyeBinary:      env("ARGUS_POPEYE_BIN", "popeye"),
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
			TrivyBinary: env("ARGUS_TRIVY_BIN", "trivy"),
			FalcoURL:    env("ARGUS_FALCO_URL", ""),
		},
		Pipelines: PipelinesConfig{
			Enabled:                env("ARGUS_PIPELINES_ENABLED", "false") == "true",
			Provider:               env("ARGUS_PIPELINES_PROVIDER", ""),
			GitHubToken:            env("ARGUS_PIPELINES_GITHUB_TOKEN", ""),
			GitHubOwner:            env("ARGUS_PIPELINES_GITHUB_OWNER", ""),
			GitHubRepo:             env("ARGUS_PIPELINES_GITHUB_REPO", ""),
			GitHubWorkflow:         env("ARGUS_PIPELINES_GITHUB_WORKFLOW", ""),
			GitLabURL:              env("ARGUS_PIPELINES_GITLAB_URL", "https://gitlab.com"),
			GitLabToken:            env("ARGUS_PIPELINES_GITLAB_TOKEN", ""),
			GitLabProjectID:        env("ARGUS_PIPELINES_GITLAB_PROJECT_ID", ""),
			GitLabRef:              env("ARGUS_PIPELINES_GITLAB_REF", "main"),
			AWSRegion:              env("ARGUS_PIPELINES_AWS_REGION", "us-east-1"),
			AWSAccessKey:           env("ARGUS_PIPELINES_AWS_ACCESS_KEY", ""),
			AWSSecretKey:           env("ARGUS_PIPELINES_AWS_SECRET_KEY", ""),
			AWSProject:             env("ARGUS_PIPELINES_AWS_PROJECT", ""),
			GCPProject:             env("ARGUS_PIPELINES_GCP_PROJECT", ""),
			GCPRegion:              env("ARGUS_PIPELINES_GCP_REGION", "global"),
			GCPCredentials:         env("ARGUS_PIPELINES_GCP_CREDENTIALS", ""),
			CircleCIToken:          env("ARGUS_PIPELINES_CIRCLECI_TOKEN", ""),
			CircleCIProjectSlug:    env("ARGUS_PIPELINES_CIRCLECI_SLUG", ""),
			AzureOrganization:      env("ARGUS_PIPELINES_AZURE_ORG", ""),
			AzureProject:           env("ARGUS_PIPELINES_AZURE_PROJECT", ""),
			AzurePipelineID:        env("ARGUS_PIPELINES_AZURE_PIPELINE_ID", ""),
			AzureToken:             env("ARGUS_PIPELINES_AZURE_TOKEN", ""),
			AzureBranch:            env("ARGUS_PIPELINES_AZURE_BRANCH", "refs/heads/main"),
			NotifyOnPROpened:       env("ARGUS_PIPELINES_NOTIFY_OPENED", "false") == "true",
			NotifyOnPRUpdated:      env("ARGUS_PIPELINES_NOTIFY_UPDATED", "false") == "true",
			NotifyOnPRCommented:    env("ARGUS_PIPELINES_NOTIFY_COMMENTED", "false") == "true",
			NotifyOnPRMerged:       env("ARGUS_PIPELINES_NOTIFY_MERGED", "false") == "true",
			AutoCodeReview:         env("ARGUS_PIPELINES_AUTO_REVIEW", "false") == "true",
			CodeReviewDestination:  env("ARGUS_PIPELINES_REVIEW_DEST", "local"),
			GDriveFolderID:         env("ARGUS_PIPELINES_GDRIVE_FOLDER", ""),
			CodeReviewS3Prefix:     env("ARGUS_PIPELINES_REVIEW_S3_PREFIX", "code-reviews/"),
			CodeReviewEmailTo:      env("ARGUS_PIPELINES_REVIEW_EMAIL_TO", ""),
			ConfluenceURL:          env("ARGUS_PIPELINES_CONFLUENCE_URL", ""),
			ConfluenceEmail:        env("ARGUS_PIPELINES_CONFLUENCE_EMAIL", ""),
			ConfluenceToken:        env("ARGUS_PIPELINES_CONFLUENCE_TOKEN", ""),
			ConfluenceSpaceKey:     env("ARGUS_PIPELINES_CONFLUENCE_SPACE", ""),
			ConfluenceParentPageID: env("ARGUS_PIPELINES_CONFLUENCE_PARENT", ""),
			NotionToken:            env("ARGUS_PIPELINES_NOTION_TOKEN", ""),
			NotionDatabaseID:       env("ARGUS_PIPELINES_NOTION_DATABASE", ""),
			EvernoteToken:          env("ARGUS_PIPELINES_EVERNOTE_TOKEN", ""),
			EvernoteNotebookGUID:   env("ARGUS_PIPELINES_EVERNOTE_NOTEBOOK", ""),
			OneNoteToken:           env("ARGUS_PIPELINES_ONENOTE_TOKEN", ""),
			OneNoteSectionID:       env("ARGUS_PIPELINES_ONENOTE_SECTION", ""),
			AmplenoteAPIKey:        env("ARGUS_PIPELINES_AMPLENOTE_KEY", ""),
			StandardNotesURL:       env("ARGUS_PIPELINES_STDNOTES_URL", "https://api.standardnotes.com"),
			StandardNotesToken:     env("ARGUS_PIPELINES_STDNOTES_TOKEN", ""),
			ObsidianVaultPath:      env("ARGUS_PIPELINES_OBSIDIAN_VAULT", ""),
			JoplinURL:              env("ARGUS_PIPELINES_JOPLIN_URL", "http://127.0.0.1:41184"),
			JoplinToken:            env("ARGUS_PIPELINES_JOPLIN_TOKEN", ""),
			LogseqGraphPath:        env("ARGUS_PIPELINES_LOGSEQ_GRAPH", ""),
			BearToken:              env("ARGUS_PIPELINES_BEAR_TOKEN", ""),
		},
		Billing: BillingConfig{
			InputCostPer1M:  envFloat("ARGUS_BILLING_INPUT_PER_1M", 0),
			OutputCostPer1M: envFloat("ARGUS_BILLING_OUTPUT_PER_1M", 0),
			MonthlyBudget:   envFloat("ARGUS_BILLING_MONTHLY_BUDGET", 0),
		},
		Features: FeaturesConfig{
			Tier:       parseTier(env("ARGUS_TIER", "pro")), // pro for dev; production gates via license
			LicenseKey: env("ARGUS_LICENSE", ""),
		},
		Server: ServerConfig{
			Port:         env("ARGUS_PORT", "8080"),
			MetricsPort:  env("ARGUS_METRICS_PORT", "9090"),
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  env("ARGUS_LOG_LEVEL", "info"),
			Format: env("ARGUS_LOG_FORMAT", "text"),
		},
		DecisionLog: DecisionLogConfig{
			Path: env("ARGUS_DECISION_LOG", "DECISION_LOG.md"),
		},
		S3: S3Config{
			Bucket:    env("ARGUS_S3_BUCKET", ""),
			Region:    env("ARGUS_S3_REGION", "us-east-1"),
			Endpoint:  env("ARGUS_S3_ENDPOINT", ""),
			AccessKey: env("ARGUS_S3_ACCESS_KEY", ""),
			SecretKey: env("ARGUS_S3_SECRET_KEY", ""),
		},
		SaaS: SaaSConfig{
			BaseURL: env("ARGUS_SAAS_BASE_URL", "https://api.argusplatform.dev"),
			APIKey:  env("ARGUS_SAAS_API_KEY", ""),
		},
		Auth: AuthConfig{
			PublicBaseURL:      env("ARGUS_AUTH_BASE_URL", "http://127.0.0.1:8080"),
			GoogleClientID:     env("ARGUS_GOOGLE_CLIENT_ID", ""),
			GoogleClientSecret: env("ARGUS_GOOGLE_CLIENT_SECRET", ""),
			OIDCIssuer:         env("ARGUS_OIDC_ISSUER", ""),
			OIDCClientID:       env("ARGUS_OIDC_CLIENT_ID", ""),
			OIDCClientSecret:   env("ARGUS_OIDC_CLIENT_SECRET", ""),
			OIDCDisplayName:    env("ARGUS_OIDC_DISPLAY_NAME", "Corporate SSO"),
			AllowLocalSignup:   env("ARGUS_AUTH_ALLOW_SIGNUP", "true") != "false",
			DevMode:            envBool("ARGUS_AUTH_DISABLED", false),
			PasskeyEnabled:     envBool("ARGUS_PASSKEY_ENABLED", false),
			PasskeyRPID:        env("ARGUS_PASSKEY_RP_ID", "localhost"),
			PasskeyRPName:      env("ARGUS_PASSKEY_RP_NAME", "Argus"),
			PasskeyRPOrigin:    env("ARGUS_PASSKEY_RP_ORIGIN", "http://localhost:8080"),
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

// envBool reads a boolean env var, accepting the conventional set of
// truthy spellings. The previous `== "true"` check silently rejected
// `=1`, `=yes`, `=on`, `=TRUE` — a common foot-gun for ops who set
// ARGUS_AUTH_DISABLED on the command line and expected it to work.
func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
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
