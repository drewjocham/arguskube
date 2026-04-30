package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"kube-watcher/mcp/api"
	"kube-watcher/mcp/monitoring/alerts"
	"kube-watcher/mcp/monitoring/history"
	"kube-watcher/mcp/server"
	"kube-watcher/pkg/audit"
	"kube-watcher/pkg/kube"
	"kube-watcher/pkg/kube/watch"
	"kube-watcher/pkg/logging"
)

var (
	version   = "1.0.0"
	gitCommit = "dev"
	buildDate = "unknown"
)

type config struct {
	showVersion, showHelp, listTools, healthCheck, healthAllowDegraded, debug bool
	execTool, toolArgs, logFile, dbPath                                       string
	interval                                                                  time.Duration
	httpAddr                                                                  string
}

func main() {
	cfg := parseFlags()

	if cfg.showVersion {
		fmt.Printf("kube-watcher v%s (commit: %s, built: %s)\n", version, gitCommit, buildDate)
		return
	}

	logger, err := logging.New(cfg.debug, cfg.logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logging.Shutdown()

	dbPath := expandPath(cfg.dbPath)
	handleErr(ensureDir(dbPath), "failed to create database directory", logger)

	baseClient, err := kube.NewClient(logger)
	handleErr(err, "kubernetes client init failed", logger)

	auditLogger := audit.NewSlogLogger(logger)
	k8sClient := kube.NewAuditClient(baseClient, auditLogger, logger, kube.AuditOptionsFromEnv()...)

	historyStore, err := history.NewStore(dbPath)
	handleErr(err, "storage init failed", logger)
	defer func() { _ = historyStore.Close() }()

	watchManager := watch.NewManager(k8sClient, logger, cfg.interval)

	alertsStore, err := alerts.NewStore(strings.TrimSuffix(dbPath, ".db") + "-alerts.db")
	handleErr(err, "alerts storage init failed", logger)
	defer func() { _ = alertsStore.Close() }()

	podTracker := watch.NewPodTracker(k8sClient.GetRawInterface(), logger)

	mcpServer, err := server.NewMCPServer(logger, server.Config{
		Version:      version,
		GitCommit:    gitCommit,
		BuildDate:    buildDate,
		K8sClient:    k8sClient,
		HistoryStore: historyStore,
		AlertsStore:  alertsStore,
		PodTracker:   podTracker,
		Watcher:      watchManager,
	})
	handleErr(err, "mcp server init failed", logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	executeAction(ctx, mcpServer, cfg, logger)
}

func parseFlags() config {
	c := config{}
	home, _ := os.UserHomeDir()
	defaultDB := filepath.Join(home, ".kube-watcher", "history.db")

	flag.BoolVar(&c.showVersion, "version", false, "display version information")
	flag.BoolVar(&c.showHelp, "help", false, "display help message")
	flag.BoolVar(&c.listTools, "list-tools", false, "list available MCP tools and exit")
	flag.StringVar(&c.execTool, "exec", "", "execute a specific tool by name")
	flag.StringVar(&c.toolArgs, "args", "{}", "JSON arguments for the tool execution")
	flag.BoolVar(&c.healthCheck, "health", false, "run a health check and exit")
	flag.BoolVar(&c.healthAllowDegraded, "health-allow-degraded", false, "with --health, exit 0 when Kubernetes is unreachable (degraded) — for CI/smoke without a cluster")
	flag.BoolVar(&c.debug, "debug", false, "enable verbose debug logging")
	flag.StringVar(&c.logFile, "log-file", "", "path to write logs (defaults to stderr)")
	flag.StringVar(&c.dbPath, "db-path", defaultDB, "path to the history database")
	flag.DurationVar(&c.interval, "interval", 30*time.Second, "polling interval for watchers")
	flag.StringVar(&c.httpAddr, "http-addr", "", "HTTP address to serve API (e.g., :8080)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "kube-watcher v%s\n", version)
		fmt.Fprintf(os.Stderr, "Usage: kube-watcher [options]\n\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if c.showHelp {
		flag.Usage()
		os.Exit(0)
	}

	return c
}

func executeAction(ctx context.Context, s *server.MCPServer, cfg config, logger *slog.Logger) {
	switch {
	case cfg.listTools:
		tools := s.ListTools()
		out, _ := json.MarshalIndent(tools, "", "  ")
		fmt.Println(string(out))

	case cfg.execTool != "":
		var args map[string]any
		if err := json.Unmarshal([]byte(cfg.toolArgs), &args); err != nil {
			logger.Error("invalid tool args JSON", "error", err)
			os.Exit(1)
		}
		res, err := s.ExecuteTool(ctx, cfg.execTool, args)
		if err != nil {
			logger.Error("tool execution failed", "tool", cfg.execTool, "error", err)
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(out))

	case cfg.healthCheck:
		h := s.HealthCheck(ctx)
		status, _ := h["status"].(string)
		if status != "healthy" {
			if cfg.healthAllowDegraded && status == "degraded" {
				logger.Warn("system degraded (Kubernetes check failed); exiting OK per --health-allow-degraded", "details", h)
				return
			}
			logger.Error("system unhealthy", "details", h)
			os.Exit(1)
		}
		logger.Info("system healthy")

	default:
		logger.Info("starting kube-watcher server", "version", version)
		if cfg.httpAddr != "" {
			apiInstance, err := api.New(api.Config{
				Server:  s,
				Logger:  logger,
				Version: version,
			})
			if err != nil {
				logger.Error("failed to create API instance", "error", err)
				os.Exit(1)
			}
			httpServer := &http.Server{
				Addr:    cfg.httpAddr,
				Handler: apiInstance.Routes(),
			}
			go func() {
				logger.Info("starting HTTP server", "addr", cfg.httpAddr)
				maxAttempts := 2
				for attempt := 1; attempt <= maxAttempts; attempt++ {
					err := httpServer.ListenAndServe()
					if err == nil || err == http.ErrServerClosed {
						// Normal shutdown, break out of retry loop
						break
					}
					// Log error with attempt number
					logger.Error("HTTP server error", "attempt", attempt, "error", err)
					if attempt == maxAttempts {
						logger.Warn("HTTP server failed after maximum retries, continuing in degraded mode (no HTTP API)")
						break
					}
					// Wait before retry
					retryDelay := 5 * time.Second
					logger.Info("retrying HTTP server start", "delay", retryDelay)
					time.Sleep(retryDelay)
					// Continue loop to retry
				}
			}()
			defer func() {
				logger.Info("shutting down HTTP server")
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = httpServer.Shutdown(shutdownCtx)
			}()
		}
		if err := s.Start(ctx); err != nil {
			logger.Error("server exit with error", "error", err)
			os.Exit(1)
		}
	}
}

func handleErr(err error, msg string, logger *slog.Logger) {
	if err != nil {
		if logger != nil {
			logger.Error(msg, "error", err)
		} else {
			fmt.Fprintf(os.Stderr, "FATAL: %s: %v\n", msg, err)
		}
		os.Exit(1)
	}
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}
