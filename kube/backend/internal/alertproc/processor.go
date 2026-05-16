package alertproc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/argues/argus/internal/alerts"
)

// Processor is the single object the rest of the app talks to about
// alert lifecycle. Detection still happens upstream (k8s.DetectAlerts);
// Processor is the layer between detection and the user — it
// deduplicates, runs the agent, applies user permissions, and tracks
// noise to fire fatigue warnings.
type Processor struct {
	logger  *slog.Logger
	db      *sql.DB
	mu      sync.RWMutex
	profile AgentProfile

	// State keyed by Signature. Live in memory because losing it on
	// restart just means the next firing acts as "first occurrence"
	// — annoying, not unsafe. Persistence would only help across
	// long backend restarts which are rare for a desktop app.
	state map[Signature]*signatureState

	// Hooks let the App layer plug behavior in without this package
	// importing wails/runtime. Keeps the pkg portable + testable.
	investigator Investigator
	notifier     Notifier
}

// signatureState is the running record for one logical alert group.
type signatureState struct {
	FirstSeen   time.Time
	LastSeen    time.Time
	FireCount   int
	SilenceUntil time.Time   // zero when not silenced
	Silences    []time.Time   // history of explicit silences for fatigue
	Ignores     []time.Time   // history of "user dismissed without action"
	LastAlertID string
	LastNotifID string

	// FatigueWarned is set once Argus has fired the meta-alert for
	// this signature, so we don't spam HIM. Reset when the user
	// clears the underlying alert or grants the agent ack rights.
	FatigueWarned bool
}

// Investigator runs the AI agent on a fresh alert. The implementation
// lives in the App layer since it needs the existing diagnostic
// context assembler. Returns the diagnosis or an error.
type Investigator interface {
	Investigate(ctx context.Context, a alerts.Alert) (*alerts.Diagnosis, error)
}

// Notifier surfaces user-visible notifications. The App-side impl
// emits to the existing argus:notification Wails channel.
type Notifier interface {
	NotifyAlert(severity, title, body string, meta map[string]any)
	NotifyFatigue(sig Signature, count int, lastTitle string)
}

// New constructs a Processor with the default profile. Call SetProfile
// to load the user's persisted choice during startup.
func New(logger *slog.Logger, db *sql.DB, inv Investigator, n Notifier) *Processor {
	return &Processor{
		logger:       logger.With("component", "alertproc"),
		db:           db,
		profile:      DefaultProfile(),
		state:        make(map[Signature]*signatureState),
		investigator: inv,
		notifier:     n,
	}
}

// Profile returns a snapshot. Cheap.
func (p *Processor) Profile() AgentProfile {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.profile
}

// SetProfile validates and stores the new profile, then persists it.
// The agent's behavior on the next alert reflects the change.
func (p *Processor) SetProfile(profile AgentProfile) error {
	profile.Sanitize()
	p.mu.Lock()
	p.profile = profile
	p.mu.Unlock()
	return p.persistProfile()
}

// LoadPersistedProfile pulls the last-saved profile out of SQLite. If
// no row exists, falls back to defaults — that's the first-launch path.
func (p *Processor) LoadPersistedProfile() {
	row := p.db.QueryRow(`SELECT body FROM agent_profile WHERE id = 1`)
	var body string
	err := row.Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return // defaults
	}
	if err != nil {
		p.logger.Warn("agent_profile read failed", "error", err.Error())
		return
	}
	var prof AgentProfile
	if err := json.Unmarshal([]byte(body), &prof); err != nil {
		p.logger.Warn("agent_profile parse failed", "error", err.Error())
		return
	}
	prof.Sanitize()
	p.mu.Lock()
	p.profile = prof
	p.mu.Unlock()
}

func (p *Processor) persistProfile() error {
	body, err := json.Marshal(p.profile)
	if err != nil {
		return err
	}
	_, err = p.db.Exec(`
		INSERT INTO agent_profile (id, body, updated_at)
		VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET body = excluded.body, updated_at = excluded.updated_at
	`, string(body), time.Now().Unix())
	return err
}

// Process is the entry point. Pass the raw alert list from
// k8s.DetectAlerts; Process returns the FILTERED list — duplicates
// and silenced alerts are removed so the UI doesn't spam.
//
// Side effects:
//   - Updates per-signature state (FireCount, LastSeen).
//   - Kicks off auto-investigation in a goroutine when the profile allows.
//   - Fires the fatigue meta-alert when thresholds are crossed.
func (p *Processor) Process(ctx context.Context, in []alerts.Alert) []alerts.Alert {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	out := make([]alerts.Alert, 0, len(in))
	prof := p.profile // local copy to release the lock for the investigation goroutine

	for _, a := range in {
		sig := SignatureOf(a)
		st, ok := p.state[sig]
		if !ok {
			st = &signatureState{FirstSeen: now}
			p.state[sig] = st
		}

		// Snapshot pre-update state so the dedup/stale decision sees the
		// previous LastSeen, not the value we're about to write. Without
		// this, now.Sub(st.LastSeen) is always ~0 and the re-fire-after-
		// 5-minutes path is dead code.
		prevFireCount := st.FireCount
		prevLastSeen := st.LastSeen

		st.LastSeen = now
		st.FireCount++
		st.LastAlertID = a.ID

		// Suppress while the user (or agent) silenced this signature.
		if !st.SilenceUntil.IsZero() && now.Before(st.SilenceUntil) {
			// Still suppressed — count as "silenced fire" for fatigue
			// math (the user hasn't acknowledged the underlying
			// problem; they just told us to be quiet about it).
			continue
		}

		// First occurrence of a new signature → kick off investigation.
		// Subsequent fires within a window are deduped from the
		// frontend perspective: we only let the FIRST one through, and
		// re-fire when the signature went quiet for more than 5 minutes.
		isFirst := prevFireCount == 0
		isStale := prevFireCount > 0 && now.Sub(prevLastSeen) > 5*time.Minute

		if isFirst || isStale {
			out = append(out, a)
			if prof.AutoInvestigate && p.investigator != nil {
				p.scheduleInvestigation(ctx, a, sig)
			}
			if prof.AutoDocument {
				p.documentFiring(a, sig, st)
			}
		}
	}

	// Fatigue sweep: any signature with too many silences/ignores
	// gets a meta-alert from Argus, once, until the user resets it
	// (by clearing or granting ack permission).
	for sig, st := range p.state {
		threshold := prof.FatigueThreshold
		count := len(st.Silences) + len(st.Ignores)
		if !st.FatigueWarned && count >= threshold {
			st.FatigueWarned = true
			p.fireFatigueWarning(sig, st)
		}
	}

	return out
}

func (p *Processor) scheduleInvestigation(ctx context.Context, a alerts.Alert, sig Signature) {
	go func(alert alerts.Alert, signature Signature) {
		invCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		diag, err := p.investigator.Investigate(invCtx, alert)
		if err != nil {
			p.logger.Warn("investigation failed",
				"alertID", alert.ID,
				"signature", signature,
				"error", err.Error(),
			)
			p.recordInvestigation(alert, signature, "", err.Error())
			return
		}
		body := ""
		if diag != nil {
			body = diag.Hypothesis
		}
		p.recordInvestigation(alert, signature, body, "")
		// Surface the agent's findings as a notification. The user can
		// always re-read them on the alert detail page.
		if p.notifier != nil && body != "" {
			p.notifier.NotifyAlert("info",
				fmt.Sprintf("Agent investigated: %s", alert.Name),
				body,
				map[string]any{
					"alertID":   alert.ID,
					"signature": string(signature),
					"namespace": alert.Namespace,
				},
			)
		}
	}(a, sig)
}

func (p *Processor) documentFiring(a alerts.Alert, sig Signature, st *signatureState) {
	body, _ := json.Marshal(map[string]any{
		"alertID":     a.ID,
		"signature":   string(sig),
		"name":        a.Name,
		"severity":    a.Severity,
		"namespace":   a.Namespace,
		"podName":     a.PodName,
		"firstSeen":   st.FirstSeen,
		"fireCount":   st.FireCount,
		"firedAt":     time.Now().Format(time.RFC3339),
	})
	if _, err := p.db.Exec(`
		INSERT INTO alert_events (signature, alert_id, kind, body, created_at)
		VALUES (?, ?, 'fired', ?, ?)
	`, string(sig), a.ID, string(body), time.Now().Unix()); err != nil {
		p.logger.Warn("alert_events insert failed", "error", err.Error())
	}
}

func (p *Processor) recordInvestigation(a alerts.Alert, sig Signature, hypothesis, errMsg string) {
	body, _ := json.Marshal(map[string]any{
		"alertID":    a.ID,
		"signature":  string(sig),
		"hypothesis": hypothesis,
		"error":      errMsg,
		"recordedAt": time.Now().Format(time.RFC3339),
	})
	if _, err := p.db.Exec(`
		INSERT INTO alert_events (signature, alert_id, kind, body, created_at)
		VALUES (?, ?, 'investigated', ?, ?)
	`, string(sig), a.ID, string(body), time.Now().Unix()); err != nil {
		p.logger.Warn("alert_events insert failed", "error", err.Error())
	}
}

// Ack marks the alert acknowledged. Returns ErrPermissionDenied if the
// caller is the agent and the profile doesn't allow it (the boolean
// `byAgent` separates user-initiated from agent-initiated acks).
func (p *Processor) Ack(alertID string, byAgent bool, note string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if byAgent && !p.profile.CanAck {
		return ErrPermissionDenied
	}
	body, _ := json.Marshal(map[string]any{
		"alertID":  alertID,
		"byAgent":  byAgent,
		"note":     note,
		"ackedAt":  time.Now().Format(time.RFC3339),
	})
	_, err := p.db.Exec(`
		INSERT INTO alert_events (signature, alert_id, kind, body, created_at)
		VALUES ('', ?, 'ack', ?, ?)
	`, alertID, string(body), time.Now().Unix())
	return err
}

// Silence suppresses every future fire of this signature for `dur`.
// The agent can only call this when the profile allows.
func (p *Processor) Silence(alertID string, byAgent bool, dur time.Duration, reason string) error {
	if byAgent && !p.profile.CanSilence {
		return ErrPermissionDenied
	}
	if dur <= 0 {
		dur = p.profile.SilenceWindow
	}
	if dur > 24*time.Hour {
		dur = 24 * time.Hour
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	// Find the signature for this alert from the most recent state.
	// Linear scan — N is bounded by registered signatures (small).
	var sig Signature
	var st *signatureState
	for s, x := range p.state {
		if x.LastAlertID == alertID {
			sig, st = s, x
			break
		}
	}
	if st == nil {
		return fmt.Errorf("alertproc: alert %q not found in state", alertID)
	}
	st.SilenceUntil = time.Now().Add(dur)
	st.Silences = append(st.Silences, time.Now())

	body, _ := json.Marshal(map[string]any{
		"alertID":      alertID,
		"signature":    string(sig),
		"byAgent":      byAgent,
		"durationSec":  int(dur.Seconds()),
		"reason":       reason,
		"silencedAt":   time.Now().Format(time.RFC3339),
	})
	if _, err := p.db.Exec(`
		INSERT INTO alert_events (signature, alert_id, kind, body, created_at)
		VALUES (?, ?, 'silenced', ?, ?)
	`, string(sig), alertID, string(body), time.Now().Unix()); err != nil {
		p.logger.Warn("alert_events insert failed", "error", err.Error())
	}
	return nil
}

// MarkIgnored is called when the user dismissed an alert without
// action. Used by the fatigue detector — repeated ignores of the same
// signature is the signal Argus watches for.
func (p *Processor) MarkIgnored(alertID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, st := range p.state {
		if st.LastAlertID == alertID {
			st.Ignores = append(st.Ignores, time.Now())
			return
		}
	}
}

func (p *Processor) fireFatigueWarning(sig Signature, st *signatureState) {
	if p.notifier == nil {
		return
	}
	count := len(st.Silences) + len(st.Ignores)
	body := fmt.Sprintf(
		"Argus has watched alert signature `%s` get silenced or dismissed **%d times** without action.\n\n"+
			"Caution — your alerts and warnings are losing value when nobody acts on them.\n\n"+
			"I'd suggest one of:\n"+
			"- Tighten or relax the threshold so fewer false-positives fire\n"+
			"- Route the alert to a different channel\n"+
			"- Mute the alert at the source if it's truly noise\n\n"+
			"I will not change anything without your permission. If you want me to silence repeats automatically the next time this happens, grant me **Can Silence** in Settings → Agent Profile.",
		sig, count,
	)
	p.notifier.NotifyFatigue(sig, count, "Alert fatigue detected")
	p.notifier.NotifyAlert("warn",
		"Caution: alerts are losing value",
		body,
		map[string]any{
			"signature":      string(sig),
			"silenceCount":   len(st.Silences),
			"ignoreCount":    len(st.Ignores),
			"firedAt":        time.Now().Format(time.RFC3339),
			"argusFatigue":   true,
			"profileCanSilence": p.profile.CanSilence,
		},
	)
}

// Investigations returns the persisted investigation history for an
// alert. Used by the alert-detail UI to show what the agent has
// already concluded without re-running.
func (p *Processor) Investigations(alertID string, limit int) ([]Investigation, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := p.db.Query(`
		SELECT body, created_at FROM alert_events
		WHERE alert_id = ? AND kind = 'investigated'
		ORDER BY created_at DESC
		LIMIT ?
	`, alertID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Investigation
	for rows.Next() {
		var body string
		var ts int64
		if err := rows.Scan(&body, &ts); err != nil {
			return nil, err
		}
		var inv Investigation
		_ = json.Unmarshal([]byte(body), &inv)
		inv.RecordedAt = time.Unix(ts, 0)
		out = append(out, inv)
	}
	return out, rows.Err()
}

// Investigation is the persisted shape; mirrors the JSON columns
// recordInvestigation writes.
type Investigation struct {
	AlertID    string    `json:"alertID"`
	Signature  string    `json:"signature"`
	Hypothesis string    `json:"hypothesis"`
	Error      string    `json:"error"`
	RecordedAt time.Time `json:"recordedAt"`
}
