package pkg

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/argues/argus/internal/saasapi"
)

// app_distload_runner.go — Wails bindings for the distributed load-test
// runner that lives on GCP. The runner provisions ephemeral spot GKE
// clusters, runs the load-test engine, then tears everything down.
//
// The desktop builds a RunnerSpec and sends it to the runner service
// via the SaaS API proxy. Events stream back over SSE.

// StartRunnerLoadTest builds a RunnerSpec from a DistLoadSpec and
// submits it to the runner service. Returns the run ID.
//
// Prerequisites:
//   - Pro entitlement (checked via feature gate)
//   - Sufficient credits (estimated and held before execution)
//   - SaaS client configured
func (a *App) StartRunnerLoadTest(spec saasapi.DistLoadSpec) (string, error) {
	if err := a.checkDistLoadGate(); err != nil {
		return "", err
	}
	client := a.distLoadClient()
	if client == nil {
		return "", fmt.Errorf("SaaS client not configured — set ARGUS_SAAS_BASE_URL and ARGUS_SAAS_API_KEY")
	}

	// 1. Build the runner spec from the user's DistLoadSpec.
	runnerSpec, err := distSpecToRunnerSpec(spec)
	if err != nil {
		return "", fmt.Errorf("invalid runner spec: %w", err)
	}

	// 2. Estimate credits.
	est, err := client.EstimateRunnerCost(a.appCtx(), runnerSpec)
	if err != nil {
		return "", fmt.Errorf("estimate cost: %w", err)
	}

	// 3. Check credit balance.
	balance, err := client.GetCreditBalance(a.appCtx())
	if err != nil {
		return "", fmt.Errorf("get credit balance: %w", err)
	}
	if balance < est.EstimatedCredits {
		return "", fmt.Errorf("insufficient credits (have %.0f, need %.0f)",
			balance, est.EstimatedCredits)
	}

	// 4. Hold credits atomically.
	if err := client.HoldRunnerCredits(a.appCtx(), runnerSpec.RunID, est.EstimatedCredits); err != nil {
		return "", fmt.Errorf("hold credits: %w", err)
	}
	runnerSpec.CreditsHeld = est.EstimatedCredits

	// 5. Start the runner.
	runID, err := client.StartRunner(a.appCtx(), runnerSpec)
	if err != nil {
		// Release the held credits on failure.
		_ = client.ReleaseRunnerCredits(a.appCtx(), runnerSpec.RunID, 0)
		return "", fmt.Errorf("start runner: %w", err)
	}

	a.logger.Info("runner load test started",
		slog.String("runId", runID),
		slog.Float64("creditsHeld", est.EstimatedCredits),
	)

	return runID, nil
}

// GetRunnerStreamURL returns the SSE stream URL for a runner run.
func (a *App) GetRunnerStreamURL(runID string) string {
	client := a.distLoadClient()
	if client == nil {
		return ""
	}
	return client.RunnerStreamURL(runID)
}

// distSpecToRunnerSpec converts a user-facing DistLoadSpec into the
// runner's RunnerSpec, with the steps derived from the spec shape.
func distSpecToRunnerSpec(spec saasapi.DistLoadSpec) (saasapi.RunnerSpec, error) {
	runID := uuid.New().String()

	steps := []saasapi.RunStep{
		{Type: "provision"},
		{Type: "publish"},
		{Type: "destroy"},
	}

	if spec.Regions == nil || len(spec.Regions) == 0 {
		// Default: single region from the broker config hint.
		spec.Regions = []saasapi.RegionSpec{
			{Provider: "gcp", Region: "us-central1", InstanceType: "e2-small", Count: 1},
		}
	}

	loadSpec := &saasapi.RunnerLoadSpec{
		Destination: spec.Destination,
		Count:       spec.Count,
		Workers:     spec.Workers,
	}

	if spec.Ramp != nil {
		loadSpec.Ramp = spec.Ramp
	} else {
		loadSpec.Ramp = &saasapi.DistLoadRamp{
			Profile:     "constant",
			Rate:        spec.RampRate,
			DurationSec: spec.TimeoutMins * 60,
		}
	}

	rs := saasapi.RunnerSpec{
		RunID:       runID,
		Name:        spec.Name,
		Broker:      spec.Broker,
		Payload:     spec.Payload,
		Regions:     spec.Regions,
		Steps:       steps,
		LoadSpec:    loadSpec,
		CreditsHeld: 0, // set after estimation
	}

	if err := validateRunnerSpec(&rs); err != nil {
		return rs, err
	}
	return rs, nil
}

// validateRunnerSpec checks the runner spec has the minimum required fields.
func validateRunnerSpec(s *saasapi.RunnerSpec) error {
	if s.RunID == "" {
		return fmt.Errorf("runID required")
	}
	if len(s.Regions) == 0 {
		return fmt.Errorf("at least one region required")
	}
	if len(s.Broker) == 0 {
		return fmt.Errorf("broker config required")
	}
	if s.LoadSpec == nil {
		return fmt.Errorf("loadSpec required")
	}
	if s.LoadSpec.Count <= 0 {
		return fmt.Errorf("count must be > 0")
	}
	if s.LoadSpec.Destination == "" {
		return fmt.Errorf("destination required")
	}
	return nil
}

var _ = time.Now // imported for timestamp handling
