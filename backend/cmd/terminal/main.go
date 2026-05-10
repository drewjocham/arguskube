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

	// Standalone terminal — a normal resizable Wails window with traffic
	// lights. We dropped the Frameless/translucent treatment because it
	// removed the macOS chrome the user expects from a "real app" (resize
	// handles, drag, traffic lights). Looks consistent with the dashboard
	// and is a lot less surprising.
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
				Title:   "KubeWatcher Terminal",
				Message: "© 2026 Argus Infrastructure",
			},
		},
	})
}
