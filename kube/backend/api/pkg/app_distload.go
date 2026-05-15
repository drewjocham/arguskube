package pkg

import (
	"fmt"

	"github.com/argues/argus/internal/features"
	"github.com/argues/argus/internal/saasapi"
	"github.com/argues/argus/pkg/broker"
	"github.com/argues/argus/pkg/loadtest"
)

// app_distload.go — Wails bindings for Distributed Load Test, the one
// load-test feature. The runner mode on the spec picks the execution
// path:
//
//   spec.Runner == "local" → run pkg/loadtest in-process on the desktop
//   spec.Runner == "cloud" → delegate to the SaaS API (provisions VMs)
//   empty                  → defaults to "cloud" for back-compat with
//                            older frontends that posted no runner field
//
// One runID name-space covers both modes. Status/cancel route by
// looking up the ID in the local registry first; misses fall through
// to the SaaS client.

// runnerCloud is the back-compat default — see file docs above.
const runnerCloud = "cloud"

// checkDistLoadGate enforces the Pro entitlement for the cloud path.
// Local runs are free (the user's own machine does the work), so the
// gate only applies when we're about to call out to SaaS.
func (a *App) checkDistLoadGate() error {
	if a.gate == nil || !a.gate.Allowed(features.FeatureDistributedLoadTest) {
		return features.ErrProRequired
	}
	return nil
}

func (a *App) distLoadClient() *saasapi.Client {
	return a.saasClient
}

// resolveRunner returns "local", "cloud", or "runner" with empty
// defaulting to "cloud" for back-compat. Anything else is rejected
// so a typo doesn't silently fall through to the wrong execution path.
//
//   - "local"  — runs pkg/loadtest in-process on the desktop
//   - "cloud"  — legacy SaaS VM provisioning (kept for back-compat)
//   - "runner" — new GCP runner service (spot GKE + OpenTofu + SSE)
func resolveRunner(spec saasapi.DistLoadSpec) (string, error) {
	switch spec.Runner {
	case "":
		return runnerCloud, nil
	case "local", "cloud", "runner":
		return spec.Runner, nil
	default:
		return "", fmt.Errorf("unknown runner %q (want \"local\", \"cloud\", or \"runner\")", spec.Runner)
	}
}

// StartDistributedLoadTest starts a run. Returns a single runID that
// addresses subsequent status/cancel calls regardless of runner mode.
//
// Dispatch:
//   - "local"  → in-process loadtest.Engine (desktop)
//   - "runner" → GCP runner service (spot GKE + OpenTofu + SSE)
//   - "cloud"  → legacy SaaS VMs (back-compat)
func (a *App) StartDistributedLoadTest(spec saasapi.DistLoadSpec) (string, error) {
	mode, err := resolveRunner(spec)
	if err != nil {
		return "", err
	}
	switch mode {
	case "local":
		return a.startLocalDistLoad(spec)
	case "runner":
		return a.StartRunnerLoadTest(spec)
	default: // "cloud" — legacy path
		if err := a.checkDistLoadGate(); err != nil {
			return "", err
		}
		client := a.distLoadClient()
		if client == nil {
			return "", fmt.Errorf("SaaS client not configured — set ARGUS_SAAS_BASE_URL and ARGUS_SAAS_API_KEY")
		}
		runID, err := client.StartDistLoad(a.appCtx(), spec)
		if err != nil {
			return "", fmt.Errorf("start distributed load test: %w", err)
		}
		return runID, nil
	}
}

// GetDistributedLoadTestStatus is the unified status RPC. Local runs
// (found in the in-process registry) are mapped onto DistLoadStatus;
// anything else is treated as a SaaS run.
func (a *App) GetDistributedLoadTestStatus(runID string) (*saasapi.DistLoadStatus, error) {
	if run, ok := a.localRun(runID); ok {
		return a.distLoadStatusOfLocal(run), nil
	}
	if err := a.checkDistLoadGate(); err != nil {
		return nil, err
	}
	client := a.distLoadClient()
	if client == nil {
		return nil, fmt.Errorf("SaaS client not configured")
	}
	status, err := client.GetDistLoadStatus(a.appCtx(), runID)
	if err != nil {
		return nil, fmt.Errorf("get distributed load test status: %w", err)
	}
	return status, nil
}

// CancelDistributedLoadTest routes the cancel the same way as status.
func (a *App) CancelDistributedLoadTest(runID string) error {
	if run, ok := a.localRun(runID); ok {
		run.cancel()
		return nil
	}
	if err := a.checkDistLoadGate(); err != nil {
		return err
	}
	client := a.distLoadClient()
	if client == nil {
		return fmt.Errorf("SaaS client not configured")
	}
	if err := client.CancelDistLoad(a.appCtx(), runID); err != nil {
		return fmt.Errorf("cancel distributed load test: %w", err)
	}
	return nil
}

// GetDistributedLoadTestResult is an alias-style fetch the frontend
// hits once a run is done. Local runs reuse the live status shape; the
// SaaS branch hits the dedicated result endpoint.
func (a *App) GetDistributedLoadTestResult(runID string) (*saasapi.DistLoadStatus, error) {
	if run, ok := a.localRun(runID); ok {
		return a.distLoadStatusOfLocal(run), nil
	}
	if err := a.checkDistLoadGate(); err != nil {
		return nil, err
	}
	client := a.distLoadClient()
	if client == nil {
		return nil, fmt.Errorf("SaaS client not configured")
	}
	result, err := client.GetDistLoadResult(a.appCtx(), runID)
	if err != nil {
		return nil, fmt.Errorf("get distributed load test result: %w", err)
	}
	return result, nil
}

// GetDistributedLoadTestRecord returns the full local RunRecord (with
// all samples). Local-only — the SaaS path doesn't expose raw samples
// over the wire today. Returns an error if the run is still in flight.
func (a *App) GetDistributedLoadTestRecord(runID string) (*loadtest.RunRecord, error) {
	run, ok := a.localRun(runID)
	if !ok {
		return nil, fmt.Errorf("unknown local load test runId %q", runID)
	}
	run.mu.RLock()
	rec := run.record
	state := run.state
	run.mu.RUnlock()
	if rec == nil {
		return nil, fmt.Errorf("load test %s not finished (state=%s)", runID, state)
	}
	return rec, nil
}

// ListDistLoadPresets returns the audit-locked starting-point presets.
// Same list whether the user picks local or cloud — the frontend
// applies one before submission.
func (a *App) ListDistLoadPresets() []loadtest.Preset {
	return loadtest.PresetList()
}

// ListDistLoadBrokerKinds exposes the broker kinds the load tester
// supports. Returns the closed Knowns enum (rather than the live
// Registered() set) so the frontend dropdown stays stable across test
// binaries that don't import every adapter init block. The cloud
// worker links the same packages, so the two runners share one menu.
// KindREST is included because it lives in Knowns.
func (a *App) ListDistLoadBrokerKinds() []broker.Kind {
	return broker.Knowns
}

func (a *App) GetDistLoadCreditBalance() (float64, error) {
	if err := a.checkDistLoadGate(); err != nil {
		return 0, err
	}
	client := a.distLoadClient()
	if client == nil {
		return 0, fmt.Errorf("SaaS client not configured")
	}
	balance, err := client.GetCreditBalance(a.appCtx())
	if err != nil {
		return 0, fmt.Errorf("get credit balance: %w", err)
	}
	return balance, nil
}

func (a *App) GetDistLoadCreditHistory() ([]saasapi.CreditTransaction, error) {
	if err := a.checkDistLoadGate(); err != nil {
		return nil, err
	}
	client := a.distLoadClient()
	if client == nil {
		return nil, fmt.Errorf("SaaS client not configured")
	}
	history, err := client.GetCreditHistory(a.appCtx())
	if err != nil {
		return nil, fmt.Errorf("get credit history: %w", err)
	}
	return history, nil
}

func (a *App) GetDistLoadHistory() ([]saasapi.DistLoadStatus, error) {
	if err := a.checkDistLoadGate(); err != nil {
		return nil, err
	}
	client := a.distLoadClient()
	if client == nil {
		return nil, fmt.Errorf("SaaS client not configured")
	}
	runs, err := client.GetDistLoadHistory(a.appCtx())
	if err != nil {
		return nil, fmt.Errorf("get distributed load test history: %w", err)
	}
	return runs, nil
}

func (a *App) EstimateDistLoadCost(spec saasapi.DistLoadSpec) (float64, error) {
	if err := a.checkDistLoadGate(); err != nil {
		return 0, err
	}
	client := a.distLoadClient()
	if client == nil {
		return 0, fmt.Errorf("SaaS client not configured")
	}
	cost, err := client.EstimateCost(a.appCtx(), spec)
	if err != nil {
		return 0, fmt.Errorf("estimate cost: %w", err)
	}
	return cost, nil
}

func (a *App) ListDistLoadRegions() ([]saasapi.RegionOption, error) {
	if err := a.checkDistLoadGate(); err != nil {
		return nil, err
	}
	client := a.distLoadClient()
	if client == nil {
		return nil, fmt.Errorf("SaaS client not configured")
	}
	regions, err := client.ListRegions(a.appCtx())
	if err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	return regions, nil
}

func (a *App) GetDistLoadUsage() (*saasapi.LoadUsageSummary, error) {
	if err := a.checkDistLoadGate(); err != nil {
		return nil, err
	}
	client := a.distLoadClient()
	if client == nil {
		return nil, fmt.Errorf("SaaS client not configured")
	}
	usage, err := client.GetUsage(a.appCtx())
	if err != nil {
		return nil, fmt.Errorf("get usage summary: %w", err)
	}
	return usage, nil
}
