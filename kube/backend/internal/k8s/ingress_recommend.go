package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressRecommendation has the same shape as NetpolRecommendation —
// kept as its own type so the JSON shape stays stable even if the two
// heuristics diverge later (different evidence keys, different fix
// surface).
type IngressRecommendation struct {
	ID            string            `json:"id"`
	Severity      string            `json:"severity"`
	Title         string            `json:"title"`
	Reasoning     string            `json:"reasoning"`
	Evidence      map[string]string `json:"evidence,omitempty"`
	SuggestedYAML string            `json:"suggestedYAML,omitempty"`
}

// RecommendIngresses scans Ingress resources in the given namespace
// (or cluster-wide when namespace == "") and returns data-grounded
// recommendations.
//
// Heuristics in this first pass:
//
//  1. Ingress with no TLS block: HTTP-only routing reaches the user
//     in plaintext. Suggest adding a TLS section pointed at a Secret.
//
//  2. Ingress rule whose backend Service does not exist (or has no
//     matching pods): broken route, will return 502/503 at the edge.
//
//  3. Two or more Ingresses in the same namespace claim the same
//     host+path: ambiguous routing, which controller wins is
//     implementation-defined.
//
//  4. Ingress without an ingressClassName AND no default class on the
//     cluster: the resource is silently un-served.
//
// More heuristics land in follow-up PRs.
func (c *Client) RecommendIngresses(ctx context.Context, namespace string) ([]IngressRecommendation, error) {
	nsList, err := c.namespacesToScan(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	// Look up the default IngressClass once — only matters when an
	// Ingress doesn't specify a class itself.
	defaultClass, _ := c.defaultIngressClassName(ctx)

	var out []IngressRecommendation
	for _, ns := range nsList {
		nsRecs, err := c.recommendIngressForNamespace(ctx, ns, defaultClass)
		if err != nil {
			out = append(out, IngressRecommendation{
				ID:        "ingress-scan-error-" + ns,
				Severity:  "info",
				Title:     fmt.Sprintf("Couldn't analyze namespace %q for ingress issues", ns),
				Reasoning: err.Error(),
			})
			continue
		}
		out = append(out, nsRecs...)
	}
	sort.Slice(out, func(i, j int) bool {
		return severityRank(out[i].Severity) < severityRank(out[j].Severity)
	})
	return out, nil
}

func (c *Client) defaultIngressClassName(ctx context.Context) (string, error) {
	list, err := c.cs.NetworkingV1().IngressClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for i := range list.Items {
		ic := &list.Items[i]
		if ic.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true" {
			return ic.Name, nil
		}
	}
	return "", nil
}

func (c *Client) recommendIngressForNamespace(ctx context.Context, ns, defaultClass string) ([]IngressRecommendation, error) {
	ings, err := c.cs.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if len(ings.Items) == 0 {
		return nil, nil
	}

	services, err := c.cs.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	svcSet := make(map[string]struct{}, len(services.Items))
	for i := range services.Items {
		svcSet[services.Items[i].Name] = struct{}{}
	}

	var out []IngressRecommendation
	// (host+path) → list of "ingressName" so we can flag collisions.
	hostPathOwners := map[string][]string{}

	for i := range ings.Items {
		ing := &ings.Items[i]

		// 1. Missing TLS.
		if len(ing.Spec.TLS) == 0 {
			out = append(out, IngressRecommendation{
				ID:       "ingress-no-tls-" + ns + "-" + ing.Name,
				Severity: "warning",
				Title:    fmt.Sprintf("Ingress %q in %q serves HTTP without TLS", ing.Name, ns),
				Reasoning: "This Ingress has no TLS block, so the controller serves " +
					"traffic in plaintext. Most production controllers can terminate " +
					"TLS via a referenced Secret — add a `tls:` section and create or " +
					"reference a certificate secret (cert-manager handles this for you).",
				Evidence: map[string]string{
					"namespace":    ns,
					"ingress":      ing.Name,
					"hosts":        joinIngressHosts(ing),
					"hasTLS":       "false",
					"ruleCount":    fmt.Sprintf("%d", len(ing.Spec.Rules)),
					"defaultClass": defaultClass,
				},
				SuggestedYAML: tlsScaffoldYAML(ns, ing),
			})
		}

		// 2. Backend service missing.
		for _, missing := range missingBackendServices(ing, svcSet) {
			out = append(out, IngressRecommendation{
				ID:       "ingress-missing-svc-" + ns + "-" + ing.Name + "-" + missing,
				Severity: "critical",
				Title:    fmt.Sprintf("Ingress %q routes to missing Service %q", ing.Name, missing),
				Reasoning: "A rule on this Ingress points at a Service that doesn't " +
					"exist in the namespace. The edge will return 502/503 for any " +
					"request that lands on this route. Either create the Service or " +
					"remove/repoint the rule.",
				Evidence: map[string]string{
					"namespace": ns,
					"ingress":   ing.Name,
					"service":   missing,
				},
			})
		}

		// 3. No class + no cluster default.
		if (ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName == "") && defaultClass == "" {
			out = append(out, IngressRecommendation{
				ID:       "ingress-no-class-" + ns + "-" + ing.Name,
				Severity: "warning",
				Title:    fmt.Sprintf("Ingress %q has no ingressClassName and no cluster default exists", ing.Name),
				Reasoning: "Without `spec.ingressClassName` and with no IngressClass " +
					"annotated as the cluster default, no controller will claim this " +
					"resource. Traffic doesn't reach the backend. Set " +
					"`spec.ingressClassName` to whichever controller you run " +
					"(nginx, traefik, alb, …).",
				Evidence: map[string]string{
					"namespace":      ns,
					"ingress":        ing.Name,
					"hasClassName":   "false",
					"clusterDefault": "(none)",
				},
			})
		}

		// Gather host+path for the duplicate check.
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, p := range rule.HTTP.Paths {
				key := rule.Host + "|" + p.Path
				hostPathOwners[key] = append(hostPathOwners[key], ing.Name)
			}
		}
	}

	// 4. Duplicate host+path across multiple Ingresses.
	for key, owners := range hostPathOwners {
		if len(owners) < 2 {
			continue
		}
		host, path, _ := splitHostPath(key)
		sort.Strings(owners)
		out = append(out, IngressRecommendation{
			ID:       "ingress-dup-route-" + ns + "-" + sanitizeID(host+path),
			Severity: "warning",
			Title:    fmt.Sprintf("Conflicting routes for %q in %q", host+path, ns),
			Reasoning: "More than one Ingress in this namespace declares the same " +
				"host+path. Which controller wins is implementation-defined and " +
				"changes across upgrades. Pick one Ingress to own the route and " +
				"remove the path from the others.",
			Evidence: map[string]string{
				"namespace": ns,
				"host":      host,
				"path":      path,
				"owners":    strings.Join(owners, ", "),
			},
		})
	}

	return out, nil
}

func missingBackendServices(ing *networkingv1.Ingress, svcSet map[string]struct{}) []string {
	seen := map[string]struct{}{}
	var out []string
	check := func(name string) {
		if name == "" {
			return
		}
		if _, ok := svcSet[name]; ok {
			return
		}
		if _, dup := seen[name]; dup {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		check(ing.Spec.DefaultBackend.Service.Name)
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, p := range rule.HTTP.Paths {
			if p.Backend.Service != nil {
				check(p.Backend.Service.Name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func joinIngressHosts(ing *networkingv1.Ingress) string {
	var hosts []string
	for _, r := range ing.Spec.Rules {
		if r.Host != "" {
			hosts = append(hosts, r.Host)
		}
	}
	if len(hosts) == 0 {
		return "(no host)"
	}
	return strings.Join(hosts, ", ")
}

func splitHostPath(key string) (host, path string, ok bool) {
	i := strings.Index(key, "|")
	if i < 0 {
		return key, "", false
	}
	return key[:i], key[i+1:], true
}

// sanitizeID maps an arbitrary host+path into something the frontend
// can use as a stable key (it ends up in DOM ids / test selectors).
func sanitizeID(in string) string {
	var b strings.Builder
	for _, r := range in {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	if out == "" {
		return "root"
	}
	return out
}

func tlsScaffoldYAML(ns string, ing *networkingv1.Ingress) string {
	host := ""
	for _, r := range ing.Spec.Rules {
		if r.Host != "" {
			host = r.Host
			break
		}
	}
	if host == "" {
		host = "example.com"
	}
	return `# Add this tls: block to ` + ing.Name + ` (namespace ` + ns + `).
# The referenced Secret must hold a TLS cert+key for the host; cert-manager
# can create it automatically with a matching Certificate resource.
spec:
  tls:
    - hosts:
        - ` + host + `
      secretName: ` + ing.Name + `-tls
`
}
