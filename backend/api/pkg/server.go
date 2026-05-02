package pkg

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/argues/kube-watcher/internal/alerts"
	"github.com/google/uuid"
)

type APIRequest struct {
	Args []interface{} `json:"args"`
}

type APIResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS for dev and SaaS mode
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/")
	methodName := path

	method := reflect.ValueOf(a).MethodByName(methodName)
	if !method.IsValid() {
		http.Error(w, "Method not found", http.StatusNotFound)
		return
	}

	var req APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	methodType := method.Type()
	in := make([]reflect.Value, methodType.NumIn())
	for i := 0; i < methodType.NumIn(); i++ {
		argType := methodType.In(i)
		if i < len(req.Args) {
			// Serialize and deserialize to handle proper type mapping (e.g., float64 to int, map to struct)
			b, _ := json.Marshal(req.Args[i])
			newVal := reflect.New(argType)
			if err := json.Unmarshal(b, newVal.Interface()); err == nil {
				in[i] = newVal.Elem()
			} else {
				in[i] = reflect.Zero(argType)
			}
		} else {
			in[i] = reflect.Zero(argType)
		}
	}

	// Safely call the method
	out := method.Call(in)

	var res APIResponse
	if len(out) > 0 {
		res.Result = out[0].Interface()
	}
	if len(out) > 1 && !out[1].IsNil() {
		errInterface := out[1].Interface()
		if err, ok := errInterface.(error); ok {
			res.Error = err.Error()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
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
