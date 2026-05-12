package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/argus/api-gov/internal/apperrors"
	"github.com/argus/api-gov/internal/models"
	"github.com/argus/api-gov/internal/service"
)

const (
	logMsgSpecCreated      = "spec created"
	logMsgSpecDeleted      = "spec deleted"
	logMsgTrafficIngested  = "traffic ingested"
	logMsgDriftScanStarted = "drift scan started"
	logMsgDriftResolved    = "drift report resolved"
	logMsgAnalysisDone     = "agent analysis done"
	logMsgTestsGenerated   = "tests generated"

	logKeySpecID     = "spec_id"
	logKeyReportID   = "report_id"
	logKeyMethod     = "method"
	logKeyPath       = "path"
	logKeyCount      = "count"
	logKeyEndpoints  = "endpoints"

	defaultPageSize = 50
	maxPageSize     = 500
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func parseIntParam(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		return defaultVal
	}
	return n
}

// ── Specs ──────────────────────────────────────────────────────

func (a *API) handleCreateSpec(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.createSpec", func(ctx context.Context) error {
		var req struct {
			Name    string          `json:"name"`
			Version string          `json:"version,omitempty"`
			Content json.RawMessage `json:"content"`
			Format  string          `json:"format,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return apperrors.Mark(apperrors.ErrBadRequest, apperrors.BadRequest)
		}

		spec, err := a.specSvc.CreateSpec(ctx, service.CreateSpecRequest{
			Name:    req.Name,
			Version: req.Version,
			Content: req.Content,
			Format:  req.Format,
		})
		if err != nil {
			return err
		}

		a.Logger.LogAttrs(ctx, slog.LevelInfo, logMsgSpecCreated,
			slog.String(logKeySpecID, spec.ID),
			slog.String("name", spec.Name),
		)
		a.counterSpecsCreated.Add(ctx, 1, metric.WithAttributes(attribute.String("spec_id", spec.ID)))

		go a.agentCli.AnalyzeSpec(ctx, spec.ID)
		writeJSON(w, http.StatusCreated, spec)
		return nil
	})
}

func (a *API) handleListSpecs(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.listSpecs", func(ctx context.Context) error {
		limit := parseIntParam(r, "limit", defaultPageSize)
		offset := parseIntParam(r, "offset", 0)
		if limit > maxPageSize {
			limit = maxPageSize
		}

		specs, total, err := a.specSvc.List(ctx, limit, offset)
		if err != nil {
			return err
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data":   specs,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
		return nil
	})
}

func (a *API) handleGetSpec(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.getSpec", func(ctx context.Context) error {
		spec, err := a.specSvc.GetByID(ctx, chi.URLParam(r, "specID"))
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, spec)
		return nil
	})
}

func (a *API) handleDeleteSpec(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.deleteSpec", func(ctx context.Context) error {
		if err := a.specSvc.Delete(ctx, chi.URLParam(r, "specID")); err != nil {
			return err
		}
		a.Logger.LogAttrs(ctx, slog.LevelInfo, logMsgSpecDeleted,
			slog.String(logKeySpecID, chi.URLParam(r, "specID")),
		)
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		return nil
	})
}

// ── Admin / GDPR ───────────────────────────────────────────────

func (a *API) handleGDPRDeleteSpec(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.gdprDeleteSpec", func(ctx context.Context) error {
		specID := chi.URLParam(r, "specID")
		if err := a.specSvc.GDPRDelete(ctx, specID); err != nil {
			return err
		}
		a.Logger.LogAttrs(ctx, slog.LevelInfo, "GDPR erasure completed",
			slog.String(logKeySpecID, specID),
		)
		writeJSON(w, http.StatusOK, map[string]string{"status": "erased"})
		return nil
	})
}

func (a *API) handleGDPRGetData(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.gdprGetData", func(ctx context.Context) error {
		specID := chi.URLParam(r, "specID")
		counts, err := a.specSvc.CountUserData(ctx, specID)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, counts)
		return nil
	})
}

func (a *API) handleGetAnomalyMetrics(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.getAnomalyMetrics", func(ctx context.Context) error {
		specID := chi.URLParam(r, "specID")
		metrics, err := a.specSvc.GetAnomalyMetrics(ctx, specID)
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, metrics)
		return nil
	})
}

// ── Endpoints ──────────────────────────────────────────────────

func (a *API) handleListEndpoints(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.listEndpoints", func(ctx context.Context) error {
		specID := chi.URLParam(r, "specID")
		limit := parseIntParam(r, "limit", defaultPageSize)
		offset := parseIntParam(r, "offset", 0)
		if limit > maxPageSize {
			limit = maxPageSize
		}

		endpoints, total, err := a.specSvc.GetEndpoints(ctx, specID, limit, offset)
		if err != nil {
			return err
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data":   endpoints,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
		return nil
	})
}

// ── Drift ──────────────────────────────────────────────────────

func (a *API) handleGetDriftReports(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.getDriftReports", func(ctx context.Context) error {
		specID := chi.URLParam(r, "specID")
		limit := parseIntParam(r, "limit", defaultPageSize)
		page := parseIntParam(r, "page", 1)

		var resolved *bool
		switch r.URL.Query().Get("resolved") {
		case "true":
			t := true
			resolved = &t
		case "false":
			f := false
			resolved = &f
		}

		filter := &models.DriftFilter{
			Resolved: resolved,
			Severity: r.URL.Query().Get("severity"),
			Category: r.URL.Query().Get("category"),
			Limit:    limit,
			Page:     page,
		}

		if a.driftSvc == nil {
			return apperrors.ErrDriftReportNotFound
		}
		reports, total, err := a.driftSvc.GetReports(ctx, specID, filter)
		if err != nil {
			return err
		}

		totalPages := 1
		if limit > 0 {
			totalPages = (total + limit - 1) / limit
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data":        reports,
			"total":       total,
			"page":        page,
			"page_size":   limit,
			"total_pages": totalPages,
		})
		return nil
	})
}

func (a *API) handleDriftSummary(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.driftSummary", func(ctx context.Context) error {
		summary, err := a.driftSvc.Summary(ctx, chi.URLParam(r, "specID"))
		if err != nil {
			return err
		}
		writeJSON(w, http.StatusOK, summary)
		return nil
	})
}

func (a *API) handleResolveDrift(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.resolveDrift", func(ctx context.Context) error {
		reportID := chi.URLParam(r, "reportID")
		if err := a.driftSvc.Resolve(ctx, reportID); err != nil {
			return err
		}
		a.Logger.LogAttrs(ctx, slog.LevelInfo, logMsgDriftResolved,
			slog.String(logKeyReportID, reportID),
		)
		writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
		return nil
	})
}

func (a *API) handleRequestDriftScan(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.requestDriftScan", func(ctx context.Context) error {
		specID := chi.URLParam(r, "specID")

		if a.agentCli != nil {
			go func() {
				if err := a.agentCli.TriggerDriftScan(ctx, specID); err != nil {
					a.Logger.LogAttrs(ctx, slog.LevelError, "drift scan failed",
						slog.String(logKeySpecID, specID),
						slog.Any(logKeyError, err),
					)
				}
			}()
		}

		a.Logger.LogAttrs(ctx, slog.LevelInfo, logMsgDriftScanStarted,
			slog.String(logKeySpecID, specID),
		)
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":  "scan_started",
			"spec_id": specID,
		})
		return nil
	})
}

// ── Traffic ────────────────────────────────────────────────────

func (a *API) handleIngestTraffic(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.ingestTraffic", func(ctx context.Context) error {
		var req struct {
			SpecID   string            `json:"spec_id"`
			Method   string            `json:"method"`
			Path     string            `json:"path"`
			Status   int               `json:"status_code"`
			Request  map[string]any    `json:"request,omitempty"`
			Response map[string]any    `json:"response,omitempty"`
			Headers  map[string]string `json:"headers,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return apperrors.Mark(apperrors.ErrValidation, apperrors.BadRequest)
		}

		if req.SpecID == "" {
			return apperrors.Mark(apperrors.ErrValidation, apperrors.BadRequest)
		}

		observed := &models.ObservedData{
			Method:     req.Method,
			Path:       req.Path,
			StatusCode: req.Status,
			Request:    req.Request,
			Response:   req.Response,
			Headers:    req.Headers,
		}

		go a.agentCli.IngestTraffic(ctx, req.SpecID, observed)

		a.Logger.LogAttrs(ctx, slog.LevelInfo, logMsgTrafficIngested,
			slog.String(logKeySpecID, req.SpecID),
			slog.String(logKeyMethod, req.Method),
			slog.String(logKeyPath, req.Path),
		)
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "ingested"})
		return nil
	})
}

// ── Agent Integration ──────────────────────────────────────────

func (a *API) handleAgentAnalyze(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.agentAnalyze", func(ctx context.Context) error {
		result, err := a.agentCli.AnalyzeSpec(ctx, chi.URLParam(r, "specID"))
		if err != nil {
			return err
		}
		a.Logger.LogAttrs(ctx, slog.LevelInfo, logMsgAnalysisDone,
			slog.String(logKeySpecID, chi.URLParam(r, "specID")),
			slog.Int(logKeyEndpoints, result.Endpoints),
		)
		writeJSON(w, http.StatusOK, result)
		return nil
	})
}

func (a *API) handleAgentGenerateTests(w http.ResponseWriter, r *http.Request) {
	a.execute(w, r, "api.agentGenerateTests", func(ctx context.Context) error {
		var req struct {
			EndpointID string `json:"endpoint_id,omitempty"`
			Count      int    `json:"count,omitempty"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Count <= 0 {
			req.Count = 5
		}

		result, err := a.agentCli.GenerateTests(ctx, chi.URLParam(r, "specID"), req.EndpointID, req.Count)
		if err != nil {
			return err
		}
		a.Logger.LogAttrs(ctx, slog.LevelInfo, logMsgTestsGenerated,
			slog.String(logKeySpecID, chi.URLParam(r, "specID")),
			slog.Int(logKeyCount, len(result.TestCases)),
		)
		writeJSON(w, http.StatusOK, result)
		return nil
	})
}
