package saasapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// ── Types ─────────────────────────────────────────────────────────────

type DistLoadSpec struct {
	Name        string          `json:"name"`
	Regions     []RegionSpec    `json:"regions"`
	Broker      json.RawMessage `json:"broker"`
	Destination string          `json:"destination"`
	PayloadSize int             `json:"payloadSize"`
	Count       int             `json:"count"`
	Workers     int             `json:"workers"`
	RampProfile string          `json:"rampProfile"`
	RampRate    int             `json:"rampRate"`
	TimeoutMins int             `json:"timeoutMins"`

	// Runner selects where the engine actually runs.
	//   "local" — the desktop runs pkg/loadtest in-process.
	//   "cloud" — the SaaS API provisions VMs and runs there.
	// Empty defaults to "cloud" at the dispatch layer for back-compat
	// with older frontends that posted no runner field.
	Runner string `json:"runner,omitempty"`

	// PresetID is the optional starting-point preset the user picked
	// (matches loadtest.Preset.ID). The frontend applies the preset's
	// fields before submission; this field carries the choice through
	// for audit/log purposes (and lets a local dispatcher record it).
	PresetID string `json:"presetId,omitempty"`
}

type RegionSpec struct {
	Provider     string `json:"provider"`
	Region       string `json:"region"`
	InstanceType string `json:"instanceType"`
	Count        int    `json:"count"`
}

type ProvisionInfo struct {
	Region       string `json:"region"`
	VMsSpec      int    `json:"vmsSpec"`
	VMsReady     int    `json:"vmsReady"`
	State        string `json:"state"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

type WorkerStatus struct {
	Region    string  `json:"region"`
	Sent      int     `json:"sent"`
	Acked     int     `json:"acked"`
	Errors    int     `json:"errors"`
	Throughput float64 `json:"throughput"`
	P50Ms     float64 `json:"p50Ms"`
	P95Ms     float64 `json:"p95Ms"`
	P99Ms     float64 `json:"p99Ms"`
	State     string  `json:"state"`
}

type LoadSummary struct {
	TotalSent    int     `json:"totalSent"`
	TotalAcked   int     `json:"totalAcked"`
	TotalErrors  int     `json:"totalErrors"`
	Throughput   float64 `json:"throughput"`
	P50LatencyMs float64 `json:"p50LatencyMs"`
	P95LatencyMs float64 `json:"p95LatencyMs"`
	P99LatencyMs float64 `json:"p99LatencyMs"`
	DurationSec  float64 `json:"durationSec"`
}

type DistLoadStatus struct {
	RunID             string           `json:"runId"`
	State             string           `json:"state"`
	Name              string           `json:"name"`
	ProvisionProgress []ProvisionInfo  `json:"provisionProgress,omitempty"`
	Workers           []WorkerStatus   `json:"workers,omitempty"`
	Summary           *LoadSummary     `json:"summary,omitempty"`
	CreditsUsed       float64          `json:"creditsUsed,omitempty"`
	CreditsEstimated  float64          `json:"creditsEstimated,omitempty"`
	Error             string           `json:"error,omitempty"`
	StartedAt         time.Time        `json:"startedAt"`
	FinishedAt        time.Time        `json:"finishedAt,omitempty"`
}

type RegionOption struct {
	Provider      string  `json:"provider"`
	Region        string  `json:"region"`
	Label         string  `json:"label"`
	InstanceTypes []string `json:"instanceTypes"`
	DefaultType   string  `json:"defaultType"`
}

type CreditTransaction struct {
	ID        string    `json:"id"`
	Amount    float64   `json:"amount"`
	Type      string    `json:"type"`
	RunID     string    `json:"runId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	Note      string    `json:"note,omitempty"`
}

type LoadUsageSummary struct {
	TotalCreditsUsed float64 `json:"totalCreditsUsed"`
	ThisMonth        float64 `json:"thisMonth"`
	RunsThisMonth    int     `json:"runsThisMonth"`
	AvgCostPerRun    float64 `json:"avgCostPerRun"`
}

type CostEstimate struct {
	EstimatedCredits float64  `json:"estimatedCredits"`
	Breakdown        *Breakdown `json:"breakdown,omitempty"`
}

type Breakdown struct {
	VMMinutes  float64 `json:"vmMinutes"`
	DataTransfer float64 `json:"dataTransfer"`
	RegionPremium float64 `json:"regionPremium"`
}

// ── API Methods ───────────────────────────────────────────────────────

func (c *Client) StartDistLoad(ctx context.Context, spec DistLoadSpec) (string, error) {
	var resp struct {
		RunID string `json:"runId"`
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/loadtest", spec, &resp); err != nil {
		return "", err
	}
	return resp.RunID, nil
}

func (c *Client) GetDistLoadStatus(ctx context.Context, runID string) (*DistLoadStatus, error) {
	var status DistLoadStatus
	if err := c.do(ctx, http.MethodGet, "/api/v1/loadtest/"+runID, nil, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

func (c *Client) CancelDistLoad(ctx context.Context, runID string) error {
	return c.do(ctx, http.MethodDelete, "/api/v1/loadtest/"+runID, nil, nil)
}

func (c *Client) GetDistLoadResult(ctx context.Context, runID string) (*DistLoadStatus, error) {
	return c.GetDistLoadStatus(ctx, runID)
}

func (c *Client) GetCreditBalance(ctx context.Context) (float64, error) {
	var resp struct {
		Balance float64 `json:"balance"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/loadtest/credits", nil, &resp); err != nil {
		return 0, err
	}
	return resp.Balance, nil
}

func (c *Client) GetCreditHistory(ctx context.Context) ([]CreditTransaction, error) {
	var resp struct {
		Transactions []CreditTransaction `json:"transactions"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/loadtest/credits/history", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Transactions == nil {
		return []CreditTransaction{}, nil
	}
	return resp.Transactions, nil
}

func (c *Client) GetDistLoadHistory(ctx context.Context) ([]DistLoadStatus, error) {
	var resp struct {
		Runs []DistLoadStatus `json:"runs"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/loadtest/history", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Runs == nil {
		return []DistLoadStatus{}, nil
	}
	return resp.Runs, nil
}

func (c *Client) EstimateCost(ctx context.Context, spec DistLoadSpec) (float64, error) {
	var est CostEstimate
	if err := c.do(ctx, http.MethodPost, "/api/v1/loadtest/estimate", spec, &est); err != nil {
		return 0, err
	}
	return est.EstimatedCredits, nil
}

func (c *Client) ListRegions(ctx context.Context) ([]RegionOption, error) {
	var resp struct {
		Regions []RegionOption `json:"regions"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/loadtest/regions", nil, &resp); err != nil {
		return nil, err
	}
	if resp.Regions == nil {
		return []RegionOption{}, nil
	}
	return resp.Regions, nil
}

func (c *Client) GetUsage(ctx context.Context) (*LoadUsageSummary, error) {
	var summary LoadUsageSummary
	if err := c.do(ctx, http.MethodGet, "/api/v1/loadtest/usage", nil, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}
