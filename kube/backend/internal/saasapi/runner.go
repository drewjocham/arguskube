package saasapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ── Types ─────────────────────────────────────────────────────────────

// RunnerSpec is the full test orchestration plan sent from the desktop
// to the runner service on GCP. The runner fans this out across regions:
// for each region it provisions spot infra (via OpenTofu + Helm), runs
// the load test, then tears everything down.
type RunnerSpec struct {
	RunID   string           `json:"runId"`
	Name    string           `json:"name"`
	Broker  json.RawMessage  `json:"broker"`
	Payload *DistLoadPayload `json:"payload,omitempty"`

	// Regions is the list of regions to run in. Each gets its own
	// ephemeral K8s cluster with the selected broker installed.
	Regions []RegionSpec `json:"regions"`

	// Steps is the ordered plan the runner executes per region.
	Steps []RunStep `json:"steps"`

	// LoadSpec carries the publish-phase parameters shared across
	// all regions.
	LoadSpec *RunnerLoadSpec `json:"loadSpec,omitempty"`

	// CreditsHeld is the amount pre-reserved before the run started.
	// The runner deducts actual spend on completion and refunds the
	// difference.
	CreditsHeld float64 `json:"creditsHeld,omitempty"`
}

// RunStep is one phase in the test lifecycle. The runner iterates
// through these in order for each region.
type RunStep struct {
	// Type: "provision" | "publish" | "scale" | "drain" | "destroy"
	Type string `json:"type"`

	// Region this step applies to. Empty means "all regions".
	Region string `json:"region,omitempty"`

	// Payload is step-specific config (e.g. scale target replicas).
	Payload json.RawMessage `json:"payload,omitempty"`
}

// RunnerLoadSpec is the publish-phase config shared across regions.
type RunnerLoadSpec struct {
	Destination string         `json:"destination"`
	Count       int            `json:"count"`
	Workers     int            `json:"workers,omitempty"`
	Ramp        *DistLoadRamp  `json:"ramp,omitempty"`
	Scale       *DistLoadScale `json:"scale,omitempty"`
}

// DistLoadScale mirrors loadtest.ScalePlan for the runner path.
type DistLoadScale struct {
	PreScaleToZero      bool  `json:"preScaleToZero,omitempty"`
	MinReplicas         int32 `json:"minReplicas,omitempty"`
	PreScaleTimeoutSec  int   `json:"preScaleTimeoutSec,omitempty"`
	PostScaleTimeoutSec int   `json:"postScaleTimeoutSec,omitempty"`
}

// RunnerEvent is a single event in the SSE stream from runner → desktop.
type RunnerEvent struct {
	RunID   string `json:"runId"`
	Type    string `json:"type"`
	Region  string `json:"region,omitempty"`
	Step    string `json:"step,omitempty"`
	Message string `json:"message,omitempty"`

	Provision *ProvisionInfo    `json:"provision,omitempty"`
	Progress  *WorkerStatus     `json:"progress,omitempty"`
	Scale     *RunnerScaleEvent `json:"scale,omitempty"`
	Summary   *LoadSummary      `json:"summary,omitempty"`
	Error     string            `json:"error,omitempty"`

	Timestamp time.Time `json:"ts"`
}

// RunnerScaleEvent mirrors loadtest.ScaleEvent for the runner stream.
type RunnerScaleEvent struct {
	At       time.Time `json:"at"`
	Phase    string    `json:"phase"`
	Replicas int32     `json:"replicas"`
	Ready    int32     `json:"ready"`
}

// RunnerRegionResult is the per-region result stored after completion.
type RunnerRegionResult struct {
	Region  string       `json:"region"`
	Success bool         `json:"success"`
	Summary *LoadSummary `json:"summary,omitempty"`
	Error   string       `json:"error,omitempty"`
	// CreditsUsed is the estimated infra cost for this region.
	CreditsUsed float64 `json:"creditsUsed,omitempty"`
}

// RunnerResult is the aggregate result returned when all regions finish.
type RunnerResult struct {
	RunID         string               `json:"runId"`
	State         string               `json:"state"` // running | done | canceled | error
	Regions       []RunnerRegionResult `json:"regions"`
	Summary       *LoadSummary         `json:"summary,omitempty"`
	CreditsHeld   float64              `json:"creditsHeld"`
	CreditsSpent  float64              `json:"creditsSpent"`
	CreditsRefund float64              `json:"creditsRefund"`
	StartedAt     time.Time            `json:"startedAt"`
	FinishedAt    time.Time            `json:"finishedAt,omitempty"`
	PresetID      string               `json:"presetId,omitempty"`
}

// ── Client Methods ─────────────────────────────────────────────────────

// StartRunner submits a runner spec to the GCP runner service (via
// the SaaS API proxy). Returns the run ID on acceptance.
func (c *Client) StartRunner(ctx context.Context, spec RunnerSpec) (string, error) {
	var resp struct {
		RunID string `json:"runId"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/runner/start", spec, &resp); err != nil {
		return "", fmt.Errorf("start runner: %w", err)
	}
	return resp.RunID, nil
}

// RunnerStreamURL returns the SSE stream URL for a run. The desktop
// connects directly to this URL (or via the SaaS proxy).
func (c *Client) RunnerStreamURL(runID string) string {
	return c.baseURL + "/api/v1/runner/" + runID + "/stream"
}

// GetRunnerStatus returns the current state of a runner run.
func (c *Client) GetRunnerStatus(ctx context.Context, runID string) (*RunnerResult, error) {
	var result RunnerResult
	if err := c.do(ctx, http.MethodGet, "/api/v1/runner/"+runID, nil, &result); err != nil {
		return nil, fmt.Errorf("get runner status: %w", err)
	}
	return &result, nil
}

// CancelRunner cancels a runner run and triggers immediate teardown.
func (c *Client) CancelRunner(ctx context.Context, runID string) error {
	if err := c.do(ctx, http.MethodDelete, "/api/v1/runner/"+runID, nil, nil); err != nil {
		return fmt.Errorf("cancel runner: %w", err)
	}
	return nil
}

// EstimateRunnerCost pre-computes the credit cost for a runner spec.
func (c *Client) EstimateRunnerCost(ctx context.Context, spec RunnerSpec) (*CostEstimate, error) {
	var est CostEstimate
	if err := c.do(ctx, http.MethodPost, "/api/v1/runner/estimate", spec, &est); err != nil {
		return nil, fmt.Errorf("estimate runner cost: %w", err)
	}
	return &est, nil
}

// HoldRunnerCredits atomically reserves credits for a run.
func (c *Client) HoldRunnerCredits(ctx context.Context, runID string, amount float64) error {
	body := map[string]any{"runId": runID, "amount": amount}
	if err := c.do(ctx, http.MethodPost, "/api/v1/runner/credits/hold", body, nil); err != nil {
		return fmt.Errorf("hold runner credits: %w", err)
	}
	return nil
}

// ReleaseRunnerCredits releases unspent credits after a run completes.
func (c *Client) ReleaseRunnerCredits(ctx context.Context, runID string, spent float64) error {
	body := map[string]any{"runId": runID, "spent": spent}
	if err := c.do(ctx, http.MethodPost, "/api/v1/runner/credits/release", body, nil); err != nil {
		return fmt.Errorf("release runner credits: %w", err)
	}
	return nil
}

// RunnerEventTypes — SSE event type constants.
const (
	EventProvisioning = "provisioning"
	EventProvisioned  = "provisioned"
	EventProgress     = "progress"
	EventScale        = "scale"
	EventError        = "error"
	EventRegionDone   = "region_done"
	EventComplete     = "complete"
)
