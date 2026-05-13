package pkg

import (
	"errors"
	"sync"

	"github.com/argues/argus/internal/userprofile"
)

// userprofileBundle pairs the SQL-backed store with the suggester that
// reads from it. We construct lazily because the *sql.DB isn't always
// available at app-construction time (tests, headless modes), and the
// userprofile features are non-essential — silently degrading to "no
// suggestions" is the correct behaviour when the DB hasn't landed.
type userprofileBundle struct {
	store     *userprofile.Store
	suggester *userprofile.Suggester
}

var (
	userprofileMu        sync.Mutex
	userprofileSingleton = map[*App]*userprofileBundle{}
)

func (a *App) userProfile() *userprofileBundle {
	userprofileMu.Lock()
	defer userprofileMu.Unlock()
	if b, ok := userprofileSingleton[a]; ok {
		return b
	}
	if a.db == nil {
		return nil // gracefully no-op
	}
	store := userprofile.NewStore(a.db, a.logger.With("component", "userprofile"))
	suggester := userprofile.NewSuggester(store, a.logger.With("component", "userprofile.suggester"))
	b := &userprofileBundle{store: store, suggester: suggester}
	userprofileSingleton[a] = b
	return b
}

// RecordView is called by the frontend on every navigation. The
// signature stays loose (strings, not enums) because the view-id
// taxonomy lives in the frontend; the suggester treats them as
// opaque keys.
func (a *App) RecordView(viewID, kubeCtx, namespace string) error {
	b := a.userProfile()
	if b == nil {
		return nil // DB not ready yet; observations during boot are not worth blocking on
	}
	return b.store.RecordNav(a.bgCtx(), viewID, kubeCtx, namespace)
}

// GetNextSuggestion returns 0 or 1 suggestion for the current view.
// The frontend calls this on nav and periodically (every ~30s while
// idle). Returning a structured result lets the frontend distinguish
// "nothing to say" (Suggestion is nil) from "silenced — don't poll
// for a while" (Suppressed flag set).
type NextSuggestionResult struct {
	Suggestion *userprofile.Suggestion `json:"suggestion"`
	Suppressed bool                    `json:"suppressed"`
	// Reason describes a non-error suppression: "budget-spent",
	// "self-throttled", or "" when Suggestion is populated.
	Reason string `json:"reason,omitempty"`
}

func (a *App) GetNextSuggestion(currentView string) (NextSuggestionResult, error) {
	b := a.userProfile()
	if b == nil {
		return NextSuggestionResult{}, nil
	}
	sg, err := b.suggester.NextFor(a.bgCtx(), currentView)
	switch {
	case errors.Is(err, userprofile.ErrBudgetSpent):
		return NextSuggestionResult{Suppressed: true, Reason: "budget-spent"}, nil
	case errors.Is(err, userprofile.ErrSilenced):
		return NextSuggestionResult{Suppressed: true, Reason: "self-throttled"}, nil
	case err != nil:
		return NextSuggestionResult{}, err
	}
	if sg == nil {
		return NextSuggestionResult{}, nil
	}
	// Stamp the audit row immediately so the daily-budget counter is
	// honest even if the user closes the app before clicking.
	_ = b.suggester.RecordShown(a.bgCtx(), sg)
	return NextSuggestionResult{Suggestion: sg}, nil
}

// MuteSuggestion records the user's "don't ask again" choice. The
// suggestion is identified by its MuteKey; the frontend hands back
// whatever GetNextSuggestion sent.
func (a *App) MuteSuggestion(muteKey string) error {
	b := a.userProfile()
	if b == nil {
		return nil
	}
	return b.store.Mute(a.bgCtx(), muteKey)
}

// AcceptSuggestion records that the user clicked the action button.
// Doesn't carry out the action — the frontend handles dispatch — but
// the outcome row drives the auto-self-throttle's "user finds these
// useful" signal so the suggester doesn't go quiet on a happy user.
func (a *App) AcceptSuggestion(muteKey, kind string) error {
	b := a.userProfile()
	if b == nil {
		return nil
	}
	return b.store.RecordAccepted(a.bgCtx(), muteKey, kind)
}

// DismissSuggestion records "closed the card without clicking" so we
// can tune the suggester later if dismissal rates are high.
func (a *App) DismissSuggestion(muteKey, kind string) error {
	b := a.userProfile()
	if b == nil {
		return nil
	}
	return b.store.RecordDismissed(a.bgCtx(), muteKey, kind)
}

// ClearUserActivity is the "Forget my activity" action surfaced in
// Settings. Drops the activity + suggestion-log tables; mutes survive
// (a mute is the user's choice and they presumably still want it).
func (a *App) ClearUserActivity() error {
	b := a.userProfile()
	if b == nil {
		return nil
	}
	return b.store.ClearActivity(a.bgCtx())
}
