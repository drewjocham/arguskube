package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gochi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/argues/argus/alert-ingress/internal/pubsub"
	"github.com/argues/argus/alert-ingress/internal/webhook"
)

const ingressPathWebhookAnomstack = "/webhooks/anomstack"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisher := initPublisher(ctx, logger)
	defer publisher.Close()

	r := gochi.NewRouter()
	r.Use(middleware.CleanPath)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Mount(ingressPathWebhookAnomstack, webhook.New(publisher, logger))

	port := os.Getenv("ALERT_INGRESS_PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      withLogging(r, logger),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		logger.Info("alert-ingress listening", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	logger.Info("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		os.Exit(1)
	}
}

func initPublisher(ctx context.Context, logger *slog.Logger) pubsub.Publisher {
	mode := os.Getenv("ALERT_INGRESS_MODE")
	switch mode {
	case "gcp":
		p, err := pubsub.NewGCP(ctx, logger, "", "")
		if err != nil {
			logger.Error("gcp pubsub init failed", "error", err)
			os.Exit(1)
		}
		return p
	default:
		logger.Info("alert-ingress running in stdout mode (set ALERT_INGRESS_MODE=gcp for PubSub)")
		return pubsub.NewStdout(logger)
	}
}

func withLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}
