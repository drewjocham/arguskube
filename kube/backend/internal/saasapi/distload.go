package saasapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/argues/argus/pkg/loadtest"
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

	// Ramp carries the per-profile knobs (rampTo, stepBy, spike sizes,
	// duration) the unified form collects. The flat RampProfile/RampRate
	// fields above remain as the back-compat shorthand for older clients
	// and the cloud worker JSON payload. When Ramp is non-nil the local
	// dispatcher uses it verbatim so the user's choice of linear/step/
	// spike actually takes effect.
	Ramp *DistLoadRamp `json:"ramp,omitempty"`

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

	// Payload, when non-nil, overrides the legacy PayloadSize byte-fill.
	// Carries the user-supplied source (upload/paste/typed/AI/file). The
	// cloud path keeps using PayloadSize for back-compat; this struct
	// lives on the local dispatcher side today.
	Payload *DistLoadPayload `json:"payload,omitempty"`

	// Scenario is REST-mode only: a multi-step plan (auth + 1..5
	// endpoints + chains + assertions). The SaaS workers own the
	// runner; the desktop just builds the spec and ships it. Empty
	// for Event-Bus tests and for single-endpoint REST tests built
	// with the legacy RESTConfig shape.
	Scenario *DistLoadScenario `json:"scenario,omitempty"`
}

// DistLoadPayload describes a user-supplied message body. Exactly one
// of {Bytes, FilePath} is meaningful per Source:
//
//   - upload/paste/type/ai → Bytes (Filename is metadata for "upload")
//   - file                 → FilePath + FileMode ("exact" or "template")
//
// AIPrompt is the prompt that produced an "ai" payload — kept only for
// audit so the Notebook report can quote the request.
// DistLoadRamp mirrors the loadtest.Ramp shape but uses JSON-friendly
// seconds (not Go Duration ns) so the frontend doesn't have to convert.
// Each profile uses a subset of fields — Validate / distLoadSpecToRunSpec
// picks the right ones based on RampProfile.
type DistLoadRamp struct {
	Profile     string `json:"profile"`               // constant | linear | step | spike
	Rate        int    `json:"rate,omitempty"`        // constant.rate, linear.from
	RampTo      int    `json:"rampTo,omitempty"`      // linear.to
	StepBy      int    `json:"stepBy,omitempty"`      // step.by
	StepEvery   int    `json:"stepEverySec,omitempty"`// step.every (seconds)
	SpikeCount  int    `json:"spikeCount,omitempty"`  // spike: number of bursts
	SpikeSize   int    `json:"spikeSize,omitempty"`   // spike: messages per burst
	SpikeIdle   int    `json:"spikeIdleSec,omitempty"`// spike: gap between bursts (seconds)
	DurationSec int    `json:"durationSec,omitempty"` // linear/step total runtime
}

// ToLoadtest translates this DistLoadRamp into the engine's
// loadtest.Ramp. One translation table for both the local dispatcher
// (api/pkg/app_distload_local.go) and the runner orchestrator
// (internal/runner/runner.go) — the load-test review flagged the two
// copies as drift-prone (the runner copy was missing case-insensitive
// profile matching and the default-rate-100 floor for constant
// profiles). Returns the zero Ramp when called on a nil receiver so
// callers can chain on optional fields.
func (r *DistLoadRamp) ToLoadtest() loadtest.Ramp {
	if r == nil {
		return loadtest.Ramp{}
	}
	out := loadtest.Ramp{Rate: float64(r.Rate)}
	if r.DurationSec > 0 {
		out.Duration = time.Duration(r.DurationSec) * time.Second
	}
	switch strings.ToLower(r.Profile) {
	case "linear":
		out.Kind = loadtest.RampLinear
		out.RampTo = float64(r.RampTo)
	case "step":
		out.Kind = loadtest.RampStep
		out.StepBy = float64(r.StepBy)
		if r.StepEvery > 0 {
			out.StepEvery = time.Duration(r.StepEvery) * time.Second
		}
	case "spike":
		out.Kind = loadtest.RampSpike
		out.SpikeCount = r.SpikeCount
		out.SpikeSize = r.SpikeSize
		if r.SpikeIdle > 0 {
			out.SpikeIdle = time.Duration(r.SpikeIdle) * time.Second
		}
	default:
		out.Kind = loadtest.RampConstant
		if out.Rate <= 0 {
			out.Rate = 100
		}
	}
	return out
}

type DistLoadPayload struct {
	Source   string `json:"source"`
	Bytes    string `json:"bytes,omitempty"`
	Filename string `json:"filename,omitempty"`
	FilePath string `json:"filePath,omitempty"`
	FileMode string `json:"fileMode,omitempty"`
	AIPrompt string `json:"aiPrompt,omitempty"`
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
