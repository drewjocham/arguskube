package palette

import (
	"context"
	"errors"
	"testing"
)

// stubPalette is a minimal ShellPalette for registry tests.
type stubPalette struct{ name string }

func (s stubPalette) Name() string                                           { return s.name }
func (s stubPalette) Tabs() []Tab                                            { return []Tab{{ID: "t", Label: "T"}} }
func (s stubPalette) List(context.Context, string) ([]Group, error)          { return nil, nil }
func (s stubPalette) Command(string, ActionID, Resource) (string, error)     { return "", ErrUnsupportedAction }

func TestRegistryRegisterAndGet(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(stubPalette{name: "k8s"})
	r.Register(stubPalette{name: "solace"})

	if r.Len() != 2 {
		t.Errorf("Len = %d, want 2", r.Len())
	}

	got, ok := r.Get("k8s")
	if !ok || got.Name() != "k8s" {
		t.Errorf("Get k8s = (%v, %v); want (k8s, true)", got, ok)
	}
	if _, ok := r.Get("missing"); ok {
		t.Error("Get missing should report not-found")
	}
}

func TestRegistryPreservesRegistrationOrder(t *testing.T) {
	t.Parallel()
	r := NewRegistry()
	r.Register(stubPalette{name: "k8s"})
	r.Register(stubPalette{name: "solace"})
	r.Register(stubPalette{name: "pubsub"})

	names := r.Names()
	want := []string{"k8s", "solace", "pubsub"}
	if !equalStr(names, want) {
		t.Errorf("Names = %v, want %v", names, want)
	}

	all := r.All()
	if len(all) != 3 || all[0].Name() != "k8s" || all[2].Name() != "pubsub" {
		t.Errorf("All() order broken: %v", paletteNames(all))
	}
}

func TestRegistryRejectsDuplicate(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("duplicate Register should panic")
		}
	}()
	r := NewRegistry()
	r.Register(stubPalette{name: "k8s"})
	r.Register(stubPalette{name: "k8s"})
}

func TestRegistryRejectsEmpty(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("Register with empty Name() should panic")
		}
	}()
	NewRegistry().Register(stubPalette{name: ""})
}

func TestRegistryRejectsNil(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("Register(nil) should panic")
		}
	}()
	NewRegistry().Register(nil)
}

func TestFindTab(t *testing.T) {
	t.Parallel()
	tabs := []Tab{{ID: "pods", Label: "Pods"}, {ID: "svc", Label: "Services"}}
	tab, ok := FindTab(tabs, "pods")
	if !ok || tab.Label != "Pods" {
		t.Errorf("FindTab pods = (%v, %v)", tab, ok)
	}
	if _, ok := FindTab(tabs, "missing"); ok {
		t.Error("FindTab missing should report not-found")
	}
}

func TestResourceStringFallback(t *testing.T) {
	t.Parallel()
	if r := (Resource{Name: "n"}); r.String() != "n" {
		t.Errorf("Resource.String empty Display = %q", r.String())
	}
	if r := (Resource{Name: "n", Display: "n-*"}); r.String() != "n-*" {
		t.Errorf("Resource.String with Display = %q", r.String())
	}
}

func TestGroupCollapsedReportsMultiMember(t *testing.T) {
	t.Parallel()
	solo := Group{Members: []Resource{{Name: "n"}}}
	if solo.Collapsed() {
		t.Error("single-member group should not report Collapsed")
	}
	multi := Group{Members: []Resource{{Name: "a"}, {Name: "b"}}}
	if !multi.Collapsed() {
		t.Error("multi-member group should report Collapsed")
	}
}

func TestErrUnsupportedActionIsSentinel(t *testing.T) {
	t.Parallel()
	// Render layer's "should I hide this button?" check uses
	// errors.Is — verify the sentinel matches itself wrapped.
	wrapped := errors.Join(ErrUnsupportedAction, errors.New("extra context"))
	if !errors.Is(wrapped, ErrUnsupportedAction) {
		t.Error("ErrUnsupportedAction is not errors.Is-compatible")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────

func equalStr(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func paletteNames(p []ShellPalette) []string {
	out := make([]string, len(p))
	for i, x := range p {
		out[i] = x.Name()
	}
	return out
}
