package usage

import (
	"math"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewAt(t.TempDir())
	if err != nil {
		t.Fatalf("NewAt: %v", err)
	}
	return s
}

func TestRecord_RoundTripAndSummary(t *testing.T) {
	s := newTestStore(t)
	// Anchor at noon UTC of the current day so the -1h record cannot
	// accidentally cross into yesterday near midnight. Using a hardcoded
	// date here would stop matching `today` once that date passes.
	nowReal := time.Now().UTC()
	now := time.Date(nowReal.Year(), nowReal.Month(), nowReal.Day(), 12, 0, 0, 0, time.UTC)

	rates := Rates{InputPerMTokens: 0.50, OutputPerMTokens: 1.50}

	if err := s.Record(Record{Model: "deepseek-chat", PromptTokens: 1000, CompletionTokens: 200, Timestamp: now}); err != nil {
		t.Fatalf("record 1: %v", err)
	}
	if err := s.Record(Record{Model: "deepseek-chat", PromptTokens: 2000, CompletionTokens: 500, Timestamp: now.Add(-1 * time.Hour)}); err != nil {
		t.Fatalf("record 2: %v", err)
	}

	sum, err := s.Summary(rates)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}

	if sum.Today.Calls != 2 || sum.Today.PromptTokens != 3000 || sum.Today.CompletionTokens != 700 {
		t.Errorf("today totals = %+v", sum.Today)
	}
	if sum.Month.Calls != 2 || sum.Lifetime.Calls != 2 {
		t.Errorf("month/lifetime calls: month=%d lifetime=%d", sum.Month.Calls, sum.Lifetime.Calls)
	}
	// Cost = (3000/1e6)*0.50 + (700/1e6)*1.50 = 0.0015 + 0.00105 = 0.00255
	if math.Abs(sum.Today.EstCostUSD-0.00255) > 1e-9 {
		t.Errorf("today cost = %v, want 0.00255", sum.Today.EstCostUSD)
	}
	if len(sum.ByModel) != 1 || sum.ByModel[0].Model != "deepseek-chat" {
		t.Errorf("byModel = %+v", sum.ByModel)
	}
	if sum.FirstRecordedAt == nil {
		t.Error("firstRecordedAt should be set")
	}
}

func TestRecord_DropsZeroTokenEntries(t *testing.T) {
	s := newTestStore(t)
	if err := s.Record(Record{Model: "x", PromptTokens: 0, CompletionTokens: 0}); err != nil {
		t.Fatalf("record: %v", err)
	}
	sum, _ := s.Summary(Rates{})
	if sum.Lifetime.Calls != 0 {
		t.Errorf("zero-token record should be dropped, got calls=%d", sum.Lifetime.Calls)
	}
}

func TestSummary_EmptyStore(t *testing.T) {
	s := newTestStore(t)
	sum, err := s.Summary(Rates{InputPerMTokens: 1, OutputPerMTokens: 2})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if sum.Today.Calls != 0 || sum.Lifetime.Calls != 0 || len(sum.ByModel) != 0 {
		t.Errorf("expected empty summary, got %+v", sum)
	}
	if sum.FirstRecordedAt != nil {
		t.Error("firstRecordedAt should be nil for empty store")
	}
}

func TestSummary_PerModelBreakdown(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	for _, r := range []Record{
		{Model: "deepseek-chat", PromptTokens: 100, CompletionTokens: 50, Timestamp: now},
		{Model: "deepseek-chat", PromptTokens: 200, CompletionTokens: 60, Timestamp: now},
		{Model: "llama-3.1-8b", PromptTokens: 1000, CompletionTokens: 500, Timestamp: now},
	} {
		if err := s.Record(r); err != nil {
			t.Fatalf("record: %v", err)
		}
	}
	sum, _ := s.Summary(Rates{InputPerMTokens: 1, OutputPerMTokens: 1})
	if len(sum.ByModel) != 2 {
		t.Fatalf("expected 2 model rows, got %d (%+v)", len(sum.ByModel), sum.ByModel)
	}
	// Sorted descending by total tokens — llama (1500) before deepseek (410).
	if sum.ByModel[0].Model != "llama-3.1-8b" {
		t.Errorf("expected llama first, got %q", sum.ByModel[0].Model)
	}
	if sum.ByModel[1].Calls != 2 {
		t.Errorf("expected deepseek calls=2, got %d", sum.ByModel[1].Calls)
	}
}

func TestStore_PersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	s1, err := NewAt(dir)
	if err != nil {
		t.Fatalf("first NewAt: %v", err)
	}
	if err := s1.Record(Record{Model: "m", PromptTokens: 10, CompletionTokens: 5}); err != nil {
		t.Fatalf("record: %v", err)
	}

	s2, err := NewAt(dir)
	if err != nil {
		t.Fatalf("second NewAt: %v", err)
	}
	sum, err := s2.Summary(Rates{})
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if sum.Lifetime.Calls != 1 || sum.Lifetime.PromptTokens != 10 {
		t.Errorf("expected 1 call/10 prompt tokens after reload, got %+v", sum.Lifetime)
	}
}

func TestClear_RemovesEverything(t *testing.T) {
	s := newTestStore(t)
	if err := s.Record(Record{Model: "m", PromptTokens: 100, CompletionTokens: 100}); err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := s.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	sum, _ := s.Summary(Rates{})
	if sum.Lifetime.Calls != 0 || len(sum.ByModel) != 0 {
		t.Errorf("expected empty after clear, got %+v", sum)
	}
}

func TestRecord_NoCostWhenRatesZero(t *testing.T) {
	s := newTestStore(t)
	if err := s.Record(Record{Model: "m", PromptTokens: 1000, CompletionTokens: 1000}); err != nil {
		t.Fatalf("record: %v", err)
	}
	sum, _ := s.Summary(Rates{})
	if sum.Today.EstCostUSD != 0 || sum.Lifetime.EstCostUSD != 0 {
		t.Errorf("expected zero cost when rates are zero, got %+v / %+v", sum.Today, sum.Lifetime)
	}
}

func TestRecord_ConcurrentSafe(t *testing.T) {
	s := newTestStore(t)
	const N = 50
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			_ = s.Record(Record{Model: "m", PromptTokens: 1, CompletionTokens: 1})
			done <- struct{}{}
		}()
	}
	for i := 0; i < N; i++ {
		<-done
	}
	sum, _ := s.Summary(Rates{})
	if sum.Lifetime.Calls != N {
		t.Errorf("expected %d calls after concurrent recording, got %d", N, sum.Lifetime.Calls)
	}
}
