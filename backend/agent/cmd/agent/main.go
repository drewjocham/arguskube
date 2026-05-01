package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/argues/kube-watcher/agent/internal/config"
	"github.com/argues/kube-watcher/agent/internal/k8s"
	"github.com/argues/kube-watcher/agent/internal/server"
	"github.com/argues/kube-watcher/agent/internal/tunnel"
	"golang.org/x/sync/errgroup"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.New(ctx)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	logger.Info("Starting KubeWatcher In-Cluster Agent")

	var tunnelClient *tunnel.Client
	if cfg.SaaSToken == "" {
		logger.Warn("SAAS_TOKEN not set. Running in local-only mode.")
	} else {
		logger.Info("SaaS Token detected. Will sync metadata to cloud.")
		tunnelClient = tunnel.NewClient(cfg.SaaSServerURL, cfg.SaaSToken, "default", logger.With("component", "tunnel"))
	}

	k8sClient, err := k8s.NewClient(ctx, logger.With("component", "k8s"))
	if err != nil {
		return err
	}

	srv := server.New(cfg.ServerPort, k8sClient, logger.With("component", "server"))

	eg, egCtx := errgroup.WithContext(ctx)

	if tunnelClient != nil {
		eg.Go(func() error {
			if err := tunnelClient.Start(egCtx); err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		})
	}

	eg.Go(func() error {
		if err := k8sClient.StartInformers(egCtx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})

	eg.Go(func() error {
		if err := srv.Start(egCtx); err != nil {
			return err
		}
		return nil
	})

	return eg.Wait()
}
