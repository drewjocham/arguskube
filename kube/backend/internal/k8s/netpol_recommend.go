package k8s

import (
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetpolRecommendation is one piece of guidance the frontend renders as
// a collapsible card under the NetworkPolicy list. The shape is
// deliberately small and data-grounded — every recommendation carries
// the evidence the heuristic used, so the user can click into the
// card and see *why* Argus is suggesting this.
type NetpolRecommendation struct {
	// ID is stable per (namespace, kind) — lets the frontend remember
	// dismissed cards without showing them again on the next refresh.
	ID string `json:"id"`

	// Severity tunes the card color: info / warning / critical.
	Severity string `json:"severity"`

	// Title is the one-line summary (e.g. "Add a default-deny
	// NetworkPolicy"). Surfaced collapsed.
	Title string `json:"title"`

	// Reasoning is the expanded explanation the user sees on click.
	// Prose, 1–3 sentences. References the evidence by name.
	Reasoning string `json:"reasoning"`

	// Evidence is the structured data the heuristic looked at. The
	// frontend renders it as a key→value list under the reasoning
	// so the user can verify the guidance against their cluster.
	Evidence map[string]string `json:"evidence,omitempty"`

	// SuggestedYAML, when non-empty, is a ready-to-apply NetworkPolicy
	// manifest the user can paste into the Create flow without
	// authoring from scratch. The card surfaces an "Apply suggested
	// fix" button when this is present.
	SuggestedYAML string `json:"suggestedYAML,omitempty"`
}

// RecommendNetworkPolicies inspects the live cluster state in a
// namespace and returns data-grounded recommendations. Empty namespace
// means "look across the whole cluster" — recommendations stay scoped
// per namespace within the result so the frontend can group them.
//
// Heuristics in this first pass:
//
//  1. Namespaces with workloads but zero NetworkPolicy resources:
//     suggest a default-deny baseline. The traffic going IN/OUT of
//     such a namespace is unconstrained today, which is the single
//     most common cause of accidental lateral movement.
//
//  2. NetworkPolicies that match nothing (empty pod selector AND zero
//     pods carrying any of its selector labels): suggest removing or
//     re-targeting. Stale policies are confusing during incident
//     review and accumulate over deployments.
//
//  3. NetworkPolicies missing both Ingress AND Egress rule blocks
//     while declaring both policy types: cluster behavior depends on
//     CNI but most users meant to leave one open; flag for review.
//
// More heuristics land in follow-up PRs.
func (c *Client) RecommendNetworkPolicies(ctx context.Context, namespace string) ([]NetpolRecommendation, error) {
	// One-shot list both namespaces (when asking cluster-wide) and
	// the NetworkPolicies + pods we'll inspect. Keeps the function
	// bounded — large clusters mean lots of objects, but we only
	// touch each one once.
	nsList, err := c.namespacesToScan(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	var out []NetpolRecommendation
	for _, ns := range nsList {
		nsRecs, err := c.recommendForNamespace(ctx, ns)
		if err != nil {
			// One bad namespace doesn't tank the whole call —
			// surface the error as an info-tier recommendation
			// so the user knows we tried but couldn't reach it.
			out = append(out, NetpolRecommendation{
				ID:        "scan-error-" + ns,
				Severity:  "info",
				Title:     fmt.Sprintf("Couldn't analyze namespace %q", ns),
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

func (c *Client) namespacesToScan(ctx context.Context, namespace string) ([]string, error) {
	if namespace != "" {
		return []string{namespace}, nil
	}
	list, err := c.cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list.Items))
	for _, n := range list.Items {
		names = append(names, n.Name)
	}
	sort.Strings(names)
	return names, nil
}

func (c *Client) recommendForNamespace(ctx context.Context, ns string) ([]NetpolRecommendation, error) {
	pods, err := c.cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	policies, err := c.cs.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var out []NetpolRecommendation

	// Heuristic 1: workload-bearing namespace with zero NetworkPolicies.
	if len(pods.Items) > 0 && len(policies.Items) == 0 {
		out = append(out, NetpolRecommendation{
			ID:       "default-deny-" + ns,
			Severity: "warning",
			Title:    fmt.Sprintf("Add a default-deny NetworkPolicy to %q", ns),
			Reasoning: fmt.Sprintf(
				"Namespace %q runs %d pod(s) but has no NetworkPolicy resources. "+
					"Without one, every pod accepts traffic from every other pod "+
					"in the cluster. A default-deny baseline is the standard "+
					"hardening step — additional policies then allow only the "+
					"flows you actually need.",
				ns, len(pods.Items),
			),
			Evidence: map[string]string{
				"namespace":   ns,
				"pods":        fmt.Sprintf("%d", len(pods.Items)),
				"netpolicies": "0",
			},
			SuggestedYAML: defaultDenyYAML(ns),
		})
	}

	// Heuristic 2: stale policies (selector matches no live pod).
	podLabels := collectPodLabelSets(pods.Items)
	for i := range policies.Items {
		p := &policies.Items[i]
		if selectorMatchesAny(p.Spec.PodSelector, podLabels) {
			continue
		}
		out = append(out, NetpolRecommendation{
			ID:       "stale-selector-" + ns + "-" + p.Name,
			Severity: "info",
			Title:    fmt.Sprintf("NetworkPolicy %q matches no pods in %q", p.Name, ns),
			Reasoning: "The pod selector on this policy doesn't match any " +
				"currently-running pod in the namespace. It might be left over " +
				"from a deleted workload, or its selector drifted away from the " +
				"pod template labels. Remove it or re-target.",
			Evidence: map[string]string{
				"namespace":       ns,
				"policy":          p.Name,
				"podSelector":     selectorString(p.Spec.PodSelector),
				"matchedPodCount": "0",
			},
		})
	}

	// Heuristic 3: policy declares both Ingress + Egress policy types
	// but supplies empty rule blocks for both. That's a default-deny
	// for both directions, which is rarely the intent vs. one or the
	// other.
	for i := range policies.Items {
		p := &policies.Items[i]
		hasIngress, hasEgress := false, false
		for _, pt := range p.Spec.PolicyTypes {
			if pt == networkingv1.PolicyTypeIngress {
				hasIngress = true
			}
			if pt == networkingv1.PolicyTypeEgress {
				hasEgress = true
			}
		}
		if !(hasIngress && hasEgress) {
			continue
		}
		if len(p.Spec.Ingress) == 0 && len(p.Spec.Egress) == 0 {
			out = append(out, NetpolRecommendation{
				ID:       "dual-deny-" + ns + "-" + p.Name,
				Severity: "info",
				Title:    fmt.Sprintf("Policy %q denies both ingress AND egress", p.Name),
				Reasoning: "This policy declares both Ingress and Egress policy " +
					"types but provides zero allow rules for either direction. " +
					"That means matching pods can't talk to anything, including " +
					"DNS — a common foot-gun. If you intended a default-deny " +
					"for only one direction, split into two policies.",
				Evidence: map[string]string{
					"namespace":    ns,
					"policy":       p.Name,
					"policyTypes":  "Ingress + Egress",
					"ingressRules": "0",
					"egressRules":  "0",
				},
			})
		}
	}

	return out, nil
}

// collectPodLabelSets pulls the label map out of every pod in the
// list. Kept separate from the corev1 list shape so the stale-
// selector check is unit-testable without spinning up a full pod
// object in tests.
func collectPodLabelSets(pods []corev1.Pod) []map[string]string {
	out := make([]map[string]string, 0, len(pods))
	for _, p := range pods {
		out = append(out, p.Labels)
	}
	return out
}

// selectorMatchesAny reports whether the given LabelSelector matches
// any of the supplied label maps. Empty matchLabels = match-anything
// (LabelSelector semantics).
func selectorMatchesAny(sel metav1.LabelSelector, podLabels []map[string]string) bool {
	if len(sel.MatchLabels) == 0 && len(sel.MatchExpressions) == 0 {
		return len(podLabels) > 0
	}
	for _, pl := range podLabels {
		if labelsContainAll(pl, sel.MatchLabels) {
			return true
		}
	}
	return false
}

func labelsContainAll(have map[string]string, need map[string]string) bool {
	for k, v := range need {
		if have[k] != v {
			return false
		}
	}
	return true
}

func selectorString(sel metav1.LabelSelector) string {
	if len(sel.MatchLabels) == 0 {
		return "(empty — matches all pods)"
	}
	parts := make([]string, 0, len(sel.MatchLabels))
	for k, v := range sel.MatchLabels {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ","
		}
		out += p
	}
	return out
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 0
	case "warning":
		return 1
	case "info":
		return 2
	default:
		return 3
	}
}

func defaultDenyYAML(ns string) string {
	return `apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: ` + ns + `
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
`
}
