package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/riandyrn/otelchi"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/djocham/kube-watcher/mcp/server"
)

const (
	prefix             = "/v1"
	defaultServiceName = "kube-watcher-api"
	maxRequestBodySize = 1 << 20
)

type Checker interface {
	Check(context.Context) error
}

type Config struct {
	Server           *server.MCPServer
	Logger           *slog.Logger
	ServiceName      string
	LivenessChecker  Checker
	ReadinessChecker Checker
	Tracer           trace.Tracer
	Version          string
}

type API struct {
	server      *server.MCPServer
	logger      *slog.Logger
	tracer      trace.Tracer
	serviceName string
	version     string
	liveness    Checker
	readiness   Checker
}

func New(cfg Config) (*API, error) {
	if cfg.Server == nil {
		return nil, errors.New("api: mcp server is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = defaultServiceName
	}
	if cfg.Tracer == nil {
		cfg.Tracer = otel.Tracer(cfg.ServiceName)
	}

	return &API{
		server:      cfg.Server,
		logger:      cfg.Logger,
		tracer:      cfg.Tracer,
		serviceName: cfg.ServiceName,
		version:     cfg.Version,
		liveness:    cfg.LivenessChecker,
		readiness:   cfg.ReadinessChecker,
	}, nil
}

func (a *API) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(a.recoveryMiddleware)
	r.Use(middleware.Logger)
	r.Use(otelchi.Middleware(a.serviceName, otelchi.WithChiRoutes(r)))

	r.Get("/", a.handleRoot)
	r.Get("/healthz", a.handleHealth)
	r.Get("/readyz", a.handleReady)

	r.Route(prefix, func(r chi.Router) {
		r.Get("/status", a.handleStatus)
		r.Get("/tools", a.handleListTools)
		r.Post("/tools/{tool}", a.handleExecuteTool)
		r.Get("/alerts", a.handleAlerts)
		r.Put("/alerts/{id}/state", a.handleUpdateAlertState)
		r.Post("/alerts/{id}/comments", a.handleAddAlertComment)
		r.Get("/history", a.handleHistory)
		r.Get("/recommendations", a.handleRecommendations)

	})

	return r
}

func (a *API) handleRoot(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "kube-watcher MCP API (%s)", a.serviceName)
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	a.respond(w, r, http.StatusOK, map[string]string{
		"service": a.serviceName,
		"version": a.version,
		"status":  "ok",
	})
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	if a.liveness != nil {
		if err := a.liveness.Check(r.Context()); err != nil {
			a.respondError(w, r, http.StatusServiceUnavailable, "liveness probe failed", err)
			return
		}
	}
	a.respond(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleReady(w http.ResponseWriter, r *http.Request) {
	if a.readiness != nil {
		if err := a.readiness.Check(r.Context()); err != nil {
			a.respondError(w, r, http.StatusServiceUnavailable, "readiness probe failed", err)
			return
		}
	}
	a.respond(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleListTools(w http.ResponseWriter, r *http.Request) {
	tools := a.server.ToolSummaries()
	a.respond(w, r, http.StatusOK, map[string]any{"tools": tools})
}

func (a *API) handleExecuteTool(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	toolName := chi.URLParam(r, "tool")

	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("tool.name", toolName))

	args, err := decodeJSON[map[string]any](r)
	if err != nil {
		a.respondError(w, r, http.StatusBadRequest, "invalid request body", err)
		return
	}

	result, err := a.server.ExecuteTool(ctx, toolName, args)
	if err != nil {
		if errors.Is(err, server.ErrToolNotFound) {
			a.respondError(w, r, http.StatusNotFound, "tool not found", err)
			return
		}
		a.respondError(w, r, http.StatusInternalServerError, "tool execution failed", err)
		return
	}

	a.respond(w, r, http.StatusOK, result)
}

func (a *API) handleAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := a.server.AlertsSnapshot(r.Context())
	a.respond(w, r, http.StatusOK, map[string]any{"alerts": alerts})
}

func (a *API) handleUpdateAlertState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	req, err := decodeJSON[struct {
		State string `json:"state"`
	}](r)
	if err != nil {
		a.respondError(w, r, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if req.State == "" {
		a.respondError(w, r, http.StatusBadRequest, "state is required", nil)
		return
	}

	if err := a.server.UpdateAlertState(ctx, id, req.State); err != nil {
		a.respondError(w, r, http.StatusInternalServerError, "failed to update alert state", err)
		return
	}
	a.respond(w, r, http.StatusOK, map[string]any{"updated": true})
}

func (a *API) handleAddAlertComment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	req, err := decodeJSON[struct {
		Author  string `json:"author"`
		Content string `json:"content"`
	}](r)
	if err != nil {
		a.respondError(w, r, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if req.Author == "" || req.Content == "" {
		a.respondError(w, r, http.StatusBadRequest, "author and content are required", nil)
		return
	}

	if err := a.server.AddAlertComment(ctx, id, req.Author, req.Content); err != nil {
		a.respondError(w, r, http.StatusInternalServerError, "failed to add comment", err)
		return
	}
	a.respond(w, r, http.StatusOK, map[string]any{"added": true})
}

func (a *API) handleHistory(w http.ResponseWriter, r *http.Request) {
	window := 72 * time.Hour
	if q := r.URL.Query().Get("window"); q != "" {
		if d, err := time.ParseDuration(q); err == nil {
			window = d
		}
	}

	records, err := a.server.IncidentHistory(r.Context(), window)
	if err != nil {
		a.respondError(w, r, http.StatusInternalServerError, "failed to fetch history", err)
		return
	}

	a.respond(w, r, http.StatusOK, map[string]any{
		"window":  window.String(),
		"records": records,
	})
}

func (a *API) respond(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func (a *API) respondError(w http.ResponseWriter, r *http.Request, status int, msg string, err error) {
	a.logger.ErrorContext(r.Context(), msg, "error", err, "path", r.URL.Path)

	span := trace.SpanFromContext(r.Context())
	span.RecordError(err)
	span.SetStatus(codes.Error, msg)

	a.respond(w, r, status, map[string]string{
		"error": msg,
	})
}

func (a *API) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				a.logger.ErrorContext(r.Context(), "panic recovered",
					"error", rec,
					"stack", string(debug.Stack()),
				)
				a.respondError(w, r, http.StatusInternalServerError, "internal server error", fmt.Errorf("%v", rec))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func decodeJSON[T any](r *http.Request) (T, error) {
	var val T
	if r.Body == nil || r.Body == http.NoBody {
		return val, nil
	}
	defer func() { _ = r.Body.Close() }()

	err := json.NewDecoder(io.LimitReader(r.Body, maxRequestBodySize)).Decode(&val)
	return val, err
}
