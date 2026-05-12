package pkg

import (
	"io"
	"log/slog"
	"testing"

	"github.com/argues/argus/internal/config"
	"github.com/argues/argus/internal/usage"
)

// usageTestApp returns a minimal *App with a usage store rooted at a temp
// directory and configurable billing rates.
func usageTestApp(t *testing.T, rates config.BillingConfig) *App {
	t.Helper()
	store, err := usage.NewAt(t.TempDir())
	if err != nil {
		t.Fatalf("usage.NewAt: %v", err)
	}
	return &App{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cfg:    &config.OnlineDataConfig{Billing: rates},
		usage:  store,
	}
}

func TestGetUsageSummary_NoStoreReturnsZeroPayload(t *testing.T) {
	a := &App{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cfg: &config.OnlineDataConfig{Billing: config.BillingConfig{
			InputCostPer1M: 0.5, OutputCostPer1M: 1.5, MonthlyBudget: 25,
		}},
		usage: nil,
	}
	got, err := a.GetUsageSummary()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.MonthlyBudget != 25 {
		t.Errorf("expected monthly budget 25, got %v", got.MonthlyBudget)
	}
	// Rates copied through even with no store.
	if got.Rates.InputPerMTokens != 0.5 || got.Rates.OutputPerMTokens != 1.5 {
		t.Errorf("rates not propagated: %+v", got.Rates)
	}
	// All counters zero.
	if got.Today.Calls != 0 || got.Lifetime.Calls != 0 || got.Month.Calls != 0 {
		t.Errorf("expected empty counters, got %+v", got)
	}
}

func TestGetUsageSummary_WiredStoreUsesConfiguredRates(t *testing.T) {
	a := usageTestApp(t, config.BillingConfig{
		InputCostPer1M: 1, OutputCostPer1M: 2, MonthlyBudget: 50,
	})
	if err := a.usage.Record(usage.Record{Model: "deepseek-chat", PromptTokens: 1_000_000, CompletionTokens: 500_000}); err != nil {
		t.Fatalf("seed record: %v", err)
	}

	got, err := a.GetUsageSummary()
	if err != nil {
		t.Fatalf("GetUsageSummary: %v", err)
	}
	if got.MonthlyBudget != 50 {
		t.Errorf("budget passthrough failed: %v", got.MonthlyBudget)
	}
	// Cost = (1M/1M)*$1 + (500k/1M)*$2 = $1 + $1 = $2
	if got.Lifetime.EstCostUSD < 1.99 || got.Lifetime.EstCostUSD > 2.01 {
		t.Errorf("expected ~$2 lifetime cost, got %v", got.Lifetime.EstCostUSD)
	}
	if len(got.ByModel) != 1 || got.ByModel[0].Model != "deepseek-chat" {
		t.Errorf("byModel breakdown wrong: %+v", got.ByModel)
	}
}

func TestClearUsageHistory_NilStoreIsNoop(t *testing.T) {
	a := &App{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cfg:    &config.OnlineDataConfig{},
	}
	if err := a.ClearUsageHistory(); err != nil {
		t.Errorf("nil store should noop, got %v", err)
	}
}

func TestClearUsageHistory_RemovesRecords(t *testing.T) {
	a := usageTestApp(t, config.BillingConfig{})
	if err := a.usage.Record(usage.Record{Model: "x", PromptTokens: 100, CompletionTokens: 50}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := a.ClearUsageHistory(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	got, _ := a.GetUsageSummary()
	if got.Lifetime.Calls != 0 {
		t.Errorf("expected zero calls after clear, got %d", got.Lifetime.Calls)
	}
}

func TestUpdateBillingRates_ValidValuesPersist(t *testing.T) {
	a := usageTestApp(t, config.BillingConfig{})
	if err := a.UpdateBillingRates(0.27, 1.10, 100); err != nil {
		t.Fatalf("UpdateBillingRates: %v", err)
	}
	if a.cfg.Billing.InputCostPer1M != 0.27 ||
		a.cfg.Billing.OutputCostPer1M != 1.10 ||
		a.cfg.Billing.MonthlyBudget != 100 {
		t.Errorf("rates not stored: %+v", a.cfg.Billing)
	}
	// Subsequent GetUsageSummary should reflect the new rates.
	got, _ := a.GetUsageSummary()
	if got.Rates.InputPerMTokens != 0.27 {
		t.Errorf("rate change not visible to summary: %+v", got.Rates)
	}
	if got.MonthlyBudget != 100 {
		t.Errorf("budget change not visible: %v", got.MonthlyBudget)
	}
}

func TestUpdateBillingRates_RejectsNegative(t *testing.T) {
	a := usageTestApp(t, config.BillingConfig{InputCostPer1M: 0.5})
	cases := []struct {
		name                       string
		input, output, monthlyBudget float64
	}{
		{"negative input", -1, 0, 0},
		{"negative output", 0, -1, 0},
		{"negative budget", 0, 0, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := a.UpdateBillingRates(c.input, c.output, c.monthlyBudget); err == nil {
				t.Error("expected error, got nil")
			}
			// State should remain unchanged.
			if a.cfg.Billing.InputCostPer1M != 0.5 {
				t.Errorf("rate mutated despite error: %v", a.cfg.Billing.InputCostPer1M)
			}
		})
	}
}
