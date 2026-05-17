package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/argus/api-gov/internal/apperrors"
	"github.com/argus/api-gov/internal/models"
)

// AgentClient communicates with the Python LangGraph agent service
// for spec analysis, drift detection, and test generation.
type AgentClient struct {
	baseURL        string
	driftThreshold float64
	client         *http.Client
	logger         *slog.Logger
}

const (
	defaultAgentTimeout = 120 * time.Second
	logKeyAgentStatus   = "agent_status"
	logKeyAgentEndpoint = "agent_endpoint"
)

var errAgentResponse = fmt.Errorf("agent service error")

func NewAgentClient(baseURL string, driftThreshold float64, logger *slog.Logger) *AgentClient {
	return &AgentClient{
		baseURL:        baseURL,
		driftThreshold: driftThreshold,
		logger:         logger,
		client: &http.Client{
			Timeout: defaultAgentTimeout,
		},
	}
}

type agentRequest struct {
	SpecID         string            `json:"spec_id,omitempty"`
	EndpointID     string            `json:"endpoint_id,omitempty"`
	Method         string            `json:"method,omitempty"`
	Path           string            `json:"path,omitempty"`
	StatusCode     int               `json:"status_code,omitempty"`
	Request        map[string]any    `json:"request,omitempty"`
	Response       map[string]any    `json:"response,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	Count          int               `json:"count,omitempty"`
	DriftThreshold float64           `json:"drift_threshold"`
}

type analyzeResponse struct {
	Endpoints     int    `json:"endpoints"`
	CriticalPaths int    `json:"critical_paths"`
	AuthRoutes    int    `json:"auth_routes"`
	Summary       string `json:"summary"`
}

type testCase struct {
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers"`
	Body        any               `json:"body,omitempty"`
	Expected    int               `json:"expected_status"`
	Description string            `json:"description"`
}

type testResponse struct {
	TestCases []testCase `json:"test_cases"`
}

func (c *AgentClient) post(ctx context.Context, path string, reqBody any, respBody any) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	c.logger.LogAttrs(ctx, slog.LevelDebug, "agent request",
		slog.String(logKeyAgentEndpoint, path),
	)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		c.logger.LogAttrs(ctx, slog.LevelError, "agent request failed",
			slog.String(logKeyAgentEndpoint, path),
			slog.Any(apperrors.LogKeyError, err),
		)
		return fmt.Errorf("agent request to %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read agent response: %w", err)
	}

	if resp.StatusCode >= 400 {
		c.logger.LogAttrs(ctx, slog.LevelError, "agent error response",
			slog.Int(logKeyAgentStatus, resp.StatusCode),
			slog.String(logKeyAgentEndpoint, path),
			slog.String("response_body", string(body)),
		)
		return fmt.Errorf("%w: agent returned %d", errAgentResponse, resp.StatusCode)
	}

	if respBody != nil && len(body) > 0 {
		if err := json.Unmarshal(body, respBody); err != nil {
			return fmt.Errorf("unmarshal agent response: %w", err)
		}
	}

	return nil
}

func (c *AgentClient) AnalyzeSpec(ctx context.Context, specID string) (*analyzeResponse, error) {
	req := agentRequest{SpecID: specID, DriftThreshold: c.driftThreshold}
	var resp analyzeResponse
	if err := c.post(ctx, "/analyze/"+specID, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *AgentClient) TriggerDriftScan(ctx context.Context, specID string) error {
	req := agentRequest{SpecID: specID, DriftThreshold: c.driftThreshold}
	return c.post(ctx, "/drift/scan/"+specID, req, nil)
}

func (c *AgentClient) IngestTraffic(ctx context.Context, specID string, data *models.ObservedData) {
	req := agentRequest{
		SpecID:     specID,
		Method:     data.Method,
		Path:       data.Path,
		StatusCode: data.StatusCode,
		Request:    data.Request,
		Response:   data.Response,
		Headers:    data.Headers,
	}

	if err := c.post(ctx, "/traffic", req, nil); err != nil {
		c.logger.LogAttrs(ctx, slog.LevelError, "traffic ingestion failed",
			slog.String("spec_id", specID),
			slog.Any(apperrors.LogKeyError, err),
		)
	}
}

func (c *AgentClient) GenerateTests(ctx context.Context, specID, endpointID string, count int) (*testResponse, error) {
	req := agentRequest{
		SpecID:         specID,
		EndpointID:     endpointID,
		Count:          count,
		DriftThreshold: c.driftThreshold,
	}
	var resp testResponse
	if err := c.post(ctx, "/tests/generate/"+specID, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
