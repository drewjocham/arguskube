package main

import (
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/argues/kube-watcher/api/pkg"
	"github.com/argues/kube-watcher/internal/ai"
	"github.com/argues/kube-watcher/internal/config"
	applogger "github.com/argues/kube-watcher/internal/logger"
	"github.com/argues/kube-watcher/internal/usage"
	"github.com/argues/kube-watcher/view"
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

	// For the standalone terminal, we only strictly need the PTY/terminal integrations
	// and potentially the AI Agent for the "Magic Wand" features.
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

	// Start the Wails instance heavily customized to mimic a modern frameless terminal (e.g., Warp/Raycast)
	return wails.Run(&options.App{
		Title:             appTitle,
		Width:             appWidth,
		Height:            appHeight,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         true,  // Remove OS chrome completely
		AlwaysOnTop:       false,
		AssetServer: &assetserver.Options{
			Assets: view.FS,
		},
		BackgroundColour:  &options.RGBA{R: 0, G: 0, B: 0, A: 0}, // fully transparent backend
		OnStartup:         app.Startup,
		OnShutdown:        app.Shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               true,
				FullSizeContent:            true,
				UseToolbar:                 false,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true, // Glassmorphism
			Appearance:           mac.NSAppearanceNameDarkAqua,
		},
	})
}
