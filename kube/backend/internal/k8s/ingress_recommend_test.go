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

func newIngress(ns, name string, host string, paths []string, tls bool, class string, backendSvc string) *networkingv1.Ingress {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{},
					},
				},
			},
		},
	}
	if class != "" {
		c := class
		ing.Spec.IngressClassName = &c
	}
	for _, p := range paths {
		pathType := networkingv1.PathTypePrefix
		ing.Spec.Rules[0].HTTP.Paths = append(ing.Spec.Rules[0].HTTP.Paths, networkingv1.HTTPIngressPath{
			Path:     p,
			PathType: &pathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{Name: backendSvc, Port: networkingv1.ServiceBackendPort{Number: 80}},
			},
		})
	}
	if tls {
		ing.Spec.TLS = []networkingv1.IngressTLS{
			{Hosts: []string{host}, SecretName: name + "-tls"},
		}
	}
	return ing
}

func TestRecommendIngresses_NoTLS(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "web"}}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "web"}}
	ing := newIngress("web", "edge", "shop.example.com", []string{"/"}, false, "nginx", "api")
	cs := fake.NewSimpleClientset(ns, svc, ing)
	c := &Client{cs: cs, cfg: &config.OnlineDataConfig{}, logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}

	got, err := c.RecommendIngresses(context.Background(), "web")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var tlsRec *IngressRecommendation
	for i := range got {
		if got[i].ID == "ingress-no-tls-web-edge" {
			tlsRec = &got[i]
		}
	}
	if tlsRec == nil {
		t.Fatalf("expected no-tls recommendation, got: %+v", got)
	}
	if tlsRec.Severity != "warning" {
		t.Errorf("severity = %q, want warning", tlsRec.Severity)
	}
	if !strings.Contains(tlsRec.SuggestedYAML, "shop.example.com") {
		t.Errorf("expected suggestedYAML to include host, got: %s", tlsRec.SuggestedYAML)
	}
}

func TestRecommendIngresses_MissingBackend(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "billing"}}
	// No "ghost" service exists.
	ing := newIngress("billing", "checkout", "pay.example.com", []string{"/"}, true, "nginx", "ghost")
	cs := fake.NewSimpleClientset(ns, ing)
	c := &Client{cs: cs, cfg: &config.OnlineDataConfig{}, logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}

	got, err := c.RecommendIngresses(context.Background(), "billing")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var miss *IngressRecommendation
	for i := range got {
		if got[i].ID == "ingress-missing-svc-billing-checkout-ghost" {
			miss = &got[i]
		}
	}
	if miss == nil {
		t.Fatalf("expected missing-backend recommendation, got: %+v", got)
	}
	if miss.Severity != "critical" {
		t.Errorf("severity = %q, want critical", miss.Severity)
	}
	if miss.Evidence["service"] != "ghost" {
		t.Errorf("evidence.service = %q, want ghost", miss.Evidence["service"])
	}
}

func TestRecommendIngresses_DuplicateRoute(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "shop"}}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "shop"}}
	a := newIngress("shop", "a", "store.example.com", []string{"/cart"}, true, "nginx", "web")
	b := newIngress("shop", "b", "store.example.com", []string{"/cart"}, true, "nginx", "web")
	cs := fake.NewSimpleClientset(ns, svc, a, b)
	c := &Client{cs: cs, cfg: &config.OnlineDataConfig{}, logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}

	got, err := c.RecommendIngresses(context.Background(), "shop")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var dup *IngressRecommendation
	for i := range got {
		if strings.HasPrefix(got[i].ID, "ingress-dup-route-shop-") {
			dup = &got[i]
		}
	}
	if dup == nil {
		t.Fatalf("expected dup-route recommendation, got: %+v", got)
	}
	if !strings.Contains(dup.Evidence["owners"], "a") || !strings.Contains(dup.Evidence["owners"], "b") {
		t.Errorf("expected both a and b in owners, got %q", dup.Evidence["owners"])
	}
}

func TestRecommendIngresses_NoClassNoDefault(t *testing.T) {
	t.Parallel()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "lost"}}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "lost"}}
	ing := newIngress("lost", "orphan", "x.example.com", []string{"/"}, true, "", "api")
	cs := fake.NewSimpleClientset(ns, svc, ing)
	c := &Client{cs: cs, cfg: &config.OnlineDataConfig{}, logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}

	got, err := c.RecommendIngresses(context.Background(), "lost")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var noClass *IngressRecommendation
	for i := range got {
		if got[i].ID == "ingress-no-class-lost-orphan" {
			noClass = &got[i]
		}
	}
	if noClass == nil {
		t.Fatalf("expected no-class recommendation, got: %+v", got)
	}
}

func TestRecommendIngresses_HealthyIngress_OnlyTLSWarn(t *testing.T) {
	t.Parallel()

	// Ingress with TLS, valid backend, explicit class, no duplicate
	// routes → should produce ZERO recommendations.
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "good"}}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "good"}}
	ing := newIngress("good", "edge", "ok.example.com", []string{"/"}, true, "nginx", "api")
	cs := fake.NewSimpleClientset(ns, svc, ing)
	c := &Client{cs: cs, cfg: &config.OnlineDataConfig{}, logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}

	got, err := c.RecommendIngresses(context.Background(), "good")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 recommendations for healthy ingress, got %d: %+v", len(got), got)
	}
}
