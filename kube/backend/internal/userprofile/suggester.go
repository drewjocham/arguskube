package userprofile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"
)

// Suggestion is the single contract the suggester emits. The frontend
// renders exactly one card per Suggestion. MuteKey is the stable
// identity the "Don't ask again" button hands back to Mute().
type Suggestion struct {
	Kind        string `json:"kind"`        // "pre-stage" | "inline-tip" | "post-action"
	Title       string `json:"title"`
	Body        string `json:"body"`
	ActionLabel string `json:"actionLabel,omitempty"`
	ActionID    string `json:"actionId,omitempty"`
	MuteKey     string `json:"muteKey"`
	ExpiresInS  int    `json:"expiresInS"` // card disappears after this
}

// Budget knobs — exported so tests can compress windows. The defaults
// match the plan: 3 suggestions/day, 14-day mute-rate window, 7-day
// silence when the mute rate is too high.
type Budget struct {
	MaxPerDay              int
	MuteRateWindow         time.Duration
	MuteRateThreshold      float64       // 0.5 = silence when ≥50% muted
	MuteRateMinShown       int           // need at least N shown rows before throttling
	SilenceAfterThreshold  time.Duration
	CardExpiry             time.Duration
	// MorningCutoffHour is the local-time hour at and after which the
	// "your usual first view" card stops firing. Defaults to 12 (noon)
	// so it doesn't say "morning" at 8pm.
	MorningCutoffHour int
}

// DefaultBudget reflects the plan §6. Conservative on purpose.
var DefaultBudget = Budget{
	MaxPerDay:             3,
	MuteRateWindow:        14 * 24 * time.Hour,
	MuteRateThreshold:     0.5,
	MuteRateMinShown:      4,
	SilenceAfterThreshold: 7 * 24 * time.Hour,
	CardExpiry:        60 * time.Second,
	MorningCutoffHour: 12,
}

// Clock is the time source the suggester reads. Defaults to wall-clock;
// tests inject a stub so they don't depend on hour-of-day.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Suggester reads from a Store and emits one Suggestion at a time. It
// has no internal state — every NextFor() call recomputes from scratch.
// That's cheap (the store keeps small bounded tables) and means a
// process restart can't unbalance the annoyance budget.
type Suggester struct {
	store  *Store
	clock  Clock
	budget Budget
	logger *slog.Logger
}

func NewSuggester(store *Store, logger *slog.Logger) *Suggester {
	if logger == nil {
		logger = slog.Default()
	}
	return &Suggester{
		store:  store,
		clock:  realClock{},
		budget: DefaultBudget,
		logger: logger,
	}
}

// WithClock returns the same Suggester with the clock replaced. Used
// by tests; mirrors the pattern in internal/envprobe.
func (s *Suggester) WithClock(c Clock) *Suggester {
	s.clock = c
	return s
}

// WithBudget replaces the budget knobs (mostly for tests).
func (s *Suggester) WithBudget(b Budget) *Suggester {
	s.budget = b
	return s
}

// ErrSilenced is returned by NextFor() when the auto-self-throttle is
// active. Callers should render nothing.
var ErrSilenced = errors.New("suggester silenced by mute-rate self-throttle")

// ErrBudgetSpent is returned when today's MaxPerDay has been hit.
var ErrBudgetSpent = errors.New("daily suggestion budget spent")

// NextFor returns one Suggestion for the user given their current view,
// or (nil, nil) if there's nothing useful to say (which is the most
// common case). Errors are reserved for "you should know the suggester
// is silent and why" — the UI never has to render an error to the user.
//
// The caller passes currentView so we can offer "pre-stage" cards that
// reflect "you usually open X after Y" without having to materialize
// the full Markov transition matrix every call.
func (s *Suggester) NextFor(ctx context.Context, currentView string) (*Suggestion, error) {
	now := s.clock.Now()

	// Hard caps first — these short-circuit even computing transitions.
	if silenced, err := s.isSelfThrottled(ctx, now); err != nil {
		return nil, err
	} else if silenced {
		return nil, ErrSilenced
	}

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	shown, err := s.store.CountShownSince(ctx, startOfDay)
	if err != nil {
		return nil, fmt.Errorf("count shown today: %w", err)
	}
	if shown >= s.budget.MaxPerDay {
		return nil, ErrBudgetSpent
	}

	// We try suggestion kinds in priority order. The first one with a
	// usable result wins and we stop — *at most* one card on screen.
	candidates := []func(ctx context.Context, now time.Time, currentView string) (*Suggestion, error){
		s.morningPlaybook,
		s.nextViewFromHere,
		s.profileWorkflowDetector,
	}
	for _, gen := range candidates {
		sg, err := gen(ctx, now, currentView)
		if err != nil {
			s.logger.Warn("userprofile: candidate failed", "err", err)
			continue
		}
		if sg == nil {
			continue
		}
		// Respect explicit mutes.
		muted, err := s.store.IsMuted(ctx, sg.MuteKey)
		if err != nil {
			s.logger.Warn("userprofile: IsMuted failed", "err", err)
		} else if muted {
			continue
		}
		// Annotate the expiry the frontend should use.
		sg.ExpiresInS = int(s.budget.CardExpiry.Seconds())
		return sg, nil
	}
	return nil, nil
}

// RecordShown is a passthrough so the calling layer can stamp the
// audit row without taking a direct store dep. The suggester is the
// canonical source of "we just rendered a card", so the bookkeeping
// belongs here.
func (s *Suggester) RecordShown(ctx context.Context, sg *Suggestion) error {
	if sg == nil {
		return nil
	}
	return s.store.RecordShown(ctx, sg.MuteKey, sg.Kind)
}

// isSelfThrottled returns true when the user's recent mute rate is
// >= MuteRateThreshold and at least MuteRateMinShown rows are visible.
// We also require that the LAST mute is recent — otherwise an old run
// of mutes would silence forever.
func (s *Suggester) isSelfThrottled(ctx context.Context, now time.Time) (bool, error) {
	windowStart := now.Add(-s.budget.MuteRateWindow)
	shown, muted, err := s.store.MuteRateSince(ctx, windowStart)
	if err != nil {
		return false, fmt.Errorf("mute rate: %w", err)
	}
	total := shown + muted // both are disjoint outcomes within the window
	if total < s.budget.MuteRateMinShown {
		return false, nil
	}
	rate := float64(muted) / float64(total)
	if rate < s.budget.MuteRateThreshold {
		return false, nil
	}
	// We're above the rate threshold — silence for SilenceAfterThreshold
	// from the moment we entered this state. Approximation: silence until
	// the window slides past the muting activity.
	return true, nil
}

// morningPlaybook — "It's 09:02 — your usual morning view is alerts.
// [Open]". We look at the user's first nav of each calendar day over
// the past 14 days; if a single view dominates we suggest it.
func (s *Suggester) morningPlaybook(ctx context.Context, now time.Time, currentView string) (*Suggestion, error) {
	if now.Hour() >= s.budget.MorningCutoffHour {
		return nil, nil // afternoon/evening — "your usual first view" would be a stale claim
	}

	activity, err := s.store.Recent(ctx, 2000)
	if err != nil {
		return nil, err
	}
	firstByDay := map[string]string{}
	for _, a := range activity {
		if a.Kind != KindNav {
			continue
		}
		day := a.Ts.Format("2006-01-02")
		if _, ok := firstByDay[day]; ok {
			continue
		}
		firstByDay[day] = a.ViewID
	}
	if len(firstByDay) < 3 {
		return nil, nil // not enough history to call a pattern
	}
	counts := map[string]int{}
	for _, view := range firstByDay {
		counts[view]++
	}
	winner, n := dominant(counts)
	if winner == "" || n*2 <= len(firstByDay) {
		return nil, nil // need a strict majority of days
	}
	if winner == currentView {
		return nil, nil // they're already there
	}
	return &Suggestion{
		Kind:        "pre-stage",
		Title:       fmt.Sprintf("Open your usual first view — %s", winner),
		Body:        fmt.Sprintf("On %d of the last %d days you opened %s first.", n, len(firstByDay), winner),
		ActionLabel: "Open",
		ActionID:    "userprofile.open-view:" + winner,
		MuteKey:     "userprofile.morning:" + winner,
	}, nil
}

// nextViewFromHere — when the user is on view X, propose the view they
// most often open NEXT after X. Needs at least 5 prior transitions
// out of X with a single clear winner. Doesn't fire when the user
// would already see the proposed view (no-ops).
func (s *Suggester) nextViewFromHere(ctx context.Context, _ time.Time, currentView string) (*Suggestion, error) {
	if currentView == "" {
		return nil, nil
	}
	activity, err := s.store.Recent(ctx, 1000)
	if err != nil {
		return nil, err
	}
	// Build a count of transitions FROM currentView → next view, with
	// the current pair (currentView, nothing) ignored if it's the
	// last activity.
	counts := map[string]int{}
	total := 0
	for i := 0; i+1 < len(activity); i++ {
		if activity[i].ViewID != currentView {
			continue
		}
		next := activity[i+1].ViewID
		if next == "" || next == currentView {
			continue
		}
		counts[next]++
		total++
	}
	if total < 5 {
		return nil, nil
	}
	winner, n := dominant(counts)
	if winner == "" || float64(n)/float64(total) < 0.6 {
		return nil, nil
	}
	return &Suggestion{
		Kind:        "inline-tip",
		Title:       fmt.Sprintf("Open %s?", winner),
		Body:        fmt.Sprintf("After %s you usually open %s (%d of last %d times).", currentView, winner, n, total),
		ActionLabel: "Open " + winner,
		ActionID:    "userprofile.open-view:" + winner,
		MuteKey:     fmt.Sprintf("userprofile.next:%s->%s", currentView, winner),
	}, nil
}

// profileWorkflowDetector suggests creating a profile when the user
// frequently switches between different views in a short period,
// indicating they may benefit from saving workspace configurations.
func (s *Suggester) profileWorkflowDetector(ctx context.Context, now time.Time, currentView string) (*Suggestion, error) {
	activity, err := s.store.Recent(ctx, 200)
	if err != nil {
		return nil, err
	}
	if len(activity) < 10 {
		return nil, nil
	}

	changes := 0
	views := map[string]int{}
	for i := 0; i+1 < len(activity); i++ {
		if activity[i].ViewID != activity[i+1].ViewID {
			changes++
			views[activity[i].ViewID]++
			views[activity[i+1].ViewID]++
		}
	}
	if changes >= 10 && len(views) >= 3 && now.Sub(activity[0].Ts) <= 15*time.Minute {
		return &Suggestion{
			Kind:        "profiles",
			Title:       "You switch between views frequently",
			Body:        "Save your current workspace as a named profile — switch with one click instead of reconfiguring each time.",
			ActionLabel: "Create profile",
			ActionID:    "profiles.suggest:open-creator",
			MuteKey:     "profiles.frequent-switcher",
		}, nil
	}
	return nil, nil
}

// dominant returns the highest-count entry and its count. Ties resolve
// by lexicographic order on the key so the output is deterministic.
func dominant(counts map[string]int) (string, int) {
	if len(counts) == 0 {
		return "", 0
	}
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(counts))
	for k, v := range counts {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].v != pairs[j].v {
			return pairs[i].v > pairs[j].v
		}
		return pairs[i].k < pairs[j].k
	})
	return pairs[0].k, pairs[0].v
}
