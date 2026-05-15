// Command runner is the distributed load-test runner service that runs
// on GCP Cloud Run. It receives RunnerSpecs from the desktop, provisions
// ephemeral spot GKE clusters per region, executes the load test, and
// streams results back via SSE.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/argues/argus/internal/runner"
	"github.com/argues/argus/internal/saasapi"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("argus-runner: %v", err)
	}
}

func run() error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	projectID := os.Getenv("GCP_PROJECT")
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}
	if projectID == "" {
		return errors.New("GCP_PROJECT environment variable required")
	}

	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	// The OpenTofu module path inside the container image.
	modulePath := os.Getenv("RUNNER_MODULE_PATH")
	if modulePath == "" {
		modulePath = "/etc/argus/terraform/modules/runner-region"
	}

	workspace := os.Getenv("RUNNER_WORKSPACE")
	if workspace == "" {
		workspace = "/var/argus/runner"
	}
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return err
	}

	apiKey := os.Getenv("RUNNER_API_KEY")

	maxConcurrent := 5
	if v := os.Getenv("RUNNER_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxConcurrent = n
		}
	}

	srv := &RunnerServer{
		logger:         logger,
		projectID:      projectID,
		modulePath:     modulePath,
		workspace:      workspace,
		apiKey:         apiKey,
		runners:        map[string]*runner.Runner{},
		sem:            make(chan struct{}, maxConcurrent),
		sseHeartbeat:   15 * time.Second,
		abandonedGrace: 60 * time.Second,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", srv.handleHealth)
	mux.HandleFunc("/api/v1/runner/start", srv.withAuth(srv.handleStart))
	mux.HandleFunc("/api/v1/runner/", srv.withAuth(srv.handleRunner))

	httpServer := &http.Server{
		Addr:         ":" + strconv.Itoa(port),
		Handler:      withLogging(mux, logger),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // SSE needs no write timeout
		IdleTimeout:  60 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Background cleanup loop.
	go func() {
		r := runner.New(saasapi.RunnerSpec{}, modulePath, workspace, logger)
		r.CleanupLoop(ctx, projectID)
	}()

	// Start HTTP server.
	go func() {
		logger.Info("runner server starting", slog.Int("port", port))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
		}
	}()

	// Wait for signal.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	logger.Info("shutting down")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Cancel all active runs.
	srv.mu.Lock()
	for id, r := range srv.runners {
		r.Cancel()
		delete(srv.runners, id)
	}
	srv.mu.Unlock()

	return httpServer.Shutdown(shutdownCtx)
}

// RunnerServer holds the active runners and HTTP handlers.
type RunnerServer struct {
	logger     *slog.Logger
	projectID  string
	modulePath string
	workspace  string
	apiKey     string

	mu      sync.RWMutex
	runners map[string]*runner.Runner

	// sem limits concurrent runs. Acquired in handleStart, released
	// when the background goroutine completes. Configurable via
	// RUNNER_MAX_CONCURRENT env var (default 5).
	sem chan struct{}

	// sseHeartbeat is how often serveSSE sends a `:keepalive` comment.
	// 15s keeps proxies (Cloud Run frontends, corporate egress) from
	// silently dropping the long-lived stream, and makes a half-closed
	// TCP socket detectable via Write error in under a minute.
	sseHeartbeat time.Duration

	// abandonedGrace is the window between the last SSE client
	// disconnecting and the runner auto-cancelling the run. Tuned for
	// "user closed laptop / reloaded the page" — long enough to allow a
	// reconnect, short enough that a forgotten run doesn't burn a
	// provisioned GKE cluster for 2h.
	abandonedGrace time.Duration
}

// minCreditsForRun returns a conservative lower bound on the credits
// the spec will consume. The desktop client computes a tighter
// estimate via saasapi.EstimateRunnerCost; this server-side floor
// catches obvious under-funding (CreditsHeld=0, spec tampered) so
// the runner doesn't burn an hour of spot-VM time before the SaaS
// API later refuses to settle the bill.
//
// Formula: regions × node_count × minPerNodeRun. Tuned against the
// cheapest expected node-hour (e2-small spot ≈ $0.01/hr → ~0.1 cred)
// over the shortest sensible run (5 min). Intentionally low — final
// settlement happens SaaS-side at completion.
func minCreditsForRun(spec saasapi.RunnerSpec) float64 {
	const minPerNodeRun = 0.5
	total := 0.0
	for _, r := range spec.Regions {
		n := r.Count
		if n < 1 {
			n = 1
		}
		total += float64(n) * minPerNodeRun
	}
	return total
}

func (s *RunnerServer) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey != "" {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != s.apiKey {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next(w, r)
	}
}

// ── Handlers ──────────────────────────────────────────────────────────

func (s *RunnerServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// POST /api/v1/runner/start — accepts a RunnerSpec and begins execution.
func (s *RunnerServer) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var spec saasapi.RunnerSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if spec.RunID == "" {
		spec.RunID = uuid.New().String()
	}
	if len(spec.Regions) == 0 {
		http.Error(w, "at least one region required", http.StatusBadRequest)
		return
	}

	// CreditsHeld must reflect a real SaaS-side reservation. Without
	// this gate, a client could send creditsHeld=0 and burn an hour of
	// spot-VM time before the SaaS API rejects the settlement. Final
	// reconciliation still happens SaaS-side; this is a cheap fast-fail.
	if spec.CreditsHeld <= 0 {
		http.Error(w, "creditsHeld must be positive — reserve credits before starting the run", http.StatusPaymentRequired)
		return
	}
	if minCreds := minCreditsForRun(spec); spec.CreditsHeld < minCreds {
		http.Error(w,
			"creditsHeld below the floor for this spec — refresh the cost estimate and re-reserve",
			http.StatusPaymentRequired)
		s.logger.Warn("rejecting run: creditsHeld below floor",
			"runId", spec.RunID,
			"creditsHeld", spec.CreditsHeld,
			"minCredits", minCreds,
			"regions", len(spec.Regions))
		return
	}

	// Acquire semaphore slot. This is the concurrency cap — prevents
	// runaway Cloud Run instances from provisioning N clusters at once.
	select {
	case s.sem <- struct{}{}:
	default:
		http.Error(w, "too many concurrent runs, try again later", http.StatusTooManyRequests)
		return
	}

	run := runner.New(spec, s.modulePath, s.workspace, s.logger)

	s.mu.Lock()
	s.runners[spec.RunID] = run
	s.mu.Unlock()

	// Execute in background.
	go func() {
		defer func() { <-s.sem }()
		_, err := run.Run(context.Background())
		if err != nil {
			s.logger.Error("run failed", "runId", spec.RunID, "error", err)
		}
		s.mu.Lock()
		delete(s.runners, spec.RunID)
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"runId": spec.RunID})
}

// GET /api/v1/runner/{runId}/stream — SSE stream
// GET /api/v1/runner/{runId} — status
// DELETE /api/v1/runner/{runId} — cancel
func (s *RunnerServer) handleRunner(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/runner/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "runId required", http.StatusBadRequest)
		return
	}
	runID := parts[0]

	s.mu.RLock()
	run, exists := s.runners[runID]
	s.mu.RUnlock()

	if !exists {
		// Check if this is a historical run (result still available).
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	// GET /api/v1/runner/{runId} — status
	if r.Method == http.MethodGet && len(parts) == 1 {
		result := run.Result()
		if result == nil {
			result = &saasapi.RunnerResult{
				RunID: runID,
				State: run.State(),
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	// GET /api/v1/runner/{runId}/stream — SSE
	if r.Method == http.MethodGet && len(parts) == 2 && parts[1] == "stream" {
		s.serveSSE(w, r, run)
		return
	}

	// DELETE /api/v1/runner/{runId} — cancel
	if r.Method == http.MethodDelete {
		run.Cancel()
		w.WriteHeader(http.StatusAccepted)
		return
	}

	http.Error(w, "not found", http.StatusNotFound)
}

// serveSSE streams runner events to the desktop via Server-Sent Events.
//
// Two reliability features layered on top of the basic fan-out:
//
//   - Heartbeat: a periodic `:keepalive` SSE comment keeps intermediaries
//     (Cloud Run frontends, corporate egress proxies) from silently
//     dropping the long-lived connection, and turns a half-closed TCP
//     socket into a Write error within one heartbeat interval.
//   - Auto-cancel on disconnect: when this handler returns, if no other
//     subscribers remain attached AND the run is still active, schedule
//     a delayed cancel after `abandonedGrace`. A reconnect inside the
//     grace window cancels the cancel, so a page reload doesn't kill
//     the run.
func (s *RunnerServer) serveSSE(w http.ResponseWriter, r *http.Request, run *runner.Runner) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := run.Stream.Subscribe()
	defer func() {
		run.Stream.Unsubscribe(ch)
		s.scheduleAbandonedCancel(run)
	}()

	heartbeat := time.NewTicker(s.sseHeartbeat)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if _, err := w.Write([]byte(":keepalive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case evt, ok := <-ch:
			if !ok {
				return
			}
			frame, err := runner.SSEEvent(evt)
			if err != nil {
				continue
			}
			_, werr := w.Write([]byte(frame))
			if werr != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// scheduleAbandonedCancel arms a delayed cancel for a run that just
// lost its last SSE subscriber. The check happens twice — once
// immediately to avoid spawning a goroutine for the common case where
// other clients are still attached, and once after the grace window
// so a reconnecting page refresh disarms the cancel.
func (s *RunnerServer) scheduleAbandonedCancel(run *runner.Runner) {
	if run.Stream.NumSubscribers() > 0 {
		return
	}
	if state := run.State(); state != "running" && state != "pending" {
		return
	}
	go func() {
		time.Sleep(s.abandonedGrace)
		if run.Stream.NumSubscribers() > 0 {
			return
		}
		if state := run.State(); state != "running" && state != "pending" {
			return
		}
		s.logger.Warn("cancelling abandoned run — no SSE subscribers after grace period",
			"grace", s.abandonedGrace)
		run.Cancel()
	}()
}

// ── Middleware ─────────────────────────────────────────────────────────

func withLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(lrw, r)
		logger.Info("request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", lrw.status),
			slog.Duration("dur", time.Since(start)),
		)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	l.status = code
	l.ResponseWriter.WriteHeader(code)
}
