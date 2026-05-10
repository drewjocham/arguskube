package config

import (
	"fmt"
	"os"
	"os/user"
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
}

// KubernetesConfig holds cluster connection settings.
type KubernetesConfig struct {
	Context   string `env:"KUBEWATCHER_CONTEXT"`
	Config    string `env:"KUBEWATCHER_KUBECONFIG"` // explicit override; empty = client-go default rules
	Namespace string `env:"KUBEWATCHER_NAMESPACE"`
	InCluster bool   `env:"KUBEWATCHER_IN_CLUSTER"`
}

// AIConfig holds settings for AI diagnostics and anomaly detection.
//
// LLMBaseURL and LLMModel let the runtime point at any OpenAI-compatible
// inference server — DeepSeek's hosted API by default, but also a self-hosted
// vLLM running on vast.ai (see infra/vastai) or GCP (see infra/gcp). When
// LLMBaseURL is set, DeepSeekAPIKey is reused as the bearer token, so the
// existing client and settings UI keep working unchanged.
type AIConfig struct {
	DeepSeekAPIKey  string `env:"DEEPSEEK_API_KEY"`
	LLMBaseURL      string `env:"KUBEWATCHER_LLM_BASE_URL"`
	LLMModel        string `env:"KUBEWATCHER_LLM_MODEL"`
	AnomstackURL    string `env:"ANOMSTACK_URL"`
	AnomstackAPIKey string `env:"ANOMSTACK_API_KEY"`
	VertexProject   string `env:"VERTEX_PROJECT"`
	VertexLocation  string `env:"VERTEX_LOCATION"`
	PrometheusURL   string `env:"PROMETHEUS_URL"` // optional: enhances metrics if available
	PopeyeBinary    string `env:"KUBEWATCHER_POPEYE_BIN"`
	ContextTokenMax int
	ContextTimeout  time.Duration
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
	SnykToken    string `env:"SNYK_TOKEN"`              // API token for Snyk CLI
	TrivyBinary  string `env:"KUBEWATCHER_TRIVY_BIN"`   // path to trivy binary (default: "trivy")
	FalcoURL     string `env:"KUBEWATCHER_FALCO_URL"`   // Falco gRPC/HTTP endpoint
}

// PipelineProvider is the chosen CI/CD backend.
// Empty string means no provider selected. Recognized values:
//   github         — GitHub Actions (workflow_dispatch via REST/GraphQL)
//   gitlab         — GitLab CI/CD (Pipeline Triggers API)
//   aws-codebuild  — AWS CodeBuild (StartBuild via AWS SDK)
//   gcp-cloudbuild — Google Cloud Build (REST/gRPC, GCP IAM)
//   circleci       — CircleCI API v2 (token in header)
//   azure          — Azure Pipelines (Runs REST API, PAT in Basic auth)
type PipelineProvider = string

// PipelinesConfig holds CI/CD provider settings. Pipelines are opt-in: when
// Enabled is false the integration is dormant regardless of which provider
// fields are populated. Only the fields for the selected Provider are used at
// runtime; the others are stored so users can switch providers without losing
// their config.
type PipelinesConfig struct {
	Enabled  bool             `env:"KUBEWATCHER_PIPELINES_ENABLED"`
	Provider PipelineProvider `env:"KUBEWATCHER_PIPELINES_PROVIDER"`

	// GitHub Actions
	GitHubToken    string `env:"KUBEWATCHER_PIPELINES_GITHUB_TOKEN"`
	GitHubOwner    string `env:"KUBEWATCHER_PIPELINES_GITHUB_OWNER"`
	GitHubRepo     string `env:"KUBEWATCHER_PIPELINES_GITHUB_REPO"`
	GitHubWorkflow string `env:"KUBEWATCHER_PIPELINES_GITHUB_WORKFLOW"` // workflow ID or filename

	// GitLab CI/CD
	GitLabURL       string `env:"KUBEWATCHER_PIPELINES_GITLAB_URL"`
	GitLabToken     string `env:"KUBEWATCHER_PIPELINES_GITLAB_TOKEN"`
	GitLabProjectID string `env:"KUBEWATCHER_PIPELINES_GITLAB_PROJECT_ID"`
	GitLabRef       string `env:"KUBEWATCHER_PIPELINES_GITLAB_REF"`

	// AWS CodeBuild
	AWSRegion    string `env:"KUBEWATCHER_PIPELINES_AWS_REGION"`
	AWSAccessKey string `env:"KUBEWATCHER_PIPELINES_AWS_ACCESS_KEY"`
	AWSSecretKey string `env:"KUBEWATCHER_PIPELINES_AWS_SECRET_KEY"`
	AWSProject   string `env:"KUBEWATCHER_PIPELINES_AWS_PROJECT"`

	// Google Cloud Build
	GCPProject     string `env:"KUBEWATCHER_PIPELINES_GCP_PROJECT"`
	GCPRegion      string `env:"KUBEWATCHER_PIPELINES_GCP_REGION"`
	GCPCredentials string `env:"KUBEWATCHER_PIPELINES_GCP_CREDENTIALS"` // path to service-account JSON

	// CircleCI
	CircleCIToken       string `env:"KUBEWATCHER_PIPELINES_CIRCLECI_TOKEN"`
	CircleCIProjectSlug string `env:"KUBEWATCHER_PIPELINES_CIRCLECI_SLUG"` // e.g. github/acme/repo

	// Azure Pipelines (Azure DevOps)
	AzureOrganization string `env:"KUBEWATCHER_PIPELINES_AZURE_ORG"`
	AzureProject      string `env:"KUBEWATCHER_PIPELINES_AZURE_PROJECT"`
	AzurePipelineID   string `env:"KUBEWATCHER_PIPELINES_AZURE_PIPELINE_ID"`
	AzureToken        string `env:"KUBEWATCHER_PIPELINES_AZURE_TOKEN"` // PAT for Basic auth
	AzureBranch       string `env:"KUBEWATCHER_PIPELINES_AZURE_BRANCH"`

	// PR notification toggles. Each fires a desktop notification when the
	// corresponding event is observed against the active provider. Defaults
	// to off so a fresh install is silent; users opt in per event type.
	NotifyOnPROpened    bool `env:"KUBEWATCHER_PIPELINES_NOTIFY_OPENED"`
	NotifyOnPRUpdated   bool `env:"KUBEWATCHER_PIPELINES_NOTIFY_UPDATED"`
	NotifyOnPRCommented bool `env:"KUBEWATCHER_PIPELINES_NOTIFY_COMMENTED"`
	NotifyOnPRMerged    bool `env:"KUBEWATCHER_PIPELINES_NOTIFY_MERGED"`

	// AutoCodeReview, when true, asks the AI to generate a code review for
	// every observed PR. The resulting markdown report is always stored
	// locally in the Reports library and additionally posted to the
	// destination below. CodeReviewDestination is one of:
	//   local           — only stored in-app (default)
	//   gdrive          — uploaded to a Google Drive folder
	//   s3              — uploaded to the S3 bucket from S3Config under the prefix below
	//   email           — sent to CodeReviewEmailTo
	//   confluence      — created/updated as a Confluence page
	//   notion          — created/updated as a Notion page
	//   evernote        — created as an Evernote note
	//   onenote         — created as a OneNote page
	//   amplenote       — created as an Amplenote note
	//   standard-notes  — created as a Standard Notes note
	//   obsidian        — written as a .md file into a local vault
	//   joplin          — created as a Joplin note via Web Clipper API
	//   logseq          — written as a .md file into a local Logseq graph
	//   bear            — created as a Bear note via x-callback-url
	AutoCodeReview        bool   `env:"KUBEWATCHER_PIPELINES_AUTO_REVIEW"`
	CodeReviewDestination string `env:"KUBEWATCHER_PIPELINES_REVIEW_DEST"`

	GDriveFolderID     string `env:"KUBEWATCHER_PIPELINES_GDRIVE_FOLDER"`
	CodeReviewS3Prefix string `env:"KUBEWATCHER_PIPELINES_REVIEW_S3_PREFIX"`
	CodeReviewEmailTo  string `env:"KUBEWATCHER_PIPELINES_REVIEW_EMAIL_TO"` // comma-separated

	// Confluence Cloud — REST v2 with API token + email Basic auth.
	ConfluenceURL          string `env:"KUBEWATCHER_PIPELINES_CONFLUENCE_URL"`           // e.g. https://acme.atlassian.net/wiki
	ConfluenceEmail        string `env:"KUBEWATCHER_PIPELINES_CONFLUENCE_EMAIL"`
	ConfluenceToken        string `env:"KUBEWATCHER_PIPELINES_CONFLUENCE_TOKEN"`        // API token, masked on read
	ConfluenceSpaceKey     string `env:"KUBEWATCHER_PIPELINES_CONFLUENCE_SPACE"`
	ConfluenceParentPageID string `env:"KUBEWATCHER_PIPELINES_CONFLUENCE_PARENT"`

	// Notion — internal integration token (Bearer).
	NotionToken      string `env:"KUBEWATCHER_PIPELINES_NOTION_TOKEN"` // masked on read
	NotionDatabaseID string `env:"KUBEWATCHER_PIPELINES_NOTION_DATABASE"`

	// Evernote — developer token (Cloud API SDK).
	EvernoteToken          string `env:"KUBEWATCHER_PIPELINES_EVERNOTE_TOKEN"` // masked on read
	EvernoteNotebookGUID   string `env:"KUBEWATCHER_PIPELINES_EVERNOTE_NOTEBOOK"`

	// Microsoft OneNote — Graph API access token + section ID.
	OneNoteToken     string `env:"KUBEWATCHER_PIPELINES_ONENOTE_TOKEN"` // masked on read
	OneNoteSectionID string `env:"KUBEWATCHER_PIPELINES_ONENOTE_SECTION"`

	// Amplenote — API key for the public REST API.
	AmplenoteAPIKey string `env:"KUBEWATCHER_PIPELINES_AMPLENOTE_KEY"` // masked on read

	// Standard Notes — server URL + session token (or self-hosted server).
	StandardNotesURL   string `env:"KUBEWATCHER_PIPELINES_STDNOTES_URL"`
	StandardNotesToken string `env:"KUBEWATCHER_PIPELINES_STDNOTES_TOKEN"` // masked on read

	// Obsidian — local vault path; reports drop in as .md files.
	ObsidianVaultPath string `env:"KUBEWATCHER_PIPELINES_OBSIDIAN_VAULT"`

	// Joplin — Web Clipper API on localhost.
	JoplinURL   string `env:"KUBEWATCHER_PIPELINES_JOPLIN_URL"`
	JoplinToken string `env:"KUBEWATCHER_PIPELINES_JOPLIN_TOKEN"` // masked on read

	// Logseq — local graph directory; reports drop in as .md pages.
	LogseqGraphPath string `env:"KUBEWATCHER_PIPELINES_LOGSEQ_GRAPH"`

	// Bear (macOS) — x-callback-url scheme uses an API token.
	BearToken string `env:"KUBEWATCHER_PIPELINES_BEAR_TOKEN"` // masked on read
}

// BillingConfig drives the pay-as-you-go usage tracker.
//
// Rates are USD per million tokens to match how DeepSeek, OpenAI, Anthropic,
// and most others publish prices. Both default to zero, in which case the UI
// shows token totals only and skips cost columns. MonthlyBudget is an optional
// soft cap — the UI badges usage when it crosses the threshold; nothing is
// blocked at runtime.
type BillingConfig struct {
	InputCostPer1M  float64 `env:"KUBEWATCHER_BILLING_INPUT_PER_1M"`
	OutputCostPer1M float64 `env:"KUBEWATCHER_BILLING_OUTPUT_PER_1M"`
	MonthlyBudget   float64 `env:"KUBEWATCHER_BILLING_MONTHLY_BUDGET"`
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
			DeepSeekAPIKey:  env("DEEPSEEK_API_KEY", ""),
			LLMBaseURL:      env("KUBEWATCHER_LLM_BASE_URL", ""),
			LLMModel:        env("KUBEWATCHER_LLM_MODEL", ""),
			AnomstackURL:    env("ANOMSTACK_URL", "http://localhost:8087"),
			AnomstackAPIKey: env("ANOMSTACK_API_KEY", ""),
			VertexProject:   env("VERTEX_PROJECT", ""),
			VertexLocation:  env("VERTEX_LOCATION", "europe-west3"),
			PrometheusURL:   env("PROMETHEUS_URL", ""), // auto-detected if empty
			PopeyeBinary:    env("KUBEWATCHER_POPEYE_BIN", "popeye"),
			ContextTokenMax: 8000,
			ContextTimeout:  3 * time.Second,
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
		Pipelines: PipelinesConfig{
			Enabled:  env("KUBEWATCHER_PIPELINES_ENABLED", "false") == "true",
			Provider: env("KUBEWATCHER_PIPELINES_PROVIDER", ""),

			GitHubToken:    env("KUBEWATCHER_PIPELINES_GITHUB_TOKEN", ""),
			GitHubOwner:    env("KUBEWATCHER_PIPELINES_GITHUB_OWNER", ""),
			GitHubRepo:     env("KUBEWATCHER_PIPELINES_GITHUB_REPO", ""),
			GitHubWorkflow: env("KUBEWATCHER_PIPELINES_GITHUB_WORKFLOW", ""),

			GitLabURL:       env("KUBEWATCHER_PIPELINES_GITLAB_URL", "https://gitlab.com"),
			GitLabToken:     env("KUBEWATCHER_PIPELINES_GITLAB_TOKEN", ""),
			GitLabProjectID: env("KUBEWATCHER_PIPELINES_GITLAB_PROJECT_ID", ""),
			GitLabRef:       env("KUBEWATCHER_PIPELINES_GITLAB_REF", "main"),

			AWSRegion:    env("KUBEWATCHER_PIPELINES_AWS_REGION", "us-east-1"),
			AWSAccessKey: env("KUBEWATCHER_PIPELINES_AWS_ACCESS_KEY", ""),
			AWSSecretKey: env("KUBEWATCHER_PIPELINES_AWS_SECRET_KEY", ""),
			AWSProject:   env("KUBEWATCHER_PIPELINES_AWS_PROJECT", ""),

			GCPProject:     env("KUBEWATCHER_PIPELINES_GCP_PROJECT", ""),
			GCPRegion:      env("KUBEWATCHER_PIPELINES_GCP_REGION", "global"),
			GCPCredentials: env("KUBEWATCHER_PIPELINES_GCP_CREDENTIALS", ""),

			CircleCIToken:       env("KUBEWATCHER_PIPELINES_CIRCLECI_TOKEN", ""),
			CircleCIProjectSlug: env("KUBEWATCHER_PIPELINES_CIRCLECI_SLUG", ""),

			AzureOrganization: env("KUBEWATCHER_PIPELINES_AZURE_ORG", ""),
			AzureProject:      env("KUBEWATCHER_PIPELINES_AZURE_PROJECT", ""),
			AzurePipelineID:   env("KUBEWATCHER_PIPELINES_AZURE_PIPELINE_ID", ""),
			AzureToken:        env("KUBEWATCHER_PIPELINES_AZURE_TOKEN", ""),
			AzureBranch:       env("KUBEWATCHER_PIPELINES_AZURE_BRANCH", "refs/heads/main"),

			NotifyOnPROpened:    env("KUBEWATCHER_PIPELINES_NOTIFY_OPENED", "false") == "true",
			NotifyOnPRUpdated:   env("KUBEWATCHER_PIPELINES_NOTIFY_UPDATED", "false") == "true",
			NotifyOnPRCommented: env("KUBEWATCHER_PIPELINES_NOTIFY_COMMENTED", "false") == "true",
			NotifyOnPRMerged:    env("KUBEWATCHER_PIPELINES_NOTIFY_MERGED", "false") == "true",

			AutoCodeReview:        env("KUBEWATCHER_PIPELINES_AUTO_REVIEW", "false") == "true",
			CodeReviewDestination: env("KUBEWATCHER_PIPELINES_REVIEW_DEST", "local"),
			GDriveFolderID:        env("KUBEWATCHER_PIPELINES_GDRIVE_FOLDER", ""),
			CodeReviewS3Prefix:    env("KUBEWATCHER_PIPELINES_REVIEW_S3_PREFIX", "code-reviews/"),
			CodeReviewEmailTo:     env("KUBEWATCHER_PIPELINES_REVIEW_EMAIL_TO", ""),

			ConfluenceURL:          env("KUBEWATCHER_PIPELINES_CONFLUENCE_URL", ""),
			ConfluenceEmail:        env("KUBEWATCHER_PIPELINES_CONFLUENCE_EMAIL", ""),
			ConfluenceToken:        env("KUBEWATCHER_PIPELINES_CONFLUENCE_TOKEN", ""),
			ConfluenceSpaceKey:     env("KUBEWATCHER_PIPELINES_CONFLUENCE_SPACE", ""),
			ConfluenceParentPageID: env("KUBEWATCHER_PIPELINES_CONFLUENCE_PARENT", ""),

			NotionToken:      env("KUBEWATCHER_PIPELINES_NOTION_TOKEN", ""),
			NotionDatabaseID: env("KUBEWATCHER_PIPELINES_NOTION_DATABASE", ""),

			EvernoteToken:        env("KUBEWATCHER_PIPELINES_EVERNOTE_TOKEN", ""),
			EvernoteNotebookGUID: env("KUBEWATCHER_PIPELINES_EVERNOTE_NOTEBOOK", ""),

			OneNoteToken:     env("KUBEWATCHER_PIPELINES_ONENOTE_TOKEN", ""),
			OneNoteSectionID: env("KUBEWATCHER_PIPELINES_ONENOTE_SECTION", ""),

			AmplenoteAPIKey: env("KUBEWATCHER_PIPELINES_AMPLENOTE_KEY", ""),

			StandardNotesURL:   env("KUBEWATCHER_PIPELINES_STDNOTES_URL", "https://api.standardnotes.com"),
			StandardNotesToken: env("KUBEWATCHER_PIPELINES_STDNOTES_TOKEN", ""),

			ObsidianVaultPath: env("KUBEWATCHER_PIPELINES_OBSIDIAN_VAULT", ""),

			JoplinURL:   env("KUBEWATCHER_PIPELINES_JOPLIN_URL", "http://127.0.0.1:41184"),
			JoplinToken: env("KUBEWATCHER_PIPELINES_JOPLIN_TOKEN", ""),

			LogseqGraphPath: env("KUBEWATCHER_PIPELINES_LOGSEQ_GRAPH", ""),

			BearToken: env("KUBEWATCHER_PIPELINES_BEAR_TOKEN", ""),
		},
		Features: FeaturesConfig{
			Tier:       parseTier(env("KUBEWATCHER_TIER", "pro")), // pro for dev; production gates via license
			LicenseKey: env("KUBEWATCHER_LICENSE", ""),
		},
		Billing: BillingConfig{
			InputCostPer1M:  envFloat("KUBEWATCHER_BILLING_INPUT_PER_1M", 0),
			OutputCostPer1M: envFloat("KUBEWATCHER_BILLING_OUTPUT_PER_1M", 0),
			MonthlyBudget:   envFloat("KUBEWATCHER_BILLING_MONTHLY_BUDGET", 0),
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

func homeDir() string {
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return "."
}
