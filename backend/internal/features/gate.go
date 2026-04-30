package features

import (
	"errors"

	"github.com/djocham/kube-watcher/internal/config"
)

// ErrProRequired is returned when a pro-only feature is accessed on the free tier.
var ErrProRequired = errors.New("this feature requires KubeWatcher Pro")

// Feature represents a gatable product feature.
type Feature string

const (
	// Free tier features
	FeatureAlerts      Feature = "alerts"
	FeatureClusterView Feature = "cluster_view"
	FeatureLogStream   Feature = "log_stream"
	FeatureTopology    Feature = "topology"

	// Pro tier features
	FeatureAIDiagnostics   Feature = "ai_diagnostics"
	FeatureCascadeCorr     Feature = "cascade_correlation"
	FeatureAnomstack       Feature = "anomstack_anomaly"
	FeatureRunbookAuto     Feature = "runbook_automation"
	FeatureDecisionLog     Feature = "decision_log_context"
	FeatureMultiCluster    Feature = "multi_cluster"
	FeatureExtendedHistory Feature = "extended_history"
	FeatureCustomRunbooks  Feature = "custom_runbooks"
)

// proOnly lists features that require Pro tier.
var proOnly = map[Feature]bool{
	FeatureAIDiagnostics:   true,
	FeatureCascadeCorr:     true,
	FeatureAnomstack:       true,
	FeatureRunbookAuto:     true,
	FeatureDecisionLog:     true,
	FeatureMultiCluster:    true,
	FeatureExtendedHistory: true,
	FeatureCustomRunbooks:  true,
}

// Gate checks whether a feature is available for the current tier.
type Gate struct {
	tier config.Tier
}

// NewGate creates a feature gate from config.
func NewGate(cfg *config.OnlineDataConfig) *Gate {
	return &Gate{tier: cfg.Features.Tier}
}

// Allowed returns true if the feature is accessible at the current tier.
func (g *Gate) Allowed(f Feature) bool {
	if proOnly[f] {
		return g.tier == config.TierPro
	}
	return true
}

// Tier returns the current tier.
func (g *Gate) Tier() config.Tier {
	return g.tier
}

// AllFeatures returns all features with their availability for the current tier.
func (g *Gate) AllFeatures() map[Feature]bool {
	all := []Feature{
		FeatureAlerts, FeatureClusterView, FeatureLogStream, FeatureTopology,
		FeatureAIDiagnostics, FeatureCascadeCorr, FeatureAnomstack, FeatureRunbookAuto,
		FeatureDecisionLog, FeatureMultiCluster, FeatureExtendedHistory, FeatureCustomRunbooks,
	}
	result := make(map[Feature]bool, len(all))
	for _, f := range all {
		result[f] = g.Allowed(f)
	}
	return result
}
