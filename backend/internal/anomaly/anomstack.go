package anomaly

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/argues/kube-watcher/internal/apperrors"
	"github.com/argues/kube-watcher/internal/config"
)

// Detector is the interface for anomaly detection backends.
type Detector interface {
	// Detect checks a metric series for anomalies and returns scored results.
	Detect(ctx context.Context, req DetectRequest) (*DetectResult, error)
	// ListJobs returns the configured anomaly detection jobs.
	ListJobs(ctx context.Context) ([]Job, error)
}

// DetectRequest holds the parameters for an anomaly detection call.
type DetectRequest struct {
	MetricName string            `json:"metric_name"`
	Labels     map[string]string `json:"labels,omitempty"`
	Window     time.Duration     `json:"window"`
}

// DetectResult holds the scored output from anomaly detection.
type DetectResult struct {
	MetricName  string    `json:"metric_name"`
	IsAnomaly   bool      `json:"is_anomaly"`
	Score       float64   `json:"score"`       // 0.0 = normal, 1.0 = extreme anomaly
	Description string    `json:"description"` // human-readable summary
	DetectedAt  time.Time `json:"detected_at"`
	ModelUsed   string    `json:"model_used"` // PyOD model name
}

// Job represents an Anomstack anomaly detection job.
type Job struct {
	Name     string `json:"name"`
	Metric   string `json:"metric"`
	Schedule string `json:"schedule"`
	LastRun  string `json:"last_run"`
	Status   string `json:"status"` // "ok", "alert", "error"
}

// AnomstackClient implements Detector against the Anomstack HTTP API.
type AnomstackClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewAnomstackClient creates a client for the Anomstack anomaly detection service.
func NewAnomstackClient(cfg *config.OnlineDataConfig, logger *slog.Logger) *AnomstackClient {
	return &AnomstackClient{
		baseURL: cfg.AI.AnomstackURL,
		apiKey:  cfg.AI.AnomstackAPIKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Detect calls Anomstack to check a metric for anomalies.
func (c *AnomstackClient) Detect(ctx context.Context, req DetectRequest) (*DetectResult, error) {
	url := fmt.Sprintf("%s/api/v1/detect", c.baseURL)

	c.logger.DebugContext(ctx, "anomstack detect request",
		slog.String("metric", req.MetricName),
		slog.String("url", url),
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, errors.Join(apperrors.ErrAnomstackFailed, err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, apperrors.Mark(apperrors.ErrAnomstackTimeout, apperrors.Retry)
		}
		return nil, errors.Join(apperrors.ErrAnomstackFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.Mark(
			fmt.Errorf("%w: status %d", apperrors.ErrAnomstackFailed, resp.StatusCode),
			apperrors.Retry,
		)
	}

	var result DetectResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, errors.Join(apperrors.ErrAnomstackFailed, err)
	}

	if result.IsAnomaly {
		c.logger.WarnContext(ctx, "anomaly detected",
			slog.String("metric", req.MetricName),
			slog.Float64("score", result.Score),
			slog.String("model", result.ModelUsed),
		)
	}

	return &result, nil
}

// ListJobs returns the configured anomaly detection jobs from Anomstack.
func (c *AnomstackClient) ListJobs(ctx context.Context) ([]Job, error) {
	url := fmt.Sprintf("%s/api/v1/jobs", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Join(apperrors.ErrAnomstackFailed, err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Join(apperrors.ErrAnomstackFailed, err)
	}
	defer resp.Body.Close()

	var jobs []Job
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, errors.Join(apperrors.ErrAnomstackFailed, err)
	}

	return jobs, nil
}
