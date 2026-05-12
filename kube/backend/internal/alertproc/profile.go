// Package alertproc handles the post-detection lifecycle of an alert:
// dedupe, auto-investigation, ack/silence (when the agent profile
// permits), and the alert-fatigue meta-warning.
//
// Argus's defaults are conservative: he ALWAYS documents, but never
// changes alert state without explicit permission. The AgentProfile
// is the single source of truth for what he's allowed to do.
package alertproc

import (
	"errors"
	"time"
)

// AgentProfile is the user's permission grant to the AI agent. Stored
// in the local SQLite db; defaults are read-only "document and watch."
type AgentProfile struct {
	// AutoInvestigate kicks off an AI investigation as soon as a new
	// (deduped) alert fires. The findings are always persisted, so
	// the user can review them in the alert detail later.
	AutoInvestigate bool `json:"autoInvestigate"`

	// AutoDocument is independent of AutoInvestigate: even when the
	// AI is off, the agent records a structured trail of what it
	// observed (severity, timestamps, dedupe stats, suggested
	// adjustments) for every alert. Default is true. The user can
	// opt out, but the fatigue detector relies on this data.
	AutoDocument bool `json:"autoDocument"`

	// CanAck lets the agent acknowledge alerts on the user's behalf.
	// OFF by default. When OFF, the agent can still suggest an ack
	// in its findings — it just can't act.
	CanAck bool `json:"canAck"`

	// CanSilence lets the agent silence alerts (suppress duplicates
	// for a window). OFF by default. When OFF, the agent can suggest
	// silencing in its findings.
	CanSilence bool `json:"canSilence"`

	// CanAdjustParams lets the agent suggest AND apply changes to
	// alert thresholds when it spots a noise pattern. OFF by default
	// — most teams don't want algorithmic threshold drift.
	CanAdjustParams bool `json:"canAdjustParams"`

	// SilenceWindow is how long a silenced alert stays suppressed.
	// 0 means "use default" (1 hour). Capped at 24h regardless.
	SilenceWindow time.Duration `json:"silenceWindow"`

	// FatigueThreshold: how many consecutive silences/ignores of the
	// same alert signature before Argus fires the "noise warning"
	// meta-alert. Default 5.
	FatigueThreshold int `json:"fatigueThreshold"`
}

// DefaultProfile is the conservative starting point: the agent
// documents and investigates but cannot mutate alert state.
func DefaultProfile() AgentProfile {
	return AgentProfile{
		AutoInvestigate:  true,
		AutoDocument:     true,
		CanAck:           false,
		CanSilence:       false,
		CanAdjustParams:  false,
		SilenceWindow:    1 * time.Hour,
		FatigueThreshold: 5,
	}
}

// Sanitize clamps user-provided values to safe ranges. Called before
// persisting anywhere. Cheap to call repeatedly.
func (p *AgentProfile) Sanitize() {
	if p.SilenceWindow <= 0 {
		p.SilenceWindow = 1 * time.Hour
	}
	if p.SilenceWindow > 24*time.Hour {
		p.SilenceWindow = 24 * time.Hour
	}
	if p.FatigueThreshold <= 0 {
		p.FatigueThreshold = 5
	}
	if p.FatigueThreshold > 100 {
		p.FatigueThreshold = 100
	}
}

// ErrPermissionDenied surfaces when the user (or agent) tries an
// action the profile doesn't allow.
var ErrPermissionDenied = errors.New("alertproc: action not permitted by agent profile")
