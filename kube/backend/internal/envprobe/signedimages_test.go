package envprobe

import (
	"context"
	"errors"
	"testing"
)

type stubGroupLister struct {
	groups []string
	err    error
}

func (s stubGroupLister) ServerGroups(ctx context.Context) ([]string, error) {
	return s.groups, s.err
}

func TestSignedImagesProbe_NoEngineIsOK(t *testing.T) {
	p := NewSignedImagesProbe(stubGroupLister{
		groups: []string{"", "apps", "batch", "networking.k8s.io"},
	})
	res := p.Run(context.Background())
	if res.Status != OK {
		t.Errorf("vanilla cluster should be OK, got %s — %s", res.Status, res.Detail)
	}
}

func TestSignedImagesProbe_DetectsKyverno(t *testing.T) {
	p := NewSignedImagesProbe(stubGroupLister{
		groups: []string{"", "apps", "kyverno.io"},
	})
	res := p.Run(context.Background())
	if res.Status != Todo {
		t.Errorf("Kyverno install should be Todo, got %s", res.Status)
	}
	if res.ActionID != "envprobe.apply-trust-policy" {
		t.Errorf("expected apply-trust-policy action, got %s", res.ActionID)
	}
	if !contains(res.Detail, "Kyverno") {
		t.Errorf("detail should name Kyverno: %s", res.Detail)
	}
}

func TestSignedImagesProbe_DetectsGatekeeper(t *testing.T) {
	p := NewSignedImagesProbe(stubGroupLister{
		groups: []string{"templates.gatekeeper.sh"},
	})
	res := p.Run(context.Background())
	if res.Status != Todo {
		t.Errorf("Gatekeeper should be Todo, got %s", res.Status)
	}
	if !contains(res.Detail, "OPA Gatekeeper") {
		t.Errorf("detail should name OPA Gatekeeper: %s", res.Detail)
	}
}

func TestSignedImagesProbe_DetectsSigstorePolicyController(t *testing.T) {
	p := NewSignedImagesProbe(stubGroupLister{
		groups: []string{"policy.sigstore.dev"},
	})
	res := p.Run(context.Background())
	if res.Status != Todo {
		t.Errorf("Sigstore policy-controller should be Todo, got %s", res.Status)
	}
	if !contains(res.Detail, "Sigstore") {
		t.Errorf("detail should name Sigstore: %s", res.Detail)
	}
}

func TestSignedImagesProbe_MultipleEnginesAreDedupedAndSorted(t *testing.T) {
	p := NewSignedImagesProbe(stubGroupLister{
		groups: []string{
			"templates.gatekeeper.sh",
			"kyverno.io",
			// Run the same probe twice; we should NOT see "Kyverno, Kyverno".
			"some/kyverno.io",
		},
	})
	res := p.Run(context.Background())
	if res.Status != Todo {
		t.Fatalf("mixed install should be Todo, got %s", res.Status)
	}
	if countOccurrences(res.Detail, "Kyverno") != 1 {
		t.Errorf("Kyverno should appear once in detail: %s", res.Detail)
	}
	// Sorted alphabetically: Kyverno, OPA Gatekeeper.
	kIdx := indexOf(res.Detail, "Kyverno")
	oIdx := indexOf(res.Detail, "OPA Gatekeeper")
	if kIdx < 0 || oIdx < 0 || kIdx > oIdx {
		t.Errorf("engines should be sorted alphabetically: %s", res.Detail)
	}
}

func TestSignedImagesProbe_ListerErrorIsSoftWarn(t *testing.T) {
	p := NewSignedImagesProbe(stubGroupLister{err: errors.New("the server is currently unable to handle the request")})
	res := p.Run(context.Background())
	if res.Status != Warn {
		t.Errorf("discovery error should be Warn (owned by other probes), got %s", res.Status)
	}
}

func TestSignedImagesProbe_NilListerIsWarn(t *testing.T) {
	p := NewSignedImagesProbe(nil)
	res := p.Run(context.Background())
	if res.Status != Warn {
		t.Errorf("nil lister (no cluster yet) should be Warn, got %s", res.Status)
	}
}

func TestDetectEngines_PureFunction(t *testing.T) {
	got := detectEngines([]string{"kyverno.io", "kyverno.io", "apps"})
	if len(got) != 1 || got[0] != "Kyverno" {
		t.Errorf("dedupe broke: %v", got)
	}
}

// SwitchContext replaces the underlying clientset on the *k8s.Client
// struct. The probe must see the new groups without re-registration —
// otherwise the user would carry a stale verdict across contexts.
func TestSignedImagesProbe_ObservesProviderChanges(t *testing.T) {
	// Round 1: empty cluster.
	var current []string
	lister := groupListerFunc(func(ctx context.Context) ([]string, error) {
		return current, nil
	})
	p := NewSignedImagesProbe(lister)
	if got := p.Run(context.Background()).Status; got != OK {
		t.Fatalf("round 1 should be OK, got %s", got)
	}
	// Round 2: cluster now has Kyverno (simulates SwitchContext).
	current = []string{"kyverno.io"}
	if got := p.Run(context.Background()).Status; got != Todo {
		t.Fatalf("round 2 should be Todo after context switch, got %s", got)
	}
}

type groupListerFunc func(ctx context.Context) ([]string, error)

func (f groupListerFunc) ServerGroups(ctx context.Context) ([]string, error) {
	return f(ctx)
}

// --- minimal local helpers; we deliberately don't import strings/test
// into the file under test just for these.

func contains(s, sub string) bool {
	return indexOf(s, sub) >= 0
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
func countOccurrences(s, sub string) int {
	n := 0
	i := 0
	for i+len(sub) <= len(s) {
		if s[i:i+len(sub)] == sub {
			n++
			i += len(sub)
		} else {
			i++
		}
	}
	return n
}
