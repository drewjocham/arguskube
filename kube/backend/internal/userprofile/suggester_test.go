package userprofile

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubClock lets us pin "now" at a known instant. The morning-playbook
// only fires within the first 10 minutes of a local day, so tests
// that target other candidates need to advance past 09:11.
type stubClock struct{ t time.Time }

func (s *stubClock) Now() time.Time { return s.t }

// fastInsert is a test helper that injects a row at a chosen timestamp.
// We bypass RecordNav() so we can backdate observations and exercise
// the morning-playbook with several days of history.
func fastInsert(t *testing.T, s *Store, ts time.Time, viewID string) {
	t.Helper()
	ctx := context.Background()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO user_activity (ts, kind, view_id) VALUES (?, ?, ?)`,
		ts.Unix(), KindNav, viewID,
	)
	if err != nil {
		t.Fatalf("fastInsert: %v", err)
	}
}

func newSuggesterAtNoon(t *testing.T) (*Suggester, *Store, *stubClock) {
	t.Helper()
	store := NewStore(newTestDB(t), discardLogger())
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC) // noon — past morning window
	clk := &stubClock{t: now}
	sg := NewSuggester(store, discardLogger()).WithClock(clk)
	return sg, store, clk
}

func newSuggesterAtMorning(t *testing.T) (*Suggester, *Store, *stubClock) {
	t.Helper()
	store := NewStore(newTestDB(t), discardLogger())
	now := time.Date(2026, 5, 13, 9, 5, 0, 0, time.UTC) // 09:05 — inside morning window
	clk := &stubClock{t: now}
	sg := NewSuggester(store, discardLogger()).WithClock(clk)
	return sg, store, clk
}

func TestSuggester_EmptyStoreEmitsNothing(t *testing.T) {
	sg, _, _ := newSuggesterAtNoon(t)
	res, err := sg.NextFor(context.Background(), "alerts")
	if err != nil {
		t.Fatalf("nextFor: %v", err)
	}
	if res != nil {
		t.Errorf("empty store should produce no suggestion, got %+v", res)
	}
}

func TestSuggester_MorningPlaybook_NeedsThreeDays(t *testing.T) {
	sg, store, clk := newSuggesterAtMorning(t)
	// Two days of history → not enough.
	for d := 1; d <= 2; d++ {
		day := time.Date(2026, 5, d, 9, 1, 0, 0, time.UTC)
		fastInsert(t, store, day, "alerts")
	}
	res, _ := sg.NextFor(context.Background(), "pods")
	if res != nil {
		t.Errorf("want no suggestion with 2 days, got %+v", res)
	}
	// Third day with the same first-of-day view → fires.
	fastInsert(t, store, time.Date(2026, 5, 3, 9, 1, 0, 0, time.UTC), "alerts")
	fastInsert(t, store, time.Date(2026, 5, 4, 9, 1, 0, 0, time.UTC), "alerts")
	clk.t = time.Date(2026, 5, 13, 9, 5, 0, 0, time.UTC)
	res, err := sg.NextFor(context.Background(), "pods")
	if err != nil {
		t.Fatalf("nextFor: %v", err)
	}
	if res == nil {
		t.Fatal("want suggestion, got nil")
	}
	if res.Kind != "pre-stage" {
		t.Errorf("want pre-stage kind, got %s", res.Kind)
	}
	if res.ActionID != "userprofile.open-view:alerts" {
		t.Errorf("unexpected action id: %s", res.ActionID)
	}
	if res.ExpiresInS == 0 {
		t.Errorf("ExpiresInS should be populated from the budget")
	}
}

func TestSuggester_MorningPlaybook_SuppressedWhenAlreadyOnView(t *testing.T) {
	sg, store, _ := newSuggesterAtMorning(t)
	for d := 1; d <= 5; d++ {
		fastInsert(t, store, time.Date(2026, 5, d, 9, 1, 0, 0, time.UTC), "alerts")
	}
	res, _ := sg.NextFor(context.Background(), "alerts")
	if res != nil {
		t.Errorf("should not nudge to a view the user is already on, got %+v", res)
	}
}

func TestSuggester_MorningPlaybook_OnlyInMorningWindow(t *testing.T) {
	sg, store, _ := newSuggesterAtNoon(t) // noon → past window
	for d := 1; d <= 5; d++ {
		fastInsert(t, store, time.Date(2026, 5, d, 9, 1, 0, 0, time.UTC), "alerts")
	}
	res, _ := sg.NextFor(context.Background(), "pods")
	if res != nil {
		t.Errorf("morning playbook should not fire at noon, got %+v", res)
	}
}

func TestSuggester_NextViewFromHere_NeedsClearMajority(t *testing.T) {
	sg, store, _ := newSuggesterAtNoon(t)
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	// Build 6 transitions from "alerts": 5 → pods, 1 → logs (≥60%).
	for i := 0; i < 5; i++ {
		fastInsert(t, store, now.Add(time.Duration(i*2)*time.Minute), "alerts")
		fastInsert(t, store, now.Add(time.Duration(i*2+1)*time.Minute), "pods")
	}
	fastInsert(t, store, now.Add(20*time.Minute), "alerts")
	fastInsert(t, store, now.Add(21*time.Minute), "logs")

	res, err := sg.NextFor(context.Background(), "alerts")
	if err != nil {
		t.Fatalf("nextFor: %v", err)
	}
	if res == nil {
		t.Fatal("want suggestion, got nil")
	}
	if res.ActionID != "userprofile.open-view:pods" {
		t.Errorf("expected next-view suggestion to point at pods, got %s", res.ActionID)
	}
	if res.Kind != "inline-tip" {
		t.Errorf("want inline-tip kind, got %s", res.Kind)
	}
}

func TestSuggester_NextViewFromHere_NoSuggestionWithoutMajority(t *testing.T) {
	sg, store, _ := newSuggesterAtNoon(t)
	now := time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC)
	// Diffuse: every transition goes somewhere different.
	for i, v := range []string{"pods", "logs", "deployments", "events", "nodes"} {
		fastInsert(t, store, now.Add(time.Duration(i*2)*time.Minute), "alerts")
		fastInsert(t, store, now.Add(time.Duration(i*2+1)*time.Minute), v)
	}
	res, _ := sg.NextFor(context.Background(), "alerts")
	if res != nil {
		t.Errorf("diffuse transitions should not suggest, got %+v", res)
	}
}

func TestSuggester_DailyBudget(t *testing.T) {
	sg, store, _ := newSuggesterAtMorning(t)
	// Build a morning-playbook pattern.
	for d := 1; d <= 6; d++ {
		fastInsert(t, store, time.Date(2026, 5, d, 9, 1, 0, 0, time.UTC), "alerts")
	}
	// Pre-fill today's shown count to the cap.
	for i := 0; i < DefaultBudget.MaxPerDay; i++ {
		_ = store.RecordShown(context.Background(), "k", "kind")
	}
	res, err := sg.NextFor(context.Background(), "pods")
	if !errors.Is(err, ErrBudgetSpent) {
		t.Errorf("want ErrBudgetSpent, got err=%v res=%v", err, res)
	}
}

func TestSuggester_MuteSuppressesSuggestion(t *testing.T) {
	sg, store, _ := newSuggesterAtMorning(t)
	for d := 1; d <= 5; d++ {
		fastInsert(t, store, time.Date(2026, 5, d, 9, 1, 0, 0, time.UTC), "alerts")
	}
	if err := store.Mute(context.Background(), "userprofile.morning:alerts"); err != nil {
		t.Fatalf("mute: %v", err)
	}
	res, _ := sg.NextFor(context.Background(), "pods")
	if res != nil {
		t.Errorf("muted suggestion should not surface, got %+v", res)
	}
}

func TestSuggester_SelfThrottle(t *testing.T) {
	sg, store, _ := newSuggesterAtMorning(t)
	for d := 1; d <= 5; d++ {
		fastInsert(t, store, time.Date(2026, 5, d, 9, 1, 0, 0, time.UTC), "alerts")
	}
	ctx := context.Background()
	// Shown 4 times, muted 4 times → 50% mute rate, above threshold.
	for i := 0; i < 4; i++ {
		_ = store.RecordShown(ctx, "k", "kind")
		_ = store.Mute(ctx, "kx")
	}
	res, err := sg.NextFor(ctx, "pods")
	if !errors.Is(err, ErrSilenced) {
		t.Errorf("want ErrSilenced, got err=%v res=%v", err, res)
	}
}

func TestSuggester_RecordShownLogsRow(t *testing.T) {
	sg, store, _ := newSuggesterAtMorning(t)
	ctx := context.Background()
	sgn := &Suggestion{Kind: "pre-stage", MuteKey: "x"}
	if err := sg.RecordShown(ctx, sgn); err != nil {
		t.Fatalf("record shown: %v", err)
	}
	n, _ := store.CountShownSince(ctx, time.Unix(0, 0))
	if n != 1 {
		t.Errorf("want 1 shown row, got %d", n)
	}
}

func TestDominant_DeterministicTieBreak(t *testing.T) {
	winner, n := dominant(map[string]int{"a": 3, "b": 3, "c": 1})
	if winner != "a" || n != 3 {
		t.Errorf("want a/3 (lex tie-break), got %s/%d", winner, n)
	}
}
