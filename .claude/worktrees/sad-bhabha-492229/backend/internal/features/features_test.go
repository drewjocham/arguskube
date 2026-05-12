package features

import (
	"testing"

	"github.com/argues/kube-watcher/internal/config"
)

func TestNewGateFree(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Features: config.FeaturesConfig{
			Tier: config.TierFree,
		},
	}
	gate := NewGate(cfg)
	if gate == nil {
		t.Fatal("NewGate returned nil")
	}
	if gate.Tier() != config.TierFree {
		t.Errorf("gate.Tier() = %q, want %q", gate.Tier(), config.TierFree)
	}
}

func TestNewGatePro(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Features: config.FeaturesConfig{
			Tier: config.TierPro,
		},
	}
	gate := NewGate(cfg)
	if gate == nil {
		t.Fatal("NewGate returned nil")
	}
	if gate.Tier() != config.TierPro {
		t.Errorf("gate.Tier() = %q, want %q", gate.Tier(), config.TierPro)
	}
}

func TestGateAllowedFree(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Features: config.FeaturesConfig{
			Tier: config.TierFree,
		},
	}
	gate := NewGate(cfg)

	freeFeatures := []Feature{
		FeatureAlerts,
		FeatureClusterView,
		FeatureLogStream,
		FeatureTopology,
		FeatureCascadeCorr,
		FeatureAnomstack,
	}
	proFeatures := []Feature{
		FeatureAIDiagnostics,
		FeatureRunbookAuto,
		FeatureDecisionLog,
		FeatureMultiCluster,
		FeatureExtendedHistory,
		FeatureCustomRunbooks,
		FeatureArgusCD,
	}

	t.Run("free tier allows free features", func(t *testing.T) {
		for _, f := range freeFeatures {
			if !gate.Allowed(f) {
				t.Errorf("free gate should allow feature %q", f)
			}
		}
	})

	t.Run("free tier denies pro features", func(t *testing.T) {
		for _, f := range proFeatures {
			if gate.Allowed(f) {
				t.Errorf("free gate should deny feature %q", f)
			}
		}
	})
}

func TestGateAllowedPro(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Features: config.FeaturesConfig{
			Tier: config.TierPro,
		},
	}
	gate := NewGate(cfg)

	allFeatures := []Feature{
		FeatureAlerts,
		FeatureClusterView,
		FeatureLogStream,
		FeatureTopology,
		FeatureCascadeCorr,
		FeatureAnomstack,
		FeatureAIDiagnostics,
		FeatureRunbookAuto,
		FeatureDecisionLog,
		FeatureMultiCluster,
		FeatureExtendedHistory,
		FeatureCustomRunbooks,
		FeatureArgusCD,
	}

	for _, f := range allFeatures {
		t.Run(string(f), func(t *testing.T) {
			if !gate.Allowed(f) {
				t.Errorf("pro gate should allow feature %q", f)
			}
		})
	}
}

func TestAllFeaturesFree(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Features: config.FeaturesConfig{
			Tier: config.TierFree,
		},
	}
	gate := NewGate(cfg)

	all := gate.AllFeatures()

	// Free features should be allowed.
	for _, f := range []Feature{
		FeatureAlerts, FeatureClusterView, FeatureLogStream,
		FeatureTopology, FeatureCascadeCorr, FeatureAnomstack,
	} {
		if !all[f] {
			t.Errorf("AllFeatures: free feature %q should be true on free tier", f)
		}
	}

	// Pro features should be denied.
	for _, f := range []Feature{
		FeatureAIDiagnostics, FeatureRunbookAuto, FeatureDecisionLog,
		FeatureMultiCluster, FeatureExtendedHistory, FeatureCustomRunbooks,
		FeatureArgusCD,
	} {
		if all[f] {
			t.Errorf("AllFeatures: pro feature %q should be false on free tier", f)
		}
	}
}

func TestAllFeaturesPro(t *testing.T) {
	cfg := &config.OnlineDataConfig{
		Features: config.FeaturesConfig{
			Tier: config.TierPro,
		},
	}
	gate := NewGate(cfg)

	all := gate.AllFeatures()

	for name, allowed := range all {
		if !allowed {
			t.Errorf("AllFeatures: feature %q should be allowed on pro tier", name)
		}
	}
}

func TestErrProRequired(t *testing.T) {
	if ErrProRequired == nil {
		t.Fatal("ErrProRequired should be a non-nil error")
	}
	if ErrProRequired.Error() != "this feature requires KubeWatcher Pro" {
		t.Errorf("ErrProRequired.Error() = %q, want %q",
			ErrProRequired.Error(), "this feature requires KubeWatcher Pro")
	}
}

func TestFeatureConstants(t *testing.T) {
	if FeatureAlerts != "alerts" {
		t.Errorf("FeatureAlerts = %q, want %q", FeatureAlerts, "alerts")
	}
	if FeatureAIDiagnostics != "ai_diagnostics" {
		t.Errorf("FeatureAIDiagnostics = %q, want %q", FeatureAIDiagnostics, "ai_diagnostics")
	}
	if FeatureArgusCD != "arguscd" {
		t.Errorf("FeatureArgusCD = %q, want %q", FeatureArgusCD, "arguscd")
	}
}

func TestGateTierRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		tier config.Tier
	}{
		{"free", config.TierFree},
		{"pro", config.TierPro},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.OnlineDataConfig{
				Features: config.FeaturesConfig{
					Tier: tt.tier,
				},
			}
			gate := NewGate(cfg)
			if gate.Tier() != tt.tier {
				t.Errorf("gate.Tier() = %q, want %q", gate.Tier(), tt.tier)
			}
		})
	}
}
