package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/argus/api-gov/pkg"
)

const (
	shutdownDuration = 10 * time.Second
	logKeyErr        = "err"
	logAppExit       = "application exited with error"
	logShutdownMsg   = "shutting down server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := pkg.New()
	if err != nil {
		slog.Error("failed to initialize application", logKeyErr, err)
		os.Exit(1)
	}

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		slog.Info("starting HTTP server", "port", app.Config.Server.Port)
		if err := app.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	<-ctx.Done()
	slog.Info(logShutdownMsg)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownDuration)
	defer cancel()

	if err := app.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", logKeyErr, err)
	}

	if err := eg.Wait(); err != nil {
		slog.Error(logAppExit, logKeyErr, err)
		os.Exit(1)
	}
}
