package pkg

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/argues/argus/internal/alerts"
	"github.com/google/uuid"
)

// apiRequestMaxBytes caps inbound RPC bodies. The Wails-style RPC over
// HTTP only carries typed JSON args; no legitimate caller approaches
// this limit. Sized loosely so internal tooling that batches resource
// lookups still fits, while DoS attacks via oversized bodies don't.
const apiRequestMaxBytes int64 = 4 << 20 // 4 MiB

type APIRequest struct {
	Args []interface{} `json:"args"`
}

type APIResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Default-deny CORS: only echo origins explicitly allowed (localhost or
	// values in ARGUS_API_ALLOWED_ORIGINS). The previous
	// `Access-Control-Allow-Origin: *` made every reflective method on App
	// callable from any browser — including DeletePod, ApplyYaml, etc.
	if !applyCORS(w, r, allowedOrigins()) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if !a.authorizeAPIRequest(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/")
	methodName := path

	if !methodAllowedOverHTTP(methodName) {
		http.Error(w, "method not exposed via HTTP API", http.StatusForbidden)
		return
	}

	method := reflect.ValueOf(a).MethodByName(methodName)
	if !method.IsValid() {
		http.Error(w, "Method not found", http.StatusNotFound)
		return
	}

	// Cap request bodies before they hit the JSON decoder. Without this
	// the dispatcher will happily buffer a multi-GB payload from a
	// malicious client and OOM the server — the audit flagged it as a
	// trivial DoS surface. 4 MiB is well above any legitimate RPC arg
	// shape this API exposes (typed args are small JSON values; the
	// large surfaces — uploads, log streams — don't go through here).
	r.Body = http.MaxBytesReader(w, r.Body, apiRequestMaxBytes)
	var req APIRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// http.MaxBytesError is what MaxBytesReader returns on overflow.
		// Distinguish from a malformed body so the client knows whether
		// to shrink or fix.
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
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
	if err := json.NewEncoder(w).Encode(res); err != nil {
		a.logger.WarnContext(r.Context(), "encode response failed",
			slog.String("error", err.Error()),
			slog.String("path", r.URL.Path),
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
//
// Webhook auth: when ARGUS_WEBHOOK_TOKEN is set, the inbound request
// must carry a matching `Authorization: Bearer <token>` OR a
// `X-Webhook-Token: <token>` header. When unset, only loopback POSTs are
// accepted — the previous open-CORS path made it trivial to inject fake
// alerts from any origin.
func (a *App) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if !applyCORS(w, r, allowedOrigins()) {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !authenticateWebhook(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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
	if err := json.NewEncoder(w).Encode(map[string]string{
		"alert_id": alertID,
		"status":   "ok",
	}); err != nil {
		a.logger.WarnContext(r.Context(), "encode response failed",
			slog.String("error", err.Error()),
			slog.String("path", r.URL.Path),
		)
	}
}

// StartHTTPServer starts the SaaS API server on the specified port.
//
// Bind address defaults to 127.0.0.1 — the loopback interface — so a
// misconfigured firewall can't expose the API to the public internet.
// Override via ARGUS_API_BIND="0.0.0.0" once you've also configured
// ARGUS_API_TOKEN and ARGUS_API_ALLOWED_ORIGINS.
func (a *App) StartHTTPServer(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", a.ServeHTTP)
	mux.HandleFunc("/webhooks/anomstack", a.HandleWebhook)
	if a.auth != nil {
		a.AuthRoutes(mux)
	}
	go a.hub.Run(a.ctx)
	mux.HandleFunc("/tunnel", a.hub.HandleTunnel)

	addr := envBindAddr(port)
	tokenSet := os.Getenv("ARGUS_API_TOKEN") != ""
	bindIsLocal := strings.HasPrefix(addr, "127.0.0.1") || strings.HasPrefix(addr, "[::1]")

	if !bindIsLocal && !tokenSet {
		a.logger.Warn("SaaS API binding to a non-loopback address WITHOUT ARGUS_API_TOKEN — refusing to start; set the token or restrict the bind",
			slog.String("addr", addr),
		)
		return
	}

	a.logger.Info("Starting SaaS API Server",
		slog.String("addr", addr),
		slog.Bool("tokenAuth", tokenSet),
		slog.Bool("loopbackOnly", bindIsLocal),
	)
	go func() {
		srv := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}
		if err := srv.ListenAndServe(); err != nil {
			a.logger.Error("SaaS API Server failed", slog.String("error", err.Error()))
		}
	}()
}

// authenticateWebhook gates /webhooks/anomstack. Token can be carried in
// either Authorization: Bearer or X-Webhook-Token to fit ops tooling that
// can't easily set arbitrary headers.
func authenticateWebhook(r *http.Request) bool {
	token := strings.TrimSpace(os.Getenv("ARGUS_WEBHOOK_TOKEN"))
	if token == "" {
		return remoteIsLocal(r)
	}
	if remoteIsLocal(r) {
		return true
	}
	got := strings.TrimSpace(r.Header.Get("X-Webhook-Token"))
	if got == "" {
		hdr := r.Header.Get("Authorization")
		if strings.HasPrefix(hdr, "Bearer ") {
			got = strings.TrimSpace(hdr[len("Bearer "):])
		}
	}
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

// httpExposedMethods is the explicit allowlist of App methods callable via
// the SaaS HTTP API. Mutating cluster operations stay Wails-only by
// default; reflection used to expose every public method indiscriminately.
//
// The set is curated and conservative — adding to it should be a deliberate
// security review. Anything dangerous (DeletePod, ApplyYaml, DeleteResource,
// LaunchPopOutTerminal, StartTerminal, RestartDeployment-style actions,
// SwitchContext that mutates kubeconfig, UpdateSettings) is intentionally
// excluded.
var httpExposedMethods = map[string]struct{}{
	// Cluster read-only data the SaaS frontend needs to render dashboards.
	"GetClusterInfo":         {},
	"GetAppMode":             {},
	"GetTier":                {},
	"GetSettings":            {}, // returns masked tokens; safe to read
	"GetClusterMetrics":      {},
	"DetectAlerts":           {},
	"ListResources":          {},
	"GetResourceDetail":      {},
	"GetResourceYaml":        {},
	"ListAllNamespaces":      {},
	"ListContexts":           {},
	"AutoResolveContext":     {},
	"RunEnvProbes":           {},
	// User-profile / "Argus suggests" surface. RecordView is high-frequency
	// (every nav) but harmless; Mute/Accept/Dismiss are user-driven so the
	// rate is naturally bounded. ClearUserActivity is deliberately omitted
	// — it's a destructive action restricted to desktop.
	"RecordView":         {},
	"GetNextSuggestion":  {},
	"MuteSuggestion":     {},
	"AcceptSuggestion":   {},
	"DismissSuggestion":  {},
	"GetTopology":            {},
	"GetWarningEvents":       {},
	"GetNamespacePodCounts":  {},
	"GetTopRestarters":       {},
	"GetDeploymentRevisions": {},
	"GetVPARecommendations":  {},
	// Diagnostics + AI chat (read-only-ish; conversation history per alert).
	"DiagnoseAlert":    {},
	"GetChatHistory":   {},
	"GetAutoSummary":   {},
	"GetAgentEventLog": {},
	"SendChatMessage":  {}, // produces side-effect: API call to LLM
	// Argus Scan + Vulnerabilities — read-only reports.
	"RunArgusScan":        {},
	"ListVulnerabilities": {},
	// Argo CD read-only.
	"GetArgusCDStatus":      {},
	"ListArgusCDApps":       {},
	"GetArgusCDApp":         {},
	"GetArgusCDResources":   {},
	"GetArgusCDDiffs":       {},
	"ListArgusCDProjects":   {},
	"TestArgusCDConnection": {},
	// Notebooks + Runbooks read.
	"ListNotebooks": {},
	"GetNotebook":   {},
	"ListRunbooks":  {},
	"GetRunbook":    {},
	// Setup / tools probes (read-only).
	"CheckTools": {},
	// Pause/unpause is benign — affects only this server's polling cadence.
	"SetPaused": {},
	// Secret reference label parsing — pure metadata, never resolves a
	// value. ResolveSecretRef is intentionally NOT in this allowlist:
	// it must only be reachable from in-process Wails bindings so a
	// browser script can't dereference the operator's vault.
	"DescribeSecretRef": {},
	// Unified OAuth flow — listing providers and starting/polling/
	// completing flows. None of these mutate cluster state; the worst
	// they can do is rate-limit the upstream provider's auth endpoint.
	"ListOAuthProviders": {},
	"StartOAuthFlow":     {},
	"PollOAuthFlow":      {},
	"CompleteOAuthFlow":  {},
	"CancelOAuthFlow":    {},
	// Spot-checks: read-only cluster probes that emit notifications.
	"RunSpotChecks":  {},
	"RunSpotCheck":   {},
	"ListSpotChecks": {},
	// Alert processor: lifecycle controls + agent profile.
	"AckAlert":            {},
	"SilenceAlert":        {},
	"MarkAlertIgnored":    {},
	"GetAgentProfile":     {},
	"SetAgentProfile":     {},
	"AlertInvestigations": {},
	// Deployment artifacts: read-only catalog + env validation.
	"GetDeployArtifacts": {},
	"GetDeployArtifact":  {},
	"ValidateEnvFile":    {},
	// Pipelines: outbound read-only fetches against provider APIs. They
	// hit api.github.com etc. with the user's stored token; nothing is
	// mutated server-side.
	"ListGitHubPullRequests": {},
	"ListGitHubBranches":     {},
	// Vault: read-only credential status + small key/value secret store.
	// SetVaultSecret/DeleteVaultSecret only touch the user's own
	// $HOME/.argus/vault/secrets.json, scoped per-machine.
	"GetVaultStatus":     {},
	"TestVaultProvider":  {},
	"ListVaultSecrets":   {},
	"SetVaultSecret":     {},
	"DeleteVaultSecret":  {},
	// External-secrets: probes the local CLIs (kubeseal/sops/gpg/age) and
	// enumerates encrypted secret sources in a namespace via the dynamic
	// client. All read-only — no cluster mutations.
	"TestSecretsTool":            {},
	"ListEncryptedSecretSources": {},
}

func methodAllowedOverHTTP(name string) bool {
	if name == "" {
		return false
	}
	_, ok := httpExposedMethods[name]
	return ok
}
