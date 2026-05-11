// Command server runs Argus in headless SaaS HTTP mode (no Wails GUI).
// This is the entry point used by the Docker image.
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/argues/argus/api/pkg"
	"github.com/argues/argus/internal/ai"
	"github.com/argues/argus/internal/anomaly"
	"github.com/argues/argus/internal/argocd"
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
	"github.com/argues/argus/internal/usage"
	"github.com/argues/argus/internal/vulnscan"
	"github.com/argues/argus/internal/workflows"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("argus-server: %v", err)
	}
}

func run() error {
	cfg, err := config.New()
	if err != nil {
		return err
	}

	logger := applogger.New(cfg)

	gate := features.NewGate(cfg)
	logger.Info("feature tier", slog.String("tier", string(cfg.Features.Tier)))

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

	argoCDClient := argocd.New(argocd.Config{
		URL:      cfg.ArgoCD.URL,
		Token:    cfg.ArgoCD.Token,
		Insecure: cfg.ArgoCD.Insecure,
	}, logger)
	if argoCDClient != nil {
		logger.Info("ArgoCD client initialized", slog.String("url", cfg.ArgoCD.URL))
	}

	notebooksStore, err := notebooks.New(cfg, logger)
	if err != nil {
		logger.Warn("notebooks store initialization failed", slog.String("error", err.Error()))
		notebooksStore, _ = notebooks.New(&config.OnlineDataConfig{}, logger)
	}

	runbooksStore, err := runbooks.New(notebooksStore, logger)
	if err != nil {
		logger.Warn("runbooks store initialization failed", slog.String("error", err.Error()))
	}

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
		Usage:           usageStore,
	})

	// Call Startup to initialise background goroutines (event loops, polling, etc.)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.Startup(ctx)

	port := 8080
	if p := os.Getenv("ARGUS_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	app.StartHTTPServer(port)
	logger.Info("Argus SaaS server running", slog.Int("port", port))

	// Block until SIGINT/SIGTERM.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	logger.Info("shutting down")
	app.Shutdown(ctx)
	return nil
}
