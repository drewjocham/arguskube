package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/argues/argus/api/pkg"
	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/config"
	applogger "github.com/argues/argus/internal/logger"
	"github.com/argues/argus/internal/usage"
	"github.com/argues/argus/view"
)

const (
	appTitle  = "Argus Terminal"
	appWidth  = 1000
	appHeight = 650
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("argus-terminal: %v", err)
	}
}

func run() error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	logger := applogger.New(cfg)

	usageStore, _ := usage.New()

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
				_ = usageStore.Record(usage.Record{
					Model:            model,
					PromptTokens:     in,
					CompletionTokens: out,
				})
			})
		}
		agent = ai.NewAgent(dsClient, logger)
		logger.Info("AI agent initialized for terminal context")
	}

	app := pkg.NewApp(pkg.AppConfig{
		Logger:  logger,
		Config:  cfg,
		Agent:   agent,
		Usage:   usageStore,
		AppMode: "terminal",
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("signal received — cleaning up terminal sessions")
		app.Shutdown(nil)
		os.Exit(0)
	}()

	return wails.Run(&options.App{
		Title:         appTitle,
		Width:         appWidth,
		Height:        appHeight,
		MinWidth:      640,
		MinHeight:     400,
		DisableResize: false,
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
				Title:   "Argus Terminal",
				Message: "© 2026 Argus Infrastructure",
			},
		},
	})
}
