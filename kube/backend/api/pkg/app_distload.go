package pkg

import (
	"fmt"

	"github.com/argues/argus/internal/features"
	"github.com/argues/argus/internal/saasapi"
)

// app_distload.go — Wails bindings for the distributed cloud load test
// feature (Pro-only). All methods delegate to the Argus SaaS platform;
// the desktop never provisions VMs directly.

func (a *App) checkDistLoadGate() error {
	if a.gate == nil || !a.gate.Allowed(features.FeatureDistributedLoadTest) {
		return features.ErrProRequired
	}
	return nil
}

func (a *App) distLoadClient() *saasapi.Client {
	return a.saasClient
}

func (a *App) StartDistributedLoadTest(spec saasapi.DistLoadSpec) (string, error) {
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

func (a *App) GetDistributedLoadTestStatus(runID string) (*saasapi.DistLoadStatus, error) {
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

func (a *App) CancelDistributedLoadTest(runID string) error {
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

func (a *App) GetDistributedLoadTestResult(runID string) (*saasapi.DistLoadStatus, error) {
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
