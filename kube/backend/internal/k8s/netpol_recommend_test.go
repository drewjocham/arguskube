package k8s

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argues/argus/internal/config"
)

func TestRecommendNetworkPolicies_DefaultDeny(t *testing.T) {
	t.Parallel()

	// Namespace with 2 pods, zero NetworkPolicies → should produce a
	// "default-deny" warning with a ready-to-apply YAML.
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "prod-api"}}
	p1 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "prod-api", Labels: map[string]string{"app": "web"}}}
	p2 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-2", Namespace: "prod-api", Labels: map[string]string{"app": "web"}}}
	cs := fake.NewSimpleClientset(ns, p1, p2)
	c := &Client{cs: cs, cfg: &config.OnlineDataConfig{}, logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}

	got, err := c.RecommendNetworkPolicies(context.Background(), "prod-api")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 recommendation, got %d: %+v", len(got), got)
	}
	r := got[0]
	if r.Severity != "warning" {
		t.Errorf("severity = %q, want warning", r.Severity)
	}
	if r.Evidence["pods"] != "2" {
		t.Errorf("evidence.pods = %q, want 2", r.Evidence["pods"])
	}
	if r.SuggestedYAML == "" || !strings.Contains(r.SuggestedYAML, "default-deny-all") {
		t.Errorf("expected a default-deny YAML, got %q", r.SuggestedYAML)
	}
}

func TestRecommendNetworkPolicies_StaleSelector(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "billing"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "billing", Labels: map[string]string{"app": "api"}}}
	// Policy selects app=worker — no pod carries that label, so stale.
	stale := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-egress", Namespace: "billing"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "worker"}},
		},
	}
	cs := fake.NewSimpleClientset(ns, pod, stale)
	c := &Client{cs: cs, cfg: &config.OnlineDataConfig{}, logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}

	got, err := c.RecommendNetworkPolicies(context.Background(), "billing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var staleRec *NetpolRecommendation
	for i := range got {
		if got[i].ID == "stale-selector-billing-worker-egress" {
			staleRec = &got[i]
		}
	}
	if staleRec == nil {
		t.Fatalf("expected stale-selector recommendation, got: %+v", got)
	}
	if staleRec.Severity != "info" {
		t.Errorf("severity = %q, want info", staleRec.Severity)
	}
	if staleRec.Evidence["matchedPodCount"] != "0" {
		t.Errorf("evidence.matchedPodCount = %q, want 0", staleRec.Evidence["matchedPodCount"])
	}
}

func TestRecommendNetworkPolicies_DualDeny(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "isolated"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "isolated", Labels: map[string]string{"app": "x"}}}
	// Declares both PolicyTypes but supplies zero rules — silent
	// "deny everything" foot-gun.
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "lockdown", Namespace: "isolated"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
		},
	}
	cs := fake.NewSimpleClientset(ns, pod, policy)
	c := &Client{cs: cs, cfg: &config.OnlineDataConfig{}, logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}

	got, err := c.RecommendNetworkPolicies(context.Background(), "isolated")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var dual *NetpolRecommendation
	for i := range got {
		if got[i].ID == "dual-deny-isolated-lockdown" {
			dual = &got[i]
		}
	}
	if dual == nil {
		t.Fatalf("expected dual-deny recommendation, got: %+v", got)
	}
}

func TestRecommendNetworkPolicies_HealthyNamespace_NoRecs(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "good"}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "good", Labels: map[string]string{"app": "x"}}}
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "x-allow", Namespace: "good"},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{From: []networkingv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{}}}},
			},
		},
	}
	cs := fake.NewSimpleClientset(ns, pod, policy)
	c := &Client{cs: cs, cfg: &config.OnlineDataConfig{}, logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}

	got, err := c.RecommendNetworkPolicies(context.Background(), "good")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 recommendations for healthy namespace, got %d: %+v", len(got), got)
	}
}

func TestSeverityRank(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"critical": 0, "warning": 1, "info": 2, "other": 3, "": 3,
	}
	for in, want := range cases {
		if got := severityRank(in); got != want {
			t.Errorf("severityRank(%q) = %d, want %d", in, got, want)
		}
	}
}

