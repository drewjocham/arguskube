package pkg

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/google/uuid"
)

// APIRequest is the JSON body of every /api/<MethodName> POST. Args are
// captured as raw JSON so each typed adapter can decode its own positional
// arguments without relying on reflection at dispatch time.
type APIRequest struct {
	Args []json.RawMessage `json:"args"`
}

// APIResponse is what every successful /api call returns. Error is populated
// when the underlying handler returned a non-nil error; in that case Result
// is omitted (omitempty handles untyped nil).
type APIResponse struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// ensureRoutes lazily builds this App's dispatch table. The table is per-App
// (not package-global) so multiple App instances — and tests — don't share
// closures bound to the wrong receiver.
func (a *App) ensureRoutes() {
	a.routesOnce.Do(func() {
		a.routesTbl = a.routes()
	})
}

// ServeHTTP dispatches /api/<MethodName> calls to the typed adapter
// registered in routes(). Any method name not in the table returns 404 —
// no reflection is involved, so unregistered methods are unreachable
// regardless of what's exported on *App.
func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	a.ensureRoutes()

	method := strings.TrimPrefix(r.URL.Path, "/api/")
	handler, ok := a.routesTbl[method]
	if !ok {
		http.Error(w, "method not found", http.StatusNotFound)
		return
	}

	var req APIRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}
	}

	res := APIResponse{}
	defer func() {
		if rec := recover(); rec != nil {
			a.logger.Error("api handler panicked",
				slog.String("method", method),
				slog.Any("panic", rec),
			)
			res = APIResponse{Error: "internal error"}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(res)
		}
	}()

	result, err := handler(req.Args)
	if err != nil {
		res.Error = err.Error()
	} else {
		res.Result = result
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		a.logger.Warn("api response encode failed",
			slog.String("method", method),
			slog.String("error", err.Error()),
		)
	}
}

// WebhookPayload is the JSON body from an anomaly detection webhook.
type WebhookPayload struct {
	Title      string            `json:"title"`
	MetricName string            `json:"metric_name"`
	Threshold  float64           `json:"threshold"`
	Score      float64           `json:"score"`
	Severity   string            `json:"severity"`
	Namespace  string            `json:"namespace"`
	PodName    string            `json:"pod_name"`
	NodeName   string            `json:"node_name"`
	Labels     map[string]string `json:"labels"`
}

// HandleWebhook receives anomaly alerts from external systems (Anomstack, Grafana, etc.)
// and merges them into the alert stream visible in the frontend.
func (a *App) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	severity := alerts.SeverityWarning
	switch strings.ToLower(payload.Severity) {
	case "critical":
		severity = alerts.SeverityCritical
	case "info":
		severity = alerts.SeverityInfo
	}
	// If threshold is high, escalate to critical.
	if payload.Threshold >= 0.9 || payload.Score >= 0.9 {
		severity = alerts.SeverityCritical
	}

	alertID := uuid.New().String()
	title := payload.Title
	if title == "" {
		title = "Anomaly: " + payload.MetricName
	}

	alert := alerts.Alert{
		ID:          alertID,
		Name:        title,
		Severity:    severity,
		Namespace:   payload.Namespace,
		Timestamp:   time.Now(),
		PodName:     payload.PodName,
		NodeName:    payload.NodeName,
		Description: fmt.Sprintf("Anomaly detected on metric %q (threshold=%.2f, score=%.2f)", payload.MetricName, payload.Threshold, payload.Score),
		Tags: []alerts.Tag{
			{Label: "anomaly", Color: "purple"},
			{Label: payload.MetricName, Color: "teal"},
		},
	}

	a.webhookMu.Lock()
	a.webhookAlerts = append(a.webhookAlerts, alert)
	// Keep only last 100 webhook alerts.
	if len(a.webhookAlerts) > 100 {
		a.webhookAlerts = a.webhookAlerts[len(a.webhookAlerts)-100:]
	}
	a.webhookMu.Unlock()

	a.logger.Info("webhook alert received",
		slog.String("alert_id", alertID),
		slog.String("title", title),
		slog.String("metric", payload.MetricName),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"alert_id": alertID,
		"status":   "ok",
	})
}

// StartHTTPServer starts the SaaS API server on the specified port.
func (a *App) StartHTTPServer(port int) {
	mux := http.NewServeMux()

	// REST API endpoint
	mux.HandleFunc("/api/", a.ServeHTTP)

	// Webhook endpoint for external anomaly detectors
	mux.HandleFunc("/webhooks/anomstack", a.HandleWebhook)

	// WebSocket Hub endpoint for in-cluster Agents
	go a.hub.Run(a.ctx)
	mux.HandleFunc("/tunnel", a.hub.HandleTunnel)

	addr := fmt.Sprintf(":%d", port)
	a.logger.Info("Starting SaaS API Server", slog.String("addr", addr))
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			a.logger.Error("SaaS API Server failed", slog.String("error", err.Error()))
		}
	}()
}
