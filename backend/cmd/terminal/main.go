package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/djocham/kube-watcher/api/pkg"
	"github.com/djocham/kube-watcher/internal/ai"
	"github.com/djocham/kube-watcher/internal/config"
	applogger "github.com/djocham/kube-watcher/internal/logger"
)

//go:embed all:view/dist
var assets embed.FS

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
	var agent *ai.Agent
	if cfg.AI.DeepSeekAPIKey != "" {
		dsClient := ai.NewDeepSeekClient(cfg.AI.DeepSeekAPIKey, logger)
		agent = ai.NewAgent(dsClient, logger)
		logger.Info("AI agent initialized for terminal context")
	}

	app := pkg.NewApp(pkg.AppConfig{
		Logger:  logger,
		Config:  cfg,
		Agent:   agent,
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
			Assets: assets,
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
