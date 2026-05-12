package pkg

import (
	"fmt"

	"github.com/argues/kube-watcher/internal/usage"
)

// UsagePayload is what the frontend sees: usage.Summary plus a configured
// monthly budget so the UI can render a budget bar without a separate call.
type UsagePayload struct {
	usage.Summary
	MonthlyBudget float64 `json:"monthlyBudget"`
}

// GetUsageSummary returns today/month/lifetime LLM token totals plus per-model
// breakdown and an estimated cost computed from the configured rates.
func (a *App) GetUsageSummary() (UsagePayload, error) {
	if a.usage == nil {
		// Usage tracking is optional — if no store was wired in, return zeros
		// so the UI can still render an empty state.
		return UsagePayload{
			Summary: usage.Summary{
				Rates: usage.Rates{
					InputPerMTokens:  a.cfg.Billing.InputCostPer1M,
					OutputPerMTokens: a.cfg.Billing.OutputCostPer1M,
				},
			},
			MonthlyBudget: a.cfg.Billing.MonthlyBudget,
		}, nil
	}
	rates := usage.Rates{
		InputPerMTokens:  a.cfg.Billing.InputCostPer1M,
		OutputPerMTokens: a.cfg.Billing.OutputCostPer1M,
	}
	sum, err := a.usage.Summary(rates)
	if err != nil {
		return UsagePayload{}, fmt.Errorf("usage summary: %w", err)
	}
	return UsagePayload{
		Summary:       sum,
		MonthlyBudget: a.cfg.Billing.MonthlyBudget,
	}, nil
}

// ClearUsageHistory removes every recorded month. Irreversible.
func (a *App) ClearUsageHistory() error {
	if a.usage == nil {
		return nil
	}
	return a.usage.Clear()
}

// UpdateBillingRates persists the input/output cost-per-1M-tokens values that
// drive the cost columns in the UI. The MonthlyBudget is stored alongside.
// Rates take effect immediately for in-memory aggregation; restart preserves
// them only when the corresponding env var is set, matching the rest of the
// settings UI's behavior.
func (a *App) UpdateBillingRates(inputPer1M, outputPer1M, monthlyBudget float64) error {
	if inputPer1M < 0 || outputPer1M < 0 || monthlyBudget < 0 {
		return fmt.Errorf("billing rates must be non-negative")
	}
	a.cfg.Billing.InputCostPer1M = inputPer1M
	a.cfg.Billing.OutputCostPer1M = outputPer1M
	a.cfg.Billing.MonthlyBudget = monthlyBudget
	return nil
}
