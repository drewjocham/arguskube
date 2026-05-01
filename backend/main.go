package main

import (
	"log"
	"log/slog"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/djocham/kube-watcher/api/pkg"
	"github.com/djocham/kube-watcher/internal/ai"
	"github.com/djocham/kube-watcher/internal/anomaly"
	"github.com/djocham/kube-watcher/internal/config"
	ctxassembly "github.com/djocham/kube-watcher/internal/context"
	"github.com/djocham/kube-watcher/internal/features"
	"github.com/djocham/kube-watcher/internal/incidents"
	"github.com/djocham/kube-watcher/internal/k8s"
	applogger "github.com/djocham/kube-watcher/internal/logger"
	"github.com/djocham/kube-watcher/internal/notebooks"
	"github.com/djocham/kube-watcher/internal/popeye"
	"github.com/djocham/kube-watcher/internal/runbooks"
	"github.com/djocham/kube-watcher/internal/setup"
	"github.com/djocham/kube-watcher/view"
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

	incidentStore := incidents.NewStore("", logger)

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
		Detector:  detector,
		Agent:     agent,
		Popeye:    popeyeRunner,
		Notebooks: notebooksStore,
		Runbooks:  runbooksStore,
		Incidents: incidentStore,
		Setup:     setupMgr,
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
