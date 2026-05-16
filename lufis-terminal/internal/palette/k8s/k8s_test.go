package k8s

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/argus/terminal/internal/palette"
)

// fakeRunner returns canned `kubectl ... -o name` output. The test
// captures the args so we can assert the right kubectl invocation
// was assembled.
type fakeRunner struct {
	gotArgs []string
	out     []byte
	err     error
}

func (f *fakeRunner) run(_ context.Context, _ string, args ...string) ([]byte, error) {
	f.gotArgs = args
	return f.out, f.err
}

func newWithRunner(r *fakeRunner) *Palette {
	return New().WithRunner(r.run)
}

func TestPaletteName(t *testing.T) {
	t.Parallel()
	if got := New().Name(); got != "k8s" {
		t.Errorf("Name = %q, want k8s", got)
	}
}

func TestPaletteTabsAreStableAndComplete(t *testing.T) {
	t.Parallel()
	tabs := New().Tabs()
	wantIDs := []string{"pods", "services", "configmaps", "nodes"}
	if len(tabs) != len(wantIDs) {
		t.Fatalf("got %d tabs, want %d", len(tabs), len(wantIDs))
	}
	for i, want := range wantIDs {
		if tabs[i].ID != want {
			t.Errorf("tabs[%d].ID = %q, want %q", i, tabs[i].ID, want)
		}
		if tabs[i].Label == "" {
			t.Errorf("tabs[%d] missing label", i)
		}
	}
}

func TestListPodsCollapsesDeploymentShape(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{out: []byte(`
pod/nginx-deployment-7c8d4-abc12
pod/nginx-deployment-7c8d4-def34
pod/nginx-deployment-7c8d4-ghi56
pod/redis-master-0
`)}
	p := newWithRunner(runner)
	groups, err := p.List(context.Background(), "pods")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (collapsed nginx + standalone redis); got %d: %+v", len(groups), groups)
	}
	// kubectl args land in the canonical "get pods -A -o name" form.
	if !equalSlice(runner.gotArgs, []string{"get", "pods", "-A", "-o", "name"}) {
		t.Errorf("kubectl args = %v, want [get pods -A -o name]", runner.gotArgs)
	}
}

func TestListUnknownTabErrors(t *testing.T) {
	t.Parallel()
	p := newWithRunner(&fakeRunner{})
	_, err := p.List(context.Background(), "no-such-tab")
	if !errors.Is(err, palette.ErrUnknownTab) {
		t.Errorf("expected ErrUnknownTab; got %v", err)
	}
}

func TestListSurfacesKubectlError(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{err: errors.New("kubectl: forbidden")}
	p := newWithRunner(runner)
	_, err := p.List(context.Background(), "pods")
	if err == nil {
		t.Fatal("expected error from kubectl runner; got nil")
	}
	if !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("expected the kubectl error to flow through; got %v", err)
	}
}

func TestCommandRendersForEachTab(t *testing.T) {
	t.Parallel()
	p := New()
	cases := []struct {
		tab      string
		resource string
		want     string
	}{
		{"pods", "nginx-7c8d4-abc12", "kubectl describe pod nginx-7c8d4-abc12"},
		{"services", "api", "kubectl describe svc api"},
		{"configmaps", "argus-cfg", "kubectl get cm argus-cfg -o yaml"},
		{"nodes", "node-1", "kubectl describe node node-1"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.tab, func(t *testing.T) {
			t.Parallel()
			got, err := p.Command(tc.tab, palette.ActionCopy, palette.Resource{Name: tc.resource})
			if err != nil {
				t.Fatalf("Command: %v", err)
			}
			if got != tc.want {
				t.Errorf("Command(%q) = %q, want %q", tc.tab, got, tc.want)
			}
		})
	}
}

func TestCommandRejectsEmptyResource(t *testing.T) {
	t.Parallel()
	p := New()
	_, err := p.Command("pods", palette.ActionCopy, palette.Resource{Name: ""})
	if err == nil {
		t.Fatal("expected an error for empty resource name")
	}
}

func TestCommandUnknownTabReturnsErrUnknownTab(t *testing.T) {
	t.Parallel()
	p := New()
	_, err := p.Command("nope", palette.ActionCopy, palette.Resource{Name: "x"})
	if !errors.Is(err, palette.ErrUnknownTab) {
		t.Errorf("expected ErrUnknownTab; got %v", err)
	}
}

func TestCommandAllRunActionsProduceSameString(t *testing.T) {
	t.Parallel()
	// Copy / Run-current / Run-new-shell differ in render-layer
	// effect, not in the command string. The palette returns the
	// same string for all three so the render layer can hand them
	// to clipboard vs PTY without further branching.
	p := New()
	picked := palette.Resource{Name: "foo"}
	a, _ := p.Command("pods", palette.ActionCopy, picked)
	b, _ := p.Command("pods", palette.ActionRunCurrent, picked)
	c, _ := p.Command("pods", palette.ActionRunNewShell, picked)
	if a != b || b != c {
		t.Errorf("expected same command across actions; got %q / %q / %q", a, b, c)
	}
}

func TestParseKubectlNamesStripsKindPrefix(t *testing.T) {
	t.Parallel()
	in := []byte(`pod/nginx-1
pod/nginx-2

pod/redis-0
`)
	got := parseKubectlNames(in)
	want := []string{"nginx-1", "nginx-2", "redis-0"}
	if !equalSlice(got, want) {
		t.Errorf("parseKubectlNames = %v, want %v", got, want)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────

func equalSlice(a, b []string) bool {
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
