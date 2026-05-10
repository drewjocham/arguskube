package anomaly

import (
	"bytes"
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

// FlinkClient implements Detector against the Flink gateway REST API.
type FlinkClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewFlinkClient creates a client for the Flink anomaly detection gateway.
func NewFlinkClient(cfg *config.OnlineDataConfig, logger *slog.Logger) *FlinkClient {
	return &FlinkClient{
		baseURL: cfg.AI.FlinkURL,
		apiKey:  cfg.AI.FlinkAPIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// detectRequest is the JSON body sent to the Flink gateway.
type flinkDetectRequest struct {
	MetricName string            `json:"metric_name"`
	Labels     map[string]string `json:"labels,omitempty"`
	Window     string            `json:"window"`
}

// detectResponse is the JSON response from the Flink gateway.
type flinkDetectResponse struct {
	MetricName  string    `json:"metric_name"`
	IsAnomaly   bool      `json:"is_anomaly"`
	Score       float64   `json:"score"`
	Description string    `json:"description"`
	DetectedAt  time.Time `json:"detected_at"`
	ModelUsed   string    `json:"model_used"`
}

func (c *FlinkClient) Detect(ctx context.Context, req DetectRequest) (*DetectResult, error) {
	url := fmt.Sprintf("%s/api/v1/detect", c.baseURL)

	body := flinkDetectRequest{
		MetricName: req.MetricName,
		Labels:     req.Labels,
		Window:     req.Window.String(),
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, errors.Join(apperrors.ErrFlinkFailed, err)
	}

	c.logger.DebugContext(ctx, "flink detect request",
		slog.String("metric", req.MetricName),
		slog.String("url", url),
	)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.Join(apperrors.ErrFlinkFailed, err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, apperrors.Mark(apperrors.ErrFlinkTimeout, apperrors.Retry)
		}
		return nil, errors.Join(apperrors.ErrFlinkFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apperrors.Mark(
			fmt.Errorf("%w: status %d", apperrors.ErrFlinkFailed, resp.StatusCode),
			apperrors.Retry,
		)
	}

	var fResp flinkDetectResponse
	if err := json.NewDecoder(resp.Body).Decode(&fResp); err != nil {
		return nil, errors.Join(apperrors.ErrFlinkFailed, err)
	}

	result := &DetectResult{
		MetricName:  fResp.MetricName,
		IsAnomaly:   fResp.IsAnomaly,
		Score:       fResp.Score,
		Description: fResp.Description,
		DetectedAt:  fResp.DetectedAt,
		ModelUsed:   fResp.ModelUsed,
	}

	if result.IsAnomaly {
		c.logger.WarnContext(ctx, "flink anomaly detected",
			slog.String("metric", req.MetricName),
			slog.Float64("score", result.Score),
			slog.String("model", result.ModelUsed),
		)
	}

	return result, nil
}

// jobResponse is the JSON response from the Flink gateway jobs endpoint.
type flinkJobResponse struct {
	Name     string `json:"name"`
	Metric   string `json:"metric"`
	Schedule string `json:"schedule"`
	LastRun  string `json:"last_run"`
	Status   string `json:"status"`
}

func (c *FlinkClient) ListJobs(ctx context.Context) ([]Job, error) {
	url := fmt.Sprintf("%s/api/v1/jobs", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Join(apperrors.ErrFlinkFailed, err)
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Join(apperrors.ErrFlinkFailed, err)
	}
	defer resp.Body.Close()

	var jobs []Job
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, errors.Join(apperrors.ErrFlinkFailed, err)
	}

	return jobs, nil
}
