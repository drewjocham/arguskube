package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/argues/kube-watcher/api/pkg"
	"github.com/argues/kube-watcher/internal/ai"
	"github.com/argues/kube-watcher/internal/argocd"
	"github.com/argues/kube-watcher/internal/anomaly"
	"github.com/argues/kube-watcher/internal/config"
	ctxassembly "github.com/argues/kube-watcher/internal/context"
	"github.com/argues/kube-watcher/internal/features"
	"github.com/argues/kube-watcher/internal/incidents"
	"github.com/argues/kube-watcher/internal/k8s"
	"github.com/argues/kube-watcher/internal/sqlitedb"
	applogger "github.com/argues/kube-watcher/internal/logger"
	"github.com/argues/kube-watcher/internal/notebooks"
	"github.com/argues/kube-watcher/internal/popeye"
	"github.com/argues/kube-watcher/internal/runbooks"
	"github.com/argues/kube-watcher/internal/setup"
	"github.com/argues/kube-watcher/internal/vulnscan"
	"github.com/argues/kube-watcher/internal/usage"
	"github.com/argues/kube-watcher/internal/workflows"
	"github.com/argues/kube-watcher/view"
)

const (
	appTitle  = "KubeWatcher — SRE Console"
	appWidth  = 1440
	appHeight = 800
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("kubewatcher: %v", err)
	}
}

func run() error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	logger := applogger.New(cfg)

	gate := features.NewGate(cfg)
	logger.Info("feature tier",
		slog.String("tier", string(cfg.Features.Tier)),
	)

	k8sClient, err := k8s.NewClient(cfg, logger)
	if err != nil {
		logger.Warn("k8s connection failed — app will start without cluster data",
			slog.String("error", err.Error()),
		)
	}

	var detector anomaly.Detector
	if cfg.AI.AnomstackURL != "" {
		detector = anomaly.NewAnomstackClient(cfg, logger)
	}

	assembler := ctxassembly.NewAssembler(cfg, gate, detector, logger)

	usageStore, err := usage.New()
	if err != nil {
		logger.Warn("usage tracking disabled — failed to open store",
			slog.String("error", err.Error()))
		usageStore = nil
	}

	var agent *ai.Agent
	if cfg.AI.DeepSeekAPIKey != "" {
		dsClient := ai.NewOpenAICompatibleClient(
			cfg.AI.DeepSeekAPIKey,
			cfg.AI.LLMBaseURL,
			cfg.AI.LLMModel,
			logger,
		)
		if usageStore != nil {
			dsClient.SetUsageRecorder(func(model string, in, out int) {
				if err := usageStore.Record(usage.Record{
					Model:            model,
					PromptTokens:     in,
					CompletionTokens: out,
				}); err != nil {
					logger.Warn("usage record dropped", slog.String("error", err.Error()))
				}
			})
		}
		agent = ai.NewAgent(dsClient, logger)
		if cfg.AI.LLMBaseURL != "" {
			logger.Info("AI agent initialized (self-hosted)",
				slog.String("base_url", cfg.AI.LLMBaseURL),
				slog.String("model", cfg.AI.LLMModel),
			)
		} else {
			logger.Info("AI agent initialized (DeepSeek)")
		}
	} else {
		logger.Warn("AI agent disabled — set DEEPSEEK_API_KEY (or LLM bearer) to enable")
	}

	popeyeRunner := popeye.NewRunner(
		cfg.AI.PopeyeBinary,
		cfg.Kubernetes.Config,
		cfg.Kubernetes.Context,
		cfg.Kubernetes.Namespace,
		logger,
	)

	var scanner *vulnscan.Scanner
	if k8sClient != nil {
		scanner = vulnscan.New(k8sClient.GetClientset(), logger)
	}

	// Argo CD client — nil if not configured.
	argoCDClient := argocd.New(argocd.Config{
		URL:      cfg.ArgoCD.URL,
		Token:    cfg.ArgoCD.Token,
		Insecure: cfg.ArgoCD.Insecure,
	}, logger)
	if argoCDClient != nil {
		logger.Info("ArgoCD client initialized", slog.String("url", cfg.ArgoCD.URL))
	} else {
		logger.Info("ArgoCD not configured — set ARGOCD_URL and ARGOCD_TOKEN to enable")
	}

	notebooksStore, err := notebooks.New(cfg, logger)
	if err != nil {
		logger.Warn("notebooks store initialization failed",
			slog.String("error", err.Error()),
		)
		notebooksStore, _ = notebooks.New(&config.OnlineDataConfig{}, logger) // fallback to local-only mode
	}

	runbooksStore, err := runbooks.New(notebooksStore, logger)
	if err != nil {
		logger.Warn("runbooks store initialization failed",
			slog.String("error", err.Error()),
		)
	}

	// Open shared SQLite database for local persistence.
	db, err := sqlitedb.Open("", logger)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	incidentStore := incidents.NewStore(db.DB, logger)

	workflowStore, err := workflows.New(db.DB, logger)
	if err != nil {
		logger.Warn("workflow store initialization failed", slog.String("error", err.Error()))
	}

	anomalySettings, err := anomaly.NewSettingsStore(db.DB, logger)
	if err != nil {
		logger.Warn("anomaly settings store initialization failed", slog.String("error", err.Error()))
	}

	setupMgr := setup.NewManager(
		cfg.Kubernetes.Config,
		cfg.Kubernetes.Context,
		cfg.Kubernetes.Namespace,
		logger,
	)

	app := pkg.NewApp(pkg.AppConfig{
		Logger:    logger,
		Config:    cfg,
		K8sClient: k8sClient,
		Gate:      gate,
		Assembler: assembler,
		Detector:        detector,
		AnomalySettings: anomalySettings,
		Agent:           agent,
		Popeye:    popeyeRunner,
		Scanner:   scanner,
		ArgoCD:    argoCDClient,
		Notebooks: notebooksStore,
		Runbooks:  runbooksStore,
		Incidents: incidentStore,
		Workflows: workflowStore,
		Setup:     setupMgr,
		Usage:     usageStore,
	})

	// Start the SaaS API server so the frontend can communicate without Wails
	app.StartHTTPServer(8080)

	return wails.Run(&options.App{
		Title:     appTitle,
		Width:     appWidth,
		Height:    appHeight,
		MinWidth:  1024,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: view.FS,
		},
		BackgroundColour: &options.RGBA{R: 26, G: 28, B: 30, A: 255}, 
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "KubeWatcher",
				Message: "© 2026 Argus Infrastructure",
			},
		},
	})
}
