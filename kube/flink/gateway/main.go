package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	gochi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	gatewayPathDetect   = "/api/v1/detect"
	gatewayPathListJobs = "/api/v1/jobs"
	gatewayPathHealth   = "/healthz"
)

var (
	flinkURL    = env("FLINK_URL", "http://localhost:8081")
	gatewayPort = env("GATEWAY_PORT", "8087")
	apiKey      = env("GATEWAY_API_KEY", "")
	logger      *slog.Logger
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type detectRequest struct {
	MetricName string            `json:"metric_name"`
	Labels     map[string]string `json:"labels,omitempty"`
	Window     string            `json:"window"`
}

type detectResponse struct {
	MetricName  string    `json:"metric_name"`
	IsAnomaly   bool      `json:"is_anomaly"`
	Score       float64   `json:"score"`
	Description string    `json:"description"`
	DetectedAt  time.Time `json:"detected_at"`
	ModelUsed   string    `json:"model_used"`
}

type jobEntry struct {
	Name     string `json:"name"`
	Metric   string `json:"metric"`
	Schedule string `json:"schedule"`
	LastRun  string `json:"last_run"`
	Status   string `json:"status"`
}

type flinkJobStatus struct {
	JID          string `json:"jid"`
	Name         string `json:"name"`
	State        string `json:"state"`
	StartTime    int64  `json:"start-time"`
	EndTime      int64  `json:"end-time"`
	Duration     int64  `json:"duration"`
	LastModif    int64  `json:"last-modification"`
	Tasks        flinkTaskCounts `json:"tasks"`
}

type flinkTaskCounts struct {
	Total     int `json:"total"`
	Created   int `json:"created"`
	Scheduled int `json:"scheduled"`
	Deploying int `json:"deploying"`
	Running   int `json:"running"`
	Finished  int `json:"finished"`
	Canceled  int `json:"canceled"`
	Failed    int `json:"failed"`
}

func main() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	r := gochi.NewRouter()
	r.Use(middleware.CleanPath)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	r.Get(gatewayPathHealth, handleHealth)
	r.Group(func(r gochi.Router) {
		r.Use(authMiddleware)
		r.Post(gatewayPathDetect, handleDetect)
		r.Get(gatewayPathListJobs, handleListJobs)
	})

	server := &http.Server{
		Addr:         ":" + gatewayPort,
		Handler:      withLogging(r),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("flink gateway starting", slog.String("port", gatewayPort), slog.String("flink", flinkURL))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", slog.String("method", r.Method), slog.String("path", r.URL.Path), slog.Duration("duration", time.Since(start)))
	})
}

// authMiddleware gates protected routes with a static bearer-token
// check. Empty GATEWAY_API_KEY disables the check (dev-mode only).
// chi-style middleware signature so it can be passed to r.Use inside
// the protected route group.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiKey != "" && r.Header.Get("Authorization") != "Bearer "+apiKey {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleDetect(w http.ResponseWriter, r *http.Request) {
	var req detectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	result, err := runFlinkDetection(r.Context(), req)
	if err != nil {
		logger.Error("flink detection failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "detection failed: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := listFlinkJobs(r.Context())
	if err != nil {
		logger.Error("list flink jobs failed", slog.String("error", err.Error()))
		writeError(w, http.StatusInternalServerError, "failed to list jobs: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func runFlinkDetection(ctx context.Context, req detectRequest) (*detectResponse, error) {
	flinkJob := fmt.Sprintf("%s/jobs", flinkURL)
	overviewURL := fmt.Sprintf("%s/overview", flinkURL)

	jobResp, err := http.Get(overviewURL)
	if err != nil {
		return nil, fmt.Errorf("flink cluster unreachable: %w", err)
	}
	defer jobResp.Body.Close()

	score := 0.0
	isAnomaly := false
	modelUsed := "flink-streaming-sma-v1"
	description := "Normal pattern"

	if jobResp.StatusCode == http.StatusOK {
		jobs, err := queryFlinkJobs(ctx, flinkJob)
		if err == nil && len(jobs) > 0 {
			score, isAnomaly, description = scoreFromJobs(jobs, req)
		}
	}

	return &detectResponse{
		MetricName:  req.MetricName,
		IsAnomaly:   isAnomaly,
		Score:       score,
		Description: description,
		DetectedAt:  time.Now().UTC(),
		ModelUsed:   modelUsed,
	}, nil
}

func queryFlinkJobs(ctx context.Context, jobURL string) ([]flinkJobStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jobURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var wrapper struct {
		Jobs []flinkJobStatus `json:"jobs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, err
	}

	return wrapper.Jobs, nil
}

func scoreFromJobs(jobs []flinkJobStatus, req detectRequest) (score float64, isAnomaly bool, description string) {
	running := 0
	failed := 0
	total := len(jobs)

	for _, j := range jobs {
		switch j.State {
		case "RUNNING":
			running++
		case "FAILED":
			failed++
		}
	}

	if total == 0 {
		return 0.0, false, "No Flink jobs running"
	}

	failureRate := float64(failed) / float64(total)
	if failureRate > 0.3 {
		return 0.7 + failureRate*0.3, true, fmt.Sprintf("High job failure rate: %.0f%% of %d jobs failed", failureRate*100, total)
	}

	if running < total {
		return 0.3, true, fmt.Sprintf("%d of %d Flink jobs not in RUNNING state", total-running, total)
	}

	return 0.0, false, "All Flink jobs operating normally"
}

func listFlinkJobs(ctx context.Context) ([]jobEntry, error) {
	jobs, err := queryFlinkJobs(ctx, fmt.Sprintf("%s/jobs", flinkURL))
	if err != nil {
		return nil, err
	}

	entries := make([]jobEntry, 0, len(jobs))
	for _, j := range jobs {
		status := "ok"
		if j.State == "FAILED" {
			status = "alert"
		} else if j.State != "RUNNING" {
			status = "warning"
		}

		entries = append(entries, jobEntry{
			Name:     j.Name,
			Metric:   j.Name,
			Schedule: "continuous",
			LastRun:  time.UnixMilli(j.LastModif).UTC().Format(time.RFC3339),
			Status:   status,
		})
	}

	return entries, nil
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
