package pkg

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/argus/api-gov/internal/api"
	"github.com/argus/api-gov/internal/config"
	"github.com/argus/api-gov/internal/database"
)

const (
	DefaultServiceName = "api-gov"
)

type App struct {
	Config *config.APIGovConfig
	routes http.Handler
	server *api.API
	db     *database.DB
}

func New() (*App, error) {
	cfg := config.New()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	db, err := database.New(context.Background(), cfg.Database.URL)
	if err != nil {
		return nil, err
	}

	srv, err := api.New(cfg, logger, db)
	if err != nil {
		db.Close()
		return nil, err
	}

	return &App{
		Config: cfg,
		routes: srv.Routes(),
		server: srv,
		db:     db,
	}, nil
}

func (a *App) Routes() http.Handler {
	return a.routes
}

func (a *App) Start(ctx context.Context) error {
	return a.server.Start(a.Config.Server.Port)
}

func (a *App) Shutdown(ctx context.Context) error {
	a.db.Close()
	return a.server.Shutdown(ctx)
}
