package pkg

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/argues/kube-watcher/internal/agentanalysis"
	"github.com/argues/kube-watcher/internal/agentconn"
	"github.com/argues/kube-watcher/internal/ai"
	"github.com/argues/kube-watcher/internal/alertproc"
	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/argues/kube-watcher/internal/anomaly"
	"github.com/argues/kube-watcher/internal/argocd"
	"github.com/argues/kube-watcher/internal/config"
	ctxassembly "github.com/argues/kube-watcher/internal/context"
	"github.com/argues/kube-watcher/internal/features"
	"github.com/argues/kube-watcher/internal/incidents"
	"github.com/argues/kube-watcher/internal/k8s"
	"github.com/argues/kube-watcher/internal/notebooks"
	"github.com/argues/kube-watcher/internal/popeye"
	"github.com/argues/kube-watcher/internal/runbooks"
	"github.com/argues/kube-watcher/internal/setup"
	"github.com/argues/kube-watcher/internal/spotcheck"
	"github.com/argues/kube-watcher/internal/terminal"
	"github.com/argues/kube-watcher/internal/usage"
	"github.com/argues/kube-watcher/internal/vulnscan"
	"github.com/argues/kube-watcher/internal/workflows"
)

// AppConfig holds all dependencies for the application. Flat, explicit.
type AppConfig struct {
	Logger          *slog.Logger
	Config          *config.OnlineDataConfig
	K8sClient       *k8s.Client
	Gate            *features.Gate
	Assembler       *ctxassembly.Assembler
	Detector        anomaly.Detector
	AnomalySettings *anomaly.SettingsStore
	Agent           *ai.Agent
	Popeye          *popeye.Runner
	Scanner         *vulnscan.Scanner
	ArgoCD          *argocd.Client
	Notebooks       *notebooks.Store
	Runbooks        *runbooks.Store
	Setup           *setup.Manager
	Incidents       *incidents.Store
	Workflows       *workflows.Store
	Usage           *usage.Store
	DB              *sql.DB
	AppMode         string
}

// App is the main application struct exposed to Wails as bindings.
type App struct {
	ctx             context.Context
	logger          *slog.Logger
	cfg             *config.OnlineDataConfig
	k8s             *k8s.Client
	gate            *features.Gate
	assembler       *ctxassembly.Assembler
	detector        anomaly.Detector
	anomalySettings *anomaly.SettingsStore
	agent           *ai.Agent
	periodicAgent   *agentanalysis.Agent
	popeye          *popeye.Runner
	scanner         *vulnscan.Scanner
	argoCD          *argocd.Client
	notebooks       *notebooks.Store
	runbooks        *runbooks.Store
	agentConn       *agentconn.Connector
	term            *terminal.Terminal
	setup           *setup.Manager
	incidents       *incidents.Store
	hub             *Hub
	workflows       *workflows.Store
	usage           *usage.Store

	// routesTbl is the typed HTTP dispatch table populated lazily on first
	// /api/* request. Built per-App (see ensureRoutes) so tests and any
	// future multi-tenant deployment don't share closures across instances.
	routesOnce sync.Once
	routesTbl  map[string]apiHandler

	appMode string

	// execSession holds the active kubectl exec session (one at a time).
	execSession *k8s.PodExecSession
	execMu      sync.RWMutex

	// paused stops background polling when the window is hidden/minimized.
	paused atomic.Bool

	// cachedMetrics holds the latest metrics for agent context.
	cachedMetrics *alerts.ClusterMetrics

	// webhookAlerts stores alerts received via the /webhooks/anomstack endpoint.
	webhookAlerts []alerts.Alert
	webhookMu     sync.RWMutex

	// auth gates /api/* on a valid session. nil until SetupAuth runs.
	auth *authState

	// spotcheck runs periodic cluster probes (nodes, metrics, docs
	// freshness) and emits findings via the notifications channel.
	// nil until startSpotChecks runs in Startup.
	spotcheck *spotcheck.Engine

	// alertproc dedupes alerts, runs the AI agent on new firings,
	// tracks silences/ignores, and fires the "alerts losing value"
	// meta-alert when noise crosses the user's threshold.
	alertproc *alertproc.Processor

	// db is the shared SQLite handle. Held here so the alertproc and
	// any future per-app stores can persist without re-opening.
	db *sql.DB
}

// NewApp constructs and initializes the main application.
func NewApp(ac AppConfig) *App {
	app := &App{
		ctx:             context.Background(),
		logger:          ac.Logger,
		cfg:             ac.Config,
		k8s:             ac.K8sClient,
		gate:            features.NewGate(ac.Config),
		assembler:       ac.Assembler,
		detector:        ac.Detector,
		anomalySettings: ac.AnomalySettings,
		agent:           ac.Agent,
		popeye:          ac.Popeye,
		scanner:         ac.Scanner,
		argoCD:          ac.ArgoCD,
		notebooks:       ac.Notebooks,
		runbooks:        ac.Runbooks,
		term:            terminal.New(ac.Logger),
		setup:           ac.Setup,
		incidents:       ac.Incidents,
		workflows:       ac.Workflows,
		db:              ac.DB,
		usage:           ac.Usage,
		appMode:         ac.AppMode,
		hub:             NewHub(ac.Logger.With("component", "saas-hub")),
	}

	// Initialize agent connector if k8s client is available.
	if ac.K8sClient != nil {
		app.agentConn = agentconn.New(
			ac.K8sClient.GetClientset(),
			ac.K8sClient.GetRestConfig(),
			ac.Logger.With("component", "agentconn"),
		)
	}

	return app
}

// GetAppMode returns the frontend display mode (e.g., 'dashboard' or 'terminal').
func (a *App) GetAppMode() string {
	if a.appMode == "" {
		return "dashboard"
	}
	return a.appMode
}

// Startup is called by Wails when the app starts. Stores the runtime context
// and kicks off background event polling.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.logger.InfoContext(ctx, "kubewatcher started",
		slog.String("tier", string(a.cfg.Features.Tier)),
	)

	// Eagerly populate cached metrics so AI agent has context from first message.
	if a.k8s != nil {
		if m, err := a.k8s.GetMetrics(ctx); err == nil && m != nil {
			a.cachedMetrics = m
			a.logger.Info("cached initial cluster metrics")
		}
	}

	// Boot the alert processor before the event loop fires so the
	// very first DetectAlerts call already deduplicates + runs the
	// agent's investigations.
	a.startAlertProcessor()

	a.StartEventLoop(ctx)

	// Start cluster spot-checks: periodic node / metrics / docs probes
	// that emit findings into the notifications channel. Cheap enough
	// to always run; gated by a 30-min timer so it doesn't hammer the
	// API server.
	a.startSpotChecks()

	a.periodicAgent = agentanalysis.NewAgent(a.logger, a.cfg, a.ctx)
	go a.periodicAgent.StartLoop(a.ctx)
}

// Shutdown is called by Wails when the app closes.
func (a *App) Shutdown(ctx context.Context) {
	a.closeExecSession()
	a.term.Close()
	a.logger.InfoContext(ctx, "kubewatcher shutting down")
}

// SetPaused pauses or resumes background polling (alerts, metrics, logs).
// Called from the frontend when the window visibility changes.
func (a *App) SetPaused(paused bool) {
	a.paused.Store(paused)
	if a.logger != nil {
		if paused {
			a.logger.Info("event loop paused (window hidden)")
		} else {
			a.logger.Info("event loop resumed (window visible)")
		}
	}
}

var errNoCluster = fmt.Errorf("no cluster connected — check kubeconfig")
