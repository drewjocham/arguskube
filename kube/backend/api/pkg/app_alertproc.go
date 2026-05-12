package pkg

import (
	"context"
	"fmt"
	"time"

	"github.com/argues/argus/internal/alertproc"
	"github.com/argues/argus/internal/alerts"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// alertprocInvestigator adapts the AI agent to the Investigator
// interface. The agent already takes a SendChatMessage-shaped call;
// here we ask it to diagnose a specific alert and return a structured
// Diagnosis.
type alertprocInvestigator struct{ app *App }

func (i *alertprocInvestigator) Investigate(ctx context.Context, a alerts.Alert) (*alerts.Diagnosis, error) {
	if i.app == nil || i.app.agent == nil {
		return nil, fmt.Errorf("AI agent not available")
	}
	// Reuse the existing diagnostic context assembler so the agent
	// sees the same context the user would, and produces the same
	// structured Diagnosis the chat path expects.
	bundle, err := i.app.assembler.Assemble(ctx, a, nil)
	if err != nil {
		return nil, err
	}
	if bundle == nil || bundle.Diagnosis == nil {
		// Bundle without a diagnosis happens when the agent is off /
		// no API key — skip silently rather than logging an error
		// that scares the user.
		return nil, nil
	}
	return bundle.Diagnosis, nil
}

// alertprocNotifier translates alertproc events into the existing
// argus:notification channel. Frontend already handles the kind+
// rerunPayload contract, so the agent's findings flow into the same
// bell as everything else.
type alertprocNotifier struct{ app *App }

func (n *alertprocNotifier) NotifyAlert(severity, title, body string, meta map[string]any) {
	if n.app == nil || n.app.ctx == nil {
		return
	}
	kind := "info"
	switch severity {
	case "warn":
		kind = "warn"
	case "error", "critical":
		kind = "error"
	}
	runtime.EventsEmit(n.app.ctx, "argus:notification", map[string]any{
		"kind":       kind,
		"title":      title,
		"body":       body,
		"rerunnable": false,
		"meta":       meta,
	})
}

func (n *alertprocNotifier) NotifyFatigue(sig alertproc.Signature, count int, lastTitle string) {
	// Specific telemetry hook so we can add a sound / dedicated
	// channel later. For now the NotifyAlert call inside processor
	// already posts the user-facing notification.
	if n.app == nil {
		return
	}
	n.app.logger.Warn("alert fatigue meta-warning fired",
		"signature", string(sig),
		"silenceCount", count,
		"lastTitle", lastTitle,
	)
}

// startAlertProcessor builds the processor, restores any persisted
// profile, and stores it on App. Idempotent so callers don't need to
// guard.
func (a *App) startAlertProcessor() {
	if a.alertproc != nil {
		return
	}
	if a.db == nil {
		return // sqlite not opened — nothing to persist into
	}
	a.alertproc = alertproc.New(a.logger, a.db,
		&alertprocInvestigator{app: a},
		&alertprocNotifier{app: a},
	)
	a.alertproc.LoadPersistedProfile()
}

// processAlertsThroughAgent runs the alert list through the processor
// and returns the deduplicated, dest-friendly slice. Detection code
// calls this BEFORE emitting alerts to the frontend.
//
// Safe to call when alertproc is nil (early-startup path) — it just
// returns the input unchanged.
func (a *App) processAlertsThroughAgent(alerts []alerts.Alert) []alerts.Alert {
	if a.alertproc == nil {
		return alerts
	}
	return a.alertproc.Process(a.ctx, alerts)
}

// AckAlert is bound for the frontend ack button. The user is the
// caller (byAgent=false), so the permission gate doesn't apply.
func (a *App) AckAlert(alertID string, note string) error {
	if a.alertproc == nil {
		return fmt.Errorf("alert processor not initialized")
	}
	return a.alertproc.Ack(alertID, false, note)
}

// SilenceAlert silences a signature for the requested duration. The
// user is the caller; agent-initiated silences flow internally.
func (a *App) SilenceAlert(alertID string, durationSeconds int, reason string) error {
	if a.alertproc == nil {
		return fmt.Errorf("alert processor not initialized")
	}
	dur := time.Duration(durationSeconds) * time.Second
	return a.alertproc.Silence(alertID, false, dur, reason)
}

// MarkAlertIgnored is for the dismiss-without-action UX. Drives the
// fatigue detector.
func (a *App) MarkAlertIgnored(alertID string) {
	if a.alertproc == nil {
		return
	}
	a.alertproc.MarkIgnored(alertID)
}

// GetAgentProfile returns the current permission grant for the UI's
// settings panel.
func (a *App) GetAgentProfile() alertproc.AgentProfile {
	if a.alertproc == nil {
		return alertproc.DefaultProfile()
	}
	return a.alertproc.Profile()
}

// SetAgentProfile validates + persists a new profile.
func (a *App) SetAgentProfile(p alertproc.AgentProfile) error {
	if a.alertproc == nil {
		return fmt.Errorf("alert processor not initialized")
	}
	return a.alertproc.SetProfile(p)
}

// AlertInvestigations returns the agent's investigation history for
// a specific alert. Powers the "what Argus thinks" panel on the
// alert detail.
func (a *App) AlertInvestigations(alertID string, limit int) ([]alertproc.Investigation, error) {
	if a.alertproc == nil {
		return nil, fmt.Errorf("alert processor not initialized")
	}
	return a.alertproc.Investigations(alertID, limit)
}
