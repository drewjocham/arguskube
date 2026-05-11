package api

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
		"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/argus/api-gov/internal/apperrors"
	"github.com/argus/api-gov/internal/config"
	"github.com/argus/api-gov/internal/database"
	"github.com/argus/api-gov/internal/service"
)

const (
	prefix              = "/api/v1"
	defaultReadTimeout  = 15 * time.Second
	defaultWriteTimeout = 60 * time.Second
	defaultIdleTimeout  = 120 * time.Second
	defaultBodyLimit    = 10 << 20

	logKeyError          = "error"
	logKeyStack          = "stack"
	logMsgPanicRecovered = "panic recovered"
)

type API struct {
	Config   *config.APIGovConfig
	Logger   *slog.Logger
	tracer   trace.Tracer
	meter    metric.Meter
	specSvc  *service.SpecService
	driftSvc *service.DriftService
	agentCli *service.AgentClient
	server   *http.Server

	counterSpecsCreated    metric.Int64Counter
	counterDriftResolved   metric.Int64Counter
	counterTrafficIngested metric.Int64Counter
	histogramLatency       metric.Float64Histogram
}

func New(cfg *config.APIGovConfig, logger *slog.Logger, db *database.DB) (*API, error) {
	if cfg == nil {
		return nil, apperrors.ErrServerConfigNil
	}

	specStore := database.NewSpecStore(db)
	endpointStore := database.NewEndpointStore(db)
	driftStore := database.NewDriftStore(db)

	specSvc := service.NewSpecService(specStore, endpointStore, logger)
	driftSvc := service.NewDriftService(driftStore, specStore, logger)
	agentCli := service.NewAgentClient(cfg.Agent.ServiceURL, cfg.LLM.DriftThreshold, logger)

	tracer := otel.Tracer("api-gov-backend")
	meter := otel.Meter("api-gov-backend")

	counterSpecsCreated, _ := meter.Int64Counter("api-gov.specs.created", metric.WithDescription("Total specs created"))
	counterDriftResolved, _ := meter.Int64Counter("api-gov.drift.resolved", metric.WithDescription("Drift reports resolved"))
	counterTrafficIngested, _ := meter.Int64Counter("api-gov.traffic.ingested", metric.WithDescription("Traffic events ingested"))
	histogramLatency, _ := meter.Float64Histogram("api-gov.api.latency", metric.WithDescription("API handler latency"))

	return &API{
		Config:               cfg,
		Logger:               logger,
		tracer:               tracer,
		meter:                meter,
		specSvc:              specSvc,
		driftSvc:             driftSvc,
		agentCli:             agentCli,
		counterSpecsCreated:  counterSpecsCreated,
		counterDriftResolved: counterDriftResolved,
		counterTrafficIngested: counterTrafficIngested,
		histogramLatency:     histogramLatency,
	}, nil
}

func (a *API) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.CleanPath)
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Logger, middleware.Recoverer)
	r.Use(a.WithCORS)
	r.Use(a.WithContentTypeJSON)
	r.Use(a.WithBodyLimit(defaultBodyLimit))
	r.Use(a.WithTimeout(30 * time.Second))

	r.Get("/health", a.handleHealth)
	r.Get("/ready", a.handleReadiness)

	r.Route(prefix, func(r chi.Router) {
		r.Post("/specs", a.handleCreateSpec)
		r.Get("/specs", a.handleListSpecs)
		r.Get("/specs/{specID}", a.handleGetSpec)
		r.Delete("/specs/{specID}", a.handleDeleteSpec)
		r.Get("/specs/{specID}/endpoints", a.handleListEndpoints)
		r.Get("/specs/{specID}/drift", a.handleGetDriftReports)
		r.Get("/specs/{specID}/drift/summary", a.handleDriftSummary)
		r.Post("/specs/{specID}/drift/scan", a.handleRequestDriftScan)
		r.Post("/specs/{specID}/drift/resolve/{reportID}", a.handleResolveDrift)
		r.Post("/traffic", a.handleIngestTraffic)
		r.Post("/analyze/{specID}", a.handleAgentAnalyze)
		r.Post("/tests/generate/{specID}", a.handleAgentGenerateTests)

		r.Route("/admin", func(r chi.Router) {
			r.Delete("/gdpr/{specID}", a.handleGDPRDeleteSpec)
			r.Get("/gdpr/{specID}", a.handleGDPRGetData)
			r.Get("/anomaly-metrics/{specID}", a.handleGetAnomalyMetrics)
		})
	})

	return r
}

func (a *API) Start(port string) error {
	a.server = &http.Server{
		Addr:         ":" + port,
		Handler:      a.Routes(),
		ReadTimeout:  defaultReadTimeout,
		WriteTimeout: defaultWriteTimeout,
		IdleTimeout:  defaultIdleTimeout,
	}
	a.Logger.Info("starting HTTP server", "port", port)
	return a.server.ListenAndServe()
}

func (a *API) Shutdown(ctx context.Context) error {
	a.Logger.Info("shutting down HTTP server")
	return a.server.Shutdown(ctx)
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status":"ok"}`))
}

func (a *API) handleReadiness(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status":"ready"}`))
}

func (a *API) execute(w http.ResponseWriter, r *http.Request, spanName string, fn func(context.Context) error) {
	ctx := r.Context()
	start := time.Now()

	var span trace.Span
	if a.tracer != nil {
		ctx, span = a.tracer.Start(ctx, spanName)
	}
	if span != nil {
		defer span.End()
	}

	defer func() {
		if rec := recover(); rec != nil {
			if span != nil {
				span.RecordError(apperrors.ErrUnknown)
				span.SetStatus(codes.Error, "panic recovered")
			}
			apperrors.WriteHTTPResponse(ctx, w, a.Logger, apperrors.ErrUnknown)
			a.Logger.ErrorContext(ctx, logMsgPanicRecovered,
				logKeyError, rec, logKeyStack, string(debug.Stack()))
		}
		if a.histogramLatency != nil {
			a.histogramLatency.Record(ctx, time.Since(start).Seconds(),
				metric.WithAttributes(attribute.String("handler", spanName)))
		}
	}()

	err := fn(ctx)
	if err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}
	apperrors.WriteHTTPResponse(ctx, w, a.Logger, err)
}
