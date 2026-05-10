package k8s

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argues/kube-watcher/internal/config"
)

func clientWithDeployments(deps ...appsv1.Deployment) *Client {
	cs := fake.NewSimpleClientset()
	ctx := context.Background()
	for i := range deps {
		_, _ = cs.AppsV1().Deployments(deps[i].Namespace).Create(ctx, &deps[i], metav1.CreateOptions{})
	}
	return &Client{
		cs:     cs,
		cfg:    &config.OnlineDataConfig{},
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

func deployment(namespace, name string) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
}

// TestFindDeploymentNamespace_SingleMatch is the happy path. Drives the
// ArgoCD-sync fallback: when the user only has a Deployment name (no
// namespace), the API would otherwise reject the request with
// "an empty namespace may not be set when a resource name is provided".
func TestFindDeploymentNamespace_SingleMatch(t *testing.T) {
	c := clientWithDeployments(
		deployment("kube-system", "coredns"),
		deployment("default", "web"),
	)
	got, err := c.FindDeploymentNamespace(context.Background(), "web")
	if err != nil {
		t.Fatalf("FindDeploymentNamespace: %v", err)
	}
	if got != "default" {
		t.Errorf("namespace = %q, want %q", got, "default")
	}
}

func TestFindDeploymentNamespace_NotFound(t *testing.T) {
	c := clientWithDeployments(deployment("default", "web"))
	_, err := c.FindDeploymentNamespace(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error for missing deployment, got nil")
	}
	if !strings.Contains(err.Error(), "no Deployment named") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFindDeploymentNamespace_AmbiguousAcrossNamespaces(t *testing.T) {
	c := clientWithDeployments(
		deployment("prod", "api"),
		deployment("staging", "api"),
	)
	_, err := c.FindDeploymentNamespace(context.Background(), "api")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
	// User-actionable hint: the error message should mention which
	// namespaces matched so the operator can decide.
	if !strings.Contains(err.Error(), "prod") || !strings.Contains(err.Error(), "staging") {
		t.Errorf("expected matching namespaces in error, got: %v", err)
	}
}

func TestFindDeploymentNamespace_RejectsEmptyName(t *testing.T) {
	c := clientWithDeployments()
	_, err := c.FindDeploymentNamespace(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}
