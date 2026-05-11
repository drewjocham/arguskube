package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/argues/argus/api/pkg"
	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/anomaly"
	"github.com/argues/argus/internal/argocd"
	"github.com/argues/argus/internal/auth"
	"github.com/argues/argus/internal/config"
	ctxassembly "github.com/argues/argus/internal/context"
	"github.com/argues/argus/internal/features"
	"github.com/argues/argus/internal/incidents"
	"github.com/argues/argus/internal/k8s"
	applogger "github.com/argues/argus/internal/logger"
	"github.com/argues/argus/internal/notebooks"
	"github.com/argues/argus/internal/popeye"
	"github.com/argues/argus/internal/runbooks"
	"github.com/argues/argus/internal/setup"
	"github.com/argues/argus/internal/sqlitedb"
	"github.com/argues/argus/internal/vulnscan"
	"github.com/argues/argus/internal/workflows"
	"github.com/argues/argus/view"
)

const (
	appTitle  = "Argus — SRE Console"
	appWidth  = 1440
	appHeight = 800
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("argus: %v", err)
	}
}

// loadDotenv reads a .env file from the working directory or the
// repo root and exports its values, but never overrides values
// already set in the real environment. Silent on a missing file —
// the file is optional for desktop dev. Real env wins so CI, Make
// targets, and shell exports continue to take precedence.
func loadDotenv() {
	candidates := []string{
		".env",
		filepath.Join("..", ".env"), // running from backend/ during `wails dev`
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			_ = godotenv.Load(p)
			return
		}
	}
}

func run() error {
	loadDotenv()

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
	switch {
	case cfg.AI.FlinkURL != "":
		detector = anomaly.NewFlinkClient(cfg, logger)
		logger.Info("anomaly detector initialized", slog.String("backend", "flink"), slog.String("url", cfg.AI.FlinkURL))
	case cfg.AI.AnomstackURL != "":
		detector = anomaly.NewAnomstackClient(cfg, logger)
		logger.Info("anomaly detector initialized", slog.String("backend", "anomstack"), slog.String("url", cfg.AI.AnomstackURL))
	default:
		logger.Warn("anomaly detection disabled — set ARGUS_FLINK_URL or ANOMSTACK_URL to enable")
	}

	assembler := ctxassembly.NewAssembler(cfg, gate, detector, logger)

	var agent *ai.Agent
	if cfg.AI.DeepSeekAPIKey != "" {
		dsClient := ai.NewDeepSeekClient(cfg.AI.DeepSeekAPIKey, logger)
		agent = ai.NewAgent(dsClient, logger)
		logger.Info("AI agent initialized (DeepSeek)")
	} else {
		logger.Warn("AI agent disabled — set DEEPSEEK_API_KEY to enable")
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

	// ARGUS_MODE controls which view this binary boots into. When the
	// user clicks "Pop out" in the dashboard, the app spawns itself with
	// MODE=terminal so the new process opens its own OS-level window with
	// just the terminal. Default is "dashboard".
	appMode := os.Getenv("ARGUS_MODE")
	if appMode == "" {
		appMode = "dashboard"
	}

	windowTitle := appTitle
	winW, winH := appWidth, appHeight
	if appMode == "terminal" {
		windowTitle = "Argus Terminal"
		winW, winH = 960, 600
	}

	app := pkg.NewApp(pkg.AppConfig{
		Logger:          logger,
		Config:          cfg,
		K8sClient:       k8sClient,
		Gate:            gate,
		Assembler:       assembler,
		Detector:        detector,
		AnomalySettings: anomalySettings,
		Agent:           agent,
		Popeye:          popeyeRunner,
		Scanner:         scanner,
		ArgoCD:          argoCDClient,
		Notebooks:       notebooksStore,
		Runbooks:        runbooksStore,
		Incidents:       incidentStore,
		Workflows:       workflowStore,
		Setup:           setupMgr,
		DB:              db.DB,
		AppMode:         appMode,
	})

	// Wire the user-account / OAuth subsystem. Without this, every
	// /api/* call returns 401 — by design, since the spec is "no
	// account = no access".
	authStore := auth.NewStore(db.DB, logger.With(slog.String("component", "auth")))
	app.SetupAuth(authStore, cfg.Auth)

	// Start the SaaS API server so the frontend can communicate without Wails
	app.StartHTTPServer(8080)

	return wails.Run(&options.App{
		Title:     windowTitle,
		Width:     winW,
		Height:    winH,
		MinWidth:  640,
		MinHeight: 400,
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
				Title:   "Argus",
				Message: "© 2026 Argus Infrastructure",
			},
		},
	})
}
