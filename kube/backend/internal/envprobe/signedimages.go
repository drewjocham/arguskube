package envprobe

import (
	"context"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GroupLister returns the API groups served by the cluster. Production
// uses the kubernetes.Interface Discovery() implementation; tests inject
// a stub so the probe runs without a real cluster.
type GroupLister interface {
	ServerGroups(ctx context.Context) ([]string, error)
}

// engineMatch defines which API group → policy engine label we expose
// to the user. Order is deliberate — we report the most-deployed engine
// first when multiple are installed, since the user likely has policy
// authored against it.
type engineMatch struct {
	GroupSuffix string // matched against the tail of every served group name
	Engine      string // human-readable engine name
}

var policyEngines = []engineMatch{
	{GroupSuffix: "kyverno.io", Engine: "Kyverno"},
	{GroupSuffix: "templates.gatekeeper.sh", Engine: "OPA Gatekeeper"},
	{GroupSuffix: "policy.sigstore.dev", Engine: "Sigstore policy-controller"},
}

// SignedImagesProbe asks the cluster's Discovery API which policy
// engines are installed. Presence of a policy engine doesn't guarantee
// signed-image enforcement, but it does mean *something* policy-shaped
// is in play; surfacing a one-click "Apply Argus trust policy" row
// then is the right default. If no engine is present the row is OK and
// purely informational.
type SignedImagesProbe struct {
	lister GroupLister
}

func NewSignedImagesProbe(lister GroupLister) *SignedImagesProbe {
	return &SignedImagesProbe{lister: lister}
}

// ClientsetProvider returns the live kubernetes.Interface. It's a
// function (not a captured value) so a context switch is observed
// without re-registering the probe — SwitchContext replaces the
// clientset on the underlying *k8s.Client and the provider re-reads it.
type ClientsetProvider func() kubernetes.Interface

// NewSignedImagesProbeFromClient wires the probe to a real
// kubernetes.Interface — used in production. Kept separate from
// NewSignedImagesProbe so tests don't have to import client-go.
func NewSignedImagesProbeFromClient(provider ClientsetProvider) *SignedImagesProbe {
	return NewSignedImagesProbe(&clientsetGroupLister{provider: provider})
}

func (p *SignedImagesProbe) ID() string { return "envprobe.signed-images" }

func (p *SignedImagesProbe) Run(ctx context.Context) Result {
	res := Result{ID: p.ID(), Title: "Cluster admission policy"}
	if p.lister == nil {
		res.Status = Warn
		res.Detail = "No cluster selected yet."
		return res
	}

	groups, err := p.lister.ServerGroups(ctx)
	if err != nil {
		// Discovery failure is owned by the DNS/TLS probes; soft-warn so
		// we don't double-report the same root cause.
		res.Status = Warn
		res.Detail = "Could not query the API server's discovery endpoint."
		return res
	}

	matches := detectEngines(groups)
	if len(matches) == 0 {
		res.Status = OK
		res.Detail = "No image-policy engine detected — Argus images can be pulled directly."
		return res
	}

	// Stable, comma-joined list so the row text doesn't shuffle between sweeps.
	sort.Strings(matches)
	res.Status = Todo
	res.Title = "Cluster requires signed images"
	res.Detail = fmt.Sprintf(
		"%s detected. Apply the Argus trust policy so the agent can be admitted.",
		strings.Join(matches, ", "),
	)
	res.ActionLabel = "Apply Argus trust policy"
	res.ActionID = "envprobe.apply-trust-policy"
	return res
}

// detectEngines returns the engine display names that are present in the
// served API groups. Exported indirectly via the probe; pulled out as a
// pure function for testability.
func detectEngines(groups []string) []string {
	seen := map[string]struct{}{}
	for _, g := range groups {
		g = strings.ToLower(g)
		for _, m := range policyEngines {
			if g == m.GroupSuffix || strings.HasSuffix(g, "/"+m.GroupSuffix) {
				seen[m.Engine] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for e := range seen {
		out = append(out, e)
	}
	return out
}

// clientsetGroupLister adapts kubernetes.Interface to GroupLister. The
// production code path goes through Discovery().ServerGroups(); we only
// keep the group name (not versions) because that's all the probe
// reasons about. The provider is re-read on every call so a context
// switch (which replaces the clientset on the *k8s.Client struct) is
// observed automatically.
type clientsetGroupLister struct {
	provider ClientsetProvider
}

func (l *clientsetGroupLister) ServerGroups(ctx context.Context) ([]string, error) {
	if l.provider == nil {
		return nil, fmt.Errorf("clientset provider not configured")
	}
	cs := l.provider()
	if cs == nil {
		return nil, fmt.Errorf("no cluster connection")
	}
	// Discovery does not accept a context; race it against ctx.Done().
	type out struct {
		list *metav1.APIGroupList
		err  error
	}
	ch := make(chan out, 1)
	go func() {
		g, err := cs.Discovery().ServerGroups()
		ch <- out{list: g, err: err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		names := make([]string, 0, len(r.list.Groups))
		for _, g := range r.list.Groups {
			names = append(names, g.Name)
		}
		return names, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
